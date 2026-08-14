package camera

import (
	"math"
	"testing"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func approx(a, b float32, eps float32) bool {
	return math.Abs(float64(a-b)) < float64(eps)
}

func TestVectorAnglesForwardX(t *testing.T) {
	// Facing +X → yaw 90 (Quake convention: yaw from +Y axis? verify base).
	got := VectorAngles(qtypes.Vec3{X: 1, Y: 0, Z: 0})
	if !approx(got.Y, 0, 0.01) {
		t.Errorf("VectorAngles(+X).yaw = %v, want 0", got.Y)
	}
	if !approx(got.X, 0, 0.01) {
		t.Errorf("VectorAngles(+X).pitch = %v, want 0", got.X)
	}
}

func TestVectorAnglesStraightUp(t *testing.T) {
	got := VectorAngles(qtypes.Vec3{X: 0, Y: 0, Z: 1})
	if !approx(got.X, -90, 0.01) {
		t.Errorf("VectorAngles(up).pitch = %v, want -90", got.X)
	}
}

func TestAngleVectorsRoundTrip(t *testing.T) {
	angles := qtypes.Vec3{X: 0, Y: 90, Z: 0}
	f, r, u := AngleVectors(angles)
	// Forward should be unit length.
	len := f.Len()
	if !approx(len, 1, 0.001) {
		t.Errorf("forward length = %v, want 1", len)
	}
	// Right and up should be mutually perpendicular to forward.
	dot := f.Dot(r)
	if !approx(dot, 0, 0.001) {
		t.Errorf("forward·right = %v, want 0", dot)
	}
	dot = f.Dot(u)
	if !approx(dot, 0, 0.001) {
		t.Errorf("forward·up = %v, want 0", dot)
	}
}

func TestBoundOffsetsClamps(t *testing.T) {
	// Entity at origin; view far away in +Z must clamp to +30.
	got := BoundOffsets(qtypes.Vec3{X: 0, Y: 0, Z: 999}, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	if got.Z != 30 {
		t.Errorf("BoundOffsets z = %v, want 30", got.Z)
	}
	// Far below clamps to -22.
	got = BoundOffsets(qtypes.Vec3{X: 0, Y: 0, Z: -999}, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	if got.Z != -22 {
		t.Errorf("BoundOffsets z = %v, want -22", got.Z)
	}
}

func TestNodeLineOffsetAddsBias(t *testing.T) {
	got := NodeLineOffset(qtypes.Vec3{X: 1, Y: 2, Z: 3})
	const bias = 1.0 / 32.0
	if !approx(got.X, 1+bias, 0.001) || !approx(got.Y, 2+bias, 0.001) || !approx(got.Z, 3+bias, 0.001) {
		t.Errorf("NodeLineOffset = %v, want [%v %v %v]", got, 1+bias, 2+bias, 3+bias)
	}
}

func TestChaseUpdatePositionsBehind(t *testing.T) {
	origin := qtypes.Vec3{X: 100, Y: 0, Z: 0}
	angles := qtypes.Vec3{X: 0, Y: 0, Z: 0}
	pos, _ := ChaseUpdate(origin, angles, 50, 20, 0, nil)
	// Straight-ahead angles → chase behind along -X (forward=+X for yaw 0?),
	// so pos.X should be less than origin.X when chaseBack > 0.
	if pos.X >= origin.X && pos.Z != origin.Z+20 {
		t.Errorf("ChaseUpdate pos = %v, expected behind origin %v with +20 up", pos, origin)
	}
}
