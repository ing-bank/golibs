package store

import (
	"context"
	"errors"
	"fmt"
	goslices "slices"
	"strconv"

	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/slices"
	"k8s.io/apimachinery/pkg/labels"
)

// ErrUnsupportedOption is returned when an option is not supported by the handler or cannot be deserialized.
var ErrUnsupportedOption = errors.New("unsupported option")

// Option can be passed to any request. The type of the option is used to identify it, so options MUST be unique by type.
// The default types (int, string, etc), should be wrapped in a custom type to avoid collisions. Handler functions can
// match the option type and handle custom logic.
type Option any

// OptionsParser is a function that parses query parameters into store options.
type OptionsParser func(ctx context.Context, query map[string]string) ([]Option, error)

// MatchOptionConfig configures the behavior of option matching functions.
type MatchOptionConfig struct {
	SkipConsume bool // If true, the matched option is not removed from the slice
}

// MatchOption searches the given options slice for an option of type T. If found, it returns the option and true.
// If the option is found and SkipConsume is false, the option is removed from the slice to avoid re-processing it.
func MatchOption[T any](opts *[]Option, optCfg ...MatchOptionConfig) (T, bool) {
	cfg := opt.Opt(MatchOptionConfig{}, optCfg)
	var zero T
	for i, option := range *opts {
		// Base case
		if v, ok := option.(T); ok {
			if !cfg.SkipConsume {
				// Remove the matched option from the slice to avoid re-processing it
				slices.RemoveIndex(opts, i)
			}
			return v, true
		}

		// Also dive into option.(T) == []Option and iterate those
		// Usually caused by passing store wrapper options incorrectly, but is a nice utility regardless
		if v, ok := option.([]Option); ok {
			if nested, found := MatchOption[T](&v); found {
				(*opts)[i] = v
				if !cfg.SkipConsume && len(v) == 0 {
					slices.RemoveIndex(opts, i)
				}
				return nested, true
			}
		}
	}
	return zero, false
}

// MatchOptionBy searches the given options slice for an option of type T that satisfies the predicate function.
// This is useful when multiple options of the same type exist but need to be distinguished by their fields (e.g., by Key).
// If found, returns the option and true. If SkipConsume is false (default), the matched option is removed from the slice.
// Also recursively searches nested []Option slices.
func MatchOptionBy[T any](opts *[]Option, predicate func(T) bool, optCfg ...MatchOptionConfig) (T, bool) {
	cfg := opt.Opt(MatchOptionConfig{}, optCfg)
	var zero T
	for i, option := range *opts {
		// Base case
		if v, ok := option.(T); ok {
			if predicate(v) {
				if !cfg.SkipConsume {
					// Remove the matched option from the slice to avoid re-processing it
					slices.RemoveIndex(opts, i)
				}
				return v, true
			}
		}

		// Also dive into option.(T) == []Option and iterate those
		if v, ok := option.([]Option); ok {
			if nested, found := MatchOptionBy[T](&v, predicate); found {
				(*opts)[i] = v
				if !cfg.SkipConsume && len(v) == 0 {
					slices.RemoveIndex(opts, i)
				}
				return nested, true
			}
		}
	}
	return zero, false
}

// CheckOptionsExhausted returns an error if there are any options remaining in the slice.
// This is useful to ensure all options were consumed/handled by the server and detect unsupported options.
func CheckOptionsExhausted(opts []Option) error {
	if len(opts) > 0 {
		var notNil bool
		for _, option := range opts {
			// check if any option is not nil
			if option == nil {
				continue
			}
			// if it's a nested []Option slice, check if it contains any non-nil elements
			v, ok := option.([]Option)
			if !ok { // non-slice option that is not nil
				notNil = true
				break
			}
			if goslices.ContainsFunc(v, func(o Option) bool {
				return o != nil
			}) {
				notNil = true
				break
			}
		}
		if !notNil {
			return nil
		}
		types := slices.Transform(opts, func(item Option) string {
			return fmt.Sprintf("%T", item)
		})
		return fmt.Errorf("%w: %s", ErrUnsupportedOption, types)
	}
	return nil
}

// OptionBuilder creates a simple option builder and matcher for a unique type T.
// Returns a constructor function that wraps a value as an option, and a matcher function that finds the option by type.
// Use this for options that are distinguished by their unique type rather than by key fields.
func OptionBuilder[T any]() (func(T) T, func(*[]Option) (T, bool)) {
	return func(value T) T {
			return value
		}, func(opts *[]Option) (T, bool) {
			return MatchOption[T](opts)
		}
}

// BoolOption represents a boolean option with a key identifier.
// Multiple BoolOption instances can coexist by having different Key values.
type BoolOption struct {
	Key   string
	Value bool
}

// String returns the string representation of the boolean value.
func (d BoolOption) String() string {
	return d.Key + "=" + strconv.FormatBool(d.Value)
}

// Serialize returns the key and value as strings for transmission over the network.
func (d BoolOption) Serialize() (string, string) {
	if d.Value {
		return d.Key, "true"
	}
	return d.Key, "false"
}

// SerializableBoolOptionBuilder creates option builder and matcher for boolean-based options.
// Uses key-based matching to allow multiple BoolOption instances to coexist in the same options slice.
// Returns a constructor function that accepts a bool value, and a matcher function that matches by key.
// The option is automatically registered for serialization/deserialization.
func SerializableBoolOptionBuilder(key string) (func(bool) BoolOption, func(*[]Option) (bool, bool)) {
	// Register with the serialization registry
	registry[key] = func(val string) (Option, error) {
		return BoolOption{Key: key, Value: val == "true"}, nil
	}

	// Wrap the withOption to accept bool instead of BoolOption
	with := func(b bool) BoolOption {
		return BoolOption{Key: key, Value: b}
	}

	// Wrap the matchOption to return bool instead of BoolOption, matching by Key
	match := func(opts *[]Option) (bool, bool) {
		boolOpt, found := MatchOptionBy[BoolOption](opts, func(opt BoolOption) bool {
			return opt.Key == key
		})
		if !found {
			return false, false
		}
		return boolOpt.Value, true
	}

	return with, match
}

// StringOption represents a string option with a key identifier.
// Multiple StringOption instances can coexist by having different Key values.
type StringOption struct {
	Key   string
	Value string
}

// String returns the string value.
func (d StringOption) String() string {
	return d.Key + "=" + d.Value
}

// Serialize returns the key and value as strings for transmission over the network.
func (d StringOption) Serialize() (string, string) {
	return d.Key, d.Value
}

// SerializableStringOptionBuilder creates option builder and matcher for string-based options.
// Uses key-based matching to allow multiple StringOption instances to coexist in the same options slice.
// Returns a constructor function that accepts a string value, and a matcher function that matches by key.
// The option is automatically registered for serialization/deserialization.
func SerializableStringOptionBuilder(key string) (func(string) StringOption, func(*[]Option) (string, bool)) {
	// Register with the serialization registry
	registry[key] = func(val string) (Option, error) {
		return StringOption{Key: key, Value: val}, nil
	}

	// Wrap the withOption to accept string instead of StringOption
	with := func(s string) StringOption {
		return StringOption{Key: key, Value: s}
	}

	// Wrap the matchOption to return string instead of StringOption, matching by Key
	match := func(opts *[]Option) (string, bool) {
		strOpt, found := MatchOptionBy[StringOption](opts, func(opt StringOption) bool {
			return opt.Key == key
		})
		if !found {
			return "", false
		}
		return strOpt.Value, true
	}

	return with, match
}

// MapStringOfStrings represents a map[string]string option with a key identifier.
// Multiple MapStringOfStrings instances can coexist by having different Key values.
// Commonly used for labels, annotations, or other key-value pair configurations.
type MapStringOfStrings struct {
	Key   string
	Value map[string]string
}

// String returns the string representation of the map using Kubernetes label format.
func (m MapStringOfStrings) String() string {
	return labels.Set(m.Value).String()
}

// Serialize returns the key and value as strings for transmission over the network.
// The map is serialized using Kubernetes label selector format.
func (m MapStringOfStrings) Serialize() (string, string) {
	return m.Key, labels.Set(m.Value).String()
}

// SerializableMapStringOfStringsBuilder creates option builder and matcher for map[string]string-based options.
// Uses key-based matching to allow multiple MapStringOfStrings instances to coexist in the same options slice.
// Returns a constructor function that accepts a map[string]string value, and a matcher function that matches by key.
// The option is automatically registered for serialization/deserialization using Kubernetes label format.
func SerializableMapStringOfStringsBuilder(key string) (func(map[string]string) MapStringOfStrings, func(*[]Option) (map[string]string, bool)) {
	// Register with the serialization registry
	registry[key] = func(val string) (Option, error) {
		// Parse the label selector string back to map[string]string
		set, err := labels.ConvertSelectorToLabelsMap(val)
		if err != nil {
			return nil, err
		}
		return MapStringOfStrings{Key: key, Value: set}, nil
	}

	// Wrap the withOption to accept map[string]string instead of MapStringOfStrings
	with := func(m map[string]string) MapStringOfStrings {
		return MapStringOfStrings{Key: key, Value: m}
	}

	// Wrap the matchOption to return map[string]string instead of MapStringOfStrings, matching by Key
	match := func(opts *[]Option) (map[string]string, bool) {
		mapOpt, found := MatchOptionBy[MapStringOfStrings](opts, func(opt MapStringOfStrings) bool {
			return opt.Key == key
		})
		if !found {
			return nil, false
		}
		return mapOpt.Value, true
	}

	return with, match
}

// WithDryRun creates a dry-run option. MatchDryRun retrieves the dry-run value from options.
// When enabled, operations should simulate execution without making actual changes.
var WithDryRun, MatchDryRun = SerializableBoolOptionBuilder("dryRun")

// DryRun is syntax sugar for enabling a dry run
var DryRun = WithDryRun(true)

// WithPrefix creates a prefix filter option. MatchPrefix retrieves the prefix value from options.
// This is a filter for list operations to match only keys with the given prefix, allowing filtering on
// backend side and supporting multiple datatypes (with different prefixes).
var WithPrefix, MatchPrefix = SerializableStringOptionBuilder("prefix")

// WithNoCache creates a no-cache option. MatchNoCache retrieves the no-cache value from options.
// When enabled, operations should bypass caching mechanisms.
var WithNoCache, MatchNoCache = SerializableBoolOptionBuilder("noCache")

// NoCache is a pre-configured BoolOption for disabling caching.
// Equivalent to WithNoCache(true).
var NoCache = WithNoCache(true) //  BoolOption{Key: "noCache", Value: true}

// WithListKeyOnly creates a keys-only option. MatchListKeyOnly retrieves the keys-only value from options.
// When enabled, list operations should return only keys without full object data.
var WithListKeyOnly, MatchListKeyOnly = SerializableBoolOptionBuilder("keysOnly")

// ListKeysOnly is a pre-configured BoolOption for list operations that only need keys.
// When used, list operations should return only keys without full object data.
var ListKeysOnly = WithListKeyOnly(true) // BoolOption{Key: "keysOnly", Value: true}

// TODO: To avoid the client passing unused options we should error if an option is not matched/used by the server
//       we can do that by removing an option from the slice when matched, and at the end of the handler check
//       if any options remain.
