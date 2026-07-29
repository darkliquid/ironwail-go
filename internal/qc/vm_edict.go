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

	// Vars is a typed view of the entity's field data.
	// May be nil if not yet accessed.
	Vars *EntVars
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

// EdictData returns a slice of the entity's private data area.
// This is the raw byte storage for EntVars fields.
func (vm *VM) EdictData(edictNum int) []byte {
	if vm == nil || edictNum < 0 || edictNum >= vm.NumEdicts || vm.EdictSize <= 28 {
		return nil
	}
	offset := edictNum * vm.EdictSize
	if offset+vm.EdictSize > len(vm.Edicts) {
		return nil
	}
	// Skip the edict_t header prefix used before entvars data.
	return vm.Edicts[offset+28 : offset+vm.EdictSize]
}

// EFloat returns a float entity field value.
// The offset is in float-units (multiply by 4 for byte offset).
func (vm *VM) EFloat(edictNum int, fieldOfs int) float32 {
	data := vm.EdictData(edictNum)
	if data == nil || fieldOfs*4+4 > len(data) {
		return 0
	}
	// Read as little-endian float32
	bits := uint32(data[fieldOfs*4]) |
		uint32(data[fieldOfs*4+1])<<8 |
		uint32(data[fieldOfs*4+2])<<16 |
		uint32(data[fieldOfs*4+3])<<24
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
	data := vm.EdictData(edictNum)
	if data == nil || fieldOfs*4+4 > len(data) {
		return
	}
	bits := math.Float32bits(v)
	data[fieldOfs*4] = byte(bits)
	data[fieldOfs*4+1] = byte(bits >> 8)
	data[fieldOfs*4+2] = byte(bits >> 16)
	data[fieldOfs*4+3] = byte(bits >> 24)
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
