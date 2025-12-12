package NeighborManage

import (
	"sync"
)

// NeighborInfo 保存邻居节点的基础信息
type NeighborInfo struct {
	NodeHash string
	Endpoint string
	Online   bool
	// 需要扩展可用于存储负载均衡相关的信息
}

// NeighborStore 抽象存储接口，可热插拔
type NeighborStore interface {
	Add(neighbor NeighborInfo) error
	Remove(nodeHash string) error
	Update(neighbor NeighborInfo) error
	Get(nodeHash string) (NeighborInfo, bool)
	List() []NeighborInfo
}

// 内存存储实现
type MemoryNeighborStore struct {
	mu        sync.RWMutex
	neighbors map[string]NeighborInfo // nodeHash -> NeighborInfo
}

func NewMemoryNeighborStore() *MemoryNeighborStore {
	return &MemoryNeighborStore{
		neighbors: make(map[string]NeighborInfo),
	}
}

func (s *MemoryNeighborStore) Add(n NeighborInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.neighbors[n.NodeHash] = n
	return nil
}

func (s *MemoryNeighborStore) Remove(nodeHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.neighbors, nodeHash)
	return nil
}

func (s *MemoryNeighborStore) Update(n NeighborInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.neighbors[n.NodeHash]; !ok {
		return nil
	}
	s.neighbors[n.NodeHash] = n
	return nil
}

func (s *MemoryNeighborStore) Get(nodeHash string) (NeighborInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.neighbors[nodeHash]
	return n, ok
}

func (s *MemoryNeighborStore) List() []NeighborInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]NeighborInfo, 0, len(s.neighbors))
	for _, n := range s.neighbors {
		list = append(list, n)
	}
	return list
}
