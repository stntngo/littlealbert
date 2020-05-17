package littlealbert

import (
	"context"
	"time"
)

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
	if c.cond(ctx) {
		return Success
	}

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
	return t.t(ctx)
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
	for _, node := range s.children {
		if result := node.Tick(ctx); result != Success {
			return result
		}
	}

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
	for _, node := range f.children {
		if result := node.Tick(ctx); result == Success || result == Running {
			return result
		}
	}

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
	result := d.child.Tick(ctx)
	if d.fn != nil {
		return d.fn(ctx, result)
	}

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
		thresh:   threshold,
	}
}

type parallel struct {
	children []Node
	thresh   int
}

func (p parallel) Children() []Node {
	return p.children
}

func (p parallel) Tick(ctx context.Context) Result {
	var successes, failures int
	for _, node := range p.children {
		switch result := node.Tick(ctx); result {
		case Success:
			successes++
		case Failure:
			failures++
		}
	}

	if successes >= p.thresh {
		return Success
	}

	if failures >= (len(p.children) - p.thresh) {
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

func (d dynamic) Tick(ctx context.Context) Result {
	return d.cons(ctx).Tick(ctx)
}

// Run executes the provided Behavior Tree at the provided Tick Rate with
// the specified per-Tick timeout and provided parent context until a non-Running
// Result is returned.
func Run(ctx context.Context, tree Node, tickRate, tickTimeout time.Duration) Result {
	for {
		tickCtx, cancel := context.WithTimeout(ctx, tickRate)

		result := tree.Tick(tickCtx)

		cancel()

		if result != Running {
			return result
		}

		select {
		case <-ctx.Done():
			return Failure
		case <-time.Tick(tickRate):
			continue
		}
	}
}
