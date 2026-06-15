package validatable

import (
	"context"
	"errors"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

type Foo struct {
	Name string
}

func (f *Foo) Validate() error {
	if f == nil {
		return errors.New("foo cannot be nil")
	}
	if f.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func Example() {
	ctx := context.Background()

	db, err := store.New(memory.New, New[string, *Foo])
	if err != nil {
		panic(err)
	}

	// Should be fine
	err = db.Apply(ctx, "one", &Foo{Name: "one"})
	fmt.Println(err)

	// Name should be required
	err = db.Apply(ctx, "one", &Foo{})
	fmt.Println(err)

	// Should not crash
	err = db.Apply(ctx, "", nil)
	fmt.Println(err)

	// Output:
	// <nil>
	// name is required
	// foo cannot be nil
}
