package access

import (
	"fmt"
	"testing"

	scope2 "github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

func Fuzz_IsAllowed(f *testing.F) {
	// Provide a seed corpus
	f.Add("POST", "dev", "TEAM1", "user")
	f.Add("DELETE", "tst", "TEAM2", "user")
	f.Add("GET", "prd", "TEAM1", "admin")
	f.Add("DELETE", "prd", "TEAM2", "admin")
	f.Add("PUT", "dev", "TEAM3", "user")
	f.Add("PATCH", "tst", "TEAM4", "user")
	f.Add("GET", "acc", "TEAM1", "admin")
	f.Add("POST", "dev", "TEAM2", "user")
	f.Add("GET", "acc", "TEAM3", "admin")
	f.Add("PATCH", "prd", "TEAM4", "admin")

	f.Fuzz(func(t *testing.T, action, environment, purpose, role string) {
		actions := []string{action}
		environments := []string{environment}
		purposes := []string{purpose}
		roles := []string{role}
		scope := basic.Scope{actions, environments, purposes, roles}
		fmt.Println(scope)
		sar := &SubjectAccessReview{Spec: scope, Status: Status{Account: Account{Scopes: []scope2.Scope{scope}}}}

		if reason, ok := reviewSAR(sar); !ok {
			t.Errorf("Sar not allowed %+v: %s", sar, reason)
		}
	})
}

func TestReviewScope(t *testing.T) {
	scopes := []scope2.Scope{
		&basic.Scope{
			Actions:      []string{"POST", "PUT", "PATCH", "DELETE", "GET"},
			Environments: []string{"dev", "tst"},
			Teams:        []string{"TEAM1", "TEAM2"},
			Roles:        []string{"user"},
		},
		&basic.Scope{
			Actions:      []string{"GET"},
			Environments: []string{"acc", "prd"},
			Teams:        []string{"TEAM1", "TEAM2"},
			Roles:        []string{"admin"},
		},
	}

	for _, test := range []basic.Scope{
		{Actions: []string{"GET"}, Environments: []string{"prd"}, Teams: []string{"TEAM1"}, Roles: []string{}},
		{Actions: []string{"DELETE"}, Environments: []string{"dev"}, Teams: []string{"TEAM2"}, Roles: []string{}},
		{Actions: []string{"GET"}, Environments: []string{"acc", "prd"}, Teams: []string{"TEAM1", "TEAM2"}, Roles: []string{}},
		{Actions: []string{"POST", "PUT", "PATCH", "DELETE", "GET"}, Environments: []string{"dev", "tst"}, Teams: []string{"TEAM1", "TEAM2"}, Roles: []string{}},
	} {

		if !MatchScope(scopes, test) {
			t.Errorf("failed to find scope %+v", test)
		}
	}
}

func Test_isAllowed(t *testing.T) {
	tests := []struct {
		name string
		spec scope2.Scope
		acc  []scope2.Scope
		want bool
	}{
		{
			name: "Default case",
			want: true,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default deny case",
			want: false,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM2"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default multi scope case",
			want: true,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"DELETE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default multi scope request case",
			want: true,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"acc"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default wildcard case",
			want: true,
			spec: &basic.Scope{
				Teams:        []string{"TEAM1", "TEAM5"},
				Environments: []string{"dev", "tst"},
				Actions:      []string{"POST", "PUT", "PATCH", "DELETE", "GET"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"dev", "tst"},
					Actions:      []string{"*"},
					Roles:        []string{},
				},
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"acc", "prd"},
					Actions:      []string{"GET"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default wildcard case with superfluous actions",
			want: true,
			spec: &basic.Scope{
				Teams:        []string{"TEAM1", "TEAM5"},
				Environments: []string{"dev", "tst"},
				Actions:      []string{"POST", "PUT", "PATCH", "DELETE", "GET"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"dev", "tst"},
					Actions:      []string{"DELETE"},
					Roles:        []string{},
				},
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"dev", "tst"},
					Actions:      []string{"*"},
					Roles:        []string{},
				},
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"dev", "tst"},
					Actions:      []string{"DELETE"},
					Roles:        []string{},
				},
				&basic.Scope{
					Teams:        []string{"TEAM1", "TEAM5"},
					Environments: []string{"acc", "prd"},
					Actions:      []string{"GET"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default allow wildcard case",
			want: true,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"prd"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"acc"},
					Teams:        []string{"*"},
					Roles:        []string{},
				},
			},
		},
		{
			name: "Default deny wildcard case",
			want: false,
			spec: &basic.Scope{
				Actions:      []string{"CREATE"},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"TEAM1"},
				Roles:        []string{},
			},
			acc: []scope2.Scope{
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"acc"},
					Teams:        []string{"TEAM1"},
					Roles:        []string{},
				},
				&basic.Scope{
					Actions:      []string{"CREATE"},
					Environments: []string{"dev"},
					Teams:        []string{"*"},
					Roles:        []string{},
				},
			},
		},
	}
	for _, tt := range tests {
		sar := SubjectAccessReview{
			Spec:   tt.spec,
			Status: Status{Account: Account{Scopes: tt.acc}},
		}

		t.Run(tt.name, func(t *testing.T) {
			if _, got := reviewSAR(&sar); got != tt.want {
				t.Errorf("reviewSAR() = %v, want %v", got, tt.want)
			}
		})
	}
}
