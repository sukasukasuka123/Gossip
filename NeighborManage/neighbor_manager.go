package NeighborManage

import (
	"time"

	"github.com/sukasukasuka123/Gossip/ConnManager"
)

// 这是邻居管理模块，负责维护邻居节点的信息和状态
// 主要是负责fanout选出合适的邻居节点进行消息传播

type NeighborNode struct {
	NodeHash string    // 节点哈希
	Address  string    // 节点地址
	Port     int       // 节点端口
	Ping     time.Time // 最后一次ping的延迟（选出延迟最低的且无传输记录的邻居）
}
type NeighborManager struct {
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

func (nm *NeighborManager) AddNeighbor(node *NeighborNode, slot *ConnManager.NeighborSlot) {
	nm.neighbors[node.NodeHash] = node
	nm.neighborsConn[node.NodeHash] = slot
	// 这里需要截取ping出来的延迟作为状态，现在这里暂时用1来占位
	nm.neighborState[node.NodeHash] = time.Now() // 初始状态为活跃
}

func (nm *NeighborManager) RemoveNeighbor(nodeHash string) {
	delete(nm.neighbors, nodeHash)
	delete(nm.neighborsConn, nodeHash)
	delete(nm.neighborState, nodeHash)
}

func (nm *NeighborManager) GetNeighbor(nodeHash string) (*NeighborNode, bool) {
	node, exists := nm.neighbors[nodeHash]
	return node, exists
}
