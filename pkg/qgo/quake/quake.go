// Package quake provides the core types and primitives for the QuakeC Virtual Machine.
//
// This package is used as a foundation for writing QuakeC logic in Go.
// The QGo compiler recognizes these types and maps them directly to QCVM
// primitive types (ev_float, ev_vector, ev_string, etc.).
package quake

// Vec3 represents a 3D vector, mapped to the QCVM 'ev_vector' type.
// In the QCVM, it is laid out as three consecutive float32 slots (X, Y, Z).
type Vec3 [3]float32

// Entity represents a handle to an entity in the game world, mapped to
// the QCVM 'ev_entity' type. Internally it is an index into the engine's
// edict (entity) table. Entity 0 is always the 'world' entity.
type Entity uintptr

// Func represents a function pointer or index in the QCVM function table,
// mapped to the 'ev_function' type. It is used for callback fields like
// .think, .touch, and .use.
type Func uintptr

// FieldOffset represents an offset into the entity field data, mapped to
// the 'ev_field' type. It is used to dynamically access fields on entities.
type FieldOffset uintptr

// Void is a marker type used for functions that do not return a value.
// In QGo, a function returning Void maps to a QCVM 'ev_void' return type.
type Void struct{}

// MakeVec3 constructs a Vec3 from three float32 values.
// This is a compiler-known helper that allows creating vector literals.
func MakeVec3(x, y, z float32) Vec3 {
	return Vec3{x, y, z}
}

// Sprintf performs string formatting and interpolation at compile time.
// The QGo compiler expands this into a sequence of 'ftos' and 'strcat'
// builtin calls. Only a subset of standard Go Sprintf features are supported.
func Sprintf(format string, args ...interface{}) string {
	return ""
}
