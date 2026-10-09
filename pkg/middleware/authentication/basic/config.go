package basic

import (
	"errors"

	"github.com/ing-bank/golibs/pkg/middleware/authentication/tokenauth"
)

// Config holds the configuration for Basic authentication.
// This is a convenience wrapper that pre-configures the tokenauth middleware
// for Basic authentication on the Authorization header.
type Config struct {
	Enabled     bool                    `json:"enabled"`
	TokenParser tokenauth.TokenParser   `json:"-"`
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.TokenParser == nil {
		return errors.New("token parser is required")
	}

	return nil
}

