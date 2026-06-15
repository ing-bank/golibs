package timeout

import (
	"fmt"
	"net/http"

	gintimeout "github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/opt"

	"sync"
	"time"
)

var ErrTimeout = fmt.Errorf("request timed out")

type Timeout struct{}

type Options struct {
	Body func(c *gin.Context, err error) any
}

func timeoutResponse(fn func(c *gin.Context, err error) any) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusRequestTimeout, fn(c, ErrTimeout))
	}
}

func Middleware(timed time.Duration, opts ...Options) gin.HandlerFunc {
	o := opt.Opt(Options{NoContent}, opts)
	lock := &sync.Mutex{}
	return gintimeout.New(
		gintimeout.WithTimeout(timed),
		gintimeout.WithResponse(func(c *gin.Context) {
			lock.Lock()
			defer lock.Unlock()
			timeoutResponse(o.Body)(c)
		}),
	)
}

func NoContent(c *gin.Context, err error) any {
	return gin.H{
		"error": err.Error(),
	}
}
