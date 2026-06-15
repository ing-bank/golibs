package configmap

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
)

func Example() {
	db, err := NewFake(Config[int]{})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	_ = db.Apply(ctx, "1", 1, store.DryRun)
	err = db.Update(ctx, "2", 2, store.DryRun)
	if !errors.IsNotFound(err) {
		panic("expected error when updating non-existing key, got nil")
	}

	items, _ := db.List(ctx, store.ListKeysOnly)
	fmt.Println(items)

	// Output:
	// []
}
