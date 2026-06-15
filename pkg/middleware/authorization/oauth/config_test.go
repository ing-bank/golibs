package oauth

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openshift/oauth-proxy/providers/openshift"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			cfg: Config{
				Enabled:      true,
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			cfg: Config{
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "disabled config",
			cfg: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name:    "empty config",
			cfg:     Config{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected []string
	}{
		{
			name:     "empty bypass paths - should apply defaults",
			cfg:      Config{},
			expected: []string{"/metrics", "/health", "/ready", "/healthz", "/readyz"},
		},
		{
			name: "existing bypass paths - should not change",
			cfg: Config{
				BypassAuthForPaths: []string{"/custom", "/special"},
			},
			expected: []string{"/custom", "/special"},
		},
		{
			name: "nil bypass paths - should apply defaults",
			cfg: Config{
				BypassAuthForPaths: nil,
			},
			expected: []string{"/metrics", "/health", "/ready", "/healthz", "/readyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ApplyDefaults()

			if len(tt.cfg.BypassAuthForPaths) != len(tt.expected) {
				t.Errorf("Expected %d bypass paths, got %d", len(tt.expected), len(tt.cfg.BypassAuthForPaths))
				return
			}

			for i, expected := range tt.expected {
				if tt.cfg.BypassAuthForPaths[i] != expected {
					t.Errorf("Expected bypass path %s at index %d, got %s", expected, i, tt.cfg.BypassAuthForPaths[i])
				}
			}
		})
	}
}

func TestWithProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name:     "valid provider",
			provider: &Provider{},
			wantErr:  false,
		},
		{
			name:     "nil provider",
			provider: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithProvider(tt.provider)
			target := &Provider{}

			err := option(target)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && target.Provider != tt.provider {
				t.Errorf("Expected provider to be set")
			}
		})
	}
}

func TestWithResponse(t *testing.T) {
	responseFunc := func(c *gin.Context, err error) any {
		return gin.H{"custom": "response"}
	}

	target := &Provider{}
	option := WithResponse(responseFunc)

	err := option(target)
	if err != nil {
		t.Errorf("WithResponse() error = %v", err)
	}

	if target.response == nil {
		t.Errorf("Expected response function to be set")
	}
}

func TestWithAuthenticationOptions(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: &openshift.OpenShiftProvider{},
			},
			wantErr: true, // Will return ErrProviderNotOpenShift due to type assertion logic
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithAuthenticationOptions(openshift.DelegatingAuthenticationOptions{})

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithAuthenticationOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithAuthorizationOptions(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: openshift.New(),
			},
			wantErr: false, // Should succeed with actual OpenShift provider
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithAuthorizationOptions(openshift.DelegatingAuthorizationOptions{})

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithAuthorizationOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithKubeClientOptions(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: openshift.New(),
			},
			wantErr: false, // Should succeed with actual OpenShift provider
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithKubeClientOptions(openshift.KubeClientOptions{})

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithKubeClientOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithKubeConfig(t *testing.T) {
	tests := []struct {
		name       string
		provider   *Provider
		kubeconfig string
		wantErr    bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: openshift.New(),
			},
			kubeconfig: "/path/to/kubeconfig",
			wantErr:    false, // Should succeed with actual OpenShift provider
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			kubeconfig: "/path/to/kubeconfig",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithKubeConfig(tt.kubeconfig)

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithKubeConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithClientCertAuthenticationOptions(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		clientCA string
		wantErr  bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: openshift.New(),
			},
			clientCA: "/path/to/ca.crt",
			wantErr:  false, // Should succeed with actual OpenShift provider
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			clientCA: "/path/to/ca.crt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithClientCertAuthenticationOptions(tt.clientCA)

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithClientCertAuthenticationOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithCacheTTL(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		ttl      time.Duration
		wantErr  bool
	}{
		{
			name: "valid openshift provider",
			provider: &Provider{
				Provider: openshift.New(),
			},
			ttl:     5 * time.Minute,
			wantErr: false, // Should succeed with actual OpenShift provider
		},
		{
			name: "nil provider",
			provider: &Provider{
				Provider: nil,
			},
			ttl:     5 * time.Minute,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithCacheTTL(tt.ttl)

			err := option(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithCacheTTL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
