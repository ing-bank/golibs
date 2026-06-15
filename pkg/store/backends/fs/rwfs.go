package fs

import (
	"io"
	"os"
)

// RWFS defines a read/write filesystem interface for mocking/testing.
type RWFS interface {
	Create(name string) (io.WriteCloser, error) // Fails if file exists
	Read(name string) (io.ReadCloser, error)
	Update(name string) (io.WriteCloser, error) // Fails if file does not exist
	Delete(name string) error
	ReadDir(name string) ([]os.DirEntry, error)
	MakeDir(path string, perm os.FileMode) error
}

var _ RWFS = OSFS{}

// OSFS is a real implementation of RWFS using the os package.
type OSFS struct{}

func NewOSFS() OSFS { return OSFS{} }

func (OSFS) Read(name string) (io.ReadCloser, error) {
	return os.Open(name)
}
func (OSFS) Create(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
}
func (OSFS) Update(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0644)
}
func (OSFS) Delete(name string) error                   { return os.Remove(name) }
func (OSFS) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (OSFS) MakeDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
