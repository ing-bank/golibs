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
	log "github.com/sirupsen/logrus"
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
	RetryableErrorFn RetryableErrorFn
	Backoff          retry.Backoff
}

// RetrierOptions defines options for configuring a Retrier.
type RetrierOptions = config.Option[*Retrier]

// RetryableErrorFn defines a function type to determine if an error is retryable.
type RetryableErrorFn func(err error) bool

// WithRetryableErrorFn sets the function to determine if an error is retryable.
func WithRetryableErrorFn(fn RetryableErrorFn) config.Opt[*Retrier] {
	return func(r *Retrier) error {
		r.RetryableErrorFn = fn
		return nil
	}
}

// WithBackoff sets the backoff strategy for the Retrier.
func WithBackoff(backoff retry.Backoff) config.Opt[*Retrier] {
	return func(r *Retrier) error {
		r.Backoff = backoff
		return nil
	}
}

// RetrierConfig holds configuration for the Retrier.
type RetrierConfig struct {
	Retries  int             `yaml:"retries" json:"retries"`
	Duration metav1.Duration `yaml:"duration" json:"duration"`
}

// DefaultRetrierConfig returns a RetrierConfig with default values applied.
func DefaultRetrierConfig() *RetrierConfig {
	c := new(RetrierConfig)
	c.ApplyDefaultRetrierConfig()
	return c
}

// ApplyDefaultRetrierConfig sets default values for RetrierConfig fields if they are not set.
func (c *RetrierConfig) ApplyDefaultRetrierConfig() {
	if c.Retries == 0 {
		c.Retries = DefaultRetryAttempts
	}
	if c.Duration.Duration == 0 {
		c.Duration = DefaultRetryDelay
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
	return nil
}

// ClonedRequest holds the original request and a copy of its body for safe retries.
type ClonedRequest struct {
	Original *http.Request
	Body     []byte
}

// NewRetrierForConfig creates a Retrier from the provided RetrierConfig, applying any provided options.
func NewRetrierForConfig(c *RetrierConfig, opts ...RetrierOptions) (*Retrier, error) {
	if c == nil {
		return nil, fmt.Errorf("config is nil")
	}
	cfg := *c // shallow copy

	// Apply the default configuration if not provided
	cfg.ApplyDefaultRetrierConfig()

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid retrier config: %w", err)
	}

	r := &Retrier{
		Backoff:          retry.NewDefaultBackoff(cfg.Retries, cfg.Duration.Duration),
		RetryableErrorFn: errors.IsRetryableError,
	}

	// Apply any additional options
	if err := config.ApplyOpts(r, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply retrier options: %w", err)
	}
	return r, nil
}

// NewRetrier creates a Retrier with default settings, applying any provided options.
func NewRetrier(opts ...RetrierOptions) *Retrier {
	cfg := DefaultRetrierConfig()
	r := &Retrier{
		Backoff:          retry.NewDefaultBackoff(cfg.Retries, cfg.Duration.Duration),
		RetryableErrorFn: errors.IsRetryableError,
	}
	if err := config.ApplyOpts(r, opts...); err != nil {
		log.Errorf("failed to apply retrier options: %s", err)
	}
	return r
}

// Tripperware returns a tripperware that retries requests based on the Retrier's settings.
func (r *Retrier) Tripperware() Tripperware {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {

			clonedReq, err := NewClonedRequest(request)
			if err != nil {
				return &response.Data{Err: err}
			}

			var resp *response.Data

			err = retry.OnError(ctx, r.Backoff, r.RetryableErrorFn, func() error {
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
