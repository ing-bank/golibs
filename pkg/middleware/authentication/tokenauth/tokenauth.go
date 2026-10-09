package tokenauth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
)

// AuthenticatedUser context key for storing authenticated username
const CtxAuthenticatedUser = "authenticated-user"

// AuthenticatedToken context key for storing the raw token (for later use by JWT authz)
const CtxAuthenticatedToken = "authenticated-token"

func Middleware(cfg *Config) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	log.Infof("Using Token Authentication middleware")
	return func(c *gin.Context) {
		ctx, span := trace.NewSpanWithContext(c.Request.Context())
		defer span.End()

		header := c.GetHeader(cfg.Header)
		if header == "" {
			// No authentication header - let it pass (authorization middleware will decide what to do)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Validate scheme prefix
		schemePrefix := cfg.Scheme + " "
		if !strings.HasPrefix(header, schemePrefix) {
			// Scheme doesn't match - let it pass (could be different auth method)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Extract token
		token := strings.TrimPrefix(header, schemePrefix)
		if token == "" {
			log.WithContext(ctx).Warnf("[auth][token] Empty token in %s header", cfg.Header)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Parse token to get username
		username, err := cfg.TokenParser.ParseToken(c, token)
		if err != nil {
			log.WithContext(ctx).WithError(err).Warnf("[auth][token] Token parsing failed")
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}

		if username == "" {
			log.WithContext(ctx).Warnf("[auth][token] Token parser returned empty username")
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Store authenticated username and token in context
		ctx = context.WithValue(ctx, CtxAuthenticatedUser, username)
		ctx = context.WithValue(ctx, CtxAuthenticatedToken, token)

		log.WithContext(ctx).WithField("user", username).Infof("[auth][token] Authenticated user")

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetAuthenticatedUser retrieves the authenticated username from context.
// Returns empty string if no authenticated user found.
func GetAuthenticatedUser(c *gin.Context) string {
	val := c.Request.Context().Value(CtxAuthenticatedUser)
	if val == nil {
		return ""
	}
	user, ok := val.(string)
	if !ok {
		return ""
	}
	return user
}

// GetAuthenticatedToken retrieves the raw authenticated token from context.
// Returns empty string if no token found.
func GetAuthenticatedToken(c *gin.Context) string {
	val := c.Request.Context().Value(CtxAuthenticatedToken)
	if val == nil {
		return ""
	}
	token, ok := val.(string)
	if !ok {
		return ""
	}
	return token
}

