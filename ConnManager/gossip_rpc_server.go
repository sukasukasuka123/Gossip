package ConnManager

import (
	"io"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type GossipChunkServer struct {
	NodeHash     string
	NeighborHash string
}

// 这是proto对应的服务端代码，负责实现pushchunks rpc方法
func (s *GossipChunkServer) PushChunks(
	stream pb.GossipChunkService_PushChunksServer,
) error {

	// 内部状态结构：跟踪每条消息的接收进度
	type MessageState struct {
		totalChunks int32
		received    map[int32]bool
	}

	messages := make(map[string]*MessageState)

	for {
		// 1. 接收 chunk
		chunk, err := stream.Recv()
		if err == io.EOF {
			// 客户端主动关闭流
			return nil
		}
		if err != nil {
			return err
		}

		key := chunk.PayloadHash

		// 2. 初始化消息状态
		state, ok := messages[key]
		if !ok {
			state = &MessageState{
				totalChunks: chunk.TotalChunks,
				received:    make(map[int32]bool),
			}
			messages[key] = state
		}

		// 3. 去重判断
		if state.received[chunk.ChunkIndex] {
			stream.Send(&pb.GossipChunkAck{
				PayloadHash: chunk.PayloadHash,
				RecvHash:    s.NodeHash,
				SessionID:   chunk.SessionID,
				ChunkIndex:  chunk.ChunkIndex,
				Status:      pb.AckStatus_ACK_DUPLICATE,
			})
			continue
		}

		// 4. 记录接收
		state.received[chunk.ChunkIndex] = true

		// 5. 返回 ACK
		stream.Send(&pb.GossipChunkAck{
			PayloadHash: chunk.PayloadHash,
			RecvHash:    "node2",
			SessionID:   chunk.SessionID,
			ChunkIndex:  chunk.ChunkIndex,
			Status:      pb.AckStatus_ACK_OK,
		})

		// 6. 判断是否完成
		if int32(len(state.received)) == state.totalChunks {
			stream.Send(&pb.GossipChunkAck{
				PayloadHash: chunk.PayloadHash,
				RecvHash:    "node2",
				SessionID:   chunk.SessionID,
				ChunkIndex:  -1,
				Status:      pb.AckStatus_ACK_COMPLETE,
			})

			delete(messages, key) // 清理状态
		}
	}
}
