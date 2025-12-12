package NeighborManage

import (
	"sync"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
)

type NeighborManager struct {
	SelfHash string                                   //方便传参
	NStore   NeighborStore                            //方法封装
	Factory  *GossipStreamFactory.DoubleStreamFactory // 流工厂(用于创造双流)

	mu    sync.RWMutex
	slots map[string]*NeighborSlot // nodeHash -> slot
}

// 创建解耦模式 NeighborManager
func NewNeighborManager(selfHash string, store NeighborStore, factory *GossipStreamFactory.DoubleStreamFactory) *NeighborManager {
	return &NeighborManager{
		SelfHash: selfHash,
		NStore:   store,
		Factory:  factory,
		slots:    make(map[string]*NeighborSlot),
	}
}

// Connect 建立连接（线程安全）
func (m *NeighborManager) Connect(neighbor NeighborInfo) (*NeighborSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 已存在
	if slot, ok := m.slots[neighbor.NodeHash]; ok {
		return slot, nil
	}

	raw, err := m.Factory.GetDoubleStream(neighbor.NodeHash, neighbor.Endpoint)
	if err != nil {
		return nil, err
	}

	slot := NewNeighborSlot(neighbor.NodeHash, raw)
	m.slots[neighbor.NodeHash] = slot
	return slot, nil
}

// Disconnect 主动断开
func (m *NeighborManager) Disconnect(nodeHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if slot, ok := m.slots[nodeHash]; ok {
		slot.Close()
		delete(m.slots, nodeHash)
	}
}

// 获取邻居的双流 Slot（线程安全）
func (m *NeighborManager) GetSlot(nodeHash string) (*NeighborSlot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	slot, ok := m.slots[nodeHash]
	return slot, ok
}

// 遍历所有在线邻居（存在意义存疑，暂时保留方法）
func (m *NeighborManager) RangeOnlineNeighbors(f func(n NeighborInfo, slot *NeighborSlot)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, n := range m.NStore.List() {
		slot, ok := m.slots[n.NodeHash]
		if ok {
			f(n, slot)
		}
	}
}
