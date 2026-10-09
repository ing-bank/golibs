// Package jwt provides Gin middleware for JWT-based authorization.
//
// This middleware transforms JWT claims into authorization scopes.
// It works in conjunction with bearer token authentication middleware.
//
// The middleware:
// 1. Reads the authenticated token from context (set by bearer authentication middleware)
// 2. Parses JWT claims using the configured JWTParser
// 3. Transforms claims into authorization scopes
// 4. Sets the authenticated username and scopes in the trust context
//
// This middleware handles AUTHORIZATION (determining what scopes/permissions the user has).
// It requires that the bearer authentication middleware has already run and set the token in context.
//
// Flow:
//   [Bearer Authentication] -> sets CtxAuthenticatedUser + CtxAuthenticatedToken in context
//   ↓
//   [JWT Authorization] -> reads token, parses claims, transforms to scopes, sets trust
//   ↓
//   [Handler]
//
package jwt

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/middleware/authentication/tokenauth"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

func Middleware(cfg *Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	log.Infof("Using JWT Authorization middleware")
	return func(c *gin.Context) {
		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		defer span.End()

		// Check if authentication middleware has already set trust in context
		existingTrust := access.GetTrust(ctx)
		if existingTrust.Trust != access.TrustNone {
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Try to get authenticated token from authentication middleware
		token := tokenauth.GetAuthenticatedToken(c)
		if token == "" {
			// No authenticated token - let it pass
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Get username from authentication
		username := tokenauth.GetAuthenticatedUser(c)
		if username == "" {
			log.WithContext(ctx).Warnf("[authz][jwt] Authenticated token but no username")
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Parse JWT claims to scopes
		scopes, err := cfg.JWTParser.ParseClaims(c, token)
		if err != nil {
			log.WithContext(ctx).WithError(err).Warnf("[authz][jwt] Failed to parse JWT claims")
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Create account with parsed scopes
		account := &access.Account{
			Trust:  access.TrustUser,
			Name:   username,
			Scopes: scopes,
		}

		log.WithContext(ctx).WithField("account", account).Infof("[authz][jwt] Set JWT authorization")

		var err2 error
		ctx, err2 = access.SetTrust(ctx, account)
		if err2 != nil {
			log.WithContext(ctx).WithError(err2).Errorf("[authz][jwt] Error setting trust")
			_ = c.AbortWithError(http.StatusUnauthorized, err2)
			return
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

