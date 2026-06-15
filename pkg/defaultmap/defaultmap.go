// Package defaultmap provides the DefaultMap type. It's key feature is the ability to describe how to handle a key
// collision via a user-provided function, once. After the collision function is provided the DefaultMap can be used
// without further checks for key presence, allowing the user to use DefaultMap without further error or conflict handling.
//
// DefaultMap[K, V] is a map-like structure that enables flexible handling of key collisions through user-defined resolution functions.
// It is particularly useful in scenarios where merging or updating map entries requires custom logic beyond simple overwrites.
//
// Key Features:
//   - Generic support for any comparable key type K and any value type V.
//   - Set: Insert or overwrite a value for a given key.
//   - Update: Insert or update a value for a given key, using a collision function to resolve conflicts.
//   - Merge: Combine two DefaultMaps, resolving key collisions with a user-provided function.
//
// Example usage:
//
//	// Create a DefaultMap with string keys and int values
//	m := DefaultMap[string, int]{}
//	// Set a value
//	m.Set("foo", 1)
//	m.Set("bar", 2)
//	// Read a value
//	val := m["foo"]
//	// Update with custom collision logic
//	m.Update("foo", 2, func(old, new int) int { return old + new }) // foo = 3
//	// Delete a value
//	delete(m, "foo")
//	// Merge another map, summing values on collision
//	other := DefaultMap[string, int]{"foo": 5, "bar": 7}
//	m.Merge(other, func(old, new int) int { return old + new }) // foo = 5, bar = 9
package defaultmap

// DefaultMap is a generic map type that provides Set, Update, and Merge methods. Its
// core utility is the use of the collision function, which allows iterative updates
// to the map without first checking key presence, as showcased in the example. For
// other functions like Get or Delete, the built-in map functions can be used directly.
type DefaultMap[K comparable, V any] map[K]V

// Set sets the value for the given key, always overriding any existing value.
func (d DefaultMap[K, V]) Set(key K, new V) {
	d.Update(key, new, func(old, new V) V {
		return new
	})
}

// Update sets the value for the given key. If the key exists, the collision function is called
// with the old and new values to determine the stored value. If the key does not exist, the new value is set.
func (d DefaultMap[K, V]) Update(key K, new V, collision func(old, new V) V) {
	current, ok := d[key]
	if !ok {
		d[key] = new
	} else {
		d[key] = collision(current, new)
	}
}

// Merge merges 'other' into 'd'. Colliding keys will be resolved via the collision function.
func (d DefaultMap[K, V]) Merge(other DefaultMap[K, V], collision func(old, new V) V) {
	for k, v := range other {
		d.Update(k, v, collision)
	}
}
