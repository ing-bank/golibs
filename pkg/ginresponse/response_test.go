package ginresponse

import (
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/http"
)

type App struct{}

func (a *App) Handler(_ *gin.Context) *Response {
	payload := gin.H{"message": "Hello, World!"}
	return New(payload)
}

func (a *App) HandlerWithHeader(_ *gin.Context) *Response {
	payload := gin.H{"message": "Hello, World!"}
	return New(payload).WithHeader("foo", "bar")
}

func (a *App) HandlerWithStatus(_ *gin.Context) *Response {
	payload := gin.H{"message": "Hello, World!"}
	return New(payload).WithStatus(202)
}

func (a *App) HandlerWithMultipleOptions(_ *gin.Context) *Response {
	payload := gin.H{"message": "Hello, World!"}
	return New(payload).WithHeader("foo", "bar").WithStatus(202)
}

func (a *App) HandlerWithErr(_ *gin.Context) *Response {
	err := http.ErrConflict // Is translated to 409 by errors package default error code translator
	return NewWithError(err)
}

func (a *App) HandlerWithHeaders(_ *gin.Context) *Response {
	payload := gin.H{"message": "Hello, World!"}
	return New(payload).WithHeaders(map[string]string{"foo": "bar", "baz": "qux"})
}

func (a *App) Register(rg gin.IRouter) {
	w, _ := NewWrapper()

	rg.GET("/handler", w.Wrap(a.Handler))
	rg.GET("/handler-header", w.Wrap(a.HandlerWithHeader))
	rg.GET("/handler-status", w.Wrap(a.HandlerWithStatus))
	rg.GET("/handler-multi", w.Wrap(a.HandlerWithMultipleOptions))
	rg.GET("/handler-err", w.Wrap(a.HandlerWithErr))
	rg.GET("/handler-headers", w.Wrap(a.HandlerWithHeaders))
}

func TestHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &App{}
	w, _ := NewWrapper()

	tests := []struct {
		name       string
		handler    func(*gin.Context) *Response
		expectCode int
		expectBody string
		expectHdr  map[string]string
	}{
		{
			name:       "Handler",
			handler:    app.Handler,
			expectCode: 200,
			expectBody: `{"message":"Hello, World!"}`,
			expectHdr:  map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name:       "HandlerWithHeader",
			handler:    app.HandlerWithHeader,
			expectCode: 200,
			expectBody: `{"message":"Hello, World!"}`,
			expectHdr:  map[string]string{"foo": "bar"},
		},
		{
			name:       "HandlerWithStatus",
			handler:    app.HandlerWithStatus,
			expectCode: 202,
			expectBody: `{"message":"Hello, World!"}`,
			expectHdr:  map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name:       "HandlerWithMultipleOptions",
			handler:    app.HandlerWithMultipleOptions,
			expectCode: 202,
			expectBody: `{"message":"Hello, World!"}`,
			expectHdr:  map[string]string{"foo": "bar"},
		},
		{
			name:       "HandlerWithErr",
			handler:    app.HandlerWithErr,
			expectCode: 409,
			expectBody: `{"error":"conflict"}`,
			expectHdr:  map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name:       "HandlerWithHeaders",
			handler:    app.HandlerWithHeaders,
			expectCode: 200,
			expectBody: `{"message":"Hello, World!"}`,
			expectHdr:  map[string]string{"foo": "bar", "baz": "qux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("http://localhost")
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = &nethttp.Request{Method: "GET", URL: u}
			w.Wrap(tt.handler)(c)

			if recorder.Code != tt.expectCode {
				t.Errorf("expected status %d, got %d", tt.expectCode, recorder.Code)
			}
			if body := recorder.Body.String(); body != tt.expectBody {
				t.Errorf("expected body %q, got %q", tt.expectBody, body)
			}
			for k, v := range tt.expectHdr {
				if got := recorder.Header().Get(k); got != v {
					t.Errorf("expected header %q=%q, got %q", k, v, got)
				}
			}
		})
	}
}
