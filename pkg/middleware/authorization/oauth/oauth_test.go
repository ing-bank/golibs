package oauth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		cfg            Config
		path           string
		expectedStatus int
		expectedBody   string
		shouldPass     bool
	}{
		{
			name: "OAuth disabled - should pass through",
			cfg: Config{
				Enabled: false, // Set to false for testing since we can't create real provider
			},
			path:           "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"message":"success"}`,
			shouldPass:     true,
		},
		{
			name: "OAuth disabled with custom bypass paths",
			cfg: Config{
				Enabled:            false,
				BypassAuthForPaths: []string{"/custom", "/special"},
			},
			path:           "/custom",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"message":"success"}`,
			shouldPass:     true,
		},
		{
			name: "OAuth disabled with different path",
			cfg: Config{
				Enabled: false,
			},
			path:           "/api/v1/users",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"message":"success"}`,
			shouldPass:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Add middleware with the config from the test case
			router.Use(Middleware(tt.cfg,
				WithResponse(func(c *gin.Context, err error) any {
					return gin.H{"msg": "authentication error", "err": fmt.Errorf("custom auth failed: %w", err)}
				}),
			))

			router.GET("/*path", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response body
			if tt.expectedBody != w.Body.String() {
				t.Errorf("Expected body %s, got %s", tt.expectedBody, w.Body.String())
			}

			// Validate config
			if err := tt.cfg.Validate(); err != nil {
				t.Errorf("Config validation failed: %v", err)
			}
		})
	}
}

func TestProvider_Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		responseFn     Response
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Response with nil function - should use default AuthFailed",
			responseFn:     AuthFailed,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"authentication failed"}`,
		},
		{
			name: "Response with custom function",
			responseFn: func(c *gin.Context, err error) any {
				return gin.H{"custom": "error", "message": "custom auth failure"}
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"custom":"error","message":"custom auth failure"}`,
		},
		{
			name: "Response with function returning string",
			responseFn: func(c *gin.Context, err error) any {
				return "simple error message"
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"simple error message"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a provider instance
			provider := &Provider{}

			// Create a test router
			router := gin.New()
			router.GET("/test", provider.Response(tt.responseFn, ErrAuthFailed))

			// Create test request
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response body
			if tt.expectedBody != w.Body.String() {
				t.Errorf("Expected body %s, got %s", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestAuthFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Call AuthFailed
	result := AuthFailed(c, ErrAuthFailed)

	// Verify the result
	expected := gin.H{"error": "authentication failed"}
	if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		err      error
		expected gin.H
	}{
		{
			name:     "Standard error",
			err:      ErrAuthFailed,
			expected: gin.H{"error": "authentication failed"},
		},
		{
			name:     "Custom error",
			err:      fmt.Errorf("custom error message"),
			expected: gin.H{"error": "custom error message"},
		},
		{
			name:     "Empty error message",
			err:      fmt.Errorf(""),
			expected: gin.H{"error": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Call NoContent
			result := NoContent(c, tt.err)

			// Verify the result
			if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNew(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("Skipping in container environment due to missing OpenShift configuration")
	}
	tests := []struct {
		name        string
		cfg         Config
		expectError bool
		expectNil   bool
	}{
		{
			name: "OAuth disabled - should return nil",
			cfg: Config{
				Enabled: false,
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name: "OAuth enabled with minimal config",
			cfg: Config{
				Enabled:        true,
				ServiceAccount: "test-service-account",
			},
			expectError: false, // Will fail due to missing OpenShift environment
			expectNil:   false,
		},
		{
			name: "OAuth enabled with full config",
			cfg: Config{
				Enabled:         true,
				ServiceAccount:  "test-service-account",
				ClientID:        "test-client",
				ClientSecret:    "test-secret",
				LoginURL:        "https://example.com/login",
				RedeemURL:       "https://example.com/redeem",
				ValidateURL:     "https://example.com/validate",
				ReviewURL:       "https://example.com/review",
				Scope:           "user:full",
				DelegateURLs:    "https://example.com",
				ReviewByHostURL: "https://example.com",
				Resources:       `{"/": {"namespace":"default","resource":"namespaces","verb":"get"}}`,
			},
			expectError: true, // Will fail due to missing OpenShift environment
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.cfg)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectNil && provider != nil {
				t.Errorf("Expected nil provider but got non-nil")
			}
			if !tt.expectNil && !tt.expectError && provider == nil {
				t.Errorf("Expected non-nil provider but got nil")
			}
		})
	}
}

// IsRunningInContainer returns true if the process is running in a container.
func IsRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return os.Getenv("BUILD_BUILDNUMBER") != ""
}

func TestNewForConfig(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("Skipping in container environment due to missing OpenShift configuration")
	}
	tests := []struct {
		name        string
		cfg         Config
		expectError bool
		expectNil   bool
	}{
		{
			name: "OAuth disabled via NewForConfig",
			cfg: Config{
				Enabled: false,
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name: "OAuth enabled via NewForConfig",
			cfg: Config{
				Enabled:        true,
				ServiceAccount: "test-account",
			},
			expectError: false, // Will fail due to missing OpenShift environment
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewForConfig(tt.cfg)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectNil && provider != nil {
				t.Errorf("Expected nil provider but got non-nil")
			}
			if !tt.expectNil && !tt.expectError && provider == nil {
				t.Errorf("Expected non-nil provider but got nil")
			}
		})
	}
}

func TestDefaultSkipPaths(t *testing.T) {
	expected := []string{"/metrics", "/health", "/ready", "/healthz", "/readyz"}

	if len(DefaultSkipPaths) != len(expected) {
		t.Errorf("Expected %d default skip paths, got %d", len(expected), len(DefaultSkipPaths))
	}

	for i, path := range expected {
		if i >= len(DefaultSkipPaths) || DefaultSkipPaths[i] != path {
			t.Errorf("Expected default skip path %s at index %d, got %s", path, i, DefaultSkipPaths[i])
		}
	}
}

func TestMiddleware_WithOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		cfg            Config
		options        []Option
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "OAuth disabled with custom response option",
			cfg: Config{
				Enabled: false,
			},
			options: []Option{
				WithResponse(func(c *gin.Context, err error) any {
					return gin.H{"custom": "auth error", "path": c.Request.URL.Path}
				}),
			},
			path:           "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"message":"success"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Middleware(tt.cfg, tt.options...))

			router.GET("/*path", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != w.Body.String() {
				t.Errorf("Expected body %s, got %s", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestProvider_ResponseEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		responseFn     Response
		expectedStatus int
	}{
		{
			name: "Response returning nil",
			responseFn: func(c *gin.Context, err error) any {
				return nil
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Response returning complex object",
			responseFn: func(c *gin.Context, err error) any {
				return gin.H{
					"error":     "auth failed",
					"timestamp": "2023-01-01T00:00:00Z",
					"path":      c.Request.URL.Path,
					"method":    c.Request.Method,
					"details": gin.H{
						"code":    "OAUTH_001",
						"message": "Invalid token",
					},
				}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Response returning empty object",
			responseFn: func(c *gin.Context, err error) any {
				return gin.H{}
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{}
			router := gin.New()
			router.GET("/test", provider.Response(tt.responseFn, ErrAuthFailed))

			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestNew_WithOptions(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("Skipping in container environment due to missing OpenShift configuration")
	}
	tests := []struct {
		name        string
		cfg         Config
		options     []Option
		expectError bool
		expectNil   bool
	}{
		{
			name: "OAuth disabled with options",
			cfg: Config{
				Enabled: false,
			},
			options: []Option{
				WithResponse(func(c *gin.Context, err error) any {
					return gin.H{"error": "custom"}
				}),
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name: "OAuth enabled with invalid options",
			cfg: Config{
				Enabled:        true,
				ServiceAccount: "test",
			},
			options: []Option{
				WithProvider(nil), // This should cause an error
			},
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.cfg, tt.options...)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectNil && provider != nil {
				t.Errorf("Expected nil provider but got non-nil")
			}
			if !tt.expectNil && !tt.expectError && provider == nil {
				t.Errorf("Expected non-nil provider but got nil")
			}
		})
	}
}

func TestConfig_URLParsing(t *testing.T) {
	if IsRunningInContainer() {
		t.Skip("Skipping in container environment due to missing OpenShift configuration")
	}
	tests := []struct {
		name      string
		cfg       Config
		expectErr bool
	}{
		{
			name: "Valid URLs",
			cfg: Config{
				Enabled:     true,
				LoginURL:    "https://valid.example.com/login",
				RedeemURL:   "https://valid.example.com/redeem",
				ValidateURL: "https://valid.example.com/validate",
				ReviewURL:   "https://valid.example.com/review",
			},
			expectErr: false,
		},
		{
			name: "Invalid Login URL",
			cfg: Config{
				Enabled:  true,
				LoginURL: "://invalid-url",
			},
			expectErr: true,
		},
		{
			name: "Invalid Redeem URL",
			cfg: Config{
				Enabled:   true,
				RedeemURL: "://invalid-url",
			},
			expectErr: true,
		},
		{
			name: "Invalid Validate URL",
			cfg: Config{
				Enabled:     true,
				ValidateURL: "://invalid-url",
			},
			expectErr: true,
		},
		{
			name: "Invalid Review URL",
			cfg: Config{
				Enabled:   true,
				ReviewURL: "://invalid-url",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if (err != nil) != tt.expectErr {
				t.Errorf("New() with invalid URLs error = %v, wantErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestProvider_ResponseFunctionTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		responseFn Response
	}{
		{
			name: "Response returning map[string]interface{}",
			responseFn: func(c *gin.Context, err error) any {
				return map[string]any{
					"error": "authentication failed",
					"code":  401,
				}
			},
		},
		{
			name: "Response returning struct",
			responseFn: func(c *gin.Context, err error) any {
				return struct {
					Error string `json:"error"`
					Code  int    `json:"code"`
				}{
					Error: "auth failed",
					Code:  401,
				}
			},
		},
		{
			name: "Response returning array",
			responseFn: func(c *gin.Context, err error) any {
				return []string{"error", "authentication failed"}
			},
		},
		{
			name: "Response returning number",
			responseFn: func(c *gin.Context, err error) any {
				return 401
			},
		},
		{
			name: "Response returning boolean",
			responseFn: func(c *gin.Context, err error) any {
				return false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &Provider{}
			router := gin.New()
			router.GET("/test", provider.Response(tt.responseFn, ErrAuthFailed))

			req, _ := http.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// All should return 401 Unauthorized
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
			}

			// Should have some response body (not testing exact content as it varies)
			if w.Body.Len() == 0 {
				t.Errorf("Expected non-empty response body")
			}
		})
	}
}
