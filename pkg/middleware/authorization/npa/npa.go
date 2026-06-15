// Package npa is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
package npa

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

// Middleware sets gin context with NPA details, if any. Does *not* abort other middleware if no NPA is found.
func Middleware(cfg Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	log.Infof("Using NPA Authorization middleware")

	// Create lookup table for NPA name to its scope
	lookup := slices.Map(cfg.AllowedNPAs, func(item AllowedNPA) string { return strings.ToLower(item.Name) })

	return func(c *gin.Context) {
		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		defer span.End()

		name := c.GetHeader(cfg.Header)
		auth, ok := lookup[strings.ToLower(name)]

		if ok {
			account := &access.Account{
				Trust:  access.TrustNPA,
				Name:   name,
				Scopes: auth.Scopes,
			}

			log.WithContext(ctx).WithField("account", account).WithField("auth", auth).Infof("[auth][npa] Authenticated NPA")

			var err error
			ctx, err = access.SetTrust(ctx, account)
			if err != nil {
				log.WithContext(ctx).WithError(err).Errorf("[mTLS] Error setting trust")
				_ = c.AbortWithError(http.StatusUnauthorized, err)
				return
			}
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
