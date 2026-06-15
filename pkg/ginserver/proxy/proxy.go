package proxy

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	gohttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/http"
)

var anyMethods = []string{
	gohttp.MethodGet, gohttp.MethodPost, gohttp.MethodPut, gohttp.MethodPatch,
	gohttp.MethodHead, gohttp.MethodOptions, gohttp.MethodDelete, gohttp.MethodConnect,
	gohttp.MethodTrace,
}

type Config struct {
	HTTPConfig http.Config `json:"client" yaml:"client"`
	BasePath   string      `json:"basePath" yaml:"basePath"`
	Routes     []Route     `json:"routes" yaml:"routes"`
}

type Route struct {
	Prefix string `json:"pathPrefix" yaml:"pathPrefix"`
	Target string `json:"targetURL" yaml:"targetURL"`
}

type Proxy struct {
	basePath   string
	httpClient *http.Client
	routes     []Route
}

func NewForConfig(cfg Config) (*Proxy, error) {
	httpClient, err := http.NewForConfig(&cfg.HTTPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	return &Proxy{
		basePath:   cfg.BasePath,
		routes:     cfg.Routes,
		httpClient: httpClient,
	}, nil
}

func (s *Proxy) Register(root gin.IRouter) {
	basePath := strings.TrimSuffix(s.basePath, "/")
	for _, method := range anyMethods {
		root.Handle(method, basePath+"/*proxyPath", s.Proxy)
	}
}

func (s *Proxy) targetURL(path string) (string, error) {
	for _, route := range s.routes {
		after, ok := strings.CutPrefix(path, s.basePath+route.Prefix)
		if ok {
			// Replace the prefix with the target URL
			return route.Target + after, nil
		}
	}
	return "", fmt.Errorf("%w: %s", http.ErrBadRequest, path)
}

func (s *Proxy) Proxy(c *gin.Context) {
	// Determine target URL based on the incoming request path
	targetURL, err := s.targetURL(c.Request.URL.Path)
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

	resp := s.httpClient.Exec(c.Request.Context(), req, nil)
	if !resp.IsOK() {
		c.AbortWithStatusJSON(gohttp.StatusBadGateway, gin.H{"error": fmt.Sprintf("Backend error: %s", resp.Error())})
		return
	}
	maps.Copy(c.Writer.Header(), resp.Headers)

	// set the status code from the response
	c.Status(resp.Status)

	// Stream the response body
	_, err = io.Copy(c.Writer, bytes.NewReader(resp.Raw))
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "Failed to proxy response: " + err.Error()})
	}
}
