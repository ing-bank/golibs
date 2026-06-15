// Package iofs is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
//
// Package iofs provides generic utilities for reading and parsing files from various file systems.
//
// It wraps the standard library's io/fs package to provide a more ergonomic interface for
// file operations with built-in support for multiple data formats and panic-safe variants.
//
// Core Features:
//
//   - Generic File Type: File[T] is a generic wrapper that can hold any file content as bytes,
//     with helper methods to convert to different formats.
//   - Multiple File Systems: Read files from fs.FS (like os.DirFS) or the OS file system directly.
//   - Format Support: Convert file content to bytes, strings, or JSON via helper methods.
//   - Panic-Safe Variants: OrDie variants that panic with descriptive errors for testing/convenience code.
//
// Usage:
//
// Reading a JSON file:
//
//	type Config struct {
//		Name string `json:"name"`
//		Port int    `json:"port"`
//	}
//
//	// Read and parse in one call
//	file, err := iofs.ReadFile[Config]("/path/to/config.json")
//	if err != nil {
//		log.Fatal(err)
//	}
//	config, err := file.Json()
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Or use the panic-safe variant during tests:
//
//	config := iofs.ReadFileOrDie[Config]("/path/to/config.json").JsonOrDie()
//
// Reading from a custom file system:
//
//	fsys := os.DirFS("./data")
//	file, err := iofs.ReadFileFS[[]byte](fsys, "data.bin")
//	if err != nil {
//		log.Fatal(err)
//	}
//	raw := file.Bytes()
//
package iofs

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

// File holds any content type
type File[T any] struct {
	Content []byte
}

// ReadFileFS reads the named file from the file system fs and returns a `File[T]` containing the file's content.
// The `T` type parameter represents the specific type of data stored in the file.
func ReadFileFS[T any](sysfs fs.FS, name string) (*File[T], error) {
	b, err := fs.ReadFile(sysfs, name)
	if err != nil {
		return nil, err
	}
	return &File[T]{b}, nil
}

// ReadFile calls ReadFileFS with the named file and the file system extracted from its name.
func ReadFile[T any](name string) (*File[T], error) {
	dir, file := filepath.Split(name)
	b, err := ReadFileFS[T](os.DirFS(dir), file)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ReadFileOrDie calls ReadFile and if the file cannot be read,
// this function panics with a descriptive error message.
func ReadFileOrDie[T any](name string) *File[T] {
	f, err := ReadFile[T](name)
	if err != nil {
		panic(err)
	}
	return f
}

// Bytes prints out the file's content as []byte
func (f *File[T]) Bytes() []byte {
	return f.Content
}

// String prints out the file's content as string
func (f *File[T]) String() string {
	return string(f.Content)
}

// Json parses the JSON-encoded data file's content
func (f *File[T]) Json() (T, error) {
	var data T
	err := json.Unmarshal(f.Content, &data)
	if err != nil {
		return data, err
	}
	return data, nil
}

// JsonOrDie calls Json and if the data cannot be unmarshalled,
// this function panics with a descriptive error message.
func (f *File[T]) JsonOrDie() T {
	data, err := f.Json()
	if err != nil {
		panic(err)
	}
	return data
}
