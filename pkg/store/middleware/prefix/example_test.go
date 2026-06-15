package prefix

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

type MyData struct{}

func Example() {
	ctx := context.Background()
	db, _ := store.New(
		memory.New,
		NewBuilder[*MyData]("prefix-"),
	)

	// Store some data
	_ = db.Apply(ctx, "key1", &MyData{}) // No prefix here, automatically applied for us
	_ = db.Apply(ctx, "key2", &MyData{}) // No prefix here, automatically applied for us

	// List the data
	items, _ := db.List(ctx)
	for _, item := range items {
		fmt.Printf("%s\n", item.Key) // No prefix here, automatically stripped for us
	}

	// For testing purposes we peek into the internals of the store to verify that the keys are indeed prefixed.
	peek := db.(*Prefix[*MyData])
	items, _ = peek.store.List(ctx)
	for _, item := range items {
		fmt.Printf("Internal key: %s\n", item.Key) // Here we see the prefix is indeed applied to the keys in the underlying store
	}

	// Output:
	// key1
	// key2
	// Internal key: prefix-key1
	// Internal key: prefix-key2
}
