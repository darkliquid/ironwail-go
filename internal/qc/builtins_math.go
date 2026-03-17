// Package qc provides QuakeC built-in functions.
//
// This file implements math/vector QuakeC built-ins.
package qc

import (
	"math"
	"math/rand"
)

// ============================================================================
// Vector/Math Builtins (1-10)
// ============================================================================

// makevectors writes new values for v_forward, v_up, and v_right
// based on the entity's angles. This is used for vector math
// and directional operations in QuakeC.
//
// QuakeC signature: void(vector ang) makevectors
func makevectors(vm *VM) {
	angles := vm.GVector(OFSParm0) // Get input angles

	// Calculate yaw (angles[1] in Quake, which is Y axis rotation)
	yaw := float32(angles[1]) * float32(math.Pi) / 180.0

	sinYaw := math.Sin(float64(yaw))
	cosYaw := math.Cos(float64(yaw))
	pitch := float32(angles[0]) * float32(math.Pi) / 180.0
	sinPitch := math.Sin(float64(pitch))
	cosPitch := math.Cos(float64(pitch))

	// v_forward = direction entity is facing
	forward := [3]float32{
		float32(cosYaw * cosPitch),
		float32(sinYaw * cosPitch),
		float32(-sinPitch),
	}

	// v_right = strafe direction (perpendicular to forward)
	right := [3]float32{
		float32(-sinYaw),
		float32(cosYaw),
		0,
	}

	// v_up = always points up in world space
	up := [3]float32{0, 0, 1}

	vm.SetGVector(OFSGlobalVForward, forward)
	vm.SetGVector(OFSGlobalVRight, right)
	vm.SetGVector(OFSGlobalVUp, up)

	// Return void (G_FLOAT(OFSReturn))
	vm.SetGFloat(OFSReturn, 0)
}

// vectoangles converts a direction vector to Euler angles.
// This is the inverse of makevectors.
//
// QuakeC signature: vector(vector dir) vectoangles

// vectoangles converts a direction vector to Euler angles.
// This is the inverse of makevectors.
//
// QuakeC signature: vector(vector dir) vectoangles
func vectoangles(vm *VM) {
	dir := vm.GVector(OFSParm0)

	// Calculate yaw from forward direction
	yaw := math.Atan2(float64(dir[0]), float64(dir[1])) * 180.0 / math.Pi

	// Calculate pitch from up component and forward z
	forwardLen := math.Sqrt(float64(dir[0]*dir[0] + dir[1]*dir[1]))
	pitch := math.Atan2(float64(dir[2]), forwardLen) * 180.0 / math.Pi

	// Roll is always 0
	angles := [3]float32{
		-float32(pitch),
		float32(yaw),
		0,
	}

	vm.SetGVector(OFSReturn, angles)
}

// vectoyaw returns the yaw angle (Y-axis rotation) from a vector.
//
// QuakeC signature: float(vector vec) vectoyaw

// vectoyaw returns the yaw angle (Y-axis rotation) from a vector.
//
// QuakeC signature: float(vector vec) vectoyaw
func vectoyaw(vm *VM) {
	vec := vm.GVector(OFSParm0)
	yaw := math.Atan2(float64(vec[0]), float64(vec[1])) * 180.0 / math.Pi
	vm.SetGFloat(OFSReturn, float32(yaw))
}

// vlen returns the length (magnitude) of a vector.
//
// QuakeC signature: float(vector vec) vlen

// vlen returns the length (magnitude) of a vector.
//
// QuakeC signature: float(vector vec) vlen
func vlen(vm *VM) {
	vec := vm.GVector(OFSParm0)
	length := vm.VectorLength(vec)
	vm.SetGFloat(OFSReturn, length)
}

// normalize normalizes a vector to unit length and returns the original length.
//
// QuakeC signature: float(vector vec) normalize

// normalize normalizes a vector to unit length and returns the original length.
//
// QuakeC signature: float(vector vec) normalize
func normalize(vm *VM) {
	vec := vm.GVector(OFSParm0)
	length := vm.VectorLength(vec)

	if length == 0 {
		vm.SetGVector(OFSReturn, [3]float32{0, 0, 0})
		vm.SetGFloat(OFSReturn, 0)
		return
	}

	// Normalize and return original length
	normalized := vm.VectorNormalize(vec)
	vm.SetGVector(OFSReturn, normalized)
	vm.SetGFloat(OFSReturn, length)
}

// random returns a random float in the range [0, 1].
//
// QuakeC signature: float() random

// random returns a random float in the range [0, 1].
//
// QuakeC signature: float() random
func random(vm *VM) {
	vm.SetGFloat(OFSReturn, rand.Float32())
}

func rintBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Round(float64(v))))
}

func floorBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Floor(float64(v))))
}

func ceilBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Ceil(float64(v))))
}

func fabsBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Abs(float64(v))))
}
