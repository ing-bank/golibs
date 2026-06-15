package store

import (
	"context"
	"fmt"
	"net/url"
)

// TODO: these functions should probably be moved to http package, as they can be used to parse gin query params
//       for strongly typed options quite nicely. Then the store/http package can just import that http utility.

type Serializable interface {
	Serialize() (string, string) // key, value
}

var ErrNotSerializable = fmt.Errorf("%w: option is not serializable", ErrUnsupportedOption)

type SerializedOptions map[string]string

type Unserialize = func(val string) (Option, error)

var registry = make(map[string]Unserialize)

// SerializableOptionBuilder creates an option builder and matcher for a serializable option type. In addition, it registers
// the option type with a key and unserialize function, so that it can be serialized and unserialized from query parameters.
// The key must be unique across all option types. If a key is registered more than once, the last registration wins.
// This allows the use of the UnserializeOptions functions
func SerializableOptionBuilder[T Serializable](key string, deserialize Unserialize) (func(T) T, func(*[]Option) (T, bool)) {
	registry[key] = deserialize
	return func(value T) T {
			return value
		}, func(opts *[]Option) (T, bool) {
			return MatchOption[T](opts) // TODO: MatchOptionConfig
		}
}

func SerializeOptions(opts []Option) (SerializedOptions, error) {
	serializedOptions := make(map[string]string)

	for _, opt := range opts {
		serializer, ok := opt.(Serializable)
		if !ok {
			return nil, ErrNotSerializable
		}

		key, value := serializer.Serialize()
		serializedOptions[key] = value
	}

	return serializedOptions, nil
}

var _ OptionsParser = UnserializeOptions

// UnserializeOptions from query params for options that are known in the registry. Unknown options are ignored.
// Registry can be populated with SerializableOptionBuilder.
func UnserializeOptions(_ context.Context, params map[string]string) ([]Option, error) {
	var opts []Option
	for key, v := range params {
		deserializer, ok := registry[key]
		if !ok {
			return nil, fmt.Errorf("%w: cannot match deserializer for key '%s'", ErrUnsupportedOption, key)
		}

		opt, err := deserializer(v)
		if err != nil {
			// TODO: not entirely sure if this warrants an UnsupportedOption err. It is supported, just can't be deserialized
			return nil, fmt.Errorf("%w: cannot deserialize key '%s': %w", ErrUnsupportedOption, key, err)
		}

		opts = append(opts, opt)
	}

	return opts, nil
}

func (s SerializedOptions) AsQuery() string {
	values := url.Values{}
	for k, v := range s {
		values.Set(k, v)
	}
	return values.Encode()
}
