package kapacitor

import "github.com/influxdata/kapacitor/pipeline"

type StatsNode struct {
	node
	s  *pipeline.StatsNode
	en Node
}

// Create a new  StreamNode which filters data from a source.
func newStatsNode(et *ExecutingTask, n *pipeline.StatsNode) (*StatsNode, error) {
	// Lookup the executing node for stats.
	en := et.lookup[n.SourceNode.ID()]
	sn := &StatsNode{
		node: node{Node: n, et: et},
		en:   en,
	}
	sn.node.runF = sn.runStats
	return sn, nil
}

func (s *StatsNode) runStats() error {
	//TODO emit stats from en
	return nil
}
