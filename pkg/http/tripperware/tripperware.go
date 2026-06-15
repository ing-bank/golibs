package tripperware

import (
	"context"
	"net/http"

	"github.com/ing-bank/golibs/pkg/http/response"
)

// This is a fork from go-kit, just changing the variables in 'Endpoint' for our use cases
//  See https://github.com/go-kit/kit

// Endpoint is the fundamental building block of servers and clients.
// It represents a single RPC method.
type Endpoint func(ctx context.Context, request *http.Request) *response.Data

// Tripperware is a chainable behavior modifier for endpoints. (rename to middleware to import go-kit)
type Tripperware func(Endpoint) Endpoint

// Chain is a helper function for composing middlewares. Requests will
// traverse them in the order they're declared. That is, the first middleware
// is treated as the outermost middleware.
func Chain(outer Tripperware, others ...Tripperware) Tripperware {
	return func(next Endpoint) Endpoint {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}

		return outer(next)
	}
}

// DefaultTripperware provides a default tripperware chain.
// It includes retrying, logging, circuit breaking, rate limiting and metrics.
var DefaultTripperware = Chain(
	NewRetrier().Tripperware(),
	Logging(),
	NewBreaker().Tripperware(),
	NewRateLimiter().Tripperware(),
	Metrics(true),
)
