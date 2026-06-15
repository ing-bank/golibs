package fs

import (
	"context"
	"fmt"
)

func Example() {
	ctx := context.Background()

	// Example usage of the Filesystem store
	store, err := New[int](Options{Basepath: "/tmp/golibs-fs-test"})
	// use NewFake[int](Options{Basepath: "test"}) for in-memory testing, see unittests
	if err != nil {
		panic(err)
	}

	err = store.Create(ctx, "foo", 1)
	fmt.Println("creating foo has result:", err) // nil

	err = store.Create(ctx, "foo", 2)
	fmt.Println("creating foo for the second time has result:", err) // Conflict

	// Apply means create when it does not exist, otherwise update
	err = store.Apply(ctx, "foo", 2)
	fmt.Println("applying foo has result:", err) // nil

	err = store.Apply(ctx, "bar", 1)
	fmt.Println("applying bar has result:", err) // nil

	items, err := store.List(ctx)
	fmt.Println("listing store has result:", err, items)

	err = store.Delete(ctx, "foo")
	fmt.Println("deleting foo has result:", err) // nil

	// Output:
	// creating foo has result: <nil>
	// creating foo for the second time has result: conflict
	// applying foo has result: <nil>
	// applying bar has result: <nil>
	// listing store has result: <nil> [{bar 1} {foo 2}]
	// deleting foo has result: <nil>
}
