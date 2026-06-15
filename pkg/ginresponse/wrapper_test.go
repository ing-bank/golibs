package ginresponse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWrap_JSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	u, _ := url.Parse("http://localhost")
	c.Request = &http.Request{Method: "GET", URL: u}
	handler := Wrap(func(c *gin.Context) (int, any) {
		return 200, map[string]string{"foo": "bar"}
	})
	// Should not panic
	handler(c)
}

func TestWrap_ErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	u, _ := url.Parse("http://localhost")
	c.Request = &http.Request{Method: "GET", URL: u}
	handler := Wrap(func(c *gin.Context) (int, any) {
		return 400, errors.New("bad request")
	})
	// Should not panic
	handler(c)
}

func TestEngine_HTTPHandler_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/err", Wrap(func(c *gin.Context) (int, any) {
		return http.StatusBadRequest, errors.New("fail")
	}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/err", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "fail")
}

func TestEngine_HTTPHandler_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/json", Wrap(func(c *gin.Context) (int, any) {
		return http.StatusOK, gin.H{"hello": "world"}
	}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/json", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello")
}
