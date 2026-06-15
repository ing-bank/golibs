package slices

import (
	"slices"
	"testing"
)

func TestThreadSafeString_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		data  []string
		check string
	}{
		{"Add and Contains", []string{"apple", "banana"}, "apple"},
		{"Add and Contains", []string{"kiwi", "melon"}, "kiwi"},
		{"Add and Contains", []string{"x", "y", "z"}, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := NewSlice[string]()
			for _, v := range tt.data {
				ts.Add(v)
			}
			if !ts.Contains(tt.check) {
				t.Errorf("Should contain %s, got %v", tt.check, ts.Values())
			}
			if ts.AddIfNotExists(tt.check) {
				t.Errorf("Should not add duplicate %s", tt.check)
			}
			if !ts.DeleteByValue(tt.check) {
				t.Errorf("DeleteByValue failed for %s", tt.check)
			}
			if ts.Delete(0) && ts.Contains(tt.check) {
				t.Errorf("Delete should not succeed for index 0 after deletion")
			}
			if ts.Update(0, "orange") {
				item, found := ts.Get(0)
				if !found || item != "orange" {
					t.Errorf("Update failed for index 0, expected 'orange', got %v", item)
				}
			}
			if ts.Update(100, "fail") {
				t.Error("Update should fail for out-of-bounds index")
			}
		})
	}
}

func TestThreadSafe_DeleteByValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		data       []string
		deleteItem string
		deleted    bool
		want       []string
	}{
		{"Contain all Values", []string{"apple", "banana"}, "apple", true, []string{"banana"}},
		{"Delete Non-existent", []string{"a", "b", "c"}, "d", false, []string{"a", "b", "c"}},
		{"Delete All", []string{"x", "y", "z"}, "x", true, []string{"y", "z"}},
		{"Delete Last", []string{"last"}, "last", true, []string{}},
		{"Delete Empty Slice", []string{}, "any", false, []string{}},
		{"Delete Single Item", []string{"only"}, "only", true, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := NewSliceOf(tt.data)
			if deleted := ts.DeleteByValue(tt.deleteItem); deleted != tt.deleted {
				t.Errorf("DeleteByValue failed for %s", tt.deleteItem)
			}
			if !slices.Equal(ts.Values(), tt.want) {
				t.Errorf("Values mismatch, expected %v, got %v", tt.want, ts.Values())
			}
		})
	}
}

func TestThreadSafeStruct_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values []Person
		check  Person
	}{
		{"Add and Contains", []Person{{"Alice", 30}, {"Bob", 25}}, Person{"Max", 30}},
		{"Add and Contains", []Person{{"Charlie", 40}, {"Dana", 22}}, Person{"Mary", 22}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := NewSlice[Person]()
			for _, v := range tt.values {
				ts.Add(v)
			}
			if ts.Contains(tt.check) {
				t.Errorf("Should contain %+v", tt.check)
			}
			if !ts.AddIfNotExists(tt.check) {
				t.Errorf("Should not add duplicate %+v", tt.check)
			}
			if !ts.Contains(tt.check) {
				t.Errorf("Should contain %+v", tt.check)
			}
			if !ts.DeleteByValue(tt.check) {
				t.Errorf("DeleteByValue failed for %+v", tt.check)
			}
		})
	}
}

func TestThreadSafeInt_GetDeleteUpdateList(t *testing.T) {
	t.Parallel()
	ts := NewSlice[int]()
	ts.Add(10)
	ts.Add(20)
	ts.Add(30)

	v, ok := ts.Get(2)
	if !ok || v != 30 {
		t.Errorf("Get failed: got %v, ok=%v", v, ok)
	}
	_, ok = ts.Get(-1)
	if ok {
		t.Error("Get should fail for negative index")
	}
	if !ts.Delete(0) {
		t.Error("Delete failed for index 0")
	}
	if ts.Delete(100) {
		t.Error("Delete should fail for out-of-bounds index")
	}
	if ts.Contains(10) {
		t.Error("Should not contain 10 after delete")
	}
	if !ts.Update(0, 99) {
		t.Error("Update failed for index 0")
	}
	v, ok = ts.Get(0)
	if !ok || v != 99 {
		t.Error("Update did not set correct value")
	}
	if ts.Update(-1, 123) {
		t.Error("Update should fail for negative index")
	}

	values := make([]int, 0, len(ts.Values()))
	ts.List(func(val int) bool {
		values = append(values, val)
		return true
	})
	if len(values) != len(ts.Values()) {
		t.Error("List did not iterate all values")
	}
}

func TestThreadSafe_ClearAndLength(t *testing.T) {
	t.Parallel()
	ts := NewSlice(1, 2, 3)
	if ts.Length() != 3 {
		t.Errorf("Expected length 3, got %d", ts.Length())
	}
	ts.Clear()
	if ts.Length() != 0 {
		t.Errorf("Expected length 0 after Clear, got %d", ts.Length())
	}
	if len(ts.Values()) != 0 {
		t.Errorf("Expected Values to be empty after Clear, got %v", ts.Values())
	}
}
