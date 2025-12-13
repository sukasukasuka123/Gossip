package NeighborManage

import (
	"sync"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
)

type NeighborManager struct {
	SelfHash string
	NStore   NeighborStore
	Factory  *GossipStreamFactory.DoubleStreamFactory

	slots sync.Map // nodeHash -> *NeighborSlot
}

// 创建解耦模式 NeighborManager
func NewNeighborManager(selfHash string, store NeighborStore, factory *GossipStreamFactory.DoubleStreamFactory) *NeighborManager {
	return &NeighborManager{
		SelfHash: selfHash,
		NStore:   store,
		Factory:  factory,
	}
}

// Connect 建立连接（线程安全）
func (m *NeighborManager) Connect(neighbor NeighborInfo) (*NeighborSlot, error) {
	// 先尝试 Load，避免重复创建
	if slot, ok := m.slots.Load(neighbor.NodeHash); ok {
		return slot.(*NeighborSlot), nil
	}

	// 创建新的双流
	raw, err := m.Factory.GetDoubleStream(neighbor.NodeHash, neighbor.Endpoint)
	if err != nil {
		return nil, err
	}

	slot := NewNeighborSlot(neighbor.NodeHash, raw)

	// 原子存储，如果已有则返回已有的
	actual, loaded := m.slots.LoadOrStore(neighbor.NodeHash, slot)
	if loaded {
		// 已经存在
		slot.Close() // 关闭自己创建的冗余 slot
		return actual.(*NeighborSlot), nil
	}

	return slot, nil
}

// Disconnect 主动断开
func (m *NeighborManager) Disconnect(nodeHash string) {
	if slot, ok := m.slots.Load(nodeHash); ok {
		slot.(*NeighborSlot).Close()
		m.slots.Delete(nodeHash)
	}
}

// 获取邻居的双流 Slot（线程安全）
func (m *NeighborManager) GetSlot(nodeHash string) (*NeighborSlot, bool) {
	slot, ok := m.slots.Load(nodeHash)
	if !ok {
		return nil, false
	}
	return slot.(*NeighborSlot), true
}

// 遍历所有在线邻居
func (m *NeighborManager) RangeOnlineNeighbors(f func(n NeighborInfo, slot *NeighborSlot)) {
	for _, n := range m.NStore.List() {
		if slot, ok := m.slots.Load(n.NodeHash); ok {
			f(n, slot.(*NeighborSlot))
		}
	}
}
