package patch

import "fmt"

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func Example() {
	bob := &Person{Name: "bob", Age: 18}

	merged, err := Merge(bob, map[string]any{"age": 19}) // Happy birthday, Bob
	if err != nil {
		panic(err)
	}

	fmt.Println(merged)

	// Output:
	// &{bob 19}
}
