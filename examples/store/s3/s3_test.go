package main

import (
	"context"
	"fmt"

	s4 "github.com/ing-bank/golibs/pkg/store/backends/s3"
)

func Example() {
	ctx := context.Background()
	client := s4.NewMockS3Client()
	// Use the following to connect to a real S3 service:
	//client, _ := s3.NewAwsS3Client(s3.ConfigAWS{
	//	Namespace: "<namespace>",
	//	Username:  "<username>",
	//	Password:  "<password>",
	//	URL:       "https://wpr.s3.ing.net",
	//})

	// Create a new S3 based store. Note that S3 is not atomic and race conditions for update/create can occur.
	store, err := s4.New(ctx, client, &s4.Config[string]{Bucket: "demo-bucket"})
	if err != nil {
		fmt.Println("New store error:", err)
		return
	}

	// Create key1
	if err := store.Create(ctx, "key1", "foo-value"); err != nil {
		fmt.Println("Create key1 error:", err)
		return
	}
	fmt.Println("Created key1")

	// Create key2
	if err := store.Create(ctx, "key2", "bar-value"); err != nil {
		fmt.Println("Create key2 error:", err)
		return
	}
	fmt.Println("Created key2")

	// List keys and values
	items, err := store.List(ctx)
	if err != nil {
		fmt.Println("List error:", err)
		return
	}
	fmt.Println("Keys and values in bucket:")
	for _, item := range items {
		fmt.Printf("  %s: %s\n", item.Key, item.Value)
	}

	// Delete keys
	for _, item := range items {
		fmt.Println("Delete", item.Key)
		if err := store.Delete(ctx, item.Key); err != nil {
			fmt.Println("Delete error:", err)
			return
		}
	}

	err = client.DeleteBucket(ctx, "demo-bucket") // Was created by constructor
	if err != nil {
		fmt.Println("Delete bucket error:", err)
		return
	}

	// Output:
	// Created key1
	// Created key2
	// Keys and values in bucket:
	//   key1: foo-value
	//   key2: bar-value
	// Delete key1
	// Delete key2
}
