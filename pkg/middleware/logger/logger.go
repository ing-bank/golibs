package logger

import (
	"os"
	"time"

	"github.com/ing-bank/golibs/pkg/slices"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var DefaultSkipPaths = defaultSkipPaths

var defaultSkipPaths = []string{"/metrics", "/healthz", "/readyz", "/swagger", "/status"}

// Config defines the config for Logger middleware.
type Config struct {
	Enabled bool `yaml:"enabled"`
	// SkipPaths defines a list of paths to skip logging.
	// Optional.
	SkipPaths []string `json:"skipPaths,omitempty" yaml:"skipPaths,omitempty"`
}

func NewLogger(skipPaths []string) *Config {
	loggerConfig := Config{
		SkipPaths: slices.Concat(defaultSkipPaths, skipPaths),
	}
	return &loggerConfig
}

// Middleware returns a gin.HandlerFunc (middleware) that logs requests using logrus.
func Middleware(cfg *Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// other handler can change c.Path
		path := c.Request.URL.Path

		// Process Request
		c.Next()

		// skip paths
		if slices.Contains(cfg.SkipPaths, path) {
			return
		}

		// Stop timer
		stop := time.Since(start)
		latency := stop.Milliseconds()

		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		clientUserAgent := c.Request.UserAgent()
		referer := c.Request.Referer()
		dataLength := max(c.Writer.Size(), 0)

		entry := log.WithContext(c.Request.Context()).WithFields(log.Fields{
			"hostname":   hostname,
			"statusCode": statusCode,
			"latency":    latency,
			"clientIP":   clientIP,
			"method":     c.Request.Method,
			"path":       path,
			"referer":    referer,
			"dataLength": dataLength,
			"userAgent":  clientUserAgent,
		})

		if len(c.Errors) > 0 {
			entry.Error(c.Errors.ByType(gin.ErrorTypePrivate).String())
		} else {
			entry.Infof("[Server] we replied with %d", statusCode)
		}
	}
}
