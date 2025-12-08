package NodeManage

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	"github.com/sukasukasuka123/Gossip/NodeManage/Logger"
	"github.com/sukasukasuka123/Gossip/NodeManage/Router"
	"github.com/sukasukasuka123/Gossip/NodeManage/Storage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type GossipNode struct {
	NodeHash      string
	Neighbors     map[string]string
	Cost          map[string]float64
	Fanout        int64
	Storage       Storage.Storage
	Router        Router.Router
	Logger        Logger.Logger
	StreamFactory *GossipStreamFactory.StreamFactory
	grpcServer    *grpc.Server

	pb.UnimplementedGossipServer
}

func NewGossipNode(
	id string,
	store Storage.Storage,
	router Router.Router,
	log Logger.Logger,
	timeout time.Duration,
) *GossipNode {

	streamFactory := GossipStreamFactory.NewStreamFactory(timeout)

	return &GossipNode{
		NodeHash:      id,
		Neighbors:     map[string]string{},
		Cost:          map[string]float64{},
		Fanout:        3,
		Logger:        log,
		Storage:       store,
		Router:        router,
		StreamFactory: streamFactory,
	}
}
func (n *GossipNode) StartGRPCServer(endpoint string) {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("[%s] failed to listen on %s: %v", n.NodeHash, endpoint, err)
	}

	// --- gRPC 配置（推荐） ---
	ka := keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
		MaxConnectionAge:  15 * time.Minute,
		Time:              2 * time.Minute,
		Timeout:           20 * time.Second,
	}

	n.grpcServer = grpc.NewServer(
		grpc.KeepaliveParams(ka),
	)

	pb.RegisterGossipServer(n.grpcServer, n)

	go func() {
		fmt.Printf("[%s] gRPC server is listening at %s\n", n.NodeHash, endpoint)
		if err := n.grpcServer.Serve(lis); err != nil {
			log.Fatalf("[%s] failed to serve gRPC: %v", n.NodeHash, err)
		}
	}()
}
func (n *GossipNode) StopGRPC() {
	if n.grpcServer != nil {
		fmt.Printf("[%s] shutting down gRPC server...\n", n.NodeHash)
		n.grpcServer.GracefulStop()
	}

	n.StreamFactory.CloseAllStreams()
}
