package ginserver

import (
	"reflect"
	"testing"

	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/certificate"
	"github.com/ing-bank/golibs/pkg/middleware/logger"
	"github.com/ing-bank/golibs/pkg/middleware/metrics"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Mode != DefaultMode {
		t.Errorf("DefaultConfig.Mode = %v, want %v", cfg.Mode, DefaultMode)
	}
	if cfg.ServiceConfig.PProfConfig == nil || cfg.ServiceConfig.PProfConfig.Enabled != DefaultPProfEnabled {
		t.Errorf("DefaultConfig.ServiceConfig.PProfConfig.Enabled = %v, want %v", cfg.ServiceConfig.PProfConfig.Enabled, DefaultPProfEnabled)
	}
	if cfg.Middleware.MetricsConfig == nil || !reflect.DeepEqual(cfg.Middleware.MetricsConfig.SkipPaths, metrics.DefaultSkipPaths) {
		t.Errorf("DefaultConfig.MetricsConfig.SkipPaths = %v, want %v", cfg.Middleware.MetricsConfig.SkipPaths, metrics.DefaultSkipPaths)
	}
	if cfg.Middleware.LoggerConfig == nil || !reflect.DeepEqual(cfg.Middleware.LoggerConfig.SkipPaths, logger.DefaultSkipPaths) {
		t.Errorf("DefaultConfig.LoggerConfig.SkipPaths = %v, want %v", cfg.Middleware.LoggerConfig.SkipPaths, logger.DefaultSkipPaths)
	}
}

func TestApplyDefaults_NilConfig(t *testing.T) {
	var cfg *Config
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ApplyDefaultConfig(nil) did not panic as expected")
		}
	}()
	ApplyDefaultConfig(cfg)
}

func TestApplyDefaults_EmptyFields(t *testing.T) {
	cfg := &Config{}
	ApplyDefaultConfig(cfg)
	if cfg.Mode != DefaultMode {
		t.Errorf("ApplyDefaultConfig(cfg).Mode = %v, want %v", cfg.Mode, DefaultMode)
	}
	if cfg.Middleware.MetricsConfig == nil || !reflect.DeepEqual(cfg.Middleware.MetricsConfig.SkipPaths, metrics.DefaultSkipPaths) {
		t.Errorf("ApplyDefaultConfig(cfg).MetricsConfig.SkipPaths = %v, want %v", cfg.Middleware.MetricsConfig.SkipPaths, metrics.DefaultSkipPaths)
	}
	if cfg.Middleware.LoggerConfig == nil || !reflect.DeepEqual(cfg.Middleware.LoggerConfig.SkipPaths, logger.DefaultSkipPaths) {
		t.Errorf("ApplyDefaultConfig(cfg).LoggerConfig.SkipPaths = %v, want %v", cfg.Middleware.LoggerConfig.SkipPaths, logger.DefaultSkipPaths)
	}
}

func TestApplyDefaults_ExistingFields(t *testing.T) {
	customMetrics := &metrics.Config{SkipPaths: []string{"/custom"}}
	customLogger := &logger.Config{SkipPaths: []string{"/log"}}
	cfg := &Config{
		Mode: ModeRelease,
		Middleware: MiddlewareConfig{
			MetricsConfig: customMetrics,
			LoggerConfig:  customLogger,
		},
	}
	ApplyDefaultConfig(cfg)
	if cfg.Mode != ModeRelease {
		t.Errorf("ApplyDefaultConfig(cfg).Mode = %v, want %v", cfg.Mode, ModeRelease)
	}
	if cfg.Middleware.MetricsConfig != customMetrics {
		t.Error("ApplyDefaultConfig should not overwrite non-nil MetricsConfig")
	}
	if cfg.Middleware.LoggerConfig != customLogger {
		t.Error("ApplyDefaultConfig should not overwrite non-nil LoggerConfig")
	}
}

func TestPProfConfig_Default(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ServiceConfig.PProfConfig == nil {
		t.Fatal("DefaultConfig().ServiceConfig.PProfConfig is nil")
	}
	if cfg.ServiceConfig.PProfConfig.Enabled != DefaultPProfEnabled {
		t.Errorf("DefaultConfig().ServiceConfig.PProfConfig.Enabled = %v, want %v", cfg.ServiceConfig.PProfConfig.Enabled, DefaultPProfEnabled)
	}
}

func TestApplyDefaults_ServiceConfig(t *testing.T) {
	cfg := &Config{}
	ApplyDefaultConfig(cfg)
	if cfg.ServiceConfig.PProfConfig == nil {
		t.Error("ApplyDefaultConfig did not initialize ServiceConfig.PProfConfig")
	}
	if cfg.ServiceConfig.MetricConfig == nil {
		t.Error("ApplyDefaultConfig did not initialize ServiceConfig.MetricConfig")
	}
	if cfg.ServiceConfig.Healthcheck == nil {
		t.Error("ApplyDefaultConfig did not initialize ServiceConfig.Healthcheck")
	}
}

func TestApplyDefaults_MiddlewareConfig_AllNil(t *testing.T) {
	m := MiddlewareConfig{}
	m.ApplyDefaults()
	if m.MetricsConfig == nil || !reflect.DeepEqual(m.MetricsConfig.SkipPaths, metrics.DefaultSkipPaths) {
		t.Error("ApplyDefaults did not set MetricsConfig correctly")
	}
	if m.LoggerConfig == nil || !reflect.DeepEqual(m.LoggerConfig.SkipPaths, logger.DefaultSkipPaths) {
		t.Error("ApplyDefaults did not set LoggerConfig correctly")
	}
	if m.RecoveryConfig == nil || !m.RecoveryConfig.Enabled {
		t.Error("ApplyDefaults did not set RecoveryConfig correctly")
	}
}

// Additional table-driven and parallel tests

func TestConfig_ApplyDefaults_Table(t *testing.T) {
	tests := []struct {
		name     string
		input    *Config
		expected GinMode
	}{
		{"NilMode", &Config{}, DefaultMode},
		{"ReleaseMode", &Config{Mode: ModeRelease}, ModeRelease},
		{"DebugMode", &Config{Mode: ModeDebug}, ModeDebug},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ApplyDefaultConfig(tt.input)
			if tt.input.Mode != tt.expected {
				t.Errorf("ApplyDefaultConfig.Mode = %v, want %v", tt.input.Mode, tt.expected)
			}
		})
	}
}

func TestMiddlewareConfig_ApplyDefaults_Table(t *testing.T) {
	tests := []struct {
		name     string
		input    MiddlewareConfig
		expected bool
	}{
		{"NilRecovery", MiddlewareConfig{}, true},
		{"RecoveryDisabled", MiddlewareConfig{RecoveryConfig: &RecoveryConfig{Enabled: false}}, false},
		{"RecoveryEnabled", MiddlewareConfig{RecoveryConfig: &RecoveryConfig{Enabled: true}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.input.ApplyDefaults()
			if tt.input.RecoveryConfig.Enabled != tt.expected {
				t.Errorf("ApplyDefaults.RecoveryConfig.Enabled = %v, want %v", tt.input.RecoveryConfig.Enabled, tt.expected)
			}
		})
	}
}

func TestServiceConfig_ApplyDefaults_Table(t *testing.T) {
	tests := []struct {
		name string
		in   ServiceConfig
	}{
		{"AllNil", ServiceConfig{}},
		{"PProfOnly", ServiceConfig{PProfConfig: &PProfConfig{Enabled: true}}},
		{"MetricOnly", ServiceConfig{MetricConfig: &MetricConfig{Enabled: false}}},
		{"HealthOnly", ServiceConfig{Healthcheck: &HealthCheckConfig{Enabled: false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.in.ApplyDefaults()
			if tt.in.PProfConfig == nil {
				t.Error("PProfConfig should not be nil after ApplyDefaults")
			}
			if tt.in.MetricConfig == nil {
				t.Error("MetricConfig should not be nil after ApplyDefaults")
			}
			if tt.in.Healthcheck == nil {
				t.Error("Healthcheck should not be nil after ApplyDefaults")
			}
		})
	}
}

func TestConfig_Validate_Table(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		expectErr bool
	}{
		{
			"MTLS enabled, TLS disabled",
			func() *Config {
				c := DefaultConfig()
				c.TLSConfig = tlsserver.Config{} // TLS disabled by zero value
				c.Middleware.CertificateAuthConfig.Enabled = true
				return c
			}(),
			true,
		},
		{
			"MTLS enabled, invalid ClientAuthType",
			func() *Config {
				c := DefaultConfig()
				// Set TLSConfig fields as needed, e.g. c.TLSConfig.ClientAuthType = "NoClientCert"
				c.Middleware.CertificateAuthConfig.Enabled = true
				return c
			}(),
			true,
		},
		{
			"MTLS disabled, TLS enabled",
			func() *Config {
				c := DefaultConfig()
				// Set TLSConfig fields as needed
				c.Middleware.CertificateAuthConfig.Enabled = false
				return c
			}(),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestInjectServerCNs(t *testing.T) {
	testCases := []struct {
		name                string
		setupConfig         func(t *testing.T) (*Config, func())
		expectedError       bool
		expectedCertCount   int
		expectedCNsContains []string
		checkFunc           func(t *testing.T, config *Config)
	}{
		{
			name: "TLS not enabled - should return early without error",
			setupConfig: func(t *testing.T) (*Config, func()) {
				config := &Config{
					TLSConfig: tlsserver.Config{},
				}
				return config, func() {}
			},
			expectedError:     false,
			expectedCertCount: 0,
		},
		{
			name: "certificate auth not enabled - should return early without error",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"test.example.com"})
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: nil,
					},
				}
				return config, cleanup
			},
			expectedError:     false,
			expectedCertCount: 0,
		},
		{
			name: "certificate auth disabled - should return early without error",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"test.example.com"})
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled: false,
						},
					},
				}
				return config, cleanup
			},
			expectedError:     false,
			expectedCertCount: 0,
		},
		{
			name: "skip server CN injection enabled - should return early without error",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"test.example.com"})
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled:               true,
							SkipServerCNInjection: true,
							Certificates:          []certificate.Certificate{},
						},
					},
				}
				return config, cleanup
			},
			expectedError:     false,
			expectedCertCount: 0,
		},
		{
			name: "valid certificate with CN only - should extract and add CN from DNS SANs",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", nil)
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled:      true,
							Certificates: []certificate.Certificate{},
						},
					},
				}
				return config, cleanup
			},
			expectedError:       false,
			expectedCertCount:   1,
			expectedCNsContains: []string{"test.example.com"},
		},
		{
			name: "valid certificate with CN and DNS SANs - should extract all DNS names",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"alt1.example.com", "alt2.example.com"})
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled:      true,
							Certificates: []certificate.Certificate{},
						},
					},
				}
				return config, cleanup
			},
			expectedError:       false,
			expectedCertCount:   1,
			expectedCNsContains: []string{"test.example.com", "alt1.example.com", "alt2.example.com"},
		},
		{
			name: "existing certificates - should append new certificate",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"test.example.com"})
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled: true,
							Certificates: []certificate.Certificate{
								{
									CNs: []string{"existing.example.com"},
								},
							},
						},
					},
				}
				return config, cleanup
			},
			expectedError:     false,
			expectedCertCount: 2,
			checkFunc: func(t *testing.T, config *Config) {
				// Verify first certificate is unchanged
				assert.Contains(t, config.Middleware.CertificateAuthConfig.Certificates[0].CNs, "existing.example.com")
				// Verify second certificate was added
				assert.Contains(t, config.Middleware.CertificateAuthConfig.Certificates[1].CNs, "test.example.com")
			},
		},
		{
			name: "invalid certificate path - should return error",
			setupConfig: func(t *testing.T) (*Config, func()) {
				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: "/nonexistent/cert.pem",
							Key:  "/nonexistent/key.pem",
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled:      true,
							Certificates: []certificate.Certificate{},
						},
					},
				}
				return config, func() {}
			},
			expectedError: true,
		},
		{
			name: "valid certificate - should have wildcard scope",
			setupConfig: func(t *testing.T) (*Config, func()) {
				certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", nil)
				require.NoError(t, err)

				config := &Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Cert: certPath,
							Key:  keyPath,
						},
					},
					Middleware: MiddlewareConfig{
						CertificateAuthConfig: &certificate.Config{
							Enabled:      true,
							Certificates: []certificate.Certificate{},
						},
					},
				}
				return config, cleanup
			},
			expectedError:     false,
			expectedCertCount: 1,
			checkFunc: func(t *testing.T, config *Config) {
				cert := config.Middleware.CertificateAuthConfig.Certificates[0]
				scope := cert.Scopes[0].(*basic.Scope)
				assert.Equal(t, []string{"*"}, scope.Actions)
				assert.Equal(t, []string{"*"}, scope.Environments)
				assert.Equal(t, []string{"*"}, scope.Teams)
				assert.Equal(t, []string{"*"}, scope.Roles)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, cleanup := tc.setupConfig(t)
			defer cleanup()

			err := config.injectServerCNs()

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tc.expectedCertCount > 0 {
					require.NotNil(t, config.Middleware.CertificateAuthConfig)
					assert.Len(t, config.Middleware.CertificateAuthConfig.Certificates, tc.expectedCertCount)

					if len(tc.expectedCNsContains) > 0 {
						lastCert := config.Middleware.CertificateAuthConfig.Certificates[len(config.Middleware.CertificateAuthConfig.Certificates)-1]
						for _, expectedCN := range tc.expectedCNsContains {
							assert.Contains(t, lastCert.CNs, expectedCN)
						}
					}
				}

				if tc.checkFunc != nil {
					tc.checkFunc(t, config)
				}
			}
		})
	}
}
