package store

import (
	"reflect"
	"testing"
)

func TestKeys(t *testing.T) {
	tests := []struct {
		name     string
		items    *ListItems[string, int]
		expected []string
	}{
		{
			name:     "nil items",
			items:    nil,
			expected: nil,
		},
		{
			name:     "empty items",
			items:    &ListItems[string, int]{},
			expected: []string{},
		},
		{
			name: "single item",
			items: &ListItems[string, int]{
				{Key: "key1", Value: 100},
			},
			expected: []string{"key1"},
		},
		{
			name: "multiple items",
			items: &ListItems[string, int]{
				{Key: "key1", Value: 100},
				{Key: "key2", Value: 200},
				{Key: "key3", Value: 300},
			},
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name: "items with duplicate values but unique keys",
			items: &ListItems[string, int]{
				{Key: "keyA", Value: 100},
				{Key: "keyB", Value: 100},
				{Key: "keyC", Value: 200},
			},
			expected: []string{"keyA", "keyB", "keyC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.items.Keys()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Keys() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestKeys_DifferentTypes(t *testing.T) {
	t.Run("int keys with string values", func(t *testing.T) {
		items := &ListItems[int, string]{
			{Key: 1, Value: "value1"},
			{Key: 2, Value: "value2"},
			{Key: 3, Value: "value3"},
		}
		expected := []int{1, 2, 3}
		result := items.Keys()
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Keys() = %v, want %v", result, expected)
		}
	})

	t.Run("string keys with string values", func(t *testing.T) {
		items := &ListItems[string, string]{
			{Key: "alpha", Value: "first"},
			{Key: "beta", Value: "second"},
			{Key: "gamma", Value: "third"},
		}
		expected := []string{"alpha", "beta", "gamma"}
		result := items.Keys()
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Keys() = %v, want %v", result, expected)
		}
	})
}

func TestAsMap(t *testing.T) {
	tests := []struct {
		name     string
		items    *ListItems[string, int]
		expected map[string]int
	}{
		{
			name:     "nil items",
			items:    nil,
			expected: map[string]int{},
		},
		{
			name:     "empty items",
			items:    &ListItems[string, int]{},
			expected: map[string]int{},
		},
		{
			name: "single item",
			items: &ListItems[string, int]{
				{Key: "key1", Value: 100},
			},
			expected: map[string]int{"key1": 100},
		},
		{
			name: "multiple items",
			items: &ListItems[string, int]{
				{Key: "key1", Value: 100},
				{Key: "key2", Value: 200},
				{Key: "key3", Value: 300},
			},
			expected: map[string]int{"key1": 100, "key2": 200, "key3": 300},
		},
		{
			name: "items with zero values",
			items: &ListItems[string, int]{
				{Key: "zero", Value: 0},
				{Key: "nonzero", Value: 42},
			},
			expected: map[string]int{"zero": 0, "nonzero": 42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.items.AsMap()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("AsMap() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAsMap_DifferentTypes(t *testing.T) {
	t.Run("int keys with string values", func(t *testing.T) {
		items := &ListItems[int, string]{
			{Key: 1, Value: "value1"},
			{Key: 2, Value: "value2"},
			{Key: 3, Value: "value3"},
		}
		expected := map[int]string{1: "value1", 2: "value2", 3: "value3"}
		result := items.AsMap()
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("AsMap() = %v, want %v", result, expected)
		}
	})

	t.Run("string keys with float values", func(t *testing.T) {
		items := &ListItems[string, float64]{
			{Key: "pi", Value: 3.14159},
			{Key: "e", Value: 2.71828},
		}
		expected := map[string]float64{"pi": 3.14159, "e": 2.71828}
		result := items.AsMap()
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("AsMap() = %v, want %v", result, expected)
		}
	})

	t.Run("int keys with struct values", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		items := &ListItems[int, Person]{
			{Key: 1, Value: Person{Name: "Alice", Age: 30}},
			{Key: 2, Value: Person{Name: "Bob", Age: 25}},
		}
		expected := map[int]Person{
			1: {Name: "Alice", Age: 30},
			2: {Name: "Bob", Age: 25},
		}
		result := items.AsMap()
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("AsMap() = %v, want %v", result, expected)
		}
	})
}

func TestKeys_PreservesOrder(t *testing.T) {
	// Keys should preserve insertion order
	items := &ListItems[int, string]{
		{Key: 5, Value: "five"},
		{Key: 2, Value: "two"},
		{Key: 8, Value: "eight"},
		{Key: 1, Value: "one"},
	}
	result := items.Keys()
	expected := []int{5, 2, 8, 1}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Keys() = %v, want %v (order should be preserved)", result, expected)
	}
}

func TestAsMap_ContentPreservation(t *testing.T) {
	// Verify that all key-value pairs are correctly transferred to the map
	items := &ListItems[string, int]{
		{Key: "a", Value: 1},
		{Key: "b", Value: 2},
		{Key: "c", Value: 3},
		{Key: "d", Value: 4},
		{Key: "e", Value: 5},
	}

	result := items.AsMap()

	// Check all keys are present
	for _, item := range *items {
		if value, ok := result[item.Key]; !ok {
			t.Errorf("Key %v not found in map", item.Key)
		} else if value != item.Value {
			t.Errorf("Map[%v] = %v, want %v", item.Key, value, item.Value)
		}
	}

	// Check map has correct length
	if len(result) != len(*items) {
		t.Errorf("Map length = %d, want %d", len(result), len(*items))
	}
}
