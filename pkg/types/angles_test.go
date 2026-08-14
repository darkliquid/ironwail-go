package types

import (
	"math"
	"testing"
)

func TestAnglesType(t *testing.T) {
	ang := Angles{Pitch: 10, Yaw: 90, Roll: 0}
	v := ang.ToVec3()
	if v.X != 10 || v.Y != 90 || v.Z != 0 {
		t.Errorf("ToVec3 mismatch: %+v", v)
	}

	fromV := AnglesFromVec3(v)
	if fromV != ang {
		t.Errorf("AnglesFromVec3 mismatch: %+v", fromV)
	}

	norm := Angles{Pitch: 370, Yaw: -190, Roll: 720}.Normalize()
	if norm.Pitch != 10 || norm.Yaw != 170 || norm.Roll != 0 {
		t.Errorf("Normalize expected (10, 170, 0), got %+v", norm)
	}

	fwd := Angles{Pitch: 0, Yaw: 90, Roll: 0}.Forward()
	if math.Abs(float64(fwd.X)) > 0.0001 || math.Abs(float64(fwd.Y-1)) > 0.0001 {
		t.Errorf("Forward expected (0, 1, 0), got %+v", fwd)
	}

	fwd2, right, up := Angles{Pitch: 0, Yaw: 0, Roll: 0}.Vectors()
	if fwd2 != (Vec3{X: 1, Y: 0, Z: 0}) {
		t.Errorf("Forward expected (1, 0, 0), got %+v", fwd2)
	}
	if right != (Vec3{X: 0, Y: -1, Z: 0}) && right != (Vec3{X: 0, Y: -1, Z: -0}) {
		t.Errorf("Right expected (0, -1, 0), got %+v", right)
	}
	if up != (Vec3{X: 0, Y: 0, Z: 1}) {
		t.Errorf("Up expected (0, 0, 1), got %+v", up)
	}
}
