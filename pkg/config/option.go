package config

import "github.com/ing-bank/golibs/pkg/slices"

type Option[T any] interface {
	ApplyOpt(T) error
}

// NewOptions casts []func(T) error as []Option[T]
func NewOptions[T any](opts ...Opt[T]) []Option[T] {
	return slices.Transform(opts, func(o Opt[T]) Option[T] { return o })
}

type Opt[T any] func(T) error

func (o Opt[T]) ApplyOpt(base T) error {
	return o(base)
}

func ApplyOpts[T any](base T, opts ...Option[T]) error {
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o.ApplyOpt(base); err != nil {
			return err
		}
	}
	return nil
}
