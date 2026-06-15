package response

import (
	"encoding/json"
	goerrors "errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ing-bank/golibs/pkg/errors"
)

type ExampleData struct {
	Foo string `json:"foo"`
}

func (a ExampleData) IsEqual(b ExampleData) bool {
	return a.Foo == b.Foo
}

func TestResponse_IsOK(t *testing.T) {
	err := fmt.Errorf("err")
	tests := []struct {
		have Data
		want bool
	}{
		{Data{Err: nil, Status: 200}, true},
		{Data{Err: nil, Status: 201}, true},
		{Data{Err: nil, Status: 202}, true},
		{Data{Err: nil, Status: 500}, false},
		{Data{Err: err, Status: 200}, false},
	}

	for _, tt := range tests {
		found := tt.have.IsOK()
		if found != tt.want {
			t.Errorf("got %v want %v", found, tt.want)
		}
	}
}

func TestResponse_Parse(t *testing.T) {
	want := &ExampleData{Foo: "bar"}
	raw, _ := json.Marshal(want)
	r := &Data{Raw: raw, Status: http.StatusOK}

	got := ExampleData{}
	if resp := r.Parse(&got); !resp.IsOK() {
		t.Errorf("failed to parse response: %v", resp.Error())
	}

	if !want.IsEqual(got) {
		t.Errorf("want: %v, got: %v", *want, got)
	}

	r.Status = http.StatusNotFound
	if resp := r.Parse(&got); resp.IsOK() {
		t.Errorf("expected failed parse")
	}
}

func TestResponse_Error(t *testing.T) {
	err := fmt.Errorf("err") //nolint:err113
	tests := []struct {
		have Data
		want error
	}{
		{Data{Err: nil, Status: 200}, nil},
		{Data{Err: nil, Status: 201}, nil},
		{Data{Err: err, Status: 201}, err},
		{Data{Err: err, Status: 201}, err},
		{Data{Err: nil, Status: 0}, errors.ErrHTTPStatus},
		{Data{Err: nil, Status: 404}, errors.ErrNotFound},
		{Data{Err: nil, Status: 500}, errors.ErrInternalServerError},
	}

	for _, tt := range tests {
		found := tt.have.Error()
		if !goerrors.Is(found, tt.want) {
			t.Errorf("got '%v' want '%v'", found, tt.want)
		}
	}
}
