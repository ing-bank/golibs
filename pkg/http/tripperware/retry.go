package tripperware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/ing-bank/golibs/pkg/retry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultRetryAttempts = 3
)

var (
	DefaultRetryDelay = metav1.Duration{Duration: 100 * time.Millisecond}
)

// Retrier provides retry functionality for HTTP requests.
type Retrier struct {
	cfg *RetrierConfig
}

// RetrierOptions defines options for configuring a Retrier.
type RetrierOptions = config.Option[*RetrierConfig]

// RetryableErrorFn defines a function type to determine if an error is retryable.
type RetryableErrorFn func(err error) bool

// WithRetryableErrorFn sets the function to determine if an error is retryable.
func WithRetryableErrorFn(fn RetryableErrorFn) config.Option[*RetrierConfig] {
	return func(r *RetrierConfig) error {
		r.RetryableErrorFn = fn
		return nil
	}
}

// WithBackoff sets the backoff strategy for the Retrier.
func WithBackoff(backoff retry.Backoff) config.Option[*RetrierConfig] {
	return func(r *RetrierConfig) error {
		r.Backoff = &backoff
		return nil
	}
}

// RetrierConfig holds configuration for the Retrier.
type RetrierConfig struct {
	Disabled         bool             `yaml:"disabled" json:"disabled"`
	Retries          int              `yaml:"retries" json:"retries"`
	Duration         metav1.Duration  `yaml:"duration" json:"duration"`
	RetryableErrorFn RetryableErrorFn `yaml:"-" json:"-"`
	Backoff          *retry.Backoff   `yaml:"-" json:"-"` // TODO: Make Backoff configurable via YAML/JSON
}

// DefaultRetrierConfig returns a RetrierConfig with default values applied.
func DefaultRetrierConfig() *RetrierConfig {
	c := new(RetrierConfig)
	c.ApplyDefaults()
	return c
}

// ApplyDefaults sets default values for RetrierConfig fields if they are not set.
func (c *RetrierConfig) ApplyDefaults() {
	if c.Retries == 0 {
		c.Retries = DefaultRetryAttempts
	}
	if c.Duration.Duration == 0 {
		c.Duration = DefaultRetryDelay
	}
	if c.Backoff == nil {
		c.Backoff = new(retry.NewDefaultBackoff(c.Retries, c.Duration.Duration))
	}
	if c.RetryableErrorFn == nil {
		c.RetryableErrorFn = errors.IsRetryableError
	}
}

// Validate checks if the RetrierConfig has valid values.
func (c *RetrierConfig) Validate() error {
	if c.Retries <= 0 {
		return fmt.Errorf("invalid retrier config: retries must be greater than zero")
	}
	if c.Duration.Duration <= 0 {
		return fmt.Errorf("invalid retrier config: duration must be greater than zero")
	}
	if c.Backoff == nil {
		return fmt.Errorf("invalid retrier config: backoff must not be nil")
	}
	if c.RetryableErrorFn == nil {
		return fmt.Errorf("invalid retrier config: retryable error function must not be nil")
	}
	return nil
}

// ClonedRequest holds the original request and a copy of its body for safe retries.
type ClonedRequest struct {
	Original *http.Request
	Body     []byte
}

// NewRetrierForConfig creates a Retrier from the provided RetrierConfig, applying any provided options.
func NewRetrierForConfig(cfg RetrierConfig) (*Retrier, error) {
	if err := config.Configure(&cfg); err != nil {
		return nil, fmt.Errorf("failed to configure retrier: %w", err)
	}

	return &Retrier{cfg: &cfg}, nil
}

// NewRetrier creates a Retrier with default settings, applying any provided options.
func NewRetrier(opts ...RetrierOptions) (*Retrier, error) {
	cfg, err := config.New[RetrierConfig]()
	if err != nil {
		return nil, fmt.Errorf("failed to create retrier config: %w", err)
	}
	if err := config.ApplyOptions(cfg, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply retrier options: %w", err)
	}
	return NewRetrierForConfig(*cfg)
}

// Tripperware returns a tripperware that retries requests based on the Retrier's settings.
func (r *Retrier) Tripperware() Tripperware {
	if r.cfg.Disabled {
		return EmptyTripperware()
	}
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {

			clonedReq, err := NewClonedRequest(request)
			if err != nil {
				return &response.Data{Err: err}
			}

			var resp *response.Data

			err = retry.OnError(ctx, *r.cfg.Backoff, r.cfg.RetryableErrorFn, func() error {
				reqCopy := clonedReq.GetRequest(ctx)
				resp = next(ctx, reqCopy)
				return resp.Error()
			})

			if resp != nil && resp.Err == nil {
				resp.Err = err
			}
			return resp
		}
	}
}

// NewClonedRequest creates a ClonedRequest from an *http.Request, buffering the body if present.
func NewClonedRequest(req *http.Request) (*ClonedRequest, error) {
	var bodyBytes []byte
	var err error
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return &ClonedRequest{
		Original: req,
		Body:     bodyBytes,
	}, nil
}

// GetRequest returns a clone of the original request with a fresh body reader.
func (c *ClonedRequest) GetRequest(ctx context.Context) *http.Request {
	reqCopy := c.Original.Clone(ctx)
	if c.Body != nil {
		reqCopy.Body = io.NopCloser(bytes.NewReader(c.Body))
	}
	return reqCopy
}
