package Storage

type Storage interface {
	// InitState 原子地初始化一个消息的状态。
	// 如果该 hash 是第一次被初始化，返回 isNew=true。
	// 如果该 hash 已经存在，返回 isNew=false。
	InitState(hash string, neighbors []string) (isNew bool, err error)

	// UpdateState 更新一个节点的状态（例如，收到了 ACK）
	UpdateState(hash string, from string) error

	// GetStates 获取该消息的所有邻居状态
	GetStates(hash string) map[string]bool

	// GetState 获取特定邻居的状态
	GetState(hash, nodeHash string) bool

	// MarkSent 标记（当前节点）已向某个邻居发送了消息
	MarkSent(hash, nodeHash string)

	// DeleteState 当所有邻居都确认后，删除状态
	DeleteState(hash string)

	cleanupSeenCacheLoop()
	UpdateShortLimit(a int) int64
	RecieveAck()
}
