package gossipstreamfactory

import (
	"context"
	"sync"

	"github.com/sukasukasuka123/Gossip/ConnManager"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type NeighborStreamFactory struct {
	slots sync.Map // key=neighborID, value=*ConnManager.NeighborSlot
	ctx   context.Context
}

func NewFactory(ctx context.Context) *NeighborStreamFactory {
	return &NeighborStreamFactory{
		ctx: ctx,
	}
}

// GetOrCreate 获取已存在的 slot 或创建新的
func (f *NeighborStreamFactory) GetOrCreate(
	neighborID string,
	client pb.GossipChunkServiceClient,
) (*ConnManager.NeighborSlot, error) {

	// 先尝试从 sync.Map 中加载
	if slotAny, ok := f.slots.Load(neighborID); ok {
		return slotAny.(*ConnManager.NeighborSlot), nil
	}

	// 创建新的 stream
	stream, err := client.PushChunks(f.ctx)
	if err != nil {
		return nil, err
	}

	// 创建 slot
	slot := ConnManager.NewNeighborSlot(neighborID, stream, 15, f.ctx)
	slot.StartRecvAck()
	slot.StartHeartbeat()

	// 尝试存入 sync.Map，防止竞态
	actual, loaded := f.slots.LoadOrStore(neighborID, slot)
	if loaded {
		// 已有其他协程创建了 slot，则关闭自己创建的
		slot.Close()
		return actual.(*ConnManager.NeighborSlot), nil
	}

	return slot, nil
}

// RemoveSlot 移除失效的 slot
func (f *NeighborStreamFactory) RemoveSlot(neighborID string) {
	if slotAny, ok := f.slots.LoadAndDelete(neighborID); ok {
		slotAny.(*ConnManager.NeighborSlot).Close()
	}
}

// GetSlot 获取现有 slot
func (f *NeighborStreamFactory) GetSlot(neighborID string) (*ConnManager.NeighborSlot, bool) {
	slotAny, ok := f.slots.Load(neighborID)
	if !ok {
		return nil, false
	}
	return slotAny.(*ConnManager.NeighborSlot), true
}
