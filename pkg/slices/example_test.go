package slices

import (
	"fmt"
	"math"
)

func ExampleUnique() {
	{
		// Simple Unique example with integers
		in := []int{1, 2, 3, 3, 2, 1, 3, 2}
		out := Unique(in)

		fmt.Println(in)  // [1 2 3 3 2 1 3 2] - Input slice not modified
		fmt.Println(out) // [1 2 3] - Unique entries, in order of first found in input slice
	}
	{
		// Unique example with custom data
		type Example struct {
			Data string
		}

		in := []Example{{"a"}, {"b"}, {"c"}, {"a"}, {"c"}}
		out := Unique(in)

		fmt.Println(in)  // [{a} {b} {c} {a} {c}] - Input slice not modified
		fmt.Println(out) // [{a} {b} {c}] - Unique entries, in order of first found in input slice
	}
	// Output:
	// [1 2 3 3 2 1 3 2]
	// [1 2 3]
	// [{a} {b} {c} {a} {c}]
	// [{a} {b} {c}]
}

func ExampleUniqueCmp() {
	type Example struct {
		Data string
	}

	// Create an array of pointers, using Unique(in) will not work because pointer values are always unique!
	// So, we have to provide our own serializer
	in := []any{&Example{"a"}, &Example{"b"}, &Example{"c"}, &Example{"a"}, &Example{"c"}}

	out := UniqueCmp(in, func(e any) string {
		return e.(*Example).Data
	})

	// Input slice not modified
	for _, i := range in {
		fmt.Printf("%s", i.(*Example).Data)
	}
	fmt.Println()

	// Unique entries, in order of first found in input slice
	for _, i := range out {
		fmt.Printf("%s", i.(*Example).Data)
	}

	// Output:
	// abcac
	// abc
}

func ExampleTransform() {
	in := []float64{1.34310, 231.7434}

	// This example converts floats to integers with rounding
	out := Transform(in, func(item float64) int {
		return int(math.Round(item))
	})

	fmt.Println(out)
	// Output:
	// [1 232]
}

func ExampleFlatMap() {
	type Group struct {
		Name   string
		Values []int
	}

	// Create some groups with Values
	groups := []Group{
		{"A", []int{1, 2}},
		{"B", []int{3, 4}},
	}
	// Flatten all Values from all groups into a single slice
	result := FlatMap(groups, func(g Group) []int {
		return g.Values
	})
	fmt.Println(result)
	// Output: [1 2 3 4]
}

func ExampleMatchAny() {
	// Example with int slices
	a := []int{1, 2, 3}
	b := []int{3, 4, 5}
	if v, ok := MatchAny(a, b); ok {
		fmt.Println("Found:", v)
	} else {
		fmt.Println("No match")
	}

	// Example with string slices
	s1 := []string{"foo", "bar"}
	s2 := []string{"baz", "bar"}
	if v, ok := MatchAny(s1, s2); ok {
		fmt.Println("Found:", v)
	} else {
		fmt.Println("No match")
	}

	// Output:
	// Found: 3
	// Found: bar
}

func ExampleMap() {
	type Data struct {
		Name string
		Age  int
	}

	people := []Data{{"Bob", 1}, {"Jim", 2}}
	lookup := Map(people, func(item Data) string { return item.Name }) // Maps Name to Person, map[string]Data

	fmt.Println(lookup["Jim"])
	// Output:
	// {Jim 2}
}

func ExampleCount() {
	items := []int{1, 2, 2, 3, 3, 3, 4, 4, 4, 4}

	counts := Count(items)
	fmt.Println(counts)

	// Output:
	// map[1:1 2:2 3:3 4:4]
}

func ExampleIsSubset() {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3, 4, 5}

	fmt.Println(IsSubset(a, b))
	fmt.Println(IsSubset(b, a))

	// Output:
	// true
	// false
}

func ExampleSymmetricDifference() {
	type Car struct {
		Make  string `json:"make"`
		Model string `json:"model"`
		Year  int    `json:"year"`
	}
	cars1 := []Car{
		{Make: "Golf", Model: "GTD", Year: 2024},
		{Make: "Passat", Model: "Alltrack", Year: 2024},
		{Make: "Golf", Model: "GTI", Year: 2027},
		{Make: "Passat", Model: "Variant", Year: 2027},
		{Make: "Golf", Model: "Variant", Year: 2028},
	}
	cars2 := []Car{
		{Make: "Golf", Model: "Sportsvan", Year: 2024},
		{Make: "Golf", Model: "Variant", Year: 2027},
		{Make: "Polo", Model: "Cross", Year: 2028},
		{Make: "Polo", Model: "Sedan", Year: 2028},
	}

	cars := SymmetricDifferenceCmp(cars1, cars2, func(t Car) string {
		// filter out by make and model
		return t.Make + t.Model
	})
	fmt.Println(cars)
	// Output:
	// [{Golf GTD 2024} {Passat Alltrack 2024} {Golf GTI 2027} {Passat Variant 2027} {Golf Sportsvan 2024} {Polo Cross 2028} {Polo Sedan 2028}]

}