//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import "testing"

func TestDestroyOITFramebuffersZeroValueNoop(t *testing.T) {
	var r Renderer
	r.destroyOITFramebuffers()

	if r.oitFB.fbo != 0 || r.oitFB.accumTex != 0 || r.oitFB.revealageTex != 0 {
		t.Fatalf("destroyOITFramebuffers modified zero-value handles: %+v", r.oitFB)
	}
	if r.oitFB.width != 0 || r.oitFB.height != 0 {
		t.Fatalf("destroyOITFramebuffers left non-zero size: %dx%d", r.oitFB.width, r.oitFB.height)
	}
}

func TestDestroyOITFramebuffersZeroesDimensions(t *testing.T) {
	var r Renderer
	r.oitFB.width = 640
	r.oitFB.height = 480
	r.destroyOITFramebuffers()

	if r.oitFB.width != 0 || r.oitFB.height != 0 {
		t.Fatalf("destroyOITFramebuffers did not reset size: %dx%d", r.oitFB.width, r.oitFB.height)
	}
}
