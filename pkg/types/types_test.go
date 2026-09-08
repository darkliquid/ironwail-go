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

func TestVec3dOperations(t *testing.T) {
	a := Vec3d{X: 1.0, Y: 2.0, Z: 3.0}
	b := Vec3d{X: 4.0, Y: 5.0, Z: 6.0}

	sum := a.Add(b)
	if sum != (Vec3d{X: 5.0, Y: 7.0, Z: 9.0}) {
		t.Fatalf("Vec3d.Add got %v, want {5 7 9}", sum)
	}

	diff := b.Sub(a)
	if diff != (Vec3d{X: 3.0, Y: 3.0, Z: 3.0}) {
		t.Fatalf("Vec3d.Sub got %v, want {3 3 3}", diff)
	}

	scale := a.Scale(2.0)
	if scale != (Vec3d{X: 2.0, Y: 4.0, Z: 6.0}) {
		t.Fatalf("Vec3d.Scale got %v, want {2 4 6}", scale)
	}

	dot := a.Dot(b)
	if dot != 32.0 {
		t.Fatalf("Vec3d.Dot got %v, want 32", dot)
	}

	cross := a.Cross(b)
	if cross != (Vec3d{X: -3.0, Y: 6.0, Z: -3.0}) {
		t.Fatalf("Vec3d.Cross got %v, want {-3 6 -3}", cross)
	}

	arr := a.Array()
	if arr != [3]float64{1.0, 2.0, 3.0} {
		t.Fatalf("Vec3d.Array got %v, want [1 2 3]", arr)
	}

	sl := a.Slice()
	if len(sl) != 3 || sl[0] != 1.0 || sl[1] != 2.0 || sl[2] != 3.0 {
		t.Fatalf("Vec3d.Slice got %v, want [1 2 3]", sl)
	}

	str := a.String()
	if str != "1 2 3" {
		t.Fatalf("Vec3d.String got %q, want '1 2 3'", str)
	}

	// Conversion tests
	v32 := a.Vec3()
	if v32 != (Vec3{X: 1.0, Y: 2.0, Z: 3.0}) {
		t.Fatalf("Vec3d.Vec3 got %v, want {1 2 3}", v32)
	}

	v64 := v32.Vec3d()
	if v64 != a {
		t.Fatalf("Vec3.Vec3d got %v, want %v", v64, a)
	}

	// Constructors
	newV64 := NewVec3d(1.0, 2.0, 3.0)
	if newV64 != a {
		t.Fatalf("NewVec3d got %v, want %v", newV64, a)
	}

	fromArr64 := Vec3dFromArray([3]float64{1.0, 2.0, 3.0})
	if fromArr64 != a {
		t.Fatalf("Vec3dFromArray got %v, want %v", fromArr64, a)
	}

	fromSlice64 := Vec3dFromSlice([]float64{1.0, 2.0, 3.0})
	if fromSlice64 != a {
		t.Fatalf("Vec3dFromSlice got %v, want %v", fromSlice64, a)
	}
}

func TestNormalizeSafe(t *testing.T) {
	zero := Vec3{}
	norm, ok := zero.NormalizeSafe()
	if ok || norm != zero {
		t.Fatalf("zero.NormalizeSafe() = (%v, %v), want (zero, false)", norm, ok)
	}

	v := Vec3{X: 3, Y: 0, Z: 4}
	norm, ok = v.NormalizeSafe()
	if !ok || !norm.ApproxEqual(Vec3{X: 0.6, Y: 0, Z: 0.8}, 1e-5) {
		t.Fatalf("v.NormalizeSafe() = (%v, %v), want ({0.6 0 0.8}, true)", norm, ok)
	}

	v64 := Vec3d{X: 3, Y: 0, Z: 4}
	norm64, ok64 := v64.NormalizeSafe()
	if !ok64 || norm64.X != 0.6 || norm64.Z != 0.8 {
		t.Fatalf("v64.NormalizeSafe() = (%v, %v), want ({0.6 0 0.8}, true)", norm64, ok64)
	}

	// Below threshold boundary check (1e-7 squared is 1e-14 <= 1e-12)
	tiny := Vec3{X: 1e-7, Y: 0, Z: 0}
	if tinyNorm, tinyOk := tiny.NormalizeSafe(); tinyOk || tinyNorm != (Vec3{}) {
		t.Fatalf("tiny.NormalizeSafe() = (%v, %v), want (zero, false)", tinyNorm, tinyOk)
	}

	// NaN vector check
	nanVec := Vec3{X: float32(math.NaN()), Y: 0, Z: 0}
	if nanNorm, nanOk := nanVec.NormalizeSafe(); nanOk || nanNorm != (Vec3{}) {
		t.Fatalf("nanVec.NormalizeSafe() = (%v, %v), want (zero, false)", nanNorm, nanOk)
	}
}

func TestVec3dMemoryLayout(t *testing.T) {
	var v Vec3d
	if unsafe.Sizeof(v) != 24 {
		t.Fatalf("Expected sizeof(Vec3d) == 24, got %d", unsafe.Sizeof(v))
	}
	if unsafe.Alignof(v) != 8 {
		t.Fatalf("Expected alignof(Vec3d) == 8, got %d", unsafe.Alignof(v))
	}
	v = Vec3d{X: 1.5, Y: 2.5, Z: 3.5}
	arrPtr := (*[3]float64)(unsafe.Pointer(&v))
	if *arrPtr != [3]float64{1.5, 2.5, 3.5} {
		t.Fatalf("Pointer reinterpretation mismatch: got %+v", *arrPtr)
	}
}

func TestVec3dExtendedOperations(t *testing.T) {
	a := Vec3d{X: 1.0, Y: 2.0, Z: 3.0}
	b := Vec3d{X: 4.0, Y: 5.0, Z: 6.0}

	// Mul & Div
	if a.Mul(2.0) != (Vec3d{X: 2.0, Y: 4.0, Z: 6.0}) {
		t.Errorf("Vec3d.Mul failed")
	}
	if (Vec3d{X: 2.0, Y: 4.0, Z: 6.0}).Div(2.0) != a {
		t.Errorf("Vec3d.Div failed")
	}

	// Neg & Negate
	if a.Neg() != (Vec3d{X: -1.0, Y: -2.0, Z: -3.0}) || a.Negate() != (Vec3d{X: -1.0, Y: -2.0, Z: -3.0}) {
		t.Errorf("Vec3d.Neg/Negate failed")
	}

	// Length & Distance
	if a.LenSq() != 14.0 || a.LengthSq() != 14.0 {
		t.Errorf("Vec3d.LenSq/LengthSq failed")
	}
	p := Vec3d{X: 3.0, Y: 4.0, Z: 0.0}
	if p.Len() != 5.0 || p.Length() != 5.0 {
		t.Errorf("Vec3d.Len/Length failed")
	}
	if (Vec3d{X: 3.0, Y: 0, Z: 0}).Distance(Vec3d{X: 0, Y: 4.0, Z: 0}) != 5.0 {
		t.Errorf("Vec3d.Distance failed")
	}
	if (Vec3d{X: 3.0, Y: 0, Z: 0}).Dist(Vec3d{X: 0, Y: 4.0, Z: 0}) != 5.0 {
		t.Errorf("Vec3d.Dist failed")
	}
	if (Vec3d{X: 3.0, Y: 0, Z: 0}).DistanceSq(Vec3d{X: 0, Y: 4.0, Z: 0}) != 25.0 {
		t.Errorf("Vec3d.DistanceSq failed")
	}

	// Normalize
	norm := p.Normalize()
	if !norm.ApproxEqual(Vec3d{X: 0.6, Y: 0.8, Z: 0}, 1e-9) {
		t.Errorf("Vec3d.Normalize failed: %v", norm)
	}
	zero := Vec3d{}
	if zero.Normalize() != zero {
		t.Errorf("Vec3d.Normalize zero vector failed")
	}

	// MA & MultiplyAdd
	ma := a.MA(2.0, b)
	if ma != (Vec3d{X: 9.0, Y: 12.0, Z: 15.0}) {
		t.Errorf("Vec3d.MA failed")
	}
	if a.MultiplyAdd(2.0, b) != ma {
		t.Errorf("Vec3d.MultiplyAdd failed")
	}

	// Lerp
	lerp := a.Lerp(b, 0.5)
	if lerp != (Vec3d{X: 2.5, Y: 3.5, Z: 4.5}) {
		t.Errorf("Vec3d.Lerp failed")
	}

	// Set
	var s Vec3d
	s.Set(7.0, 8.0, 9.0)
	if s != (Vec3d{X: 7.0, Y: 8.0, Z: 9.0}) {
		t.Errorf("Vec3d.Set failed")
	}

	// Equals & ApproxEqual
	if !a.Equals(Vec3d{X: 1.0, Y: 2.0, Z: 3.0}) {
		t.Errorf("Vec3d.Equals failed")
	}
	if !a.ApproxEqual(Vec3d{X: 1.0000001, Y: 1.9999999, Z: 3.0000001}, 1e-5) {
		t.Errorf("Vec3d.ApproxEqual failed")
	}

	// Angles and AngleVectors
	ang := (Vec3d{X: 0, Y: 1, Z: 0}).Angles()
	if ang.Y != 90.0 {
		t.Errorf("Vec3d.Angles expected Yaw 90, got %f", ang.Y)
	}
	fwd, _, _ := (Vec3d{X: 0, Y: 90, Z: 0}).AngleVectors()
	if math.Abs(fwd.X) > 1e-6 || math.Abs(fwd.Y-1.0) > 1e-6 {
		t.Errorf("Vec3d.AngleVectors expected (0,1,0), got %v", fwd)
	}

	// String format for Vec3
	v32 := Vec3{X: 1.5, Y: 2.5, Z: 3.5}
	if v32.String() != "1.5 2.5 3.5" {
		t.Errorf("Vec3.String() got %q, want '1.5 2.5 3.5'", v32.String())
	}
}
