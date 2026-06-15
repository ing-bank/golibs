package manager

import (
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	httpcheck "github.com/ing-bank/golibs/pkg/healthcheck/checks/http"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks/telnet"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/oauth"
	"github.com/ing-bank/golibs/pkg/tlsclient"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInjectConfig(t *testing.T) {
	baseWebserverConfig := &Config{
		Config: ginserver.Config{
			HTTPServer: httpserver.Config{
				Host: "localhost",
				Port: 8080,
			},
		},
	}

	tlsWebserverConfig := &Config{
		Config: ginserver.Config{
			HTTPServer: httpserver.Config{
				Host: "localhost",
				Port: 8443,
			},
			TLSConfig: tlsserver.Config{
				Config: tlsutils.Config{
					Cert: "cert.pem",
					Key:  "key.pem",
				},
			},
		},
	}

	oauthWebserverConfig := &Config{
		Config: ginserver.Config{
			HTTPServer: httpserver.Config{
				Host: "localhost",
				Port: 8080,
			},
			Middleware: ginserver.MiddlewareConfig{
				OauthConfig: &oauth.Config{
					Enabled: true,
				},
			},
		},
	}

	testCases := []struct {
		name              string
		webserverConfig   *Config
		sidecarConfig     *Config
		expectedSidecar   *Config
		expectError       bool
		checkSidecarFunc  func(t *testing.T, sidecar *Config)
		checkSidecarFuncs []func(t *testing.T, sidecar *Config)
	}{
		{
			name:            "no healthcheck configured, should add default telnet check",
			webserverConfig: baseWebserverConfig,
			sidecarConfig:   &Config{},
			checkSidecarFuncs: []func(t *testing.T, sidecar *Config){
				func(t *testing.T, sidecar *Config) {
					assert.NotNil(t, sidecar.SidecarServiceConfig.Healthcheck)
				},
				func(t *testing.T, sidecar *Config) {
					assert.Len(t, sidecar.SidecarServiceConfig.Healthcheck.Jobs, 1)
				},
				func(t *testing.T, sidecar *Config) {
					assert.NotNil(t, sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet)
				},
				func(t *testing.T, sidecar *Config) {
					assert.Equal(t, "localhost:8080", sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.Address)
				},
			},
		},
		{
			name:            "http healthcheck without address, should inject webserver address",
			webserverConfig: baseWebserverConfig,
			sidecarConfig: &Config{
				SidecarServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Config: healthcheck.Config{
							Jobs: []healthcheck.JobConfig{
								{
									ProbeHandler: healthcheck.ProbeHandler{
										HTTPGet: &httpcheck.Config{},
									},
								},
							},
						},
					},
				},
			},
			checkSidecarFunc: func(t *testing.T, sidecar *Config) {
				assert.Equal(t, "localhost", sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Host)
				assert.Equal(t, uint16(8080), sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Port)
			},
		},
		{
			name:            "http healthcheck with tls, should inject tls config",
			webserverConfig: tlsWebserverConfig,
			sidecarConfig: &Config{
				SidecarServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Config: healthcheck.Config{
							Jobs: []healthcheck.JobConfig{
								{
									ProbeHandler: healthcheck.ProbeHandler{
										HTTPGet: &httpcheck.Config{
											Scheme: "https",
										},
									},
								},
							},
						},
					},
				},
			},
			checkSidecarFunc: func(t *testing.T, sidecar *Config) {
				assert.True(t, sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.UseTLS())
				assert.True(t, sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.InsecureSkipVerify)
			},
		},
		{
			name:            "telnet healthcheck with tls, should inject tls config",
			webserverConfig: tlsWebserverConfig,
			sidecarConfig: &Config{
				SidecarServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Config: healthcheck.Config{
							Jobs: []healthcheck.JobConfig{
								{
									ProbeHandler: healthcheck.ProbeHandler{
										Telnet: &telnet.Config{
											Address: "localhost:8443",
										},
									},
								},
							},
						},
					},
				},
			},
			checkSidecarFunc: func(t *testing.T, sidecar *Config) {
				assert.True(t, sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.TLSConfig.UseTLS())
			},
		},
		{
			name:            "oauth enabled, should add authorization header to http probe",
			webserverConfig: oauthWebserverConfig,
			sidecarConfig: &Config{
				SidecarServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Config: healthcheck.Config{
							Jobs: []healthcheck.JobConfig{
								{
									ProbeHandler: healthcheck.ProbeHandler{
										HTTPGet: &httpcheck.Config{
											Host: "localhost",
											Port: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			checkSidecarFunc: func(t *testing.T, sidecar *Config) {
				headers := sidecar.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.HTTPHeaders
				assert.Len(t, headers, 1)
				assert.Equal(t, httpcheck.UseContextToken, headers[0]["Authorization"])
			},
		},
		{
			name: "service config should be copied to sidecar",
			webserverConfig: &Config{
				Config: ginserver.Config{
					ServiceConfig: ginserver.ServiceConfig{
						MetricConfig: &ginserver.MetricConfig{Enabled: true},
					},
				},
			},
			sidecarConfig: &Config{
				SidecarServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{Enabled: true},
				},
			},
			checkSidecarFunc: func(t *testing.T, sidecar *Config) {
				assert.Equal(t, sidecar.SidecarServiceConfig, sidecar.ServiceConfig)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := injectConfig(tc.sidecarConfig, tc.webserverConfig)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.checkSidecarFunc != nil {
					tc.checkSidecarFunc(t, tc.sidecarConfig)
				}
				if tc.checkSidecarFuncs != nil {
					for _, check := range tc.checkSidecarFuncs {
						check(t, tc.sidecarConfig)
					}
				}
			}
		})
	}
}

func TestNewTelnetCheck(t *testing.T) {
	name := "test-telnet"
	desc := "Test telnet health check"
	address := "localhost:1234"
	tlsConfig := tlsclient.Config{InsecureSkipVerify: true}
	jobConfig := newTelnetCheck(name, desc, address, tlsConfig)

	assert.Equal(t, name, jobConfig.Name)
	assert.Equal(t, desc, jobConfig.Description)
	assert.Equal(t, 1*time.Second, jobConfig.Timeout)
	assert.NotNil(t, jobConfig.ProbeHandler.Telnet)
	assert.Equal(t, address, jobConfig.ProbeHandler.Telnet.Address)
	assert.Equal(t, tlsConfig, jobConfig.ProbeHandler.Telnet.TLSConfig)
	assert.Contains(t, jobConfig.Endpoints, healthcheck.HealthEndpoint)
	assert.Contains(t, jobConfig.Endpoints, healthcheck.ReadyEndpoint)
}

func TestReuseTLSConfig(t *testing.T) {
	serverConfig := tlsserver.Config{
		Config: tlsutils.Config{
			Cert:    "server-cert.pem",
			Key:     "server-key.pem",
			RootCAs: []string{"ca.pem"},
		},
	}

	clientConfig := reuseTLSConfig(serverConfig)

	assert.Equal(t, serverConfig.Cert, clientConfig.Cert)
	assert.Equal(t, serverConfig.Key, clientConfig.Key)
	assert.True(t, clientConfig.InsecureSkipVerify)
	assert.Equal(t, serverConfig.RootCAs, clientConfig.RootCAs)
}

func TestNewHTTPCheck(t *testing.T) {
	name := "test-http"
	desc := "Test HTTP health check"
	host := "localhost"
	port := uint16(8080)
	tlsConfig := tlsclient.Config{InsecureSkipVerify: true}
	jobConfig := newHTTPCheck(name, desc, host, port, tlsConfig)

	assert.Equal(t, name, jobConfig.Name)
	assert.Equal(t, desc, jobConfig.Description)
	assert.Equal(t, 3*time.Second, jobConfig.Timeout)
	assert.NotNil(t, jobConfig.ProbeHandler.HTTPGet)
	assert.Equal(t, host, jobConfig.ProbeHandler.HTTPGet.Host)
	assert.Equal(t, port, jobConfig.ProbeHandler.HTTPGet.Port)
	assert.Equal(t, healthcheck.HealthEndpoint.String(), jobConfig.ProbeHandler.HTTPGet.Path)
	assert.Equal(t, tlsConfig, jobConfig.ProbeHandler.HTTPGet.TLSConfig)
	assert.Contains(t, jobConfig.Endpoints, healthcheck.HealthEndpoint)
	assert.Contains(t, jobConfig.Endpoints, healthcheck.ReadyEndpoint)
}

func TestInjectConfig_ProbeModification(t *testing.T) {
	// Test to verify that loop correctly modifies probes in place (not copies)
	t.Run("HTTPGet probe TLS config is modified in place", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
			},
		}

		dest := &Config{
			SidecarServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{
					Config: healthcheck.Config{
						Jobs: []healthcheck.JobConfig{
							{
								ProbeHandler: healthcheck.ProbeHandler{
									HTTPGet: &httpcheck.Config{
										Scheme: "https",
									},
								},
							},
						},
					},
				},
			},
		}

		// Before injection, TLS should not be configured
		assert.False(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.UseTLS())

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// After injection, TLS should be configured (proving modification worked)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.UseTLS())
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.InsecureSkipVerify)
		assert.Equal(t, "cert.pem", dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.Cert)
	})

	t.Run("Telnet probe TLS config is modified in place", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
			},
		}

		dest := &Config{
			SidecarServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{
					Config: healthcheck.Config{
						Jobs: []healthcheck.JobConfig{
							{
								ProbeHandler: healthcheck.ProbeHandler{
									Telnet: &telnet.Config{
										Address: "localhost:8443",
									},
								},
							},
						},
					},
				},
			},
		}

		// Before injection, TLS should not be configured
		assert.False(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.TLSConfig.UseTLS())

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// After injection, TLS should be configured (proving modification worked)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.TLSConfig.UseTLS())
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.TLSConfig.InsecureSkipVerify)
		assert.Equal(t, "cert.pem", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.TLSConfig.Cert)
	})

	t.Run("HTTPGet probe address is modified in place", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "example.com",
					Port: 9090,
				},
			},
		}

		dest := &Config{
			SidecarServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{
					Config: healthcheck.Config{
						Jobs: []healthcheck.JobConfig{
							{
								ProbeHandler: healthcheck.ProbeHandler{
									HTTPGet: &httpcheck.Config{},
								},
							},
						},
					},
				},
			},
		}

		// Before injection, address should be empty
		assert.Equal(t, "", dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Host)
		assert.Equal(t, uint16(0), dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Port)

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// After injection, address should be set (proving modification worked)
		assert.Equal(t, "example.com", dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Host)
		assert.Equal(t, uint16(9090), dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Port)
	})

	t.Run("Multiple probes are all modified", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
			},
		}

		dest := &Config{
			SidecarServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{
					Config: healthcheck.Config{
						Jobs: []healthcheck.JobConfig{
							{
								ProbeHandler: healthcheck.ProbeHandler{
									HTTPGet: &httpcheck.Config{Scheme: "https"},
								},
							},
							{
								ProbeHandler: healthcheck.ProbeHandler{
									Telnet: &telnet.Config{Address: "localhost:8443"},
								},
							},
							{
								ProbeHandler: healthcheck.ProbeHandler{
									HTTPGet: &httpcheck.Config{Scheme: "https"},
								},
							},
						},
					},
				},
			},
		}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// All probes should have TLS configured
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.UseTLS())
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[1].Telnet.TLSConfig.UseTLS())
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[2].HTTPGet.TLSConfig.UseTLS())
	})
}

func TestInjectConfig_DefaultHealthCheck(t *testing.T) {
	t.Run("creates HTTP check when source healthcheck is enabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: true,
					},
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// Should create healthcheck with HTTP probe
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Enabled)
		assert.Len(t, dest.SidecarServiceConfig.Healthcheck.Jobs, 1)
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet)
		assert.Equal(t, "localhost", dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Host)
		assert.Equal(t, uint16(8080), dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Port)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.TLSConfig.UseTLS())
	})

	t.Run("creates Telnet check when source healthcheck is disabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: false,
					},
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// Should create healthcheck with Telnet probe
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Enabled)
		assert.Len(t, dest.SidecarServiceConfig.Healthcheck.Jobs, 1)
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet)
		assert.Equal(t, "localhost:8080", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.Address)
	})

	t.Run("creates Telnet check when source healthcheck is nil", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "example.com",
					Port: 9090,
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// Should create healthcheck with Telnet probe
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.Enabled)
		assert.Len(t, dest.SidecarServiceConfig.Healthcheck.Jobs, 1)
		assert.NotNil(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet)
		assert.Equal(t, "example.com:9090", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Telnet.Address)
	})

	t.Run("does not create default check if dest already has healthcheck", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
		}

		dest := &Config{
			SidecarServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{
					Enabled: true,
					Config: healthcheck.Config{
						Jobs: []healthcheck.JobConfig{
							{
								ProbeHandler: healthcheck.ProbeHandler{
									HTTPGet: &httpcheck.Config{
										Host: "custom.com",
										Port: 1234,
									},
								},
							},
						},
					},
				},
			},
		}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// Should keep existing healthcheck, not replace it
		assert.Len(t, dest.SidecarServiceConfig.Healthcheck.Jobs, 1)
		assert.Equal(t, "custom.com", dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Host)
		assert.Equal(t, uint16(1234), dest.SidecarServiceConfig.Healthcheck.Jobs[0].HTTPGet.Port)
	})

	t.Run("default check includes proper interval and system info", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		assert.Equal(t, metav1.Duration{Duration: 5 * time.Second}, dest.SidecarServiceConfig.Healthcheck.Interval)
		assert.True(t, dest.SidecarServiceConfig.Healthcheck.SystemInfo)
	})

	t.Run("default HTTP check has correct name and description", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: true,
					},
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		assert.Equal(t, "default-main-webserver-check", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Name)
		assert.Equal(t, "Default HTTP health check for main webserver", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Description)
	})

	t.Run("default Telnet check has correct name and description", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: false,
					},
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		assert.Equal(t, "default-main-webserver-check", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Name)
		assert.Equal(t, "Default telnet health check for main webserver", dest.SidecarServiceConfig.Healthcheck.Jobs[0].Description)
	})

	t.Run("proxy health checks have correct names when proxy is enabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "localhost",
				Port:    9092,
				Override: ginserver.Config{
					ServiceConfig: ginserver.ServiceConfig{
						Healthcheck: &ginserver.HealthCheckConfig{
							Enabled: true,
						},
					},
				},
			},
		}

		dest := &Config{}

		err := injectConfig(dest, source)
		assert.NoError(t, err)

		// Should have 2 jobs: webserver + proxycar
		assert.Len(t, dest.SidecarServiceConfig.Healthcheck.Jobs, 2)

		// First job should be webserver check
		assert.Contains(t, dest.SidecarServiceConfig.Healthcheck.Jobs[0].Name, "webserver")

		// Second job should be proxycar check
		assert.Equal(t, "default-proxy-check", dest.SidecarServiceConfig.Healthcheck.Jobs[1].Name)
		assert.Equal(t, "Default HTTP health check for ProxyCar", dest.SidecarServiceConfig.Healthcheck.Jobs[1].Description)
	})
}

func TestCreateDefaultHealthChecks(t *testing.T) {
	t.Run("creates HTTP check for webserver when healthcheck is enabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: true,
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 1)
		assert.Equal(t, "default-main-webserver-check", jobs[0].Name)
		assert.Equal(t, "Default HTTP health check for main webserver", jobs[0].Description)
		assert.NotNil(t, jobs[0].HTTPGet)
		assert.Equal(t, "localhost", jobs[0].HTTPGet.Host)
		assert.Equal(t, uint16(8080), jobs[0].HTTPGet.Port)
	})

	t.Run("creates Telnet check for webserver when healthcheck is disabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: false,
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 1)
		assert.Equal(t, "default-main-webserver-check", jobs[0].Name)
		assert.Equal(t, "Default telnet health check for main webserver", jobs[0].Description)
		assert.NotNil(t, jobs[0].Telnet)
		assert.Equal(t, "localhost:8080", jobs[0].Telnet.Address)
	})

	t.Run("creates Telnet check for webserver when healthcheck is nil", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "example.com",
					Port: 9090,
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 1)
		assert.NotNil(t, jobs[0].Telnet)
		assert.Equal(t, "example.com:9090", jobs[0].Telnet.Address)
	})

	t.Run("creates both webserver and proxycar checks when proxy is enabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "localhost",
				Port:    9092,
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// First job is webserver
		assert.Equal(t, "default-main-webserver-check", jobs[0].Name)
		assert.NotNil(t, jobs[0].Telnet)
		assert.Equal(t, "localhost:8080", jobs[0].Telnet.Address)

		// Second job is proxycar
		assert.Equal(t, "default-proxy-check", jobs[1].Name)
		assert.NotNil(t, jobs[1].Telnet)
		assert.Equal(t, "localhost:9092", jobs[1].Telnet.Address)
	})

	t.Run("creates HTTP check for proxycar when proxy healthcheck is enabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "proxy.example.com",
				Port:    9092,
				Override: ginserver.Config{
					ServiceConfig: ginserver.ServiceConfig{
						Healthcheck: &ginserver.HealthCheckConfig{
							Enabled: true,
						},
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// Proxycar check should be HTTP
		assert.Equal(t, "default-proxy-check", jobs[1].Name)
		assert.Equal(t, "Default HTTP health check for ProxyCar", jobs[1].Description)
		assert.NotNil(t, jobs[1].HTTPGet)
		assert.Equal(t, "proxy.example.com", jobs[1].HTTPGet.Host)
		assert.Equal(t, uint16(9092), jobs[1].HTTPGet.Port)
	})

	t.Run("creates Telnet check for proxycar when proxy healthcheck is disabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "proxy.example.com",
				Port:    9092,
				Override: ginserver.Config{
					ServiceConfig: ginserver.ServiceConfig{
						Healthcheck: &ginserver.HealthCheckConfig{
							Enabled: false,
						},
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// Proxycar check should be Telnet
		assert.Equal(t, "default-proxy-check", jobs[1].Name)
		assert.Equal(t, "Default telnet health check for ProxyCar", jobs[1].Description)
		assert.NotNil(t, jobs[1].Telnet)
		assert.Equal(t, "proxy.example.com:9092", jobs[1].Telnet.Address)
	})

	t.Run("only creates webserver check when proxy is disabled", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8080,
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: false,
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 1)
		assert.Equal(t, "default-main-webserver-check", jobs[0].Name)
	})

	t.Run("TLS config is passed correctly to health checks", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: true,
					},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "localhost",
				Port:    9092,
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// Both jobs should be HTTP checks since proxy inherits healthcheck enabled from main server
		// Both should have TLS configured
		assert.NotNil(t, jobs[0].HTTPGet)
		assert.True(t, jobs[0].HTTPGet.TLSConfig.UseTLS())
		assert.Equal(t, "cert.pem", jobs[0].HTTPGet.TLSConfig.Cert)
		assert.True(t, jobs[0].HTTPGet.TLSConfig.InsecureSkipVerify)

		assert.NotNil(t, jobs[1].HTTPGet)
		assert.True(t, jobs[1].HTTPGet.TLSConfig.UseTLS())
		assert.Equal(t, "cert.pem", jobs[1].HTTPGet.TLSConfig.Cert)
		assert.True(t, jobs[1].HTTPGet.TLSConfig.InsecureSkipVerify)
	})

	t.Run("proxycar uses no TLS when Override.TLSConfig.Disabled is true", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
				ServiceConfig: ginserver.ServiceConfig{
					Healthcheck: &ginserver.HealthCheckConfig{
						Enabled: true,
					},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "localhost",
				Port:    9092,
				Override: ginserver.Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Disabled: true,
						},
					},
					ServiceConfig: ginserver.ServiceConfig{
						Healthcheck: &ginserver.HealthCheckConfig{
							Enabled: true,
						},
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// Webserver should have TLS
		assert.NotNil(t, jobs[0].HTTPGet)
		assert.True(t, jobs[0].HTTPGet.TLSConfig.UseTLS())
		assert.Equal(t, "cert.pem", jobs[0].HTTPGet.TLSConfig.Cert)

		// Proxycar should NOT have TLS (disabled via override)
		assert.NotNil(t, jobs[1].HTTPGet)
		assert.False(t, jobs[1].HTTPGet.TLSConfig.UseTLS())
		assert.Empty(t, jobs[1].HTTPGet.TLSConfig.Cert)
		assert.Empty(t, jobs[1].HTTPGet.TLSConfig.Key)
	})

	t.Run("proxycar telnet check has no TLS when Override.TLSConfig.Disabled is true", func(t *testing.T) {
		source := &Config{
			Config: ginserver.Config{
				HTTPServer: httpserver.Config{
					Host: "localhost",
					Port: 8443,
				},
				TLSConfig: tlsserver.Config{
					Config: tlsutils.Config{
						Cert: "cert.pem",
						Key:  "key.pem",
					},
				},
			},
			ProxyConfig: ProxyConfig{
				Enabled: true,
				Host:    "localhost",
				Port:    9092,
				Override: ginserver.Config{
					TLSConfig: tlsserver.Config{
						Config: tlsutils.Config{
							Disabled: true,
						},
					},
				},
			},
		}

		jobs := createDefaultHealthChecks(source)

		assert.Len(t, jobs, 2)

		// Webserver should have TLS
		assert.NotNil(t, jobs[0].Telnet)
		assert.True(t, jobs[0].Telnet.TLSConfig.UseTLS())

		// Proxycar should NOT have TLS (disabled via override)
		assert.NotNil(t, jobs[1].Telnet)
		assert.False(t, jobs[1].Telnet.TLSConfig.UseTLS())
		assert.Empty(t, jobs[1].Telnet.TLSConfig.Cert)
		assert.Empty(t, jobs[1].Telnet.TLSConfig.Key)
	})
}
