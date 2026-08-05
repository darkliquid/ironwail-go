package camera

import (
	"testing"
)

func TestCameraSystem_UpdateZoom(t *testing.T) {
	sys := NewSystem()
	if sys.Zoom != 1.0 {
		t.Fatalf("expected initial zoom 1.0, got %f", sys.Zoom)
	}

	sys.ZoomDir = 1.0
	sys.UpdateZoom(0.1)
	if sys.Zoom <= 1.0 {
		t.Fatalf("expected zoom to increase, got %f", sys.Zoom)
	}

	sys.ZoomDir = -1.0
	sys.UpdateZoom(1.0)
	if sys.Zoom != 1.0 {
		t.Fatalf("expected zoom to clamp at min 1.0, got %f", sys.Zoom)
	}
}
