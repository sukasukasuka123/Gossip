package Router

// Gossip 传播策略接口
type Router interface {
	SelectTargets(cost map[string]float64, state map[string]bool) []string
}
