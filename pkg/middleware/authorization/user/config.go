package user

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

type Config struct {
	Enabled        bool   `json:"enabled"`        // Should be used by the caller, not used here
	UsernameHeader string `json:"usernameHeader"` // Header to identify username

	ScopeType   string      `json:"scopeType"`
	ScopeParser ScopeParser `json:"-"`
}

type ScopeParser interface {
	ParseUserHeader(c *gin.Context) []scope.Scope
}

func (c *Config) ApplyDefaults() {
	if c.ScopeType == "" {
		c.ScopeType = basic.ScopeType
	}

	if c.ScopeParser != nil {
		return
	}

	parser, err := scope.MatchCustomParser[ScopeParser](c.ScopeType)
	if err != nil {
		return
	}
	c.ScopeParser = parser
}

func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.UsernameHeader == "" {
		return errors.New("username header is required")
	}
	if c.ScopeParser == nil {
		return errors.New("no parser found")
	}

	return nil
}
