// Package graceful provides utilities for graceful shutdown and background task management.
//
// It enables running functions with graceful shutdown capabilities, where tasks are notified
// via context cancellation to exit cleanly. If a task doesn't exit within a timeout period,
// an error is returned. The package handles OS signals (SIGINT, SIGTERM) automatically.
//
// Main Functions:
//
//   - Run: Runs a single function with graceful shutdown on OS signals.
//   - RunAll: Runs multiple functions concurrently and waits for all to complete.
//   - RunBackground: Runs a function in a separate goroutine, returning an error channel.
//   - RunAllBackground: Runs multiple functions concurrently in separate goroutines.
//   - RunPeriodically: Runs a function repeatedly at a fixed interval.
//   - RunT: Generic version of Run that returns a value.
//
// Options:
//
//   - RunOptions: Configures shutdown timeout (default: 30 seconds).
//   - RunAllOptions: Configures fail-fast behavior and shutdown timeout.
//
// Behavior:
//
// When an OS signal (SIGINT/SIGTERM) is received:
//  1. The context passed to the function is cancelled
//  2. The function is expected to exit in response to context cancellation
//  3. If the function doesn't exit within the shutdown timeout, it returns a DeadlineExceeded error
//
// The FailFast option in RunAllOptions controls whether all tasks are stopped on first error:
//   - true (default): All goroutines are cancelled on the first error
//   - false: All goroutines run to completion and errors are joined
//
// Panic Recovery:
// All Run functions recover from panics within the executed function and return an
// ErrPanicRecovered error containing the panic value.
//
// Example usage:
//
//	// Run a single function with graceful shutdown
//	err := graceful.Run(ctx, func(ctx context.Context) error {
//		return server.ListenAndServe()
//	}, graceful.RunOptions{ShutdownTimeout: 15 * time.Second})
//
//	// Run multiple functions concurrently
//	err := graceful.RunAll(ctx,
//		func(ctx context.Context) error { return server1.Serve(ctx) },
//		func(ctx context.Context) error { return server2.Serve(ctx) },
//	)
//
//	// Run functions in background with error handling
//	errChan := graceful.RunAllBackground(ctx, []func(ctx context.Context) error{
//		func(ctx context.Context) error { return worker1.Start(ctx) },
//		func(ctx context.Context) error { return worker2.Start(ctx) },
//	})
//	for err := range errChan {
//		if err != nil {
//			log.Printf("background task error: %v", err)
//		}
//	}
package graceful

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ing-bank/golibs/pkg/opt"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 30 * time.Second
	// DefaultFailFast indicates if the background tasks should be stopped on first error
	DefaultFailFast = true
	// DefaultInterval is the default interval for periodic tasks
	DefaultInterval = 1 * time.Minute
)

var ErrPanicRecovered = fmt.Errorf("panic recovered")

// Run calls `do` until either `do` exits or a stop signal was received.
// In case of a stop signal the context given to `do` is cancelled, and it is expected
// that the `do` function exits, otherwise it times out with
// a context DeadlineExceeded error message after the provided timeout
func Run(ctx context.Context, do func(ctx context.Context) error, opts ...RunOptions) error {
	o := opt.Opt(RunOptions{DefaultShutdownTimeout}, opts)
	result, err := RunT(ctx, do, o)
	if err != nil {
		return err
	}
	return result
}

// RunAll runs all given functions in separate goroutines and waits for them to finish.
// It will stop all functions on the first error encountered.
// It collects all errors and returns them as a single error using errors.Join.
func RunAll(ctx context.Context, doFn ...func(ctx context.Context) error) error {
	var allErrs []error
	for errCh := range RunAllBackground(ctx, doFn, FailFast) {
		if errCh != nil {
			allErrs = append(allErrs, errCh)
		}
	}
	return errors.Join(allErrs...)
}

// RunBackground runs the given function in a separate goroutine and returns a channel to receive any error it produces.
func RunBackground(ctx context.Context, doFn func(ctx context.Context) error, opts ...RunAllOptions) <-chan error {
	return RunAllBackground(ctx, []func(ctx context.Context) error{doFn}, opts...)
}

func RunAllBackground(ctx context.Context, doFn []func(ctx context.Context) error, opts ...RunAllOptions) <-chan error {
	o := opt.Opt(RunAllOptions{DefaultFailFast, DefaultShutdownTimeout}, opts)
	var wg sync.WaitGroup
	wg.Add(len(doFn))
	ctx, cancel := context.WithCancel(ctx)
	errChan := make(chan error, len(doFn))

	for _, do := range doFn {
		go func(fn func(ctx context.Context) error) {
			defer wg.Done()
			if err := Run(ctx, fn, RunOptions{o.ShutdownTimeout}); err != nil {
				if o.FailFast {
					// stop all background tasks
					cancel()
				}
				errChan <- err
			}
		}(do)
	}

	go func() {
		wg.Wait()
		close(errChan)
		cancel()
	}()

	return errChan
}

func RunAllBackgroundFunc(ctx context.Context, doFn []func(ctx context.Context) <-chan error, opts ...RunAllOptions) <-chan error {
	o := opt.Opt(RunAllOptions{DefaultFailFast, DefaultShutdownTimeout}, opts)
	errChan := make(chan error, len(doFn))
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)

	wg.Add(len(doFn))
	for _, do := range doFn {
		go func(f func(ctx context.Context) <-chan error) {
			defer wg.Done()
			if err := Run(ctx, func(ctx context.Context) error {
				if err := <-f(ctx); err != nil {
					return err
				}
				return nil
			}, RunOptions{o.ShutdownTimeout}); err != nil { // max 30 seconds to shut down each function
				if o.FailFast {
					// stop all background tasks
					cancel()
				}
				errChan <- err
			}
		}(do)
	}

	go func() {
		wg.Wait()
		close(errChan)
		cancel()
	}()

	return errChan
}

func RunAllBackgroundFuncE(ctx context.Context, doFn []func(ctx context.Context) <-chan error, opts ...RunAllOptions) error {
	var allErrs []error
	for errCh := range RunAllBackgroundFunc(ctx, doFn, opts...) {
		if errCh != nil {
			allErrs = append(allErrs, errCh)
		}
	}
	return errors.Join(allErrs...)
}

func RunPeriodically[T any](ctx context.Context, do func(ctx context.Context) T, interval time.Duration) <-chan T {
	respChan := make(chan T, 1)
	_ = RunBackground(ctx, func(ctx context.Context) error {
		defer close(respChan)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if res := do(ctx); any(res) != nil {
					respChan <- res
				}
			}
		}
	})
	return respChan
}

func RunT[T any](ctx context.Context, do func(ctx context.Context) T, opts ...RunOptions) (T, error) {
	// return immediately if the context is already done
	if ctx.Err() != nil {
		var zero T
		return zero, ctx.Err()
	}

	o := opt.Opt(RunOptions{DefaultShutdownTimeout}, opts)
	ctx, stop := signal.NotifyContext(ctx,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	var err error
	var data T
	var shutdownCh = make(chan struct{})
	go func() {
		defer close(shutdownCh)
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%w: %v", ErrPanicRecovered, r)
			}
		}()
		data = do(ctx)
	}()

	select {
	case <-ctx.Done():
	case <-shutdownCh:
	}
	// stop and wait for do func to finish or end with a timeout
	stop()

	select {
	case <-shutdownCh:
	case <-time.After(o.ShutdownTimeout):
		err = fmt.Errorf("the provided `do` function did not finish within the shutdown timeout period: %w", context.DeadlineExceeded)
	}
	return data, err
}
