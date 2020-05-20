package littlealbert

import (
	"context"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var (
	defaultTickRate    = 10 * time.Second
	defaultTickTimeout = time.Second
	defaultTracer      = opentracing.NoopTracer{}
)

// RunConfiguration ...
type RunConfiguration struct {
	tickRate    time.Duration
	tickTimeout time.Duration
	tracer      opentracing.Tracer
}

func defaultRunConfig() *RunConfiguration {
	return &RunConfiguration{
		tickRate:    defaultTickRate,
		tickTimeout: defaultTickTimeout,
		tracer:      &defaultTracer,
	}
}

// RunOption ...
type RunOption func(config *RunConfiguration)

func WithTracer(tracer opentracing.Tracer) RunOption {
	return func(config *RunConfiguration) {
		config.tracer = tracer
	}
}

// Run executes the provided Behavior Tree at the provided Tick Rate with
// the specified per-Tick timeout and provided parent context until a non-Running
// Result is returned.
func Run(ctx context.Context, tree Node, opts ...RunOption) Result {
	config := defaultRunConfig()

	for _, opt := range opts {
		opt(config)
	}

	opentracing.SetGlobalTracer(config.tracer)

	for {
		tickCtx, cancel := context.WithTimeout(ctx, config.tickTimeout)
		root := opentracing.StartSpan("littlealbert::root")
		tickCtx = opentracing.ContextWithSpan(tickCtx, root)

		result := tree.Tick(tickCtx)

		cancel()
		root.LogFields(
			log.String("node_type", "root"),
			log.String("node_result", result.String()),
		)

		root.Finish()

		if result != Running {
			return result
		}

		select {
		case <-ctx.Done():
			return Failure
		case <-time.Tick(config.tickRate):
			continue
		}
	}
}
