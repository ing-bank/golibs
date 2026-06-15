package ginserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func randomAddr() string {
	return fmt.Sprintf(":%d", 10000+time.Now().Nanosecond()%20000)
}

type mockRoute struct {
	registered bool
}

func (m *mockRoute) Register(r gin.IRouter) {
	m.registered = true
}

func TestNew_DefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, err := New(WithAddr(randomAddr()))
	assert.NoError(t, err)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.Engine)
}

func TestNewForConfig_NilOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := DefaultConfig()
	engine, err := NewForConfig(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestEngine_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	route := &mockRoute{}
	engine.Register(route)
	assert.True(t, route.registered)
}

func TestRegisterRoutes_NilRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, nil)
	// Should not panic or error
}

func TestEngine_Shutdown_NoTraceProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	err := engine.HTTPServer.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestDefaultEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := DefaultEngine()
	assert.NotNil(t, engine)
}

func TestEngine_RunHeathCheckBackground_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_ = engine.RunBackground(ctx)
	defer cancel()
	if err := engine.Wait(); err != nil {
		assert.NoError(t, err)
	}
}

func TestEngine_RunBackground_NilHealthCheckClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	err := engine.Run(ctx)
	assert.NoError(t, err)
}

func TestEngine_HTTPHandler_NilWrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/not-found", nil)
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code) // Gin default for no handler
}

func TestRegisterRoutes_MultipleRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r1 := &mockRoute{}
	r2 := &mockRoute{}
	RegisterRoutes(engine, r1, r2)
	assert.True(t, r1.registered)
	assert.True(t, r2.registered)
}

func TestEngine_Register_NilRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, _ := New(WithAddr(randomAddr()))
	engine.Register(nil)
	// Should not panic or error
}

func TestEngine_Wait(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("normal operation", func(t *testing.T) {
		engine, err := New(WithAddr(randomAddr()))
		assert.NoError(t, err)
		assert.NotNil(t, engine)
		// Wait should return immediately since server is not running
		err = engine.Wait()
		assert.NoError(t, err)
	})
}
