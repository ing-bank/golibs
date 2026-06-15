package fs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Note: fs.NewFake constructor is in fs.go. This file contains the in-memory implementation of RWFS for testing/mocking.

// MemFS is a minimal in-memory implementation of RWFS for testing/mocking a CRUD store.
type MemFS struct {
	files map[string][]byte
}

func NewMemFS() *MemFS {
	return &MemFS{files: make(map[string][]byte)}
}

func (m *MemFS) Create(name string) (io.WriteCloser, error) {
	if _, ok := m.files[name]; ok {
		return nil, os.ErrExist
	}
	return &memFileWriter{fs: m, name: name}, nil
}

func (m *MemFS) Read(name string) (io.ReadCloser, error) {
	data, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MemFS) Update(name string) (io.WriteCloser, error) {
	if _, ok := m.files[name]; !ok {
		return nil, os.ErrNotExist
	}
	return &memFileWriter{fs: m, name: name}, nil
}

func (m *MemFS) Delete(name string) error {
	if _, ok := m.files[name]; ok {
		delete(m.files, name)
		return nil
	}
	return os.ErrNotExist
}

func (m *MemFS) ReadDir(name string) ([]os.DirEntry, error) {
	var entries []os.DirEntry
	for fname := range m.files {
		if strings.HasPrefix(fname, name) { // Base path
			entries = append(entries, memDirEntry{name: filepath.Base(fname)})
		}
	}
	return entries, nil
}

func (m *MemFS) MakeDir(path string, perm os.FileMode) error {
	// No-op for in-memory
	return nil
}

type memFileWriter struct {
	fs     *MemFS
	name   string
	buf    bytes.Buffer
	closed bool
}

func (w *memFileWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("write to closed file")
	}
	return w.buf.Write(p)
}

func (w *memFileWriter) Close() error {
	if w.closed {
		return errors.New("already closed")
	}
	w.fs.files[w.name] = w.buf.Bytes()
	w.closed = true
	return nil
}

type memDirEntry struct {
	name string
}

func (e memDirEntry) Name() string { return e.name }

// IsDir always returns false for in-memory files.
func (e memDirEntry) IsDir() bool { return false }

// Type returns the file mode (always 0 for in-memory files).
func (e memDirEntry) Type() os.FileMode { return 0 }

// Info is not implemented for in-memory files.
func (e memDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("not implemented") }

var _ RWFS = &MemFS{}
