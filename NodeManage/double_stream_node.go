// ==================== DoubleStreamNode.go ====================
package NodeManage

import (
	"fmt"
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
	smgr *sm.StorageManage, // 传入 StorageManage
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
		slot.ForwardTo(node.MM)
		node.startAckSender(slot)
	})
	return node
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

// 这是一个理论上的函数，应该在建立连接后立即运行
func (n *DoubleStreamNode) startAckSender(slot *NeighborManage.NeighborSlot) {
	go func() {
		for ack := range n.MM.AckSendChan {
			// 简单逻辑：如果 ACK 来自本节点（n.NodeHash），就发送给邻居
			// 更好的方式是 ACK 应该有一个 ToHash 字段，但目前假设所有 ACK 都发送给这个唯一的邻居。
			if ack.FromHash == n.NodeHash {
				if err := slot.SendAck(ack); err != nil {
					// 处理错误，例如连接断开
					n.Logger.Log(fmt.Sprintf("Failed to send ACK to %s: %v", slot.PeerHash, err), n.NodeHash)
					return // 退出循环
				}
			}
		}
	}()
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
	// **新增：启动 ACK 发送协程**
	// n.MM 是 MessageManager 的指针，n.MM.AckSendChan 是节点自身的待发送 ACK 队列
	n.startAckSender(slot) // <-- 关键新增
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

	// 2. ❗ 关键：首先关闭所有客户端双流连接
	//    这一步会向远程服务器发送 EOF，允许远程服务器上的 gRPC 流接收 Goroutine 退出。
	if n.NeighborMgr != nil && n.NeighborMgr.Factory != nil {
		n.NeighborMgr.Factory.CloseAll()
		n.Logger.Log("All client streams closed.", n.NodeHash)
	}

	// 3. 等待一小段时间让处理循环结束
	//    给那些正在 select 或 channel 上等待的 Goroutine 100ms 响应时间。
	time.Sleep(100 * time.Millisecond)

	// 4. 停止 gRPC 服务器（现在它不会被阻塞，因为流已经被关闭）
	if n.grpcServer != nil {
		// 如果 Node 1 的 GracefulStop 被阻塞，说明 Node 2 上的客户端流还没关闭
		// 但 Node 2 自身也会在测试结尾调用 StopGRPC，所以它们最终会互相解除阻塞。
		n.grpcServer.GracefulStop()
		n.Logger.Log("gRPC server GracefulStop done.", n.NodeHash)
	}

	// 5. 关闭 MessageManager 的通道（确保 Manager 自身的 Goroutine 退出）
	//    注意：如果这些通道在 MessageManager 内部被循环使用，您可能需要检查关闭时机。
	//    假设这些通道的关闭会导致 MessageManager Loop 退出。
	if n.MM != nil {
		// 确保 MM 已经实例化
		close(n.MM.MesRecvChan)
		close(n.MM.AckRecvChan)
		n.Logger.Log("MessageManager channels closed.", n.NodeHash)
	}

	n.Logger.Log("DoubleStreamNode fully stopped.", n.NodeHash)
}
