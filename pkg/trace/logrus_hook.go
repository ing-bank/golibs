package trace

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// LogrusHook is a logrus hook that adds logs to the active span as events.
// This is a replacement for the uptrace/opentelemetry-go-extra/otellogrus package
// using only official OpenTelemetry packages.
type LogrusHook struct {
	levels           []logrus.Level
	errorStatusLevel logrus.Level
}

// NewLogrusHook returns a logrus hook configured to add log entries as span events.
func NewLogrusHook(levels ...logrus.Level) *LogrusHook {
	if len(levels) == 0 {
		levels = []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
		}
	}
	return &LogrusHook{
		levels:           levels,
		errorStatusLevel: logrus.ErrorLevel,
	}
}

// Levels returns the logrus levels on which this hook is fired.
func (h *LogrusHook) Levels() []logrus.Level {
	return h.levels
}

// Fire is called when a log event is fired.
func (h *LogrusHook) Fire(entry *logrus.Entry) error {
	ctx := entry.Context
	if ctx == nil {
		ctx = context.Background()
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	// Build attributes from logrus fields
	attrs := make([]attribute.KeyValue, 0, len(entry.Data)+2)
	attrs = append(attrs,
		attribute.String("log.severity", entry.Level.String()),
		attribute.String("log.message", entry.Message),
	)

	for k, v := range entry.Data {
		attrs = append(attrs, convertToAttribute(k, v))
	}

	// Add the log entry as a span event
	span.AddEvent("log", trace.WithAttributes(attrs...))

	// Set span status to error if the log level is error or above
	if entry.Level <= h.errorStatusLevel {
		span.SetStatus(codes.Error, entry.Message)
	}

	return nil
}

// convertToAttribute converts a value to an OpenTelemetry attribute.
func convertToAttribute(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case nil:
		return attribute.String(key, "<nil>")
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case int32:
		return attribute.Int64(key, int64(v))
	case uint:
		return attribute.Int64(key, int64(v))
	case uint64:
		return attribute.Int64(key, int64(v))
	case uint32:
		return attribute.Int64(key, int64(v))
	case float64:
		return attribute.Float64(key, v)
	case float32:
		return attribute.Float64(key, float64(v))
	case bool:
		return attribute.Bool(key, v)
	case fmt.Stringer:
		return attribute.String(key, v.String())
	case error:
		return attribute.String(key, v.Error())
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}
