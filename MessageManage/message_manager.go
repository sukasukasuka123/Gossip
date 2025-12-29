package MessageManage

// 这是消息管理模块，负责调用消息分块、重组、缓存等功能
import (
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type MessageManager struct {
	chunker     *Chunker
	reassembler *Reassembler
	cache       MessageCache
}

func NewMessageManager(
	chunker *Chunker,
	reassembler *Reassembler,
	cache MessageCache,
) *MessageManager {
	return &MessageManager{
		chunker:     chunker,
		reassembler: reassembler,
		cache:       cache,
	}
}

func (m *MessageManager) HandleIncomingChunk(chunk *pb.GossipChunk) (complete bool, payload []byte) {
	return m.reassembler.AddChunk(
		chunk.PayloadHash,
		chunk.ChunkIndex,
		chunk.TotalChunks,
		chunk.ChunkData,
	)
}
