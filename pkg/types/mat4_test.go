package types

import (
	"math"
	"testing"
)

func TestIdentityMatrix(t *testing.T) {
	m := IdentityMatrix()
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			want := float32(0)
			if row == col {
				want = 1
			}
			if got := m[col*4+row]; got != want {
				t.Errorf("m[%d][%d] = %v, want %v", row, col, got, want)
			}
		}
	}
}

func TestRotationMatrixZ90(t *testing.T) {
	// 90° rotation around Z should map X→Y, Y→-X.
	m := RotationMatrix(math.Pi/2, 2)
	v := Vec4{X: 1, Y: 0, Z: 0, W: 1}
	got := Mat4MulVec4(m, v)
	if math.Abs(float64(got.X)) > 1e-6 || math.Abs(float64(got.Y-1)) > 1e-6 {
		t.Errorf("Rz(90°) * (1,0,0) = (%v, %v, %v), want (0, 1, 0)", got.X, got.Y, got.Z)
	}
}

func TestViewMatrixIdentityAtOrigin(t *testing.T) {
	// Zero angles at origin should produce identity.
	m := ViewMatrix(Vec3{}, Vec3{})
	id := IdentityMatrix()
	for i := 0; i < 16; i++ {
		if math.Abs(float64(m[i]-id[i])) > 1e-5 {
			t.Errorf("ViewMatrix(0,0)[%d] = %v, want %v", i, m[i], id[i])
		}
	}
}

// TestVPMatrixForwardPoint verifies that a point 10 units forward (Quake +X)
// from the camera ends up at the center of the screen after VP transform.
func TestVPMatrixForwardPoint(t *testing.T) {
	origin := Vec3{X: 100, Y: 200, Z: 50}
	angles := Vec3{} // pitch=0, yaw=0, roll=0
	view := ViewMatrix(angles, origin)
	fovx := float32(90.0 * math.Pi / 180.0)
	fovy := float32(73.74 * math.Pi / 180.0) // approximate for 4:3 aspect
	proj := FrustumMatrix(fovx, fovy, 0.5, 4096)
	vp := Mat4Multiply(proj, view)

	// Point 10 units forward from camera
	p := Vec4{X: 110, Y: 200, Z: 50, W: 1}
	clip := Mat4MulVec4(vp, p)
	// NDC: should be at center (0, 0)
	ndcX := clip.X / clip.W
	ndcY := clip.Y / clip.W
	if math.Abs(float64(ndcX)) > 1e-4 || math.Abs(float64(ndcY)) > 1e-4 {
		t.Errorf("forward point NDC = (%v, %v), want (0, 0)", ndcX, ndcY)
	}
	if clip.W <= 0 {
		t.Errorf("forward point clip.W = %v, want > 0 (in front of camera)", clip.W)
	}
}

// TestVPMatrixRightPoint verifies that a point to the right in Quake (-Y)
// ends up at +X on screen.
func TestVPMatrixRightPoint(t *testing.T) {
	origin := Vec3{}
	angles := Vec3{} // yaw=0
	view := ViewMatrix(angles, origin)
	fovx := float32(90.0 * math.Pi / 180.0)
	fovy := float32(73.74 * math.Pi / 180.0)
	proj := FrustumMatrix(fovx, fovy, 0.5, 4096)
	vp := Mat4Multiply(proj, view)

	// 10 forward, 5 to the right (-Y in Quake)
	p := Vec4{X: 10, Y: -5, Z: 0, W: 1}
	clip := Mat4MulVec4(vp, p)
	ndcX := clip.X / clip.W
	ndcY := clip.Y / clip.W
	if ndcX <= 0 {
		t.Errorf("right-side point NDC X = %v, want > 0 (right side of screen)", ndcX)
	}
	if math.Abs(float64(ndcY)) > 1e-4 {
		t.Errorf("right-side point NDC Y = %v, want ~0 (vertically centered)", ndcY)
	}
}

// TestVPMatrixUpPoint verifies that a point above the camera (Quake +Z)
// ends up at +Y on screen.
func TestVPMatrixUpPoint(t *testing.T) {
	origin := Vec3{}
	angles := Vec3{}
	view := ViewMatrix(angles, origin)
	fovx := float32(90.0 * math.Pi / 180.0)
	fovy := float32(73.74 * math.Pi / 180.0)
	proj := FrustumMatrix(fovx, fovy, 0.5, 4096)
	vp := Mat4Multiply(proj, view)

	// 10 forward, 5 up
	p := Vec4{X: 10, Y: 0, Z: 5, W: 1}
	clip := Mat4MulVec4(vp, p)
	ndcX := clip.X / clip.W
	ndcY := clip.Y / clip.W
	if math.Abs(float64(ndcX)) > 1e-4 {
		t.Errorf("above point NDC X = %v, want ~0 (horizontally centered)", ndcX)
	}
	if ndcY <= 0 {
		t.Errorf("above point NDC Y = %v, want > 0 (top of screen)", ndcY)
	}
}

// TestVPMatrixYaw90 verifies that yaw=90 makes the camera look along +Y.
func TestVPMatrixYaw90(t *testing.T) {
	origin := Vec3{}
	angles := Vec3{Y: 90} // yaw=90°
	view := ViewMatrix(angles, origin)
	fovx := float32(90.0 * math.Pi / 180.0)
	fovy := float32(73.74 * math.Pi / 180.0)
	proj := FrustumMatrix(fovx, fovy, 0.5, 4096)
	vp := Mat4Multiply(proj, view)

	// Point along +Y should now be "forward" (screen center)
	p := Vec4{X: 0, Y: 10, Z: 0, W: 1}
	clip := Mat4MulVec4(vp, p)
	ndcX := clip.X / clip.W
	ndcY := clip.Y / clip.W
	if math.Abs(float64(ndcX)) > 1e-4 || math.Abs(float64(ndcY)) > 1e-4 {
		t.Errorf("yaw=90 forward point NDC = (%v, %v), want (0, 0)", ndcX, ndcY)
	}
	if clip.W <= 0 {
		t.Errorf("yaw=90 forward point clip.W = %v, want > 0", clip.W)
	}
}
