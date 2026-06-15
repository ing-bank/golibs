package trace

import (
	"context"
	"github.com/ing-bank/golibs/pkg/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NewSpan returns a new span from the global tracer. Depending on the `opts`
// argument, the span is either a plain one or a customised one. Each resulting
// span must be completed with `defer span.End()` right after the call.
func NewSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Use the global trace provider to instantiate a span anywhere
	return otel.Tracer("").Start(ctx, name, opts...)
}

func NewSpanWithContext(ctx context.Context) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	return trace.ContextWithSpan(ctx, span), span
}

func GetTraceIDFromContext(ctx context.Context) trace.TraceID {
	return trace.SpanFromContext(ctx).SpanContext().TraceID()
}

func NewSpanFromID(ctx context.Context, name string, traceId, spanId string) (context.Context, trace.Span) {
	traceID, _ := trace.TraceIDFromHex(traceId)
	spanID, _ := trace.SpanIDFromHex(spanId)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: 0x1,
		Remote:     true,
	})

	return NewSpan(trace.ContextWithRemoteSpanContext(ctx, sc), name)
}

// AddSpanTags adds a new tags to the span. It will appear under "Tags" section
// of the selected span. Use this if you think the tag and its value could be
// useful while debugging.
func AddSpanTags(span trace.Span, tags map[string]string) {
	list := make([]attribute.KeyValue, len(tags))

	var i int
	for k, v := range tags {
		list[i] = attribute.Key(k).String(v)
		i++
	}
	span.SetAttributes(list...)
}

// AddSpanEvents adds a new events to the span. It will appear under the "Logs"
// section of the selected span. Use this if the event could mean anything
// valuable while debugging.
func AddSpanEvents(span trace.Span, name string, events map[string]string) {
	list := make([]trace.EventOption, len(events))

	var i int
	for k, v := range events {
		list[i] = trace.WithAttributes(attribute.Key(k).String(v))
		i++
	}
	span.AddEvent(name, list...)
}

// Error fail the span and record the error message
func Error(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func RebuildTraceContext(ctx context.Context, name, requestID, traceID, spanID string) (context.Context, trace.Span) {
	c, span := NewSpanFromID(ctx, name, traceID, spanID)
	return utils.SetRequestID(c, requestID), span
}
