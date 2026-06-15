package npa

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

type Config struct {
	Enabled     bool         `json:"enabled"` // Should be used by the caller, not used here
	Header      string       `json:"header"`
	AllowedNPAs []AllowedNPA `json:"allowedNPAs"`

	ScopeType   string      `json:"scopeType"`
	ScopeParser ScopeParser `json:"-"`
}

type AllowedNPA struct {
	Scopes    []scope.Scope     `json:"-"`
	RawScopes []json.RawMessage `json:"scopes"`
	Name      string            `json:"name"`
}

type ScopeParser interface {
	ParseNpaScope(json.RawMessage) (scope.Scope, error)
}

func (c *Config) ApplyDefaults() {
	if c.ScopeParser != nil {
		return
	}

	if c.ScopeType == "" {
		c.ScopeType = basic.ScopeType
	}

	// Find scope parser based on ScopeType, if not already set
	parser, _ := scope.MatchCustomParser[ScopeParser](c.ScopeType)
	c.ScopeParser = parser

	// Parse Raw JSON scope configs via parser, if needed
	for i, npa := range c.AllowedNPAs {
		if npa.Scopes == nil && npa.RawScopes != nil {
			npa.Scopes = make([]scope.Scope, len(npa.RawScopes))
			for j, rawScope := range npa.RawScopes {
				npaScope, _ := c.ScopeParser.ParseNpaScope(rawScope)
				c.AllowedNPAs[i].Scopes[j] = npaScope
			}
		}
	}
}

func (c *Config) Validate() error {
	if c.Enabled {
		if c.Header == "" {
			return errors.New("NPA header cannot be empty")
		}
		if len(c.AllowedNPAs) == 0 {
			return fmt.Errorf("allowedNPA middleware is enabled but no allowed NPAs are configured")
		}

		for i, npa := range c.AllowedNPAs {
			if npa.Name == "" {
				return errors.New("NPA name cannot be empty")
			}

			if npa.Scopes == nil {
				return fmt.Errorf("empty or unparsed scope in NPA configuration at index %d", i)
			}

			for j, auth := range npa.Scopes {
				if err := auth.Validate(); err != nil {
					return fmt.Errorf("invalid scope in NPA configuration at index %d - %d: %w", i, j, err)
				}
			}

		}
	}

	return nil
}
