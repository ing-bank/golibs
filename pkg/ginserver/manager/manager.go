package manager

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/ginserver/proxy"
	"github.com/ing-bank/golibs/pkg/graceful"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	httpcheck "github.com/ing-bank/golibs/pkg/healthcheck/checks/http"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks/telnet"
	"github.com/ing-bank/golibs/pkg/task/job"
	"github.com/ing-bank/golibs/pkg/tlsclient"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultSidecarHost = "127.0.0.1"
	DefaultSidecarPort = 9090
)

type Config struct {
	ginserver.Config
	SidecarServiceConfig ginserver.ServiceConfig `yaml:"sidecarServices" json:"sidecarServices"`
	SidecarConfig        SidecarConfig           `yaml:"sidecar" json:"sidecar"`
	ProxyConfig          ProxyConfig             `yaml:"proxy" json:"proxy"`
}

type SidecarConfig struct {
	Host string `json:"host" yaml:"host"`
	Port uint16 `json:"port" yaml:"port"`
}

type Manager struct {
	Webserver       *ginserver.Engine
	Sidecar         *ginserver.Engine
	Proxy           *ginserver.Engine
	shutdownTimeout time.Duration
}

func newHTTPCheck(name, desc, host string, port uint16, tlsConfig tlsclient.Config) healthcheck.JobConfig {
	return healthcheck.JobConfig{
		Config: job.Config{
			Name:        name,
			Description: desc,
			Timeout:     3 * time.Second,
		},
		ProbeHandler: healthcheck.ProbeHandler{
			HTTPGet: &httpcheck.Config{
				Host:      host,
				Port:      port,
				Path:      healthcheck.HealthEndpoint.String(),
				TLSConfig: tlsConfig,
			},
		},
		Endpoints: []healthcheck.Endpoint{
			healthcheck.HealthEndpoint,
			healthcheck.ReadyEndpoint,
		},
	}
}

func newTelnetCheck(name, desc, address string, tlsConfig tlsclient.Config) healthcheck.JobConfig {
	return healthcheck.JobConfig{
		Config: job.Config{
			Name:        name,
			Description: desc,
			Timeout:     1 * time.Second,
		},
		ProbeHandler: healthcheck.ProbeHandler{
			Telnet: &telnet.Config{
				Address:   address,
				TLSConfig: tlsConfig,
			},
		},
		Endpoints: []healthcheck.Endpoint{
			healthcheck.HealthEndpoint,
			healthcheck.ReadyEndpoint,
		},
	}
}

func NewManager(c *Config, opts ...Option) (*Manager, error) {
	if c == nil {
		return nil, fmt.Errorf("config is nil")
	}
	webserverConfig := *c // shallow copy
	// apply default values to the configuration
	if err := webserverConfig.ApplyDefaults(); err != nil {
		return nil, err
	}

	// metrics are exposed on the sidecar
	if webserverConfig.ServiceConfig.MetricConfig != nil {
		webserverConfig.ServiceConfig.MetricConfig.Enabled = false
	}

	webserver, err := ginserver.NewForConfig(&webserverConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	sidecarConfig := *c // shallow copy

	// apply default values to the sidecar configuration
	if err := ApplySidecarDefaults(&sidecarConfig); err != nil {
		return nil, fmt.Errorf("failed to apply sidecar defaults: %w", err)
	}

	// inject TLS config into HTTPGet probes so that they can perform health checks
	if err := injectConfig(&sidecarConfig, &webserverConfig); err != nil {
		return nil, fmt.Errorf("failed to apply default sidecar configuration: %w", err)
	}

	sidecar, err := ginserver.NewForConfig(&sidecarConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	proxyConfig := *c // shallow copy
	proxycar, err := newProxyCar(proxyConfig, &webserverConfig)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		Webserver:       webserver,
		Sidecar:         sidecar,
		shutdownTimeout: webserverConfig.HTTPServer.ShutdownTimeout.Duration,
		Proxy:           proxycar,
	}

	// apply options to gin engine
	if err := config.ApplyOpts(m, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply Engine option: %w", err)
	}

	return m, nil
}

func newProxyCar(proxyConfig Config, mainConfig *Config) (*ginserver.Engine, error) {
	if mainConfig.ProxyConfig.Enabled && len(mainConfig.ProxyConfig.Config.Routes) > 0 {
		// apply default values to the configuration
		if err := ApplyProxyDefaults(&proxyConfig); err != nil {
			return nil, fmt.Errorf("failed to apply proxy defaults: %w", err)
		}

		if err := injectProxyConfig(&proxyConfig, mainConfig); err != nil {
			return nil, fmt.Errorf("failed to apply default sidecar configuration: %w", err)
		}
		proxyclient, err := proxy.NewForConfig(proxyConfig.ProxyConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy client: %w", err)
		}
		proxycar, err := ginserver.NewForConfig(&proxyConfig.Config, ginserver.WithRoutes(proxyclient))
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy server: %w", err)
		}
		return proxycar, nil
	}
	return nil, nil
}

func injectProxyConfig(c *Config, webserverConfig *Config) error {
	// inject TLS to the proxy client to perform mTLS calls to the webserver if mTLS is enabled on the webserver
	if webserverConfig.TLSConfig.UseTLS() {
		c.ProxyConfig.Config.HTTPConfig.TLS = reuseTLSConfig(webserverConfig.TLSConfig)
	}
	return nil
}

// ApplySidecarDefaults applies default values to the sidecar configuration
func ApplySidecarDefaults(c *Config) error {
	// apply default values to the configuration
	if err := c.ApplyDefaults(); err != nil {
		return err
	}

	c.HTTPServer.Host = DefaultSidecarHost
	c.HTTPServer.Port = DefaultSidecarPort

	if c.SidecarConfig.Host != "" {
		c.HTTPServer.Host = c.SidecarConfig.Host
	}
	if c.SidecarConfig.Port > 0 {
		c.HTTPServer.Port = c.SidecarConfig.Port
	}

	if c.TLSConfig.UseTLS() {
		// sidecar should not request client certificates
		// even when mTLS is enabled on the main webserver
		// as it is not supposed to be called by clients directly
		// but only perform health checks against the webserver
		// and call external services
		c.TLSConfig.ClientAuthType = tls.NoClientCert.String()
	}

	// remove all middleware for the sidecar
	// as it is not supposed to be called by clients directly
	// but only perform health checks against the webserver
	// and call external services.
	// custom middleware can be added via WithSidecarOptions
	c.Middleware = ginserver.MiddlewareConfig{
		RecoveryConfig: &ginserver.RecoveryConfig{
			Enabled: true,
		},
	}
	return nil
}

// createDefaultHealthChecks creates default health check jobs for the webserver and optionally the proxycar.
// It returns a slice of JobConfig that includes:
// - A health check for the main webserver (HTTP if healthcheck enabled, Telnet otherwise)
// - A health check for the proxycar (if proxy is enabled)
func createDefaultHealthChecks(source *Config) []healthcheck.JobConfig {
	webserverAddr := fmt.Sprintf("%s:%d", source.HTTPServer.Host, source.HTTPServer.Port)

	var defaultWebserverCheck healthcheck.JobConfig
	if source.ServiceConfig.Healthcheck != nil && source.ServiceConfig.Healthcheck.Enabled {
		defaultWebserverCheck = newHTTPCheck("default-main-webserver-check", "Default HTTP health check for main webserver",
			source.HTTPServer.Host, source.HTTPServer.Port, reuseTLSConfig(source.TLSConfig))
	} else {
		defaultWebserverCheck = newTelnetCheck("default-main-webserver-check", "Default telnet health check for main webserver",
			webserverAddr, reuseTLSConfig(source.TLSConfig))
	}

	jobs := []healthcheck.JobConfig{defaultWebserverCheck}

	// Add proxycar health check if proxy is enabled
	if source.ProxyConfig.Enabled {
		proxycarAddr := fmt.Sprintf("%s:%d", source.ProxyConfig.Host, source.ProxyConfig.Port)

		var defaultProxycarCheck healthcheck.JobConfig
		var proxyHasHealthCheck = source.ProxyConfig.Override.ServiceConfig.Healthcheck != nil && source.ProxyConfig.Override.ServiceConfig.Healthcheck.Enabled
		var proxyInheritHealthCheck = source.ProxyConfig.Override.ServiceConfig.Healthcheck == nil && source.ServiceConfig.Healthcheck != nil && source.ServiceConfig.Healthcheck.Enabled

		var tlsconfig tlsserver.Config
		if source.ProxyConfig.Override.TLSConfig.Disabled {
			tlsconfig = tlsserver.Config{}
		} else {
			tlsconfig = source.TLSConfig
		}

		if proxyHasHealthCheck || proxyInheritHealthCheck {
			defaultProxycarCheck = newHTTPCheck("default-proxy-check", "Default HTTP health check for ProxyCar",
				source.ProxyConfig.Host, source.ProxyConfig.Port, reuseTLSConfig(tlsconfig))
		} else {
			defaultProxycarCheck = newTelnetCheck("default-proxy-check", "Default telnet health check for ProxyCar",
				proxycarAddr, reuseTLSConfig(tlsconfig))
		}

		jobs = append(jobs, defaultProxycarCheck)
	}

	return jobs
}

// injectConfig injects the source's TLS configuration into the dest's health check probes.
// This allows the dest to perform health checks against a webserver with mTLS enabled.
// It also adds a default telnet health check if no health checks are configured.
// The function modifies the dest Config in place.
func injectConfig(dest *Config, source *Config) error {
	// address of the main webserver
	webserverAddr := fmt.Sprintf("%s:%d", source.HTTPServer.Host, source.HTTPServer.Port)

	if dest.SidecarServiceConfig.Healthcheck != nil {
		for i := range dest.SidecarServiceConfig.Healthcheck.Jobs {
			probe := &dest.SidecarServiceConfig.Healthcheck.Jobs[i]

			if probe.HTTPGet != nil {
				// only inject address if not already set or if it matches the webserver address
				var noAddress = probe.HTTPGet.Host == "" && probe.HTTPGet.Port == 0
				// or if it matches the webserver address (host and port)
				var sameAddress = probe.HTTPGet.Host == source.HTTPServer.Host && probe.HTTPGet.Port == source.HTTPServer.Port
				if noAddress || sameAddress {
					if err := probe.HTTPGet.WithAddress(webserverAddr); err != nil {
						return fmt.Errorf("failed to upgrade HTTPGet probe address: %w", err)
					}
				}

				// only inject TLS config if not already set
				if probe.HTTPGet.Scheme != "http" && !probe.HTTPGet.TLSConfig.UseTLS() {
					// infer scheme from webserver config
					if source.TLSConfig.UseTLS() {
						probe.HTTPGet.TLSConfig = reuseTLSConfig(source.TLSConfig)
					}
				}

				// if Oauth middleware is enabled, add probe authorization header with context token
				// only if the probe is targeting the webserver itself
				if sameAddress && source.Middleware.OauthConfig != nil && source.Middleware.OauthConfig.Enabled {
					if probe.HTTPGet.HTTPHeaders == nil {
						probe.HTTPGet.HTTPHeaders = make([]map[string]string, 0)
					}
					probe.HTTPGet.HTTPHeaders = append(probe.HTTPGet.HTTPHeaders, map[string]string{
						"Authorization": httpcheck.UseContextToken,
					})
				}
			}

			if probe.Telnet != nil {
				// only inject address if not already set or if it matches the webserver address
				var sameAddress = probe.Telnet.Address == webserverAddr
				// TODO: add tcps and udps protocol support to differentiate between tcp and tls
				if sameAddress && !probe.Telnet.TLSConfig.UseTLS() {
					if source.TLSConfig.UseTLS() {
						probe.Telnet.TLSConfig = reuseTLSConfig(source.TLSConfig)
					}
				}
			}
		}
	}

	// add default health check if none configured
	if dest.SidecarServiceConfig.Healthcheck == nil {
		dest.SidecarServiceConfig.Healthcheck = &ginserver.HealthCheckConfig{
			Enabled: true,
			Config: healthcheck.Config{
				Jobs:       createDefaultHealthChecks(source),
				Interval:   metav1.Duration{Duration: 5 * time.Second},
				SystemInfo: true,
			},
		}
	}

	// use sidecar service config for the sidecar server to initialize
	// e.g., health checks, metrics, pprof, etc.
	// the sidecar server listens on a different port (DefaultSidecarAddr)
	dest.ServiceConfig = dest.SidecarServiceConfig

	return nil
}

// reuseTLSConfig creates a new tlsclient.Config from a tlsserver.Config.
// It's used to configure health check clients to trust the webserver's certificate.
// InsecureSkipVerify is set to true to simplify health checks against mTLS-enabled servers.
func reuseTLSConfig(s tlsserver.Config) tlsclient.Config {
	return *tlsclient.NewConfig(s.Cert, s.Key, true, s.RootCAs...)
}

func (m *Manager) With(opts ...Option) error {
	return config.ApplyOpts(m, opts...)
}

func (m *Manager) Run(ctx context.Context) error {
	return graceful.RunAllBackgroundFuncE(ctx, []func(ctx context.Context) <-chan error{m.RunBackground})
}

func (m *Manager) RunBackground(ctx context.Context) <-chan error {
	runAll := []func(ctx context.Context) <-chan error{
		m.Webserver.RunBackground,
		m.Sidecar.RunBackground,
	}
	if m.Proxy != nil {
		runAll = append(runAll, m.Proxy.RunBackground)
	}
	return graceful.RunAllBackgroundFunc(ctx, runAll, graceful.NewRunAllOptions(true, m.shutdownTimeout))
}
