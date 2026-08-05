// Package pipeline implements the WebGPU render-pipeline constructors and
// shared GPU layout helpers for the GoGPU world renderer.
//
// # Purpose
//
// The root renderer package used to own these constructors as
// (*Renderer) methods in world_pipelines_gogpu.go. They are pure functions
// over wgpu handles: they take a device, shader modules, a layout, and a
// surface format, and return pipelines. The root keeps thin delegating
// wrappers so call sites (world_upload_gogpu.go) are unchanged.
//
// The deep 16+2 stage will additionally move the Renderer's wgpu field group
// into a pipeline.Resources object owned by the parent; that ownership
// transfer is intentionally not done here (it mutates the type graph). This
// package is the behavior-preserving Pattern A seam for that later step.
//
// # Original C lineage
//
// C Ironwail compiled shaders through glslang/driver state; the Go port
// builds wgpu pipelines with explicit layouts. The vertex buffer layout
// (48-byte WorldVertex) mirrors the C vertex attribute arrays in gl_rsurf.c.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/pipeline -count=1
package pipeline

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// WorldUniformBufferSize matches worldUniformBufferSize in the parent
// renderer (world.go). It sizes the UBO binding for world pipelines.
const WorldUniformBufferSize = 128

// WorldDepthTextureFormat is the depth format used by world pipelines and
// depth attachments. Depth32FloatStencil8 is used instead of
// Depth24PlusStencil8 because the wgpu HAL maps Depth24PlusStencil8 to
// VK_FORMAT_D24_UNORM_S8_UINT, which NVIDIA GPUs do not support.
const WorldDepthTextureFormat = gputypes.TextureFormatDepth32FloatStencil8

// NonDecalDepthStencilState returns the depth/stencil state used by world
// render passes that do not write stencil marks (the decal pass has its own).
func NonDecalDepthStencilState(depthWrite bool) *wgpu.DepthStencilState {
	stencilFace := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      wgpu.StencilOperationKeep,
		DepthFailOp: wgpu.StencilOperationKeep,
		PassOp:      wgpu.StencilOperationKeep,
	}
	return &wgpu.DepthStencilState{
		Format:            WorldDepthTextureFormat,
		DepthWriteEnabled: depthWrite,
		DepthCompare:      gputypes.CompareFunctionLessEqual,
		StencilFront:      stencilFace,
		StencilBack:       stencilFace,
		StencilReadMask:   0,
		StencilWriteMask:  0,
	}
}

// CreatePipeline creates a wgpu render pipeline with logging, mirroring the
// parent's validatedGoGPURenderPipeline helper.
func CreatePipeline(device *wgpu.Device, desc *wgpu.RenderPipelineDescriptor) (*wgpu.RenderPipeline, error) {
	if device == nil {
		return nil, fmt.Errorf("nil device")
	}
	if desc == nil {
		return nil, fmt.Errorf("nil render pipeline descriptor")
	}
	slog.Debug("Creating GPU Render Pipeline", "label", desc.Label, "vertex shader", fmt.Sprintf("%p", desc.Vertex.Module), "fragment shader", fmt.Sprintf("%p", desc.Fragment))
	return device.CreateRenderPipeline(desc)
}

// WorldVertexBufferLayout is the 48-byte WorldVertex layout shared by every
// world pipeline. The offsets must match world.WorldVertex (see
// docs/VERTEX_LAYOUT.md).
func WorldVertexBufferLayout() gputypes.VertexBufferLayout {
	return gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
		},
	}
}
