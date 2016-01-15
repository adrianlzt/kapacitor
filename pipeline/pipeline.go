package pipeline

import (
	"bytes"
	"fmt"

	"github.com/influxdata/kapacitor/tick"
)

// A complete data processing pipeline. Starts with a single source.
// tick:ignore
type Pipeline struct {
	sources []Node
	id      ID
	sorted  []Node
}

// Create a pipeline from a given script.
// tick:ignore
func CreatePipeline(script string, sourceEdge EdgeType, scope *tick.Scope) (*Pipeline, error) {
	p := &Pipeline{}
	var src Node
	switch sourceEdge {
	case StreamEdge:
		src = newStreamNode()
		scope.Set("stream", src)
	case BatchEdge:
		src = newSourceBatchNode()
		scope.Set("batch", src)
	default:
		return nil, fmt.Errorf("source edge type must be either Stream or Batch not %s", sourceEdge)
	}
	src.setPipeline(p)
	p.addSource(src)

	err := tick.Evaluate(script, scope)
	if err != nil {
		return nil, err
	}
	p.Walk(p.setID)
	return p, nil

}

func (p *Pipeline) addSource(n Node) {
	p.sources = append(p.sources, n)
}

func (p *Pipeline) setID(n Node) error {
	n.setID(p.id)
	p.id++
	return nil
}

// Walks the entire pipeline and calls func f on each node exactly once.
// f will be called on a node n only after all of its parents have already had f called.
// tick:ignore
func (p *Pipeline) Walk(f func(n Node) error) error {
	if p.sorted == nil {
		p.sort()
	}
	for _, n := range p.sorted {
		err := f(n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline) sort() {
	for _, src := range p.sources {
		p.visit(src)
	}
	//reverse p.sorted
	s := p.sorted
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Depth first search topological sorting of a DAG.
// https://en.wikipedia.org/wiki/Topological_sorting#Algorithms
func (p *Pipeline) visit(n Node) {
	if n.tMark() {
		panic("pipeline contains a cycle")
	}
	if !n.pMark() {
		n.setTMark(true)
		for _, c := range n.Children() {
			p.visit(c)
		}
		n.setPMark(true)
		n.setTMark(false)
		p.sorted = append(p.sorted, n)
	}
}

// Return a graphviz .dot formatted byte array.
// tick:ignore
func (p *Pipeline) Dot(name string) []byte {

	var buf bytes.Buffer

	buf.Write([]byte("digraph "))
	buf.Write([]byte(name))
	buf.Write([]byte(" {\n"))
	p.Walk(func(n Node) error {
		n.dot(&buf)
		return nil
	})
	buf.Write([]byte("}"))

	return buf.Bytes()
}
