package store

import (
	"errors"
	"reflect"
	"testing"
)

func TestMatchOption(t *testing.T) {
	type MyOpt int
	var WithMyOpt, matchMyOpt = OptionBuilder[MyOpt]()
	opts := []Option{WithMyOpt(42)}
	val, ok := matchMyOpt(&opts)
	if !ok || val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestMatchOption_RemovesMatchedIndex(t *testing.T) {
	type A int
	type B string
	var WithA, matchA = OptionBuilder[A]()
	var WithB, _ = OptionBuilder[B]()

	opts := []Option{WithA(1), WithB("foo")}
	_, ok := matchA(&opts)
	if !ok {
		t.Fatal("expected to match A")
	}
	if len(opts) != 1 || reflect.TypeOf(opts[0]) != reflect.TypeFor[B]() {
		t.Errorf("expected only B to remain, got %v", opts)
	}
}

func TestMatchOption_SkipConsume(t *testing.T) {
	type A int
	var WithA, _ = OptionBuilder[A]()
	opts := []Option{WithA(1)}
	_, ok := MatchOption[A](&opts, MatchOptionConfig{SkipConsume: true})
	if !ok {
		t.Fatal("expected to match A")
	}
	if len(opts) != 1 {
		t.Errorf("expected option not to be removed, got %v", opts)
	}
}

func TestMatchOption_NestedOptions(t *testing.T) {
	type A int
	type B string
	var WithA, _ = OptionBuilder[A]()
	var WithB, matchB = OptionBuilder[B]()

	nested := []Option{WithB("bar")}
	opts := []Option{WithA(1), nested}
	val, ok := matchB(&opts)
	if !ok || val != "bar" {
		t.Errorf("expected to match nested B, got %v", val)
	}
	// After matching, nested should be empty and removed from opts
	if len(opts) != 1 || reflect.TypeOf(opts[0]) != reflect.TypeFor[A]() {
		t.Errorf("expected only A to remain, got %v", opts)
	}
}

func TestMatchOption_NestedOptions_SkipConsume(t *testing.T) {
	type A int
	type B string
	var WithA, _ = OptionBuilder[A]()
	var WithB, _ = OptionBuilder[B]()

	nested := []Option{WithB("bar")}
	opts := []Option{WithA(1), nested}
	_, ok := MatchOption[B](&opts, MatchOptionConfig{SkipConsume: true})
	if !ok {
		t.Fatal("expected to match nested B")
	}
	// Both options should remain
	if len(opts) != 2 {
		t.Errorf("expected both options to remain, got %v", opts)
	}
}

func TestMatchOption_MultipleNestedLevels(t *testing.T) {
	type A int
	type B string
	type C float64
	var WithA, matchA = OptionBuilder[A]()
	var WithB, matchB = OptionBuilder[B]()
	var WithC, matchC = OptionBuilder[C]()

	nested2 := []Option{WithC(3.14)}
	nested1 := []Option{WithB("foo"), nested2}
	opts := []Option{WithA(1), nested1}

	val, ok := matchC(&opts)
	if !ok || val != 3.14 {
		t.Errorf("expected to match nested C, got %v", val)
	}
	// After matching, nested2 should be empty and removed from nested1, which should still contain B
	if len(opts) != 2 {
		t.Errorf("expected two options to remain, got %v", opts)
	}
	// nested1 should now only contain B
	nested1After, _ := opts[1].([]Option)
	if len(nested1After) != 1 || reflect.TypeOf(nested1After[0]) != reflect.TypeFor[B]() {
		t.Errorf("expected nested1 to only contain B, got %v", nested1After)
	}

	valB, ok := matchB(&opts)
	if !ok || valB != "foo" {
		t.Errorf("expected to match nested B, got %v", valB)
	}

	// Now only A should remain
	valA, ok := matchA(&opts)
	if !ok || valA != 1 {
		t.Errorf("expected to match A, got %v", valA)
	}
}

func TestMatchOption_NoMatch(t *testing.T) {
	type A int
	type B string
	var WithA, _ = OptionBuilder[A]()
	var _, matchB = OptionBuilder[B]()

	opts := []Option{WithA(1)}
	_, ok := matchB(&opts)
	if ok {
		t.Errorf("expected no match for B")
	}
	if len(opts) != 1 {
		t.Errorf("expected options unchanged, got %v", opts)
	}
}

func TestDryRunSerialize(t *testing.T) {
	dryRunTrue := BoolOption{"dryRun", true}
	if k, v := dryRunTrue.Serialize(); k != "dryRun" || v != "true" {
		t.Errorf("unexpected serialize result for true: %s, %s", k, v)
	}
	dryRunFalse := BoolOption{"dryRun", false}
	if k, v := dryRunFalse.Serialize(); k != "dryRun" || v != "false" {
		t.Errorf("unexpected serialize result for false: %s, %s", k, v)
	}
}

func TestWithDryRun(t *testing.T) {
	opt := WithDryRun(true)
	dr, _ := MatchDryRun(&[]Option{opt})
	if !dr {
		t.Errorf("expected true, got false")
	}
	opt = WithDryRun(false)
	dr, _ = MatchDryRun(&[]Option{opt})
	if dr {
		t.Errorf("expected false, got true")
	}
}

var withLabelSelector, matchLabelSelector = SerializableStringOptionBuilder("labelSelector")

func TestWithPrefixAndLabelSelector_CanBeMatchedIndependently(t *testing.T) {
	// Test that both WithPrefix and WithLabelSelector can coexist and be matched independently
	opts := []Option{
		WithPrefix("my-prefix"),
		withLabelSelector("app=test"),
	}

	// Match prefix
	prefix, ok := MatchPrefix(&opts)
	if !ok || prefix != "my-prefix" {
		t.Errorf("expected to match prefix 'my-prefix', got %v (ok=%v)", prefix, ok)
	}

	// Match label selector (should still work after prefix is consumed)
	labelSelector, ok := matchLabelSelector(&opts)
	if !ok || labelSelector != "app=test" {
		t.Errorf("expected to match labelSelector 'app=test', got %v (ok=%v)", labelSelector, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestWithPrefixAndLabelSelector_OrderIndependent(t *testing.T) {

	// Test that matching order doesn't matter
	opts := []Option{
		withLabelSelector("env=prod"),
		WithPrefix("test-prefix"),
	}

	// Match label selector first this time
	labelSelector, ok := matchLabelSelector(&opts)
	if !ok || labelSelector != "env=prod" {
		t.Errorf("expected to match labelSelector 'env=prod', got %v (ok=%v)", labelSelector, ok)
	}

	// Match prefix second
	prefix, ok := MatchPrefix(&opts)
	if !ok || prefix != "test-prefix" {
		t.Errorf("expected to match prefix 'test-prefix', got %v (ok=%v)", prefix, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestMatchOptionBy(t *testing.T) {
	// Test the new MatchOptionBy function with StringOption
	opts := []Option{
		StringOption{Key: "key1", Value: "value1"},
		StringOption{Key: "key2", Value: "value2"},
	}

	// Match by key1
	opt1, ok := MatchOptionBy[StringOption](&opts, func(opt StringOption) bool {
		return opt.Key == "key1"
	})
	if !ok || opt1.Value != "value1" {
		t.Errorf("expected to match key1 with value1, got %v (ok=%v)", opt1, ok)
	}

	// Match by key2
	opt2, ok := MatchOptionBy[StringOption](&opts, func(opt StringOption) bool {
		return opt.Key == "key2"
	})
	if !ok || opt2.Value != "value2" {
		t.Errorf("expected to match key2 with value2, got %v (ok=%v)", opt2, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestNoCacheAndListKeysOnly_CanBeMatchedIndependently(t *testing.T) {
	// Test that both NoCache and ListKeysOnly can coexist and be matched independently
	opts := []Option{NoCache, ListKeysOnly}

	// Match NoCache
	noCache, ok := MatchNoCache(&opts)
	if !ok || !noCache {
		t.Errorf("expected to match NoCache with value true, got %v (ok=%v)", noCache, ok)
	}

	// Match ListKeysOnly (should still work after NoCache is consumed)
	keysOnly, ok := MatchListKeyOnly(&opts)
	if !ok || !keysOnly {
		t.Errorf("expected to match ListKeysOnly with value true, got %v (ok=%v)", keysOnly, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestNoCacheAndListKeysOnly_OrderIndependent(t *testing.T) {
	// Test that matching order doesn't matter
	opts := []Option{ListKeysOnly, NoCache}

	// Match ListKeysOnly first this time
	keysOnly, ok := MatchListKeyOnly(&opts)
	if !ok || !keysOnly {
		t.Errorf("expected to match ListKeysOnly with value true, got %v (ok=%v)", keysOnly, ok)
	}

	// Match NoCache second
	noCache, ok := MatchNoCache(&opts)
	if !ok || !noCache {
		t.Errorf("expected to match NoCache with value true, got %v (ok=%v)", noCache, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestWithNoCacheAndWithListKeyOnly_CanBeMatchedIndependently(t *testing.T) {
	// Test using the With* constructors
	opts := []Option{WithNoCache(true), WithListKeyOnly(true)}

	// Match NoCache
	noCache, ok := MatchNoCache(&opts)
	if !ok || !noCache {
		t.Errorf("expected to match NoCache with value true, got %v (ok=%v)", noCache, ok)
	}

	// Match ListKeysOnly
	keysOnly, ok := MatchListKeyOnly(&opts)
	if !ok || !keysOnly {
		t.Errorf("expected to match ListKeysOnly with value true, got %v (ok=%v)", keysOnly, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestNoCacheAndListKeysOnly_WithDifferentValues(t *testing.T) {
	// Test with false values
	opts := []Option{WithNoCache(false), WithListKeyOnly(true)}

	// Match NoCache
	noCache, ok := MatchNoCache(&opts)
	if !ok || noCache {
		t.Errorf("expected to match NoCache with value false, got %v (ok=%v)", noCache, ok)
	}

	// Match ListKeysOnly
	keysOnly, ok := MatchListKeyOnly(&opts)
	if !ok || !keysOnly {
		t.Errorf("expected to match ListKeysOnly with value true, got %v (ok=%v)", keysOnly, ok)
	}

	// Both options should be consumed
	if len(opts) != 0 {
		t.Errorf("expected all options to be consumed, got %v", opts)
	}
}

func TestCheckOptionsExhausted(t *testing.T) {
	type CustomOption string

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "empty options slice",
			opts:    []Option{},
			wantErr: false,
		},
		{
			name:    "nil options slice",
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "single nil option",
			opts:    []Option{nil},
			wantErr: false,
		},
		{
			name:    "multiple nil options",
			opts:    []Option{nil, nil, nil},
			wantErr: false,
		},
		{
			name:    "single non-nil option",
			opts:    []Option{CustomOption("test")},
			wantErr: true,
		},
		{
			name:    "multiple non-nil options",
			opts:    []Option{WithDryRun(true), WithPrefix("test")},
			wantErr: true,
		},
		{
			name:    "mixed nil and non-nil options",
			opts:    []Option{nil, CustomOption("test"), nil},
			wantErr: true,
		},
		{
			name:    "empty nested slice",
			opts:    []Option{[]Option{}},
			wantErr: false,
		},
		{
			name:    "nested slice with nil",
			opts:    []Option{[]Option{nil}},
			wantErr: false,
		},
		{
			name:    "nested slice with non-nil option",
			opts:    []Option{[]Option{WithDryRun(true)}},
			wantErr: true,
		},
		{
			name:    "nested slice with multiple non-nil options",
			opts:    []Option{[]Option{WithDryRun(true), WithPrefix("test")}},
			wantErr: true,
		},
		{
			name:    "multiple nested slices with nil",
			opts:    []Option{[]Option{nil}, []Option{nil, nil}},
			wantErr: false,
		},
		{
			name:    "mixed top-level and nested options",
			opts:    []Option{CustomOption("top"), []Option{WithDryRun(true)}},
			wantErr: true,
		},
		{
			name:    "nested empty slice with top-level nil",
			opts:    []Option{nil, []Option{}, nil},
			wantErr: false,
		},
		{
			name: "deeply nested options",
			opts: []Option{[]Option{[]Option{WithDryRun(true)}}},
			// Note: CheckOptionsExhausted only checks one level deep for nested slices
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckOptionsExhausted(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckOptionsExhausted() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify error message contains ErrUnsupportedOption
			if tt.wantErr && err != nil {
				if !errors.Is(err, ErrUnsupportedOption) {
					t.Errorf("CheckOptionsExhausted() error should wrap ErrUnsupportedOption, got %v", err)
				}
			}
		})
	}
}
