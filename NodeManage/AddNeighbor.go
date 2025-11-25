package NodeManage

import "fmt"

func (node *GossipNode[T]) AddNeighbor(nodeHash string, endpoint string) {
	node.Neighbors[nodeHash] = endpoint
	node.Cost[nodeHash] = 1.0
	nowSTTL := node.Storage.UpdateShortLimit(len(node.Neighbors))
	node.Logger.Log(fmt.Sprintf("[INFO] Short TTL updated to %d seconds.", nowSTTL), node.NodeHash)
}
