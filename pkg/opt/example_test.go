package opt

import "fmt"

func Greet(optionalGreeting ...string) {
	greeting := Opt("hello", optionalGreeting)
	fmt.Println(greeting)
}

func ExampleOpt() {
	Greet()              // Uses default 'hello'
	Greet("hi")          // Overrides default
	Greet("hi", "hello") // Overrides default, 2nd argument is ignored

	// Output:
	// hello
	// hi
	// hi
}
