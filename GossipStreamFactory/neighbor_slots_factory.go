package gossipstreamfactory

// 这是管理邻居stream连接的工厂，用于产出neighbor slots,
// 同时需要做的是维护这些slots的生命周期和每个连接的创建上协程心跳机制
// 以及通过心跳更新每个节点的连接状态

import (
	"context"
	"sync"

	"github.com/sukasukasuka123/Gossip/ConnManager"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type NeighborStreamFactory struct {
	mu    sync.Mutex
	slots map[string]*ConnManager.NeighborSlot // neighborID -> slot
}

func NewFactory() *NeighborStreamFactory {
	return &NeighborStreamFactory{
		slots: make(map[string]*ConnManager.NeighborSlot),
	}
}

func (f *NeighborStreamFactory) GetOrCreate(
	neighborID string,
	client pb.GossipChunkServiceClient,
) (*ConnManager.NeighborSlot, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	if slot, ok := f.slots[neighborID]; ok {
		return slot, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.PushChunks(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	slot := ConnManager.NewNeighborSlot(neighborID, stream, cancel)
	f.slots[neighborID] = slot
	slot.StartHeartbeat()

	return slot, nil
}
