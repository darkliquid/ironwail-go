package engine

import (
	"testing"
)

func TestRegistry_RegisterLookup(t *testing.T) {
	r := NewRegistry[string, int]()
	r.Register("origin", 0)
	r.Register("velocity", 12)

	v, ok := r.Lookup("origin")
	if !ok || v != 0 {
		t.Fatalf("Lookup(origin) = %d, %v; want 0, true", v, ok)
	}
	v, ok = r.Lookup("velocity")
	if !ok || v != 12 {
		t.Fatalf("Lookup(velocity) = %d, %v; want 12, true", v, ok)
	}
	_, ok = r.Lookup("missing")
	if ok {
		t.Fatal("Lookup(missing) should return false")
	}
}

func TestRegistry_MustLookup(t *testing.T) {
	r := NewRegistry[string, int]()
	r.Register("x", 42)

	v := r.MustLookup("x")
	if v != 42 {
		t.Fatalf("MustLookup(x) = %d; want 42", v)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("MustLookup(missing) should panic")
		}
	}()
	r.MustLookup("missing")
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry[string, int]()
	r.Register("x", 1)

	defer func() {
		if recover() == nil {
			t.Fatal("duplicate Register should panic")
		}
	}()
	r.Register("x", 2)
}

func TestRegistry_Freeze(t *testing.T) {
	r := NewRegistry[string, int]()
	r.Register("x", 1)
	r.Freeze()

	// Lookup still works after freeze
	v, ok := r.Lookup("x")
	if !ok || v != 1 {
		t.Fatalf("Lookup after Freeze = %d, %v; want 1, true", v, ok)
	}

	// Register after freeze panics
	defer func() {
		if recover() == nil {
			t.Fatal("Register after Freeze should panic")
		}
	}()
	r.Register("y", 2)
}

func TestRegistry_Range(t *testing.T) {
	r := NewRegistry[string, int]()
	r.Register("a", 1)
	r.Register("b", 2)

	sum := 0
	r.Range(func(_ string, v int) bool {
		sum += v
		return true
	})
	if sum != 3 {
		t.Fatalf("Range sum = %d; want 3", sum)
	}
}
