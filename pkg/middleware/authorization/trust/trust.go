package trust

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Enabled          bool            `json:"enabled"`
	RequiredScope    scope.Scope     `json:"-"`             // Any of these scopes is sufficient to access
	RawRequiredScope json.RawMessage `json:"requiredScope"` // Any of these scopes is sufficient to access

	ScopeType   string      `json:"scopeType"`
	ScopeParser ScopeParser `json:"-"`
}

type ScopeParser interface {
	ParseTrustScope(json.RawMessage) (scope.Scope, error)
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
	if c.RequiredScope == nil && c.RawRequiredScope != nil {
		trustScope, _ := c.ScopeParser.ParseTrustScope(c.RawRequiredScope)
		c.RequiredScope = trustScope
	}
}
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.RequiredScope == nil {
		return errors.New("required scope is missing")
	}
	if err := c.RequiredScope.Validate(); err != nil {
		return err
	}
	return nil
}

func Middleware(cfg Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		sar, err := access.NewSubjectAccessReview(c, cfg.RequiredScope)
		if err != nil {
			log.WithFields(log.Fields{
				"scope": cfg.RequiredScope, "error": err.Error(),
			}).Errorf("[trust] Access review error")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		log.WithField("sar", sar).Infof("[trust] Access review")
		if sar.Status.Allowed {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
