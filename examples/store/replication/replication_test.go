package main

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/replicate"
	s4 "github.com/ing-bank/golibs/pkg/store/backends/s3"
)

func Example() {
	ctx := context.Background()

	// Note that S3 backend is not atomic. Create/Update calls are done by a sequential read and put. This means
	// that using S3 for creating resource locks may produce race conditions.
	db, err := store.New[string, string](
		replicate.NewBuilder(replicate.Config{WorkflowName: "replicate"},
			s4.NewBuilder(ctx, s4.NewMockS3Client(), &s4.Config[string]{Bucket: "bucket1"}), // E.g. DCR
			s4.NewBuilder(ctx, s4.NewMockS3Client(), &s4.Config[string]{Bucket: "bucket1"}), // E.g. WPR
		),
		// Add middleware as required
	)
	if err != nil {
		panic(err)
	}

	rollbackErrs := make(chan error)
	err = db.Create(ctx, "foo", "bar", replicate.WithRollback(rollbackErrs))
	if err != nil {
		panic(err)
	}
	fmt.Println(err) // <nil>

	value, err := db.Read(ctx, "foo")
	if err != nil {
		panic(err)
	}
	fmt.Println("foo:", value) // foo: bar

	// Output:
	// <nil>
	// foo: bar
}
