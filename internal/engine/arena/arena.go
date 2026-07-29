package arena

import (
	"unsafe"
)

// DefaultChunkSize is the default memory chunk allocation size (4 MB).
const DefaultChunkSize = 4 * 1024 * 1024

// Arena is a bump-pointer region memory allocator backed by pre-allocated byte chunks.
type Arena struct {
	chunkSize  int
	chunks     [][]byte
	currChunk  int
	currOffset int
	allocated  int
	reserved   int
}

// NewArena creates a new region arena allocator with the specified default chunk size.
// If defaultChunkSize <= 0, DefaultChunkSize is used.
func NewArena(defaultChunkSize int) *Arena {
	if defaultChunkSize <= 0 {
		defaultChunkSize = DefaultChunkSize
	}
	return &Arena{
		chunkSize: defaultChunkSize,
	}
}

// alignUp rounds n up to the nearest multiple of align (must be a power of 2).
func alignUp(n, align int) int {
	return (n + align - 1) &^ (align - 1)
}

// allocBytes allocates n raw bytes from the arena, aligned to align-byte boundary.
func (a *Arena) allocBytes(n, align int) []byte {
	if n <= 0 {
		return nil
	}
	if align < 1 {
		align = 1
	}

	// Calculate aligned offset in current chunk if chunk exists
	var alignedOffset int
	if len(a.chunks) > 0 && a.currChunk < len(a.chunks) {
		alignedOffset = alignUp(a.currOffset, align)
	}

	// Check if current chunk has enough room
	if len(a.chunks) > 0 && a.currChunk < len(a.chunks) && alignedOffset+n <= len(a.chunks[a.currChunk]) {
		chunk := a.chunks[a.currChunk]
		res := chunk[alignedOffset : alignedOffset+n]
		a.currOffset = alignedOffset + n
		a.allocated += n
		return res
	}

	// Need to advance chunk or allocate a new chunk
	newChunkSize := a.chunkSize
	if n > newChunkSize {
		newChunkSize = alignUp(n, 8)
	}

	// Check if we can reuse an already allocated reserved chunk from a previous Reset()
	nextChunkIndex := 0
	if len(a.chunks) > 0 {
		nextChunkIndex = a.currChunk + 1
	}
	if len(a.chunks) > 0 && a.currOffset > 0 {
		a.currChunk++
		nextChunkIndex = a.currChunk
	}

	if nextChunkIndex < len(a.chunks) && len(a.chunks[nextChunkIndex]) >= n {
		a.currChunk = nextChunkIndex
		chunk := a.chunks[a.currChunk]
		alignedOffset = 0
		res := chunk[alignedOffset : alignedOffset+n]
		a.currOffset = alignedOffset + n
		a.allocated += n
		return res
	}

	// Allocate a new chunk
	newChunk := make([]byte, newChunkSize)
	a.reserved += newChunkSize

	if nextChunkIndex < len(a.chunks) {
		a.chunks[nextChunkIndex] = newChunk
		a.currChunk = nextChunkIndex
	} else {
		a.chunks = append(a.chunks, newChunk)
		a.currChunk = len(a.chunks) - 1
	}

	a.currOffset = n
	a.allocated += n
	return newChunk[:n]
}

// Alloc allocates a slice of count elements of type T from the arena.
func Alloc[T any](a *Arena, count int) []T {
	if count <= 0 {
		return nil
	}
	var elem T
	elemSize := int(unsafe.Sizeof(elem))
	if elemSize == 0 {
		return make([]T, count)
	}

	elemAlign := int(unsafe.Alignof(elem))
	if elemAlign < 8 {
		elemAlign = 8 // Standard 8-byte alignment for hardware alignment safety
	}

	totalBytes := elemSize * count
	buf := a.allocBytes(totalBytes, elemAlign)
	if len(buf) == 0 {
		return nil
	}

	// Convert []byte slice header to []T slice using unsafe.Slice
	return unsafe.Slice((*T)(unsafe.Pointer(&buf[0])), count)
}

// New allocates a single element of type T from the arena and returns its pointer.
func New[T any](a *Arena) *T {
	slice := Alloc[T](a, 1)
	if len(slice) == 0 {
		return nil
	}
	return &slice[0]
}

// Reset resets the arena allocation offsets back to zero without freeing backing chunk memory.
func (a *Arena) Reset() {
	a.currChunk = 0
	a.currOffset = 0
	a.allocated = 0
}

// BytesAllocated returns the number of active allocated bytes in the arena.
func (a *Arena) BytesAllocated() int {
	return a.allocated
}

// BytesReserved returns the total capacity of backing byte chunks held by the arena.
func (a *Arena) BytesReserved() int {
	return a.reserved
}
