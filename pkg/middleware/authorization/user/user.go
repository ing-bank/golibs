// Package user provides Gin middleware for user authorization.
//
// This middleware extracts user information and scopes from trusted headers
// and sets up the authorization context for downstream handlers.
//
// The header value is treated as a username/identity, and scopes are extracted
// via the ScopeParser.
//
// Configuration Options:
//   - Enabled: Whether the middleware is active
//   - UsernameHeader: The header name to read user information from (e.g., "X-User")
//   - ScopeType: The scope type to use for parsing (e.g., "basic")
//   - ScopeParser: Custom implementation that extracts scopes from the header value
//
// IMPORTANT: This middleware should only be used when the application is protected
// by an authenticating gateway or reverse proxy that validates and injects the
// username header. Do NOT use this middleware to read headers directly from
// untrusted clients, as those headers can be spoofed and users can impersonate
// other identities. For direct token authentication, use the authentication
// middleware instead (e.g., bearer, basic) to verify user identities first.
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

func Middleware(cfg *Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	log.Infof("Using User Authorization middleware")
	return func(c *gin.Context) {
		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		defer span.End()

		// Read username from configured header (must be from trusted gateway)
		username := c.GetHeader(cfg.UsernameHeader)
		if username != "" {
			account := &access.Account{
				Trust:  access.TrustUser,
				Name:   username,
				Scopes: cfg.ScopeParser.ParseUserHeader(c, username),
			}

			log.WithContext(ctx).WithField("account", account).Infof("[authz][user] Set user authorization")

			var err error
			ctx, err = access.SetTrust(ctx, account)
			if err != nil {
				log.WithContext(ctx).WithError(err).Errorf("[authz][user] Error setting trust")
				_ = c.AbortWithError(http.StatusUnauthorized, err)
				return
			}
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
