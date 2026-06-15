package retry

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"math"
	"net"
	"testing"
	"time"
)

// Unit test for the Step method
func TestBackoffStep(t *testing.T) {
	tests := []struct {
		name    string
		backoff Backoff
	}{
		{
			name: "Run Forever Backoff with a Cap of 2s",
			backoff: Backoff{
				Backoff: wait.Backoff{
					Steps:    math.MaxInt,
					Duration: 250 * time.Millisecond,
					Cap:      2 * time.Second,
					Factor:   5,
				}, RunForever: true},
		},
		{
			name: "Run Forever Backoff with a Cap of 10s",
			backoff: Backoff{
				Backoff: wait.Backoff{
					Steps:    math.MaxInt,
					Duration: 250 * time.Millisecond,
					Cap:      3 * time.Second,
					Factor:   5,
				}, RunForever: true},
		},
	}

	background := t.Context()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(background)
			ctx, timedout := context.WithTimeout(ctx, 30*time.Second)
			defer timedout()

			go func() {
				time.Sleep(time.Second * 5)
				cancel()
			}()

			var duration time.Duration
			_ = OnError(ctx, tt.backoff, AlwaysRetry, func() error {
				duration = tt.backoff.Step()
				return errors.New("fake error") //nolint:err113
			})
			if duration.Seconds() != tt.backoff.Cap.Seconds() {
				t.Errorf("the cap duration is %s, got %s", tt.backoff.Cap.String(), duration.String())
			}
		})
	}
}

func TestOnError_UntilContextCancel(t *testing.T) {
	opts := Backoff{Backoff: wait.Backoff{Factor: 1.0, Steps: math.MaxInt, Duration: 5 * time.Second}}
	fakeErr := errors.New("fake error")

	ctx, cancel := context.WithCancel(t.Context())

	ctx, timedout := context.WithTimeout(ctx, 10*time.Second)
	defer timedout()

	go func() {
		time.Sleep(time.Second * 1)
		cancel()
	}()

	alwaysRetry := func(err error) bool {
		return true
	}

	start := time.Now()
	// cancel context after 1 second
	err := OnError(ctx, opts, alwaysRetry, func() error {
		return fakeErr
	})
	if !errors.Is(err, fakeErr) {
		t.Errorf("unexpected error: %v", err)
	}
	elapsed := time.Now().Sub(start).Seconds()
	if elapsed > 1.2 {
		t.Errorf("unexpected elapsed time: %v", elapsed)
	}
}

func dial() (err error) {
	conn, err := net.DialTimeout("tcp", ":", 1*time.Nanosecond)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()
	return nil
}

func TestOnError_WaitInterrupted(t *testing.T) {
	opts := Backoff{Backoff: wait.Backoff{Factor: 1.0, Steps: math.MaxInt, Duration: 5 * time.Second}}
	neverRetry := func(err error) bool {
		return false
	}
	err := OnError(t.Context(), opts, neverRetry, func() error {
		return dial()
	})
	if err == nil {
		t.Errorf("expected error 'dial tcp :0: i/o timeout', got: %#v", err)
	}
}

func TestOnError_WithTimer(t *testing.T) {
	fakeErr := errors.New("fake error")

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()

	count := 0
	steps := 5
	err := OnErrorWithTimer(ctx, steps, 1*time.Second, func(err error) bool {
		return errors.Is(err, fakeErr) //nolint:err113
	}, func() error {
		count = count + 1
		return fakeErr
	})
	if !errors.Is(err, fakeErr) {
		t.Errorf("unexpected error: %v", err)
	}
	if count != steps {
		t.Errorf("unexpected number of times, expected %d, got %d", steps, count)
	}

	elapsed := time.Now().Sub(start).Seconds()
	if elapsed > 4.1 {
		t.Errorf("unexpected elapsed time: %v", elapsed)
	}
}
