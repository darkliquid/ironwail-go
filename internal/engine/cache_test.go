package engine

import (
	"sync"
	"testing"
)

func TestCache_GetSet(t *testing.T) {
	c := NewCache[string, int]()
	c.Set("a", 1)
	c.Set("b", 2)

	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	v, ok = c.Get("b")
	if !ok || v != 2 {
		t.Fatalf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	_, ok = c.Get("c")
	if ok {
		t.Fatal("Get(c) should return false for missing key")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := NewCache[string, int]()

	// First call stores the value
	v, existed := c.GetOrSet("x", 42)
	if existed || v != 42 {
		t.Fatalf("GetOrSet(x, 42) = %d, %v; want 42, false", v, existed)
	}

	// Second call returns existing
	v, existed = c.GetOrSet("x", 99)
	if !existed || v != 42 {
		t.Fatalf("GetOrSet(x, 99) = %d, %v; want 42, true", v, existed)
	}
}

func TestCache_Delete(t *testing.T) {
	c := NewCache[string, int]()
	c.Set("a", 1)
	c.Delete("a")
	_, ok := c.Get("a")
	if ok {
		t.Fatal("Get after Delete should return false")
	}
	// Delete of missing key is no-op
	c.Delete("nonexistent")
}

func TestCache_Clear(t *testing.T) {
	c := NewCache[string, int]()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("Len after Clear = %d; want 0", c.Len())
	}
}

func TestCache_Range(t *testing.T) {
	c := NewCache[string, int]()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	sum := 0
	c.Range(func(_ string, v int) bool {
		sum += v
		return true
	})
	if sum != 6 {
		t.Fatalf("Range sum = %d; want 6", sum)
	}

	// Test early termination
	count := 0
	c.Range(func(_ string, _ int) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("Range with early stop: count = %d; want 1", count)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache[int, int]()
	var wg sync.WaitGroup

	// 10 writers + 10 readers concurrently
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Set(n*100+j, j)
			}
		}(i)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Get(n*100 + j)
			}
		}(i)
	}
	wg.Wait()
}
