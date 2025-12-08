package NodeManage

import (
	"fmt"

	"github.com/sukasukasuka123/Gossip/MessageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

func pbToInternal(msg *pb.GossipMessage) MessageManage.GossipMessage[[]byte] {
	return MessageManage.GossipMessage[[]byte]{
		Hash:     msg.GetHash(),
		FromHash: msg.GetFromHash(),
		Payload:  msg.GetPayLoad(),
	}
}

func (n *GossipNode) Stream(stream pb.Gossip_StreamServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		isNew, _ := n.Storage.InitState(msg.Hash, keys(n.Neighbors))

		stream.Send(&pb.GossipACK{
			Hash:     msg.Hash,
			FromHash: n.NodeHash,
		})

		if isNew {
			internal := pbToInternal(msg)
			go n.broadcast(internal)
		}
	}
}

func (n *GossipNode) broadcast(msg MessageManage.GossipMessage[[]byte]) {
	for _, target := range keys(n.Neighbors) {
		if n.Storage.GetState(msg.Hash, target) {
			continue
		}

		ep := n.Neighbors[target]
		stream, err := n.StreamFactory.GetStream(target, ep)
		if err != nil {
			n.Logger.Log("connect fail: "+err.Error(), n.NodeHash)
			continue
		}

		n.Storage.MarkSent(msg.Hash, target)

		go stream.Send(&pb.GossipMessage{
			Hash:     msg.Hash,
			FromHash: msg.FromHash,
			PayLoad:  msg.Payload,
		})
	}
}
func (n *GossipNode) SendMessageToNode(targetHash string, msg *pb.GossipMessage) error {
	ep, ok := n.Neighbors[targetHash]
	if !ok {
		return fmt.Errorf("neighbor %s not found", targetHash)
	}

	// 从 StreamFactory 获取或建立 Stream
	stream, err := n.StreamFactory.GetStream(targetHash, ep)
	if err != nil {
		n.Logger.Log(fmt.Sprintf("[WARN] Failed to get stream for %s: %v", targetHash, err), n.NodeHash)
		return err
	}

	// 异步发送消息
	go func() {
		err := stream.Send(msg)
		if err != nil {
			n.Logger.Log(fmt.Sprintf("[WARN] Failed to send msg %s to %s: %v", msg.Hash, targetHash, err), n.NodeHash)
		} else {
			n.Logger.Log(fmt.Sprintf("[INFO] Sent msg %s to %s", msg.Hash, targetHash), n.NodeHash)
		}
	}()

	// 标记已发送
	n.Storage.MarkSent(msg.Hash, targetHash)
	return nil
}
