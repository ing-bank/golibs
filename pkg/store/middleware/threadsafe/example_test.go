package threadsafe

import (
	"context"
	"time"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func Example() {
	ctx := context.Background()
	cache, _ := store.New[int, int](memory.New, New) // NewBuilder -> threadsafe.

	go func() {
		for i := range 1000 {
			_ = cache.Apply(ctx, i, i)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	go func() {
		for i := range 1000 {
			_ = cache.Apply(ctx, i, -i)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Output:
}
