//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

const sceneCompositeUniformBufferSize = 16

const sceneCompositeVertexShaderWGSL = `
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

const sceneCompositeFragmentShaderWGSL = `
struct SceneCompositeUniforms {
    uvScaleWarpTime: vec4<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var sceneSampler: sampler;

@group(0) @binding(1)
var sceneTexture: texture_2d<f32>;

@group(0) @binding(2)
var<uniform> uniforms: SceneCompositeUniforms;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    var uv = input.uv;
    let uvScale = uniforms.uvScaleWarpTime.xy;
    let warpAmp = uniforms.uvScaleWarpTime.z;
    let time = uniforms.uvScaleWarpTime.w;

    if (warpAmp > 0.0) {
        let textureSizeVec = vec2<f32>(textureDimensions(sceneTexture));
        let ddx = dpdx(uv.x) * textureSizeVec.x;
        let ddy = dpdy(uv.y) * textureSizeVec.y;
        let aspect = abs(ddy) / max(abs(ddx), 0.0001);
        let amp = vec2<f32>(warpAmp, warpAmp * aspect);
        uv = amp + uv * (1.0 - 2.0 * amp);
        uv += amp * sin(vec2<f32>(uv.y / max(aspect, 0.0001), uv.x) * (3.14159265 * 8.0) + time);
    }

    return textureSample(sceneTexture, sceneSampler, uv * uvScale);
}
`

func (r *Renderer) sceneSurfaceFormat() gputypes.TextureFormat {
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r != nil && r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return surfaceFormat
}

func (r *Renderer) destroySceneCompositeResourcesLocked() {
	if r.sceneCompositeBindGroup != nil {
		r.sceneCompositeBindGroup.Destroy()
		r.sceneCompositeBindGroup = nil
	}
	if r.sceneCompositeUniformBuffer != nil {
		r.sceneCompositeUniformBuffer.Destroy()
		r.sceneCompositeUniformBuffer = nil
	}
	if r.sceneCompositeSampler != nil {
		r.sceneCompositeSampler.Destroy()
		r.sceneCompositeSampler = nil
	}
	if r.sceneCompositeBindGroupLayout != nil {
		r.sceneCompositeBindGroupLayout.Destroy()
		r.sceneCompositeBindGroupLayout = nil
	}
	if r.sceneCompositePipelineLayout != nil {
		r.sceneCompositePipelineLayout.Destroy()
		r.sceneCompositePipelineLayout = nil
	}
	if r.sceneCompositePipeline != nil {
		r.sceneCompositePipeline.Destroy()
		r.sceneCompositePipeline = nil
	}
	if r.sceneCompositeVertexShader != nil {
		r.sceneCompositeVertexShader.Destroy()
		r.sceneCompositeVertexShader = nil
	}
	if r.sceneCompositeFragmentShader != nil {
		r.sceneCompositeFragmentShader.Destroy()
		r.sceneCompositeFragmentShader = nil
	}
}

func (r *Renderer) destroyWorldRenderTargetLocked() {
	if r.sceneCompositeBindGroup != nil {
		r.sceneCompositeBindGroup.Destroy()
		r.sceneCompositeBindGroup = nil
	}
	if r.worldRenderTextureView != nil {
		r.worldRenderTextureView.Destroy()
		r.worldRenderTextureView = nil
	}
	if r.worldRenderTexture != nil {
		r.worldRenderTexture.Destroy()
		r.worldRenderTexture = nil
	}
	r.worldRenderWidth = 0
	r.worldRenderHeight = 0
}

func (r *Renderer) ensureSceneCompositeResourcesLocked(device hal.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.sceneCompositePipeline != nil && r.sceneCompositeBindGroupLayout != nil && r.sceneCompositeSampler != nil && r.sceneCompositeUniformBuffer != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, sceneCompositeVertexShaderWGSL, "Scene Composite Vertex Shader")
	if err != nil {
		return fmt.Errorf("create scene composite vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, sceneCompositeFragmentShaderWGSL, "Scene Composite Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		return fmt.Errorf("create scene composite fragment shader: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Scene Composite BGL",
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
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   sceneCompositeUniformBufferSize,
				},
			},
		},
	})
	if err != nil {
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create scene composite bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Scene Composite Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create scene composite pipeline layout: %w", err)
	}

	sampler, err := device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "Scene Composite Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  1,
	})
	if err != nil {
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create scene composite sampler: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Scene Composite Uniform Buffer",
		Size:             sceneCompositeUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		sampler.Destroy()
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create scene composite uniform buffer: %w", err)
	}

	pipeline, err := validatedGoGPURenderPipeline(device, &hal.RenderPipelineDescriptor{
		Label:  "Scene Composite Pipeline",
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
				Format:    r.sceneSurfaceFormat(),
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		uniformBuffer.Destroy()
		sampler.Destroy()
		pipelineLayout.Destroy()
		bindGroupLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create scene composite pipeline: %w", err)
	}

	r.sceneCompositeVertexShader = vertexShader
	r.sceneCompositeFragmentShader = fragmentShader
	r.sceneCompositeBindGroupLayout = bindGroupLayout
	r.sceneCompositePipelineLayout = pipelineLayout
	r.sceneCompositeSampler = sampler
	r.sceneCompositeUniformBuffer = uniformBuffer
	r.sceneCompositePipeline = pipeline
	return nil
}

func (r *Renderer) ensureWorldRenderTargetLocked(device hal.Device, width, height int) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid world render target size %dx%d", width, height)
	}
	if err := r.ensureSceneCompositeResourcesLocked(device); err != nil {
		return err
	}
	if r.worldRenderTexture != nil && r.worldRenderTextureView != nil &&
		r.sceneCompositeBindGroup != nil &&
		r.worldRenderWidth == width && r.worldRenderHeight == height {
		return nil
	}

	r.destroyWorldRenderTargetLocked()

	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label: "World Scene Texture",
		Size: hal.Extent3D{
			Width:              uint32(width),
			Height:             uint32(height),
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        r.sceneSurfaceFormat(),
		Usage:         gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageTextureBinding,
	})
	if err != nil {
		return fmt.Errorf("create world scene texture: %w", err)
	}

	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "World Scene Texture View",
		Format:          r.sceneSurfaceFormat(),
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Destroy()
		return fmt.Errorf("create world scene texture view: %w", err)
	}

	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "World Scene Composite BG",
		Layout: r.sceneCompositeBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.sceneCompositeSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
			{
				Binding: 2,
				Resource: gputypes.BufferBinding{
					Buffer: r.sceneCompositeUniformBuffer.NativeHandle(),
					Offset: 0,
					Size:   sceneCompositeUniformBufferSize,
				},
			},
		},
	})
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return fmt.Errorf("create world scene composite bind group: %w", err)
	}

	r.worldRenderTexture = texture
	r.worldRenderTextureView = view
	r.sceneCompositeBindGroup = bindGroup
	r.worldRenderWidth = width
	r.worldRenderHeight = height
	return nil
}

func (dc *DrawContext) surfaceHALTextureView() hal.TextureView {
	if dc == nil || dc.ctx == nil {
		return nil
	}
	surfaceView := dc.ctx.SurfaceView()
	if surfaceView == nil {
		return nil
	}
	return surfaceView.HalTextureView()
}

func (dc *DrawContext) currentHALRenderTargetView() hal.TextureView {
	if dc == nil {
		return nil
	}
	if dc.sceneRenderActive && dc.sceneRenderTarget != nil {
		return dc.sceneRenderTarget
	}
	return dc.surfaceHALTextureView()
}

func shouldUseSceneRenderTarget(state *RenderFrameState) bool {
	if state == nil || !state.WaterWarp {
		return false
	}
	if state.DrawWorld || state.DrawEntities || len(state.DecalMarks) > 0 || state.ViewModel != nil {
		return true
	}
	return state.DrawParticles && state.Particles != nil
}

func (dc *DrawContext) clearCurrentHALRenderTarget(clearColor [4]float32) {
	if dc == nil || dc.renderer == nil {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	textureView := dc.currentHALRenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Scene Target Clear Encoder"})
	if err != nil {
		return
	}
	if err := encoder.BeginEncoding("scene-target-clear"); err != nil {
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Scene Target Clear Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:       textureView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: float64(clearColor[0]), G: float64(clearColor[1]), B: float64(clearColor[2]), A: float64(clearColor[3])},
		}},
	})
	renderPass.End()

	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		return
	}
	_ = queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0)
}

func (dc *DrawContext) enableSceneRenderTarget() bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	device := dc.renderer.getHALDevice()
	if device == nil {
		return false
	}
	width, height := dc.renderer.Size()
	if width <= 0 || height <= 0 {
		return false
	}

	r := dc.renderer
	r.mu.Lock()
	err := r.ensureWorldRenderTargetLocked(device, width, height)
	if err == nil {
		dc.sceneRenderTarget = r.worldRenderTextureView
		dc.sceneRenderActive = dc.sceneRenderTarget != nil
	}
	r.mu.Unlock()
	if err != nil {
		return false
	}
	return dc.sceneRenderActive
}

func (dc *DrawContext) disableSceneRenderTarget() {
	if dc == nil {
		return
	}
	dc.sceneRenderActive = false
	dc.sceneRenderTarget = nil
}

func (dc *DrawContext) compositeSceneRenderTarget(warpActive bool, warpTime float32, clearColor [4]float32) bool {
	if dc == nil || dc.renderer == nil || dc.sceneRenderTarget == nil {
		return false
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return false
	}
	surfaceView := dc.surfaceHALTextureView()
	if surfaceView == nil {
		return false
	}

	r := dc.renderer
	r.mu.RLock()
	pipeline := r.sceneCompositePipeline
	bindGroup := r.sceneCompositeBindGroup
	uniformBuffer := r.sceneCompositeUniformBuffer
	r.mu.RUnlock()
	if pipeline == nil || bindGroup == nil || uniformBuffer == nil {
		return false
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Scene Composite Encoder"})
	if err != nil {
		return false
	}
	if err := encoder.BeginEncoding("scene-composite"); err != nil {
		return false
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Scene Composite Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: float64(clearColor[0]), G: float64(clearColor[1]), B: float64(clearColor[2]), A: float64(clearColor[3])},
		}},
	})
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	if err := queue.WriteBuffer(uniformBuffer, 0, sceneCompositeUniformBytes(warpActive, warpTime)); err != nil {
		return false
	}
	renderPass.Draw(3, 1, 0, 0)
	renderPass.End()

	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		return false
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		return false
	}
	return true
}

func sceneCompositeUniformBytes(warpActive bool, warpTime float32) []byte {
	buf := make([]byte, sceneCompositeUniformBufferSize)
	warpAmp := float32(0)
	if warpActive {
		warpAmp = 1.0 / 256.0
	}
	values := [4]float32{1, 1, warpAmp, warpTime}
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
