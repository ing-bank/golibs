// Package basic provides a convenience wrapper for Basic authentication.
//
// This middleware is a thin wrapper around tokenauth middleware that automatically
// configures it for Basic authentication:
// - Scheme: "Basic"
// - Header: "Authorization"
//
// For custom configurations, use the tokenauth package directly.
//
package basic

import (
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/middleware/authentication/tokenauth"
)

// Middleware returns a Basic authentication middleware configured with
// the Authorization header and Basic scheme.
func Middleware(cfg *Config) gin.HandlerFunc {
	return tokenauth.Middleware(&tokenauth.Config{
		Enabled:     cfg.Enabled,
		Scheme:      "Basic",
		Header:      "Authorization",
		TokenParser: cfg.TokenParser,
	})
}

// GetAuthenticatedUser retrieves the authenticated username from context.
// This is a convenience function that wraps tokenauth.GetAuthenticatedUser.
func GetAuthenticatedUser(c *gin.Context) string {
	return tokenauth.GetAuthenticatedUser(c)
}

// GetAuthenticatedToken retrieves the raw authenticated token from context.
// This is a convenience function that wraps tokenauth.GetAuthenticatedToken.
func GetAuthenticatedToken(c *gin.Context) string {
	return tokenauth.GetAuthenticatedToken(c)
}

// CtxAuthenticatedUser is the context key for authenticated username.
const CtxAuthenticatedUser = tokenauth.CtxAuthenticatedUser

// CtxAuthenticatedToken is the context key for authenticated token.
const CtxAuthenticatedToken = tokenauth.CtxAuthenticatedToken


