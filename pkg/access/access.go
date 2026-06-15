// Package access does Subject Access Reviews based on Trust set in the request contexts.
// For example, consider authentication middlewares such as user, NPA, JWT, and mTLS. Each
// of these middlewares set the trust context via the access.SetTrust method when they are
// active (e.g. NPA will only set the trust when it actually finds a valid NPA). The access
// package then uses said trust (via access.GetTrust) and compare it with the requested access
// via a SubjectAccessReview, where the status shows if the given trust satisfies the requested access.
//
// TODO: We're still trying to improve the usability of this package to make it easier to adapt to different scopes.
//
// Example usage:
//
//	 // Authorization Middleware configured elsewhere
//	 // Then, inside an HTTP handler that has `c *gin.Context`
//	 requestedScope := &BasicScope{
//		  Actions:      []string{"GET"},
//		  Environments: []string{"dev"},
//		  Teams:        []string{"team1"},
//		  Roles:        []string{"user"},
//		}
//		sar, err := NewSubjectAccessReview(ginCtx, requestedScope)
//	 if err != nil {
//	   // handle error
//	 }
//	 if !sar.Status.Allowed { // TODO: unauthorized }
package access

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
)

var AnyAction = []string{scope.Wildcard}

type SubjectAccessReview struct {
	Spec   scope.Scope `json:"spec"`   // Requested scope
	Status Status      `json:"status"` // Allowed access
}

type Status struct {
	Account Account `json:"account"`
	Allowed bool    `json:"allowed"`
	Reason  string  `json:"reason"`
}

type Account struct {
	Trust  TrustLevel    `json:"trust"`
	Name   string        `json:"name"`
	Scopes []scope.Scope `json:"scopes"`
}

func (a Account) String() string {
	return fmt.Sprintf("{%s %s %v}", a.Trust, a.Name, a.Scopes)
}

func (a Account) Validate() error {
	if a.Trust == "" {
		return fmt.Errorf("account trust level is required")
	}
	for _, s := range a.Scopes {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// NewSubjectAccessReview matches the requested scope against the scopes of the account in the trust context
// and returns a SubjectAccessReview with the result.
func NewSubjectAccessReview(c *gin.Context, scope scope.Scope) (*SubjectAccessReview, error) {
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create subject access review because scope is invalid: %w", err)
	}

	sar := &SubjectAccessReview{
		Spec: scope,
		Status: Status{
			Account: GetTrust(c.Request.Context()),
		},
	}

	reason, allowed := reviewSAR(sar)
	sar.Status.Allowed = allowed
	sar.Status.Reason = reason

	return sar, nil
}

// MatchScope checks whether requested is a subset of allowed. Requested may be spread over multiple
// allowed scopes. Wildcards are taken into account.
func MatchScope(allowed []scope.Scope, requested scope.Scope) bool {
	root := make(Node)

	for _, s := range allowed {
		for _, labels := range s.AsLabels() {
			root.Insert(labels)
		}
	}

	for _, labels := range requested.AsLabels() {
		if !root.Find(labels) {
			return false
		}
	}

	return true
}

func reviewSAR(sar *SubjectAccessReview) (string, bool) {
	allowed := MatchScope(sar.Status.Account.Scopes, sar.Spec)
	labels := sar.Spec.AsLabels()

	humanAllowed := "cannot"
	if allowed {
		humanAllowed = "can"
	}
	return fmt.Sprintf("%s %s %v", sar.Status.Account.Trust, humanAllowed, labels), allowed
}
