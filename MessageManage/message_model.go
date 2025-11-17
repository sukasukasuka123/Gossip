package MessageManage

// T 是消息体 (Payload)
type GossipMessage[T any] struct {
	Hash     string `json:"hash"`
	FromHash string `json:"from_hash"` // 关键新增：消息来自哪个节点
	Payload  T      `json:"payload"`
}

type GossipAck struct {
	Hash     string `json:"hash"`
	FromHash string `json:"from_hash"` // ACK 来自哪个节点
}
