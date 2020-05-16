package littlealbert

import "context"

// RunUntilTrue will run the underlying child Node until it returns
// a Successful Result effectively ignoring any Failures..
func RunUntilTrue(child Node) Node {
	return Decorator(
		child,
		func(_ context.Context, result Result) Result {
			if result == Success {
				return Success
			}

			return Running
		},
	)
}

// RunUntilFailure will run the underlying child Node until it returns
// a Failure Result effectively ignoring any Successes.
func RunUntilFailure(child Node) Node {
	return Decorator(
		child,
		func(_ context.Context, status Result) Result {
			if status == Failure {
				return Failure
			}

			return Running
		},
	)
}

// Invert inverts the Result returned by the child Node.
func Invert(child Node) Node {
	return Decorator(
		child,
		func(_ context.Context, result Result) Result {
			switch result {
			case Success:
				return Failure
			case Failure:
				return Success
			default:
				return result
			}
		},
	)
}

// Ternary constructs a classic branching
// "If {predicate} then {whenTrue} else {whenFalse}"
// BT subtree, executing the Node provided under the
// whenTrue argument if and only if the predicate
// returns successfully otherwise executing the node
// within the whenFalse argument.
func Ternary(predicate, whenTrue, whenFalse Node) Node {
	return Fallback(
		Sequence(predicate, whenTrue),
		whenFalse,
	)
}