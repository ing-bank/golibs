package list

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

type MyData struct {
	Items []string `json:"items"`
}

func NewLargeData() *MyData {
	var items []string
	for i := range 50 {
		items = append(items, strconv.Itoa(i))
	}
	return &MyData{Items: items}
}

func Example() {
	// This example test shows how to efficiently handle listing of keys with large amount of value data

	// Create in memory store. It's important that our data is a pointer so that it can truly be nil
	// to make the most out of the ListKeysOnly feature.
	db, _ := store.New[string, *MyData](memory.New)

	// Populate with some large data sets
	ctx := context.Background()
	_ = db.Create(ctx, "1", NewLargeData())
	_ = db.Create(ctx, "2", NewLargeData())
	_ = db.Create(ctx, "3", NewLargeData())

	// List items, with KeysOnlyFeature
	items, err := db.List(ctx, store.ListKeysOnly)
	if err != nil {
		panic(err)
	}
	fmt.Println(items) // Values will be the defaults, in case of pointers: nil

	item, err := db.Read(ctx, "1")
	if err != nil {
		panic(err)
	}
	fmt.Println(item) // We can still read the values, of course

	// Output:
	// [{1 <nil>} {2 <nil>} {3 <nil>}]
	// &{[0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49]}
}
