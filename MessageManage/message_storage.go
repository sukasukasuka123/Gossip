// MessageManage/cache.go
package MessageManage

import "sync"

type MessageCache interface {
	Put(hash string, data []byte)
	Get(hash string) ([]byte, bool)
	Evict(hash string)
}

type InMemoryCache struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store: make(map[string][]byte),
	}
}

func (c *InMemoryCache) Put(hash string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 存储副本，避免外部修改
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	c.store[hash] = dataCopy
}

func (c *InMemoryCache) Get(hash string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.store[hash]
	if !exists {
		return nil, false
	}

	// 返回副本，避免外部修改
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy, true
}

func (c *InMemoryCache) Evict(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, hash)
}

func (c *InMemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

func (c *InMemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string][]byte)
}
