package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/task/job"
)

// HandlerFunc is a helper to turn a function into a checks.Handler
// for use in tests.
type HandlerFunc func(ctx context.Context) error

func (f HandlerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

func newTestHealthCheck(t *testing.T) *HealthCheck {
	h, err := New(
		WithSystemInfo(),
		WithComponent(&Component{
			Name:    "myservice",
			Version: "v1.0",
		}),
		WithAddChecks(
			JobConfig{
				Config: job.Config{
					Name:        "check-success",
					Description: "A check that always succeeds",
					Timeout:     5 * time.Second,
				},
				CustomHandler: HandlerFunc(func(context.Context) error { return nil }),
				Endpoints: []Endpoint{
					HealthEndpoint,
					ReadyEndpoint,
				},
			},
			JobConfig{
				Config: job.Config{
					Name:        "check-fail",
					Description: "A check that always fails",
					MayFail:     true,
					Timeout:     5 * time.Minute,
				},
				CustomHandler: HandlerFunc(func(context.Context) error { return fmt.Errorf("failed during custom health check") }),
				Endpoints: []Endpoint{
					ReadyEndpoint,
				},
			},
		),
	)
	if err != nil {
		t.Fatalf("Failed to create HealthCheck: %v", err)
	}
	// --- Populate the cache with job results ---
	ctx := t.Context()
	jobs := h.AllChecks()
	for i, j := range jobs {
		result := job.Run(ctx, j)
		_ = h.cache.Apply(ctx, i, *result)
	}
	return h
}

func TestHandle_Cases(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		endpoint       Endpoint
		expectedCode   int
		expectedStatus string
	}{
		{
			name:           "OK/PartialContent",
			endpoint:       HealthEndpoint,
			expectedCode:   206,
			expectedStatus: "Partial Content",
		},
		{
			name:           "PartialContent",
			endpoint:       ReadyEndpoint,
			expectedCode:   206,
			expectedStatus: "Partial Content",
		},
		{
			name:           "ServiceUnavailable",
			endpoint:       StatusEndpoint,
			expectedCode:   503,
			expectedStatus: "Service Unavailable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHealthCheck(t)
			jobs := h.JobsForEndpoint(tc.endpoint)
			code, result := h.handle(t.Context(), jobs)
			if code != tc.expectedCode {
				t.Errorf("Expected %d, got %d", tc.expectedCode, code)
			}
			if r, ok := result.(*Response); !ok || r.Status != tc.expectedStatus {
				t.Errorf("Expected result status '%s', got %+v", tc.expectedStatus, result)
			}
		})
	}
}

func TestRouteRegisterAndRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("RouteRegister", func(t *testing.T) {
		engine := gin.New()
		h := newTestHealthCheck(t)
		h.RouteRegister(engine)
		routes := engine.Routes()
		var foundStatus, foundHealthz, foundReadyz bool
		for _, r := range routes {
			if r.Path == "/status" && r.Method == http.MethodGet {
				foundStatus = true
			}
			if r.Path == "/healthz" && r.Method == http.MethodGet {
				foundHealthz = true
			}
			if r.Path == "/readyz" && r.Method == http.MethodGet {
				foundReadyz = true
			}
		}
		if !foundStatus {
			t.Error("Route /status not registered by RouteRegister")
		}
		if !foundHealthz {
			t.Error("Route /healthz not registered by RouteRegister")
		}
		if !foundReadyz {
			t.Error("Route /readyz not registered by RouteRegister")
		}
	})

	t.Run("Register", func(t *testing.T) {
		engine := gin.New()
		h := newTestHealthCheck(t)
		h.Register(engine)
		routes := engine.Routes()
		var foundStatus, foundHealthz, foundReadyz bool
		for _, r := range routes {
			if r.Path == "/status" && r.Method == http.MethodGet {
				foundStatus = true
			}
			if r.Path == "/healthz" && r.Method == http.MethodGet {
				foundHealthz = true
			}
			if r.Path == "/readyz" && r.Method == http.MethodGet {
				foundReadyz = true
			}
		}
		if !foundStatus {
			t.Error("Route /status not registered by Register")
		}
		if !foundHealthz {
			t.Error("Route /healthz not registered by Register")
		}
		if !foundReadyz {
			t.Error("Route /readyz not registered by Register")
		}
	})
}

func TestHandlerFuncReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHealthCheck(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/readyz", nil)
	result := h.HandlerFuncReadyz(c)
	if result.StatusCode != 206 {
		t.Errorf("Expected 206, got %d", result.StatusCode)
	}
	if r, ok := result.Body.(*Response); !ok || r.Status != "Partial Content" {
		t.Errorf("Expected result status 'Partial Content', got %+v", result.Body)
	}
}

func TestHandlerFuncHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHealthCheck(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/healthz", nil)
	result := h.HandlerFuncHealthz(c)
	if result.StatusCode != 206 {
		t.Errorf("Expected 206, got %d", result.StatusCode)
	}
	if r, ok := result.Body.(*Response); !ok || r.Status != "Partial Content" {
		t.Errorf("Expected result status 'Partial Content', got %+v", result.Body)
	}
}

func TestHandlerFuncStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHealthCheck(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/status", nil)
	result := h.HandlerFuncStatus(c)
	if result.StatusCode != 206 {
		t.Errorf("Expected 206, got %d", result.StatusCode)
	}
	if r, ok := result.Body.(*Response); !ok || r.Status != "Partial Content" {
		t.Errorf("Expected result status 'Partial Content', got %+v", result.Body)
	}
}
