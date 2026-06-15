package clone

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/utilities/defaultmap"
)

func Example() {
	source, _ := store.New[string, int](memory.New)
	destination, _ := store.New[string, int](memory.New)

	// Populate
	ctx := context.Background()
	_ = source.Create(ctx, "1", 1)
	_ = source.Create(ctx, "2", 2)
	_ = source.Create(ctx, "3", 3)
	_ = destination.Create(ctx, "0", 0)  // Will untouched
	_ = destination.Create(ctx, "1", -1) // Will be resolved via collision function

	// Clone store
	merger, err := defaultmap.NewForStore(destination)
	if err != nil {
		panic(err)
	}
	collision := func(old, new int) int { return new }
	if err := merger.Merge(ctx, source, collision); err != nil {
		panic(err)
	}

	// List items
	items, err := destination.List(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println(items)

	// Output:
	// [{0 0} {1 1} {2 2} {3 3}]
}
