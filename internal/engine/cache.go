package engine

import "sync"

// Cache is a generic, thread-safe key/value store designed for runtime object
// caching. It wraps a map with a sync.RWMutex so that multiple goroutines can
// read concurrently while writes are serialized.
//
// In a game engine, caches appear everywhere: model caches (avoid reloading
// the same .mdl from disk), sound caches (keep decoded PCM in memory), texture
// caches (avoid re-uploading to the GPU). The C Quake engine used global
// arrays with linear search; this generic version gives O(1) lookup with type
// safety.
//
// Example usage:
//
//	models := engine.NewCache[string, *model.Model]()
//	models.Set("progs/player.mdl", playerModel)
//	m, ok := models.Get("progs/player.mdl")
type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewCache creates an empty Cache ready for use.
func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{items: make(map[K]V)}
}

// Get retrieves a value by key. The second return value indicates whether the
// key was present. Multiple goroutines can call Get concurrently.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	v, ok := c.items[key]
	c.mu.RUnlock()
	return v, ok
}

// Set stores a key/value pair, overwriting any previous value for that key.
func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.items[key] = value
	c.mu.Unlock()
}

// GetOrSet returns the existing value for key if present. Otherwise it stores
// and returns the provided value. The boolean reports whether the value was
// already present (true) or newly stored (false). This is useful for
// lazy-loading patterns where the first caller populates the cache.
func (c *Cache[K, V]) GetOrSet(key K, value V) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.items[key]; ok {
		return v, true
	}
	c.items[key] = value
	return value, false
}

// Delete removes a key from the cache. It is a no-op if the key is absent.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Clear removes all entries from the cache. This is typically called on map
// change or subsystem reset to release references to stale objects.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	c.items = make(map[K]V)
	c.mu.Unlock()
}

// Len returns the number of entries currently in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Range calls fn for each key/value pair in the cache. If fn returns false,
// iteration stops. The cache is read-locked for the duration of the iteration,
// so fn must not call other Cache methods (deadlock).
func (c *Cache[K, V]) Range(fn func(K, V) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.items {
		if !fn(k, v) {
			break
		}
	}
}
