package engine

// Set is a generic mathematical set backed by a Go map. It provides O(1)
// membership testing, which the C Quake engine achieved with linear array
// scans or hash tables.
//
// In Go, the idiomatic "set" pattern is map[T]struct{}, but writing
// `if _, ok := m[x]; ok` everywhere is verbose and error-prone. This type
// wraps that pattern with clear semantics.
//
// Example usage:
//
//	stringFields := engine.NewSet("classname", "targetname", "model")
//	if stringFields.Has("classname") { ... }
type Set[T comparable] struct {
	m map[T]struct{}
}

// NewSet creates a Set pre-populated with the given elements.
func NewSet[T comparable](elements ...T) Set[T] {
	s := Set[T]{m: make(map[T]struct{}, len(elements))}
	for _, e := range elements {
		s.m[e] = struct{}{}
	}
	return s
}

// Add inserts an element into the set. If the element already exists, this is
// a no-op.
func (s *Set[T]) Add(element T) {
	s.m[element] = struct{}{}
}

// Has returns true if the element is in the set.
func (s Set[T]) Has(element T) bool {
	_, ok := s.m[element]
	return ok
}

// Remove deletes an element from the set. It is a no-op if absent.
func (s *Set[T]) Remove(element T) {
	delete(s.m, element)
}

// Len returns the number of elements in the set.
func (s Set[T]) Len() int {
	return len(s.m)
}

// Slice returns all elements as a slice in arbitrary order. Useful for
// iteration when order doesn't matter.
func (s Set[T]) Slice() []T {
	result := make([]T, 0, len(s.m))
	for k := range s.m {
		result = append(result, k)
	}
	return result
}

// Range calls fn for each element. If fn returns false, iteration stops.
func (s Set[T]) Range(fn func(T) bool) {
	for k := range s.m {
		if !fn(k) {
			break
		}
	}
}
