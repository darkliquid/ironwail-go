package renderer

import "testing"

func TestProjectionFOVForCamera(t *testing.T) {
	camera := CameraState{FOV: 90, Time: 0}
	if got := projectionFOVForCamera(camera); got != 90 {
		t.Fatalf("projectionFOVForCamera() without waterwarp = %v, want 90", got)
	}

	camera.WaterwarpFOV = true
	got := projectionFOVForCamera(camera)
	want := ApplyWaterwarpFOV(camera.FOV, camera.Time)
	if got != want {
		t.Fatalf("projectionFOVForCamera() with waterwarp = %v, want %v", got, want)
	}
	if got >= camera.FOV {
		t.Fatalf("projectionFOVForCamera() = %v, want less than base FOV %v at t=0", got, camera.FOV)
	}
}
