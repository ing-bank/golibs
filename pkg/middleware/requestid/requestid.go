package requestid

import (
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/utils"
)

type Config struct {
	Enabled bool `yaml:"enabled"`
}

func Middleware(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}
		ctx := c.Request.Context()

		// TODO: the context key should be configurable
		c.Request = c.Request.WithContext(utils.NewRequestID(ctx))

		c.Next()
	}
}
