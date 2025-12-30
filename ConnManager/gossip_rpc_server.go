// ConnManager/gossip_chunk_server.go
package ConnManager

import (
	"io"
	"log"

	"github.com/sukasukasuka123/Gossip/MessageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type GossipChunkServer struct {
	pb.UnimplementedGossipChunkServiceServer

	NodeHash       string
	messageManager *MessageManage.MessageManager
	handler        MessageManage.MessageHandler // 消息完成后的回调
}

// NewGossipChunkServer 创建 gRPC 服务端
func NewGossipChunkServer(
	nodeHash string,
	messageManager *MessageManage.MessageManager,
	handler MessageManage.MessageHandler,
) *GossipChunkServer {
	return &GossipChunkServer{
		NodeHash:       nodeHash,
		messageManager: messageManager,
		handler:        handler,
	}
}

// PushChunks 实现 gRPC 流式接收
func (s *GossipChunkServer) PushChunks(
	stream pb.GossipChunkService_PushChunksServer,
) error {
	for {
		// 1. 接收 chunk
		chunk, err := stream.Recv()
		if err == io.EOF {
			// 客户端主动关闭流
			return nil
		}
		if err != nil {
			log.Printf("[Server] 接收 chunk 错误: %v", err)
			return err
		}

		// 2. 处理 chunk（包含去重、重组逻辑）
		complete, payload, isDuplicate := s.messageManager.HandleIncomingChunk(chunk)

		// 3. 根据处理结果返回不同的 ACK
		if isDuplicate {
			// 消息已经完整接收过，返回重复 ACK
			stream.Send(&pb.GossipChunkAck{
				PayloadHash: chunk.PayloadHash,
				RecvHash:    s.NodeHash,
				SessionID:   chunk.SessionID,
				ChunkIndex:  chunk.ChunkIndex,
				Status:      pb.AckStatus_ACK_DUPLICATE,
			})
			continue
		}

		if !complete {
			// Chunk 接收成功，但消息还未完成
			stream.Send(&pb.GossipChunkAck{
				PayloadHash: chunk.PayloadHash,
				RecvHash:    s.NodeHash,
				SessionID:   chunk.SessionID,
				ChunkIndex:  chunk.ChunkIndex,
				Status:      pb.AckStatus_ACK_OK,
			})
		} else {
			// 4. 消息接收完成
			log.Printf("[Server] 消息接收完成: hash=%s, sender=%s, size=%d bytes",
				chunk.PayloadHash, chunk.SenderHash, len(payload))

			// 返回完成 ACK
			stream.Send(&pb.GossipChunkAck{
				PayloadHash: chunk.PayloadHash,
				RecvHash:    s.NodeHash,
				SessionID:   chunk.SessionID,
				ChunkIndex:  chunk.ChunkIndex,
				Status:      pb.AckStatus_ACK_COMPLETE,
			})

			// 5. 异步触发消息完成回调（不阻塞接收流程）
			if s.handler != nil {
				go s.handler.OnMessageComplete(
					chunk.PayloadHash,
					payload,
					chunk.SenderHash,
				)
			}
		}
	}
}
