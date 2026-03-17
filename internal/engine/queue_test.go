package engine

import (
	"testing"
)

func TestQueue_PushPop(t *testing.T) {
	q := NewQueue[int](4)
	q.Push(1)
	q.Push(2)
	q.Push(3)

	v, ok := q.Pop()
	if !ok || v != 1 {
		t.Fatalf("Pop = %d, %v; want 1, true", v, ok)
	}
	v, ok = q.Pop()
	if !ok || v != 2 {
		t.Fatalf("Pop = %d, %v; want 2, true", v, ok)
	}
	v, ok = q.Pop()
	if !ok || v != 3 {
		t.Fatalf("Pop = %d, %v; want 3, true", v, ok)
	}
	_, ok = q.Pop()
	if ok {
		t.Fatal("Pop on empty queue should return false")
	}
}

func TestQueue_Peek(t *testing.T) {
	q := NewQueue[string](4)
	_, ok := q.Peek()
	if ok {
		t.Fatal("Peek on empty queue should return false")
	}
	q.Push("hello")
	v, ok := q.Peek()
	if !ok || v != "hello" {
		t.Fatalf("Peek = %q, %v; want hello, true", v, ok)
	}
	if q.Len() != 1 {
		t.Fatal("Peek should not remove elements")
	}
}

func TestQueue_Grow(t *testing.T) {
	q := NewQueue[int](4)
	// Push more than initial capacity to trigger grow
	for i := 0; i < 20; i++ {
		q.Push(i)
	}
	if q.Len() != 20 {
		t.Fatalf("Len = %d; want 20", q.Len())
	}
	for i := 0; i < 20; i++ {
		v, ok := q.Pop()
		if !ok || v != i {
			t.Fatalf("Pop[%d] = %d, %v; want %d, true", i, v, ok, i)
		}
	}
}

func TestQueue_WrapAround(t *testing.T) {
	q := NewQueue[int](4)
	// Fill and partially drain to move head forward
	q.Push(1)
	q.Push(2)
	q.Pop()
	q.Pop()
	// Now head is at 2, push wrapping around
	q.Push(10)
	q.Push(20)
	q.Push(30)
	q.Push(40)
	// This should trigger grow with wrap-around copy
	q.Push(50)

	for _, want := range []int{10, 20, 30, 40, 50} {
		v, ok := q.Pop()
		if !ok || v != want {
			t.Fatalf("Pop = %d, %v; want %d, true", v, ok, want)
		}
	}
}

func TestQueue_Clear(t *testing.T) {
	q := NewQueue[int](4)
	q.Push(1)
	q.Push(2)
	q.Clear()
	if q.Len() != 0 {
		t.Fatalf("Len after Clear = %d; want 0", q.Len())
	}
	_, ok := q.Pop()
	if ok {
		t.Fatal("Pop after Clear should return false")
	}
}
