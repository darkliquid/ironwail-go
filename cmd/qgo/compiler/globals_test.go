package compiler

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestGlobalAllocator_ReservesSystem(t *testing.T) {
	ga := NewGlobalAllocator()
	if ga.NextOffset() != qc.OFSParmStart {
		t.Fatalf("expected next offset %d, got %d", qc.OFSParmStart, ga.NextOffset())
	}
}

func TestGlobalAllocator_AllocGlobal(t *testing.T) {
	ga := NewGlobalAllocator()
	ofs := ga.AllocGlobal("health", 1)
	if ofs != qc.OFSParmStart {
		t.Fatalf("first global should be at %d, got %d", qc.OFSParmStart, ofs)
	}

	// Duplicate returns same offset
	ofs2 := ga.AllocGlobal("health", 1)
	if ofs2 != ofs {
		t.Fatal("duplicate alloc should return same offset")
	}

	// Next alloc is sequential
	ofs3 := ga.AllocGlobal("armor", 1)
	if ofs3 != qc.OFSParmStart+1 {
		t.Fatalf("second global should be at %d, got %d", qc.OFSParmStart+1, ofs3)
	}
}

func TestGlobalAllocator_SetFloat(t *testing.T) {
	ga := NewGlobalAllocator()
	ofs := ga.AllocGlobal("health", 1)
	ga.SetFloat(ofs, 100.0)

	data := ga.Data()
	got := math.Float32frombits(data[ofs])
	if got != 100.0 {
		t.Fatalf("expected 100.0, got %f", got)
	}
}

func TestGlobalAllocator_SetVector(t *testing.T) {
	ga := NewGlobalAllocator()
	ofs := ga.AllocGlobal("origin", 3)
	ga.SetVector(ofs, [3]float32{1, 2, 3})

	data := ga.Data()
	for i, want := range []float32{1, 2, 3} {
		got := math.Float32frombits(data[ofs+uint16(i)])
		if got != want {
			t.Fatalf("component %d: got %f, want %f", i, got, want)
		}
	}
}
