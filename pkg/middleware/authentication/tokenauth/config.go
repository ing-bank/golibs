// Package tokenauth provides generic token-based authentication middleware.
//
// This middleware validates tokens from HTTP headers and extracts authenticated user identities.
// It supports any token scheme (Bearer, Basic, custom) through configurable scheme validation
// and customizable token parsing via the TokenParser interface.
//
package tokenauth

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
)

// Config holds the configuration for token authentication middleware.
type Config struct {
	Enabled bool   `json:"enabled"` // Should be used by the caller, not used here
	Scheme  string `json:"scheme"`  // Auth scheme prefix (e.g., "Bearer", "Basic")
	Header  string `json:"header"`  // Header name to read from (typically "Authorization")

	TokenParser TokenParser `json:"-"`
}

// TokenParser extracts username and additional metadata from a token.
type TokenParser interface {
	// ParseToken extracts the authenticated username from the token.
	// It validates the token and returns the username.
	// If an error is returned, the authentication fails and the request is rejected.
	// If an empty string is returned with nil error, the middleware passes the request through
	// without setting authentication context (allowing other auth methods to handle it).
	ParseToken(c *gin.Context, token string) (username string, err error)
}

// JWTParser additionally transforms JWT claims to scopes.
// This interface is used by JWT authorization middleware.
type JWTParser interface {
	TokenParser
	// ParseClaims transforms JWT claims to scopes.
	// This is typically called by JWT authorization middleware, not by this package.
	ParseClaims(c *gin.Context, token string) ([]scope.Scope, error)
}

func (c *Config) ApplyDefaults() {
	if c.Header == "" {
		c.Header = "Authorization"
	}
	if c.Scheme == "" {
		c.Scheme = "Bearer"
	}
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Header == "" {
		return errors.New("header is required")
	}
	if c.Scheme == "" {
		return errors.New("scheme is required")
	}
	if c.TokenParser == nil {
		return errors.New("token parser is required")
	}

	return nil
}

