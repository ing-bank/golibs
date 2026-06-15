// Package authorization provides middleware for extracting and validating authorization information.
//
// Architecture:
// This package is part of a two-layer authorization system:
//
//  1. Authenticating Gateway (Frontend): An external authenticating gateway (e.g., reverse proxy,
//     API gateway) sits in front of the server and performs authentication. Upon successful
//     authentication, it sets protected headers on the request containing authorization scopes
//     and account information. This component is not part of these libraries and should be handled
//     externally.
//
//  2. Authorization Middleware (Backend): This middleware reads those protected headers and
//     extracts the Account information, storing it in the request context for later use.
//
// Workflow:
//
//		Client Request
//		    ↓
//		Authenticating Gateway (verifies credentials)
//		    ↓ (sets protected headers, calls backend)
//	 Backend
//		    ↓
//		Authorization Middleware (reads headers)
//		    ↓ (stores Account in context)
//		Application Handler (uses context Account for access control)
//
// Usage:
//
// The middleware reads protected headers set by the authenticating gateway and extracts
// authorization information:
//
//	// Middleware reads headers and sets Account in context
//	authMiddleware := authorization.NewMiddleware(...)
//	router.Use(authMiddleware)
//
//	// In handlers, retrieve the Account from context
//	handler := func(c *gin.Context) {
//		account := access.GetTrustFromGinContext(c)
//		// Use account.Scopes for access control via access.NewSubjectAccessReview
//	}
//
// Integration with Access Control:
//
// After this middleware extracts the Account and stores it in the request context, the
// access package can be used to perform authorization checks:
//
//	import "github.com/ing-bank/golibs/pkg/access"
//
//	// In handler
//	requestedScope := &access.BasicScope{
//		Actions:      []string{"GET"},
//		Environments: []string{"production"},
//		Teams:        []string{"TEAM1"},
//		Roles:        []string{"user"},
//	}
//	sar, err := access.NewSubjectAccessReview(c, requestedScope)
//	if !sar.Status.Allowed {
//		c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
//		return
//	}
//
// Protected Headers:
// The authenticating gateway is expected to set standard or custom headers containing:
//   - Account name/user ID
//   - Authorization scopes (teams, environments, roles, actions)
//   - Trust level (what authentication method was used)
//
// These headers are then parsed by this middleware and converted to Account objects
// stored in the request context.
package authorization
