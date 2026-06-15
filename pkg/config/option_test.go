package config

import (
	"errors"
	"testing"
)

type Foo struct {
	Bar string
}

func WithBar(name string) Opt[*Foo] {
	return func(f *Foo) error {
		f.Bar = name
		return nil
	}
}

func TestApplyOpts_WithName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"set name to Alice", "Alice", "Alice"},
		{"set name to Bob", "Bob", "Bob"},
		{"set name to empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			foo := &Foo{}
			opt := WithBar(tt.input)
			err := ApplyOpts(foo, opt)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if foo.Bar != tt.want {
				t.Errorf("expected name %q, got %q", tt.want, foo.Bar)
			}
		})
	}
}

func TestApplyOpts_NilOption(t *testing.T) {
	t.Parallel()
	foo := &Foo{}
	err := ApplyOpts(foo, nil)
	if err != nil {
		t.Errorf("expected nil error for nil option, got %v", err)
	}
}

func TestOpt_ApplyOpt_Error(t *testing.T) {
	t.Parallel()
	foo := &Foo{}
	opt := Opt[*Foo](func(f *Foo) error {
		return errors.New("fail")
	})
	err := opt.ApplyOpt(foo)
	if err == nil || err.Error() != "fail" {
		t.Errorf("expected error 'fail', got %v", err)
	}
}
