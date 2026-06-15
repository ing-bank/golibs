package manager

import (
	"crypto/tls"

	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/ginserver/proxy"
	"github.com/ing-bank/golibs/pkg/tlsserver"
)

const (
	DefaultProxyHost        = "127.0.0.1"
	DefaultProxyPort uint16 = 9092
)

type ProxyConfig struct {
	Host         string `json:"host" yaml:"host"`
	Port         uint16 `json:"port" yaml:"port"`
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	proxy.Config `json:",inline" yaml:",inline"`
	Override     ginserver.Config `json:"override" yaml:"override"`
}

// ApplyProxyDefaults applies default values to the proxycar configuration
func ApplyProxyDefaults(c *Config) error {
	// apply default values to the configuration
	if err := c.ApplyDefaults(); err != nil {
		return err
	}

	// metrics are exposed on the sidecar
	if c.ServiceConfig.MetricConfig != nil {
		c.ServiceConfig.MetricConfig.Enabled = false
	}

	// these services are not supported for proxycar
	c.ServiceConfig.ProxyConfig = nil
	c.ServiceConfig.Reloader = nil
	c.ServiceConfig.MetricConfig.Enabled = false

	c.HTTPServer.Host = DefaultProxyHost
	if c.ProxyConfig.Host != "" {
		c.HTTPServer.Host = c.ProxyConfig.Host
	}
	c.HTTPServer.Port = DefaultProxyPort
	if c.ProxyConfig.Port > 0 {
		c.HTTPServer.Port = c.ProxyConfig.Port
	}

	// bypass mTLS certificate verification for the proxy server
	if c.TLSConfig.UseTLS() {
		c.TLSConfig.ClientAuthType = tls.NoClientCert.String()
	}
	if c.Middleware.CertificateAuthConfig != nil {
		c.Middleware.CertificateAuthConfig.Enabled = false
	}

	// disable https if required
	if c.ProxyConfig.Override.TLSConfig.Disabled {
		c.TLSConfig = tlsserver.Config{}
	}

	// apply override configuration if provided
	if c.ProxyConfig.Override.Middleware.OauthConfig != nil {
		c.Middleware.OauthConfig = c.ProxyConfig.Override.Middleware.OauthConfig
	}
	if c.ProxyConfig.Override.Middleware.GZIPConfig != nil {
		c.Middleware.GZIPConfig = c.ProxyConfig.Override.Middleware.GZIPConfig
	}
	if c.ProxyConfig.Override.Middleware.CertificateAuthConfig != nil {
		c.Middleware.CertificateAuthConfig = c.ProxyConfig.Override.Middleware.CertificateAuthConfig
	}
	if c.ProxyConfig.Override.Middleware.RecoveryConfig != nil {
		c.Middleware.RecoveryConfig = c.ProxyConfig.Override.Middleware.RecoveryConfig
	}
	if c.ProxyConfig.Override.Middleware.MetricsConfig != nil {
		c.Middleware.MetricsConfig = c.ProxyConfig.Override.Middleware.MetricsConfig
	}
	if c.ProxyConfig.Override.Middleware.LoggerConfig != nil {
		c.Middleware.LoggerConfig = c.ProxyConfig.Override.Middleware.LoggerConfig
	}
	if c.ProxyConfig.Override.Middleware.TraceConfig != nil {
		c.Middleware.TraceConfig = c.ProxyConfig.Override.Middleware.TraceConfig
	}
	if c.ProxyConfig.Override.Middleware.RequestIDConfig != nil {
		c.Middleware.RequestIDConfig = c.ProxyConfig.Override.Middleware.RequestIDConfig
	}
	if c.ProxyConfig.Override.Middleware.UserAuthConfig != nil {
		c.Middleware.UserAuthConfig = c.ProxyConfig.Override.Middleware.UserAuthConfig
	}
	if c.ProxyConfig.Override.Middleware.NPAAuthConfig != nil {
		c.Middleware.NPAAuthConfig = c.ProxyConfig.Override.Middleware.NPAAuthConfig
	}

	return nil
}
