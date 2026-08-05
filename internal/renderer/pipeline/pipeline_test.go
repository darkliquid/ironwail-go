package pipeline

import (
	"testing"

	"github.com/gogpu/gputypes"
)

func TestWorldUniformBufferSize(t *testing.T) {
	if WorldUniformBufferSize != 128 {
		t.Fatalf("WorldUniformBufferSize = %d, want 128 (world.go worldUniformBufferSize)", WorldUniformBufferSize)
	}
}

func TestWorldDepthTextureFormat(t *testing.T) {
	if WorldDepthTextureFormat != gputypes.TextureFormatDepth32FloatStencil8 {
		t.Fatalf("WorldDepthTextureFormat = %v, want Depth32FloatStencil8", WorldDepthTextureFormat)
	}
}

func TestWorldVertexBufferLayout(t *testing.T) {
	layout := WorldVertexBufferLayout()
	if layout.ArrayStride != 48 {
		t.Fatalf("ArrayStride = %d, want 48 (sizeof(WorldVertex))", layout.ArrayStride)
	}
	if layout.StepMode != gputypes.VertexStepModeVertex {
		t.Fatalf("StepMode = %v, want Vertex", layout.StepMode)
	}
	wantAttrs := []struct {
		format gputypes.VertexFormat
		offset uint64
		loc    uint32
	}{
		{gputypes.VertexFormatFloat32x3, 0, 0},  // Position
		{gputypes.VertexFormatFloat32x2, 12, 1}, // TexCoord
		{gputypes.VertexFormatFloat32x2, 20, 2}, // LightmapCoord
		{gputypes.VertexFormatFloat32x3, 28, 3}, // Normal
		{gputypes.VertexFormatFloat32, 40, 4},   // LightmapLayer
		{gputypes.VertexFormatUint32, 44, 5},    // MaterialID
	}
	if len(layout.Attributes) != len(wantAttrs) {
		t.Fatalf("len(Attributes) = %d, want %d", len(layout.Attributes), len(wantAttrs))
	}
	for i, want := range wantAttrs {
		got := layout.Attributes[i]
		if got.Format != want.format || got.Offset != want.offset || got.ShaderLocation != want.loc {
			t.Fatalf("attr[%d] = %+v, want format=%v offset=%d loc=%d", i, got, want.format, want.offset, want.loc)
		}
	}
}

func TestNonDecalDepthStencilState(t *testing.T) {
	state := NonDecalDepthStencilState(true)
	if state.Format != WorldDepthTextureFormat {
		t.Fatalf("Format = %v, want %v", state.Format, WorldDepthTextureFormat)
	}
	if !state.DepthWriteEnabled {
		t.Fatal("DepthWriteEnabled = false, want true")
	}
	if state.DepthCompare != gputypes.CompareFunctionLessEqual {
		t.Fatalf("DepthCompare = %v, want LessEqual", state.DepthCompare)
	}
	if state.StencilReadMask != 0 || state.StencilWriteMask != 0 {
		t.Fatalf("stencil masks = (read %d, write %d), want (0,0)", state.StencilReadMask, state.StencilWriteMask)
	}
	disabled := NonDecalDepthStencilState(false)
	if disabled.DepthWriteEnabled {
		t.Fatal("NonDecalDepthStencilState(false).DepthWriteEnabled = true, want false")
	}
}

// TestWorldDynamicLightBufferSizeParity pins the buffer size that the
// dynamic-lights bind group layout declares, matching world.go's
// gogpuWorldDynamicLightBufferSize. A mismatch regressed rendering to a
// black screen ("bind group ... number of bindings in descriptor (2) does
// not match the number defined in the layout (1)").
func TestWorldDynamicLightBufferSizeParity(t *testing.T) {
	if WorldDynamicLightBufferSize != 16+512*32 {
		t.Fatalf("WorldDynamicLightBufferSize = %d, want %d", WorldDynamicLightBufferSize, 16+512*32)
	}
}
