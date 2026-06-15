package metrics

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var DefaultSkipPaths = defaultSkipPaths

var defaultSkipPaths = []string{"/metrics", "/health", "/ready", "/healthz", "/readyz", "/swagger"}

var labels = []string{"status", "path", "method"}

var uptime = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "golibs_middleware_server_uptime",
		Help: "HTTP service uptime.",
	}, nil,
)

var promRequestCount = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "golibs_middleware_server_http_request_count_total",
		Help: "Total number of HTTP requests made.",
	}, labels,
)

var promRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "golibs_middleware_server_http_request_duration_miliseconds",
		Help:    "HTTP request latencies in seconds.",
		Buckets: []float64{0.1, 1, 2.5, 10, 20},
	}, labels,
)

var promRequestSizeBytes = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "golibs_middleware_server_http_request_size_bytes",
		Help: "HTTP request sizes in bytes.",
	}, labels,
)

var promResponseSizeBytes = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "golibs_middleware_server_http_response_size_bytes",
		Help: "HTTP response sizes in bytes.",
	}, labels,
)

type Config struct {
	Enabled   bool     `yaml:"enabled"`
	SkipPaths []string `json:"skipPaths,omitempty" yaml:"skipPaths,omitempty"`
}

// NewMetricsConfig creates a new metrics config with default skip paths and any additional skip paths provided
func NewMetricsConfig(skipPaths []string) *Config {
	mc := &Config{
		SkipPaths: slices.Concat(defaultSkipPaths, skipPaths),
	}
	go func() {
		for range time.Tick(time.Second) {
			uptime.WithLabelValues().Inc()
		}
	}()

	return mc
}

// Middleware returns a Gin middleware handler that records Prometheus metrics
func Middleware(cfg *Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		start := time.Now()

		c.Next()

		if slices.Contains(cfg.SkipPaths, path) {
			return
		}
		status := fmt.Sprintf("%d", c.Writer.Status())

		labelValues := []string{status, c.FullPath(), method}

		// no response content will return -1
		respSize := max(c.Writer.Size(), 0)

		promRequestCount.WithLabelValues(labelValues...).Inc()
		promRequestDuration.WithLabelValues(labelValues...).Observe(float64(time.Since(start).Milliseconds()))
		promRequestSizeBytes.WithLabelValues(labelValues...).Observe(float64(c.Request.ContentLength))
		promResponseSizeBytes.WithLabelValues(labelValues...).Observe(float64(respSize))
	}
}
