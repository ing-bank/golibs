package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/access/scope/dynamic"
)

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name string
		// Given
		user   string
		scopes []basic.Scope

		// Want
		expectedScopes []basic.Scope
		empty          bool
	}{
		{
			name: "Legitimate user with scopes",
			user: "foo",
			scopes: []basic.Scope{
				{
					Actions:      []string{scope.Wildcard},
					Environments: []string{"dev", "tst"},
					Teams:        []string{"team-alpha"},
					Roles:        []string{"user"},
				},
				{
					Actions:      []string{http.MethodGet},
					Environments: []string{"acc", "prd"},
					Teams:        []string{"team-beta"},
					Roles:        []string{"user"},
				},
			},
			expectedScopes: []basic.Scope{
				{
					Actions:      []string{scope.Wildcard},
					Environments: []string{"dev", "tst"},
					Teams:        []string{"team-alpha"},
					Roles:        []string{"user"},
				},
				{
					Actions:      []string{http.MethodGet},
					Environments: []string{"acc", "prd"},
					Teams:        []string{"team-beta"},
					Roles:        []string{"user"},
				},
			},
		},
		{
			name:   "User without username header",
			scopes: []basic.Scope{},
			empty:  true,
		},
		{
			name: "User with single scope",
			user: "foo",
			scopes: []basic.Scope{
				{
					Actions:      []string{http.MethodPost},
					Environments: []string{"dev"},
					Teams:        []string{"team-gamma"},
					Roles:        []string{"admin"},
				},
			},
			expectedScopes: []basic.Scope{
				{
					Actions:      []string{http.MethodPost},
					Environments: []string{"dev"},
					Teams:        []string{"team-gamma"},
					Roles:        []string{"admin"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register a dynamic scope parser for this test
			err := dynamic.RegisterScopeType[basic.Scope]("test-user",
				dynamic.WithUserHeaderParser(func(_ *gin.Context) []basic.Scope {
					return tt.scopes
				}),
			)
			if err != nil {
				t.Fatalf("failed to register scope type: %v", err)
			}

			cfg := &Config{
				Enabled:        true,
				UsernameHeader: "User",
				ScopeType:      "test-user",
			}
			cfg.ApplyDefaults()

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequestWithContext(t.Context(), "GET", "", nil)
			if tt.user != "" {
				req.Header.Set("User", tt.user)
			}
			c.Request = req

			Middleware(cfg)(c)
			trust := access.GetTrust(c.Request.Context())

			if tt.empty {
				if len(trust.Scopes) > 0 {
					t.Fatalf("expected no trust scopes but got %v", trust.Scopes)
				}
			} else {
				if len(trust.Scopes) != len(tt.expectedScopes) {
					t.Fatalf("trust.Scopes len = %d, expected %d", len(trust.Scopes), len(tt.expectedScopes))
				}
				for i := range tt.expectedScopes {
					expected := tt.expectedScopes[i]
					actual := trust.Scopes[i].(basic.Scope)
					if len(actual.Teams) != len(expected.Teams) || (len(actual.Teams) > 0 && actual.Teams[0] != expected.Teams[0]) {
						t.Errorf("%s: scope[%d].Teams = %v, want %v", tt.name, i, actual.Teams, expected.Teams)
					}
					if len(actual.Actions) != len(expected.Actions) || (len(actual.Actions) > 0 && actual.Actions[0] != expected.Actions[0]) {
						t.Errorf("%s: scope[%d].Actions = %v, want %v", tt.name, i, actual.Actions, expected.Actions)
					}
					if len(actual.Environments) != len(expected.Environments) || (len(actual.Environments) > 0 && actual.Environments[0] != expected.Environments[0]) {
						t.Errorf("%s: scope[%d].Environments = %v, want %v", tt.name, i, actual.Environments, expected.Environments)
					}
				}
			}
		})
	}
}
