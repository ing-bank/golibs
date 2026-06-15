package fsnotify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"github.com/ing-bank/golibs/pkg/utils"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func TestWatch(t *testing.T) {
	// Create a fake filesystem
	fs := fstest.MapFS{"target.txt": {Data: []byte(utils.RandStr())}}
	w := NewIntervalWatcher(fs, false)

	globVarLock := sync.Mutex{}
	var (
		wasGrace   bool
		wasChanged bool
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel() // We trigger this manually, as well

	// Run the watcher, set flags accordingly
	go func() {
		err := w.Watch(ctx, []string{"target.txt"}, func(_ context.Context, hashes map[string][]byte) {
			globVarLock.Lock()
			wasChanged = true
			globVarLock.Unlock()
		}, 10*time.Millisecond)
		if err == nil {
			globVarLock.Lock()
			wasGrace = true
			globVarLock.Unlock()
		} else {
			t.Errorf("watcher failed: %v", err)
		}
	}()

	// Wait one cycle, no events should be triggered
	time.Sleep(15 * time.Millisecond)
	globVarLock.Lock()
	if wasChanged {
		t.Fatalf("expected hashes not to change")
	}
	globVarLock.Unlock()

	w.Lock() // Lock not required for normal FS
	// Modify mocked filesystem
	fs["target.txt"] = &fstest.MapFile{Data: []byte(utils.RandStr())}
	w.Unlock()

	// Wait cycle, we should get an event
	time.Sleep(20 * time.Millisecond)
	globVarLock.Lock()
	if !wasChanged {
		t.Fatalf("expected hashes to change")
	}
	globVarLock.Unlock()

	// Trigger graceful shutdown
	cancel()
	time.Sleep(10 * time.Millisecond) // Give some time for Goroutine
	globVarLock.Lock()
	if !wasGrace {
		t.Fatalf("expected graceful shutdown")
	}
	globVarLock.Unlock()
}

func TestCalcHash(t *testing.T) {
	data := utils.RandStr()

	hasher := sha256.New()
	hasher.Write([]byte(data))
	shasum := hasher.Sum(nil)

	var buffer bytes.Buffer
	if _, err := buffer.WriteString(data); err != nil {
		t.Errorf("error creating buffer: %v", err)
	}

	hash, err := IntervalWatcher{}.calcHash(&buffer)
	if err != nil {
		t.Errorf("error calculating hash: %v", err)
	} else if !bytes.Equal(shasum, hash) {
		t.Errorf("incorrect hash: expected %v, but found %v", shasum, hash)
	}
}
