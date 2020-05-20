package littlealbert

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

var noop = opentracing.NoopTracer{}

func childSpanFromContext(ctx context.Context, operation string) (opentracing.Span, context.Context) {
	span := opentracing.SpanFromContext(ctx)
	var tracer opentracing.Tracer = &noop

	if span != nil {
		tracer = span.Tracer()
	}

	return opentracing.StartSpanFromContextWithTracer(
		ctx,
		tracer,
		"littlealbert::"+operation,
	)
}
