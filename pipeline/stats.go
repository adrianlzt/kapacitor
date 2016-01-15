package pipeline

import "time"

type StatsNode struct {
	chainnode
	// tick:ignore
	SourceNode Node
	// tick:ignore
	Interval time.Duration
}

func newStatsNode(n Node) *StatsNode {
	return &StatsNode{
		chainnode: newBasicChainNode("stats", StreamEdge, StreamEdge),
	}
}
