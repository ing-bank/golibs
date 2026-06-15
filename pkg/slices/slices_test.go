package slices

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestUnique(t *testing.T) {
	{
		in := []int{1, 1, 2, 3, 3, 4, 5, 6, 6}
		want := []int{1, 2, 3, 4, 5, 6}
		got := Unique(in)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	}
	{
		in := []string{"a", "b", "c", "b"}
		want := []string{"a", "b", "c"}
		got := Unique(in)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	}
}

func TestUniqueCmp(t *testing.T) {
	type Example struct {
		Data string
	}

	in := []Example{{"a"}, {"b"}, {"c"}, {"a"}, {"c"}}
	want := []Example{{"a"}, {"b"}, {"c"}}

	{
		got := Unique(in)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	}
	{
		got := UniqueCmp(in, func(e Example) string {
			return e.Data
		})
		if !reflect.DeepEqual(want, got) {
			t.Errorf("want %v, got %v", want, got)
		}
	}
}

func TestTransform(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	want := []string{"1", "2", "3", "4", "5"}

	got := Transform(in, func(item int) string {
		return fmt.Sprintf("%d", item)
	})

	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestFlatMap(t *testing.T) {
	input := []int{1, 2, 3}
	f := func(a int) []int {
		return []int{a, a * 2}
	}
	expected := []int{1, 2, 2, 4, 3, 6}
	result := FlatMap(input, f)
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestFilter(t *testing.T) {
	err1 := fmt.Errorf("1")
	err2 := fmt.Errorf("2")
	in := []error{nil, err1, err1, nil, err2, nil}
	want := []error{err1, err1, err2}

	got := Filter(in, func(item error) bool {
		return item != nil
	})

	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestMap(t *testing.T) {
	type Data struct {
		Name string
		Age  int
	}

	people := []Data{{"Bob", 1}, {"Jim", 2}}
	lookup := Map(people, func(item Data) string { return item.Name })

	if length := len(lookup); length != 2 {
		t.Errorf("expected length 2 but got %d", length)
	}

	for _, person := range people {
		found, ok := lookup[person.Name]

		if !ok {
			t.Errorf("did not find person %s", person.Name)
		}

		if !reflect.DeepEqual(&person, &found) {
			t.Errorf("expected same person in lookup, expected=%v, got=%v", person, found)
		}
	}
}

func TestIsSubset(t *testing.T) {
	type testCase struct {
		name string
		a    []int
		b    []int
		want bool
	}
	tests := []testCase{
		{
			name: "equal sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: true,
		},
		{
			name: "a is larger than b",
			a:    []int{1, 2, 3},
			b:    []int{1, 2},
			want: false,
		},
		{
			name: "a is empty",
			a:    []int{},
			b:    []int{1, 2},
			want: true,
		},
		{
			name: "b is empty",
			a:    []int{1, 2, 3},
			b:    []int{},
			want: false,
		},
		{
			name: "a has one element of b",
			a:    []int{1},
			b:    []int{1, 2, 3},
			want: true,
		},
		{
			name: "a is subset of b",
			a:    []int{7, 8},
			b:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSubset(tt.a, tt.b); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestMapValues(t *testing.T) {
	type testCase struct {
		name string
		in   map[string]int
		want []int
	}
	tests := []testCase{
		{
			name: "simple case",
			in:   map[string]int{"a": 1, "b": 2, "c": 3},
			want: []int{1, 2, 3},
		},
		{
			name: "duplicates",
			in:   map[string]int{"a": 1, "b": 2, "c": 2},
			want: []int{1, 2, 2},
		},
		{
			name: "unsorted map values",
			in:   map[string]int{"a": 1, "c": 3, "b": 2},
			want: []int{1, 2, 3},
		},
		{
			name: "empty array",
			in:   map[string]int{},
			want: []int{},
		},
		{
			name: "larger unsorted array",
			in:   map[string]int{"j": 10, "k": 11, "c": 3, "d": 4, "l": 12, "q": 17, "r": 18, "m": 13, "n": 14, "o": 15, "p": 16, "a": 1, "b": 2, "e": 5, "f": 6, "g": 7, "h": 8, "i": 9, "s": 19},
			want: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapValues(tt.in); !slices.Equal(tt.want, got) {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// Test function for Contains with []int
func TestContains(t *testing.T) {
	{
		type Foo struct {
			A string
			B string
		}

		one := []Foo{{"A", "B"}}
		find := Foo{"A", "B"}

		if !Contains(one, find) {
			t.Errorf("%s should contain %s", one, find)
		}
	}
	{
		tests := []struct {
			name     string
			slice    []int
			value    int
			expected bool
		}{
			{"Value present at index 1", []int{1, 2, 3}, 2, true},
			{"Value present at index 0", []int{2, 1, 3}, 2, true},
			{"Value not present", []int{1, 3, 4}, 2, false},
			{"Empty slice", []int{}, 1, false},
			{"Single element slice, value present", []int{1}, 1, true},
			{"Single element slice, value absent", []int{2}, 1, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := Contains(tt.slice, tt.value)
				if result != tt.expected {
					t.Errorf("Contains(%v, %v) = %v; want %v", tt.slice, tt.value, result, tt.expected)
				}
			})
		}
	}
}

// Test function for Contains with []string
func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{"Value present at index 1", []string{"/apple", "/banana", "cherry"}, "/banana", true},
		{"Value present at index 0", []string{"banana", "apple", "cherry"}, "banana", true},
		{"Value not present", []string{"apple", "cherry", "date"}, "banana", false},
		{"Empty slice", []string{}, "apple", false},
		{"Single element slice, value present", []string{"apple"}, "apple", true},
		{"Single element slice, value absent", []string{"banana"}, "apple", false},
		{"Value present with duplicates", []string{"apple", "banana", "banana"}, "banana", true},
		{"Value present at last index", []string{"apple", "banana", "cherry"}, "cherry", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.slice, tt.value)
			if result != tt.expected {
				t.Errorf("Contains(%v, %v) = %v; want %v", tt.slice, tt.value, result, tt.expected)
			}
		})
	}
}

func TestReMap(t *testing.T) {
	type Data struct {
		A string
		B int
	}

	start := map[string]Data{"a": {"a", 1}, "b": {"b", 2}}
	end := ReMap(start, func(item Data) int {
		return item.B
	})
	if len(start) != len(end) {
		t.Errorf("len(start) = %d; len(end) = %d", len(start), len(end))
	}
	if end[1] != start["a"] {
		t.Errorf("end = %v; want %v", end[1], start["a"])
	}
	if end[2] != start["b"] {
		t.Errorf("end = %v; want %v", end[2], start["b"])
	}
}

func TestMergeMap(t *testing.T) {
	type args struct {
		a  map[string]int
		bs []map[string]int
	}
	tests := []struct {
		name string
		args args
		want map[string]int
	}{
		{
			name: "Single map",
			args: args{
				a:  map[string]int{"a": 1, "b": 2},
				bs: []map[string]int{},
			},
			want: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "Two maps with no overlap",
			args: args{
				a:  map[string]int{"a": 1},
				bs: []map[string]int{{"b": 2}},
			},
			want: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "Two maps with overlap",
			args: args{
				a:  map[string]int{"a": 1},
				bs: []map[string]int{{"a": 2, "b": 3}},
			},
			want: map[string]int{"a": 2, "b": 3},
		},
		{
			name: "Multiple maps",
			args: args{
				a:  map[string]int{"a": 1},
				bs: []map[string]int{{"b": 2}, {"c": 3}},
			},
			want: map[string]int{"a": 1, "b": 2, "c": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeMap(tt.args.a, tt.args.bs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcat_Union(t *testing.T) {
	type testCase struct {
		name string
		a, b []int
		want []int
	}
	tests := []testCase{
		{
			name: "disjoint sets",
			a:    []int{1, 2, 3},
			b:    []int{4, 5, 6},
			want: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name: "overlapping sets",
			a:    []int{1, 2, 3},
			b:    []int{3, 4, 5},
			want: []int{1, 2, 3, 4, 5},
		},
		{
			name: "identical sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: []int{1, 2, 3},
		},
		{
			name: "one empty",
			a:    []int{},
			b:    []int{1, 2, 3},
			want: []int{1, 2, 3},
		},
		{
			name: "both empty",
			a:    []int{},
			b:    []int{},
			want: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Concat(tt.a, tt.b)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Concat(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestOverlap_Intersection(t *testing.T) {
	type testCase struct {
		name string
		a, b []int
		want []int
	}
	tests := []testCase{
		{
			name: "disjoint sets",
			a:    []int{1, 2, 3},
			b:    []int{4, 5, 6},
			want: []int{},
		},
		{
			name: "overlapping sets",
			a:    []int{1, 2, 3},
			b:    []int{3, 4, 5},
			want: []int{3},
		},
		{
			name: "identical sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: []int{1, 2, 3},
		},
		{
			name: "one empty",
			a:    []int{},
			b:    []int{1, 2, 3},
			want: []int{},
		},
		{
			name: "both empty",
			a:    []int{},
			b:    []int{},
			want: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Overlap(tt.a, tt.b)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Overlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDifference(t *testing.T) {
	type testCase struct {
		name string
		a, b []int
		want []int
	}
	tests := []testCase{
		{
			name: "disjoint sets",
			a:    []int{1, 2, 3},
			b:    []int{4, 5, 6},
			want: []int{1, 2, 3},
		},
		{
			name: "overlapping sets",
			a:    []int{1, 2, 3},
			b:    []int{3, 4, 5},
			want: []int{1, 2},
		},
		{
			name: "identical sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: []int{},
		},
		{
			name: "one empty",
			a:    []int{},
			b:    []int{1, 2, 3},
			want: []int{},
		},
		{
			name: "both empty",
			a:    []int{},
			b:    []int{},
			want: []int{},
		},
		{
			name: "b empty",
			a:    []int{1, 2, 3},
			b:    []int{},
			want: []int{1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Difference(tt.a, tt.b)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Difference(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}

			got = DifferenceCmp(tt.a, tt.b, func(a int) int {
				return a
			})
			if !slices.Equal(got, tt.want) {
				t.Errorf("Difference(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSymmetricDifference(t *testing.T) {
	type testCase struct {
		name string
		a, b []int
		want []int
	}
	tests := []testCase{
		{
			name: "disjoint sets",
			a:    []int{1, 2, 3},
			b:    []int{4, 5, 6},
			want: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name: "overlapping sets",
			a:    []int{1, 2, 3},
			b:    []int{3, 4, 5},
			want: []int{1, 2, 4, 5},
		},
		{
			name: "identical sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: []int{},
		},
		{
			name: "one empty",
			a:    []int{},
			b:    []int{1, 2, 3},
			want: []int{1, 2, 3},
		},
		{
			name: "both empty",
			a:    []int{},
			b:    []int{},
			want: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SymmetricDifference(tt.a, tt.b)
			// Order is not guaranteed, so sort before comparing
			slices.Sort(got)
			slices.Sort(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("SymmetricDifference(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	tests := []struct {
		name       string
		collection []int
		subset     []int
		want       int
		wantOk     bool
	}{
		{
			name:       "matching element found (subset in collection)",
			collection: []int{1, 2, 3},
			subset:     []int{3, 4, 5},
			want:       3,
			wantOk:     true,
		},
		{
			name:       "no matching element",
			collection: []int{1, 2},
			subset:     []int{3, 4},
			want:       0,
			wantOk:     false,
		},
		{
			name:       "multiple matches, should return first from subset",
			collection: []int{4, 2, 5},
			subset:     []int{2, 3, 4},
			want:       2,
			wantOk:     true,
		},
		{
			name:       "empty subset",
			collection: []int{1, 2},
			subset:     []int{},
			want:       0,
			wantOk:     false,
		},
		{
			name:       "empty collection",
			collection: []int{},
			subset:     []int{1, 2},
			want:       0,
			wantOk:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := MatchAny(tt.collection, tt.subset)
			if ok != tt.wantOk || got != tt.want {
				t.Errorf("%s: expected %v, %v; got %v, %v", tt.name, tt.want, tt.wantOk, got, ok)
			}
		})
	}
}

func TestMatchAnyStrings(t *testing.T) {
	tests := []struct {
		name       string
		collection []string
		subset     []string
		want       string
		wantOk     bool
	}{
		{
			name:       "matching string found",
			collection: []string{"baz", "bar"},
			subset:     []string{"foo", "bar"},
			want:       "bar",
			wantOk:     true,
		},
		{
			name:       "no matching string",
			collection: []string{"baz", "qux"},
			subset:     []string{"foo", "bar"},
			want:       "",
			wantOk:     false,
		},
		{
			name:       "multiple matches, should return first from subset",
			collection: []string{"foo", "bar", "baz"},
			subset:     []string{"boz", "foo", "bar"},
			want:       "foo",
			wantOk:     true,
		},
		{
			name:       "empty subset",
			collection: []string{"foo", "bar"},
			subset:     []string{},
			want:       "",
			wantOk:     false,
		},
		{
			name:       "empty collection",
			collection: []string{},
			subset:     []string{"foo", "bar"},
			want:       "",
			wantOk:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := MatchAny(tt.collection, tt.subset)
			if ok != tt.wantOk || got != tt.want {
				t.Errorf("%s: expected %v, %v; got %v, %v", tt.name, tt.want, tt.wantOk, got, ok)
			}
		})
	}
}

func TestRemoveIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		index    int
		expected []int
	}{
		{
			name:     "remove middle element",
			input:    []int{1, 2, 3, 4, 5},
			index:    2,
			expected: []int{1, 2, 4, 5},
		},
		{
			name:     "remove first element",
			input:    []int{1, 2, 3, 4, 5},
			index:    0,
			expected: []int{2, 3, 4, 5},
		},
		{
			name:     "remove last element",
			input:    []int{1, 2, 3, 4, 5},
			index:    4,
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "remove from single element slice",
			input:    []int{1},
			index:    0,
			expected: []int{},
		},
		{
			name:     "remove from empty slice",
			input:    []int{},
			index:    0,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveIndex(&tt.input, tt.index)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("RemoveIndex(%v, %d) = %v, want %v", tt.input, tt.index, tt.input, tt.expected)
			}
		})
	}
}

func TestClone(t *testing.T) {
	src := []int{1, 2, 3, 4, 5}
	cloned := Clone(src)

	if !reflect.DeepEqual(src, cloned) {
		t.Errorf("cloned slice does not match source: got %v, want %v", cloned, src)
	}

	// Modify the source and ensure the clone does not change
	src[0] = 99
	if cloned[0] == 99 {
		t.Errorf("modifying source affected clone: got %v, want %v", cloned[0], 1)
	}

	// Modify the clone and ensure the source does not change
	cloned[1] = 88
	if src[1] == 88 {
		t.Errorf("modifying clone affected source: got %v, want %v", src[1], 2)
	}
}

func TestDifferenceCmp(t *testing.T) {
	type Country struct {
		Name string
	}
	tests := []struct {
		name string
		a    []Country
		b    []Country
		want []Country
	}{
		{
			name: "Find out the missing countries",
			a:    []Country{{"Brazil"}, {"Germany"}, {"Peru"}, {"Bahrain"}, {"Tunisia"}},
			b:    []Country{{"Brazil"}, {"Peru"}, {"Bahrain"}},
			want: []Country{{"Germany"}, {"Tunisia"}},
		},
		{
			name: "Find out the missing countries2",
			a:    []Country{{"Aruba"}, {"Germany"}, {"Peru"}, {"Bahrain"}},
			b:    []Country{{"Brazil"}, {"Peru"}, {"Bahrain"}},
			want: []Country{{"Aruba"}, {"Germany"}},
		},
		{
			name: "disjoint sets",
			a:    []Country{{"Brazil"}, {"Germany"}, {"Peru"}},
			b:    []Country{{"Bahrain"}, {"Tunisia"}, {"Canada"}},
			want: []Country{{"Brazil"}, {"Germany"}, {"Peru"}},
		},
		{
			name: "identical sets",
			a:    []Country{{"Brazil"}, {"Germany"}, {"Peru"}},
			b:    []Country{{"Brazil"}, {"Germany"}, {"Peru"}},
			want: []Country{},
		},
		{
			name: "first empty",
			a:    []Country{},
			b:    []Country{{"Brazil"}, {"Germany"}},
			want: []Country{},
		},
		{
			name: "second empty",
			a:    []Country{{"Brazil"}, {"Germany"}},
			b:    []Country{},
			want: []Country{{"Brazil"}, {"Germany"}},
		},
		{
			name: "both empty",
			a:    []Country{},
			b:    []Country{},
			want: []Country{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DifferenceCmp(tt.a, tt.b, func(c Country) string {
				return c.Name
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DifferenceCmp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDifferenceCmpInt(t *testing.T) {
	type testCase struct {
		name string
		a    []int
		b    []int
		want []int
	}
	tests := []testCase{
		{
			name: "Find out the missing numbers",
			a:    []int{1, 2, 3, 8, 9},
			b:    []int{1, 2, 8},
			want: []int{3, 9},
		},
		{
			name: "disjoint sets",
			a:    []int{1, 2, 3},
			b:    []int{4, 5, 6},
			want: []int{1, 2, 3},
		},
		{
			name: "overlapping sets",
			a:    []int{1, 2, 3},
			b:    []int{3, 4, 5},
			want: []int{1, 2},
		},
		{
			name: "identical sets",
			a:    []int{1, 2, 3},
			b:    []int{1, 2, 3},
			want: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DifferenceCmp(tt.a, tt.b, func(i int) int {
				return i
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DifferenceCmp() = %v, want %v", got, tt.want)
			}
		})
	}
}
