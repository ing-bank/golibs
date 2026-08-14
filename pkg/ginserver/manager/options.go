package manager

import (
	"fmt"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/ginserver"
)

type Option = config.Option[*Manager]

func WithWebserverOptions(opts ...config.Option[*ginserver.Engine]) Option {
	return func(m *Manager) error {
		return m.Webserver.With(opts...)
	}
}

func WithSidecarOptions(opts ...config.Option[*ginserver.Engine]) Option {
	return func(m *Manager) error {
		return m.Sidecar.With(opts...)
	}
}

func WithProxycarOptions(opts ...config.Option[*ginserver.Engine]) Option {
	return func(m *Manager) error {
		if m.Proxy != nil {
			return m.Proxy.With(opts...)
		}
		if m.proxyInitialized {
			return fmt.Errorf("proxycar is not enabled")
		}
		m.proxyOptions = append(m.proxyOptions, opts...)
		return nil
	}
}
