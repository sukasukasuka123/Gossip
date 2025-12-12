package NodeManage

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	"github.com/sukasukasuka123/Gossip/Logger"
	"github.com/sukasukasuka123/Gossip/MessageManage"
	"github.com/sukasukasuka123/Gossip/Router"
	"github.com/sukasukasuka123/Gossip/Storage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type SingleStreamNode struct {
	NodeHash      string
	Neighbors     map[string]string
	Cost          map[string]float64
	Fanout        int64
	Storage       Storage.Storage
	Router        Router.Router
	Logger        Logger.Logger
	StreamFactory *GossipStreamFactory.StreamFactory

	grpcServer *grpc.Server
	pb.UnimplementedGossipServer
}

func NewSingleStreamNode(id string, store Storage.Storage, router Router.Router, log Logger.Logger, timeout time.Duration) *SingleStreamNode {
	return &SingleStreamNode{
		NodeHash:      id,
		Neighbors:     map[string]string{}, //nodehash-endpoint
		Cost:          map[string]float64{},
		Fanout:        3,
		Storage:       store,
		Router:        router,
		Logger:        log,
		StreamFactory: GossipStreamFactory.NewStreamFactory(timeout),
	}
}

func (n *SingleStreamNode) StartGRPCServer(endpoint string) {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("[%s] failed to listen: %v", n.NodeHash, err)
	}

	ka := keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
		MaxConnectionAge:  15 * time.Minute,
		Time:              2 * time.Minute,
		Timeout:           20 * time.Second,
	}

	n.grpcServer = grpc.NewServer(grpc.KeepaliveParams(ka))
	pb.RegisterGossipServer(n.grpcServer, n)

	go func() {
		fmt.Printf("[%s] gRPC server listening at %s\n", n.NodeHash, endpoint)
		if err := n.grpcServer.Serve(lis); err != nil {
			log.Fatalf("[%s] gRPC serve failed: %v", n.NodeHash, err)
		}
	}()
}

func (n *SingleStreamNode) StopGRPC() {
	if n.grpcServer != nil {
		fmt.Printf("[%s] stopping gRPC server\n", n.NodeHash)
		n.grpcServer.GracefulStop()
	}
	n.StreamFactory.CloseAllStreams()
}

func (n *SingleStreamNode) Stream(stream pb.Gossip_StreamServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		isNew, _ := n.Storage.InitState(msg.Hash, keys(n.Neighbors))
		stream.Send(&pb.GossipACK{
			Hash:     msg.Hash,
			FromHash: n.NodeHash,
		})

		if isNew {
			internal := MessageManage.GossipMessage[[]byte]{
				Hash:     msg.Hash,
				FromHash: msg.FromHash,
				Payload:  msg.GetPayLoad(),
			}
			go n.broadcast(internal)
		}
	}
}

func (n *SingleStreamNode) broadcast(msg MessageManage.GossipMessage[[]byte]) {
	for _, target := range keys(n.Neighbors) {
		if n.Storage.GetState(msg.Hash, target) {
			continue
		}

		ep := n.Neighbors[target]
		stream, err := n.StreamFactory.GetStream(target, ep)
		if err != nil {
			n.Logger.Log("connect fail: "+err.Error(), n.NodeHash)
			continue
		}

		n.Storage.MarkSent(msg.Hash, target)
		go stream.Send(&pb.GossipMessage{
			Hash:     msg.Hash,
			FromHash: msg.FromHash,
			PayLoad:  msg.Payload,
		})
	}
}
