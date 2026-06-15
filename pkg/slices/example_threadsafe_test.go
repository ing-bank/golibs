package slices

import (
	"fmt"
	"sync"
)

type Person struct {
	Name string
	Age  int
}

func ExampleThreadSafe_List() {
	ts := NewSlice[Person]()

	var wg sync.WaitGroup
	people := []Person{
		{"Charlie", 40},
		{"Bob", 25},
		{"Alice", 30},
	}

	for _, p := range people {
		wg.Add(1)
		go func(person Person) {
			defer wg.Done()
			ts.Add(person)
		}(p)
	}
	wg.Wait()

	ts.SortFunc(func(a, b Person) int {
		if a.Age < b.Age {
			return -1
		}
		if a.Age > b.Age {
			return 1
		}
		return 0
	})
	ts.List(func(p Person) bool {
		fmt.Printf("%s (%d)\n", p.Name, p.Age)
		return true
	})

	// Output:
	// Bob (25)
	// Alice (30)
	// Charlie (40)
}
