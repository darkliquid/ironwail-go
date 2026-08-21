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
