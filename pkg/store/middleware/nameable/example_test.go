package nameable

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

func (f *Foo) GetName() string {
	return f.Name
}

func Example() {
	ctx := context.Background()

	db, err := store.New(memory.New, New[*Foo])
	if err != nil {
		panic(err)
	}

	one := &Foo{Name: "one"}

	// Applying entry where key matches object name should be fine
	err = db.Apply(ctx, "one", one)
	if err != nil {
		panic(err)
	}
	fmt.Println(err)

	// Should cause error
	err = db.Apply(ctx, "two", one)
	if err == nil {
		panic(err)
	}

	fmt.Printf("%v -> %v", errors.Is(err, ErrNameableKeyMismatch), err)

	// Output:
	// <nil>
	// true -> object key does not match object name
}
