package NodeManage_simpleMode

// double_stream_node.go
import (
	"fmt"
	"log"
	"time"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	"github.com/sukasukasuka123/Gossip/Logger"
	"github.com/sukasukasuka123/Gossip/MessageManage"
	"github.com/sukasukasuka123/Gossip/NeighborManage"
	"github.com/sukasukasuka123/Gossip/Router"
	sm "github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
	"google.golang.org/grpc"
)

type DoubleStreamNode struct {
	NodeHash string
	Cost     map[string]float64
	Fanout   int64

	Router Router.Router
	Logger Logger.Logger
	MM     *MessageManage.MessageManager

	NeighborMgr *NeighborManage.NeighborManager

	// gRPC 服务器实例
	grpcServer *grpc.Server
	stopChan   chan struct{}
}

func NewDoubleStreamNode(
	nodeHash string,
	router Router.Router,
	logger Logger.Logger,
	Nstore NeighborManage.NeighborStore,
	factory *GossipStreamFactory.DoubleStreamFactory,
	smgr *sm.StorageManage,
) *DoubleStreamNode {
	// 初始化消息管理器
	mm := MessageManage.NewMessageManager(nodeHash, smgr)

	// 初始化邻居管理器
	neighborMgr := NeighborManage.NewNeighborManager(nodeHash, Nstore, factory)

	node := &DoubleStreamNode{
		NodeHash:    nodeHash,
		Cost:        make(map[string]float64),
		Fanout:      3,
		Router:      router,
		Logger:      logger,
		MM:          mm,
		NeighborMgr: neighborMgr,
		stopChan:    make(chan struct{}),
	}

	// 自动绑定已有在线邻居的消息流
	neighborMgr.RangeOnlineNeighbors(func(info NeighborManage.NeighborInfo, slot *NeighborManage.NeighborSlot) {
		// 绑定消息接收转发
		slot.ForwardTo(node.MM)
		// 启动ACK写入协程
		slot.StartAckWriter(node.MM)
	})

	// 启动全局ACK路由器
	go node.startGlobalAckRouter()

	return node
}

// startGlobalAckRouter 全局ACK路由器：从MM.AckSendChan读取ACK，路由到对应的NeighborSlot
func (n *DoubleStreamNode) startGlobalAckRouter() {
	for {
		select {
		case ack := <-n.MM.AckSendChan:
			if ack == nil {
				continue
			}

			// 查找这个ACK对应的消息来源节点
			sourceNodeHash, ok := n.MM.GetMessageSource(ack.Hash)
			if !ok {
				log.Printf("[Node %s] Cannot find source for ACK %s, skipping", n.NodeHash, ack.Hash)
				continue
			}

			// 获取目标邻居的slot
			slot, ok := n.NeighborMgr.GetSlot(sourceNodeHash)
			if !ok {
				log.Printf("[Node %s] Cannot find slot for neighbor %s, skipping ACK %s",
					n.NodeHash, sourceNodeHash, ack.Hash)
				continue
			}

			// 使用非阻塞写入，避免死锁
			select {
			case slot.AckWriteChan <- ack:
				log.Printf("[Node %s] ACK routed to %s: Hash=%s", n.NodeHash, sourceNodeHash, ack.Hash)
			default:
				// 如果通道满了，异步发送（避免阻塞路由器）
				go func(s *NeighborManage.NeighborSlot, a *pb.GossipACK) {
					s.AckWriteChan <- a
					log.Printf("[Node %s] ACK async routed to %s: Hash=%s", n.NodeHash, s.PeerHash, a.Hash)
				}(slot, ack)
			}

		case <-n.stopChan:
			log.Printf("[Node %s] Global ACK router stopped", n.NodeHash)
			return
		}
	}
}

// 发送消息给单个目标
func (n *DoubleStreamNode) SendMessage(targetHash string, msg *pb.GossipMessage) error {
	slot, ok := n.NeighborMgr.GetSlot(targetHash)
	if !ok {
		return fmt.Errorf("neighbor %s not found", targetHash)
	}

	// 设置发送方信息
	msg.FromHash = n.NodeHash

	if err := slot.SendMessage(msg); err != nil {
		n.Logger.Log("SendMessage fail: "+err.Error(), n.NodeHash)
		return err
	}

	// 标记已发送
	n.MM.OnMessageSend(msg)
	return nil
}

// 广播消息（使用 Router Fanout 策略）
func (n *DoubleStreamNode) Broadcast(msg *pb.GossipMessage) {
	// 设置发送方信息
	msg.FromHash = n.NodeHash

	statesIface, _ := n.MM.SM.ScheduleSync(sm.OpGetAllStates, sm.Payload{Hash: msg.Hash})
	states := statesIface.(map[string]bool)

	router := Router.NewFanoutRouter()
	targets := router.SelectTargets(n.Cost, states)

	for _, nodeHash := range targets {
		_ = n.SendMessage(nodeHash, msg)
		n.MM.SM.Schedule(sm.OpMarkSent, sm.Payload{
			Hash:     msg.Hash,
			NodeHash: nodeHash,
		})
	}
}

// ConnectToNeighbor 主动连接到邻居节点
func (n *DoubleStreamNode) ConnectToNeighbor(neighbor NeighborManage.NeighborInfo) error {
	slot, err := n.NeighborMgr.Connect(neighbor)
	if err != nil {
		n.Logger.Log(fmt.Sprintf("Failed to connect to %s: %v", neighbor.NodeHash, err), n.NodeHash)
		return err
	}
	n.Logger.Log(fmt.Sprintf("Successfully connected to %s", neighbor.NodeHash), n.NodeHash)

	// 绑定消息接收转发
	slot.ForwardTo(n.MM)

	// 启动ACK写入协程
	slot.StartAckWriter(n.MM)

	return nil
}

// DisconnectNeighbor 断开与邻居的连接
func (n *DoubleStreamNode) DisconnectNeighbor(nodeHash string) {
	n.NeighborMgr.Disconnect(nodeHash)
	n.Logger.Log(fmt.Sprintf("Disconnected from %s", nodeHash), n.NodeHash)
}

// StopGRPC 停止所有服务
func (n *DoubleStreamNode) StopGRPC() {
	// 1. 发送停止信号给内部 Goroutine
	close(n.stopChan)
	n.Logger.Log("Stop signal sent.", n.NodeHash)

	// 2. 关闭所有客户端双流连接
	if n.NeighborMgr != nil && n.NeighborMgr.Factory != nil {
		n.NeighborMgr.Factory.CloseAll()
		n.Logger.Log("All client streams closed.", n.NodeHash)
	}

	// 3. 等待一小段时间让处理循环结束
	time.Sleep(100 * time.Millisecond)

	// 4. 停止 gRPC 服务器
	if n.grpcServer != nil {
		n.grpcServer.GracefulStop()
		n.Logger.Log("gRPC server GracefulStop done.", n.NodeHash)
	}

	// 5. 关闭 MessageManager 的通道
	if n.MM != nil {
		close(n.MM.MesRecvChan)
		close(n.MM.AckRecvChan)
		// 不关闭 AckSendChan，因为 globalAckRouter 正在使用
		// globalAckRouter 会在 stopChan 关闭后退出
		n.Logger.Log("MessageManager channels closed.", n.NodeHash)
	}

	n.Logger.Log("DoubleStreamNode fully stopped.", n.NodeHash)
}
