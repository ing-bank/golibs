package logger

import (
	"context"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func Example() {
	ctx := context.Background()
	db, _ := store.New[string, int](memory.New, New)
	_, _ = db.List(ctx, store.ListKeysOnly)
	// the output is via logger, so stdout doesn't show sadly, but for example:
	// time="2026-04-02T16:58:18+02:00" level=info msg="listing store entries succeeded" length=0 options="store.ListKeyOnlyOption=true"

	// Output:
}
