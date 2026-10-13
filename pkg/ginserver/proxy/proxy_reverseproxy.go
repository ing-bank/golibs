package proxy

import (
	"crypto/tls"
	"fmt"
	"net"
	gohttp "net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/tlsclient"
)

// defaultDialTimeout is the default timeout for establishing a connection
const defaultDialTimeout = 30 * time.Second

// defaultTLSHandshakeTimeout is the default timeout for TLS handshake
const defaultTLSHandshakeTimeout = 10 * time.Second

// defaultKeepAliveInterval is the default interval for TCP keep-alive probes
const defaultKeepAliveInterval = 30 * time.Second

// defaultIdleConnTimeout is the default maximum time to keep idle connections
const defaultIdleConnTimeout = 90 * time.Second

// defaultMaxIdleConns is the default maximum number of idle connections
const defaultMaxIdleConns = 100

// defaultResponseHeaderTimeout is the default timeout for receiving response headers
// Set to 0 (disabled) for streaming operations like Kubernetes watches
const defaultResponseHeaderTimeout = time.Duration(0)

// getDurationOrDefault returns the value from the pointer, or the default if nil
func getDurationOrDefault(value *int, defaultDuration time.Duration) time.Duration {
	if value == nil {
		return defaultDuration
	}
	return time.Duration(*value) * time.Second
}

// getIntOrDefault returns the value from the pointer, or the default if nil
func getIntOrDefault(value *int, defaultInt int) int {
	if value == nil {
		return defaultInt
	}
	return *value
}

// newReverseProxy creates a Proxy using the net/http/httputil.ReverseProxy mode
func newReverseProxy(cfg Config) (*Proxy, error) {
	// Create TLS config if enabled
	var tlsConfig *tls.Config
	var err error

	if cfg.ReverseProxyConfig.TLS.UseTLS() {
		tlsConfig, err = tlsclient.NewForConfig(&cfg.ReverseProxyConfig.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Extract timeout values with defaults
	dialTimeout := getDurationOrDefault(cfg.ReverseProxyConfig.DialTimeout, defaultDialTimeout)
	tlsHandshakeTimeout := getDurationOrDefault(cfg.ReverseProxyConfig.TLSHandshakeTimeout, defaultTLSHandshakeTimeout)
	keepAliveInterval := getDurationOrDefault(cfg.ReverseProxyConfig.KeepAliveInterval, defaultKeepAliveInterval)
	idleConnTimeout := getDurationOrDefault(cfg.ReverseProxyConfig.IdleConnTimeout, defaultIdleConnTimeout)
	responseHeaderTimeout := getDurationOrDefault(cfg.ReverseProxyConfig.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
	maxIdleConns := getIntOrDefault(cfg.ReverseProxyConfig.MaxIdleConns, defaultMaxIdleConns)

	// Create custom transport with TLS config and timeouts
	transport := &gohttp.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: keepAliveInterval,
		}).DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		IdleConnTimeout:       idleConnTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		MaxIdleConns:          maxIdleConns,
	}

	reverseProxyRoutes := make([]routeProxy, 0, len(cfg.Routes))

	for _, route := range cfg.Routes {
		targetURL, err := url.Parse(route.Target)
		if err != nil {
			return nil, fmt.Errorf("failed to parse target URL %q: %w", route.Target, err)
		}

		// Create a reverse proxy for this target
		routeProxy := &routeProxy{
			prefix: route.Prefix,
			target: targetURL,
			proxy: &httputil.ReverseProxy{
				Rewrite:   func(r *httputil.ProxyRequest) {},
				Transport: transport,
			},
		}
		reverseProxyRoutes = append(reverseProxyRoutes, *routeProxy)
	}

	return &Proxy{
		basePath:           cfg.BasePath,
		mode:               ProxyModeReverseProxy,
		methods:            cfg.Methods,
		routes:             cfg.Routes,
		reverseProxyRoutes: reverseProxyRoutes,
	}, nil
}

// proxyWithReverseProxy handles proxying via net/http/httputil.ReverseProxy
// This mode supports streaming (for Kubernetes watches) but does not support tripperware
func (s *Proxy) proxyWithReverseProxy(c *gin.Context) {
	// Find the matching route
	route, remainingPath, err := s.findReverseProxyRoute(c.Request.URL.Path)
	if err != nil {
		c.AbortWithStatusJSON(gohttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Construct the target path by merging target's path with remaining path
	targetPath := strings.TrimSuffix(route.target.Path, "/")
	if !strings.HasPrefix(remainingPath, "/") {
		remainingPath = "/" + remainingPath
	}
	newPath := targetPath + remainingPath

	// Rewrite the request URL
	c.Request.URL.Path = newPath
	c.Request.URL.Host = route.target.Host
	c.Request.URL.Scheme = route.target.Scheme
	c.Request.Host = route.target.Host

	// Use the reverse proxy to handle the request
	// This supports streaming (e.g., Kubernetes watches) natively
	route.proxy.ServeHTTP(c.Writer, c.Request)
}

// findReverseProxyRoute finds the matching route for a given path and returns
// the route proxy and the remaining path after the prefix
func (s *Proxy) findReverseProxyRoute(path string) (*routeProxy, string, error) {
	for i := range s.reverseProxyRoutes {
		route := &s.reverseProxyRoutes[i]
		fullPrefix := s.basePath + route.prefix
		if after, ok := strings.CutPrefix(path, fullPrefix); ok {
			return route, after, nil
		}
	}

	return nil, "", fmt.Errorf("%w: no matching route for path %s", http.ErrBadRequest, path)
}
