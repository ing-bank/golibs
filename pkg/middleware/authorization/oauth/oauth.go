// Package oauth is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
package oauth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/trace"
	"github.com/openshift/oauth-proxy/providers"
	"github.com/openshift/oauth-proxy/providers/openshift"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrAuthFailed = fmt.Errorf("authentication failed")
)

var defaultSkipPaths = []string{"/metrics", "/health", "/ready", "/healthz", "/readyz"}

// DefaultSkipPaths defines the default paths to skip authentication
var DefaultSkipPaths = defaultSkipPaths

// Response defines a function type for handling authentication failure responses
type Response func(c *gin.Context, err error) any

// Provider wraps the OpenShift OAuth provider for use in Gin middleware
type Provider struct {
	providers.Provider
	response Response
}

// SetKubeConfig sets the kubeconfig file path for the OpenShift provider
func (p *Provider) SetKubeConfig(kubeconfig string) error {
	if openshiftProvider, ok := p.Provider.(*openshift.OpenShiftProvider); ok {
		openshiftProvider.KubeClientOptions.RemoteKubeConfigFile = kubeconfig
		openshiftProvider.AuthenticationOptions.RemoteKubeConfigFile = kubeconfig
		openshiftProvider.AuthorizationOptions.RemoteKubeConfigFile = kubeconfig
		return nil
	}
	return ErrProviderNotOpenShift
}

// NewForConfig is a helper function to create a new Provider from Config
func NewForConfig(cfg Config) (*Provider, error) {
	return New(cfg)
}

// New creates a new Provider using the OpenShift OAuth proxy library
func New(cfg Config, opts ...Option) (*Provider, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid OpenShift OAuth configuration: %w", err)
	}

	// Create OpenShift provider instance
	openshiftProvider := openshift.New()

	// Set CA file if provided
	if cfg.ClientCAFile != "" {
		openshiftProvider.SetClientCAFile(cfg.ClientCAFile)
	}

	// Set review CAs
	if len(cfg.ReviewCAs) > 0 {
		openshiftProvider.SetReviewCAs(cfg.ReviewCAs)
	}

	// Load defaults with service account and delegation settings
	providerData, err := openshiftProvider.LoadDefaults(
		cfg.ServiceAccount,
		cfg.DelegateURLs,
		cfg.ReviewByHostURL,
		cfg.Resources,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading OpenShift provider defaults: %w", err)
	}

	// Override with explicit configuration
	if cfg.ClientID != "" {
		providerData.ClientID = cfg.ClientID
	}
	if cfg.ClientSecret != "" {
		providerData.ClientSecret = cfg.ClientSecret
	}
	if cfg.Scope != "" {
		providerData.Scope = cfg.Scope
	}

	// Parse URLs if provided
	var reviewURL *url.URL
	if cfg.ReviewURL != "" {
		reviewURL, err = url.Parse(cfg.ReviewURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing review URL: %w", err)
		}
	}

	if cfg.LoginURL != "" {
		loginURL, err := url.Parse(cfg.LoginURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing login URL: %w", err)
		}
		providerData.ConfigLoginURL = loginURL
	}

	if cfg.RedeemURL != "" {
		redeemURL, err := url.Parse(cfg.RedeemURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing redeem URL: %w", err)
		}
		providerData.ConfigRedeemURL = redeemURL
	}

	if cfg.ValidateURL != "" {
		validateURL, err := url.Parse(cfg.ValidateURL)
		if err != nil {
			return nil, fmt.Errorf("error parsing validate URL: %w", err)
		}
		providerData.ValidateURL = validateURL
	}

	p := &Provider{
		Provider: openshiftProvider,
	}

	_, kubernetesServiceHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, kubernetesServicePort := os.LookupEnv("KUBERNETES_SERVICE_PORT")
	if !kubernetesServiceHost && !kubernetesServicePort {
		path := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		if err := p.SetKubeConfig(path); err != nil {
			return nil, fmt.Errorf("failed to set kubeconfig: %w", err)
		}
	}

	if err := config.ApplyOpts(p, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply TLS option: %w", err)
	}

	if p.response == nil {
		p.response = AuthFailed
	}

	if err := openshiftProvider.Complete(providerData, reviewURL); err != nil {
		return nil, fmt.Errorf("error completing OpenShift provider setup: %w", err)
	}

	return p, nil
}

// Middleware returns a gin.HandlerFunc that authenticates requests using the OpenShift OAuth provider
func Middleware(cfg Config, opts ...Option) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Create the OAuth provider
	oauthProvider, err := New(cfg, opts...)
	if err != nil {
		log.Fatalf("Failed to create OpenShift OAuth provider: %v", err)
	}

	log.Infof("Using OpenShift OAuth Authorization middleware")
	return func(c *gin.Context) {
		// Bypass authentication for specified paths
		if slices.Contains(cfg.BypassAuthForPaths, c.Request.URL.Path) {
			c.Next()
			return
		}

		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		defer span.End()

		// Attach the context with tracing to the request
		c.Request = c.Request.WithContext(ctx)

		// Use the actual OpenShift provider's ValidateRequest method (e.g., Bearer token)
		session, err := oauthProvider.ValidateRequest(c.Request)
		if err != nil {
			log.WithContext(ctx).WithError(err).Warn("[middleware][oauth] Authentication failed")
			oauthProvider.Response(oauthProvider.response, err)(c)
			return
		}

		if session == nil {
			log.WithContext(ctx).WithError(err).Warn("[middleware][oauth] No valid session found")
			oauthProvider.Response(oauthProvider.response, err)(c)
			return
		}

		// Store session in context for downstream use
		setSessionInContext(c, session, ctx)

		log.WithContext(ctx).Debugf("[middleware][oauth] Authentication successful: user=%s email=%s", session.User, session.Email)
		c.Next()
	}
}

// setSessionInContext stores the session details in the Gin context for downstream use.
func setSessionInContext(c *gin.Context, session *providers.SessionState, ctx context.Context) {
	c.Set("oauth_session", session)
	c.Set("oauth_user", session.User)
	c.Set("oauth_email", session.Email)
	c.Set("oauth_access_token", session.AccessToken)
	c.Request = c.Request.WithContext(ctx)
}

// Response returns a gin.HandlerFunc that handles authentication failure responses
func (p *Provider) Response(fn Response, err error) gin.HandlerFunc {
	if err == nil {
		err = ErrAuthFailed
	}
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, fn(c, err))
	}
}

// AuthFailed is a helper function to return a standard authentication failed response
func AuthFailed(c *gin.Context, err error) any {
	if err == nil {
		err = ErrAuthFailed
	}
	return NoContent(c, err)
}

// NoContent is a helper function to return an error message in JSON format
func NoContent(c *gin.Context, err error) any {
	return gin.H{
		"error": err.Error(),
	}
}
