package timed

import (
	"context"
	goerrors "errors"
	"testing"
	"time"

	errors "github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestCache(optCfg ...*Config) *Timed[string, string] {
	cfg := opt.Opt(&Config{
		SyncPeriod: metav1.Duration{Duration: 100 * time.Millisecond}, // Use 1s sync period for second precision
		MaxAge:     metav1.Duration{Duration: 2 * time.Second},        // Use 2 seconds for expiration
	}, optCfg)
	c, _ := NewForBuilders[string, string](cfg, memory.New, threadsafe.New)
	return c
}

func TestApplyAndRead(t *testing.T) {
	t.Parallel()
	c := newTestCache()
	err := c.Apply(t.Context(), "a", "1")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	val, err := c.Read(t.Context(), "a")
	if err != nil || val != "1" {
		t.Fatalf("Read failed: got %v, err %v", val, err)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	c := newTestCache()
	_ = c.Apply(t.Context(), "a", "1")
	err := c.Delete(t.Context(), "a")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = c.Read(t.Context(), "a")
	if err == nil {
		t.Fatalf("Expected error for deleted key")
	}
}

func TestExpiration(t *testing.T) {
	t.Parallel()
	c := newTestCache()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	_ = c.Apply(t.Context(), "a", "1")

	if err := c.Run(ctx); err != nil { // Wait longer than MaxAgeSec (2s)
		t.Fatalf("Error from cache run: %v", err)
	}

	foo, err := c.Read(t.Context(), "a")
	if err == nil {
		t.Fatalf("Expected entry to be expired, got value: %v", foo)
	}
}

func TestTimestampReApplyOnAccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cfg := &Config{
		SyncPeriod: metav1.Duration{Duration: 1100 * time.Millisecond}, // Use 1s sync period for second precision
		MaxAge:     metav1.Duration{Duration: 2 * time.Second},         // Use 2 seconds for expiration
	}
	c := newTestCache(cfg)

	errChan := make(chan error)
	go func() {
		errChan <- c.Run(ctx)
	}()
	_ = c.Apply(ctx, "a", "1")
	time.Sleep(1 * time.Second)

	_, err := c.Read(ctx, "a")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Key should be cleaned up
	_, err = c.Read(ctx, "a")
	if err == nil {
		t.Fatalf("Expected entry to be expired after enough time")
	}

	if err := <-errChan; err != nil { // Wait longer than MaxAgeSec (2s)
		t.Fatalf("Error from cache run: %v", err)
	}
}

func TestReadNonExistentReturnsNotFound(t *testing.T) {
	t.Parallel()
	c := newTestCache()
	_, err := c.Read(t.Context(), "nope")
	if !goerrors.Is(err, errors.ErrNotFound) {
		t.Fatalf("Expected http.ErrNotFound, got %v", err)
	}
}

func TestOptionsValidation(t *testing.T) {
	t.Parallel()
	opts := &Config{SyncPeriod: metav1.Duration{}, MaxAge: metav1.Duration{Duration: 1 * time.Second}}
	_, err := New[string, string](opts, nil)
	if err == nil {
		t.Fatalf("Expected error for invalid options")
	}
	opts = &Config{SyncPeriod: metav1.Duration{Duration: 1 * time.Second}, MaxAge: metav1.Duration{}}
	_, err = New[string, string](opts, nil)
	if err == nil {
		t.Fatalf("Expected error for invalid options")
	}
}
