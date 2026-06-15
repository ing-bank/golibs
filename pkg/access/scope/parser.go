package scope

import (
	"fmt"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
)

type Parser interface {
	store.Nameable
}

// ConfigParser overloads the parser interface by introducing ParseConfigScope

// Registry keeps record of all known scope parsers
var registry = map[string]Parser{}

func RegisterParser(parser Parser) {
	registry[parser.GetName()] = parser
}

// MatchCustomParser finds a parser by name and tries to cast it to the required type.
// This allows Parsers to overload the Parser interface.
func MatchCustomParser[T any](name string) (T, error) {
	var zero T
	parser, err := MatchScopeParser(name)
	if err != nil {
		return zero, err
	}

	conv, ok := parser.(T)
	if !ok {
		return zero, fmt.Errorf("expected type %T but got %T", zero, parser)
	}
	return conv, nil
}

// MatchScopeParser finds a parser by name from the registry
func MatchScopeParser(name string) (Parser, error) {
	parser, ok := registry[name]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return parser, nil
}
