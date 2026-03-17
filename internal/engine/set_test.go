package engine

import (
	"sort"
	"testing"
)

func TestSet_Basic(t *testing.T) {
	s := NewSet("a", "b", "c")
	if s.Len() != 3 {
		t.Fatalf("Len = %d; want 3", s.Len())
	}
	if !s.Has("a") || !s.Has("b") || !s.Has("c") {
		t.Fatal("Has returned false for inserted elements")
	}
	if s.Has("d") {
		t.Fatal("Has returned true for missing element")
	}
}

func TestSet_Add(t *testing.T) {
	s := NewSet[string]()
	s.Add("x")
	s.Add("x") // duplicate
	if s.Len() != 1 {
		t.Fatalf("Len after duplicate Add = %d; want 1", s.Len())
	}
}

func TestSet_Remove(t *testing.T) {
	s := NewSet("a", "b")
	s.Remove("a")
	if s.Has("a") {
		t.Fatal("Has returned true after Remove")
	}
	if s.Len() != 1 {
		t.Fatalf("Len after Remove = %d; want 1", s.Len())
	}
	s.Remove("nonexistent") // no-op
}

func TestSet_Slice(t *testing.T) {
	s := NewSet("c", "a", "b")
	sl := s.Slice()
	sort.Strings(sl)
	if len(sl) != 3 || sl[0] != "a" || sl[1] != "b" || sl[2] != "c" {
		t.Fatalf("Slice = %v; want [a b c]", sl)
	}
}

func TestSet_Range(t *testing.T) {
	s := NewSet(1, 2, 3, 4, 5)
	count := 0
	s.Range(func(_ int) bool {
		count++
		return count < 3
	})
	if count != 3 {
		t.Fatalf("Range with early stop: count = %d; want 3", count)
	}
}
