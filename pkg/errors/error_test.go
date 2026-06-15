package errors

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"syscall"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var apiError = apierrors.NewTooManyRequestsError("take it easy")

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"kube error apiErrorsIsRetryable", apiError, true},
		{"retry unknown error", errors.New("unknown error"), false},
		{"not found", ErrNotFound, false},
		{"net.Error timeout", mockNetError{timeout: true}, true},
		{"net.Error temporary", mockNetError{temporary: true}, true},
		{"syscall ECONNRESET", syscall.ECONNRESET, true},
		{"syscall ECONNREFUSED", syscall.ECONNREFUSED, true},
		{"syscall ETIMEDOUT", syscall.ETIMEDOUT, true},
		{"syscall EHOSTUNREACH", syscall.EHOSTUNREACH, true},
		{"syscall ENETUNREACH", syscall.ENETUNREACH, true},
		{"syscall ECONNABORTED", syscall.ECONNABORTED, true},
		{"syscall EPIPE", syscall.EPIPE, true},
		{"url.Error wrapping timeout", &url.Error{Err: mockNetError{timeout: true}}, true},
		{"url.Error wrapping non-retryable", &url.Error{Err: errors.New("other")}, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"net.DNSError temporary", &net.DNSError{IsTemporary: true}, true},
		{"net.DNSError timeout", &net.DNSError{IsTimeout: true}, true},
		{"url.Error wrapping io.EOF", &url.Error{Err: io.EOF}, true},
		{"url.Error wrapping context.Canceled", &url.Error{Err: context.Canceled}, false},
		{"non-retryable error", errors.New("other"), false},
		{"nil error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout, temporary bool
}

func (e mockNetError) Error() string   { return "mock net error" }
func (e mockNetError) Timeout() bool   { return e.timeout }
func (e mockNetError) Temporary() bool { return e.temporary }

func TestStatusCodeAsErr(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  error
	}{
		{"OK status", 200, nil},
		{"BadRequest", 400, ErrBadRequest},
		{"Unauthorized", 401, ErrUnauthorized},
		{"Unknown status", 999, ErrHTTPStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StatusCodeAsErr(tt.input)
			if (got == nil) != (tt.want == nil) || (got != nil && got.Error() != tt.want.Error()) {
				t.Errorf("StatusCodeAsErr(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestErrAsStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, 200},
		{"ErrBadRequest", ErrBadRequest, 400},
		{"ErrNotFound", ErrNotFound, 404},
		{"unknown error", errors.New("other"), 500},
		{"api error", apiError, 429},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ErrAsStatusCode(tt.err)
			if got != tt.want {
				t.Errorf("ErrAsStatusCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestAlwaysRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, true},
		{"non-nil error", errors.New("fail"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AlwaysRetry(tt.err)
			if got != tt.want {
				t.Errorf("AlwaysRetry(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRunOnError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"non-nil error", errors.New("fail"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RunOnError(tt.err)
			if got != tt.want {
				t.Errorf("RunOnError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetriableErrorFn(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"retryable error", ErrRequestTimeout, true},
		{"non-retryable error", errors.New("fail"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RetriableErrorFn(tt.err)
			if got != tt.want {
				t.Errorf("RetriableErrorFn(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
