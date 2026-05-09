package renderer

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/gogpu/gogpu"
)

func TestGoGPUFrameStateUsesPrimaryWindowSurface(t *testing.T) {
	dc := drawContextWithGoGPUPrimarySurface(t)

	if !dc.markGoGPUFrameContentForOverlay() {
		t.Fatal("markGoGPUFrameContentForOverlay() = false, want true")
	}

	frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug()
	if !ok {
		t.Fatal("getGoGPUFrameStateForDebug() ok = false, want true")
	}
	if !frameCleared || hasPendingClear {
		t.Fatalf("gogpu frame state = (frameCleared=%v, hasPendingClear=%v), want (true, false)", frameCleared, hasPendingClear)
	}
}

func drawContextWithGoGPUPrimarySurface(t *testing.T) *DrawContext {
	t.Helper()

	ctxValue := reflect.New(reflect.TypeOf((*gogpu.Context)(nil)).Elem())
	ctxElem := ctxValue.Elem()

	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer {
		t.Fatal("gogpu.Context.renderer field not found")
	}
	rendererPtr := reflect.New(rendererField.Type().Elem())
	setUnexportedReflectField(rendererField, rendererPtr)

	primaryField := rendererPtr.Elem().FieldByName("primary")
	if !primaryField.IsValid() || primaryField.Kind() != reflect.Pointer {
		t.Fatal("gogpu.Renderer.primary field not found")
	}
	primaryPtr := reflect.New(primaryField.Type().Elem())
	setUnexportedReflectField(primaryField, primaryPtr)

	primaryElem := primaryPtr.Elem()
	setUnexportedReflectField(primaryElem.FieldByName("frameCleared"), reflect.ValueOf(false))
	setUnexportedReflectField(primaryElem.FieldByName("hasPendingClear"), reflect.ValueOf(true))

	return &DrawContext{ctx: ctxValue.Interface().(*gogpu.Context)}
}

func setUnexportedReflectField(field, value reflect.Value) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(value)
}
