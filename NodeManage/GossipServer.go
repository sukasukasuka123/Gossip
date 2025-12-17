package NodeManage

import (
	"fmt"
	"io"
	"log"
	"net"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
	"google.golang.org/grpc"
)

// GossipServer 实现 proto 定义的 Gossip 服务
type GossipServer struct {
	pb.UnimplementedGossipServer
	node *DoubleStreamNode
}

func NewGossipServer(node *DoubleStreamNode) *GossipServer {
	return &GossipServer{
		node: node,
	}
}

// MessageStream 实现消息流服务（Server 端只接收）
func (s *GossipServer) MessageStream(stream pb.Gossip_MessageStreamServer) error {
	ctx := stream.Context()
	var remoteNodeHash string

	for {
		msg, err := stream.Recv()
		if msg == nil { // <-- 检查 msg 是否为 nil
			if err != nil {
				// 如果 msg 为 nil，且有错误，则处理错误并继续
				s.node.Logger.Log(fmt.Sprintf("MessageStream recv error from %s: %v", remoteNodeHash, err), s.node.NodeHash)
				if err == io.EOF {
					s.node.Logger.Log(fmt.Sprintf("MessageStream closed by %s", remoteNodeHash), s.node.NodeHash)
					return nil
				}
				return err
			}
			// 如果 msg 为 nil，且没有错误，可能是一个心跳或格式错误，先跳过或返回
			continue
		}
		log.Print("MessageStream recv called from GossipServer" + msg.GetHash() + ", FromHash: " + msg.GetFromHash())

		// 记录远程节点身份
		if remoteNodeHash == "" && msg.FromHash != "" {
			remoteNodeHash = msg.FromHash
			s.node.Logger.Log(fmt.Sprintf("MessageStream connected from %s", remoteNodeHash), s.node.NodeHash)
		}

		// 将接收到的消息放入处理队列
		select {
		case s.node.MM.MesRecvChan <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// AckStream 实现 ACK 流服务（Server 端只接收）
func (s *GossipServer) AckStream(stream pb.Gossip_AckStreamServer) error {
	ctx := stream.Context()
	var remoteNodeHash string

	for {
		ack, err := stream.Recv()
		log.Print("AckStream recv called from GossipServer" + ack.GetHash() + ", FromHash: " + ack.GetFromHash())
		if err == io.EOF {
			s.node.Logger.Log(fmt.Sprintf("AckStream closed by %s", remoteNodeHash), s.node.NodeHash)
			return nil
		}
		if err != nil {
			s.node.Logger.Log(fmt.Sprintf("AckStream recv error from %s: %v", remoteNodeHash, err), s.node.NodeHash)
			return err
		}

		// 记录远程节点身份
		if remoteNodeHash == "" && ack.FromHash != "" {
			remoteNodeHash = ack.FromHash
			s.node.Logger.Log(fmt.Sprintf("AckStream connected from %s", remoteNodeHash), s.node.NodeHash)
		}

		// 将接收到的 ACK 放入处理队列
		select {
		case s.node.MM.AckRecvChan <- ack:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// StartGRPCServer 启动 gRPC 服务端
func (n *DoubleStreamNode) StartGRPCServer(port string) error {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	n.grpcServer = grpc.NewServer()
	gossipServer := NewGossipServer(n)
	pb.RegisterGossipServer(n.grpcServer, gossipServer)

	n.Logger.Log(fmt.Sprintf("gRPC server listening on %s", port), n.NodeHash)

	// 启动服务（阻塞调用）
	go func() {
		if err := n.grpcServer.Serve(lis); err != nil {
			n.Logger.Log(fmt.Sprintf("gRPC server error: %v", err), n.NodeHash)
		}
	}()

	return nil
}
