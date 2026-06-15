package retry

import (
	"context"
	"fmt"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sretry "k8s.io/client-go/util/retry"
)

// TODO: Use type aliases for Kubernetes

var (
	DefaultBackoff = Backoff{Backoff: k8sretry.DefaultBackoff}

	// RunForever is a Backoff configuration that retries indefinitely.
	// It has a cap of 1 hour, meaning that once the delay between retries reaches 1 hour,
	// subsequent retries will occur hourly
	RunForever = Backoff{
		Backoff: wait.Backoff{
			Steps:    math.MaxInt,
			Duration: 250 * time.Millisecond,
			Cap:      1 * time.Hour,
			Factor:   5,
			Jitter:   0.1,
		}, RunForever: true}

	// RetryOnce is a Backoff configuration that retries only once.
	RetryOnce = Backoff{
		Backoff: wait.Backoff{
			Steps:    2,
			Duration: 50 * time.Millisecond,
			Factor:   2.0,
		}}

	// NoRetry is a Backoff configuration that do not retry
	NoRetry = Backoff{Backoff: wait.Backoff{Steps: 1}}

	// AlwaysRetry is a function that always returns true,
	// indicating that a retry should always be attempted.
	AlwaysRetry = func(err error) bool {
		return true
	}
)

type Backoff struct {
	wait.Backoff
	RunForever bool
}

// NewBackoff returns an exponential backoff used as error-handling strategy for rerunning a given function
// with the specified amount of attempts and a slightly increased timer
func NewBackoff(step int, duration time.Duration, factor, jitter float64, runForever bool) Backoff {
	return Backoff{
		Backoff: wait.Backoff{
			Steps:    step,     // the number of retries
			Duration: duration, // the sleep time before next iteration
			Factor:   factor,
			Jitter:   jitter,
		},
		RunForever: runForever,
	}
}

// NewDefaultBackoff returns a backoff with the specified number of steps and duration.
func NewDefaultBackoff(step int, duration time.Duration) Backoff {
	return Backoff{
		Backoff: wait.Backoff{
			Steps:    step,     // the number of retries
			Duration: duration, // the sleep time before next iteration
			Factor:   2.0,
			Jitter:   0.1,
		}}
}

// OnError tries to execute fn until it returns true, an error, or the context is cancelled.
// retriable will be invoked after the first interval if the context is not cancelled first.
// backoff defines the maximum retries and the wait interval between two retries.
func OnError(ctx context.Context, backoff Backoff, retriable func(error) bool, fn func() error) error {
	var lastErr error

	err := ExponentialBackoffWithContext(ctx, backoff, func(context.Context) (bool, error) {
		err := fn()
		switch {
		case err == nil:
			return true, nil
		case retriable(err):
			lastErr = err
			return false, nil
		default:
			return false, err
		}
	})
	// use lastErr only if the retriable function returns true
	// otherwise err will be nil
	if wait.Interrupted(err) && lastErr != nil {
		err = lastErr
	}
	return err
}

// OnErrorWithTimer wraps the OnError function in order to provide the steps and duration cycle
func OnErrorWithTimer(ctx context.Context, steps int, duration time.Duration, retriable func(error) bool, fn func() error) error {
	if steps == 0 {
		return fmt.Errorf("steps cannot be zero")
	}
	backoff := Backoff{
		Backoff:    wait.Backoff{Steps: steps, Duration: duration},
		RunForever: false,
	}
	return OnError(ctx, backoff,
		func(err error) bool {
			return retriable(err)
		}, func() error {
			return fn()
		})
}

// ExponentialBackoffWithContext repeats a condition check with exponential backoff.
// It immediately returns an error if the condition returns an error, the context is cancelled,
// the deadline is reached (only when RunForever is set to false), or if the maximum attempts defined in backoff is exceeded (ErrWaitTimeout).
// If an error is returned by the condition the backoff stops immediately. The condition will
// never be invoked more than backoff.Steps times.
func ExponentialBackoffWithContext(ctx context.Context, backoff Backoff, condition wait.ConditionWithContextFunc) error {
	for {
		if backoff.Steps <= 0 && !backoff.RunForever {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if ok, err := runConditionWithCrashProtectionWithContext(ctx, condition); err != nil || ok {
			return err
		}

		if backoff.Steps == 1 && !backoff.RunForever {
			break
		}

		waitBeforeRetry := backoff.Step()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitBeforeRetry):
		}
	}

	return wait.ErrWaitTimeout
}

// runConditionWithCrashProtectionWithContext runs a ConditionWithContextFunc with crash protection.
func runConditionWithCrashProtectionWithContext(ctx context.Context, condition wait.ConditionWithContextFunc) (bool, error) {
	defer runtime.HandleCrash()
	return condition(ctx)
}
