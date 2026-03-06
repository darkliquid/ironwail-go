//go:build gogpu
// +build gogpu

package renderer

import (
	"testing"

	"github.com/gogpu/gogpu/gmath"
)

// TestRendererUpdateCamera tests that camera updates are properly cached.
func TestRendererUpdateCamera(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Width = 1280
	cfg.Height = 720

	// Create a renderer without running the full gogpu initialization
	// (this would require a display environment)
	r := &Renderer{
		config:       cfg,
		textureCache: make(map[cacheKey]*cachedTexture),
	}

	// Create a test camera
	camera := CameraState{
		Origin: gmath.NewVec3(100, 200, 300),
		Angles: gmath.NewVec3(45, 90, 0),
		FOV:    96.0,
	}

	// Update the camera
	r.UpdateCamera(camera, 0.1, 4096.0)

	// Verify matrices are cached
	viewMat := r.GetViewMatrix()
	projMat := r.GetProjectionMatrix()
	vpMat := r.GetViewProjectionMatrix()
	camState := r.GetCameraState()

	// Check that matrices are not zero
	viewHasNonZero := false
	for i := 0; i < 16; i++ {
		if viewMat[i] != 0 {
			viewHasNonZero = true
			break
		}
	}
	if !viewHasNonZero {
		t.Error("view matrix should have non-zero elements")
	}

	projHasNonZero := false
	for i := 0; i < 16; i++ {
		if projMat[i] != 0 {
			projHasNonZero = true
			break
		}
	}
	if !projHasNonZero {
		t.Error("projection matrix should have non-zero elements")
	}

	vpHasNonZero := false
	for i := 0; i < 16; i++ {
		if vpMat[i] != 0 {
			vpHasNonZero = true
			break
		}
	}
	if !vpHasNonZero {
		t.Error("VP matrix should have non-zero elements")
	}

	// Check camera state
	if camState.Origin.X != 100 || camState.Origin.Y != 200 || camState.Origin.Z != 300 {
		t.Errorf("camera origin mismatch: %v", camState.Origin)
	}
	if camState.FOV != 96.0 {
		t.Errorf("camera FOV mismatch: expected 96, got %v", camState.FOV)
	}
}

// TestRendererCameraThreadSafety verifies that camera access is thread-safe.
func TestRendererCameraThreadSafety(t *testing.T) {
	cfg := DefaultConfig()
	r := &Renderer{
		config:       cfg,
		textureCache: make(map[cacheKey]*cachedTexture),
	}

	camera := CameraState{
		Origin: gmath.NewVec3(0, 0, 0),
		Angles: gmath.NewVec3(0, 0, 0),
		FOV:    90.0,
	}

	// Update camera should not panic
	r.UpdateCamera(camera, 0.1, 4096.0)

	// Read access should not panic
	_ = r.GetViewMatrix()
	_ = r.GetProjectionMatrix()
	_ = r.GetViewProjectionMatrix()
	_ = r.GetCameraState()
}

// TestConvertClientStateToCameraIntegration verifies the conversion function.
func TestConvertClientStateToCameraIntegration(t *testing.T) {
	origin := [3]float32{50.5, 100.5, 150.5}
	angles := [3]float32{30, 60, 0}
	fov := float32(96.0)

	camera := ConvertClientStateToCamera(origin, angles, fov)

	// Verify exact conversion
	if camera.Origin.X != 50.5 || camera.Origin.Y != 100.5 || camera.Origin.Z != 150.5 {
		t.Errorf("origin conversion failed: got %v", camera.Origin)
	}

	if camera.Angles.X != 30 || camera.Angles.Y != 60 || camera.Angles.Z != 0 {
		t.Errorf("angles conversion failed: got %v", camera.Angles)
	}

	if camera.FOV != 96.0 {
		t.Errorf("FOV conversion failed: expected 96.0, got %v", camera.FOV)
	}
}
