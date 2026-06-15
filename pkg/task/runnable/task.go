// Package task contains functions that strive to make it easier to schedule goroutines
// and gather the (error) results. The main focus is that regardless of whether a goroutine
// finishes in time, or successfully, there is a traceable result. That means it is
// easy to track which goroutines finished, and also those that did not (successfully).
//
// The core of this package is a Runnable. The error status of a Runnable implementation
// is set automatically by the return value, and they are aggregated by the Run function.
// A Runnable implementation only takes a context as an argument, so any other parameters
// should be kept inside your struct.
//
// Take the following example:
//
//	type Task struct { ... } // Your concurrent task
//	func (task Task) Run(ctx context.Context) error { ... } // Implement Runnable
//
//	func example() {
//	    tasks := []Runnable{ &Task{ ... }, ...} // Tasks you wish to be executed concurrently
//	    errs := Run(tasks, context.TODO()) // Run 'tasks' concurrently, context will be passed to each task
//	    ... // Do something with errors, probably
//	}
package runnable

import (
	"context"
	"errors"
	"runtime/debug"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Runnable structs can be passed to the Run function below, for context aware concurrent execution.
type Runnable interface {
	Run(context.Context) error
}

var _ Runnable = &DynamicRunnable{}

// DynamicRunnable allows you to pass funcs
type DynamicRunnable struct {
	DynamicRun func(context.Context) error
}

func (r DynamicRunnable) Run(ctx context.Context) error {
	if r.DynamicRun == nil {
		return nil
	}
	return r.DynamicRun(ctx)
}

func NewRunnable(run func(ctx context.Context) error) *DynamicRunnable {
	return &DynamicRunnable{DynamicRun: run}
}

// Run runs multiple tasks until completion, using Run, or when context is done.
// Each task may return an optional error, the returned error list
// is in-order as the given task list and may contain nil values.
func Run(tasks []Runnable, ctx context.Context) []error {
	var taskErrors = make([]error, len(tasks))
	g, ctx := errgroup.WithContext(ctx)
	for i, task := range tasks {
		g.Go(func() error {
			errCh := make(chan error, 1)
			go func(t Runnable) {
				defer func() {
					if err := recover(); err != nil {
						log.Errorf("[CRITICAL] Recovering from exception in task: %v %s", err, string(debug.Stack()))
						errCh <- errors.New("internal server error")
					}
					defer close(errCh)
				}()
				errCh <- t.Run(ctx)
			}(task)
			select {
			case err := <-errCh:
				taskErrors[i] = err // Task is finished
			case <-ctx.Done():
				taskErrors[i] = errors.New("timeout")
				return ctx.Err()
			}
			return nil
		})
	}
	_ = g.Wait()

	return taskErrors
}

// AnyError returns true when there is any non-nil value in the list of errors provided, otherwise false
func AnyError(errors []error) bool {
	for _, err := range errors {
		if err != nil {
			return true
		}
	}
	return false
}
