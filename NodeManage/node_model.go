package NodeManage

import (
	"Gossip/GossipClientFactory"
	"Gossip/NodeManage/Logger"
	"Gossip/NodeManage/Router"
	"Gossip/NodeManage/Storage"
	pb "Gossip/gossip_rpc/proto"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
)

type GossipNode struct {
	NodeHash                     string
	Neighbors                    map[string]string                        // nodeHash -> endpoint
	Cost                         map[string]float64                       // 节点通信代价
	Fanout                       int64                                    // fanout 数量
	Storage                      Storage.Storage                          // 消息存储
	Router                       Router.Router                            // 路由策略
	Logger                       Logger.Logger                            // 日志接口
	clientFactory                *GossipClientFactory.GossipClientFactory // grpcclient管理
	pb.UnimplementedGossipServer                                          // 必须嵌入，满足 gRPC server 接口
}

func NewGossipNode(
	id string,
	store Storage.Storage,
	router Router.Router,
	log Logger.Logger,
	timeout time.Duration,
) *GossipNode {
	clientFactory := GossipClientFactory.NewGossipClientFactory(timeout)
	return &GossipNode{
		NodeHash:      id,
		Neighbors:     map[string]string{},
		Cost:          map[string]float64{},
		Fanout:        3,
		Logger:        log,
		Storage:       store,
		Router:        router,
		clientFactory: clientFactory,
	}
}

func (n *GossipNode) StartGRPCServer(endpoint string) {
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatalf("[%s] failed to listen on %s: %v", n.NodeHash, endpoint, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterGossipServer(grpcServer, n)

	go func() {
		fmt.Printf("[%s] gRPC server listening on %s\n", n.NodeHash, endpoint)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("[%s] failed to serve gRPC: %v", n.NodeHash, err)
		}
	}()
}
