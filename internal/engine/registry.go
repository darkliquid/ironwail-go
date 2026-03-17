package engine

import (
	"fmt"
	"sync"
)

// Registry is a generic write-once, read-many lookup table. It is designed for
// data that is built during initialization (e.g., entity field offsets, builtin
// function tables, subsystem registrations) and then read repeatedly at
// runtime without modification.
//
// Unlike Cache, Registry enforces uniqueness: registering the same key twice
// panics, catching wiring bugs early. An optional Freeze method makes the
// registry permanently read-only, which is useful for data that must not
// change after startup (like the QuakeC field layout).
//
// Example usage:
//
//	fields := engine.NewRegistry[string, int]()
//	fields.Register("origin", 0)
//	fields.Register("velocity", 12)
//	fields.Freeze()
//	offset, ok := fields.Lookup("origin")
type Registry[K comparable, V any] struct {
	mu     sync.RWMutex
	items  map[K]V
	frozen bool
}

// NewRegistry creates an empty, unfrozen Registry.
func NewRegistry[K comparable, V any]() *Registry[K, V] {
	return &Registry[K, V]{items: make(map[K]V)}
}

// Register adds a key/value pair. It panics if the key already exists or if
// the registry has been frozen. The panic is intentional: duplicate
// registrations indicate a programming error that should be caught immediately
// rather than silently overwriting data.
func (r *Registry[K, V]) Register(key K, value V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		panic(fmt.Sprintf("engine.Registry: attempt to register %v after Freeze", key))
	}
	if _, exists := r.items[key]; exists {
		panic(fmt.Sprintf("engine.Registry: duplicate registration for key %v", key))
	}
	r.items[key] = value
}

// Lookup retrieves a value by key. The second return value indicates whether
// the key was found.
func (r *Registry[K, V]) Lookup(key K) (V, bool) {
	r.mu.RLock()
	v, ok := r.items[key]
	r.mu.RUnlock()
	return v, ok
}

// MustLookup retrieves a value by key, panicking if the key is not found.
// Use this when the caller knows the key must exist (e.g., looking up a
// QuakeC builtin by number after validation).
func (r *Registry[K, V]) MustLookup(key K) V {
	v, ok := r.Lookup(key)
	if !ok {
		panic(fmt.Sprintf("engine.Registry: key %v not found", key))
	}
	return v
}

// Freeze makes the registry permanently read-only. Any subsequent Register
// call will panic. This is typically called at the end of initialization to
// guarantee that runtime lookups see a stable dataset.
func (r *Registry[K, V]) Freeze() {
	r.mu.Lock()
	r.frozen = true
	r.mu.Unlock()
}

// Len returns the number of registered entries.
func (r *Registry[K, V]) Len() int {
	r.mu.RLock()
	n := len(r.items)
	r.mu.RUnlock()
	return n
}

// Range calls fn for each key/value pair. If fn returns false, iteration
// stops. The registry is read-locked during iteration.
func (r *Registry[K, V]) Range(fn func(K, V) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for k, v := range r.items {
		if !fn(k, v) {
			break
		}
	}
}
