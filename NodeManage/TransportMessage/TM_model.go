package TransportMessage

import (
	"context"
)

// 消息发送接口（http / grpc / ws 均可实现）
type Transport interface {
	SendMessage(ctx context.Context, endpoint string, data []byte) error
	SendAck(ctx context.Context, endpoint string, data []byte) error
}
