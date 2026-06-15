package graceful

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var (
	errImmediate  = errors.New("immediate error")
	errAfterDelay = errors.New("after delay error")
)

type testCase struct {
	name     string
	ctx      context.Context
	do       func(ctx context.Context) error
	expected error
}

func TestRunGracefully(t *testing.T) {
	testCases := []testCase{
		{
			name: "Hanging do function with Ctrl+C",
			ctx:  t.Context(),
			do: func(ctx context.Context) error {
				var errCh = make(chan error)
				go func() {
					time.Sleep(1 * time.Second)
					p, err := os.FindProcess(os.Getpid())
					if err != nil {
						errCh <- fmt.Errorf("failed to find process: %w", err)
					}
					_ = p.Signal(os.Interrupt) // ignore error, test context
				}()
				err := <-errCh
				return err
			},
			expected: context.DeadlineExceeded,
		},
		{
			name: "Respecting canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				return ctx
			}(),
			do: func(ctx context.Context) error {
				select {
				case <-time.After(1 * time.Second):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			expected: context.Canceled,
		},
		{
			name: "Returning an error",
			ctx:  t.Context(),
			do: func(ctx context.Context) error {
				return errors.New("expected error") //nolint:err113
			},
			expected: errors.New("expected error"),
		},
		{
			name: "Stopping successfully after Ctrl+C",
			ctx:  t.Context(),
			do: func(ctx context.Context) error {
				var errCh = make(chan error)
				go func() {
					time.Sleep(100 * time.Millisecond)
					p, err := os.FindProcess(os.Getpid())
					if err != nil {
						errCh <- fmt.Errorf("failed to find process: %w", err)
					}
					_ = p.Signal(os.Interrupt) // ignore error, test context
				}()
				select {
				case err := <-errCh:
					return err
				case <-ctx.Done():
					return nil
				}
			},
			expected: nil,
		},
		{
			name: "Panic in do function is caught",
			ctx:  t.Context(),
			do: func(ctx context.Context) error {
				panic("crash")
			},
			expected: fmt.Errorf("%w: %v", ErrPanicRecovered, "crash"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.ctx, tc.do, RunOptions{ShutdownTimeout: 1 * time.Second})
			if !errors.Is(err, tc.expected) && (err == nil || tc.expected == nil || err.Error() != tc.expected.Error()) {
				t.Errorf("expected %v, got %v", tc.expected, err)
			}
		})
	}
}

func TestRunBackground(t *testing.T) {
	type args struct {
		ctx context.Context
		do  func(ctx context.Context) error
		opt RunAllOptions
	}
	tests := []struct {
		name     string
		args     args
		expected error
	}{
		{
			name: "do returns nil",
			args: args{
				ctx: t.Context(),
				do: func(ctx context.Context) error {
					return nil
				},
				opt: RunAllOptions{
					FailFast:        true,
					ShutdownTimeout: 1 * time.Second,
				},
			},
			expected: nil,
		},
		{
			name: "do returns error",
			args: args{
				ctx: t.Context(),
				do: func(ctx context.Context) error {
					return errors.New("background error")
				},
				opt: RunAllOptions{
					FailFast:        false,
					ShutdownTimeout: 1 * time.Second,
				},
			},
			expected: errors.New("background error"),
		},
		{
			name: "context canceled before do returns",
			args: args{
				ctx: func() context.Context {
					ctx, cancel := context.WithCancel(t.Context())
					cancel()
					return ctx
				}(),
				do: func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(2 * time.Second):
						return nil
					}
				},
				opt: RunAllOptions{
					FailFast:        false,
					ShutdownTimeout: 1 * time.Second,
				},
			},
			expected: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := <-RunBackground(tt.args.ctx, tt.args.do, tt.args.opt)
			if !errors.Is(err, tt.expected) && (err == nil || tt.expected == nil || err.Error() != tt.expected.Error()) {
				t.Errorf("RunBackground() error = %v, want %v", err, tt.expected)
			}
		})
	}
}

func TestRunGracefullyTripleNested(t *testing.T) {
	tests := []struct {
		name     string
		do       func(ctx context.Context) error
		expected error
	}{
		{
			name: "success",
			do: func(ctx context.Context) error {
				return Run(ctx, func(ctx context.Context) error {
					return Run(ctx, func(ctx context.Context) error {
						return nil
					}, NewRunOptions(1*time.Second))
				})
			},
			expected: nil,
		},
		{
			name: "innermost error",
			do: func(ctx context.Context) error {
				return Run(ctx, func(ctx context.Context) error {
					return Run(ctx, func(ctx context.Context) error {
						return errors.New("innermost error")
					})
				})
			},
			expected: errors.New("innermost error"),
		},
		{
			name: "middle error",
			do: func(ctx context.Context) error {
				return Run(ctx, func(ctx context.Context) error {
					return errors.New("middle error")
				})
			},
			expected: errors.New("middle error"),
		},
		{
			name: "outer error",
			do: func(ctx context.Context) error {
				return errors.New("outer error")
			},
			expected: errors.New("outer error"),
		},
		{
			name: "innermost panic",
			do: func(ctx context.Context) error {
				return Run(ctx, func(ctx context.Context) error {
					return Run(ctx, func(ctx context.Context) error {
						panic("innermost panic")
					})
				})
			},
			expected: ErrPanicRecovered,
		},
		{
			name: "middle panic",
			do: func(ctx context.Context) error {
				return Run(ctx, func(ctx context.Context) error {
					panic("middle panic")
				})
			},
			expected: ErrPanicRecovered,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			err := Run(ctx, tt.do)
			if tt.expected == nil {
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
			} else if errors.Is(tt.expected, ErrPanicRecovered) {
				if !errors.Is(err, ErrPanicRecovered) {
					t.Errorf("expected panic recovery error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.expected.Error() {
					t.Errorf("expected %v, got %v", tt.expected, err)
				}
			}
		})
	}
}

func TestRunT_Generic(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	type testCase[T comparable] struct {
		name        string
		do          func(ctx context.Context) T
		expected    T
		errExpected error // nil, ErrPanicRecovered, or any error
	}

	intTests := []testCase[int]{
		{
			name:        "int success",
			do:          func(ctx context.Context) int { return 42 },
			expected:    42,
			errExpected: nil,
		},
		{
			name:        "int panic",
			do:          func(ctx context.Context) int { panic("test panic") },
			expected:    0,
			errExpected: ErrPanicRecovered,
		},
	}

	personTests := []testCase[Person]{
		{
			name:        "person success",
			do:          func(ctx context.Context) Person { return Person{Name: "Alice", Age: 30} },
			expected:    Person{Name: "Alice", Age: 30},
			errExpected: nil,
		},
		{
			name:        "person panic",
			do:          func(ctx context.Context) Person { panic("test panic") },
			expected:    Person{},
			errExpected: ErrPanicRecovered,
		},
	}

	for _, tt := range intTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			val, err := RunT(ctx, tt.do)
			if tt.errExpected == nil && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
			if errors.Is(tt.errExpected, ErrPanicRecovered) && !errors.Is(err, ErrPanicRecovered) {
				t.Errorf("expected panic recovery error, got %v", err)
			}
			if tt.errExpected != nil && !errors.Is(tt.errExpected, ErrPanicRecovered) {
				if err == nil || err.Error() != tt.errExpected.Error() {
					t.Errorf("expected %v, got %v", tt.errExpected, err)
				}
			}
			if val != tt.expected {
				t.Errorf("expected value %v, got %v", tt.expected, val)
			}
		})
	}

	for _, tt := range personTests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			val, err := RunT(ctx, tt.do)
			if tt.errExpected == nil && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
			if errors.Is(tt.errExpected, ErrPanicRecovered) && !errors.Is(err, ErrPanicRecovered) {
				t.Errorf("expected panic recovery error, got %v", err)
			}
			if tt.errExpected != nil && !errors.Is(tt.errExpected, ErrPanicRecovered) {
				if err == nil || err.Error() != tt.errExpected.Error() {
					t.Errorf("expected %v, got %v", tt.errExpected, err)
				}
			}
			if val != tt.expected {
				t.Errorf("expected value %v, got %v", tt.expected, val)
			}
		})
	}
}

func TestRunT_SuccessAndPanic(t *testing.T) {
	ctx := t.Context()
	val, err := RunT(ctx, func(ctx context.Context) int { return 7 })
	if err != nil || val != 7 {
		t.Errorf("expected 7, got %v, err %v", val, err)
	}
	_, err = RunT(ctx, func(ctx context.Context) int { panic("fail") })
	if err == nil || !errors.Is(err, ErrPanicRecovered) {
		t.Error("expected panic recovery error")
	}
}

func TestRunBackgroundSimple(t *testing.T) {
	ctx := t.Context()
	ch := RunBackground(ctx, func(ctx context.Context) error {
		return nil
	})
	if err := <-ch; err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestRunAllBackgroundFunc(t *testing.T) {
	ctx := t.Context()
	var calls int32
	fn := func(ctx context.Context) <-chan error {
		ch := make(chan error, 1)
		atomic.AddInt32(&calls, 1)
		ch <- nil
		return ch
	}
	ch := RunAllBackgroundFunc(ctx, []func(ctx context.Context) <-chan error{fn, fn}, FailFast)
	count := 0
	for err := range ch {
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 results for all-nil, got %d", count)
	}

	// Now test with errors
	errFn := func(ctx context.Context) <-chan error {
		ch := make(chan error, 1)
		ch <- errors.New("fail")
		return ch
	}
	errCh := RunAllBackgroundFunc(ctx, []func(context.Context) <-chan error{errFn, errFn}, FailFast)
	errCount := 0
	for err := range errCh {
		if err == nil || err.Error() != "fail" {
			t.Errorf("expected 'fail', got %v", err)
		}
		errCount++
	}
	if errCount != 2 {
		t.Errorf("expected 2 error results, got %d", errCount)
	}
}

func TestRunAllBackgroundFunc_PanicRecovery(t *testing.T) {
	ctx := t.Context()

	panicFn := func(ctx context.Context) <-chan error {
		panic("background panic!")
	}
	ch := RunAllBackgroundFunc(ctx, []func(context.Context) <-chan error{panicFn}, FailFast)
	var found bool
	for err := range ch {
		if err != nil && errors.Is(err, ErrPanicRecovered) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one ErrPanicRecovered, got none")
	}
}

func TestRunAllBackgroundFunc_WithOpts(t *testing.T) {
	opts := NewRunAllOptions(false, 1*time.Second)

	failFastFunc := func(ctx context.Context) <-chan error {
		return RunBackground(ctx, func(ctx context.Context) error {
			return errImmediate
		}, opts)
	}
	blockFunc := func(ctx context.Context) <-chan error {
		return RunBackground(ctx, func(ctx context.Context) error {
			select {
			case <-time.After(2 * time.Second):
				return errAfterDelay
			case <-ctx.Done():
				return context.Canceled
			}
		}, opts)
	}
	panicFunc := func(ctx context.Context) <-chan error {
		return RunBackground(ctx, func(ctx context.Context) error {
			panic("recoverable panic")
		}, opts)
	}

	type testCase struct {
		name     string
		buildFns []func(context.Context) <-chan error
		opts     RunAllOptions
		errs     []error
	}

	tests := []testCase{
		{
			name: "fail-fast-and-blocking",
			buildFns: []func(context.Context) <-chan error{
				failFastFunc,
				blockFunc,
			},
			opts: FailFast,
			errs: []error{errImmediate, context.Canceled},
		},
		{
			name: "panic-and-fail-fast-func",
			buildFns: []func(context.Context) <-chan error{
				failFastFunc,
				panicFunc,
			},
			opts: NewRunAllOptions(false, 3*time.Second),
			errs: []error{errImmediate, ErrPanicRecovered},
		},
		{
			name: "block-and-fail-fast",
			buildFns: []func(context.Context) <-chan error{
				blockFunc,
				failFastFunc,
			},
			opts: NewRunAllOptions(true, 3*time.Second),
			errs: []error{context.Canceled, errImmediate},
		},
		{
			name: "wait-and-not-fail-fast",
			buildFns: []func(context.Context) <-chan error{
				func(ctx context.Context) <-chan error {
					return RunBackground(ctx, func(ctx context.Context) error {
						select {
						case <-time.After(2 * time.Second):
							return errAfterDelay
						case <-ctx.Done():
							return context.Canceled
						}
					}, NewRunAllOptions(false, 3*time.Second))
				},
				func(ctx context.Context) <-chan error {
					return RunBackground(ctx, func(ctx context.Context) error {
						return errImmediate
					}, NewRunAllOptions(false, 3*time.Second))
				},
			},
			opts: NewRunAllOptions(false, 3*time.Second),
			errs: []error{errImmediate, errAfterDelay},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errChan := RunAllBackgroundFunc(runCtx, tc.buildFns, tc.opts)

			var err error
			for {
				select {
				case errCh, ok := <-errChan:
					if !ok {
						// Channel closed, check joined error
						for _, expectedErr := range tc.errs {
							if !errors.Is(err, expectedErr) {
								t.Errorf("expected combined error to include '%v', got '%v'", expectedErr, err)
							}
						}
						return
					}
					if errCh != nil {
						err = errors.Join(err, errCh)
					}
				case <-runCtx.Done():
					return
				}
			}
		})
	}
}

func TestRunAllBackground_WithOpts(t *testing.T) {

	failFastFunc := func(ctx context.Context) error {
		return errImmediate
	}

	blockFunc := func(ctx context.Context) error {
		select {
		case <-time.After(1 * time.Second):
			return nil
		case <-ctx.Done():
			return context.Canceled
		}
	}

	panicFunc := func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		panic("recoverable panic")
	}

	type testCase struct {
		name     string
		buildFns []func(context.Context) error
		opts     RunAllOptions
		errs     []error
	}

	tests := []testCase{
		{
			name: "fail-fast-and-blocking",
			buildFns: []func(context.Context) error{
				failFastFunc,
				blockFunc,
			},
			opts: FailFast,
			errs: []error{errImmediate, context.Canceled},
		},
		{
			name: "panic-and-fail-fast",
			buildFns: []func(context.Context) error{
				panicFunc,
				failFastFunc,
			},
			opts: NewRunAllOptions(false, 2*time.Second),
			errs: []error{errImmediate, ErrPanicRecovered},
		},
		{
			name: "wait-and-not-fail-fast",
			buildFns: []func(context.Context) error{
				func(ctx context.Context) error {
					select {
					case <-time.After(2 * time.Second):
						return errAfterDelay
					case <-ctx.Done():
						return nil
					}
				},
				func(ctx context.Context) error {
					return errImmediate
				},
			},
			opts: NewRunAllOptions(false, 1*time.Second),
			errs: []error{errImmediate, errAfterDelay},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			runCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			errChan := RunAllBackground(runCtx, tc.buildFns, tc.opts)

			var err error
			for {
				select {
				case errCh, ok := <-errChan:
					if !ok {
						for _, expectedErr := range tc.errs {
							if !errors.Is(err, expectedErr) {
								t.Errorf("expected combined error to include %v, got %v", expectedErr, err)
							}
						}
						return
					}
					if errCh == nil {
						t.Logf("received error: %v", errCh)
					}
					err = errors.Join(err, errCh)

				case <-runCtx.Done():
					t.Fatal("timeout: blocking function was not cancelled in time or channel did not close")
				}
			}
		})
	}
}

func TestRunAllBackgroundFuncE_AllSuccess(t *testing.T) {
	ctx := t.Context()
	doFn := []func(context.Context) <-chan error{
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- nil
			close(ch)
			return ch
		},
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- nil
			close(ch)
			return ch
		},
	}
	err := RunAllBackgroundFuncE(ctx, doFn)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestRunAllBackgroundFuncE_SomeErrors(t *testing.T) {
	ctx := t.Context()
	err1 := errors.New("error 1")
	doFn := []func(context.Context) <-chan error{
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- err1
			close(ch)
			return ch
		},
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- nil
			close(ch)
			return ch
		},
	}
	err := RunAllBackgroundFuncE(ctx, doFn)
	if err == nil || !errors.Is(err, err1) {
		t.Errorf("expected error containing %v, got %v", err1, err)
	}
}

func TestRunAllBackgroundFuncE_AllErrors(t *testing.T) {
	ctx := t.Context()
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	doFn := []func(context.Context) <-chan error{
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- err1
			close(ch)
			return ch
		},
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			ch <- err2
			close(ch)
			return ch
		},
	}
	err := RunAllBackgroundFuncE(ctx, doFn)
	if err == nil || !(errors.Is(err, err1) && errors.Is(err, err2)) {
		t.Errorf("expected error containing %v and %v, got %v", err1, err2, err)
	}
}

func TestRunAllBackgroundFuncE_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	doFn := []func(context.Context) <-chan error{
		func(ctx context.Context) <-chan error {
			ch := make(chan error, 1)
			time.Sleep(10 * time.Millisecond)
			ch <- nil
			close(ch)
			return ch
		},
	}
	cancel() // Cancel before running
	err := RunAllBackgroundFuncE(ctx, doFn)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRunAll(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	tests := []struct {
		name    string
		doFns   []func(context.Context) error
		ctx     context.Context
		expects []error
	}{
		{
			name: "all succeed",
			doFns: []func(context.Context) error{
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			},
			ctx:     t.Context(),
			expects: nil,
		},
		{
			name: "some errors",
			doFns: []func(context.Context) error{
				func(ctx context.Context) error { return err1 },
				func(ctx context.Context) error { return nil },
			},
			ctx:     t.Context(),
			expects: []error{err1},
		},
		{
			name: "all errors",
			doFns: []func(context.Context) error{
				func(ctx context.Context) error { return err1 },
				func(ctx context.Context) error { return err2 },
			},
			ctx:     t.Context(),
			expects: []error{err1, err2},
		},
		{
			name: "context canceled",
			doFns: []func(context.Context) error{
				func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				},
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			}(),
			expects: []error{context.Canceled},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RunAll(tc.ctx, tc.doFns...)
			if tc.expects == nil {
				if err != nil {
					t.Errorf("expected nil, got %v", err)
				}
			} else {
				for _, expected := range tc.expects {
					if !errors.Is(err, expected) {
						t.Errorf("expected error containing %v, got %v", expected, err)
					}
				}
			}
		})
	}
}

func TestRunPeriodically_TwoTicks(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count int
	do := func(ctx context.Context) int {
		count++
		if count == 3 {
			cancel() // stop after 3 ticks
		}
		return count
	}
	ch := RunPeriodically(ctx, do, 10*time.Millisecond)
	var results []int
	for v := range ch {
		results = append(results, v)
	}
	if len(results) < 3 {
		t.Errorf("expected at least 3 ticks, got %d", len(results))
	}
	if results[0] != 1 || results[1] != 2 || results[2] != 3 {
		t.Errorf("unexpected tick values: %v", results)
	}
}
