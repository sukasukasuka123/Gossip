package MessageManage

// 这是消息管理模块，负责将大消息切分为多个chunk，
// 并通过消息队列管理这些chunk的发送，确保消息的完整性和顺序性

import pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"

type Chunker struct {
	chunkSize int
}

func NewChunker(size int) *Chunker {
	return &Chunker{chunkSize: size}
}

func (c *Chunker) Split(
	payload []byte,
	payloadHash string,
	sessionID int32,
	sender string,
) []*pb.GossipChunk {

	var chunks []*pb.GossipChunk
	total := (len(payload) + c.chunkSize - 1) / c.chunkSize

	for i := 0; i < total; i++ {
		start := i * c.chunkSize
		end := start + c.chunkSize
		if end > len(payload) {
			end = len(payload)
		}

		chunks = append(chunks, &pb.GossipChunk{
			PayloadHash: payloadHash,
			SenderHash:  sender,
			SessionID:   sessionID,
			ChunkIndex:  int32(i),
			TotalChunks: int32(total),
			ChunkData:   payload[start:end],
		})
	}
	return chunks
}
