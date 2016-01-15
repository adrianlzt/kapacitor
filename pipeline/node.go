package pipeline

import (
	"bytes"
	"fmt"
	"time"

	"github.com/influxdata/kapacitor/tick"
)

// The type of data that travels along an edge connecting two nodes in a Pipeline.
type EdgeType int

const (
	// No data is transfered
	NoEdge EdgeType = iota
	// Data is transfered immediately and one point at a time.
	StreamEdge
	// Data is transfered in batches as soon as it is ready.
	BatchEdge
	// Data is transfered as it is received from a map function.
	ReduceEdge
)

type ID int

func (e EdgeType) String() string {
	switch e {
	case StreamEdge:
		return "stream"
	case BatchEdge:
		return "batch"
	case ReduceEdge:
		return "reduce"
	default:
		return "unknown EdgeType"
	}
}

//Generic node in a pipeline
type Node interface {
	// List of parents of this node.
	Parents() []Node
	// List of children of this node.
	Children() []Node
	// Add a parent node only, does not add the child relation.
	addParent(p Node)
	// Links a child node by adding both the parent and child relation.
	linkChild(c Node)

	// Short description of the node does not need to be unique
	Desc() string

	// Friendly readable unique name of the node
	Name() string
	SetName(string)

	// Unique id for the node
	ID() ID
	setID(ID)

	// The type of input the node wants.
	Wants() EdgeType
	// The type of output the node provides.
	Provides() EdgeType

	// Helper methods for walking DAG
	tMark() bool
	setTMark(b bool)
	pMark() bool
	setPMark(b bool)
	setPipeline(*Pipeline)
	pipeline() *Pipeline

	// Return .dot string to graph DAG
	dot(buf *bytes.Buffer)
}

type node struct {
	p        *Pipeline
	desc     string
	name     string
	id       ID
	parents  []Node
	children []Node
	wants    EdgeType
	provides EdgeType
	tm       bool
	pm       bool
}

// tick:ignore
func (n *node) Desc() string {
	return n.desc
}

// tick:ignore
func (n *node) ID() ID {
	return n.id
}

func (n *node) setID(id ID) {
	n.id = id
}

// tick:ignore
func (n *node) Name() string {
	if n.name == "" {
		n.name = fmt.Sprintf("%s%d", n.Desc(), n.ID())
	}
	return n.name
}

// tick:ignore
func (n *node) SetName(name string) {
	n.name = name
}

// tick:ignore
func (n *node) Parents() []Node {
	return n.parents
}

// tick:ignore
func (n *node) Children() []Node {
	return n.children
}

func (n *node) addParent(c Node) {
	n.parents = append(n.parents, c)
}

func (n *node) linkChild(c Node) {
	n.children = append(n.children, c)
	c.addParent(n)
}

func (n *node) tMark() bool {
	return n.tm
}

func (n *node) setTMark(b bool) {
	n.tm = b
}

func (n *node) pMark() bool {
	return n.pm
}

func (n *node) setPMark(b bool) {
	n.pm = b
}

func (n *node) setPipeline(p *Pipeline) {
	n.p = p
}
func (n *node) pipeline() *Pipeline {
	return n.p
}

// tick:ignore
func (n *node) Wants() EdgeType {
	return n.wants
}

// tick:ignore
func (n *node) Provides() EdgeType {
	return n.provides
}

func (n *node) dot(buf *bytes.Buffer) {
	for _, c := range n.children {
		buf.Write([]byte(fmt.Sprintf("%s -> %s;\n", n.Name(), c.Name())))
	}
}

// Create a new stream of data that contains the internal statistics of the node.
func (n *node) Stats(interval time.Duration) *StatsNode {
	stats := newStatsNode(n)
	n.pipeline().addSource(stats)
	return stats
}

// ---------------------------------
// Chaining methods
//

// basic implementation of node + chaining methods
type chainnode struct {
	node
}

func newBasicChainNode(desc string, wants, provides EdgeType) chainnode {
	return chainnode{node{
		desc:     desc,
		wants:    wants,
		provides: provides,
	}}
}

// Create a new node that filters the data stream by a given expression.
func (n *chainnode) Where(expression tick.Node) *WhereNode {
	w := newWhereNode(n.provides, expression)
	n.linkChild(w)
	return w
}

// Create an http output node that caches the most recent data it has received.
// The cached data is available at the given endpoint.
// The endpoint is the relative path from the API endpoint of the running task.
// For example if the task endpoint is at "/api/v1/task/<task_name>" and endpoint is
// "top10", then the data can be requested from "/api/v1/task/<task_name>/top10".
func (n *chainnode) HttpOut(endpoint string) *HTTPOutNode {
	h := newHTTPOutNode(n.provides, endpoint)
	n.linkChild(h)
	return h
}

// Create an influxdb output node that will store the incoming data into InfluxDB.
func (n *chainnode) InfluxDBOut() *InfluxDBOutNode {
	i := newInfluxDBOutNode(n.provides)
	n.linkChild(i)
	return i
}

// Create an alert node, which can trigger alerts.
func (n *chainnode) Alert() *AlertNode {
	a := newAlertNode(n.provides)
	n.linkChild(a)
	return a
}

// Perform the union of this node and all other given nodes.
func (n *chainnode) Union(node ...Node) *UnionNode {
	u := newUnionNode(n.provides, node)
	n.linkChild(u)
	return u
}

// Join this node with other nodes. The data is joined on timestamp.
func (n *chainnode) Join(others ...Node) *JoinNode {
	others = append([]Node{n}, others...)
	j := newJoinNode(n.provides, others)
	return j
}

// Create an eval node that will evaluate the given transformation function to each data point.
//  A list of expressions may be provided and will be evaluated in the order they are given
// and results of previous expressions are made available to later expressions.
func (n *chainnode) Eval(expressions ...tick.Node) *EvalNode {
	e := newEvalNode(n.provides, expressions)
	n.linkChild(e)
	return e
}

// Group the data by a set of tags.
//
// Can pass literal * to group by all dimensions.
// Example:
//    .groupBy(*)
//
func (n *chainnode) GroupBy(tag ...interface{}) *GroupByNode {
	g := newGroupByNode(n.provides, tag)
	n.linkChild(g)
	return g
}

// Curently you must use MapReduce
//// Perform just the map step of a map-reduce operation.
//// A map step must always be followed by a reduce step.
//// See Apply for performing simple transformations.
//// See MapReduce for performing map-reduce in one command.
////
//// NOTE: Map can only be applied to batch edges.
//func (n *chainnode) Map(f interface{}) (c *MapNode) {
//	if n.Provides() != BatchEdge {
//		panic("cannot MapReduce non batch edge, did you forget to window the data?")
//	}
//	c = newMapNode(f)
//	n.linkChild(c)
//	return c
//}
//
//// Perform just the reduce step of a map-reduce operation.
////
//// NOTE: Reduce can only be applied to map edges.
//func (n *chainnode) Reduce(f interface{}) (c *ReduceNode) {
//	switch n.Provides() {
//	case ReduceEdge:
//		c = newReduceNode(f)
//	default:
//		panic("cannot Reduce non reduce edge, did you forget to map the data?")
//	}
//	n.linkChild(c)
//	return c
//}

// Perform a map-reduce operation on the data.
// The built-in functions under `influxql` provide the
// selection,aggregation, and transformation functions
// from the InfluxQL language.
//
// MapReduce may be applied to either a batch or a stream edge.
// In the case of a batch each batch is passed to the mapper idependently.
// In the case of a stream all incoming data points that have
// the exact same time are combined into a batch and sent to the mapper.
func (n *chainnode) MapReduce(mr MapReduceInfo) *ReduceNode {
	var m *MapNode
	var r *ReduceNode
	m = newMapNode(n.Provides(), mr.Map)
	r = newReduceNode(mr.Reduce, mr.Edge)
	n.linkChild(m)
	m.linkChild(r)
	return r
}

// Create a new node that windows the stream by time.
//
// NOTE: Window can only be applied to stream edges.
func (n *chainnode) Window() *WindowNode {
	if n.Provides() != StreamEdge {
		panic("cannot Window batch edge")
	}
	w := newWindowNode()
	n.linkChild(w)
	return w
}

// Create a new node that samples the incoming points or batches.
//
// One point will be emitted every count or duration specified.
func (n *chainnode) Sample(rate interface{}) *SampleNode {
	s := newSampleNode(n.Provides(), rate)
	n.linkChild(s)
	return s
}

// Create a new node that computes the derivative of adjacent points.
func (n *chainnode) Derivative(field string) *DerivativeNode {
	s := newDerivativeNode(n.Provides(), field)
	n.linkChild(s)
	return s
}
