package main

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/logger"
	"github.com/ing-bank/golibs/pkg/store/middleware/metrics"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
)

func Example() {
	ctx := context.Background()

	// Stores are composed of a backend and optional decorators.
	// Here we create a store with an in-memory backend, and add thread-safety,
	// logging and metrics as decorators. More complicated configuration can be
	// done by using builder functions, see the replication example for that.
	db, err := store.New[string, int](
		memory.New,
		threadsafe.New,
		logger.New,
		metrics.New,
	)
	if err != nil {
		panic(err)
	}

	_ = db.Create(ctx, "foo", 1)
	_ = db.Apply(ctx, "bar", 2)

	val, _ := db.Read(ctx, "foo")
	fmt.Println("Value for 'foo':", val)

	// Listing the store returns a list of key-value pairs
	items, _ := db.List(ctx)
	mapped := items.AsMap() // We can convert the list to a map for easier access, basically copying the store
	fmt.Println(len(mapped), mapped["bar"], mapped["foo"])

	// Output:
	// Value for 'foo': 1
	// 2 2 1
}
