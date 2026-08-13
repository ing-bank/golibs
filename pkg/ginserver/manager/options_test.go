package manager

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/ginserver/proxy"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithProxycarOptions_WhenProxyDisabledAfterInitialization_ReturnsError(t *testing.T) {
	t.Parallel()

	m := &Manager{proxyInitialized: true}
	err := WithProxycarOptions(ginserver.WithAddr("127.0.0.1:9999"))(m)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxycar is not enabled")
}

func TestNewManager_AppliesProxycarOptionsBeforeProxyRoutes(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	m, err := NewManager(&Config{
		Config: ginserver.Config{
			HTTPServer: httpserver.Config{
				Host: "127.0.0.1",
				Port: 8080,
			},
			ServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{Enabled: false},
			},
		},
		ProxyConfig: ProxyConfig{
			Enabled: true,
			Config: proxy.Config{
				BasePath: "/proxy",
				Routes: []proxy.Route{
					{Prefix: "/svc", Target: upstream.URL},
				},
			},
		},
	}, WithProxycarOptions(ginserver.WithMiddleware(func(c *gin.Context) {
		c.Writer.Header().Set("X-Proxy-Middleware", "applied")
		c.Next()
	})))
	require.NoError(t, err)
	require.NotNil(t, m.Proxy)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/svc/resource", nil)
	m.Proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "applied", rec.Header().Get("X-Proxy-Middleware"))
}

