package slices

import (
	"slices"
	"sync"
)

type ThreadSafe[T comparable] struct {
	sync.RWMutex
	data []T
}

func NewSlice[T comparable](data ...T) *ThreadSafe[T] {
	var newslice []T
	if data == nil {
		newslice = make([]T, 0)
	} else {
		newslice = data
	}
	return &ThreadSafe[T]{
		data:    newslice,
		RWMutex: sync.RWMutex{},
	}
}
func NewSliceOf[T comparable](data []T) *ThreadSafe[T] {
	return NewSlice(data...)
}

func (ts *ThreadSafe[T]) Add(value T) {
	ts.Lock()
	defer ts.Unlock()
	ts.data = append(ts.data, value)
}

func (ts *ThreadSafe[T]) AddIfNotExists(value T) bool {
	ts.Lock()
	defer ts.Unlock()
	if !slices.Contains(ts.data, value) {
		ts.data = append(ts.data, value)
		return true
	}
	return false
}

func (ts *ThreadSafe[T]) Contains(value T) bool {
	ts.RLock()
	defer ts.RUnlock()
	return slices.Contains(ts.data, value)
}

func (ts *ThreadSafe[T]) Values() []T {
	ts.RLock()
	defer ts.RUnlock()
	return ts.data
}

func (ts *ThreadSafe[T]) Get(index int) (T, bool) {
	ts.RLock()
	defer ts.RUnlock()
	var zero T
	if index < 0 || index >= len(ts.data) {
		return zero, false
	}
	return ts.data[index], true
}

func (ts *ThreadSafe[T]) Delete(index int) bool {
	ts.Lock()
	defer ts.Unlock()
	if index < 0 || index >= len(ts.data) {
		return false
	}
	ts.data = append(ts.data[:index], ts.data[index+1:]...)
	return true
}

func (ts *ThreadSafe[T]) DeleteByValue(value T) bool {
	ts.Lock()
	defer ts.Unlock()
	var found bool
	ts.data = slices.DeleteFunc(ts.data, func(v T) bool {
		if v == value {
			found = true
			return true
		}
		return false
	})
	return found
}

func (ts *ThreadSafe[T]) Update(index int, value T) bool {
	ts.Lock()
	defer ts.Unlock()
	if index < 0 || index >= len(ts.data) {
		return false
	}
	ts.data[index] = value
	return true
}

func (ts *ThreadSafe[T]) List(fn func(value T) bool) {
	ts.RLock()
	defer ts.RUnlock()
	for _, value := range ts.data {
		if !fn(value) {
			break
		}
	}
}

func (ts *ThreadSafe[T]) Clear() {
	ts.Lock()
	defer ts.Unlock()
	ts.data = make([]T, 0)
}

func (ts *ThreadSafe[T]) Length() int {
	ts.RLock()
	defer ts.RUnlock()
	return len(ts.data)
}

func (ts *ThreadSafe[T]) SortFunc(cmp func(a T, b T) int) {
	ts.Lock()
	defer ts.Unlock()
	slices.SortFunc(ts.data, cmp)
}
