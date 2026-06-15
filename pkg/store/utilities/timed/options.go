package timed

import (
	"context"

	"github.com/ing-bank/golibs/pkg/store"
)

// DoFunc is called during Run before expired entries are deleted.
type DoFunc func(context.Context) error

// WithFunc sets a function to be called during Run before expired entries are deleted.
var WithFunc, matchWithFunc = store.OptionBuilder[DoFunc]()
