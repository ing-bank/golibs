// Package fsnotify provides IntervalWatcher which can be used to monitor changes to files.
// It calculates a hash per file given and checks per given interval whether that hash changes.
// When a hash changes a callback is called. IntervalWatcher uses polling because true fs events
// may require elevated permissions.
package fsnotify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"
)

// IntervalWatcher can be used to monitor changes to files via the Watch method
type IntervalWatcher struct {
	sync.Locker
	FS        fs.FS // Filesystem to use
	NotRootFS bool  // Setting to true will no longer trim leading slashes from file paths for assumed root FS
}

func NewIntervalWatcher(fs fs.FS, nonRootFS bool) *IntervalWatcher {
	return &IntervalWatcher{
		Locker:    &sync.Mutex{},
		FS:        fs,
		NotRootFS: nonRootFS,
	}
}

// Watch calculates hashes for the given files each interval. If a hash changes the do function is called
// This function is blocking and terminates gracefully when the context is cancelled.
// Keep note of fs.FS valid paths (no leading slashes, https://pkg.go.dev/io/fs#ValidPath). By default,
// this function tries to be user-friendly and removes leading slashes from filenames, assuming root FS
// is mounted, this behavior can be disabled by setting the NotRootFS flag.
func (w IntervalWatcher) Watch(ctx context.Context, files []string, do func(ctx context.Context, hashes map[string][]byte), interval time.Duration) error {
	if w.FS == nil {
		w.FS = os.DirFS("/")
	}

	hashes := map[string][]byte{} // Filename to corresponding hash
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			ok, err := w.fileHashChanged(files, hashes)
			if err != nil {
				return err
			}
			if ok {
				do(ctx, hashes)
			}
		}
	}
}

func (w IntervalWatcher) fileHashChanged(files []string, hashes map[string][]byte) (bool, error) {
	for _, file := range files {
		if !w.NotRootFS && len(file) > 2 && file[0] == '/' {
			file = file[1:] // Root FS already mounted
		}

		hash, err := w.calcFileHash(file)
		if err != nil {
			return false, err
		}

		old, oldExists := hashes[file]
		if !oldExists {
			hashes[file] = hash
		} else if !bytes.Equal(old, hash) {
			hashes[file] = hash
			return true, nil
		}
	}
	return false, nil
}

func (w IntervalWatcher) calcFileHash(file string) ([]byte, error) {
	w.Lock()
	defer w.Unlock()
	f, err := w.FS.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	return w.calcHash(f)
}

func (_ IntervalWatcher) calcHash(fd io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	hasher.Write(raw)
	return hasher.Sum(nil), nil
}
