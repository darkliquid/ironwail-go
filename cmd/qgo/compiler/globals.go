package compiler

import (
	"fmt"
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
		data:    make([]uint32, qc.ReservedOFS),
		nextOfs: qc.ReservedOFS,
		named:   make(map[string]uint16),
		temps:   make(map[uint16]bool),
	}

	// Pre-register system globals at their fixed offsets
	ga.named["self"] = qc.OFSSelf
	ga.named["other"] = qc.OFSOther
	ga.named["world"] = qc.OFSWorld
	ga.named["time"] = qc.OFSTime
	ga.named["frametime"] = qc.OFSFrameTime
	ga.named["force_retouch"] = qc.OFSForceRetouch
	ga.named["mapname"] = qc.OFSMapName
	ga.named["deathmatch"] = qc.OFSDeathmatch
	ga.named["coop"] = qc.OFSCoop
	ga.named["teamplay"] = qc.OFSTeamplay
	ga.named["serverflags"] = qc.OFSServerFlags
	ga.named["total_secrets"] = qc.OFSTotalSecrets
	ga.named["total_monsters"] = qc.OFSTotalMonsters
	ga.named["found_secrets"] = qc.OFSFoundSecrets
	ga.named["killed_monsters"] = qc.OFSKilledMonsters

	// parm1..parm16 begin at OFSParmStart (43)
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("parm%d", i+1)
		ga.named[name] = qc.OFSParmStart + uint16(i)
	}

	ga.named["v_forward"] = qc.OFSGlobalVForward
	ga.named["v_up"] = qc.OFSGlobalVUp
	ga.named["v_right"] = qc.OFSGlobalVRight

	ga.named["trace_allsolid"] = qc.OFSTraceAllSolid
	ga.named["trace_startsolid"] = qc.OFSTraceStartSolid
	ga.named["trace_fraction"] = qc.OFSTraceFraction
	ga.named["trace_endpos"] = qc.OFSTraceEndPos
	ga.named["trace_plane_normal"] = qc.OFSTracePlaneNormal
	ga.named["trace_plane_dist"] = qc.OFSTracePlaneDist
	ga.named["trace_ent"] = qc.OFSTraceEnt
	ga.named["trace_inopen"] = qc.OFSTraceInOpen
	ga.named["trace_inwater"] = qc.OFSTraceInWater
	ga.named["msg_entity"] = qc.OFSMsgEntity

	// Next free offset after all pre-registered system globals
	ga.nextOfs = qc.OFSMsgEntity + 1
	ga.grow(int(ga.nextOfs))

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
