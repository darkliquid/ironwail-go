package quake

// Vec3 represents a 3D vector (mapped to EvVector).
type Vec3 [3]float32

// Entity represents an entity handle (mapped to EvEntity).
type Entity uintptr

// Func represents a function pointer (mapped to EvFunction).
type Func uintptr

// FieldOffset represents an entity field offset (mapped to EvField).
type FieldOffset uintptr

// Void is used for function return types (mapped to EvVoid).
type Void struct{}
