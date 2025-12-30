// NeighborManage/neighbor_manager.go
package NeighborManage

import (
	"sort"
	"sync"
	"time"

	"github.com/sukasukasuka123/Gossip/ConnManager"
)

type NeighborNode struct {
	NodeHash string
	Address  string
	Port     int
	Ping     time.Duration
}

type NeighborManager struct {
	mu            sync.RWMutex
	neighbors     map[string]*NeighborNode
	neighborsConn map[string]*ConnManager.NeighborSlot
	neighborState map[string]time.Time

	//  新增：fanout 管理器
	fanoutManager *FanoutManager
}

// NewNeighborManager 创建邻居管理器
func NewNeighborManager(fanoutStrategy FanoutStrategy, fanoutCount int) *NeighborManager {
	return &NeighborManager{
		neighbors:     make(map[string]*NeighborNode),
		neighborsConn: make(map[string]*ConnManager.NeighborSlot),
		neighborState: make(map[string]time.Time),
		fanoutManager: NewFanoutManager(fanoutStrategy, fanoutCount, 10*time.Minute),
	}
}

// AddNeighbor 添加邻居节点
func (nm *NeighborManager) AddNeighbor(node *NeighborNode, slot *ConnManager.NeighborSlot) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.neighbors[node.NodeHash] = node
	nm.neighborsConn[node.NodeHash] = slot
	nm.neighborState[node.NodeHash] = time.Now()
}

// RemoveNeighbor 移除邻居节点
func (nm *NeighborManager) RemoveNeighbor(nodeHash string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	delete(nm.neighbors, nodeHash)
	delete(nm.neighborsConn, nodeHash)
	delete(nm.neighborState, nodeHash)
}

// GetNeighbor 获取单个邻居节点
func (nm *NeighborManager) GetNeighbor(nodeHash string) (*NeighborNode, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.neighbors[nodeHash]
	return node, exists
}

// GetAllNeighbors 获取所有邻居节点
func (nm *NeighborManager) GetAllNeighbors() []*NeighborNode {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NeighborNode, 0, len(nm.neighbors))
	for _, node := range nm.neighbors {
		nodeCopy := &NeighborNode{
			NodeHash: node.NodeHash,
			Address:  node.Address,
			Port:     node.Port,
			Ping:     node.Ping,
		}
		nodes = append(nodes, nodeCopy)
	}
	return nodes
}

// GetNeighborSlot 获取邻居连接槽
func (nm *NeighborManager) GetNeighborSlot(nodeHash string) (*ConnManager.NeighborSlot, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	slot, exists := nm.neighborsConn[nodeHash]
	return slot, exists
}

// GetBestNeighbors 根据延迟排序获取最优的 N 个邻居（用于内部排序）
func (nm *NeighborManager) GetBestNeighbors(count int) []*NeighborNode {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NeighborNode, 0, len(nm.neighbors))
	for _, node := range nm.neighbors {
		nodeCopy := &NeighborNode{
			NodeHash: node.NodeHash,
			Address:  node.Address,
			Port:     node.Port,
			Ping:     node.Ping,
		}
		nodes = append(nodes, nodeCopy)
	}

	// 按 Ping 延迟排序（从低到高）
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Ping < nodes[j].Ping
	})

	if count > len(nodes) {
		count = len(nodes)
	}

	return nodes[:count]
}

// 新增：FanoutMessage 根据策略选择邻居并返回他们的连接槽
func (nm *NeighborManager) FanoutMessage(messageHash string, excludeHash string) []*ConnManager.NeighborSlot {
	// 1. 获取所有邻居并按延迟排序
	allNeighbors := nm.GetBestNeighbors(len(nm.neighbors))

	// 2. 使用 FanoutManager 选择邻居
	selectedNeighbors := nm.fanoutManager.SelectNeighborsForFanout(
		messageHash,
		allNeighbors,
		excludeHash,
	)

	// 3. 获取对应的连接槽
	slots := make([]*ConnManager.NeighborSlot, 0, len(selectedNeighbors))
	nm.mu.RLock()
	for _, neighbor := range selectedNeighbors {
		if slot, exists := nm.neighborsConn[neighbor.NodeHash]; exists {
			slots = append(slots, slot)
		}
	}
	nm.mu.RUnlock()

	return slots
}

// 新增：FanoutMessageWithCallback 带回调的 fanout，可以在发送成功后标记状态
func (nm *NeighborManager) FanoutMessageWithCallback(
	messageHash string,
	excludeHash string,
	sendFunc func(slot *ConnManager.NeighborSlot, neighborHash string) error,
) error {
	// 1. 获取所有邻居并按延迟排序
	allNeighbors := nm.GetBestNeighbors(len(nm.neighbors))

	// 2. 使用 FanoutManager 选择邻居
	selectedNeighbors := nm.fanoutManager.SelectNeighborsForFanout(
		messageHash,
		allNeighbors,
		excludeHash,
	)

	// 3. 发送给每个选中的邻居
	for _, neighbor := range selectedNeighbors {
		slot, exists := nm.GetNeighborSlot(neighbor.NodeHash)
		if !exists {
			continue
		}

		// 执行发送回调
		if err := sendFunc(slot, neighbor.NodeHash); err != nil {
			// 发送失败，可以记录日志
			continue
		}

		// 发送成功，标记状态
		nm.fanoutManager.MarkMessageSent(messageHash, neighbor.NodeHash)
	}

	// 标记消息 fanout 完成
	nm.fanoutManager.MarkMessageComplete(messageHash)

	return nil
}

// 新增：GetMessageFanoutState 获取消息的 fanout 状态
func (nm *NeighborManager) GetMessageFanoutState(messageHash string) (*MessageFanoutState, bool) {
	return nm.fanoutManager.GetMessageState(messageHash)
}

// 新增：HasSentToNeighbor 检查消息是否已发送给某个邻居
func (nm *NeighborManager) HasSentToNeighbor(messageHash, neighborHash string) bool {
	return nm.fanoutManager.HasSentToNeighbor(messageHash, neighborHash)
}

// 新增：GetFanoutStats 获取 fanout 统计信息
func (nm *NeighborManager) GetFanoutStats() map[string]interface{} {
	return nm.fanoutManager.GetStats()
}

// UpdatePing 更新邻居的 Ping 延迟
func (nm *NeighborManager) UpdatePing(nodeHash string, ping time.Duration) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if node, exists := nm.neighbors[nodeHash]; exists {
		node.Ping = ping
		nm.neighborState[nodeHash] = time.Now()
	}
}

// GetNeighborState 获取邻居的最后活跃时间
func (nm *NeighborManager) GetNeighborState(nodeHash string) (time.Time, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	state, exists := nm.neighborState[nodeHash]
	return state, exists
}

// ActiveNeighborCount 获取活跃邻居数量
func (nm *NeighborManager) ActiveNeighborCount() int {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	return len(nm.neighbors)
}

// HasNeighbor 检查邻居是否存在
func (nm *NeighborManager) HasNeighbor(nodeHash string) bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	_, exists := nm.neighbors[nodeHash]
	return exists
}

// GetNeighborsByPingThreshold 获取延迟低于阈值的邻居
func (nm *NeighborManager) GetNeighborsByPingThreshold(threshold time.Duration) []*NeighborNode {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NeighborNode, 0)
	for _, node := range nm.neighbors {
		if node.Ping <= threshold {
			nodeCopy := &NeighborNode{
				NodeHash: node.NodeHash,
				Address:  node.Address,
				Port:     node.Port,
				Ping:     node.Ping,
			}
			nodes = append(nodes, nodeCopy)
		}
	}

	return nodes
}

// GetStaleNeighbors 获取长时间未活跃的邻居
func (nm *NeighborManager) GetStaleNeighbors(staleDuration time.Duration) []string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	now := time.Now()
	staleNodes := make([]string, 0)

	for nodeHash, lastActive := range nm.neighborState {
		if now.Sub(lastActive) > staleDuration {
			staleNodes = append(staleNodes, nodeHash)
		}
	}

	return staleNodes
}

// CleanupStaleNeighbors 清理长时间未活跃的邻居
func (nm *NeighborManager) CleanupStaleNeighbors(staleDuration time.Duration) int {
	staleNodes := nm.GetStaleNeighbors(staleDuration)

	for _, nodeHash := range staleNodes {
		nm.RemoveNeighbor(nodeHash)
	}

	return len(staleNodes)
}

// GetNeighborList 获取邻居哈希列表
func (nm *NeighborManager) GetNeighborList() []string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	list := make([]string, 0, len(nm.neighbors))
	for nodeHash := range nm.neighbors {
		list = append(list, nodeHash)
	}

	return list
}

// GetNeighborInfo 获取邻居的详细信息（包括状态）
func (nm *NeighborManager) GetNeighborInfo(nodeHash string) (node *NeighborNode, lastActive time.Time, exists bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	n, nodeExists := nm.neighbors[nodeHash]
	if !nodeExists {
		return nil, time.Time{}, false
	}

	nodeCopy := &NeighborNode{
		NodeHash: n.NodeHash,
		Address:  n.Address,
		Port:     n.Port,
		Ping:     n.Ping,
	}

	lastActive, _ = nm.neighborState[nodeHash]
	return nodeCopy, lastActive, true
}
