package littlealbert

import "context"

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

// Conditional is any function which, given a context, returns
// a boolean value.
type Conditional func(context.Context) bool

// Tick turns the conditional boolean into a valid Behavior
// Tree Node.
func (c Conditional) Tick(ctx context.Context) Result {
	if c(ctx) {
		return Success
	}

	return Failure
}

// Task is any childless function which, given a context,
// returns a Behavior Tree Result.
type Task func(context.Context) Result

// Tick turns the childless Task function into a valid
// Behavior Tree Node.
func (t Task) Tick(ctx context.Context) Result {
	return t(ctx)
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
func Decorator(child Node, modifier func(context.Context, Result) Result) Node {
	return &decorator{
		child: child,
		fn:    modifier,
	}
}

type decorator struct {
	child Node
	fn    func(context.Context, Result) Result
}

func (d decorator) Tick(ctx context.Context) Result {
	return d.fn(ctx, d.child.Tick(ctx))

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
