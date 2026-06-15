package memory

import (
	"context"
	"fmt"

	store "github.com/ing-bank/golibs/pkg/store"
)

func Example() {
	ctx := context.Background()

	// Example usage of the Memory cache store
	db, err := New[string, int]()
	if err != nil {
		panic(err)
	}

	err = db.Create(ctx, "foo", 1)
	fmt.Println("creating foo has result:", err) // nil

	err = db.Create(ctx, "foo", 2)
	fmt.Println("creating foo for the second time has result:", err) // Conflict

	// Apply means create when it does not exist, otherwise update
	err = db.Apply(ctx, "foo", 2)
	fmt.Println("applying foo has result:", err) // nil

	err = db.Apply(ctx, "bar", 1)
	fmt.Println("applying bar has result:", err) // nil

	err = db.Apply(ctx, "skip", 1, store.WithDryRun(true))
	fmt.Println("applying skip has result:", err) // nil

	items, err := db.List(ctx)
	fmt.Println("listing store has result:", err, items)

	err = db.Delete(ctx, "foo")
	fmt.Println("deleting foo has result:", err) // nil

	// Output:
	// creating foo has result: <nil>
	// creating foo for the second time has result: conflict
	// applying foo has result: <nil>
	// applying bar has result: <nil>
	// applying skip has result: <nil>
	// listing store has result: <nil> [{bar 1} {foo 2}]
	// deleting foo has result: <nil>
}
