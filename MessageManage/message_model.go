package MessageManage

// T 是消息体 (Payload)
type GossipMessage[T any] struct {
	Hash     string `json:"hash"`      // 消息的哈希值
	FromHash string `json:"from_hash"` // 消息来自哪个节点
	Payload  T      `json:"payload"`
}

type GossipAck struct {
	Hash     string `json:"hash"`      // ACK 对应的消息哈希值
	FromHash string `json:"from_hash"` // ACK 来自哪个节点
}
