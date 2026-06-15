package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

// Middleware sets gin context with NPA details, if any. Does *not* abort other middleware if no NPA is found.
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

		name := c.GetHeader(cfg.UsernameHeader)
		if name != "" {
			account := &access.Account{
				Trust:  access.TrustUser,
				Name:   name,
				Scopes: cfg.ScopeParser.ParseUserHeader(c),
			}

			log.WithContext(ctx).WithField("account", account).Infof("[auth][user] Authenticated user")

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
