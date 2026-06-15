package checks

import (
	"context"
)

var _ Handler = (*CustomHandler)(nil)

type Handler interface {
	Check(ctx context.Context) error
}

// HandlerFunc is an adapter to easily create health check handlers from
// ordinary functions.
func HandlerFunc(handlerFn func(ctx context.Context) error) Handler {
	return New(handlerFn)
}

// CustomHandler is a health check handler that uses a custom function to perform the check.
type CustomHandler struct {
	handlerFunc func(ctx context.Context) error
}

// New creates a new CustomHandler with the provided function.
func New(handlerFunc func(ctx context.Context) error) *CustomHandler {
	return &CustomHandler{
		handlerFunc: handlerFunc,
	}
}

// Check executes the custom health check function.
func (o *CustomHandler) Check(ctx context.Context) error {
	return o.handlerFunc(ctx)
}
