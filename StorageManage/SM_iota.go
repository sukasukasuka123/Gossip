package StorageManage

type StorageOp int

const (
	OpInitState    StorageOp = iota // 初始化一条消息的状态
	OpUpdateState                   // 更新某邻居状态（收到ACK）
	OpMarkSent                      // 标记已发送给某邻居
	OpDeleteState                   // 删除该 hash 的所有状态
	OpGetState                      // 查询某邻居的状态
	OpGetAllStates                  // 获取该消息的完整状态表
)

// Payload 用于封装不同操作需要的参数
type Payload struct {
	Hash      string
	Neighbors []string
	NodeHash  string
}

// IStorage 定义统一接口，方便热插拔
type IStorage interface {
	InitState(hash string, neighbors []string) (bool, error)
	UpdateState(hash string, from string) error
	GetStates(hash string) map[string]bool
	GetState(hash, nodeHash string) bool
	MarkSent(hash, nodeHash string)
	DeleteState(hash string)
}
