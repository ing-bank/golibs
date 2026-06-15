package access

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
)

func ExampleNewSubjectAccessReview() {
	// Mimic a logged-in user account, scope for users is * in dev/tst but read-only in acc/prd
	// This step is usually done by authorization middleware
	user := &Account{
		Trust: "user",
		Name:  "foo",
		Scopes: []scope.Scope{
			&basic.Scope{
				Actions:      []string{scope.Wildcard},
				Environments: []string{"dev", "tst"},
				Teams:        []string{"team1"},
				Roles:        []string{"user"},
			},
			&basic.Scope{
				Actions:      []string{http.MethodGet},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"team1"},
				Roles:        []string{"user"},
			},
		},
	}

	// Fake authorization middleware
	mockedReq, _ := http.NewRequest(http.MethodGet, "", nil)
	ginCtx := &gin.Context{Request: mockedReq}
	if err := SetTrustInGinContext(ginCtx, user); err != nil {
		panic(err)
	}

	// Check scope that the user requests against what the user is allowed to do (stored in Gin context)
	requestedScope := &basic.Scope{
		Actions:      []string{"GET"},
		Environments: []string{"dev"},
		Teams:        []string{"team1"},
		Roles:        []string{"user"},
	}
	sar, err := NewSubjectAccessReview(ginCtx, requestedScope)
	if err != nil {
		panic(err)
	}
	fmt.Println(sar.Status.Allowed)
	fmt.Println(sar.Status.Reason)

	// Output:
	// true
	// user can [[team1 dev GET user]]
}

func ExampleMatchScope() {
	{
		// In this RBAC setup we do not define roles. We only use actions, environments and pCodes.
		allowed := []scope.Scope{
			&basic.Scope{
				Actions:      []string{"*"},
				Environments: []string{"dev", "tst"},
				Teams:        []string{"team1", "team2"},
			},
			&basic.Scope{
				Actions:      []string{"GET"},
				Environments: []string{"acc", "prd"},
				Teams:        []string{"team1", "team2"},
			},
		}

		for _, requested := range []basic.Scope{
			{Actions: []string{"DELETE"}, Environments: []string{"dev"}, Teams: []string{"team2"}}, // Allowed
			{Actions: []string{"GET"}, Environments: []string{"tst"}, Teams: []string{"team1"}},    // Allowed
			{Actions: []string{"GET"}, Environments: []string{"prd"}, Teams: []string{"team1"}},    // Allowed
			{Actions: []string{"DELETE"}, Environments: []string{"prd"}, Teams: []string{"team1"}}, // Not allowed
		} {
			fmt.Println(requested, MatchScope(allowed, requested))
		}
	}
	{ // This RBAC below achieves the same test results as the one above, just defined differently. We do add roles and also show an admin user as extra.
		allowed := []scope.Scope{
			&basic.Scope{
				Actions:      []string{"POST", "PUT", "PATCH", "DELETE"},
				Environments: []string{"dev", "tst"},
				Teams:        []string{"team1", "team2"},
				Roles:        []string{"user"},
			},
			&basic.Scope{
				Actions:      []string{"GET"},
				Environments: []string{"*"},
				Teams:        []string{"team1", "team2"},
				Roles:        []string{"user"},
			},
			&basic.Scope{
				Actions:      []string{"GET"}, // This is extra
				Environments: []string{"*"},
				Teams:        []string{"*"},
				Roles:        []string{"admin"},
			},
		}

		for _, requested := range []basic.Scope{
			{Actions: []string{"DELETE"}, Environments: []string{"dev"}, Teams: []string{"team2"}, Roles: []string{"user"}},  // Allowed
			{Actions: []string{"GET"}, Environments: []string{"tst"}, Teams: []string{"team1"}, Roles: []string{"user"}},     // Allowed
			{Actions: []string{"GET"}, Environments: []string{"prd"}, Teams: []string{"team1"}, Roles: []string{"user"}},     // Allowed
			{Actions: []string{"DELETE"}, Environments: []string{"prd"}, Teams: []string{"team1"}, Roles: []string{"user"}},  // Not allowed
			{Actions: []string{"DELETE"}, Environments: []string{"prd"}, Teams: []string{"team1"}, Roles: []string{"admin"}}, // Not allowed - Extra
			{Actions: []string{"GET"}, Environments: []string{"prd"}, Teams: []string{"team3"}, Roles: []string{"admin"}},    // Allowed - Extra
		} {
			fmt.Println(requested, MatchScope(allowed, requested))
		}
	}

	// Output:
	// {[DELETE] [dev] [team2] []} true
	// {[GET] [tst] [team1] []} true
	// {[GET] [prd] [team1] []} true
	// {[DELETE] [prd] [team1] []} false
	// {[DELETE] [dev] [team2] [user]} true
	// {[GET] [tst] [team1] [user]} true
	// {[GET] [prd] [team1] [user]} true
	// {[DELETE] [prd] [team1] [user]} false
	// {[DELETE] [prd] [team1] [admin]} false
	// {[GET] [prd] [team3] [admin]} true
}

func ExampleNode() {
	root := make(Node)

	root.Insert([]string{"Bob", "car", "read"})
	root.Insert([]string{"Bob", "car", "list"})
	root.Insert([]string{"Janice", "car", "*"})
	root.Insert([]string{"Janice", "car", "read"}) // Superfluous, Janice can already do everything with car
	root.Insert([]string{"Dave", "*", "read"})
	root.Insert([]string{"Dave", "*", "list"})
	root.Insert([]string{"Dave", "car", "drive"})

	root.Print()

	for _, rbac := range [][]string{
		{"Bob", "car", "read"},     // true
		{"Bob", "car", "drive"},    // false
		{"Janice", "car", "read"},  // true
		{"Janice", "car", "drive"}, // true
		{"Dave", "car", "read"},    // true
		{"Dave", "car", "drive"},   // true
	} {
		fmt.Println("Can", rbac, "=", root.Find(rbac))
	}

	// Output:
	// --- Node ---
	// Bob
	// -car
	// --list
	// --read
	// Dave
	// -*
	// --list
	// --read
	// -car
	// --drive
	// Janice
	// -car
	// --*
	// Can [Bob car read] = true
	// Can [Bob car drive] = false
	// Can [Janice car read] = true
	// Can [Janice car drive] = true
	// Can [Dave car read] = true
	// Can [Dave car drive] = true
}
