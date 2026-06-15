package s3

import (
	"context"
	"fmt"
)

func Example() {
	ctx := context.Background()
	client := NewMockS3Client()

	cfg := &Config[int]{Bucket: "test-Bucket"}
	store, err := New[int](ctx, client, cfg)
	if err != nil {
		panic(err)
	}

	// Create
	err = store.Create(ctx, "foo", 42)
	fmt.Println("creating foo has result:", err)

	// Read
	val, err := store.Read(ctx, "foo")
	fmt.Println("reading foo has result:", err, val)

	// Update
	err = store.Update(ctx, "foo", 43)
	fmt.Println("updating foo has result:", err)

	// Apply (upsert)
	err = store.Apply(ctx, "bar", 99)
	fmt.Println("applying bar has result:", err)

	// List
	items, err := store.List(ctx)
	fmt.Println("listing store has result:", err, items)

	// Delete
	err = store.Delete(ctx, "foo")
	fmt.Println("deleting foo has result:", err)

	// Output:
	// creating foo has result: <nil>
	// reading foo has result: <nil> 42
	// updating foo has result: <nil>
	// applying bar has result: <nil>
	// listing store has result: <nil> [{bar 99} {foo 43}]
	// deleting foo has result: <nil>
}
