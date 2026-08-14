package types

import (
	"math"
	"testing"
	"unsafe"
)

func TestVec3MemoryLayout(t *testing.T) {
	var v Vec3
	if unsafe.Sizeof(v) != 12 {
		t.Fatalf("Expected sizeof(Vec3) == 12, got %d", unsafe.Sizeof(v))
	}
	if unsafe.Alignof(v) != 4 {
		t.Fatalf("Expected alignof(Vec3) == 4, got %d", unsafe.Alignof(v))
	}
	// Zero-copy pointer reinterpretation
	v = Vec3{X: 1.5, Y: 2.5, Z: 3.5}
	arrPtr := (*[3]float32)(unsafe.Pointer(&v))
	if *arrPtr != [3]float32{1.5, 2.5, 3.5} {
		t.Fatalf("Pointer reinterpretation mismatch: got %+v", *arrPtr)
	}
}

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

	// Methods
	if a.Add(b) != (Vec3{X: 5, Y: 7, Z: 9}) {
		t.Errorf("Add failed")
	}
	if b.Sub(a) != (Vec3{X: 3, Y: 3, Z: 3}) {
		t.Errorf("Sub failed")
	}
	if a.Scale(2) != (Vec3{X: 2, Y: 4, Z: 6}) {
		t.Errorf("Scale failed")
	}
	if a.Mul(2) != (Vec3{X: 2, Y: 4, Z: 6}) {
		t.Errorf("Mul failed")
	}
	if (Vec3{X: 2, Y: 4, Z: 6}).Div(2) != a {
		t.Errorf("Div failed")
	}
	if a.Neg() != (Vec3{X: -1, Y: -2, Z: -3}) || a.Negate() != (Vec3{X: -1, Y: -2, Z: -3}) {
		t.Errorf("Neg/Negate failed")
	}
	if a.LenSq() != 14 {
		t.Errorf("LenSq expected 14, got %f", a.LenSq())
	}
	if (Vec3{X: 3, Y: 4, Z: 0}).Len() != 5 {
		t.Errorf("Len expected 5, got %f", (Vec3{X: 3, Y: 4, Z: 0}).Len())
	}
	if (Vec3{X: 3, Y: 0, Z: 0}).Distance(Vec3{X: 0, Y: 4, Z: 0}) != 5 {
		t.Errorf("Distance expected 5, got %f", (Vec3{X: 3, Y: 0, Z: 0}).Distance(Vec3{X: 0, Y: 4, Z: 0}))
	}
	if (Vec3{X: 3, Y: 0, Z: 0}).DistanceSq(Vec3{X: 0, Y: 4, Z: 0}) != 25 {
		t.Errorf("DistanceSq expected 25, got %f", (Vec3{X: 3, Y: 0, Z: 0}).DistanceSq(Vec3{X: 0, Y: 4, Z: 0}))
	}
	if !a.Equals(Vec3{X: 1, Y: 2, Z: 3}) {
		t.Errorf("Equals expected true")
	}
	if !a.ApproxEqual(Vec3{X: 1.00001, Y: 2.00001, Z: 2.99999}, 0.001) {
		t.Errorf("ApproxEqual expected true")
	}

	// Conversions
	arr := a.Array()
	if arr != [3]float32{1, 2, 3} {
		t.Errorf("Array expected [1, 2, 3], got %+v", arr)
	}
	sl := a.Slice()
	if len(sl) != 3 || sl[0] != 1 || sl[1] != 2 || sl[2] != 3 {
		t.Errorf("Slice expected [1, 2, 3], got %+v", sl)
	}
	fromArr := Vec3FromArray([3]float32{7, 8, 9})
	if fromArr != (Vec3{X: 7, Y: 8, Z: 9}) {
		t.Errorf("Vec3FromArray failed: %+v", fromArr)
	}
	fromSlice := Vec3FromSlice([]float32{10, 11, 12})
	if fromSlice != (Vec3{X: 10, Y: 11, Z: 12}) {
		t.Errorf("Vec3FromSlice failed: %+v", fromSlice)
	}
}
