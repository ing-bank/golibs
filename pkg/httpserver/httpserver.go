package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/graceful"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	*http.Server
	shutdownTimeout time.Duration
	errChan         <-chan error
}

func New(opts ...Option) (*Server, error) {
	cfg := DefaultConfig()
	s, err := NewForConfig(cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}
	return s, nil
}

func NewForConfig(cfg *Config, opts ...Option) (*Server, error) {
	// apply default values to the configuration
	cfg.ApplyDefaults()
	// validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// configure the HTTP server with the provided configuration
	s := &Server{
		Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			ReadTimeout:       cfg.ReadTimeout.Duration,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout.Duration,
			WriteTimeout:      cfg.WriteTimeout.Duration,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		},
		shutdownTimeout: cfg.ShutdownTimeout.Duration,
		errChan:         make(chan error, 1),
	}

	// apply options to the server
	if err := config.ApplyOpts(s, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply TLS option: %w", err)
	}

	// validate the  server configuration
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return s, nil
}

func DefaultServerConfig() *Server {
	return &Server{
		Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", DefaultHost, DefaultPort),
			ReadHeaderTimeout: time.Duration(DefaultReadHeaderTimeout),
			ReadTimeout:       time.Duration(DefaultReadTimeout),
			WriteTimeout:      time.Duration(DefaultWriteTimeout),
			MaxHeaderBytes:    DefaultMaxHeaderBytes,
		},
		shutdownTimeout: time.Duration(DefaultShutdownTimeout),
	}
}

func (h *Server) ApplyDefaults() *Server {
	if h.Server == nil {
		h.Server = &http.Server{} //nolint // ReadHeaderTimeout is set later in ApplyDefaults
	}
	if h.Server.Addr == "" {
		h.Server.Addr = fmt.Sprintf("%s:%d", DefaultHost, DefaultPort)
	}
	if h.Server.ReadTimeout <= 0 {
		h.Server.ReadTimeout = time.Duration(DefaultReadTimeout)
	}
	if h.Server.ReadHeaderTimeout <= 0 {
		h.Server.ReadHeaderTimeout = time.Duration(DefaultReadHeaderTimeout)
	}
	if h.Server.WriteTimeout <= 0 {
		h.Server.WriteTimeout = time.Duration(DefaultWriteTimeout)
	}
	if h.Server.MaxHeaderBytes <= 0 {
		h.Server.MaxHeaderBytes = DefaultMaxHeaderBytes
	}
	if h.shutdownTimeout <= 0 {
		h.shutdownTimeout = time.Duration(DefaultShutdownTimeout)
	}
	return h
}

func (h *Server) Validate() error {
	if h.Server == nil {
		return errors.New("httpServer cannot be nil")
	}
	if h.shutdownTimeout <= 0 {
		return errors.New("shutdownTimeout must be a positive duration")
	}
	if h.Server.Addr == "" {
		return errors.New("httpServer address cannot be empty")
	}
	if h.Server.ReadHeaderTimeout <= 0 {
		return errors.New("httpServer ReadHeaderTimeout must be a positive duration")
	}
	if h.Server.WriteTimeout <= 0 {
		return errors.New("httpServer WriteTimeout must be a positive duration")
	}
	if h.Server.MaxHeaderBytes <= 0 {
		return errors.New("httpServer MaxHeaderBytes must be a positive integer")
	}
	if h.Server.TLSConfig != nil && h.Server.TLSConfig.MinVersion < tls.VersionTLS12 {
		return errors.New("httpServer TLSConfig must have a minimum version of TLS 1.2")
	}
	return nil
}

func (h *Server) RegisterRoutes(routes ...http.Handler) {
	for _, route := range routes {
		h.Server.Handler.(*gin.Engine).Use(gin.WrapH(route))
	}
}

func Run(ctx context.Context, r http.Handler) error {
	// Create a new HTTP server
	s := defaultServer().withHandler(r)

	// Run the server with the provided context
	return run(ctx, s.Server, s.shutdownTimeout)
}

func RunTLS(ctx context.Context, r http.Handler, tlsConfig *tls.Config) error {
	// Create a new HTTP server
	s, err := New(WithHandler(r), WithTLS(tlsConfig))
	if err != nil {
		return err
	}
	return run(ctx, s.Server, s.shutdownTimeout)
}

func (h *Server) Run(ctx context.Context) error {
	return run(ctx, h.Server, h.shutdownTimeout)
}

func (h *Server) RunBackground(ctx context.Context) <-chan error {
	h.errChan = graceful.RunBackground(ctx, func(ctx context.Context) error {
		return run(ctx, h.Server, h.shutdownTimeout)
	})
	return h.errChan
}

func (h *Server) Wait() error {
	if h.errChan == nil {
		return nil
	}
	return <-h.errChan
}

func run(ctx context.Context, httpServer *http.Server, shutdownTimeout time.Duration) error {
	if httpServer.Handler == nil {
		return errors.New("httpServer handler cannot be nil")
	}
	return graceful.Run(ctx, func(ctx context.Context) error {
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			if httpServer.TLSConfig != nil {
				log.WithContext(ctx).Infof("starting HTTPS server on %s...", httpServer.Addr)
				return httpServer.ListenAndServeTLS("", "")
			}
			log.WithContext(ctx).Infof("starting HTTP server on %s...", httpServer.Addr)
			return httpServer.ListenAndServe()
		})

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Wait for context cancellation and gracefully shut down the server
		g.Go(func() error {
			<-ctx.Done()
			log.WithContext(ctx).Info("shutting down server...")
			return httpServer.Shutdown(shutdownCtx)
		})

		if err := g.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		log.WithContext(ctx).Info("server shutdown complete")

		return nil
	}, graceful.NewRunOptions(shutdownTimeout))
}

func defaultServer() *Server {
	server, err := New()
	if err != nil {
		log.Fatalf("Failed to create HTTP server: %v", err)
	}
	return server
}

func (h *Server) withHandler(handler http.Handler) *Server {
	if handler == nil {
		log.Warn("Handler is nil, using default handler")
		handler = http.DefaultServeMux
	}
	h.Server.Handler = handler
	return h
}
