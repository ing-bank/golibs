package checks

import (
	"context"
	"testing"
)

func TestCustomHandler_Check(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		handlerFn func(ctx context.Context) error
		wantErr   bool
	}{
		{
			name:      "success",
			handlerFn: func(ctx context.Context) error { return nil },
			wantErr:   false,
		},
		{
			name:      "error",
			handlerFn: func(ctx context.Context) error { return context.Canceled },
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			o := &CustomHandler{
				handlerFunc: tt.handlerFn,
			}
			if err := o.Check(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerFunc(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		handlerFn func(ctx context.Context) error
		wantErr   bool
	}{
		{
			name:      "success",
			handlerFn: func(ctx context.Context) error { return nil },
			wantErr:   false,
		},
		{
			name:      "error",
			handlerFn: func(ctx context.Context) error { return context.DeadlineExceeded },
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			h := HandlerFunc(tt.handlerFn)
			if err := h.Check(ctx); (err != nil) != tt.wantErr {
				t.Errorf("HandlerFunc() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		handlerFn func(ctx context.Context) error
		wantErr   error
	}{
		{
			name:      "success",
			handlerFn: func(ctx context.Context) error { return nil },
			wantErr:   nil,
		},
		{
			name:      "error",
			handlerFn: func(ctx context.Context) error { return context.Canceled },
			wantErr:   context.Canceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := New(tt.handlerFn)
			ctx := t.Context()
			err := got.Check(ctx)
			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("New().Check() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("New().Check() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
