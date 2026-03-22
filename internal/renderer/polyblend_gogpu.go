//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
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
		r.polyBlendUniformBuffer.Destroy()
		r.polyBlendUniformBuffer = nil
	}
	if r.polyBlendBindGroup != nil {
		r.polyBlendBindGroup.Destroy()
		r.polyBlendBindGroup = nil
	}
	if r.polyBlendBindGroupLayout != nil {
		r.polyBlendBindGroupLayout.Destroy()
		r.polyBlendBindGroupLayout = nil
	}
	if r.polyBlendPipelineLayout != nil {
		r.polyBlendPipelineLayout.Destroy()
		r.polyBlendPipelineLayout = nil
	}
	if r.polyBlendPipeline != nil {
		r.polyBlendPipeline.Destroy()
		r.polyBlendPipeline = nil
	}
	if r.polyBlendVertexShader != nil {
		r.polyBlendVertexShader.Destroy()
		r.polyBlendVertexShader = nil
	}
	if r.polyBlendFragmentShader != nil {
		r.polyBlendFragmentShader.Destroy()
		r.polyBlendFragmentShader = nil
	}
}

func (r *Renderer) ensurePolyBlendResourcesLocked(device hal.Device) error {
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
		vertexShader.Destroy()
		return fmt.Errorf("create polyblend fragment shader: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create polyblend bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "PolyBlend Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create polyblend pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "PolyBlend Uniform Buffer",
		Size:             polyBlendUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create polyblend uniform buffer: %w", err)
	}

	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "PolyBlend Uniform BG",
		Layout: bindGroupLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   polyBlendUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create polyblend bind group: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "PolyBlend Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
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
		bindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
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

	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "PolyBlend Render Encoder"})
	if err != nil {
		slog.Warn("failed to create polyblend encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("polyblend"); err != nil {
		slog.Warn("failed to begin polyblend encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "PolyBlend Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
	})
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}

	if err := queue.WriteBuffer(uniformBuffer, 0, polyBlendUniformBytes(blend)); err != nil {
		slog.Warn("failed to upload polyblend uniform", "error", err)
		renderPass.End()
		return
	}
	renderPass.Draw(3, 1, 0, 0)
	renderPass.End()

	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish polyblend encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit polyblend commands", "error", err)
	}
}

func polyBlendUniformBytes(blend [4]float32) []byte {
	buf := make([]byte, polyBlendUniformBufferSize)
	for i, v := range blend {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(clamp01(v)))
	}
	return buf
}
