package Router

import "sort"

// Gossip 传播策略接口
type Router interface {
	SelectTargets(cost map[string]float64, fanout int, state map[string]bool) []string
}

// FanoutRouter 默认 fanout 传播
type FanoutRouter struct{}

func NewFanoutRouter() *FanoutRouter {
	return &FanoutRouter{}
}

func (FanoutRouter) SelectTargets(cost map[string]float64, fanout int, state map[string]bool) []string {
	candidates := []string{}
	for hash, ok := range state {
		if !ok { // 未发送/未收到 ACK
			candidates = append(candidates, hash)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return cost[candidates[i]] < cost[candidates[j]]
	})

	if len(candidates) > fanout {
		return candidates[:fanout]
	}
	return candidates
}
