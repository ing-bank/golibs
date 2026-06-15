package fsnotify

import (
	"context"
	"fmt"
	"github.com/ing-bank/golibs/pkg/utils"
	"testing/fstest"
	"time"
)

func ExampleIntervalWatcher_Watch() {
	// Create a fake filesystem, only necessary for unittests
	fs := fstest.MapFS{"target.txt": {Data: []byte(utils.RandStr())}}

	// Create a new IntervalWatcher, fs can be omitted outside unittests
	w := NewIntervalWatcher(fs, false)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Give Watch some time to start
		time.Sleep(100 * time.Millisecond)

		// Modify mocked filesystem
		w.Lock()
		fs["target.txt"] = &fstest.MapFile{Data: []byte(utils.RandStr())}
		w.Unlock()
	}()

	// Watch files for changes
	_ = w.Watch(ctx, []string{"target.txt"}, func(ctx context.Context, hashes map[string][]byte) {
		fmt.Println("Target was modified")

		// Stop watcher via context
		cancel()
	}, 10*time.Millisecond)

	fmt.Println("Graceful shutdown complete")

	// Output:
	// Target was modified
	// Graceful shutdown complete
}
