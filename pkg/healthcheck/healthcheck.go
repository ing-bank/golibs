package healthcheck

import (
	"context"
	"fmt"
	"math"
	"runtime"
	slices0 "slices"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/graceful"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks/http"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks/ok"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks/telnet"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
	timed2 "github.com/ing-bank/golibs/pkg/store/utilities/timed"
	"github.com/ing-bank/golibs/pkg/task/job"
)

var _ fmt.Stringer = (*Endpoint)(nil)

// Endpoint is a string type
type Endpoint string

func (e Endpoint) String() string {
	return string(e)
}

const (
	RootEndpoint   Endpoint = "/"
	HealthEndpoint Endpoint = "/healthz"
	ReadyEndpoint  Endpoint = "/readyz"
	StatusEndpoint Endpoint = "/status"
)

// Check represents the health check response.
type Check struct {
	Job      job.Job
	Endpoint []Endpoint
}

// System runtime variables about the go process.
type System struct {
	// Version is the go version.
	Version string `json:"version"`
	// GoroutinesCount is the number of the current goroutines.
	GoroutinesCount int `json:"goroutines_count"`
	// TotalAllocBytes is the total bytes allocated.
	TotalAllocBytes int `json:"total_alloc_bytes"`
	// HeapObjectsCount is the number of objects in the go heap.
	HeapObjectsCount int `json:"heap_objects_count"`
	// TotalAllocBytes is the bytes allocated and not yet freed.
	AllocBytes int `json:"alloc_bytes"`
}

// Component descriptive values about the component for which checks are made
type Component struct {
	Enabled bool `json:"enabled"`
	// Name is the name of the component.
	Name string `json:"name"`
	// Version is the component version.
	Version     string `json:"version"`
	Environemnt string `json:"environment,omitempty"`
}

// HealthCheck is the health-checks container
type HealthCheck struct {
	// Component holds information on the component for which checks are made
	Component *Component `json:"component,omitempty"`
	// Checks holds the registered checks.
	Checks []Check `json:"checks,omitempty"`
	// System holds information of the go process.
	System *System `json:"system,omitempty"`
	// cache holds the timed cache for job results.
	cache *timed2.Timed[int, job.JobResult]
	// Interval is the interval to run all checks.
	interval          time.Duration
	systemInfoEnabled bool
}

func (h *HealthCheck) With(opts ...Option) error {
	return config.ApplyOpts(h, opts...)
}

type Response struct {
	// Component holds information on the component for which checks are made
	Component  *Component      `json:"component,omitempty"`
	Status     string          `json:"status"`
	JobResults []job.JobResult `json:"checks,omitempty"`
	// System holds information of the go process.
	System *System `json:"system,omitempty"`
}

// New instantiates and build new health check container
func New(opts ...Option) (*HealthCheck, error) {
	cfg := DefaultConfig()
	h, err := NewForConfig(cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create health check: %w", err)
	}
	return h, nil
}

func DefaultCheck() JobConfig {
	return JobConfig{
		Config: job.Config{
			Name:        "default",
			Description: "Default health check for /healthz and /readyz endpoints; always returns 200 if server is running",
			Timeout:     1 * time.Second,
		},
		ProbeHandler: ProbeHandler{
			OK: ok.New(),
		},
		Endpoints: []Endpoint{
			HealthEndpoint,
			ReadyEndpoint,
		},
	}
}

type ProbeHandler struct {
	HTTPGet *http.Config   `json:"httpGet,omitempty"`
	OK      *ok.Config     `json:"ok,omitempty"`
	Telnet  *telnet.Config `json:"telnet,omitempty"`
}

func NewHandlerFor(c *JobConfig, opts ...Option) (checks.Handler, error) {
	if c.OK != nil {
		return ok.New(), nil
	}
	if c.HTTPGet != nil {
		return http.New(c.HTTPGet)
	}
	if c.Telnet != nil {
		return telnet.New(c.Telnet)
	}
	if c.CustomHandler != nil {
		return c.CustomHandler, nil
	}
	// could add more built-in handlers here (e.g., TCP, gRPC, etc.)
	// otherwise return an error
	return nil, fmt.Errorf("no health checks configured")
}

func NewTimedCache(cfg *timed2.Config) (*timed2.Timed[int, job.JobResult], error) {
	backend, err := store.New[int, timed2.CacheItem[job.JobResult]](memory.New, threadsafe.New)
	if err != nil {
		return nil, err
	}

	return timed2.New[int, job.JobResult](cfg, backend)
}

func NewForConfig(c *Config, opts ...Option) (*HealthCheck, error) {
	if c == nil {
		return nil, fmt.Errorf("no health check config provided")
	}
	cfg := *c // shallow copy

	// apply default values to the config
	ApplyDefaultConfig(&cfg)

	// validate the config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid health check config: %w", err)
	}

	timedCache, err := NewTimedCache(&cfg.CacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create timed cache: %w", err)
	}

	h := &HealthCheck{
		cache:             timedCache,
		interval:          cfg.Interval.Duration,
		systemInfoEnabled: cfg.SystemInfo,
		Component:         cfg.ComponentConfig,
	}
	if err := config.ApplyOpts(h, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply TLS option: %w", err)
	}

	if len(cfg.Jobs) == 0 {
		if err := h.Add(DefaultCheck()); err != nil {
			return nil, err
		}
	}

	if err := h.Add(cfg.Jobs...); err != nil {
		return nil, fmt.Errorf("could not register checks: %w", err)
	}

	return h, nil
}

// Add registers a check config to be performed.
func (h *HealthCheck) Add(jobs ...JobConfig) error {
	for _, c := range jobs {
		handler, err := NewHandlerFor(&c)
		if err != nil {
			return fmt.Errorf("could not create handler for job %q: %w", c.Name, err)
		}

		j, err := job.New(c.Name, c.Description, c.Timeout, c.MayFail, handler.Check)
		if err != nil {
			return fmt.Errorf("could not create job: %w", err)
		}

		// register a new job
		h.Checks = append(h.Checks, Check{
			Job:      j,
			Endpoint: c.Endpoints,
		})
	}
	return nil
}

func (h *HealthCheck) HasEndpoint(e Endpoint) bool {
	for _, c := range h.Checks {
		if slices0.Contains(c.Endpoint, e) {
			return true
		}
	}
	return false
}

func (h *HealthCheck) AllChecks() []job.Job {
	var jobs []job.Job
	for _, c := range h.Checks {
		jobs = append(jobs, c.Job)
	}
	return jobs
}

func (h *HealthCheck) run(ctx context.Context, jobs []job.Job, component *Component) *Response {
	jobResults := h.FetchJobResults(ctx, jobs)
	var system *System
	if h.systemInfoEnabled {
		system = newSystemMetrics()
	}
	return h.newResponse(jobResults, component, system)
}

func (h *HealthCheck) newResponse(jobResults []job.JobResult, component *Component, system *System) *Response {
	status := slices.Transform(jobResults, func(item job.JobResult) job.Availability {
		return item.GetAvailability()
	})

	return &Response{
		Status:     getStatusResponse(status),
		JobResults: jobResults,
		Component:  component,
		System:     system,
	}
}

func (h *HealthCheck) FetchJobResults(ctx context.Context, jobs []job.Job) []job.JobResult {
	var jobResults []job.JobResult
	if h.cache != nil {
		for i, j := range jobs {
			cached, err := h.cache.Read(ctx, i)
			if err == nil {
				jobResults = append(jobResults, cached)
			} else {
				jobResults = append(jobResults, *job.ToJobResult(j))
			}
		}
		return jobResults
	}
	return jobResults
}

func (h *HealthCheck) RunBackground(ctx context.Context) <-chan error {
	jobs := h.AllChecks()
	// start periodic job runner and cache purger
	return graceful.RunAllBackgroundFunc(ctx, []func(ctx context.Context) <-chan error{
		func(ctx context.Context) <-chan error {
			// run all jobs periodically and store results in cache
			// this way the HTTP handlers can return quickly with cached results
			// while the jobs are run in the background
			return graceful.RunPeriodically(ctx, func(ctx context.Context) error {
				runner := job.RunAny(ctx, jobs)
				for k, v := range runner {
					_ = h.cache.Apply(ctx, k, v)
				}
				return nil
			}, h.interval)
		},
		// run the cache purger
		func(ctx context.Context) <-chan error {
			return graceful.RunBackground(ctx, func(ctx context.Context) error {
				return h.cache.Run(ctx)
			})
		},
	})
}

func newSystemMetrics() *System {
	s := runtime.MemStats{}
	runtime.ReadMemStats(&s)

	maxInt := math.MaxInt
	var heapObjects, allocBytes int
	if s.HeapObjects > uint64(maxInt) {
		heapObjects = int(maxInt)
	} else {
		heapObjects = int(s.HeapObjects)
	}
	if s.Alloc > uint64(maxInt) {
		allocBytes = int(maxInt)
	} else {
		allocBytes = int(s.Alloc)
	}

	return &System{
		Version:          runtime.Version(),
		GoroutinesCount:  runtime.NumGoroutine(),
		TotalAllocBytes:  int(s.TotalAlloc), //nolint:gosec
		HeapObjectsCount: heapObjects,       //nolint:gosec
		AllocBytes:       allocBytes,        //nolint:gosec
	}
}

func getStatusResponse(statuses []job.Availability) string {
	switch {
	case len(statuses) == 0:
		return job.StatusUnavailable.String()
	case slices.Contains(statuses, job.StatusUnavailable):
		return job.StatusUnavailable.String()
	case slices.Contains(statuses, job.StatusPartiallyAvailable):
		return job.StatusPartiallyAvailable.String()
	case slices.Contains(statuses, job.StatusOK):
		return job.StatusOK.String()
	default:
		return job.StatusUnavailable.String()
	}
}

func AvailabilityFromString(s string) int {
	switch s {
	case job.StatusOK.String():
		return int(job.StatusOK)
	case job.StatusUnavailable.String():
		return int(job.StatusUnavailable)
	case job.StatusPartiallyAvailable.String():
		return int(job.StatusPartiallyAvailable)
	}
	return int(job.StatusUnavailable)
}

// JobsForEndpoint returns all jobs registered for the specified endpoint.
func (h *HealthCheck) JobsForEndpoint(endpoint Endpoint) []job.Job {
	jobs := []job.Job{}
	for _, c := range h.Checks {
		if slices0.Contains(c.Endpoint, endpoint) {
			jobs = append(jobs, c.Job)
		}
	}
	return jobs
}
