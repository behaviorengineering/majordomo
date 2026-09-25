package observability

import (
	"context"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartChainSpan starts a CHAIN span (OpenInference orchestration).
func StartChainSpan(ctx context.Context, serviceName, operationName string) (context.Context, trace.Span) {
	if serviceName == "" {
		serviceName = DefaultServiceName
	}
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, operationName, trace.WithAttributes(
		attribute.String("openinference.span.kind", "CHAIN"),
		attribute.String("command.operation", operationName),
		attribute.String("service.name", serviceName),
	))
	spanCtx := span.SpanContext()
	ctx = core.WithExecutionState(ctx)
	if state := core.GetExecutionState(ctx); state != nil {
		_ = state
		ctx = context.WithValue(ctx, contextKey("otel.trace_id"), spanCtx.TraceID().String())
		ctx = context.WithValue(ctx, contextKey("otel.span_id"), spanCtx.SpanID().String())
	}
	return ctx, span
}

type contextKey string

// EndSpanWithStatus ends a span with OK or ERROR from the named return err.
func EndSpanWithStatus(span trace.Span, err *error) {
	if span == nil {
		return
	}
	if err != nil && *err != nil {
		span.RecordError(*err)
		span.SetStatus(codes.Error, (*err).Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// TraceIDFromContext returns the active OTEL trace id if present.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	if span != nil {
		sc := span.SpanContext()
		if sc.IsValid() {
			return sc.TraceID().String()
		}
	}
	if v, ok := ctx.Value(contextKey("otel.trace_id")).(string); ok {
		return v
	}
	return ""
}
