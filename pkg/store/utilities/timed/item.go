package timed

import "time"

// CacheItem represents a value stored in the Timed along with
// the Unix timestamp (in seconds) indicating when it was last set or accessed.
// Accessing the cache entry resets its timestamp.
type CacheItem[V any] struct {
	Value     V
	Timestamp int64 // Unix timestamp in seconds
}

func NewCacheItem[V any](value V) CacheItem[V] {
	return CacheItem[V]{
		Value:     value,
		Timestamp: time.Now().Unix(),
	}
}

func (ci *CacheItem[V]) IsExpired(maxAge time.Duration) bool {
	return time.Now().Unix()-ci.Timestamp >= int64(maxAge.Seconds())
}

func (ci *CacheItem[V]) RefreshTimestamp() {
	ci.Timestamp = time.Now().Unix()
}
