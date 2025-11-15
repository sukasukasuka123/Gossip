package NodeManage

import (
	"Gossip/NodeManage/Router"
	"Gossip/NodeManage/Storage"
	"Gossip/NodeManage/TransportMessage"
)

type GossipNode[T any] struct {
	NodeHash  string
	Neighbors map[string]string          // nodeHash -> endpoint
	Cost      map[string]float64         // 节点通信代价
	Fanout    int                        // fanout 数量
	Storage   Storage.Storage            // 消息存储
	Router    Router.Router              // 路由策略
	Transport TransportMessage.Transport // 网络发送接口
}

func NewGossipNode[T any](
	id string,
	endpoint string,
	payload T,
	store Storage.Storage,
	trans TransportMessage.Transport,
	router Router.Router,
) *GossipNode[T] {

	return &GossipNode[T]{
		NodeHash:  id,
		Neighbors: map[string]string{},
		Cost:      map[string]float64{},
		Fanout:    3,

		Storage:   store,
		Transport: trans,
		Router:    router,
	}
}
