// Package http provides HTTP health check implementation for verifying service availability.
//
// # Overview
//
// The HTTP health check validates that a service is responding to HTTP requests with a
// status code less than 500. It supports custom headers, TLS configuration, and can use
// context-based authentication tokens.
//
// # Configuration
//
// Health checks are configured via the Config struct, which supports:
//   - Target endpoint (host, port, scheme, path)
//   - TLS/HTTPS with certificate verification
//   - Custom HTTP headers (including context-based tokens)
//
// # Example: External HTTPS Probe with Authentication
//
//	  path: /healthz
//	  scheme: https
//	  host: 127.0.0.1
//	  port: 8051
//	  httpHeaders:
//	    - name: Authorization
//	      value: <<use-context>>
//	    - name: X-Username
//	      value: foobar
//	  tls:
//	    cert: "./examples/mtls-server/tls.crt"
//	    key: "./examples/mtls-server/tls.key"
//	    insecureSkipVerify: true
//
// # Authorization Headers
//
// The special value "<<use-context>>" in the Authorization header instructs the health
// check to extract the authentication token from the current request context. This is
// useful when the same authentication (OAuth, JWT, etc.) needs to be applied to both
// the main request and health check probes.

package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	gohttp "net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/tlsclient"
	"github.com/ing-bank/golibs/pkg/utils"
)

var _ checks.Handler = (*Client)(nil)

const (
	defaultRequestTimeout = 5 * time.Second

	// UseContextToken is a special value for the Authorization header that indicates
	// the token should be fetched from the current context in use.
	UseContextToken = "<<use-context>>"

	// AuthorizationHeader is the HTTP Authorization header name.
	AuthorizationHeader = "Authorization"
)

// Config is the HTTP checker configuration settings container.
type Config struct {
	// Path is the remote service health check Path.
	Path string `json:"path" yaml:"path"`
	// Scheme is the URL scheme, either "http" or "https". If empty, it will be inferred from TLSConfig.
	Scheme string `json:"scheme" yaml:"scheme"`
	// Host is the remote service host.
	Host string `json:"host" yaml:"host"`
	// Port is the remote service port.
	Port uint16 `json:"port" yaml:"port"`
	// TLSConfig is the TLS configuration for the HTTP client.
	TLSConfig tlsclient.Config `json:"tls" yaml:"tls"`
	// HTTPHeaders are additional HTTP headers to include in the request.
	HTTPHeaders []map[string]string `json:"httpHeaders" yaml:"httpHeaders"`
}

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.Scheme != "" && c.Scheme != "http" && c.Scheme != "https" {
		return fmt.Errorf("scheme must be either 'http' or 'https'")
	}
	if c.TLSConfig.UseTLS() && c.Scheme == "http" {
		return fmt.Errorf("TLSConfig is set but scheme is 'http'; use 'https' scheme for TLS")
	}
	return nil
}

// WithAddress sets the Host and Port from a given address string (e.g., "example.com:8080").
func (c *Config) WithAddress(address string) error {
	host, port, err := utils.ExtractHostPort(address)
	if err != nil {
		return fmt.Errorf("failed to extract host and port from webserver address %q: %w", address, err)
	}
	if host == "" || (net.ParseIP(host) == nil && !isValidHostname(host)) {
		return fmt.Errorf("invalid host: %q", host)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if c.Host == "" {
		c.Host = host
	}
	if c.Port == 0 {
		c.Port = uint16(port) //nolint:gosec // port range is already validated above
	}
	return nil
}

// isValidHostname checks if a string is a valid DNS hostname (RFC 1123)
func isValidHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for part := range strings.SplitSeq(host, ".") {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
		for _, r := range part {
			if !(r == '-' || r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				return false
			}
		}
		if part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
	}
	return true
}

// NormalizeURLFor constructs and normalizes the full URL from the Config.
func NormalizeURLFor(cfg Config) (string, error) {
	urlStr := cfg.Host
	if cfg.Port != 0 {
		urlStr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	if cfg.Path != "" {
		urlStr += cfg.Path
	}

	// Determine scheme: prefer cfg.Scheme, else infer from TLSConfig
	scheme := cfg.Scheme
	if scheme == "" {
		scheme = "http"
		if cfg.TLSConfig.UseTLS() {
			scheme = "https"
		}
	}

	// Prepend scheme if missing
	if !strings.Contains(urlStr, "://") {
		urlStr = fmt.Sprintf("%s://%s", scheme, urlStr)
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	return parsed.String(), nil
}

// Client is the HTTP health check client.
type Client struct {
	httpClient *http.Client
	urlz       string
	headers    []map[string]string
}

// New creates new HTTP service health check that verifies the following:
// - connection establishing
// - getting response status from defined URL
// - verifying that status code is less than 500
func New(c *Config) (*Client, error) {
	cfg := *c // shallow copy

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid HTTP health check config: %w", err)
	}

	var tlsconfig *tls.Config
	var err error

	clientOptions := []http.ClientOption{http.WithCheckRedirect(func(_ *gohttp.Request, _ []*gohttp.Request) error {
		return gohttp.ErrUseLastResponse
	})}

	if c.TLSConfig.UseTLS() {
		tlsconfig, err = tlsclient.NewForConfig(&c.TLSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config for health check: %w", err)
		}
		clientOptions = append(clientOptions, http.WithNewTransport(
			http.WithTLSConfig(tlsconfig),
		))
	}

	client, err := http.NewClient(clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	normalizedURL, err := NormalizeURLFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize URL: %w", err)
	}

	return &Client{
		httpClient: client,
		urlz:       normalizedURL,
		headers:    cfg.HTTPHeaders,
	}, nil
}

// buildRequestOptions constructs the HTTP request options including headers and authorization.
func buildRequestOptions(headersList []map[string]string) []http.RequestOption {
	var opts []http.RequestOption

	// merge all header maps into a single map
	mergedHeaders := make(map[string]string)
	for _, headerMap := range headersList {
		maps.Copy(mergedHeaders, headerMap)
	}

	// check for Authorization header with UseContextToken value
	token, ok := mergedHeaders[AuthorizationHeader]
	// if the token is the special UseContextToken, set up bearer auth from context
	if ok && token == UseContextToken {
		opts = append(opts, http.WithBearerAuth(""))
		// remove the Authorization header to avoid duplication
		delete(mergedHeaders, AuthorizationHeader)
	}
	opts = append(opts, http.WithHeaders(mergedHeaders))
	return opts
}

// Check performs the HTTP health check by sending a GET request to the configured URL.
// It returns an error if the request fails or if the response status code is 500 or greater.
func (c *Client) Check(ctx context.Context) error {
	// build request options with headers
	opts := buildRequestOptions(c.headers)
	// perform the HTTP GET request
	resp := c.httpClient.Get(ctx, c.urlz, opts...)
	if resp.Error() != nil && resp.Status == 0 {
		return fmt.Errorf("failed to perform HTTP GET request: %w", resp.Error())

	} else if resp.Error() != nil {
		return fmt.Errorf("HTTP request to '%s' failed (%s): %s", c.urlz, gohttp.StatusText(resp.Status), bytes.Trim(resp.Raw, "\n"))
	}
	return nil
}
