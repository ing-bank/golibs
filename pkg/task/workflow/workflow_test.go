package workflow

import (
	"context"

	"github.com/ing-bank/golibs/pkg/retry"
	"github.com/ing-bank/golibs/pkg/task/runnable"

	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Test utilities ---

var FakeError = fmt.Errorf("fake error")

type State struct {
	sync.Mutex
	counter int
}

func NewState() *State {
	return &State{Mutex: sync.Mutex{}, counter: 0}
}

type FakeActivity struct {
	name string
}

func (a FakeActivity) Name() string {
	return a.name
}

type LongRequest struct{}

func (a LongRequest) Name() string {
	return "LongRequest"
}

func (a LongRequest) Run(ctx context.Context, _ *FakeRequest) error {
	return FakeError
}

type FakeRequest struct {
	TraceID string
}

func (a FakeActivity) Run(_ context.Context, state *State) error {
	time.Sleep(50 * time.Millisecond)
	state.Lock()
	defer state.Unlock()
	state.counter++
	if state.counter%3 == 2 {
		return fmt.Errorf("mod 3")
	}
	return nil
}

// --- Test cases below ---

func TestWorkflow_Execute(t *testing.T) {
	c := Chain[*State]{FakeActivity{"a"}, FakeActivity{"b"}, FakeActivity{"c"}}

	state := NewState()
	w := NewWorkflow("TestWorkflow", c)

	start := time.Now()
	err := w.Execute(t.Context(), state)
	elapsedMs := time.Now().Sub(start).Milliseconds()

	if err != nil {
		t.Fatalf("expected no err but got: %v", err)
	}
	if state.counter != (3 + 1) { // 3 runs, one retry
		t.Fatalf("expected state to be 4, but got %d", state.counter)
	}
	if elapsedMs < 200 || elapsedMs > 250 { // 3x 50ms + 1 retry 50ms
		t.Errorf("expected roughly 150ms to be elasped, but got %d", elapsedMs)
	}
}

func TestWorkflow_ExecuteConcurrent(t *testing.T) {
	c := Chain[*State]{FakeActivity{"a"}, FakeActivity{"b"}, FakeActivity{"c"}}

	state := NewState()
	w := NewWorkflow("TestWorkflow", c)

	start := time.Now()
	errs := w.ExecuteConcurrent(t.Context(), state)
	elapsedMs := time.Now().Sub(start).Milliseconds()

	if runnable.AnyError(errs) {
		t.Fatalf("expected no err but got: %v", errs)
	}
	if state.counter != (3 + 1) { // 3 runs, one retry
		t.Fatalf("expected state to be 4, but got %d", state.counter)
	}
	if elapsedMs < 100 || elapsedMs > 150 { // All concurrent (50ms), but one retry (+50ms)
		t.Errorf("expected roughly 50ms to be elasped, but got %d", elapsedMs)
	}
}

func TestWorkflow_Abort(t *testing.T) {
	weave := NewActivity("post", func(ctx context.Context, f *FakeRequest) error {
		return nil
	})
	c := NewChain[*FakeRequest](LongRequest{}).Weave(weave)

	wf := NewWorkflow("TestWorkflow", c).
		WithRetryPolicy(retry.RunForever).
		WithWorkflowID(func(state *FakeRequest) string {
			return state.TraceID
		})

	// abort after few seconds
	go func(wf *Workflow[*FakeRequest]) {
		time.Sleep(time.Second * 1)
		wf.Stop("0007")
		time.Sleep(time.Millisecond * 500)
		wf.Stop("0007")
		time.Sleep(time.Millisecond * 500)
		wf.Stop("0008")
	}(wf)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	req := &FakeRequest{TraceID: "0007"}

	start := time.Now()
	if err := wf.Execute(ctx, req); !errors.Is(err, FakeError) {
		t.Fatalf("expected '%s' but got: '%s'", FakeError, err)
	}
	elapsed := time.Now().Sub(start).Seconds()
	if elapsed > 1.2 {
		t.Errorf("unexpected elapsed time: %v", elapsed)
	}

	// ensure the request ID can be canceled twice
	if err := wf.Execute(ctx, req); !errors.Is(err, FakeError) {
		t.Fatalf("expected '%s' but got: '%s'", FakeError, err)
	}
	req2 := &FakeRequest{TraceID: "0008"}
	if err := wf.Execute(ctx, req2); !errors.Is(err, FakeError) {
		t.Fatalf("expected '%s' but got: '%s'", FakeError, err)
	}

	elapsed = time.Now().Sub(start).Seconds()
	if elapsed > 2.1 {
		t.Errorf("unexpected elapsed time: %v", elapsed)
	}
}
