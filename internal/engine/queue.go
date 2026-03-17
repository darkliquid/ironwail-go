package engine

// Queue is a generic FIFO ring buffer. It stores elements in a contiguous
// slice and uses head/tail indices to avoid shifting elements on every
// dequeue — the same optimization used in the C Quake command buffer
// (Cbuf_AddText / Cbuf_Execute).
//
// When the buffer is full, Push doubles its capacity. This means Queue never
// drops elements, making it suitable for bursty workloads like console command
// batching where a config.cfg exec might enqueue hundreds of commands at once.
//
// Queue is NOT thread-safe. If concurrent access is needed, wrap it with a
// mutex or use channels instead.
//
// Example usage:
//
//	q := engine.NewQueue[string](16)
//	q.Push("map start")
//	q.Push("god")
//	cmd, ok := q.Pop() // "map start", true
type Queue[T any] struct {
	buf  []T
	head int // index of next element to dequeue
	tail int // index of next free slot
	len  int // number of elements currently queued
}

// NewQueue creates a Queue with the given initial capacity. The capacity is a
// hint — the queue grows automatically if more elements are pushed.
func NewQueue[T any](capacity int) *Queue[T] {
	if capacity < 4 {
		capacity = 4
	}
	return &Queue[T]{buf: make([]T, capacity)}
}

// Push appends an element to the back of the queue. If the internal buffer is
// full, it doubles in size. This amortizes to O(1) per push.
func (q *Queue[T]) Push(v T) {
	if q.len == len(q.buf) {
		q.grow()
	}
	q.buf[q.tail] = v
	q.tail = (q.tail + 1) % len(q.buf)
	q.len++
}

// Pop removes and returns the front element. If the queue is empty, it returns
// the zero value and false.
func (q *Queue[T]) Pop() (T, bool) {
	if q.len == 0 {
		var zero T
		return zero, false
	}
	v := q.buf[q.head]
	var zero T
	q.buf[q.head] = zero // clear reference for GC
	q.head = (q.head + 1) % len(q.buf)
	q.len--
	return v, true
}

// Peek returns the front element without removing it. Returns false if empty.
func (q *Queue[T]) Peek() (T, bool) {
	if q.len == 0 {
		var zero T
		return zero, false
	}
	return q.buf[q.head], true
}

// Len returns the number of elements currently in the queue.
func (q *Queue[T]) Len() int {
	return q.len
}

// Clear removes all elements and resets the queue to empty.
func (q *Queue[T]) Clear() {
	var zero T
	for i := range q.buf {
		q.buf[i] = zero
	}
	q.head = 0
	q.tail = 0
	q.len = 0
}

// grow doubles the buffer capacity, linearizing the ring into a fresh slice.
func (q *Queue[T]) grow() {
	newCap := len(q.buf) * 2
	newBuf := make([]T, newCap)
	// Copy elements in order: head..end, then 0..tail
	if q.head < q.tail {
		copy(newBuf, q.buf[q.head:q.tail])
	} else {
		n := copy(newBuf, q.buf[q.head:])
		copy(newBuf[n:], q.buf[:q.tail])
	}
	q.head = 0
	q.tail = q.len
	q.buf = newBuf
}
