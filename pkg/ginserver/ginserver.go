package ginserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/ginserver/proxy"
	"github.com/ing-bank/golibs/pkg/graceful"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/ing-bank/golibs/pkg/reloader"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	"github.com/ing-bank/golibs/pkg/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type Route interface {
	Register(gin.IRouter)
}

type Engine struct {
	*gin.Engine
	HTTPServer *httpserver.Server
	Services   *Service
	errChan    <-chan error
}

func (e *Engine) With(opts ...Option) error {
	return config.ApplyOpts(e, opts...)
}

func New(opts ...Option) (*Engine, error) {
	cfg := DefaultConfig()
	s, err := NewForConfig(cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}
	return s, nil
}

func NewForConfig(c *Config, opts ...Option) (*Engine, error) {
	return NewCustomEngine(c, MiddlewareOption{}, opts...)
}

func NewEngine(c *Config, opts ...Option) (*Engine, error) {
	if c == nil {
		return nil, fmt.Errorf("config is nil")
	}
	cfg := *c // shallow copy

	gin.SetMode(string(cfg.Mode))

	router := gin.New()
	e := &Engine{
		Engine: router,
	}

	// configure the HTTP server with the provided configuration
	serverOptions := config.NewOptions(
		httpserver.WithHandler(router),
	)

	var tlsconfig *tls.Config
	var err error
	if cfg.TLSConfig.UseTLS() {
		tlsconfig, err = tlsserver.NewForConfig(&cfg.TLSConfig)
		if err != nil {
			return nil, err
		}
		serverOptions = append(serverOptions, httpserver.WithTLS(tlsconfig))
	}

	e.HTTPServer, err = httpserver.NewForConfig(&cfg.HTTPServer, serverOptions...)
	if err != nil {
		return nil, err
	}

	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return e, nil
}

func (e *Engine) UseMiddleware(cfg MiddlewareConfig, opts ...MiddlewareOption) error {
	middleware, err := NewMiddlewareForConfig(&cfg, opts...)
	if err != nil {
		return fmt.Errorf("failed to create middleware: %w", err)
	}
	if middleware != nil {
		e.Use(middleware...)
	}
	return nil
}

func NewCustomEngine(c *Config, middlewareOption MiddlewareOption, engineOption ...Option) (*Engine, error) {
	if c == nil {
		return nil, fmt.Errorf("config is nil")
	}
	cfg := *c // shallow copy

	// apply default values to the configuration
	ApplyDefaultConfig(&cfg)
	// validate the server configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	e, err := NewEngine(&cfg, engineOption...)
	if err != nil {
		return nil, err
	}

	// ensure middleware are loaded before any routes are registered
	if err := e.UseMiddleware(cfg.Middleware, middlewareOption); err != nil {
		return nil, err
	}

	// register services (e.g., health checks, metrics, pprof)
	if err := e.RegisterServices(&cfg.ServiceConfig); err != nil {
		return nil, fmt.Errorf("failed to create middleware: %w", err)
	}

	// apply options to gin engine after loading middleware and services
	if err := config.ApplyOpts(e, engineOption...); err != nil {
		return nil, fmt.Errorf("failed to apply Engine option: %w", err)
	}

	if !trace.IsTracerProviderRegistered() && cfg.Middleware.TraceConfig.Enabled {
		return nil, fmt.Errorf("middleware tracing is enabled but no trace provider is configured")
	}

	return e, nil
}

type Service struct {
	prober   *healthcheck.HealthCheck
	reloader *reloader.DeploymentReloader
}

func (e *Engine) PrintRoutes() {
	routes := e.Routes()
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Path < routes[j].Path
	})
	for _, route := range routes {
		log.Infof("[%s] %-6s -> %s", e.HTTPServer.Server.Addr, route.Method, route.Path)
	}
}

func (e *Engine) RegisterServices(cfg *ServiceConfig) error {
	if e.Services == nil {
		e.Services = new(Service)
	}

	if !e.HasHealthChecks() && cfg.Healthcheck.Enabled {
		healthcheckClient, err := healthcheck.NewForConfig(&cfg.Healthcheck.Config)
		if err != nil {
			return err
		}
		e.Services.prober = healthcheckClient
	}

	if cfg.ProxyConfig.Enabled {
		proxycar, err := proxy.NewForConfig(cfg.ProxyConfig.Config)
		if err != nil {
			return err
		}
		e.Register(proxycar)
	}

	if cfg.Reloader.Enabled {
		reloaderClient, err := reloader.NewForConfig(&cfg.Reloader.Config)
		if err != nil {
			return err
		}
		e.Services.reloader = reloaderClient
	}

	if e.HasHealthChecks() && cfg.Healthcheck.Enabled {
		e.Services.prober.Register(e.Engine)
	}
	if cfg.MetricConfig.Enabled {
		e.Engine.GET(cfg.MetricConfig.Path, gin.WrapH(promhttp.Handler()))
	}
	if cfg.PProfConfig.Enabled {
		pprof.Register(e.Engine)
	}

	return nil
}

func (e *Engine) Validate() error {
	if e.HTTPServer == nil {
		return fmt.Errorf("http server is nil")
	}
	return e.HTTPServer.Validate()
}

func (e *Engine) Shutdown(ctx context.Context) error {
	if e == nil || e.HTTPServer == nil {
		return nil
	}
	return e.HTTPServer.Shutdown(ctx)
}

func GroupWithMiddlewares(rg gin.IRouter, apiPath string, middlewares ...gin.HandlerFunc) gin.IRouter {
	prefixRouter := rg.Group(apiPath)
	for _, m := range middlewares {
		prefixRouter.Use(m)
	}
	return prefixRouter
}

// Register registers multiple routes to the engine's router.
func (e *Engine) Register(routes ...Route) {
	RegisterRoutes(e, routes...)
}

// DefaultEngine creates a default Engine instance with default configuration.
// It panics if the engine cannot be created.
func DefaultEngine() *Engine {
	router, err := New()
	if err != nil {
		log.Fatalf("failed to create default router: %v", err)
	}
	return router
}

// Run starts the HTTP server and blocks until it stops or an error occurs.
func (e *Engine) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var err error
	for errCh := range e.RunBackground(ctx) {
		if errCh != nil {
			// stop all background tasks
			cancel()
			// collect all errors
			err = errors.Join(err, errCh)
		}
	}
	return err
}

func (e *Engine) HasHealthChecks() bool {
	return e.Services != nil && e.Services.prober != nil
}

// RunBackground starts the HTTP server in the background and returns a channel to receive errors.
func (e *Engine) RunBackground(ctx context.Context) <-chan error {
	var doFn []func(ctx context.Context) <-chan error
	// run health checks in the background if configured
	if e.HasHealthChecks() {
		doFn = append(doFn, e.Services.prober.RunBackground)
	}
	if e.Services.reloader != nil {
		doFn = append(doFn, e.Services.reloader.RunBackground)
	}

	doFn = append(doFn, e.HTTPServer.RunBackground)
	// print all registered routes
	e.PrintRoutes()

	e.errChan = graceful.RunAllBackgroundFunc(ctx, doFn, graceful.FailFast)

	return e.errChan
}

// Wait blocks until the HTTP server stops and returns any error that occurred.
func (e *Engine) Wait() error {
	if e.errChan == nil {
		return nil
	}
	var err error
	for errCh := range e.errChan {
		if errCh != nil {
			err = errors.Join(err, errCh)
		}
	}
	return err
}

// RegisterRoutes registers multiple routes to the given gin router.
func RegisterRoutes(root gin.IRouter, routes ...Route) {
	for _, r := range routes {
		if r == nil {
			continue
		}
		r.Register(root)
	}
}
