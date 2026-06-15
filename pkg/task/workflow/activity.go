package workflow

import (
	"context"
	"strings"
)

// Activity describes a named task that takes runs with a state of 'any' type. It is also possible to satisfy this
// interface with a function via the AnonActivity struct.
type Activity[T any] interface {
	Name() string
	Run(ctx context.Context, state T) error
}

type Chain[T any] []Activity[T]

// NewChain creates a chain of activities
func NewChain[T any](acts ...Activity[T]) Chain[T] {
	chain := Chain[T]{}
	chain = append(chain, acts...)
	return chain
}

// Weave builds a new chain that adds 'acts' after every entry in the previous chain. E.g. given
// a chain as [a,b,c,d] with 'acts' as 'e' the result would be [a,e,b,e,c,e,d,e]. Can be used to
// define post-tasks for every activity, e.g. a database commit.
func (c Chain[T]) Weave(acts ...Activity[T]) Chain[T] {
	var chain []Activity[T]
	for _, item := range c {
		chain = append(chain, item)
		chain = append(chain, acts...)
	}
	return chain
}

// Describe returns a comma separated list of all activity names
func (c Chain[T]) Describe() string {
	var names []string
	for _, item := range c {
		names = append(names, item.Name())
	}
	return strings.Join(names, ", ")
}

var _ Activity[int] = &AnonActivity[int]{}

// AnonActivity can be used to satisfy the Activity interface without defining a new struct type.
type AnonActivity[T any] struct {
	name string
	run  func(ctx context.Context, state T) error
}

func (a AnonActivity[T]) Name() string {
	return a.name
}

func (a AnonActivity[T]) Run(ctx context.Context, state T) error {
	return a.run(ctx, state)
}

func NewActivity[T any](name string, run func(ctx context.Context, state T) error) Activity[T] {
	return &AnonActivity[T]{name, run} // Immutable
}
