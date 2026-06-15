package http

import (
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/store"
)

type ValidatableNameable interface {
	config.Validatable
	store.Nameable
}
