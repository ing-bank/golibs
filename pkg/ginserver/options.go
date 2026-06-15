package ginserver

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/healthcheck"
)

type Option = config.Option[*Engine]

// WithMode sets the gin mode for the engine
func WithMode(mode GinMode) config.Opt[*Engine] {
	return func(e *Engine) error {
		gin.SetMode(string(mode))
		return nil
	}
}

// WithHealthChecks sets up the engine's health checker with the provided options
func WithHealthChecks(opts ...healthcheck.Option) config.Opt[*Engine] {
	return func(e *Engine) error {
		if e.Services == nil {
			e.Services = new(Service)
		}
		if e.Services.prober == nil {
			h, err := healthcheck.New(opts...)
			if err != nil {
				return err
			}
			e.Services.prober = h
			return nil
		}
		return e.Services.prober.With(opts...)
	}
}

// WithHealthCheckAdd adds the provided health check jobs to the engine's prober
func WithHealthCheckAdd(jobs ...healthcheck.JobConfig) config.Opt[*Engine] {
	return func(e *Engine) error {
		if e.Services == nil {
			return fmt.Errorf("services not initialized")
		}
		if e.Services.prober == nil {
			return fmt.Errorf("prober not initialized")
		}
		return e.Services.prober.Add(jobs...)
	}
}

// WithRoutes registers the provided routes with the gin engine
func WithRoutes(routes ...Route) config.Opt[*Engine] {
	return func(e *Engine) error {
		for _, r := range routes {
			r.Register(e.Engine)
		}
		return nil
	}
}

// WithAddr sets the address for the gin engine
func WithAddr(addr string) config.Opt[*Engine] {
	return func(e *Engine) error {
		e.HTTPServer.Addr = addr
		return nil
	}
}

// WithServices sets the services for the gin engine
func WithServices(s *Service) config.Opt[*Engine] {
	return func(e *Engine) error {
		e.Services = s
		return nil
	}
}

// WithMiddleware adds the provided middleware handlers to the gin engine
func WithMiddleware(m ...gin.HandlerFunc) config.Opt[*Engine] {
	return func(e *Engine) error {
		e.Use(m...)
		return nil
	}
}
