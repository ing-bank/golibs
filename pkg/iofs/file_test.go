package iofs

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestCase[T any] struct {
	name  string
	input *File[T]
	want  string
}

type MyFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

const (
	file1 = `{"name": "foo", "type": "json"}`
	file2 = `{"name": "bar", "type": "json"}`
)

var testFiles = fstest.MapFS{
	"file1.json": {
		Data:    []byte(file1),
		Mode:    0456,
		ModTime: time.Now(),
	},
	"file2.json": {
		Data:    []byte(file2),
		Mode:    0456,
		ModTime: time.Now(),
	},
}

func TestReadFileFS(t *testing.T) {
	var testcases = []TestCase[MyFile]{
		{
			name:  "test 1",
			input: readFile[MyFile]("file1.json"),
			want:  file1,
		}, {
			name:  "test 2",
			input: readFile[MyFile]("file2.json"),
			want:  file2,
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValues(t, tt.want, tt.input.String())
			assert.EqualValues(t, []byte(tt.want), tt.input.Bytes())
		})
	}
}

func TestReadFile(t *testing.T) {
	tmpdir := t.TempDir()
	f1, err := os.CreateTemp(tmpdir, "file1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f1.Write([]byte(file1))
	if err != nil {
		t.Fatal(err)
	}
	f2, err := os.CreateTemp(tmpdir, "file2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f2.Write([]byte(file2))
	if err != nil {
		t.Fatal(err)
	}
	var testcases = []TestCase[MyFile]{
		{
			name:  "test 1",
			input: ReadFileOrDie[MyFile](f1.Name()),
			want:  file1,
		},
		{
			name:  "test 2",
			input: ReadFileOrDie[MyFile](f2.Name()),
			want:  file2,
		},
	}
	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValues(t, tt.want, tt.input.String())
			assert.EqualValues(t, []byte(tt.want), tt.input.Bytes())
		})
	}
}

func readFile[T any](name string) *File[T] {
	f, err := ReadFileFS[T](testFiles, name)
	if err != nil {
		panic(err)
	}
	return f
}
