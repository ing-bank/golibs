package dependencies

import (
	"context"
	"errors"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/nameable"
	"github.com/ing-bank/golibs/pkg/store/middleware/validatable"
)

type Foo struct {
	Name string
}

// Store backends can return dependencies in the constructor backend builder functions, e.g. NewBackend
// Here we hijack the memory store, assume in this example that "we are memory store"
func NewStoreWithDependencies() (store.Store[string, *Foo], error) {
	return store.New[string, *Foo](
		memory.NewBuilder[string, *Foo](),
		nameable.New,
		validatable.New[string, *Foo],
	)
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

func (f *Foo) GetName() string {
	return f.Name
}

func Example() {
	ctx := context.Background()

	db, err := NewStoreWithDependencies()
	if err != nil {
		panic(err)
	}

	// Should be fine
	err = db.Apply(ctx, "one", &Foo{Name: "one"})
	fmt.Println(err)

	// Name should be required
	err = db.Apply(ctx, "one", &Foo{})
	fmt.Println(err)

	// Name should match object key
	err = db.Apply(ctx, "one", &Foo{Name: "two"})
	fmt.Println(err)

	// Should not crash
	err = db.Apply(ctx, "", nil)
	fmt.Println(err)

	// Output:
	// <nil>
	// name is required
	// object key does not match object name
	// foo cannot be nil
}
