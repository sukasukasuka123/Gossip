// MessageManage/message_manager.go
package MessageManage

import (
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type MessageManager struct {
	chunker      *Chunker
	reassembler  *Reassembler
	cache        MessageCache
	deduplicator *MessageDeduplicator
}

// MessageHandler 消息接收完成的回调接口
type MessageHandler interface {
	OnMessageComplete(payloadHash string, payload []byte, senderHash string)
}

func NewMessageManager(
	chunker *Chunker,
	reassembler *Reassembler,
	cache MessageCache,
	deduplicator *MessageDeduplicator,
) *MessageManager {
	return &MessageManager{
		chunker:      chunker,
		reassembler:  reassembler,
		cache:        cache,
		deduplicator: deduplicator,
	}
}

// HandleIncomingChunk 处理传入的 chunk
// 返回：是否完成重组，完整的 payload，是否是重复消息
func (m *MessageManager) HandleIncomingChunk(chunk *pb.GossipChunk) (complete bool, payload []byte, isDuplicate bool) {
	// 1. 消息级别去重检查
	if m.deduplicator.IsDuplicate(chunk.PayloadHash) {
		return false, nil, true
	}

	// 2. 重组 chunk
	complete, payload = m.reassembler.AddChunk(
		chunk.PayloadHash,
		chunk.ChunkIndex,
		chunk.TotalChunks,
		chunk.ChunkData,
	)

	// 3. 如果消息完成，标记为已见并缓存
	if complete {
		m.deduplicator.MarkAsSeen(chunk.PayloadHash)
		m.cache.Put(chunk.PayloadHash, payload)
	}

	return complete, payload, false
}

// GetChunker 获取分块器
func (m *MessageManager) GetChunker() *Chunker {
	return m.chunker
}

// GetReassembler 获取重组器
func (m *MessageManager) GetReassembler() *Reassembler {
	return m.reassembler
}

// GetCache 获取缓存
func (m *MessageManager) GetCache() MessageCache {
	return m.cache
}

// GetDeduplicator 获取去重器
func (m *MessageManager) GetDeduplicator() *MessageDeduplicator {
	return m.deduplicator
}

// IsMessageSeen 检查消息是否已处理
func (m *MessageManager) IsMessageSeen(payloadHash string) bool {
	return m.deduplicator.IsDuplicate(payloadHash)
}
