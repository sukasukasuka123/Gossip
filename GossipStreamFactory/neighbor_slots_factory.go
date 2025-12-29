// gossipstreamfactory/factory.go
package gossipstreamfactory

import (
	"context"
	"sync"

	"github.com/sukasukasuka123/Gossip/ConnManager"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type NeighborStreamFactory struct {
	mu    sync.Mutex
	slots map[string]*ConnManager.NeighborSlot
	ctx   context.Context
}

func NewFactory(ctx context.Context) *NeighborStreamFactory {
	return &NeighborStreamFactory{
		slots: make(map[string]*ConnManager.NeighborSlot),
		ctx:   ctx,
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

	stream, err := client.PushChunks(f.ctx)
	if err != nil {
		return nil, err
	}

	// ✅ NewNeighborSlot 内部会启动滑动窗口协程
	slot := ConnManager.NewNeighborSlot(neighborID, stream, 15, f.ctx)
	f.slots[neighborID] = slot

	// ✅ 启动 ACK 接收和心跳
	slot.StartRecvAck()
	slot.StartHeartbeat()

	return slot, nil
}

// RemoveSlot 移除失效的 slot
func (f *NeighborStreamFactory) RemoveSlot(neighborID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if slot, ok := f.slots[neighborID]; ok {
		slot.Close()
		delete(f.slots, neighborID)
	}
}

// GetSlot 获取现有 slot
func (f *NeighborStreamFactory) GetSlot(neighborID string) (*ConnManager.NeighborSlot, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	slot, ok := f.slots[neighborID]
	return slot, ok
}
