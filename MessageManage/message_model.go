package MessageManage

type GossipMessage[T any] struct {
	Hash    string `json:"hash"`
	Payload T      `json:"payload"`
}

type GossipAck struct {
	Hash     string `json:"hash"`
	FromHash string `json:"from_id"`
}
