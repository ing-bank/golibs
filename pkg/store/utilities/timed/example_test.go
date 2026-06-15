package timed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/logger"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Example() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new timed cache with a sync period of 1 second
	cfg := &Config{
		SyncPeriod: metav1.Duration{Duration: 1 * time.Second},
		MaxAge:     metav1.Duration{Duration: 3 * time.Second},
	}

	store, _ := NewForBuilders[string, string](cfg, memory.New, threadsafe.New, logger.New)

	// Start cache maintenance tasks in the background
	do := func(ctx context.Context) error {
		return nil
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- store.Run(ctx, do)
	}()

	// Update an entry
	_ = store.Create(ctx, "foo", "bar")

	// Retrieve the entry
	val, err := store.Read(ctx, "foo")
	fmt.Println(val, err == nil)

	// Wait for the entry to expire
	time.Sleep(5 * time.Second)
	_, err = store.Read(ctx, "foo")
	fmt.Println(err != nil)

	cancel()
	if err := <-errChan; err != nil { // Wait longer than MaxAgeSec (2s)
		if !errors.Is(err, context.Canceled) {
			fmt.Println("Error from cache run:", err)
		}
	}

	// Output:
	// bar true
	// true
}
