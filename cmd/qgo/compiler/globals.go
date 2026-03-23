package compiler

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/qc"
)

// GlobalAllocator manages the QCVM global address space.
// It pre-reserves system slots and allocates space for globals, temps, and locals.
type GlobalAllocator struct {
	data    []uint32         // Global data (raw 32-bit slots)
	nextOfs uint16           // Next free offset
	named   map[string]uint16 // Name -> offset mapping
	temps   map[uint16]bool   // Currently free temp offsets
}

// NewGlobalAllocator creates a new allocator with system globals pre-reserved.
func NewGlobalAllocator() *GlobalAllocator {
	ga := &GlobalAllocator{
		data:    make([]uint32, qc.OFSParmStart),
		nextOfs: qc.OFSParmStart,
		named:   make(map[string]uint16),
		temps:   make(map[uint16]bool),
	}
	return ga
}

// AllocGlobal allocates a named global variable with the given number of slots.
func (ga *GlobalAllocator) AllocGlobal(name string, slots uint16) uint16 {
	if ofs, ok := ga.named[name]; ok {
		return ofs
	}
	ofs := ga.nextOfs
	ga.named[name] = ofs
	ga.grow(int(ofs) + int(slots))
	ga.nextOfs += slots
	return ofs
}

// AllocAnon allocates an unnamed global with the given number of slots.
func (ga *GlobalAllocator) AllocAnon(slots uint16) uint16 {
	ofs := ga.nextOfs
	ga.grow(int(ofs) + int(slots))
	ga.nextOfs += slots
	return ofs
}

// AllocTemp allocates a temporary global (tries to reuse freed temps first).
func (ga *GlobalAllocator) AllocTemp(slots uint16) uint16 {
	// Simple: try to find a freed temp of matching size (single slot only)
	if slots == 1 {
		for ofs := range ga.temps {
			delete(ga.temps, ofs)
			return ofs
		}
	}
	return ga.AllocAnon(slots)
}

// FreeTemp marks a temp slot as available for reuse.
func (ga *GlobalAllocator) FreeTemp(ofs uint16, slots uint16) {
	if slots == 1 {
		ga.temps[ofs] = true
	}
	// Multi-slot temps aren't reused (vector temps are rare)
}

// SetFloat sets a float value at the given global offset.
func (ga *GlobalAllocator) SetFloat(ofs uint16, val float64) {
	ga.grow(int(ofs) + 1)
	ga.data[ofs] = math.Float32bits(float32(val))
}

// SetInt sets a raw int32 value at the given global offset.
func (ga *GlobalAllocator) SetInt(ofs uint16, val int32) {
	ga.grow(int(ofs) + 1)
	ga.data[ofs] = uint32(val)
}

// SetVector sets a vector value at the given global offset (3 slots).
func (ga *GlobalAllocator) SetVector(ofs uint16, v [3]float32) {
	ga.grow(int(ofs) + 3)
	ga.data[ofs] = math.Float32bits(v[0])
	ga.data[ofs+1] = math.Float32bits(v[1])
	ga.data[ofs+2] = math.Float32bits(v[2])
}

// Lookup returns the offset of a named global, or ok=false if not found.
func (ga *GlobalAllocator) Lookup(name string) (uint16, bool) {
	ofs, ok := ga.named[name]
	return ofs, ok
}

// Data returns the raw global data as uint32 slices.
func (ga *GlobalAllocator) Data() []uint32 {
	return ga.data
}

// NumGlobals returns the total number of global slots allocated.
func (ga *GlobalAllocator) NumGlobals() int {
	return len(ga.data)
}

// NextOffset returns the next available global offset.
func (ga *GlobalAllocator) NextOffset() uint16 {
	return ga.nextOfs
}

func (ga *GlobalAllocator) grow(needed int) {
	for len(ga.data) < needed {
		ga.data = append(ga.data, 0)
	}
}
