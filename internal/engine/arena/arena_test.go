package arena_test

import (
	"testing"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/engine/arena"
)

type sampleStruct struct {
	A int64
	B float32
	C bool
	D [3]float32
}

func TestArena_NewAndAlloc(t *testing.T) {
	a := arena.NewArena(1024)

	// Allocate a single struct
	item := arena.New[sampleStruct](a)
	if item == nil {
		t.Fatalf("expected non-nil pointer from New")
	}
	item.A = 42
	item.B = 3.14
	if item.A != 42 || item.B != 3.14 {
		t.Errorf("field mismatch: got A=%d, B=%f", item.A, item.B)
	}

	// Allocate a slice of structs
	slice := arena.Alloc[sampleStruct](a, 10)
	if len(slice) != 10 {
		t.Fatalf("expected slice length 10, got %d", len(slice))
	}
	if cap(slice) != 10 {
		t.Fatalf("expected slice capacity 10, got %d", cap(slice))
	}
	slice[9].A = 99
	if slice[9].A != 99 {
		t.Errorf("slice item mismatch: got %d", slice[9].A)
	}
}

func TestArena_DefaultChunkSize(t *testing.T) {
	a := arena.NewArena(0) // Default chunk size
	ptr := arena.New[int64](a)
	if ptr == nil {
		t.Fatalf("expected non-nil pointer with default chunk size")
	}
	if a.BytesReserved() != arena.DefaultChunkSize {
		t.Errorf("expected reserved bytes %d, got %d", arena.DefaultChunkSize, a.BytesReserved())
	}
}

func TestArena_ZeroSizeStruct(t *testing.T) {
	a := arena.NewArena(1024)
	slice := arena.Alloc[struct{}](a, 5)
	if len(slice) != 5 {
		t.Fatalf("expected slice len 5 for struct{}, got %d", len(slice))
	}
	ptr := arena.New[struct{}](a)
	if ptr == nil {
		t.Fatalf("expected non-nil pointer for New[struct{}]")
	}
}

func TestArena_InvalidAllocCounts(t *testing.T) {
	a := arena.NewArena(1024)

	sliceZero := arena.Alloc[int32](a, 0)
	if sliceZero != nil {
		t.Errorf("expected nil slice for 0 count, got %v", sliceZero)
	}

	sliceNeg := arena.Alloc[int32](a, -5)
	if sliceNeg != nil {
		t.Errorf("expected nil slice for negative count, got %v", sliceNeg)
	}

	ptrNeg := arena.New[int32](a)
	if ptrNeg == nil {
		t.Fatalf("expected non-nil pointer from New[int32]")
	}
}

func TestArena_Alignment(t *testing.T) {
	a := arena.NewArena(1024)

	// Do several allocations of different types and verify 8-byte alignment
	for i := 0; i < 50; i++ {
		_ = arena.Alloc[byte](a, 1) // 1 byte alloc
		ptr := arena.New[uint64](a)
		addr := uintptr(unsafe.Pointer(ptr))
		if addr%8 != 0 {
			t.Fatalf("iteration %d: unaligned uint64 address %X (not 8-byte aligned)", i, addr)
		}
	}
}

func TestArena_ChunkGrowth(t *testing.T) {
	// Small chunk size of 128 bytes
	a := arena.NewArena(128)

	// Allocate more than 128 bytes total
	slice1 := arena.Alloc[int64](a, 10) // 80 bytes
	slice2 := arena.Alloc[int64](a, 10) // 80 bytes (should trigger new chunk)

	if len(slice1) != 10 || len(slice2) != 10 {
		t.Fatalf("expected 10 elements each, got %d and %d", len(slice1), len(slice2))
	}

	slice1[0] = 100
	slice2[0] = 200
	if slice1[0] != 100 || slice2[0] != 200 {
		t.Errorf("data corruption across chunks: slice1[0]=%d, slice2[0]=%d", slice1[0], slice2[0])
	}
}

func TestArena_LargeAllocation(t *testing.T) {
	a := arena.NewArena(128)

	// Allocate a slice larger than default chunk size
	large := arena.Alloc[byte](a, 1024)
	if len(large) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(large))
	}
	large[1023] = 0xFF
	if large[1023] != 0xFF {
		t.Errorf("large allocation write failed")
	}
}

func TestArena_ResetAndReuseMultipleChunks(t *testing.T) {
	a := arena.NewArena(128)

	// Allocate 3 chunks worth of data
	_ = arena.Alloc[int64](a, 10) // Chunk 0
	_ = arena.Alloc[int64](a, 10) // Chunk 1
	_ = arena.Alloc[int64](a, 10) // Chunk 2

	reserved := a.BytesReserved()

	// Reset arena
	a.Reset()

	if a.BytesAllocated() != 0 {
		t.Errorf("expected 0 bytes allocated after Reset, got %d", a.BytesAllocated())
	}
	if a.BytesReserved() != reserved {
		t.Errorf("expected reserved memory %d to persist after Reset, got %d", reserved, a.BytesReserved())
	}

	// Re-allocate 3 chunks worth of data to verify chunk reuse without new memory allocation
	s1 := arena.Alloc[int64](a, 10)
	s2 := arena.Alloc[int64](a, 10)
	s3 := arena.Alloc[int64](a, 10)

	if len(s1) != 10 || len(s2) != 10 || len(s3) != 10 {
		t.Fatalf("expected slices of len 10 after reset reuse")
	}
	if a.BytesReserved() != reserved {
		t.Errorf("expected no additional chunk allocations on reuse, reserved before=%d, after=%d", reserved, a.BytesReserved())
	}
}
