package oauth

import (
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/openshift/oauth-proxy/providers/openshift"
)

var (
	ErrProviderNotInitialized = fmt.Errorf("oauth Provider is not initialized")
	ErrProviderNotOpenShift   = fmt.Errorf("oauth Provider is not an OpenShiftProvider")
)

// Config holds the configuration for the OAuth provider implementing OpenShift OAuth Proxy pattern.
//
// This struct defines all necessary fields to configure OAuth authentication and authorization
// with OpenShift clusters. The configuration supports Subject Access Reviews (SAR) for fine-grained
// permission checking and resource access control.
//
// Key Configuration Areas:
//   - Authentication: ClientID, ClientSecret, LoginURL, RedeemURL for OAuth flow
//   - Authorization: ReviewURL, Resources for Subject Access Review (SAR) checks
//   - Security: ClientCAFile, ReviewCAs for certificate-based authentication
//   - Access Control: BypassAuthForPaths for public endpoints
//
// Subject Access Review (SAR) Configuration:
// The Resources field should contain JSON array of SAR rules that define required permissions.
// Multiple rules require ALL permissions to be granted. For host-specific rules, use
//
// Example SAR rule: [{"resource":"pods","verb":"get","namespace":"default"}]
//
// For implementation details and reference, see:
//   - https://github.com/openshift/oauth-proxy
//   - Run 'oc explain subjectaccessreview' for SAR schema details
//
// Minimal example:
//
//	oauth:
//	  enabled: true
//	  # Only users with permission to get pods in the default namespace are authorized
//	  resources: |
//	    {"/": {"namespace":"default","resource":"pods","verb":"get"}}
type Config struct {
	Enabled            bool     `json:"enabled" yaml:"enabled"`
	ClientCAFile       string   `json:"clientCAFile" yaml:"clientCAFile"`
	ClientID           string   `json:"clientID" yaml:"clientID"`
	ClientSecret       string   `json:"clientSecret" yaml:"clientSecret"`
	ReviewCAs          []string `json:"reviewCAs" yaml:"reviewCAs"`
	ServiceAccount     string   `json:"serviceAccount" yaml:"serviceAccount"`
	DelegateURLs       string   `json:"delegateURLs" yaml:"delegateURLs"`
	ReviewByHostURL    string   `json:"reviewByHostURL" yaml:"reviewByHostURL"`
	Resources          string   `json:"resources" yaml:"resources"`
	LoginURL           string   `json:"loginURL" yaml:"loginURL"`
	RedeemURL          string   `json:"redeemURL" yaml:"redeemURL"`
	ReviewURL          string   `json:"reviewURL" yaml:"reviewURL"`
	ValidateURL        string   `json:"validateURL" yaml:"validateURL"`
	Scope              string   `json:"scope" yaml:"scope"`
	BypassAuthForPaths []string `json:"bypassAuthForPaths" yaml:"bypassAuthForPaths"`
}

// Validate checks the configuration for validity
func (c *Config) Validate() error {
	return nil
}

// ApplyDefaults sets default values for the configuration
func (c *Config) ApplyDefaults() {
	if len(c.BypassAuthForPaths) == 0 {
		c.BypassAuthForPaths = DefaultSkipPaths
	}
}

type Option = config.Option[*Provider]

// WithProvider sets the OAuth provider
func WithProvider(p *Provider) config.Opt[*Provider] {
	return func(s *Provider) error {
		if p == nil {
			return fmt.Errorf("oauth Provider cannot be nil")
		}
		s.Provider = p
		return nil
	}
}

// WithResponse sets the response handler for authentication failures
func WithResponse(handler Response) config.Opt[*Provider] {
	return func(s *Provider) error {
		s.response = handler
		return nil
	}
}

// WithAuthenticationOptions sets the authentication options for the OpenShift provider
func WithAuthenticationOptions(opts openshift.DelegatingAuthenticationOptions) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		if openshiftProvider, ok := s.Provider.(*openshift.OpenShiftProvider); ok {
			openshiftProvider.AuthenticationOptions = opts
		}
		return ErrProviderNotOpenShift
	}
}

// WithAuthorizationOptions sets the authorization options for the OpenShift provider
func WithAuthorizationOptions(opts openshift.DelegatingAuthorizationOptions) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		if openshiftProvider, ok := s.Provider.(*openshift.OpenShiftProvider); ok {
			openshiftProvider.AuthorizationOptions = opts
			return nil
		}
		return ErrProviderNotOpenShift
	}
}

// WithKubeClientOptions sets the Kubernetes client options for the OpenShift provider
func WithKubeClientOptions(opts openshift.KubeClientOptions) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		if openshiftProvider, ok := s.Provider.(*openshift.OpenShiftProvider); ok {
			openshiftProvider.KubeClientOptions = opts
			return nil
		}
		return ErrProviderNotOpenShift
	}
}

// WithKubeConfig sets the kubeconfig file for the OpenShift provider
func WithKubeConfig(kubeconfig string) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		return s.SetKubeConfig(kubeconfig)
	}
}

// WithClientCertAuthenticationOptions sets the client certificate authentication options for the OpenShift provider
func WithClientCertAuthenticationOptions(clientCA string) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		if openshiftProvider, ok := s.Provider.(*openshift.OpenShiftProvider); ok {
			openshiftProvider.AuthenticationOptions.ClientCert = openshift.ClientCertAuthenticationOptions{ClientCA: clientCA}
			return nil
		}
		return ErrProviderNotOpenShift
	}
}

// WithCacheTTL sets the cache TTL for both authentication and authorization in the OpenShift provider
func WithCacheTTL(t time.Duration) config.Opt[*Provider] {
	return func(s *Provider) error {
		if s.Provider == nil {
			return ErrProviderNotInitialized
		}
		if openshiftProvider, ok := s.Provider.(*openshift.OpenShiftProvider); ok {
			openshiftProvider.AuthenticationOptions.CacheTTL = t
			openshiftProvider.AuthorizationOptions.AllowCacheTTL = t
			return nil
		}
		return ErrProviderNotOpenShift
	}
}
