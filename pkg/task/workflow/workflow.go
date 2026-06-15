package workflow

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/retry"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/task/runnable"
	"github.com/ing-bank/golibs/pkg/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

// Workflow can be used to execute a Chain of Activity, effectively calling the Run function on each Activity
// in the list. Workflows can execute the Chain in order, or concurrently, and has options for retries.
type Workflow[T any] struct {
	Name  string
	chain Chain[T]
	mu    sync.Map
	opts  WorkflowOpts[T]
}

type WithWorkflowID[T any] func(state T) string

// WorkflowOpts describes the retry options for each Activity
type WorkflowOpts[T any] struct {
	Backoff       retry.Backoff                                              // Required value when set, don't set zero otherwise no tries at all
	OnStepFailure func(ctx context.Context, step string, state T, err error) // TODO: make this an Activity?
	WorkflowID    WithWorkflowID[T]                                          // a unique identifier to correlate with a running workflow
	IsRetriable   func(err error) bool                                       // checks if an error is retriable
	EnableLogging bool                                                       // enable logging for each step in the workflow
	SkipStateLog  bool                                                       // skip logging the state for each activity in the workflow. Logging must be enabled for this to have effect.
}

// WithRetryPolicy sets the backoff policy used for the retry
func (w *Workflow[T]) WithRetryPolicy(backoff retry.Backoff) *Workflow[T] {
	w.opts.Backoff = backoff
	return w
}

// WithOnStepFailure defines the callback function in case of failure
func (w *Workflow[T]) WithOnStepFailure(callback func(ctx context.Context, step string, state T, err error)) *Workflow[T] {
	w.opts.OnStepFailure = callback
	return w
}

// WithWorkflowID sets the workflow ID used for canceling
func (w *Workflow[T]) WithWorkflowID(id WithWorkflowID[T]) *Workflow[T] {
	w.opts.WorkflowID = id
	return w
}

// WithIsRetriable sets the retriable func
func (w *Workflow[T]) WithIsRetriable(retriable func(err error) bool) *Workflow[T] {
	w.opts.IsRetriable = retriable
	return w
}

// WithEnableLogging sets the flag to enable logging for each step in the workflow
func (w *Workflow[T]) WithEnableLogging(enable bool) *Workflow[T] {
	w.opts.EnableLogging = enable
	return w
}

// WithSkipStateLog sets the flag to skip logging the state for each activity in the workflow. Logging must be enabled for this to have effect.
func (w *Workflow[T]) WithSkipStateLog(skip bool) *Workflow[T] {
	w.opts.SkipStateLog = skip
	return w
}

func NewWorkflow[T any](name string, chain Chain[T]) *Workflow[T] {
	return &Workflow[T]{name, chain, sync.Map{}, WorkflowOpts[T]{
		Backoff:       retry.DefaultBackoff,
		OnStepFailure: nil,
		WorkflowID: func(state T) string {
			return xid.New().String()
		},
		IsRetriable: func(_ error) bool {
			return true
		},
	}}
}

// NewWorkflowFor creates a workflow that executes an Activity with `do` for every item with retry RunForever.
// E.g.
//
//	NewWorkflowFor("example", []int{1,2,3}, func(ctx context.Context, item int, state int) error {
//	  return item + state < 10 // Item would range over provided list, state yet to be provided
//	})
func NewWorkflowFor[T, S any](name string, items []S, do func(ctx context.Context, item S, state T) error) *Workflow[T] {
	chain := Chain[T]{}
	for _, item := range items {
		chain = append(chain, NewActivity(name, func(ctx context.Context, state T) error {
			return do(ctx, item, state)
		}))
	}
	return NewWorkflow(name, chain)
}

// register stores a context cancel function to handle the execute calls
func (w *Workflow[T]) register(id string, cancelFunc context.CancelFunc) {
	w.mu.Store(id, cancelFunc)
}

// Stop call the cancelFunc to abort a running workflow
func (w *Workflow[T]) Stop(id string) {
	if cancel, ok := w.mu.Load(id); ok {
		cancel.(context.CancelFunc)()
	}
}

// Finished checks if the corresponding workflow has finished
func (w *Workflow[T]) Finished(id string) bool {
	_, ok := w.mu.Load(id)
	return !ok
}

// reset cleans up the context cancel function
func (w *Workflow[T]) unregister(id string) {
	w.mu.Delete(id)
}

// Chain returns a chain of activities
func (w *Workflow[T]) Chain() Chain[T] {
	return w.chain
}

// Execute calls the list of Activity in the Chain in-order and retries an Activity on error.
// Blocks until all Activity complete, or when context is cancelled.
func (w *Workflow[T]) Execute(ctx context.Context, state T) error {
	ctx, span := trace.NewSpan(ctx, w.Name)
	defer span.End()

	labels := prometheus.Labels{"workflow": w.Name, "concurrent": "false"}
	workflowExecuteStarted.With(labels).Inc()
	start := time.Now()

	id := w.opts.WorkflowID(state)
	ctx, cancelFunc := context.WithCancel(ctx)
	w.register(id, cancelFunc)
	defer w.unregister(id)

	if w.opts.EnableLogging {
		log.WithContext(ctx).Infof("[Workflow] [%s] Starting workflow with activities: %s", w.Name, w.chain.Describe())
	}

	var err error
	for _, activity := range w.chain {
		if err = w.executeActivity(ctx, state, activity); err != nil {
			if w.opts.EnableLogging {
				log.WithContext(ctx).Errorf("[Workflow] [%s] Workflow %s failed at activity: %s", w.Name, activity.Name(), err)
			}
			break
		}
	}

	// Metrics
	dur := time.Since(start).Seconds()
	workflowExecuteDuration.With(labels).Observe(dur)
	if err != nil {
		workflowExecuteFailed.With(labels).Inc()
		return err
	}
	workflowExecuteSucceeded.With(labels).Inc()

	if w.opts.EnableLogging {
		log.WithContext(ctx).Infof("[Workflow] [%s] Workflow completed successfully", w.Name)
	}
	return nil
}

// ExecuteConcurrent calls all Activity in the Chain concurrently and retries an Activity on error.
// Blocks until all Activity complete, or when context is cancelled.
func (w *Workflow[T]) ExecuteConcurrent(ctx context.Context, state T) []error {
	ctx, span := trace.NewSpan(ctx, w.Name)
	defer span.End()

	labels := prometheus.Labels{"workflow": w.Name, "concurrent": "true"}
	workflowExecuteStarted.With(labels).Inc()
	start := time.Now()

	id := w.opts.WorkflowID(state)
	ctx, cancelFunc := context.WithCancel(ctx)
	w.register(id, cancelFunc)
	defer w.unregister(id)

	if w.opts.EnableLogging {
		log.WithContext(ctx).Infof("[Workflow] [%s] Starting concurrent workflow: %s with activities", w.Name, w.chain.Describe())
	}

	var tasks []runnable.Runnable
	for _, activity := range w.chain {
		tasks = append(tasks, runnable.NewRunnable(func(ctx context.Context) error {
			return w.executeActivity(ctx, state, activity)
		}))
	}
	errs := runnable.Run(tasks, ctx)
	filteredErrs := slices.Filter(errs, func(item error) bool {
		return item != nil
	})

	// Metrics
	dur := time.Since(start).Seconds()
	workflowExecuteDuration.With(labels).Observe(dur)
	if len(filteredErrs) > 0 {
		workflowExecuteFailed.With(labels).Inc()
	} else {
		workflowExecuteSucceeded.With(labels).Inc()
	}

	// Logging
	if w.opts.EnableLogging {
		if len(filteredErrs) == 0 {
			log.WithContext(ctx).Infof("[Workflow] [%s] Concurrent workflow completed successfully", w.Name)
		}
		for i, err := range errs {
			if err != nil {
				log.WithContext(ctx).Errorf("[Workflow] [%s] Concurrent workflow failed at activity %s: %v", w.Name, w.chain[i].Name(), err)
			}
		}
	}

	return filteredErrs
}

func (w *Workflow[T]) executeActivity(ctx context.Context, state T, activity Activity[T]) error {
	retriable := func(err error) bool {
		if err == nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if w.opts.OnStepFailure != nil {
			w.opts.OnStepFailure(ctx, activity.Name(), state, err)
		}
		return w.opts.IsRetriable(err)
	}

	err := retry.OnError(ctx, w.opts.Backoff, retriable, func() error {
		workerChan := make(chan error, 1)
		go w.run(ctx, activity, state, workerChan)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-workerChan: // Task is finished
			return err
		}
	})
	if err != nil {
		return fmt.Errorf("%s: %w", activity.Name(), err)
	}
	return err
}

func (w *Workflow[T]) run(ctx context.Context, activity Activity[T], state T, workerChan chan<- error) {
	ctx, span := trace.NewSpan(ctx, activity.Name())
	defer span.End()

	// Metrics
	labels := prometheus.Labels{"workflow": w.Name, "activity": activity.Name()}
	workflowActivityStarted.With(labels).Inc()
	start := time.Now()

	if w.opts.EnableLogging {
		if w.opts.SkipStateLog {
			log.WithContext(ctx).Infof("[Workflow] [%s] Starting workflow with activities: %s", w.Name, activity.Name())
		} else {
			log.WithContext(ctx).WithField("state", state).Infof("[Workflow] [%s] Starting activity: %s", w.Name, activity.Name())
		}
	}

	defer func() {
		dur := time.Since(start).Seconds()
		workflowActivityDuration.With(labels).Observe(dur)
		if err := recover(); err != nil {
			log.WithContext(ctx).Printf("[Workflow] [%s] [CRITICAL] Recovering from exception in task: %v %s\n", w.Name, err, string(debug.Stack()))
			workflowActivityFailed.With(labels).Inc()
			workerChan <- errors.ErrInternalServerError
			return
		}
	}()

	// Execute
	err := activity.Run(ctx, state)

	if w.opts.EnableLogging {
		if err != nil {
			log.WithContext(ctx).Errorf("[Workflow] [%s] Activity %s failed: %v", w.Name, activity.Name(), err)
		} else {
			log.WithContext(ctx).Infof("[Workflow] [%s] Activity %s succeeded", w.Name, activity.Name())
		}
	}

	// Metrics
	if err != nil {
		workflowActivityFailed.With(labels).Inc()
	} else {
		workflowActivitySucceeded.With(labels).Inc()
	}

	workerChan <- err
}

// AsActivity converts the workflow to an Activity to allow for nesting of Workflow
func (w *Workflow[T]) AsActivity() Activity[T] {
	return NewActivity(w.Name, w.Execute)
}
