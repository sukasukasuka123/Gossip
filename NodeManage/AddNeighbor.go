package NodeManage

func (node *GossipNode[T]) AddNeighbor(nodeHash string, endpoint string) {
	node.Neighbors[nodeHash] = endpoint
	node.Cost[nodeHash] = 1.0
}
