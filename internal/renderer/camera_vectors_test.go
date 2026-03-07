//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"
	"testing"
)

func radians(degrees float32) float32 {
	return degrees * (float32(math.Pi) / 180)
}

func assertVec3Close(t *testing.T, name string, got [3]float32, want [3]float32) {
	t.Helper()
	const epsilon = 1e-5
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > epsilon {
			t.Fatalf("%s[%d] = %v, want %v (full got=%v want=%v)", name, i, got[i], want[i], got, want)
		}
	}
}

func TestAngleVectorsMatchQuakeZeroAngles(t *testing.T) {
	forward := angleVectors(0, 0)
	right := angleVectorsRight(0)
	up := angleVectorsUp(0, 0)

	assertVec3Close(t, "forward", [3]float32{forward.X, forward.Y, forward.Z}, [3]float32{1, 0, 0})
	assertVec3Close(t, "right", [3]float32{right.X, right.Y, right.Z}, [3]float32{0, -1, 0})
	assertVec3Close(t, "up", [3]float32{up.X, up.Y, up.Z}, [3]float32{0, 0, 1})
}

func TestAngleVectorsMatchQuakeYaw90(t *testing.T) {
	yaw := radians(90)
	forward := angleVectors(0, yaw)
	right := angleVectorsRight(yaw)
	up := angleVectorsUp(0, yaw)

	assertVec3Close(t, "forward", [3]float32{forward.X, forward.Y, forward.Z}, [3]float32{0, 1, 0})
	assertVec3Close(t, "right", [3]float32{right.X, right.Y, right.Z}, [3]float32{1, 0, 0})
	assertVec3Close(t, "up", [3]float32{up.X, up.Y, up.Z}, [3]float32{0, 0, 1})
}

func TestAngleVectorsMatchQuakePitchUp90(t *testing.T) {
	pitch := radians(-90)
	forward := angleVectors(pitch, 0)
	up := angleVectorsUp(pitch, 0)

	assertVec3Close(t, "forward", [3]float32{forward.X, forward.Y, forward.Z}, [3]float32{0, 0, 1})
	assertVec3Close(t, "up", [3]float32{up.X, up.Y, up.Z}, [3]float32{-1, 0, 0})
}
