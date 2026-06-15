// Package certificate is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
package certificate

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

var DefaultSkipPaths = []string{"/metrics", "/health", "/ready", "/healthz", "/readyz", "/swagger"}

type Config struct {
	Enabled               bool          `json:"enabled"`
	SkipServerCNInjection bool          `json:"skipServerCNInjection"`
	Certificates          []Certificate `json:"certificates"`
	DisableLogging        bool          `json:"disableLogging"`
	SkipPaths             []string      `json:"skipPaths,omitempty" yaml:"skipPaths,omitempty"`
	ScopeType             string        `json:"scopeType"`
	ScopeParser           ScopeParser   `json:"-"`
}

type ScopeParser interface {
	ParseCertificateScope(json.RawMessage) (scope.Scope, error)
}

type Certificate struct {
	CNs []string `json:"cns"`

	Scopes    []scope.Scope     `json:"-"`
	RawScopes []json.RawMessage `json:"scopes"`
}

func (c *Config) ApplyDefaults() {
	if c.SkipPaths == nil {
		c.SkipPaths = DefaultSkipPaths
	}

	if c.ScopeParser != nil {
		return
	}

	if c.ScopeType == "" {
		c.ScopeType = basic.ScopeType
	}

	// Find scope parser based on ScopeType, if not already set
	parser, _ := scope.MatchCustomParser[ScopeParser](c.ScopeType)
	c.ScopeParser = parser
}

func (c *Config) Validate() error {
	if c.Enabled && len(c.Certificates) == 0 {
		return fmt.Errorf("certificateAuth is enabled but no allowed hosts certificates are configured")
	}

	for i, cert := range c.Certificates {
		if len(cert.CNs) == 0 {
			return fmt.Errorf("certificateAuth is enabled but no allowed hosts certificate CNs are configured")
		}
		for _, s := range cert.Scopes {
			if err := s.Validate(); err != nil {
				return fmt.Errorf("invalid scope in certificateAuth configuration at index %d: %w", i, err)

			}
		}
	}
	return nil
}

func Middleware(cfg Config) gin.HandlerFunc {
	if !cfg.Enabled {
		log.Infof("mTLS Authorization middleware is disabled")
		return func(c *gin.Context) {
			c.Next()
		}
	}

	log.Infof("Using mTLS Authorization middleware")
	return func(c *gin.Context) {
		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		span.SetName("mTLS Authorization Middleware")
		defer span.End()

		path := c.Request.URL.Path

		if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
			if !cfg.DisableLogging && !slices.Contains(cfg.SkipPaths, path) {
				log.WithContext(ctx).Errorf("[mTLS] No client certificate provided")
			}
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		cns := tlsutils.ExtractCommonNames(c.Request.TLS.PeerCertificates)
		dnsNames := tlsutils.ExtractDNSNames(c.Request.TLS.PeerCertificates)

		cert, matchBy, found := MatchCertificate(cfg.Certificates, slices.Concat(cns, dnsNames))
		if !found {
			if !cfg.DisableLogging && !slices.Contains(cfg.SkipPaths, path) {
				fields := log.Fields{"cns": cns, "dnsNames": dnsNames, "allowedHosts": cfg.Certificates}
				log.WithContext(ctx).WithFields(fields).Errorf("[mTLS] Client mTLS failed")
			}
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		account := &access.Account{
			Trust:  access.TrustCertificate,
			Name:   matchBy,
			Scopes: cert.Scopes,
		}
		if !cfg.DisableLogging && !slices.Contains(cfg.SkipPaths, path) {
			fields := log.Fields{"cns": cns, "dnsNames": dnsNames, "matchedBy": matchBy}
			log.WithContext(ctx).WithFields(fields).Infof("[auth][mtls] Matched certificate")
		}
		ctx, err := access.SetTrust(ctx, account)
		if err != nil {
			log.WithContext(ctx).WithError(err).Errorf("[mTLS] Error setting trust")
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// MatchCertificate checks for a matching host by CN
func MatchCertificate(hosts []Certificate, commonNames []string) (Certificate, string, bool) {
	for _, host := range hosts {
		if cn, match := host.MatchAnyCommonName(commonNames); match {
			return host, cn, true
		}
	}
	return Certificate{}, "", false
}
