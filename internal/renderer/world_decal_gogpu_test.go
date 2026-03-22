//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"testing"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

type wgpuTextureViewStub struct{}

func (wgpuTextureViewStub) Destroy()              {}
func (wgpuTextureViewStub) NativeHandle() uintptr { return 1 }

func TestDecalDepthStencilStateMatchesOpenGLParity(t *testing.T) {
	state := decalDepthStencilState()
	if state == nil {
		t.Fatal("decalDepthStencilState() = nil")
	}
	if state.Format != worldDepthTextureFormat {
		t.Fatalf("Format = %v, want %v", state.Format, worldDepthTextureFormat)
	}
	if state.DepthWriteEnabled {
		t.Fatal("DepthWriteEnabled = true, want false")
	}
	if state.DepthCompare != gputypes.CompareFunctionLessEqual {
		t.Fatalf("DepthCompare = %v, want %v", state.DepthCompare, gputypes.CompareFunctionLessEqual)
	}
	for _, face := range []struct {
		name  string
		state hal.StencilFaceState
	}{
		{name: "front", state: state.StencilFront},
		{name: "back", state: state.StencilBack},
	} {
		if face.state.Compare != gputypes.CompareFunctionEqual {
			t.Fatalf("%s.Compare = %v, want %v", face.name, face.state.Compare, gputypes.CompareFunctionEqual)
		}
		if face.state.FailOp != hal.StencilOperationKeep {
			t.Fatalf("%s.FailOp = %v, want %v", face.name, face.state.FailOp, hal.StencilOperationKeep)
		}
		if face.state.DepthFailOp != hal.StencilOperationKeep {
			t.Fatalf("%s.DepthFailOp = %v, want %v", face.name, face.state.DepthFailOp, hal.StencilOperationKeep)
		}
		if face.state.PassOp != hal.StencilOperationIncrementClamp {
			t.Fatalf("%s.PassOp = %v, want %v", face.name, face.state.PassOp, hal.StencilOperationIncrementClamp)
		}
	}
	if state.StencilReadMask != 0xFFFFFFFF {
		t.Fatalf("StencilReadMask = %#x, want %#x", state.StencilReadMask, uint32(0xFFFFFFFF))
	}
	if state.StencilWriteMask != 0xFFFFFFFF {
		t.Fatalf("StencilWriteMask = %#x, want %#x", state.StencilWriteMask, uint32(0xFFFFFFFF))
	}
	if state.DepthBias != -1 {
		t.Fatalf("DepthBias = %d, want -1", state.DepthBias)
	}
	if state.DepthBiasSlopeScale != -2 {
		t.Fatalf("DepthBiasSlopeScale = %v, want -2", state.DepthBiasSlopeScale)
	}
}

func TestDecalDepthAttachmentForViewAllowsStencilWrites(t *testing.T) {
	attachment := decalDepthAttachmentForView(hal.TextureView(&wgpuTextureViewStub{}))
	if attachment == nil {
		t.Fatal("decalDepthAttachmentForView() = nil")
	}
	if attachment.DepthLoadOp != gputypes.LoadOpLoad {
		t.Fatalf("DepthLoadOp = %v, want %v", attachment.DepthLoadOp, gputypes.LoadOpLoad)
	}
	if attachment.DepthStoreOp != gputypes.StoreOpStore {
		t.Fatalf("DepthStoreOp = %v, want %v", attachment.DepthStoreOp, gputypes.StoreOpStore)
	}
	if attachment.StencilLoadOp != gputypes.LoadOpLoad {
		t.Fatalf("StencilLoadOp = %v, want %v", attachment.StencilLoadOp, gputypes.LoadOpLoad)
	}
	if attachment.StencilStoreOp != gputypes.StoreOpStore {
		t.Fatalf("StencilStoreOp = %v, want %v", attachment.StencilStoreOp, gputypes.StoreOpStore)
	}
	if attachment.StencilReadOnly {
		t.Fatal("StencilReadOnly = true, want false")
	}
}

func TestDecalDepthAttachmentForViewNilView(t *testing.T) {
	if got := decalDepthAttachmentForView(nil); got != nil {
		t.Fatalf("decalDepthAttachmentForView(nil) = %#v, want nil", got)
	}
}
