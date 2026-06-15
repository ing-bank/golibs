// The Gin Timeout middleware fails the race tests by default, this is outside our control. Perhaps we should write our own.
//go:build !race

package timeout

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Middleware(100*time.Millisecond, Options{
		Body: func(c *gin.Context, err error) any { return "timeout occurred" },
	}))
	router.GET("/test", func(c *gin.Context) {
		time.Sleep(500 * time.Millisecond)
		c.String(http.StatusOK, "success")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, `"timeout occurred"`, w.Body.String())
}

func TestMiddleware_NoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Middleware(100 * time.Millisecond)) // No Body specified, should use NoContent
	router.GET("/nocontext", func(c *gin.Context) {
		time.Sleep(500 * time.Millisecond)
		c.String(http.StatusOK, "success")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/nocontext", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, fmt.Sprintf(`{"error":"%s"}`, ErrTimeout.Error()), w.Body.String()) // NoContent returns nil, which is marshaled as null
}
