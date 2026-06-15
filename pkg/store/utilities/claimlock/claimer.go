package claimlock

import (
	"context"

	"github.com/ing-bank/golibs/pkg/store"
)

type Claimer[V any] interface {
	GetLocks(ctx context.Context) (store.ListItems[string, V], error)
	GetLockOwner(ctx context.Context, value V) (string, error)
	ClaimLock(ctx context.Context, key string, value V) error
}

var _ Claimer[any] = (*AnonymousClaimer[any])(nil)

type AnonymousClaimer[V any] struct {
	locks func(ctx context.Context) (store.ListItems[string, V], error)
	owner func(ctx context.Context, value V) (string, error)
	claim func(ctx context.Context, key string, value V) error
}

// NewAnonymousClaimer implements a Claimer by delegating the methods to the provided functions. This allows for
// flexible implementations of the Claimer interface without defining a new struct type for each implementation.
func NewAnonymousClaimer[V any](
	locks func(ctx context.Context) (store.ListItems[string, V], error),
	owner func(ctx context.Context, value V) (string, error),
	claim func(ctx context.Context, key string, value V) error,
) *AnonymousClaimer[V] {
	return &AnonymousClaimer[V]{
		locks: locks,
		owner: owner,
		claim: claim,
	}
}

func (c *AnonymousClaimer[V]) GetLocks(ctx context.Context) (store.ListItems[string, V], error) {
	return c.locks(ctx)
}

func (c *AnonymousClaimer[V]) GetLockOwner(ctx context.Context, value V) (string, error) {
	return c.owner(ctx, value)
}

func (c *AnonymousClaimer[V]) ClaimLock(ctx context.Context, key string, value V) error {
	return c.claim(ctx, key, value)
}
