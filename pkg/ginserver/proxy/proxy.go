package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	ghttp "github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/tlsclient"
)

// ProxyMode determines which proxy implementation to use
type ProxyMode string

const (
	// ProxyModeHTTPClient uses the golibs HTTP client (default, supports tripperware)
	ProxyModeHTTPClient ProxyMode = "httpClient"
	// ProxyModeReverseProxy uses net/http/httputil.ReverseProxy (supports streaming)
	ProxyModeReverseProxy ProxyMode = "reverseProxy"
)

// DefaultMethods contains HTTP methods supported by the proxy
var DefaultMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect,
	http.MethodTrace,
}

// ReverseProxyConfig holds configuration specific to reverseProxy mode
type ReverseProxyConfig struct {
	TLS tlsclient.Config `json:"tls" yaml:"tls"`

	// Timeouts for the underlying HTTP transport (in seconds)
	// These control connection lifecycle and are important for streaming/watch operations
	DialTimeout           *int `json:"dialTimeout" yaml:"dialTimeout"`           // Connection establishment timeout (default: 30s)
	TLSHandshakeTimeout   *int `json:"tlsHandshakeTimeout" yaml:"tlsHandshakeTimeout"`   // TLS handshake timeout (default: 10s)
	IdleConnTimeout       *int `json:"idleConnTimeout" yaml:"idleConnTimeout"`     // Maximum time to keep idle connections (default: 90s)
	KeepAliveInterval     *int `json:"keepAliveInterval" yaml:"keepAliveInterval"`   // Keep-alive probe interval (default: 30s)
	ResponseHeaderTimeout *int `json:"responseHeaderTimeout" yaml:"responseHeaderTimeout"` // Timeout for receiving response headers (default: 0 = disabled, important for streaming)
	MaxIdleConns          *int `json:"maxIdleConns" yaml:"maxIdleConns"`         // Maximum idle connections (default: 100)
}

// Config defines proxy configuration with mode selection
type Config struct {
	Mode               ProxyMode           `json:"mode" yaml:"mode"`                 // "httpClient" (default) or "reverseProxy"
	HTTPConfig         *ghttp.Config       `json:"client" yaml:"client"`             // used when Mode=="httpClient"
	ReverseProxyConfig *ReverseProxyConfig `json:"reverseProxy" yaml:"reverseProxy"` // used when Mode=="reverseProxy"
	BasePath           string              `json:"basePath" yaml:"basePath"`
	Methods            []string            `json:"methods" yaml:"methods"`
	Routes             []Route             `json:"routes" yaml:"routes"`
}

// Deprecated: Use Config directly. This is kept for backwards compatibility only.
type HTTPConfig = ghttp.Config

type Route struct {
	Prefix string `json:"pathPrefix" yaml:"pathPrefix"`
	Target string `json:"targetURL" yaml:"targetURL"`
}

type Proxy struct {
	basePath string
	mode     ProxyMode
	methods  []string
	routes   []Route

	// httpClient mode fields
	httpClient *ghttp.Client

	// reverseProxy mode fields
	reverseProxyRoutes []routeProxy
}

type routeProxy struct {
	prefix string
	target *url.URL
	proxy  *httputil.ReverseProxy
}

// ApplyDefaults sets default values for empty config fields
func (c *Config) ApplyDefaults() error {
	if c.BasePath == "" {
		c.BasePath = "/proxy"
	}
	if c.Methods == nil {
		c.Methods = DefaultMethods
	}
	if c.Mode == "" {
		c.Mode = ProxyModeHTTPClient
	}
	return nil
}

// NewForConfig creates a new Proxy from configuration with validation
func NewForConfig(cfg Config) (*Proxy, error) {
	if err := cfg.ApplyDefaults(); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Validate mode-config consistency
	switch cfg.Mode {
	case ProxyModeHTTPClient:
		if cfg.ReverseProxyConfig != nil {
			return nil, fmt.Errorf("ReverseProxyConfig must be nil when mode is %q", ProxyModeHTTPClient)
		}
		// If HTTPConfig is nil, use empty defaults
		if cfg.HTTPConfig == nil {
			cfg.HTTPConfig = &ghttp.Config{}
		}
		return newHTTPClientProxy(cfg)

	case ProxyModeReverseProxy:
		if cfg.HTTPConfig != nil {
			return nil, fmt.Errorf("HTTPConfig must be nil when mode is %q", ProxyModeReverseProxy)
		}
		if cfg.ReverseProxyConfig == nil {
			return nil, fmt.Errorf("ReverseProxyConfig required for %q mode", ProxyModeReverseProxy)
		}
		return newReverseProxy(cfg)

	default:
		return nil, fmt.Errorf("unknown proxy mode: %q", cfg.Mode)
	}
}

func (s *Proxy) Register(root gin.IRouter) {
	basePath := strings.TrimSuffix(s.basePath, "/")
	for _, method := range s.methods {
		root.Handle(method, basePath+"/*proxyPath", s.Proxy)
	}
}

func (s *Proxy) findRoute(path string) (string, error) {
	for _, route := range s.routes {
		fullPrefix := s.basePath + route.Prefix
		if after, ok := strings.CutPrefix(path, fullPrefix); ok {
			return route.Target + after, nil
		}
	}
	return "", fmt.Errorf("%w: no matching route for path %s", ghttp.ErrBadRequest, path)
}

func (s *Proxy) Proxy(c *gin.Context) {
	switch s.mode {
	case ProxyModeHTTPClient:
		s.proxyWithHTTPClient(c)
	case ProxyModeReverseProxy:
		s.proxyWithReverseProxy(c)
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unknown proxy mode: %q", s.mode)})
	}
}
