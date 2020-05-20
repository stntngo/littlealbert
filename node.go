package littlealbert

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/opentracing/opentracing-go/log"
)

const maxNodeConcurrency = 10

// Result represents the result of a Node's execution
// within the context of a Behavior Tree.
type Result int

const (
	// Invalid is an invalid Result status. It should never be returned.
	Invalid Result = iota
	// Running is returned by a Node to indicate that its subtree is currently
	// executing one or more Tasks.
	Running
	// Success is returned by a Node to indicate that its subtree has
	// completed its tasks successfully.
	Success
	// Failure is returned by a Node to indicate that its subtree has
	// failed to complete its tasks successfully.
	Failure
)

// String returns the canonical string representation of
// the Result.
func (r Result) String() string {
	switch r {
	case 1:
		return "running"
	case 2:
		return "success"
	case 3:
		return "failure"
	}

	return "invalid"
}

// Node defines the minimum interface necessary to execute a Node
// within the context of a Behavior Tree.
type Node interface {
	Tick(context.Context) Result
}

// NamedNode extends the minimum Node interface to allow
// naming nodes and subtrees.
type NamedNode interface {
	Node

	Name() string
}

// ParentNode extends the minimum Node interface to allow
// nodes containing children to provide exported access
// to their children.
type ParentNode interface {
	Node

	Children() []Node
}

// Conditional is any function which, given a context, returns
// a boolean value.
func Conditional(name string, cond func(context.Context) bool) Node {
	return &conditional{
		name: name,
		cond: cond,
	}
}

type conditional struct {
	name string
	cond func(context.Context) bool
}

func (c conditional) Name() string {
	return c.name
}

func (c conditional) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, c.name)
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "conditional"),
	)

	if c.cond(ctx) {
		span.LogFields(
			log.String("node_result", Success.String()),
		)
		return Success
	}

	span.LogFields(
		log.String("node_result", Failure.String()),
	)
	return Failure
}

// Task is any childless function which, given a context,
// returns a Behavior Tree Result.
func Task(name string, t func(context.Context) Result) Node {
	return &task{
		name: name,
		t:    t,
	}
}

type task struct {
	name string
	t    func(context.Context) Result
}

func (t task) Name() string {
	return t.name
}

// Tick turns the childless Task function into a valid
// Behavior Tree Node.
func (t task) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, t.name)
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "task"),
	)

	result := t.t(ctx)

	span.LogFields(
		log.String("node_result", result.String()),
	)

	return result
}

// Sequence nodes route their execution ticks to their
// children from left to right until it finds a child that
// returns a non-Success Result, then returning that Result.
// A Sequence node will only return a Success Result
// if and only if all of its Children return Succes Results.
func Sequence(children ...Node) Node {
	return &sequence{
		children: children,
	}
}

type sequence struct {
	children []Node
}

func (s sequence) Children() []Node {
	return s.children
}

func (s sequence) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, "sequence")
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "sequence"),
	)

	for _, node := range s.children {
		if result := node.Tick(ctx); result != Success {
			span.LogFields(
				log.String("node_result", result.String()),
			)

			return result
		}
	}

	span.LogFields(
		log.String("node_result", Success.String()),
	)

	return Success
}

// Fallback Nodes route their execution ticks to their chldren
// from left to right until it finds a child that returns a Success
// or Running Result. A Fallback Node succeeds so long as any single
// child Node succeeds and a Fallback Node fails if and only if all
// of its children Nodes fail.
func Fallback(children ...Node) Node {
	return &fallback{
		children: children,
	}
}

type fallback struct {
	children []Node
}

func (f fallback) Children() []Node {
	return f.children
}

func (f fallback) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, "fallback")
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "fallback"),
	)

	for _, node := range f.children {
		if result := node.Tick(ctx); result == Success || result == Running {
			span.LogFields(
				log.String("node_result", result.String()),
			)

			return result
		}
	}

	span.LogFields(
		log.String("node_result", Failure.String()),
	)

	return Failure
}

// Decorator Nodes are control flow nodes that manipulate the Result returned
// by their single child Node.
func Decorator(name string, child Node, modifier func(context.Context, Result) Result) Node {
	return &decorator{
		name:  name,
		child: child,
		fn:    modifier,
	}
}

type decorator struct {
	name  string
	child Node
	fn    func(context.Context, Result) Result
}

func (d decorator) Name() string {
	return d.name
}

func (d decorator) Children() []Node {
	return []Node{d.child}
}

func (d decorator) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, d.name)
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "decorator"),
	)

	result := d.child.Tick(ctx)

	span.LogFields(
		log.String("wrapped_result", result.String()),
	)

	if d.fn != nil {
		result = d.fn(ctx, result)
	}

	span.LogFields(
		log.String("node_result", result.String()),
	)

	return result
}

// Parallel nodes route their execution tick to all children nodes every time
// their Tick method is called. The Parallel node Tick returns Success only if
// the number of Success Results returned by the child Tick calls is equal to
// or exceeds the Threshold value set in thresh. Conversely, the Parallel node
// returns Failure should the number of Failure results returned by children
// Nodes exceeds len(children) - thresh.
func Parallel(threshold int, children ...Node) Node {
	return &parallel{
		children: children,
		thresh:   uint64(threshold),
	}
}

type parallel struct {
	children []Node
	thresh   uint64
}

func (p parallel) Children() []Node {
	return p.children
}

func (p parallel) Tick(ctx context.Context) (res Result) {
	var successes, failures uint64

	span, ctx := childSpanFromContext(ctx, "parallel")
	defer func() {
		span.LogFields(
			log.String("node_type", "parallel"),
			log.String("node_result", res.String()),
			log.Int("parallel_success_count", int(successes)),
			log.Int("parallel_failure_count", int(failures)),
		)
		span.Finish()
	}()

	children := make(chan Node, len(p.children))

	for _, node := range p.children {
		children <- node
	}

	close(children)

	var wg sync.WaitGroup
	for i := 0; i < maxNodeConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for node := range children {
				switch result := node.Tick(ctx); result {
				case Success:
					atomic.AddUint64(&successes, 1)
				case Failure:
					atomic.AddUint64(&failures, 1)
				}
			}
		}()
	}

	wg.Wait()

	if successes >= p.thresh {
		return Success
	}

	if failures >= (uint64(len(p.children)) - p.thresh) {
		return Failure
	}

	return Running
}

// Dynamic nodes are nodes whose children cannot be defined at compile time.
// The subtree defined by the Dynamic node is constructed at runtime when
// the cons function is called.
func Dynamic(name string, cons func(context.Context) Node) Node {
	return &dynamic{
		name: name,
		cons: cons,
	}
}

type dynamic struct {
	name string
	cons func(context.Context) Node
}

func (d dynamic) Name() string {
	return d.name
}

func (d dynamic) Children() []Node {
	return []Node{d.cons(context.Background())}
}

func (d dynamic) construct(ctx context.Context) Node {
	span, ctx := childSpanFromContext(ctx, d.name+"::constructor")
	defer span.Finish()

	span.LogFields(
		log.String("dynamic_step", "constructor"),
	)

	return d.cons(ctx)
}

func (d dynamic) Tick(ctx context.Context) Result {
	span, ctx := childSpanFromContext(ctx, d.name)
	defer span.Finish()

	span.LogFields(
		log.String("node_type", "dynamic"),
	)

	return d.construct(ctx).Tick(ctx)
}
