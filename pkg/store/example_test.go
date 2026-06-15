package store

import (
	"context"
	"fmt"
)

func Example() {
	// --- Define some custom options ---
	type RetryOption int
	var WithRetries, matchRetryOption = OptionBuilder[RetryOption]()

	// --- User sets some options ---
	myOptions := []Option{WithRetries(5)}

	// --- Extract the options, usually executed in the handler ---
	if retries, defined := matchRetryOption(&myOptions); defined {
		fmt.Println("Retries", retries)
	}

	// Output:
	// Retries 5
}

func ExampleOptionBuilder() {
	// --- Define some custom options ---
	type RetryOption int
	var WithRetries, matchRetryOption = OptionBuilder[RetryOption]()

	type FooOption int // Define another int to showcase conflict resolution by type
	var WithFoo, matchFooOption = OptionBuilder[FooOption]()
	_ = WithFoo // Not used

	type AdvancedConfig struct {
		Foo string
	}
	var WithAdvancedConfig = func(foo string) AdvancedConfig { // Custom constructor
		return AdvancedConfig{Foo: foo}
	}
	var matchAdvancedConfig = MatchOption[AdvancedConfig]

	// --- User sets some options ---
	myOptions := []Option{
		WithRetries(5),
		WithAdvancedConfig("bar"),
	}

	// --- Extract the options, usually executed in the handler ---
	if retries, defined := matchRetryOption(&myOptions); defined {
		fmt.Println("Retries", retries)
	}

	if retries, defined := matchFooOption(&myOptions); defined {
		fmt.Println("Foo", retries)
	}

	if advanced, defined := matchAdvancedConfig(&myOptions); defined {
		fmt.Println("Advanced", advanced)
	}

	// Output:
	// Retries 5
	// Advanced {bar}
}

func ExampleSerializableOptionBuilder() {
	// Serializable options are useful when options need to be passed over the network or stored.
	// They must implement a Serialize method and have a corresponding unserialize function.
	//  type TestSerializable string
	//
	//  func (t TestSerializable) Serialize() (string, string) {
	//	  return "test", string(t)
	//  }
	//  func unserializeTest(val string) (Option, error) {
	//	  return TestSerializable(val), nil
	//  }

	var WithTestSerializable, matchTestSerializable = SerializableOptionBuilder[TestSerializable]("test", unserializeTest)
	opts := []Option{WithTestSerializable("hello")}
	serialized, _ := SerializeOptions(opts)
	fmt.Println("Serialized:", serialized)

	unserialized, _ := UnserializeOptions(context.TODO(), serialized)
	val, _ := matchTestSerializable(&unserialized)
	fmt.Println("Unserialized:", val)

	// Output:
	// Serialized: map[test:hello]
	// Unserialized: hello
}
