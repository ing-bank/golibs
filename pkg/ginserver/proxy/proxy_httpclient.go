package proxy

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	gohttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/http"
)

// newHTTPClientProxy creates a Proxy using the golibs HTTP client mode
func newHTTPClientProxy(cfg Config) (*Proxy, error) {
	httpClient, err := http.NewClientForConfig(*cfg.HTTPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &Proxy{
		basePath:   cfg.BasePath,
		mode:       ProxyModeHTTPClient,
		methods:    cfg.Methods,
		routes:     cfg.Routes,
		httpClient: httpClient,
	}, nil
}

// proxyWithHTTPClient handles proxying via the golibs HTTP client
// This mode supports tripperware (retry, circuit-breaker, metrics, etc.)
// but buffers the entire response in memory
func (s *Proxy) proxyWithHTTPClient(c *gin.Context) {
	// Determine target URL based on the incoming request path
	targetURL, err := s.findRoute(c.Request.URL.Path)
	if err != nil {
		c.AbortWithStatusJSON(gohttp.StatusBadRequest, gin.H{"error": "Failed to get target URL: " + err.Error()})
		return
	}
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	req, err := gohttp.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(gohttp.StatusBadRequest, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}
	req.Header = c.Request.Header.Clone()

	resp := s.httpClient.Exec(c.Request.Context(), req)
	if resp.Err != nil { // Catch only transport errors, ignore HTTP errors (4xx, 5xx) as they are valid responses from the backend
		c.AbortWithStatusJSON(gohttp.StatusBadGateway, gin.H{"error": fmt.Sprintf("Backend error: %s", resp.Error())})
		return
	}
	maps.Copy(c.Writer.Header(), resp.Headers)

	// set the status code from the response
	c.Status(resp.Status)

	// Write the response body (buffered)
	_, err = io.Copy(c.Writer, bytes.NewReader(resp.Raw))
	if err != nil {
		c.AbortWithStatusJSON(gohttp.StatusInternalServerError, gin.H{"error": "Failed to proxy response: " + err.Error()})
	}
}
