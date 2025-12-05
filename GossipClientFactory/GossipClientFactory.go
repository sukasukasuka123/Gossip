package GossipClientFactory

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"

	"google.golang.org/grpc"
)

type GossipClientFactory struct {
	clients map[string]pb.GossipClient
	conns   map[string]*grpc.ClientConn
	lock    sync.Mutex
	timeout time.Duration
}

func NewGossipClientFactory(timeout time.Duration) *GossipClientFactory {
	return &GossipClientFactory{
		clients: make(map[string]pb.GossipClient),
		conns:   make(map[string]*grpc.ClientConn),
		timeout: timeout,
	}
}

func (f *GossipClientFactory) GetClient(nodeHash, endpoint string) (pb.GossipClient, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if client, ok := f.clients[nodeHash]; ok {
		return client, nil
	}

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", endpoint, err)
	}

	client := pb.NewGossipClient(conn)
	f.clients[nodeHash] = client
	f.conns[nodeHash] = conn
	return client, nil
}

func (f *GossipClientFactory) Release() {
	f.lock.Lock()
	defer f.lock.Unlock()
	for nodeHash, conn := range f.conns {
		_ = conn.Close()
		delete(f.conns, nodeHash)
		delete(f.clients, nodeHash)
	}
}

func (f *GossipClientFactory) ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), f.timeout)
}
