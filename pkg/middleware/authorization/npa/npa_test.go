package npa

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

func TestMiddleware(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Enabled: true,
		Header:  "npa",
		AllowedNPAs: []AllowedNPA{
			{
				Name: "npa-1",
				Scopes: []scope.Scope{
					basic.Scope{
						Actions:      []string{scope.Wildcard},
						Environments: []string{"dev", "tst"},
						Teams:        []string{"team-alpha"},
						Roles:        []string{"user"},
					},
				},
			},
			{
				Name: "npa-2",
				Scopes: []scope.Scope{
					basic.Scope{
						Actions:      []string{http.MethodGet},
						Environments: []string{"acc", "prd"},
						Teams:        []string{"team-beta"},
						Roles:        []string{"user"},
					},
				},
			},
			{
				Name: "npa-3",
				Scopes: []scope.Scope{
					basic.Scope{
						Actions:      []string{scope.Wildcard},
						Environments: []string{"dev"},
						Teams:        []string{"team-gamma"},
						Roles:        []string{"admin"},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		// Given
		npa string

		// Want
		expectedScope basic.Scope
		empty         bool
	}{
		{
			name: "Legitimate npa-1",
			npa:  "npa-1",
			expectedScope: basic.Scope{
				Actions:      []string{scope.Wildcard},
				Environments: []string{"dev", "tst"},
				Teams:        []string{"team-alpha"},
				Roles:        []string{"user"},
			},
		},
		{
			name: "Case insensitive npa-1",
			npa:  "NPA-1",
			expectedScope: basic.Scope{
				Actions:      []string{scope.Wildcard},
				Environments: []string{"dev", "tst"},
				Teams:        []string{"team-alpha"},
				Roles:        []string{"user"},
			},
		},
		{
			name: "Legitimate npa-2",
			npa:  "npa-2",
			expectedScope: basic.Scope{
				Actions:      []string{http.MethodGet},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"team-beta"},
				Roles:        []string{"user"},
			},
		},
		{
			name: "Legitimate npa-3",
			npa:  "npa-3",
			expectedScope: basic.Scope{
				Actions:      []string{scope.Wildcard},
				Environments: []string{"dev"},
				Teams:        []string{"team-gamma"},
				Roles:        []string{"admin"},
			},
		},
		{
			name:  "Illegitimate npa",
			npa:   "user",
			empty: true,
		},
		{
			name:  "No npa header",
			empty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequestWithContext(t.Context(), "GET", "", nil)
			if tt.npa != "" {
				req.Header.Set("Npa", tt.npa)
			}
			c.Request = req

			Middleware(*cfg)(c)
			trust := access.GetTrust(c.Request.Context())

			if tt.empty {
				if len(trust.Scopes) > 0 {
					t.Fatalf("expected no trust scopes but got %v", trust.Scopes)
				}
			} else {
				if len(trust.Scopes) != 1 {
					t.Fatalf("trust.Scopes = %d, expected 1", len(trust.Scopes))
				}

				actual := trust.Scopes[0].(basic.Scope)
				if actual.Teams[0] != tt.expectedScope.Teams[0] {
					t.Errorf("%s: Teams = %v, want %v", tt.name, actual.Teams, tt.expectedScope.Teams)
				}
				if actual.Environments[0] != tt.expectedScope.Environments[0] {
					t.Errorf("%s: Environments = %v, want %v", tt.name, actual.Environments, tt.expectedScope.Environments)
				}
				if actual.Roles[0] != tt.expectedScope.Roles[0] {
					t.Errorf("%s: Roles = %v, want %v", tt.name, actual.Roles, tt.expectedScope.Roles)
				}
			}
		})
	}
}
