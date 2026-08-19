package renderer

import (
	"testing"
	"unsafe"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/wgpu"
)

// TestRenderIntoWorldTextureNilView asserts a nil view is rejected
// (ADR-0006: the engine must not render into an invalid texture).
func TestRenderIntoWorldTextureNilView(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)
	if err := dc.RenderIntoWorldTexture(gpucontext.TextureView{}); err == nil {
		t.Fatal("RenderIntoWorldTexture(nil view) = nil error, want error")
	}
}

// TestRenderIntoWorldTextureSetsTarget asserts RenderIntoWorldTexture routes
// the world render target to the provided view (ADR-0006, research 0006 §4):
// the scene target seam (currentWGPURenderTargetView) must be redirected to
// the gpuview texture so the compositor blits it under the UI.
func TestRenderIntoWorldTextureSetsTarget(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)

	fakeView := &wgpu.TextureView{}
	view := gpucontext.NewTextureView(unsafe.Pointer(fakeView))

	if err := dc.RenderIntoWorldTexture(view); err != nil {
		t.Fatalf("RenderIntoWorldTexture: %v", err)
	}
	if !dc.sceneRenderActive {
		t.Fatal("sceneRenderActive = false, want true")
	}
	if dc.sceneRenderTarget != fakeView {
		t.Fatal("sceneRenderTarget not set to the provided view")
	}
}
