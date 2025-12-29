package MessageManage

// 这是消息存储模块，负责消息的热缓存和冷存储管理

type MessageCache interface {
	Put(hash string, data []byte)
	Get(hash string) ([]byte, bool)
	Evict(hash string)
}

type InMemoryCache struct {
	store map[string][]byte
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{store: make(map[string][]byte)}
}
