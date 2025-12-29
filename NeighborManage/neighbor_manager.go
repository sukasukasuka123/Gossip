package NeighborManage

import (
	"sort"
	"sync"
	"time"

	"github.com/sukasukasuka123/Gossip/ConnManager"
)

// 这是邻居管理模块，负责维护邻居节点的信息和状态
// 主要是负责fanout选出合适的邻居节点进行消息传播

type NeighborNode struct {
	NodeHash string        // 节点哈希
	Address  string        // 节点地址
	Port     int           // 节点端口
	Ping     time.Duration // 最后一次ping的延迟（ Duration 类型，表示延迟）
}
type NeighborManager struct {
	mu            sync.RWMutex
	neighbors     map[string]*NeighborNode             // 邻居节点哈希 -> 邻居节点信息
	neighborsConn map[string]*ConnManager.NeighborSlot // 邻居节点哈希 -> 连接槽
	neighborState map[string]time.Time                 // 邻居节点哈希 -> 最后一次活跃时间
}

func NewNeighborManager() *NeighborManager {
	return &NeighborManager{
		neighbors:     make(map[string]*NeighborNode),
		neighborsConn: make(map[string]*ConnManager.NeighborSlot),
		neighborState: make(map[string]time.Time),
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
		// 创建副本以避免外部修改
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

// GetBestNeighbors 根据延迟排序获取最优的 N 个邻居
func (nm *NeighborManager) GetBestNeighbors(count int) []*NeighborNode {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// 获取所有邻居
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

	// 返回前 N 个
	if count > len(nodes) {
		count = len(nodes)
	}

	return nodes[:count]
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

	// 返回副本
	nodeCopy := &NeighborNode{
		NodeHash: n.NodeHash,
		Address:  n.Address,
		Port:     n.Port,
		Ping:     n.Ping,
	}

	lastActive, _ = nm.neighborState[nodeHash]
	return nodeCopy, lastActive, true
}
