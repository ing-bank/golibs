// Package slices offers generic utilities for slices

// TODO: some of these functions are now part of Go STDLIB, and can be removed

package slices

import (
	"cmp"
	"maps"
	"slices"
)

// Unique returns a new slice containing all the unique entries.
// The original slice, 's', is not modified. When comparing pointer
// values use UniqueCmp to peek data as pointer values are always unique.
func Unique[S ~[]E, E comparable](s S) S {
	return UniqueCmp(s, func(e E) E {
		return e
	})
}

// UniqueCmp works the same as Unique, but allows a serialize function to
// denote what makes an entry truly unique, or to peek pointer values.
func UniqueCmp[S ~[]E, E any, T comparable](s S, serialize func(E) T) S {
	exists := map[T]E{}
	uniques := make(S, 0)

	for _, item := range s {
		serial := serialize(item)

		if _, ok := exists[serial]; ok {
			continue
		}
		exists[serial] = item
		uniques = append(uniques, item) // Do this in same loop to preserve order, and it's efficient
	}

	return uniques
}

// Transform takes a slice and converts each item to a different data type
func Transform[S ~[]E, E any, B any](s S, trans func(item E) B) []B {
	transformations := make([]B, len(s))
	for i, item := range s {
		transformations[i] = trans(item)
	}
	return transformations
}

// MatchAny returns the first matching entry from 'subset' that exists in 'collection', or false if no match was found.
func MatchAny[S ~[]E, E comparable](collection, subset S) (E, bool) {
	for _, v := range subset {
		if slices.Contains(collection, v) {
			return v, true
		}
	}
	return *new(E), false
}

// FlatMap maps each element of the slice to a slice of another type and flattens the result into a single slice.
func FlatMap[S ~[]A, A any, B any](s S, f func(A) []B) []B {
	var result []B
	for _, v := range s {
		result = append(result, f(v)...)
	}
	return result
}

// FilterEmpty removes empty entries from a slice
func FilterEmpty[S ~[]E, E *P, P any](s S) []E {
	return Filter(s, func(item E) bool {
		return item != nil
	})
}

// Filter creates a new slice with elements from s that should be kept
func Filter[S ~[]E, E any](s S, keep func(item E) bool) []E {
	filtered := make([]E, 0)
	for _, item := range s {
		if keep(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// Map transforms a slice of items to a map
func Map[S ~[]E, E any, B comparable](s S, key func(item E) B) map[B]E {
	mapped := make(map[B]E)
	for _, item := range s {
		mapped[key(item)] = item
	}
	return mapped
}

// MapValues returns the values of the given map sorted based on the keys
func MapValues[S ~map[K]V, K cmp.Ordered, V any](s S) []V {
	_, values := MapItems(s)
	return values
}

func MapItems[S ~map[K]V, K cmp.Ordered, V any](s S) ([]K, []V) {
	keys := make([]K, len(s))
	items := make([]V, len(s))

	// Sort map keys
	i := 0
	for key, _ := range s {
		keys[i] = key
		i += 1
	}
	slices.Sort(keys)

	// Build values in sorted order
	for i, key := range keys {
		items[i] = s[key]
		i += 1
	}

	return keys, items
}

// MergeMap returns a new map with keys and values from the provided maps. Conflicted keys are overridden, in order.
func MergeMap[S ~map[K]V, K cmp.Ordered, V any](a S, bs ...S) map[K]V {
	c := make(map[K]V)
	maps.Copy(c, a)
	for _, b := range bs {
		maps.Copy(c, b)
	}
	return c
}

func ReMap[S ~map[K]V, K, T cmp.Ordered, V any](s S, key func(item V) T) map[T]V {
	return Map(MapValues(s), key)
}

// Count counts occurrences of indexes
func Count[S ~[]E, E comparable](s S) map[E]int {
	counts := make(map[E]int)
	for _, item := range s {
		if count, ok := counts[item]; ok {
			counts[item] = count + 1
		} else {
			counts[item] = 1
		}
	}
	return counts
}

// IsSubset returns true if all items in 'a' are present in 'b'
func IsSubset[S ~[]E, E comparable](a S, b S) bool {
	for _, item := range a {
		if !slices.Contains(b, item) {
			return false
		}
	}
	return true
}

// Contains returns true if 'v' is in 's'
func Contains[S ~[]E, E comparable](s S, v E) bool {
	return slices.Contains(s, v)
}

// Concat is similar to concat of the stdlib, but it only keeps unique entries.
// Also known as the union of two slices: A ∪ B
func Concat[S ~[]E, E comparable](s ...S) []E {
	return Unique(slices.Concat(s...))
}

// Overlap returns the intersection of two slices. A ∩ B
func Overlap[S ~[]E, E comparable](a S, b S) S {
	overlap := make(S, 0)
	for _, item := range a {
		if slices.Contains(b, item) {
			overlap = append(overlap, item)
		}
	}
	return overlap
}

// Difference returns the difference of two slices. A \ B or A - B.
// It returns a new slice containing elements that are in 'A' but not in 'B'.
func Difference[S ~[]E, E comparable](a S, b S) S {
	difference := make(S, 0)
	for _, item := range a {
		if !slices.Contains(b, item) {
			difference = append(difference, item)
		}
	}
	return difference
}

// DifferenceCmp returns the difference of two slices. A \ B or A - B.
// It returns a new slice containing elements that are in 'A' but not in 'B'.
func DifferenceCmp[S ~[]T, T any, E comparable](a S, b S, comp func(T) E) S {
	var lookup = make(map[E]struct{}, len(a))
	result := make(S, 0, len(a))

	for _, v := range b {
		lookup[comp(v)] = struct{}{}
	}
	for _, v := range a {
		if _, ok := lookup[comp(v)]; !ok {
			result = append(result, v)
		}
	}
	return result
}

// SymmetricDifference returns the symmetric difference of two slices. A Δ B.
// It returns a new slice containing elements that are in either 'A' or 'B',
// but not in both.
func SymmetricDifference[S ~[]E, E comparable](a S, b S) S {
	differenceA := Difference(a, b)
	differenceB := Difference(b, a)
	return slices.Concat(differenceA, differenceB)
}

// SymmetricDifferenceCmp returns the symmetric difference of two slices. A Δ B.
// It returns a new slice containing elements that are in either 'A' or 'B',
// but not in both.
func SymmetricDifferenceCmp[S ~[]T, T any, E comparable](a S, b S, comp func(T) E) S {
	differenceA := DifferenceCmp(a, b, comp)
	differenceB := DifferenceCmp(b, a, comp)
	return slices.Concat(differenceA, differenceB)
}

func RemoveIndex[S ~[]E, E any](s *S, index int) {
	if index < 0 || index >= len(*s) {
		return
	}
	*s = append((*s)[:index], (*s)[index+1:]...)
}

// Clone returns a copy of the provided slice.
func Clone[S ~[]E, E any](s S) S {
	cloned := make(S, len(s))
	copy(cloned, s)
	return cloned
}
