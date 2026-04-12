package renderer

import (
	"fmt"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const overlayCompositeVertexShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var uvs = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.0),
        vec2<f32>(2.0, 0.0),
        vec2<f32>(0.0, 2.0),
    );

    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}
`

const overlayCompositeFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var overlaySampler: sampler;

@group(0) @binding(1)
var overlayTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return textureSample(overlayTexture, overlaySampler, input.uv);
}
`

func (r *Renderer) destroyOverlayCompositeResourcesLocked() {
	if r.overlayCompositeBindGroup != nil {
		r.overlayCompositeBindGroup.Release()
		r.overlayCompositeBindGroup = nil
	}
	r.overlayCompositeTextureView = nil
	if r.overlayCompositeBindGroupLayout != nil {
		r.overlayCompositeBindGroupLayout.Release()
		r.overlayCompositeBindGroupLayout = nil
	}
	if r.overlayCompositePipelineLayout != nil {
		r.overlayCompositePipelineLayout.Release()
		r.overlayCompositePipelineLayout = nil
	}
	if r.overlayCompositePipeline != nil {
		r.overlayCompositePipeline.Release()
		r.overlayCompositePipeline = nil
	}
	if r.overlayCompositeVertexShader != nil {
		r.overlayCompositeVertexShader.Release()
		r.overlayCompositeVertexShader = nil
	}
	if r.overlayCompositeFragmentShader != nil {
		r.overlayCompositeFragmentShader.Release()
		r.overlayCompositeFragmentShader = nil
	}
}

func (r *Renderer) ensureOverlayCompositeResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.overlayCompositePipeline != nil && r.overlayCompositeBindGroupLayout != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, overlayCompositeVertexShaderWGSL, "Overlay Composite Vertex Shader")
	if err != nil {
		return fmt.Errorf("create overlay composite vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, overlayCompositeFragmentShaderWGSL, "Overlay Composite Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create overlay composite fragment shader: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Overlay Composite BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create overlay composite bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Overlay Composite Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create overlay composite pipeline layout: %w", err)
	}

	pipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "Overlay Composite Pipeline",
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
				Format: r.sceneSurfaceFormat(),
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
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create overlay composite pipeline: %w", err)
	}

	r.overlayCompositeVertexShader = vertexShader
	r.overlayCompositeFragmentShader = fragmentShader
	r.overlayCompositeBindGroupLayout = bindGroupLayout
	r.overlayCompositePipelineLayout = pipelineLayout
	r.overlayCompositePipeline = pipeline
	return nil
}

func (r *Renderer) ensureOverlayCompositeBindGroupLocked(device *wgpu.Device, tex *gogpu.Texture) error {
	if device == nil || tex == nil || tex.View() == nil || tex.Sampler() == nil {
		return fmt.Errorf("invalid overlay texture")
	}
	if r.overlayCompositeBindGroup != nil && r.overlayCompositeTextureView == tex.View() {
		return nil
	}
	if r.overlayCompositeBindGroup != nil {
		r.overlayCompositeBindGroup.Release()
		r.overlayCompositeBindGroup = nil
	}

	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Overlay Composite BG",
		Layout: r.overlayCompositeBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: tex.Sampler()},
			{Binding: 1, TextureView: tex.View()},
		},
	})
	if err != nil {
		return fmt.Errorf("create overlay composite bind group: %w", err)
	}
	r.overlayCompositeBindGroup = bindGroup
	r.overlayCompositeTextureView = tex.View()
	return nil
}

func (dc *DrawContext) renderOverlayTextureHAL(tex *gogpu.Texture) bool {
	if dc == nil || dc.renderer == nil || tex == nil {
		return false
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return false
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureOverlayCompositeResourcesLocked(device); err != nil {
		r.mu.Unlock()
		return false
	}
	if err := r.ensureOverlayCompositeBindGroupLocked(device, tex); err != nil {
		r.mu.Unlock()
		return false
	}
	pipeline := r.overlayCompositePipeline
	bindGroup := r.overlayCompositeBindGroup
	r.mu.Unlock()
	if pipeline == nil || bindGroup == nil {
		return false
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Overlay Composite Encoder"})
	if err != nil {
		return false
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Overlay Composite Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
	})
	if err != nil {
		return false
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return false
	}

	cmdBuffer, err := encoder.Finish()
	if err != nil {
		return false
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		return false
	}
	return true
}
