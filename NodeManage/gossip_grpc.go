package NodeManage

import (
	"Gossip/MessageManage"
	pb "Gossip/gossip_rpc/proto"
	"context"
	"fmt"
)

func pbToInternal(msg *pb.GossipMessage) MessageManage.GossipMessage[[]byte] {
	return MessageManage.GossipMessage[[]byte]{
		Hash:     msg.GetHash(),
		FromHash: msg.GetFromHash(),
		Payload:  msg.GetPayLoad(),
	}
}

func (n *GossipNode) PutMessageToClient(ctx context.Context, message *pb.GossipMessage) (*pb.GossipACK, error) {
	hash := message.GetHash()
	fromHash := message.GetFromHash()

	// 初始化状态，判断是否新消息
	isNew, err := n.Storage.InitState(hash, keys(n.Neighbors))
	if err != nil {
		n.Logger.Log(fmt.Sprintf("[ERROR] Storage InitState failed for hash %s: %v", hash, err), n.NodeHash)
		return nil, err
	}

	// ACK 直接作为 gRPC 返回值
	ack := &pb.GossipACK{
		Hash:     hash,
		FromHash: n.NodeHash,
	}

	if isNew {
		internalMsg := pbToInternal(message)
		// 中间需要做存储的调用方法存储
		go n.broadcastToTargets(internalMsg) // 异步广播
		n.Logger.Log(fmt.Sprintf("[INFO] NEW MESSAGE RECEIVED: %s (from %s)", hash, fromHash), n.NodeHash)
	} else {
		n.Logger.Log(fmt.Sprintf("[DEBUG] Repeated message received: %s (from %s). ACK returned.", hash, fromHash), n.NodeHash)
	}

	return ack, nil
}

func (n *GossipNode) broadcastToTargets(msg MessageManage.GossipMessage[[]byte]) {
	sentCount := int64(0)              // 统计已发送数量
	fanout := n.Fanout                 // 最大广播数
	neighborsKeys := keys(n.Neighbors) // 获取所有邻居哈希列表

	for _, targetHash := range neighborsKeys {
		// 已经发过就跳过
		if n.Storage.GetState(msg.Hash, targetHash) {
			continue
		}

		// 超过 fanout 限制就停止
		if sentCount >= fanout {
			break
		}

		endpoint := n.Neighbors[targetHash]
		client, err := n.clientFactory.GetClient(targetHash, endpoint)
		if err != nil {
			n.Logger.Log(fmt.Sprintf("[ERROR] Failed to get client for %s: %v", targetHash, err), n.NodeHash)
			continue
		}

		// 标记将要发送（更新 state map）
		n.Storage.MarkSent(msg.Hash, targetHash)
		sentCount++

		// 异步发送消息
		go func(c pb.GossipClient, m MessageManage.GossipMessage[[]byte]) {
			ctx, cancel := n.clientFactory.ContextWithTimeout()
			defer cancel()

			_, err := c.PutMessageToClient(ctx, &pb.GossipMessage{
				Hash:     m.Hash,
				FromHash: m.FromHash,
				PayLoad:  m.Payload,
			})
			if err != nil {
				n.Logger.Log(fmt.Sprintf("[WARN] Failed to send message %s to %s: %v", m.Hash, targetHash, err), n.NodeHash)
			}
		}(client, msg)
	}
}
