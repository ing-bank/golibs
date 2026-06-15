package manager

import (
	"testing"

	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/certificate"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/npa"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/oauth"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/user"
	"github.com/ing-bank/golibs/pkg/middleware/gzip"
	"github.com/ing-bank/golibs/pkg/middleware/logger"
	"github.com/ing-bank/golibs/pkg/middleware/metrics"
	"github.com/ing-bank/golibs/pkg/middleware/requestid"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyProxyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("sets default host and port", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, DefaultProxyHost, c.HTTPServer.Host)
		assert.Equal(t, DefaultProxyPort, c.HTTPServer.Port)
	})

	t.Run("uses custom host and port when provided", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "custom.example.com",
				Port:    9999,
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, "custom.example.com", c.HTTPServer.Host)
		assert.Equal(t, uint16(9999), c.HTTPServer.Port)
	})

	t.Run("disables metrics when applied", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			Config: ginserver.Config{
				ServiceConfig: ginserver.ServiceConfig{
					MetricConfig: &ginserver.MetricConfig{
						Enabled: true,
					},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.False(t, c.ServiceConfig.MetricConfig.Enabled)
	})

	t.Run("disables client cert authentication when TLS is enabled", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			Config: ginserver.Config{
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
					ClientAuthType: tlsutils.TLSRequireAndVerifyClientCert,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, "NoClientCert", c.TLSConfig.ClientAuthType)
	})

	t.Run("disables certificate auth middleware", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			Config: ginserver.Config{
				Middleware: ginserver.MiddlewareConfig{
					CertificateAuthConfig: &certificate.Config{
						Enabled: true,
					},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.False(t, c.Middleware.CertificateAuthConfig.Enabled)
	})

	t.Run("disables TLS when Override.TLSConfig.Disabled is true", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			Config: ginserver.Config{
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
					ClientAuthType: "RequireAndVerifyClientCert",
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Disabled: true,
						},
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.False(t, c.TLSConfig.UseTLS())
		assert.Empty(t, c.TLSConfig.Cert)
		assert.Empty(t, c.TLSConfig.Key)
	})
}

func TestApplyProxyDefaults_MiddlewareOverrides(t *testing.T) {
	t.Parallel()

	t.Run("overrides OAuth config when provided", func(t *testing.T) {
		t.Parallel()
		customOAuth := &oauth.Config{
			Enabled: true,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						OauthConfig: customOAuth,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customOAuth, c.Middleware.OauthConfig)
		assert.True(t, c.Middleware.OauthConfig.Enabled)
	})

	t.Run("overrides GZIP config when provided", func(t *testing.T) {
		t.Parallel()
		customGZIP := &gzip.Config{
			Enabled:         true,
			CompressionMode: gzip.BestCompression,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						GZIPConfig: customGZIP,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customGZIP, c.Middleware.GZIPConfig)
		assert.True(t, c.Middleware.GZIPConfig.Enabled)
		assert.Equal(t, gzip.BestCompression, c.Middleware.GZIPConfig.CompressionMode)
	})

	t.Run("overrides CertificateAuth config when provided", func(t *testing.T) {
		t.Parallel()
		customCertAuth := &certificate.Config{
			Enabled: true,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						CertificateAuthConfig: customCertAuth,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customCertAuth, c.Middleware.CertificateAuthConfig)
	})

	t.Run("overrides Recovery config when provided", func(t *testing.T) {
		t.Parallel()
		customRecovery := &ginserver.RecoveryConfig{
			Enabled: false,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						RecoveryConfig: customRecovery,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customRecovery, c.Middleware.RecoveryConfig)
		assert.False(t, c.Middleware.RecoveryConfig.Enabled)
	})

	t.Run("overrides Metrics config when provided", func(t *testing.T) {
		t.Parallel()
		customMetrics := &metrics.Config{
			Enabled:   false,
			SkipPaths: []string{"/custom"},
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						MetricsConfig: customMetrics,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customMetrics, c.Middleware.MetricsConfig)
		assert.False(t, c.Middleware.MetricsConfig.Enabled)
		assert.Equal(t, []string{"/custom"}, c.Middleware.MetricsConfig.SkipPaths)
	})

	t.Run("overrides Logger config when provided", func(t *testing.T) {
		t.Parallel()
		customLogger := &logger.Config{
			Enabled:   true,
			SkipPaths: []string{"/logs"},
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						LoggerConfig: customLogger,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customLogger, c.Middleware.LoggerConfig)
	})

	t.Run("overrides Trace config when provided", func(t *testing.T) {
		t.Parallel()
		customTrace := &ginserver.TraceConfig{
			Enabled:     true,
			ServiceName: "custom-service",
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						TraceConfig: customTrace,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customTrace, c.Middleware.TraceConfig)
		assert.Equal(t, "custom-service", c.Middleware.TraceConfig.ServiceName)
	})

	t.Run("overrides RequestID config when provided", func(t *testing.T) {
		t.Parallel()
		customRequestID := &requestid.Config{
			Enabled: false,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						RequestIDConfig: customRequestID,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customRequestID, c.Middleware.RequestIDConfig)
	})

	t.Run("overrides UserAuth config when provided", func(t *testing.T) {
		t.Parallel()
		customUserAuth := &user.Config{
			Enabled: true,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						UserAuthConfig: customUserAuth,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customUserAuth, c.Middleware.UserAuthConfig)
	})

	t.Run("overrides NPAAuth config when provided", func(t *testing.T) {
		t.Parallel()
		customNPAAuth := &npa.Config{
			Enabled: true,
		}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						NPAAuthConfig: customNPAAuth,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customNPAAuth, c.Middleware.NPAAuthConfig)
	})

	t.Run("multiple middleware overrides work together", func(t *testing.T) {
		t.Parallel()
		customOAuth := &oauth.Config{Enabled: true}
		customGZIP := &gzip.Config{Enabled: true, CompressionMode: gzip.BestSpeed}
		customLogger := &logger.Config{Enabled: false}

		c := &Config{
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						OauthConfig:  customOAuth,
						GZIPConfig:   customGZIP,
						LoggerConfig: customLogger,
					},
				},
			},
		}

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		assert.Equal(t, customOAuth, c.Middleware.OauthConfig)
		assert.Equal(t, customGZIP, c.Middleware.GZIPConfig)
		assert.Equal(t, customLogger, c.Middleware.LoggerConfig)
	})

	t.Run("nil override middleware configs are not applied", func(t *testing.T) {
		t.Parallel()
		c := &Config{
			Config: ginserver.Config{
				Middleware: ginserver.MiddlewareConfig{
					OauthConfig: &oauth.Config{Enabled: false},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Override: ginserver.Config{
					Middleware: ginserver.MiddlewareConfig{
						OauthConfig: nil, // nil should not override
					},
				},
			},
		}

		// Store original config
		originalOAuth := c.Middleware.OauthConfig

		err := ApplyProxyDefaults(c)
		require.NoError(t, err)

		// Original should remain since override is nil
		assert.Equal(t, originalOAuth, c.Middleware.OauthConfig)
	})
}
