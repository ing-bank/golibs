package manager

import (
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/ginserver"
)

type Option = config.Option[*Manager]

func WithWebserverOptions(opts ...config.Option[*ginserver.Engine]) config.Opt[*Manager] {
	return func(m *Manager) error {
		return m.Webserver.With(opts...)
	}
}

func WithSidecarOptions(opts ...config.Option[*ginserver.Engine]) config.Opt[*Manager] {
	return func(m *Manager) error {
		return m.Sidecar.With(opts...)
	}
}

func WithProxycarOptions(opts ...config.Option[*ginserver.Engine]) config.Opt[*Manager] {
	return func(m *Manager) error {
		return m.Proxy.With(opts...)
	}
}
