// Package qc — Edict type and entity-field accessors extracted from vm.go
// to keep vm.go under the project's 1000-line hard ceiling. The semantics
// are unchanged; this file only hosts the Edict value type, the
// E{Float,Int,Vector,String,Function,Entity} readers, their SetE*
// counterparts, and the small helpers (Edict, EdictFieldOffset,
// EdictData) that back them.
package qc

import "math"

// Edict represents a server entity for QuakeC integration.
// Num is the entity number (0 = worldspawn).
// Vars provides typed access to the entity's fields.
//
// Edicts are the fundamental game objects in Quake:
// players, monsters, items, triggers, etc.
type Edict struct {
	// Num is the entity number in the edict array.
	Num int
}

// Entity Field Access Methods
//
// These methods provide access to entity field data stored in the
// Edicts byte array. Fields are accessed by their offset from the
// start of the entity's private data area.
//
// Entity layout in Edicts array:
//
//   [0..27]: edict prefix/header data
//   [28..]: EntVars fields (EntityFields * 4 bytes)
//
// Field offsets are determined by the progs.dat file and can be
// looked up using FindField.

// edictBaseFor returns vm.Edicts and the byte offset of edictNum's private
// data area, with the same validation EdictData performs. Splitting the
// bounds check from the slice keeps EFloat/SetEFloat on the fast path
// (they check the field range directly against the backing array instead of
// re-slicing per call).
//
// Layout (must match EdictData): each edict occupies [n*EdictSize,
// n*EdictSize+EdictSize); bytes [0..27] are the edict_t header, so the
// private data area is [off, off+EdictSize-28) where off = n*EdictSize+28.
func (vm *VM) edictBaseFor(edictNum int) (base []byte, off int, end int, ok bool) {
	if vm == nil || edictNum < 0 || edictNum >= vm.NumEdicts || vm.EdictSize <= 28 {
		return nil, 0, 0, false
	}
	base = vm.Edicts
	off = edictNum*vm.EdictSize + 28
	end = off + vm.EdictSize - 28
	if end > len(base) {
		return nil, 0, 0, false
	}
	return base, off, end, true
}

// EdictData returns a slice of the entity's private data area.
// This is the raw byte storage for EntVars fields.
func (vm *VM) EdictData(edictNum int) []byte {
	base, off, end, ok := vm.edictBaseFor(edictNum)
	if !ok {
		return nil
	}
	return base[off:end]
}

// EFloat returns a float entity field value.
// The offset is in float-units (multiply by 4 for byte offset).
func (vm *VM) EFloat(edictNum int, fieldOfs int) float32 {
	base, off, end, ok := vm.edictBaseFor(edictNum)
	if !ok {
		return 0
	}
	f := fieldOfs*4 + off
	if f+4 > end {
		return 0
	}
	// Read as little-endian float32
	bits := uint32(base[f]) |
		uint32(base[f+1])<<8 |
		uint32(base[f+2])<<16 |
		uint32(base[f+3])<<24
	return math.Float32frombits(bits)
}

// EInt returns an integer entity field value.
// The offset is in float-units (multiply by 4 for byte offset).
func (vm *VM) EInt(edictNum int, fieldOfs int) int32 {
	// Bit-cast: EFloat reads raw IEEE 754 bits, reinterpret as int32.
	return int32(math.Float32bits(vm.EFloat(edictNum, fieldOfs)))
}

// EVector returns a 3-component vector entity field.
// The offset points to the first component.
func (vm *VM) EVector(edictNum int, fieldOfs int) [3]float32 {
	return [3]float32{
		vm.EFloat(edictNum, fieldOfs),
		vm.EFloat(edictNum, fieldOfs+1),
		vm.EFloat(edictNum, fieldOfs+2),
	}
}

// EString returns a string entity field value.
// Returns the string table index, not the actual string.
func (vm *VM) EString(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// EFunction returns a function reference entity field.
func (vm *VM) EFunction(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// EEntity returns an entity reference entity field.
func (vm *VM) EEntity(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// SetEFloat sets a float entity field value.
func (vm *VM) SetEFloat(edictNum int, fieldOfs int, v float32) {
	base, off, end, ok := vm.edictBaseFor(edictNum)
	if !ok {
		return
	}
	f := fieldOfs*4 + off
	if f+4 > end {
		return
	}
	bits := math.Float32bits(v)
	base[f] = byte(bits)
	base[f+1] = byte(bits >> 8)
	base[f+2] = byte(bits >> 16)
	base[f+3] = byte(bits >> 24)
}

// SetEInt sets an integer entity field value.
func (vm *VM) SetEInt(edictNum int, fieldOfs int, v int32) {
	// Bit-cast: store int32 bits as float32 for SetEFloat.
	vm.SetEFloat(edictNum, fieldOfs, math.Float32frombits(uint32(v)))
}

// SetEVector sets a 3-component vector entity field.
func (vm *VM) SetEVector(edictNum int, fieldOfs int, v [3]float32) {
	vm.SetEFloat(edictNum, fieldOfs, v[0])
	vm.SetEFloat(edictNum, fieldOfs+1, v[1])
	vm.SetEFloat(edictNum, fieldOfs+2, v[2])
}

// SetEString sets a string entity field by table index.
func (vm *VM) SetEString(edictNum int, fieldOfs int, stringIdx int32) {
	vm.SetEInt(edictNum, fieldOfs, stringIdx)
}

// SetEFunction sets a function reference entity field.
func (vm *VM) SetEFunction(edictNum int, fieldOfs int, funcNum int32) {
	vm.SetEInt(edictNum, fieldOfs, funcNum)
}

// SetEEntity sets an entity reference entity field.
func (vm *VM) SetEEntity(edictNum int, fieldOfs int, entityNum int32) {
	vm.SetEInt(edictNum, fieldOfs, entityNum)
}

// Edict returns the edict for the given entity number.
// Returns nil if the entity number is invalid.
func (vm *VM) Edict(num int) *Edict {
	if num < 0 || num >= vm.NumEdicts {
		return nil
	}
	return &Edict{Num: num}
}

// EdictFieldOffset returns the byte offset for an entity field.
// The offset is relative to the start of the entity's private data.
func (vm *VM) EdictFieldOffset(fieldOfs int) int {
	return 28 + fieldOfs*4
}
