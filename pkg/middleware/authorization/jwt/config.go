package jwt

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
)

// Config holds the configuration for JWT authorization middleware.
type Config struct {
	Enabled bool   `json:"enabled"`   // Should be used by the caller, not used here
	ScopeType string `json:"scopeType"` // Scope type for claim transformation

	JWTParser JWTParser `json:"-"`
}

// JWTParser transforms JWT claims to scopes.
type JWTParser interface {
	// ParseClaims transforms JWT claims to scopes.
	// The token has already been validated by the authentication middleware.
	ParseClaims(c *gin.Context, token string) ([]scope.Scope, error)
}

func (c *Config) ApplyDefaults() {
	// Defaults are typically handled by the implementation
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.JWTParser == nil {
		return errors.New("JWT parser is required")
	}

	return nil
}

