package tokenauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
)

// MockTokenParser for testing
type mockTokenParser struct {
	parseFunc func(c *gin.Context, token string) (*access.Account, error)
}

func (m *mockTokenParser) ParseToken(c *gin.Context, token string) (*access.Account, error) {
	if m.parseFunc != nil {
		return m.parseFunc(c, token)
	}
	return &access.Account{Name: "test-user"}, nil
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		header            string
		headerValue       string
		mockParser        func(c *gin.Context, token string) (*access.Account, error)
		expectedUser      string
		expectedToken     string
		shouldHaveContext bool
		shouldAbort       bool
	}{
		{
			name:        "Valid Bearer token",
			header:      "Authorization",
			headerValue: "Bearer test-token-123",
			mockParser: func(c *gin.Context, token string) (*access.Account, error) {
				return &access.Account{Name: "user1"}, nil
			},
			expectedUser:      "user1",
			expectedToken:     "test-token-123",
			shouldHaveContext: true,
		},
		{
			name:        "No Authorization header",
			header:      "Authorization",
			headerValue: "",
			mockParser: func(c *gin.Context, token string) (*access.Account, error) {
				return &access.Account{Name: "user1"}, nil
			},
			expectedUser:      "",
			expectedToken:     "",
			shouldHaveContext: false,
		},
		{
			name:        "Wrong scheme in header",
			header:      "Authorization",
			headerValue: "Basic dXNlcjpwYXNz",
			mockParser: func(c *gin.Context, token string) (*access.Account, error) {
				return &access.Account{Name: "user1"}, nil
			},
			expectedUser:      "",
			expectedToken:     "",
			shouldHaveContext: false,
		},
		{
			name:        "Empty token",
			header:      "Authorization",
			headerValue: "Bearer ",
			mockParser: func(c *gin.Context, token string) (*access.Account, error) {
				return &access.Account{Name: "user1"}, nil
			},
			expectedUser:      "",
			expectedToken:     "",
			shouldHaveContext: false,
		},
		{
			name:              "Parser returns nil account",
			header:            "Authorization",
			headerValue:       "Bearer test-token",
			mockParser:        func(c *gin.Context, token string) (*access.Account, error) { return nil, nil },
			expectedUser:      "",
			expectedToken:     "",
			shouldHaveContext: false,
		},
		{
			name:              "Parser returns error - should abort",
			header:            "Authorization",
			headerValue:       "Bearer invalid-token",
			mockParser:        func(c *gin.Context, token string) (*access.Account, error) { return nil, errors.New("invalid token") },
			expectedUser:      "",
			expectedToken:     "",
			shouldHaveContext: false,
			shouldAbort:       true,
		},
		{
			name:        "Custom header name",
			header:      "X-API-Token",
			headerValue: "Bearer custom-token-456",
			mockParser: func(c *gin.Context, token string) (*access.Account, error) {
				return &access.Account{Name: "user2"}, nil
			},
			expectedUser:      "user2",
			expectedToken:     "custom-token-456",
			shouldHaveContext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled:     true,
				Scheme:      "Bearer",
				Header:      tt.header,
				TokenParser: &mockTokenParser{parseFunc: tt.mockParser},
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "", nil)

			if tt.headerValue != "" {
				req.Header.Set(tt.header, tt.headerValue)
			}
			c.Request = req

			Middleware(cfg)(c)

			user := GetAuthenticatedUser(c)
			token := GetAuthenticatedToken(c)

			if tt.shouldAbort {
				if recorder.Code != http.StatusUnauthorized {
					t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
				}
			} else if tt.shouldHaveContext {
				if user != tt.expectedUser {
					t.Errorf("GetAuthenticatedUser() = %q, want %q", user, tt.expectedUser)
				}
				if token != tt.expectedToken {
					t.Errorf("GetAuthenticatedToken() = %q, want %q", token, tt.expectedToken)
				}
			} else {
				if user != "" {
					t.Errorf("Expected no user, but got %q", user)
				}
				if token != "" {
					t.Errorf("Expected no token, but got %q", token)
				}
			}
		})
	}
}

func TestMiddlewareDisabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req, _ := http.NewRequestWithContext(t.Context(), "GET", "", nil)
	req.Header.Set("Authorization", "Bearer should-be-ignored")
	c.Request = req

	Middleware(cfg)(c)

	user := GetAuthenticatedUser(c)
	token := GetAuthenticatedToken(c)

	if user != "" || token != "" {
		t.Error("Disabled middleware should not set context")
	}
}
