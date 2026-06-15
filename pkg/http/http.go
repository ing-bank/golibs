// Package http provides a wrapper around the standard library http.Client with support for
// tripperware middleware, transport customization, and simplified request/response handling.
//
// Core Features:
//
//   - Simplified Request/Response: All requests return a Response type that simplifies payload
//     parsing and error handling.
//   - Tripperware: Middleware for client requests enabling cross-cutting concerns like metrics,
//     retries, and logging.
//   - Transport Customization: Easy configuration of TLS, mTLS, and other transport options.
//   - Request Options: RequestOption functions allow per-request customization (headers, auth, etc.).
//   - Configuration: Load HTTP client settings from config files with validation.
//
// Basic Usage:
//
//	// Create a (customized) client
//	client, err := http.NewClient() // could also use http.DefaultClient to skip initialization
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Perform requests
//  var foo &Foo{}
//	if resp := client.Get(ctx, "https://example.com/api/foo").Parse(foo); !resp.IsOK() {
//		panic(resp.Error())
//	}
//
// Tripperware:
// Tripperware is middleware that wraps each HTTP request before it's sent. The DefaultTripperware
// provides common functionality. Custom tripperware can be added via ClientOptions:
//
//	client, _ := http.NewClient(http.WithTripperware(myCustomTripperware))
//
// Configuration:
// HTTP clients can be created from a Config struct, which includes TLS settings and other options:
//
//	config := &http.Config{
//		TLS: tlsclient.Config{ /* ... */ },
//	}
//	client, err := http.NewForConfig(config)
//
// Polling:
// The Await method polls an endpoint until a ready function returns true, useful for
// waiting on service startup or status changes.
//
//	err := client.Await(ctx, "http://localhost:8080/health", func(resp *response.Data) bool {
//		return resp.IsOK()
//	}, http.AwaitOpts{Delay: 1 * time.Second, Steps: 60})

package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/ing-bank/golibs/pkg/http/tripperware"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/tlsclient"
)

// DefaultClient is the standard HTTP Client with Tripperware. It's recommended to use NewClient to create new clients
// if you want to set additional options
var DefaultClient = &Client{
	Tripperware: tripperware.DefaultTripperware,
	Http:        http.DefaultClient,
}

// Client provides a wrapper for http.Client to provide tripperware, transport and other request options
type Client struct {
	Http                  *http.Client
	Tripperware           tripperware.Tripperware
	DefaultRequestOptions []RequestOption
}

// AwaitOpts describe polling behavior
type AwaitOpts struct {
	Delay time.Duration // Time between steps
	Steps int           // Maximum amount of steps to execute
}

// ErrTimeoutStatusChange is used by Await
var ErrTimeoutStatusChange = errors.New("timed out awaiting status change")

// NewClient creates a Client and allows optional ClientOption(s) to be set. ClientOptions
// can modify the HTTP client itself, set tripperware, or set default RequestOptions.
func NewClient(opts ...ClientOption) (*Client, error) {
	client := &Client{
		Tripperware: func(endpoint tripperware.Endpoint) tripperware.Endpoint {
			return endpoint
		},
		Http: &http.Client{},
	}

	if err := config.ApplyOpts(client, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply client options: %w", err)
	}

	return client, nil
}

func NewForConfig(c *Config, opts ...ClientOption) (*Client, error) {
	cfg := *c // shallow copy

	// apply default values to the config
	ApplyDefaults(&cfg)
	// validate the config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid HTTP health check config: %w", err)
	}

	clientOptions := []ClientOption{
		// not follow HTTP redirects automatically and instead return the last response received.
		WithCheckRedirect(func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}),
	}

	var tlsconfig *tls.Config
	var err error

	if c.TLS.UseTLS() {
		tlsconfig, err = tlsclient.NewForConfig(&c.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config for health check: %w", err)
		}
		clientOptions = append(clientOptions, WithNewTransport(
			WithTLSConfig(tlsconfig),
		))
	}

	client, err := NewClient(clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// set default tripperware if none was provided
	if client.Tripperware == nil {
		client.Tripperware = tripperware.DefaultTripperware
	}

	// apply all extra options
	// this allows overriding the default tripperware if needed
	// and setting default request options
	if err := client.With(opts...); err != nil {
		return nil, fmt.Errorf("failed to apply client options: %w", err)
	}

	return client, nil
}

// do executes a http request, it applies default request options and calls tripperware
func (c *Client) do(ctx context.Context, method, url string, request any, options ...RequestOption) *response.Data {
	// Create *http.Request
	req, err := build(ctx, method, url, request)
	if err != nil {
		return &response.Data{Err: fmt.Errorf("%w: %s %s: %v", ErrBuildRequest, method, url, err)}
	}

	// Set request options
	if err := config.ApplyOpts(req, append(c.DefaultRequestOptions, options...)...); err != nil {
		return &response.Data{Err: fmt.Errorf("%w: %v", ErrFailedRequestOption, err)}
	}

	// Execute request via tripperware chain
	return c.Tripperware(func(_ context.Context, request *http.Request) *response.Data {
		resp, err := c.Http.Do(request)
		if err != nil {
			return &response.Data{Err: err}
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		return &response.Data{Raw: raw, Status: resp.StatusCode, Err: err, Headers: resp.Header.Clone()}
	})(ctx, req)
}

// build a http.Request from provided parameters. Automatically parses the provided request as JSON
// if applicable. Bytes or http.Requests are just passed through.
func build(ctx context.Context, method, url string, request any) (*http.Request, error) {
	var req *http.Request
	var err error

	switch v := request.(type) {
	case *http.Request:
		req = v
	case []byte:
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(v))
	case nil:
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	default:
		raw, marshalErr := json.Marshal(request)
		if marshalErr != nil {
			err = marshalErr
		} else {
			req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(raw))
		}
	}
	return req, err
}

func (c *Client) Exec(ctx context.Context, req *http.Request, options ...RequestOption) *response.Data {
	return c.do(ctx, req.Method, req.URL.String(), req, options...)
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, url string, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodGet, url, nil, options...)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, url string, request any, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodPost, url, request, options...)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, url string, request any, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodPut, url, request, options...)
}

// Patch performs a PATCH request
func (c *Client) Patch(ctx context.Context, url string, request any, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodPatch, url, request, options...)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, url string, request any, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodDelete, url, request, options...)
}

// Head performs a HEAD request
func (c *Client) Head(ctx context.Context, url string, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodHead, url, nil, options...)
}

// Options performs a Options request
func (c *Client) Options(ctx context.Context, url string, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodOptions, url, nil, options...)
}

// Trace performs a TRACE request
func (c *Client) Trace(ctx context.Context, url string, options ...RequestOption) *response.Data {
	return c.do(ctx, http.MethodTrace, url, nil, options...)
}

// Await polls an endpoint by performing a GET request and then calling the provided ready function. Await exists
// without error only when the ready function returns true. Polling behavior can be modified via AwaitOpts.
func (c *Client) Await(ctx context.Context, url string, ready func(response *response.Data) bool, awaitOpt ...AwaitOpts) error {
	opts := opt.Opt(AwaitOpts{
		Delay: 5 * time.Second,
		Steps: 100,
	}, awaitOpt)

	for i := 0; i < opts.Steps; i++ {
		if ready(c.Get(ctx, url)) {
			return nil
		}
		time.Sleep(opts.Delay)
	}

	return ErrTimeoutStatusChange
}
