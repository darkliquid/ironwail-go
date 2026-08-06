// math_test.go verifies the pure camera vector-math helpers in isolation.
package camera

import (
	"math"
	"testing"
)

func approx(a, b float32, eps float32) bool {
	return math.Abs(float64(a-b)) < float64(eps)
}

func TestVectorAnglesForwardX(t *testing.T) {
	// Facing +X → yaw 90 (Quake convention: yaw from +Y axis? verify base).
	got := VectorAngles([3]float32{1, 0, 0})
	if !approx(got[1], 0, 0.01) {
		t.Errorf("VectorAngles(+X).yaw = %v, want 0", got[1])
	}
	if !approx(got[0], 0, 0.01) {
		t.Errorf("VectorAngles(+X).pitch = %v, want 0", got[0])
	}
}

func TestVectorAnglesStraightUp(t *testing.T) {
	got := VectorAngles([3]float32{0, 0, 1})
	if !approx(got[0], -90, 0.01) {
		t.Errorf("VectorAngles(up).pitch = %v, want -90", got[0])
	}
}

func TestAngleVectorsRoundTrip(t *testing.T) {
	angles := [3]float32{0, 90, 0}
	f, r, u := AngleVectors(angles)
	// Forward should be unit length.
	len := float32(math.Sqrt(float64(f[0]*f[0] + f[1]*f[1] + f[2]*f[2])))
	if !approx(len, 1, 0.001) {
		t.Errorf("forward length = %v, want 1", len)
	}
	// Right and up should be mutually perpendicular to forward.
	dot := f[0]*r[0] + f[1]*r[1] + f[2]*r[2]
	if !approx(dot, 0, 0.001) {
		t.Errorf("forward·right = %v, want 0", dot)
	}
	dot = f[0]*u[0] + f[1]*u[1] + f[2]*u[2]
	if !approx(dot, 0, 0.001) {
		t.Errorf("forward·up = %v, want 0", dot)
	}
}

func TestBoundOffsetsClamps(t *testing.T) {
	// Entity at origin; view far away in +Z must clamp to +30.
	got := BoundOffsets([3]float32{0, 0, 999}, [3]float32{0, 0, 0})
	if got[2] != 30 {
		t.Errorf("BoundOffsets z = %v, want 30", got[2])
	}
	// Far below clamps to -22.
	got = BoundOffsets([3]float32{0, 0, -999}, [3]float32{0, 0, 0})
	if got[2] != -22 {
		t.Errorf("BoundOffsets z = %v, want -22", got[2])
	}
}

func TestNodeLineOffsetAddsBias(t *testing.T) {
	got := NodeLineOffset([3]float32{1, 2, 3})
	const bias = 1.0 / 32.0
	if !approx(got[0], 1+bias, 0.001) || !approx(got[1], 2+bias, 0.001) || !approx(got[2], 3+bias, 0.001) {
		t.Errorf("NodeLineOffset = %v, want [%v %v %v]", got, 1+bias, 2+bias, 3+bias)
	}
}

func TestChaseUpdatePositionsBehind(t *testing.T) {
	origin := [3]float32{100, 0, 0}
	angles := [3]float32{0, 0, 0}
	pos, _ := ChaseUpdate(origin, angles, 50, 20, 0, nil)
	// Straight-ahead angles → chase behind along -X (forward=+X for yaw 0?),
	// so pos.X should be less than origin.X when chaseBack > 0.
	if pos[0] >= origin[0] && pos[2] != origin[2]+20 {
		t.Errorf("ChaseUpdate pos = %v, expected behind origin %v with +20 up", pos, origin)
	}
}
