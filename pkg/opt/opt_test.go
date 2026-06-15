package opt

import (
	"testing"
)

func TestOpt(t *testing.T) {
	Foo := func(b ...string) string {
		return Opt("a", b)
	}

	tests := []struct {
		have []string
		want string
	}{
		{nil, "a"},
		{[]string{"a"}, "a"},
		{[]string{"b"}, "b"},
		{[]string{"a", "b"}, "a"},
	}

	for _, tt := range tests {
		found := Foo(tt.have...)
		if found != tt.want {
			t.Errorf("got %v want %v", found, tt.want)
		}
	}
}
