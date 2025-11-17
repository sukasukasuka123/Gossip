package NodeManage

import (
	message "Gossip/MessageManage"
	"context"
	"encoding/json"
	"fmt"
)

func (n *GossipNode[T]) sendAckToSenders(hash string, toNodeHash string) {
	endpoint, ok := n.Neighbors[toNodeHash]
	if !ok {
		logText := fmt.Sprintf("[WARN] Cannot send ACK for hash %s: unknown sender hash %s", hash, toNodeHash)
		n.Logger.Log(logText, n.NodeHash)
		return
	}

	ack := message.GossipAck{Hash: hash, FromHash: n.NodeHash}
	data, _ := json.Marshal(ack)

	// 异步发送 ACK
	go n.Transport.SendAck(context.Background(), endpoint, data)
}

func (n *GossipNode[T]) broadcastToTargets(msg message.GossipMessage[T]) {
	// ！！！关键修改：告诉下游，这条消息是 *我* 发的
	msg.FromHash = n.NodeHash

	data, _ := json.Marshal(msg)

	state := n.Storage.GetStates(msg.Hash)
	if state == nil {
		logText := fmt.Sprintf("[ERROR] Cannot broadcast hash %s: state is nil", msg.Hash)
		n.Logger.Log(logText, n.NodeHash)
		return
	}

	targets := n.Router.SelectTargets(n.Cost, state)
	sentCount := int64(0)

	for _, t := range targets {
		// 已经发过就跳过 (GetState 会处理 state[t] 不存在的情况)
		if n.Storage.GetState(msg.Hash, t) {
			continue
		}
		// 超过 fanout 就不继续发了
		if sentCount >= n.Fanout {
			break
		}
		sentCount++

		// 标记将要发送 (这会更新 state map)
		n.Storage.MarkSent(msg.Hash, t)

		// 异步发送
		go n.Transport.SendMessage(context.Background(), n.Neighbors[t], data)
	}
}
