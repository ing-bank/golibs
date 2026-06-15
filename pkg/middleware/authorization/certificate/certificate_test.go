package certificate

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

// ...existing code...

func TestMTLSMiddleware(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Enabled: true,
		Certificates: []Certificate{
			{
				CNs: []string{"example.com"},
				Scopes: []scope.Scope{
					basic.Scope{
						Actions:      []string{scope.Wildcard},
						Environments: []string{scope.Wildcard},
						Teams:        []string{"team-admin"},
						Roles:        []string{"admin"},
					},
				},
			},
			{
				CNs: []string{"test.com"},
				Scopes: []scope.Scope{
					basic.Scope{
						Actions:      []string{"GET"},
						Environments: []string{"dev"},
						Teams:        []string{"team-read"},
						Roles:        []string{"user"},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		cns           []string
		expectedScope basic.Scope
		expectAuth    bool
	}{
		{
			name: "Trusted host with wildcard access",
			cns:  []string{"example.com"},
			expectedScope: basic.Scope{
				Actions:      []string{scope.Wildcard},
				Environments: []string{scope.Wildcard},
				Teams:        []string{"team-admin"},
				Roles:        []string{"admin"},
			},
			expectAuth: true,
		},
		{
			name: "Trusted host with specific access",
			cns:  []string{"test.com"},
			expectedScope: basic.Scope{
				Actions:      []string{"GET"},
				Environments: []string{"dev"},
				Teams:        []string{"team-read"},
				Roles:        []string{"user"},
			},
			expectAuth: true,
		},
		{
			name:       "Untrusted host",
			cns:        []string{"untrusted.com"},
			expectAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req, _ := http.NewRequestWithContext(t.Context(), "GET", "", nil)
			req.TLS = &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{
					{Subject: pkix.Name{CommonName: tt.cns[0]}},
				},
			}
			c.Request = req

			Middleware(cfg)(c)
			trust := access.GetTrust(c.Request.Context())

			if tt.expectAuth {
				if len(trust.Scopes) != 1 {
					t.Fatalf("trust.Scopes = %d, expected 1", len(trust.Scopes))
				}

				actual := trust.Scopes[0].(basic.Scope)
				if actual.Teams[0] != tt.expectedScope.Teams[0] {
					t.Errorf("%s: Teams = %v, want %v", tt.name, actual.Teams, tt.expectedScope.Teams)
				}
				if actual.Roles[0] != tt.expectedScope.Roles[0] {
					t.Errorf("%s: Roles = %v, want %v", tt.name, actual.Roles, tt.expectedScope.Roles)
				}
				if actual.Environments[0] != tt.expectedScope.Environments[0] {
					t.Errorf("%s: Environments = %v, want %v", tt.name, actual.Environments, tt.expectedScope.Environments)
				}
				if actual.Actions[0] != tt.expectedScope.Actions[0] {
					t.Errorf("%s: Actions = %v, want %v", tt.name, actual.Actions, tt.expectedScope.Actions)
				}
			} else {
				if len(trust.Scopes) > 0 {
					t.Fatalf("expected no trust scopes but got %v", trust.Scopes)
				}
			}
		})
	}
}
