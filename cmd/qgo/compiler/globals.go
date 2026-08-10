package compiler

import (
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// GlobalAllocator manages the QCVM global address space.
// It pre-reserves system slots and allocates space for globals, temps, and locals.
type GlobalAllocator struct {
	data    []uint32          // Global data (raw 32-bit slots)
	nextOfs uint16            // Next free offset
	named   map[string]uint16 // Name -> offset mapping
	temps   map[uint16]bool   // Currently free temp offsets
}

// FreeGlobalBase is the first offset at which the compiler may allocate
// user-defined global variables. It sits just past the QCVM parameter
// region (parm1..parm16 = OFSParmStart..OFSParmStart+15) so that no
// allocation can ever collide with parm slots during function calls, and
// past the last system global (msg_entity = OFSMsgEntity = 81).
//
// This is the ONE source of truth for the free-global start: both the
// GlobalAllocator (codegen) and the Lowerer's function-value cell allocator
// derive their cursors from it so a layout change can never silently diverge
// (the earlier 43..91 param-collision regression did exactly that).
//
// Note the QCVM call-param region is OFSParm0..OFSParm7 (4..27); parm1..16
// at 43..58 are the *named* global aliases. Starting at 91 keeps every
// compiler-owned global clear of both.
const FreeGlobalBase uint16 = qc.OFSParmStart + 16*3

// systemParamWindow reports whether ofs falls inside the QCVM system/param
// window (ReservedOFS..OFSMsgEntity inclusive plus parm1..parm16). The
// allocator and the lowerer must never hand out an offset here; this guard
// is the defense-in-depth backstop behind FreeGlobalBase (plan D2).
func systemParamWindow(ofs uint16, slots uint16) bool {
	// 43..58 are the named parm1..parm16 aliases; 4..27 are the call param
	// slots OFSParm0..7. All are off-limits to compiler-owned globals.
	if ofs >= qc.OFSParmStart && ofs <= qc.OFSParmStart+15 {
		return true
	}
	if ofs >= qc.OFSParm0 && ofs < qc.OFSParm0+8*3 {
		return true
	}
	// Also keep clear of the system globals block (28..81) that preexists in
	// the VM; new globals start at FreeGlobalBase.
	return ofs < qc.ReservedOFS
}

// systemGlobalOffsetByName returns the fixed QCVM global offset for a
// compiler-known system global name ("self", "time", "mapname", "parm1",
// "trace_endpos", ...), or ok=false. Both the GlobalAllocator (codegen) and
// the Lowerer (resolveObject for //qgo:tagged package vars) use this single
// table so a package var like `Self *quake.Entity //qgo:self` always resolves
// to the same slot the engine writes via QCVM globals.
func systemGlobalOffsetByName(name string) (uint16, bool) {
	ofs, ok := systemGlobalOffsets[name]
	return ofs, ok
}

var systemGlobalOffsets = func() map[string]uint16 {
	m := map[string]uint16{
		"self":            qc.OFSSelf,
		"other":           qc.OFSOther,
		"world":           qc.OFSWorld,
		"time":            qc.OFSTime,
		"frametime":       qc.OFSFrameTime,
		"force_retouch":   qc.OFSForceRetouch,
		"mapname":         qc.OFSMapName,
		"deathmatch":      qc.OFSDeathmatch,
		"coop":            qc.OFSCoop,
		"teamplay":        qc.OFSTeamplay,
		"serverflags":     qc.OFSServerFlags,
		"total_secrets":   qc.OFSTotalSecrets,
		"total_monsters":  qc.OFSTotalMonsters,
		"found_secrets":   qc.OFSFoundSecrets,
		"killed_monsters": qc.OFSKilledMonsters,

		"v_forward": qc.OFSGlobalVForward,
		"v_up":      qc.OFSGlobalVUp,
		"v_right":   qc.OFSGlobalVRight,

		"trace_allsolid":     qc.OFSTraceAllSolid,
		"trace_startsolid":   qc.OFSTraceStartSolid,
		"trace_fraction":     qc.OFSTraceFraction,
		"trace_endpos":       qc.OFSTraceEndPos,
		"trace_plane_normal": qc.OFSTracePlaneNormal,
		"trace_plane_dist":   qc.OFSTracePlaneDist,
		"trace_ent":          qc.OFSTraceEnt,
		"trace_inopen":       qc.OFSTraceInOpen,
		"trace_inwater":      qc.OFSTraceInWater,
		"msg_entity":         qc.OFSMsgEntity,
	}
	// parm1..parm16 begin at OFSParmStart (43).
	for i := 0; i < 16; i++ {
		m[fmt.Sprintf("parm%d", i+1)] = qc.OFSParmStart + uint16(i)
	}
	return m
}()

// NewGlobalAllocator creates a new allocator with system globals pre-reserved.
func NewGlobalAllocator() *GlobalAllocator {
	ga := &GlobalAllocator{
		data:    make([]uint32, qc.ReservedOFS),
		nextOfs: qc.ReservedOFS,
		named:   make(map[string]uint16),
		temps:   make(map[uint16]bool),
	}

	// Pre-register system globals at their fixed offsets
	for name, ofs := range systemGlobalOffsets {
		ga.named[name] = ofs
	}

	// Next free offset after all pre-registered system globals. Start at
	// FreeGlobalBase (past the system block AND the parm1..16 region) so no
	// compile-emitted global can collide with a call parameter slot.
	ga.nextOfs = FreeGlobalBase
	ga.grow(int(ga.nextOfs))

	return ga
}

// Reserve allocates a named global at a specific preassigned offset (used by
// function-value cells whose offset was allocated during lowering). If the
// name was already registered at the same offset it is a no-op (dedupe);
// a conflicting offset is a hard error. Anonymous cells (empty name) are
// always fresh.
func (ga *GlobalAllocator) Reserve(ofs uint16, name string, slots uint16) {
	if name != "" {
		if existing, ok := ga.named[name]; ok {
			if existing != ofs {
				panic(fmt.Sprintf("Reserve(%q): already at %d, want %d", name, existing, ofs))
			}
			return
		}
	}
	if systemParamWindow(ofs, slots) {
		panic(fmt.Sprintf("Reserve(%q): offset %d falls in system/param window", name, ofs))
	}
	if name != "" {
		ga.named[name] = ofs
	}
	if int(ofs)+int(slots) > len(ga.data) {
		ga.grow(int(ofs) + int(slots))
	}
	if int(ofs)+int(slots) > int(ga.nextOfs) {
		ga.nextOfs = ofs + slots
	}
}

// AllocGlobal allocates a named global variable with the given number of slots.
func (ga *GlobalAllocator) AllocGlobal(name string, slots uint16) uint16 {
	if ofs, ok := ga.named[name]; ok {
		return ofs
	}
	ofs := ga.nextOfs
	if systemParamWindow(ofs, slots) {
		// Backstop: never allocate inside the system/param window.
		panic(fmt.Sprintf("AllocGlobal(%q): offset %d falls in system/param window", name, ofs))
	}
	ga.named[name] = ofs
	ga.grow(int(ofs) + int(slots))
	ga.nextOfs += slots
	return ofs
}

// AllocAnon allocates an unnamed global with the given number of slots.
func (ga *GlobalAllocator) AllocAnon(slots uint16) uint16 {
	ofs := ga.nextOfs
	if systemParamWindow(ofs, slots) {
		panic(fmt.Sprintf("AllocAnon: offset %d falls in system/param window", ofs))
	}
	ga.grow(int(ofs) + int(slots))
	ga.nextOfs += slots
	return ofs
}

// AllocTemp allocates a temporary global (tries to reuse freed temps first).
func (ga *GlobalAllocator) AllocTemp(slots uint16) uint16 {
	// Simple: try to find a freed temp of matching size (single slot only)
	if slots == 1 {
		var best uint16
		found := false
		for ofs := range ga.temps {
			if !found || ofs < best {
				best = ofs
				found = true
			}
		}
		if found {
			delete(ga.temps, best)
			return best
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
