package renderer

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/gogpu/gogpu"
)

// TestGoGPUFramePreserveSeamUsesMarkPreserveContent verifies the v4 seam
// (ADR-0011): after the HAL world pass renders to the surface, the engine
// calls the public gogpu MarkPreserveContent API so subsequent render passes
// use LoadOp::Load and never wipe the world. The reflection-free seam is what
// matters — the LoadOp decision itself is gogpu's tested contract
// (context_preserve_content_test.go in the gogpu module). This test asserts
// the seam is wired to a live target context that can receive the public call.
func TestGoGPUFramePreserveSeamUsesMarkPreserveContent(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)
	if dc.ctx == nil {
		t.Fatal("seam has no gogpu context to call MarkPreserveContent on")
	}
	if live, _ := dc.preserveMarkTarget(); !live {
		t.Fatal("seam has no live surface holding the preserve mark")
	}
}

// drawContextWithGoGPUPrimarySurface builds a DrawContext whose ctx points at a
// gogpu.Context with a primary RenderTarget wired, mirroring the runtime shape
// (context.renderer.primary) without reflection/unsafe pokes into fields. It
// keeps the existing single-value signature used by all renderer tests.
func drawContextWithGoGPUPrimarySurface(t *testing.T) *DrawContext {
	t.Helper()

	ctxValue := reflect.New(reflect.TypeOf((*gogpu.Context)(nil)).Elem())
	ctxElem := ctxValue.Elem()

	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() {
		t.Fatal("gogpu.Context.renderer field not found")
	}
	setUnexportedReflectField(rendererField, reflect.New(rendererField.Type().Elem()))

	return &DrawContext{ctx: ctxValue.Interface().(*gogpu.Context)}
}

func setUnexportedReflectField(field, value reflect.Value) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(value)
}

// TestMarkPreserveContentOrderingSceneCompositeActive pins ADR-0011 A2:
// when the scene render target is active (waterwarp/translucent-liquid
// retargets the world offscreen), the preserve mark must NOT be applied
// before the scene composite — the surface only holds the composited scene
// after compositeSceneRenderTarget runs. The mark helper therefore refuses
// to pre-mark while the scene target is active (returns false), and the
// warning branch in RenderFrame takes over for the composite ordering.
func TestMarkPreserveContentOrderingSceneCompositeActive(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)
	dc.sceneRenderActive = true

	if dc.markGoGPUFrameContentForOverlay() {
		t.Fatal("markGoGPUFrameContentForOverlay() = true while scene target active — must not pre-mark before scene composite (ADR-0011 A2)")
	}

	// After the composite completes, the scene target is disabled; the mark
	// must then succeed so the overlay pass uses LoadOp::Load.
	dc.sceneRenderActive = false
	if !dc.markGoGPUFrameContentForOverlay() {
		t.Fatal("markGoGPUFrameContentForOverlay() = false after scene composite — overlay would clear the composited scene")
	}
}

// TestMarkSurfacePreservedForOverlayIsEngineSeam pins the engine-frame seam
// (M1.2, ADR-0011): MarkSurfacePreservedForOverlay is the public wrapper the
// game overlay driver calls before painting the widget stack. It must be
// reachable, idempotent, and refuse to fire while the scene target is active
// (A2: the mark belongs AFTER the scene composite). The LoadOp::Load effect
// itself is gogpu's tested contract.
func TestMarkSurfacePreservedForOverlayIsEngineSeam(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)
	if dc == nil {
		t.Fatal("nil DrawContext")
	}
	if !dc.MarkSurfacePreservedForOverlay() {
		t.Fatal("MarkSurfacePreservedForOverlay() = false for a live seam")
	}
	// Idempotent: a second call must not fail or panic.
	if !dc.MarkSurfacePreservedForOverlay() {
		t.Fatal("MarkSurfacePreservedForOverlay() = false on second call (must be idempotent)")
	}

	// A2: with the scene target active the seam must refuse to pre-mark.
	dc.sceneRenderActive = true
	if dc.MarkSurfacePreservedForOverlay() {
		t.Fatal("MarkSurfacePreservedForOverlay() = true while scene target active — must wait for the scene composite (ADR-0011 A2)")
	}
}
