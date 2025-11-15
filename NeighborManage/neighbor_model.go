package NeighborManage

type Neighbor struct {
	NodeHash string `json:"nodehash"` // 邻居节点的哈希值
	Endpoint string `json:"endpoint"` // 邻居节点的端点地址
}

var NeighborMap map[string]Neighbor // 邻居哈希值——>邻居信息
