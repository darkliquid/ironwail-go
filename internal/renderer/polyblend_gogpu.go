//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const polyBlendUniformBufferSize = 16

const polyBlendVertexShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );

    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    return output;
}
`

const polyBlendFragmentShaderWGSL = `
struct PolyBlendUniforms {
    blendColor: vec4<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: PolyBlendUniforms;

@fragment
fn fs_main(_input: VertexOutput) -> @location(0) vec4<f32> {
    return uniforms.blendColor;
}
`

func (r *Renderer) destroyPolyBlendResourcesLocked() {
	if r.polyBlendUniformBuffer != nil {
		r.polyBlendUniformBuffer.Release()
		r.polyBlendUniformBuffer = nil
	}
	if r.polyBlendBindGroup != nil {
		r.polyBlendBindGroup.Release()
		r.polyBlendBindGroup = nil
	}
	if r.polyBlendBindGroupLayout != nil {
		r.polyBlendBindGroupLayout.Release()
		r.polyBlendBindGroupLayout = nil
	}
	if r.polyBlendPipelineLayout != nil {
		r.polyBlendPipelineLayout.Release()
		r.polyBlendPipelineLayout = nil
	}
	if r.polyBlendPipeline != nil {
		r.polyBlendPipeline.Release()
		r.polyBlendPipeline = nil
	}
	if r.polyBlendVertexShader != nil {
		r.polyBlendVertexShader.Release()
		r.polyBlendVertexShader = nil
	}
	if r.polyBlendFragmentShader != nil {
		r.polyBlendFragmentShader.Release()
		r.polyBlendFragmentShader = nil
	}
}

func (r *Renderer) ensurePolyBlendResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.polyBlendPipeline != nil && r.polyBlendBindGroup != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, polyBlendVertexShaderWGSL, "PolyBlend Vertex Shader")
	if err != nil {
		return fmt.Errorf("create polyblend vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, polyBlendFragmentShaderWGSL, "PolyBlend Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create polyblend fragment shader: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "PolyBlend Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   polyBlendUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create polyblend bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "PolyBlend Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create polyblend pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "PolyBlend Uniform Buffer",
		Size:             polyBlendUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create polyblend uniform buffer: %w", err)
	}

	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "PolyBlend Uniform BG",
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{
			Binding: 0,
			Buffer:  uniformBuffer,
			Offset:  0,
			Size:    polyBlendUniformBufferSize,
		}},
	})
	if err != nil {
		uniformBuffer.Release()
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create polyblend bind group: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "PolyBlend Render Pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{
						SrcFactor: gputypes.BlendFactorSrcAlpha,
						DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						Operation: gputypes.BlendOperationAdd,
					},
					Alpha: gputypes.BlendComponent{
						SrcFactor: gputypes.BlendFactorOne,
						DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
						Operation: gputypes.BlendOperationAdd,
					},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		bindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create polyblend pipeline: %w", err)
	}

	r.polyBlendVertexShader = vertexShader
	r.polyBlendFragmentShader = fragmentShader
	r.polyBlendBindGroupLayout = bindGroupLayout
	r.polyBlendPipelineLayout = pipelineLayout
	r.polyBlendUniformBuffer = uniformBuffer
	r.polyBlendBindGroup = bindGroup
	r.polyBlendPipeline = pipeline
	return nil
}

func (dc *DrawContext) renderPolyBlendHAL(blend [4]float32) {
	if dc == nil || dc.renderer == nil || blend[3] <= 0 {
		return
	}

	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentWGPURenderTargetView()
	if textureView == nil {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensurePolyBlendResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure polyblend resources", "error", err)
		return
	}
	pipeline := r.polyBlendPipeline
	uniformBuffer := r.polyBlendUniformBuffer
	bindGroup := r.polyBlendBindGroup
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || bindGroup == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "PolyBlend Render Encoder"})
	if err != nil {
		slog.Warn("failed to create polyblend encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "PolyBlend Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
	})
	if err != nil {
		slog.Warn("failed to begin polyblend render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}

	if err := queue.WriteBuffer(uniformBuffer, 0, polyBlendUniformBytes(blend)); err != nil {
		slog.Warn("failed to upload polyblend uniform", "error", err)
		_ = renderPass.End()
		return
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		slog.Warn("failed to end polyblend render pass", "error", err)
		return
	}

	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish polyblend encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit polyblend commands", "error", err)
	}
	_ = device.WaitIdle() // Restore blocking submit (wgpu v0.23.2 Submit is non-blocking)
}

func polyBlendUniformBytes(blend [4]float32) []byte {
	buf := make([]byte, polyBlendUniformBufferSize)
	for i, v := range blend {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(clamp01(v)))
	}
	return buf
}
