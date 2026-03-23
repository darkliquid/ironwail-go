// Package qc provides QuakeC built-in functions.
//
// This file implements math/vector QuakeC built-ins.
package qc

import (
	"fmt"
	"math"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
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
	angles := vm.GVector(OFSParm0)
	forwardVec, rightVec, upVec := qtypes.AngleVectors(qtypes.Vec3{
		X: angles[0],
		Y: angles[1],
		Z: angles[2],
	})
	forward := [3]float32{forwardVec.X, forwardVec.Y, forwardVec.Z}
	right := [3]float32{rightVec.X, rightVec.Y, rightVec.Z}
	up := [3]float32{upVec.X, upVec.Y, upVec.Z}

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

	yaw := float64(vectoyawValue(dir))

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
	vm.SetGFloat(OFSReturn, vectoyawValue(vec))
}

func vectoyawValue(vec [3]float32) float32 {
	if vec[0] == 0 && vec[1] == 0 {
		return 0
	}
	yaw := math.Atan2(float64(vec[1]), float64(vec[0])) * 180.0 / math.Pi
	if yaw < 0 {
		yaw += 360
	}
	return float32(yaw)
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

// normalize normalizes a vector to unit length.
//
// QuakeC signature: vector(vector vec) normalize
func normalize(vm *VM) {
	vec := vm.GVector(OFSParm0)
	length := vm.VectorLength(vec)

	if length == 0 {
		vm.SetGVector(OFSReturn, [3]float32{0, 0, 0})
		return
	}

	normalized := vm.VectorNormalize(vec)
	vm.SetGVector(OFSReturn, normalized)
}

// random returns a random float in the range [0, 1].
//
// QuakeC signature: float() random

// random returns a random float matching C Quake's PF_random behavior.
// With sv_gameplayfix_random (default): ((rand()&0x7fff)+0.5)*(1/0x8000)
// produces values in open interval (0,1), never exactly 0 or 1.
// Without fix: (rand()&0x7fff)/0x7fff, values in [0,1].
//
// QuakeC signature: float() random
func random(vm *VM) {
	// Match C's 15-bit quantization: rand() & 0x7fff
	r := vm.compatRNG.Int() & 0x7fff
	// Default: gameplayfix_random=1 formula avoids exact 0.0 and 1.0.
	// Legacy fallback when sv_gameplayfix_random=0 keeps classic [0,1] endpoints.
	num := (float32(r) + 0.5) * (1.0 / 0x8000)
	if cv := cvar.Get("sv_gameplayfix_random"); cv != nil && cv.Int == 0 {
		num = float32(r) * (1.0 / 0x7fff)
	}
	vm.SetGFloat(OFSReturn, num)
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

// sin returns the sine of an angle in degrees.
// QuakeC signature: float(float angle) sin
func sinBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Sin(float64(v)*math.Pi/180.0)))
}

// cos returns the cosine of an angle in degrees.
// QuakeC signature: float(float angle) cos
func cosBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Cos(float64(v)*math.Pi/180.0)))
}

// sqrt returns the square root of a value.
// QuakeC signature: float(float value) sqrt
func sqrtBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Sqrt(float64(v))))
}

// stof converts a string to a float.
// QuakeC signature: float(string s) stof
func stofBuiltin(vm *VM) {
	s := vm.GString(OFSParm0)
	var f float64
	fmt.Sscanf(s, "%f", &f)
	vm.SetGFloat(OFSReturn, float32(f))
}

// minBuiltin returns the smaller of two floats.
// QuakeC signature: float(float a, float b) min
func minBuiltin(vm *VM) {
	a := vm.GFloat(OFSParm0)
	b := vm.GFloat(OFSParm0 + 3)
	if a < b {
		vm.SetGFloat(OFSReturn, a)
	} else {
		vm.SetGFloat(OFSReturn, b)
	}
}

// maxBuiltin returns the larger of two floats.
// QuakeC signature: float(float a, float b) max
func maxBuiltin(vm *VM) {
	a := vm.GFloat(OFSParm0)
	b := vm.GFloat(OFSParm0 + 3)
	if a > b {
		vm.SetGFloat(OFSReturn, a)
	} else {
		vm.SetGFloat(OFSReturn, b)
	}
}

// boundBuiltin clamps a value between min and max.
// QuakeC signature: float(float min, float value, float max) bound
func boundBuiltin(vm *VM) {
	lo := vm.GFloat(OFSParm0)
	v := vm.GFloat(OFSParm0 + 3)
	hi := vm.GFloat(OFSParm0 + 6)
	if v < lo {
		v = lo
	} else if v > hi {
		v = hi
	}
	vm.SetGFloat(OFSReturn, v)
}

// powBuiltin raises base to exponent.
// QuakeC signature: float(float base, float exp) pow
func powBuiltin(vm *VM) {
	base := vm.GFloat(OFSParm0)
	exp := vm.GFloat(OFSParm0 + 3)
	vm.SetGFloat(OFSReturn, float32(math.Pow(float64(base), float64(exp))))
}

// asinBuiltin returns the arcsine in degrees.
func asinBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Asin(float64(v))*180.0/math.Pi))
}

// acosBuiltin returns the arccosine in degrees.
func acosBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Acos(float64(v))*180.0/math.Pi))
}

// atanBuiltin returns the arctangent in degrees.
func atanBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Atan(float64(v))*180.0/math.Pi))
}

// atan2Builtin returns the two-argument arctangent in degrees.
func atan2Builtin(vm *VM) {
	y := vm.GFloat(OFSParm0)
	x := vm.GFloat(OFSParm0 + 3)
	vm.SetGFloat(OFSReturn, float32(math.Atan2(float64(y), float64(x))*180.0/math.Pi))
}

// tanBuiltin returns the tangent of an angle in degrees.
func tanBuiltin(vm *VM) {
	v := vm.GFloat(OFSParm0)
	vm.SetGFloat(OFSReturn, float32(math.Tan(float64(v)*math.Pi/180.0)))
}

// modBuiltin returns the remainder of a/b.
func modBuiltin(vm *VM) {
	a := vm.GFloat(OFSParm0)
	b := vm.GFloat(OFSParm0 + 3)
	if b == 0 {
		console.Printf("PF_mod: mod by zero\n")
		vm.SetGFloat(OFSReturn, 0)
		return
	}
	vm.SetGFloat(OFSReturn, a-float32(int(a/b))*b)
}
