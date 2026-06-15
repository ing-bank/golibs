package job

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultTimeout = 5 * time.Second
	StateSuccess   = "SUCCESS"
	StateFailed    = "FAILED"
)

var (
	ErrJobAlreadyRegistered = errors.New("job already registered")
	ErrJobNotStarted        = errors.New("job not started")
	ErrJobTimeout           = errors.New("job timed out")
)

type Availability int

// Possible health statuses
const (
	StatusOK                 Availability = http.StatusOK
	StatusPartiallyAvailable Availability = http.StatusPartialContent
	StatusUnavailable        Availability = http.StatusServiceUnavailable
	StatusTimeout            Availability = http.StatusRequestTimeout
)

func (a Availability) String() string {
	return http.StatusText(int(a))
}

type Check func(ctx context.Context) error

type Config struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	MayFail     bool          `json:"mayFail,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
}

func (c *Config) Validate() error {
	if c == nil {
		return nil
	}
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (c *Config) ApplyDefaults() *Config {
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	return c
}

type Job struct {
	Config Config
	Check  Check
}

// Name returns the name of the job.
func (j *Job) Name() string {
	return j.Config.Name
}

// Validate validates the job configuration.
func (j *Job) Validate() error {
	if err := j.Config.Validate(); err != nil {
		return err
	}
	if j.Check == nil {
		return errors.New("check function is required")
	}
	return nil
}

// JobResult represents the result of a job execution.
type JobResult struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Timeout     string    `json:"timeout"`
	MayFail     bool      `json:"mayFail,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	Duration    string    `json:"duration"`
	State       string    `json:"state"`
	Error       error     `json:"error,omitempty"`
}

func New(name, description string, timeout time.Duration, mayFail bool, check Check) (Job, error) {
	cfg := &Config{
		Name:        name,
		Description: description,
		Timeout:     timeout,
		MayFail:     mayFail,
	}
	// apply default values to the config
	cfg.ApplyDefaults()
	// create the job
	job := Job{
		Config: *cfg,
		Check:  check,
	}
	// validate the job configuration
	if err := job.Validate(); err != nil {
		return Job{}, err
	}
	return job, nil
}

func NewOrDie(name, description string, timeout time.Duration, mayFail bool, check Check) Job {
	job, err := New(name, description, timeout, mayFail, check)
	if err != nil {
		panic(err)
	}
	return job
}

func (js *JobResult) AddError(err error) *JobResult {
	js.Error = err
	return js
}

func (js *JobResult) IsRunning() bool {
	return js.FinishedAt.IsZero()
}

func (js *JobResult) HasErrors() bool {
	return js.Error != nil
}

func (js *JobResult) IsSuccessful() bool {
	if js.IsRunning() {
		return false
	}
	// Otherwise, it's only successful if there are no errors
	return !js.HasErrors()
}

func (js *JobResult) GetAvailability() Availability {
	if js.IsSuccessful() {
		return StatusOK
	}
	if !js.MayFail && js.HasErrors() {
		return StatusUnavailable
	}
	return StatusPartiallyAvailable
}

func (js *JobResult) MarkAsStarted() *JobResult {
	js.StartedAt = time.Now()
	return js
}

func (js *JobResult) MarkAsCompleted() *JobResult {
	js.FinishedAt = time.Now()
	duration := js.FinishedAt.Sub(js.StartedAt)
	js.Duration = duration.String()

	if js.IsSuccessful() {
		js.State = StateSuccess
	} else {
		js.State = StateFailed
	}
	return js
}

func (js *JobResult) MarshalJSON() ([]byte, error) {
	type Alias JobResult
	return json.Marshal(&struct {
		*Alias
		Error string `json:"error,omitempty"`
	}{
		Alias: (*Alias)(js),
		Error: func() string {
			if js.Error != nil {
				return js.Error.Error()
			}
			return ""
		}(),
	})
}

// ToJobResult converts a Job to a JobResult with initial values.
func ToJobResult(job Job) *JobResult {
	return &JobResult{
		ID:          uuid.New(),
		Name:        job.Config.Name,
		Description: job.Config.Description,
		Timeout:     job.Config.Timeout.String(),
		MayFail:     job.Config.MayFail,
		Error:       ErrJobNotStarted,
	}
}

// Run executes a job with a timeout and returns the result.
func Run(ctx context.Context, job Job) *JobResult {
	errChan := make(chan error, 1)

	jobResult := ToJobResult(job)
	jobResult.MarkAsStarted()

	go func(j Job) {
		defer close(errChan)
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("[CRITICAL] Recovering from exception in task: %v %s", err, string(debug.Stack()))
				errChan <- errors.New("internal server error")
			}
		}()

		errChan <- j.Check(ctx)
	}(job)

	select {
	case err := <-errChan:
		jobResult.AddError(err)
	case <-time.After(job.Config.Timeout):
		jobResult.AddError(ErrJobTimeout)
	case <-ctx.Done():
		jobResult.AddError(ctx.Err())
	}

	jobResult.MarkAsCompleted()

	return jobResult
}

func RunAny(ctx context.Context, jobs []Job) []JobResult {
	results := make([]JobResult, len(jobs))
	g, ctx := errgroup.WithContext(ctx)
	for i := range jobs {
		g.Go(func() error {
			results[i] = *Run(ctx, jobs[i])
			return nil
		})
	}
	_ = g.Wait()
	return results
}
