// Package utils is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
package utils

import (
	"context"

	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

const RequestIdKey = "rid"
const LoggerRequestIdKey = "rid"

func GetValueFromContextOrDefault[T string | bool](ctx context.Context, key string, def T) T {
	rawVal := ctx.Value(key)
	val, ok := rawVal.(T)
	if ok {
		return val
	}
	return def
}

func GetRequestIDFromContext(ctx context.Context) string {
	return GetValueFromContextOrDefault(ctx, RequestIdKey, "unknown")
}

func NewRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, RequestIdKey, xid.New().String())
}

func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIdKey, requestID)
}

func GetLoggerFromContext(ctx context.Context, parentLogger *log.Entry) *log.Entry {
	var logger *log.Entry
	if parentLogger != nil {
		logger = parentLogger
	} else {
		logger = new(log.Entry)
	}
	requestID := GetRequestIDFromContext(ctx)
	return logger.WithContext(ctx).WithField(LoggerRequestIdKey, requestID)
}
