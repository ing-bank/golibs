package scope

import (
	"encoding/json"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/slices"
)

// Wildcard implies access to all scoped labels of the same depth of the Node tree
const Wildcard string = "*"

// A Scope presents a set of permissions
type Scope interface {
	config.Validatable // Validate() error

	// AsLabels converts the scope into all possible combinations (Cartesian product).
	// For example a scope such as {"action": ["GET", "DELETE"], "resource": ["A", "B"]} would be converted to:
	// [["GET", "A"], ["GET", "B"], ["DELETE", "A"], ["DELETE", "B"]]
	// To match everything, use "*", e.g. ["GET", "*"].
	AsLabels() [][]string
}

func FromJSON[T Scope](raw json.RawMessage) (Scope, error) {
	var t T
	return t, json.Unmarshal(raw, &t)
}

// AsScopeSlice casts a slice to a slice of Scope
func AsScopeSlice[S ~[]E, E Scope](items S) []Scope {
	if items == nil {
		return nil
	}
	return slices.Transform(items, func(item E) Scope {
		return item
	})
}
