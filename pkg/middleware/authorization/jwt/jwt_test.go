package jwt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/middleware/authentication/tokenauth"
)

// MockJWTParser for testing
type mockJWTParser struct {
	claimsFunc func(c *gin.Context, token string) ([]scope.Scope, error)
}

func (m *mockJWTParser) ParseClaims(c *gin.Context, token string) ([]scope.Scope, error) {
	if m.claimsFunc != nil {
		return m.claimsFunc(c, token)
	}
	return []scope.Scope{}, nil
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		setAuthContext    bool
		mockParser        func(c *gin.Context, token string) ([]scope.Scope, error)
		expectedScopes    []basic.Scope
		shouldHaveTrust   bool
		shouldSkip        bool // true if middleware should skip (no auth context)
	}{
		{
			name:           "Valid JWT with scopes",
			setAuthContext: true,
			mockParser: func(c *gin.Context, token string) ([]scope.Scope, error) {
				scopes := make([]scope.Scope, 1)
				scopes[0] = basic.Scope{
					Actions:      []string{"read"},
					Environments: []string{"prod"},
					Teams:        []string{"team-alpha"},
					Roles:        []string{"user"},
				}
				return scopes, nil
			},
			expectedScopes: []basic.Scope{
				{
					Actions:      []string{"read"},
					Environments: []string{"prod"},
					Teams:        []string{"team-alpha"},
					Roles:        []string{"user"},
				},
			},
			shouldHaveTrust: true,
		},
		{
			name:           "No authentication context",
			setAuthContext: false,
			mockParser: func(c *gin.Context, token string) ([]scope.Scope, error) {
				return []scope.Scope{}, nil
			},
			shouldSkip: true,
		},
		{
			name:           "JWT parsing returns error",
			setAuthContext: true,
			mockParser: func(c *gin.Context, token string) ([]scope.Scope, error) {
				return nil, errors.New("invalid JWT")
			},
			shouldSkip: true,
		},
		{
			name:           "JWT with empty scopes",
			setAuthContext: true,
			mockParser: func(c *gin.Context, token string) ([]scope.Scope, error) {
				return []scope.Scope{}, nil
			},
			shouldHaveTrust: true,
			expectedScopes: []basic.Scope{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled:   true,
				JWTParser: &mockJWTParser{claimsFunc: tt.mockParser},
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequestWithContext(t.Context(), "GET", "", nil)

			// Set up authentication context if needed
			if tt.setAuthContext {
				ctx := req.Context()
				ctx = context.WithValue(ctx, tokenauth.CtxAuthenticatedUser, "test-user")
				ctx = context.WithValue(ctx, tokenauth.CtxAuthenticatedToken, "test-jwt-token")
				req = req.WithContext(ctx)
			}

			c.Request = req

			Middleware(cfg)(c)

			trust := access.GetTrust(c.Request.Context())

			if tt.shouldSkip {
				if trust.Trust != access.TrustNone {
					t.Errorf("Expected no trust, but got %v", trust.Trust)
				}
			} else if tt.shouldHaveTrust {
				if trust.Trust != access.TrustUser {
					t.Errorf("Expected TrustUser, got %v", trust.Trust)
				}
				if trust.Name != "test-user" {
					t.Errorf("Expected username 'test-user', got %q", trust.Name)
				}
				if len(trust.Scopes) != len(tt.expectedScopes) {
					t.Errorf("Expected %d scopes, got %d", len(tt.expectedScopes), len(trust.Scopes))
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

	// Set up authentication context
	ctx := req.Context()
	ctx = context.WithValue(ctx, tokenauth.CtxAuthenticatedUser, "test-user")
	ctx = context.WithValue(ctx, tokenauth.CtxAuthenticatedToken, "test-token")
	req = req.WithContext(ctx)

	c.Request = req

	Middleware(cfg)(c)

	trust := access.GetTrust(c.Request.Context())
	if trust.Trust != access.TrustNone {
		t.Error("Disabled middleware should not set trust")
	}
}


