package types

import (
	"math"
	"testing"
)

func TestMathUtils(t *testing.T) {
	if r := QRint(1.5); r != 2 {
		t.Errorf("Expected 2, got %d", r)
	}
	if r := QRint(-1.5); r != -2 {
		t.Errorf("Expected -2, got %d", r)
	}

	if l := QLog2(8); l != 3 {
		t.Errorf("Expected 3, got %d", l)
	}
	if l := QLog2(7); l != 2 {
		t.Errorf("Expected 2, got %d", l)
	}

	if n := QNextPow2(7); n != 8 {
		t.Errorf("Expected 8, got %d", n)
	}
	if n := QNextPow2(8); n != 8 {
		t.Errorf("Expected 8, got %d", n)
	}
}

func TestAngles(t *testing.T) {
	if diff := AngleDifference(10, 350); diff != 20 {
		t.Errorf("Expected 20, got %f", diff)
	}
	if diff := AngleDifference(350, 10); diff != -20 {
		t.Errorf("Expected -20, got %f", diff)
	}

	if norm := NormalizeAngle(370); norm != 10 {
		t.Errorf("Expected 10, got %f", norm)
	}
	if norm := NormalizeAngle(-190); norm != 170 {
		t.Errorf("Expected 170, got %f", norm)
	}
}

func cAngleModReference(a float32) float32 {
	scaled := int32(float64(a) * (65536.0 / 360.0))
	return float32((360.0 / 65536.0) * float64(uint16(scaled)))
}

func TestAngleModMatchesCQuantization(t *testing.T) {
	step := float32(360.0 / 65536.0)
	tests := []float32{
		0,
		360,
		-1,
		721.5,
		step,        // exact 1-step value
		2 * step,    // exact 2-step value
		0.5 * step,  // truncates to 0
		1.9 * step,  // truncates to 1 step
		-step,       // wraps to 360 - 1 step
		-0.5 * step, // truncates to 0 then wraps via mask
		1e9,
		-1e9,
		1e12,
		-1e12,
	}

	for _, in := range tests {
		got := AngleMod(in)
		want := cAngleModReference(in)
		if got != want {
			t.Fatalf("AngleMod(%f) = %f, want %f", in, got, want)
		}
	}
}

func TestVectorAngles(t *testing.T) {
	forward := Vec3{X: 1, Y: 0, Z: 0}
	angles := VectorAngles(forward)
	if angles.X != 0 || angles.Y != 0 || angles.Z != 0 {
		t.Errorf("Expected (0,0,0), got %+v", angles)
	}

	forward = Vec3{X: 0, Y: 1, Z: 0}
	angles = VectorAngles(forward)
	if angles.Y != 90 {
		t.Errorf("Expected Yaw 90, got %f", angles.Y)
	}

	forward = Vec3{X: 0, Y: 0, Z: 1}
	angles = VectorAngles(forward)
	if angles.X != -90 {
		t.Errorf("Expected Pitch -90, got %f", angles.X)
	}
}

func TestAngleVectors(t *testing.T) {
	angles := Vec3{X: 0, Y: 90, Z: 0}
	forward, _, _ := AngleVectors(angles)
	if math.Abs(float64(forward.X)) > 0.0001 || math.Abs(float64(forward.Y-1)) > 0.0001 {
		t.Errorf("Expected forward (0,1,0), got %+v", forward)
	}
}

func TestVec3Utils(t *testing.T) {
	a := Vec3{X: 1, Y: 2, Z: 3}
	b := Vec3{X: 4, Y: 5, Z: 6}
	ma := Vec3MA(a, 2, b)
	if ma.X != 9 || ma.Y != 12 || ma.Z != 15 {
		t.Errorf("Expected (9,12,15), got %+v", ma)
	}

	lerp := Vec3Lerp(a, b, 0.5)
	if lerp.X != 2.5 || lerp.Y != 3.5 || lerp.Z != 4.5 {
		t.Errorf("Expected (2.5,3.5,4.5), got %+v", lerp)
	}
}
