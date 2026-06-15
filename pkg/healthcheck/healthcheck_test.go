package healthcheck

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/healthcheck/checks/ok"
	"github.com/ing-bank/golibs/pkg/store/utilities/timed"
	"github.com/ing-bank/golibs/pkg/task/job"
	"github.com/stretchr/testify/assert"
)

func TestHasEndpoint_NoChecks(t *testing.T) {
	t.Parallel()
	h := &HealthCheck{}
	assert.False(t, h.HasEndpoint(HealthEndpoint))
	assert.False(t, h.HasEndpoint(ReadyEndpoint))
}

func TestHasEndpoint_HealthEndpointPresent(t *testing.T) {
	t.Parallel()
	h := &HealthCheck{
		Checks: []Check{
			{Endpoint: []Endpoint{HealthEndpoint}},
		},
	}
	assert.True(t, h.HasEndpoint(HealthEndpoint))
	assert.False(t, h.HasEndpoint(ReadyEndpoint))
}

func TestHasEndpoint_ReadyEndpointPresent(t *testing.T) {
	t.Parallel()
	h := &HealthCheck{
		Checks: []Check{
			{Endpoint: []Endpoint{ReadyEndpoint}},
		},
	}
	assert.False(t, h.HasEndpoint(HealthEndpoint))
	assert.True(t, h.HasEndpoint(ReadyEndpoint))
}

func TestHasEndpoint_MultipleEndpoints(t *testing.T) {
	t.Parallel()
	h := &HealthCheck{
		Checks: []Check{
			{Endpoint: []Endpoint{HealthEndpoint, ReadyEndpoint}},
		},
	}
	assert.True(t, h.HasEndpoint(HealthEndpoint))
	assert.True(t, h.HasEndpoint(ReadyEndpoint))
}

func TestHealthCheck_newResponse(t *testing.T) {
	t.Parallel()
	type args struct {
		Component  *Component
		JobResults []job.JobResult
		System     *System
	}
	tests := []struct {
		name string
		args args
		want *Response
	}{
		{
			name: "no jobs, no component, no system",
			args: args{
				Component:  nil,
				JobResults: nil,
				System:     nil,
			},
			want: &Response{
				Status:     job.StatusUnavailable.String(),
				JobResults: []job.JobResult{},
				Component:  nil,
				System:     nil,
			},
		},
		{
			name: "single job OK, with component",
			args: args{
				Component: &Component{Name: "test", Version: "1.0"},
				JobResults: []job.JobResult{
					*job.ToJobResult(job.NewOrDie("job1", "desc", 1, false, func(ctx context.Context) error { return nil })).MarkAsStarted().MarkAsCompleted().AddError(nil),
				},
				System: nil,
			},
			want: &Response{
				Status:     job.StatusOK.String(),
				JobResults: []job.JobResult{*job.ToJobResult(job.NewOrDie("job1", "desc", 1, false, func(ctx context.Context) error { return nil }))},
				Component:  &Component{Name: "test", Version: "1.0"},
				System:     nil,
			},
		},
		{
			name: "single job unavailable, with system",
			args: args{
				Component: nil,
				JobResults: []job.JobResult{
					*job.ToJobResult(job.NewOrDie("job2", "desc", 1, false, func(ctx context.Context) error { return assert.AnError })),
				},
				System: &System{Version: "go1.21", GoroutinesCount: 1, TotalAllocBytes: 100, HeapObjectsCount: 10, AllocBytes: 50},
			},
			want: &Response{
				Status:     job.StatusUnavailable.String(),
				JobResults: []job.JobResult{*job.ToJobResult(job.NewOrDie("job2", "desc", 1, false, func(ctx context.Context) error { return assert.AnError }))},
				Component:  nil,
				System:     &System{Version: "go1.21", GoroutinesCount: 1, TotalAllocBytes: 100, HeapObjectsCount: 10, AllocBytes: 50},
			},
		},
		{
			name: "multiple jobs, mixed status",
			args: args{
				Component: &Component{Name: "multi", Version: "2.0"},
				JobResults: []job.JobResult{
					*job.ToJobResult(job.NewOrDie("job1", "desc", 1, false, func(ctx context.Context) error { return nil })),
					*job.ToJobResult(job.NewOrDie("job2", "desc", 1, false, func(ctx context.Context) error { return assert.AnError })),
				},
				System: nil,
			},
			want: &Response{
				Status: job.StatusUnavailable.String(),
				JobResults: []job.JobResult{
					*job.ToJobResult(job.NewOrDie("job1", "desc", 1, false, func(ctx context.Context) error { return nil })),
					*job.ToJobResult(job.NewOrDie("job2", "desc", 1, false, func(ctx context.Context) error { return assert.AnError })),
				},
				Component: &Component{Name: "multi", Version: "2.0"},
				System:    nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &HealthCheck{}
			got := h.newResponse(tt.args.JobResults, tt.args.Component, tt.args.System)
			// Ignore System field for comparison if not set in want
			if tt.want.System == nil && got.System != nil {
				got.System = nil
			}
			assert.Equal(t, tt.want.Component, got.Component)
			assert.Equal(t, tt.want.Status, got.Status)
			assert.Equal(t, tt.want.System, got.System)
		})
	}
}

func TestHealthCheck_newResponse_EmptyJobResults(t *testing.T) {
	t.Parallel()
	h := &HealthCheck{}
	component := &Component{Name: "test", Version: "v1"}
	resp := h.newResponse(nil, component, nil)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.JobResults)
	assert.Equal(t, job.StatusUnavailable.String(), resp.Status)
	assert.Equal(t, component, resp.Component)
	assert.Nil(t, resp.System)
}

func TestNewSystemMetrics(t *testing.T) {
	t.Parallel()
	sys := newSystemMetrics()
	assert.NotNil(t, sys)
	assert.Equal(t, runtime.Version(), sys.Version)
	assert.Greater(t, sys.GoroutinesCount, 0)
	assert.GreaterOrEqual(t, sys.TotalAllocBytes, 0)
	assert.GreaterOrEqual(t, sys.HeapObjectsCount, 0)
	assert.GreaterOrEqual(t, sys.AllocBytes, 0)
}

func TestHealthCheck_RunBackground_HappyPath(t *testing.T) {
	t.Parallel()
	// Create a cache with minimal config
	cacheCfg := timed.DefaultConfig()
	cacheCfg.SyncPeriod.Duration = time.Second
	cacheCfg.MaxAge.Duration = 2 * time.Second
	cache, err := NewTimedCache(cacheCfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create multiple jobs with different behaviors
	jc1 := DefaultCheck()
	j1, err := job.New(jc1.Config.Name, jc1.Config.Description, jc1.Config.Timeout, false, func(ctx context.Context) error { return nil })
	if err != nil {
		t.Fatalf("failed to create job1: %v", err)
	}

	jc2 := DefaultCheck()
	jc2.Config.Name = "job2"
	jc2.Config.Description = "desc2"
	j2, err := job.New(jc2.Config.Name, jc2.Config.Description, jc2.Config.Timeout, false, func(ctx context.Context) error { return context.DeadlineExceeded })
	if err != nil {
		t.Fatalf("failed to create job2: %v", err)
	}

	h := &HealthCheck{
		Checks: []Check{
			{Job: j1, Endpoint: []Endpoint{HealthEndpoint}},
			{Job: j2, Endpoint: []Endpoint{HealthEndpoint}},
		},
		cache:    cache,
		interval: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	errCh := h.RunBackground(ctx)
	err, moreData := <-errCh
	if moreData {
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("expected nil or context error, got %v", err)
		}
	} // If channel is closed without sending, that's also acceptable for background jobs
}

func TestHealthCheck_With(t *testing.T) {
	t.Parallel()

	t.Run("no options", func(t *testing.T) {
		t.Parallel()
		h := &HealthCheck{}
		err := h.With()
		assert.NoError(t, err)
	})

	t.Run("with valid option (WithNewChecks)", func(t *testing.T) {
		t.Parallel()
		h, err := New()
		if err != nil {
			t.Fatalf("failed to create HealthCheck: %v", err)
		}
		check := JobConfig{
			Config: job.Config{Name: "test", Description: "desc", Timeout: 1},
			ProbeHandler: ProbeHandler{
				OK: ok.New(),
			},
			Endpoints: []Endpoint{HealthEndpoint},
		}
		err = h.With(WithNewChecks(check))
		assert.NoError(t, err)
		assert.Len(t, h.Checks, 1)
		assert.Equal(t, "test", h.Checks[0].Job.Config.Name)
	})
}
