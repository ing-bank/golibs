package httpserver

import (
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
)

type Option = config.Option[*Server]

func WithHTTPServer(server *http.Server) config.Opt[*Server] {
	return func(s *Server) error {
		if server == nil {
			return errors.New("http.Server cannot be nil")
		}
		s.Server = server
		return nil
	}
}

func WithShutdownTimeout(timeout time.Duration) config.Opt[*Server] {
	return func(server *Server) error {
		if timeout <= 0 {
			return errors.New("shutdownTimeout must be a positive duration")
		}
		server.shutdownTimeout = timeout
		return nil
	}
}

func WithHandler(handler http.Handler) config.Opt[*Server] {
	return func(s *Server) error {
		if s.Server == nil {
			return errors.New("http.Server is not initialized")
		}
		if handler == nil {
			return errors.New("handler cannot be nil")
		}
		s.Server.Handler = handler
		return nil
	}
}

func WithTLS(tlsConfig *tls.Config) config.Opt[*Server] {
	return func(s *Server) error {
		if s.Server == nil {
			return errors.New("http.Server is not initialized")
		}
		if tlsConfig == nil {
			return errors.New("tls.Config cannot be nil")
		}
		s.Server.TLSConfig = tlsConfig
		return nil
	}
}

func WithAddr(addr string) config.Opt[*Server] {
	return func(s *Server) error {
		if s.Server == nil {
			return errors.New("http.Server is not initialized")
		}
		if addr == "" {
			return errors.New("address cannot be empty")
		}
		s.Server.Addr = addr
		return nil
	}
}
