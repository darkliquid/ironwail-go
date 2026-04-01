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
    let uvScale = uniforms.uvScaleWarpTime.xy;
    return textureSample(sceneTexture, sceneSampler, input.uv * uvScale);
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
		r.sceneCompositeBindGroup.Release()
		r.sceneCompositeBindGroup = nil
	}
	if r.sceneCompositeUniformBuffer != nil {
		r.sceneCompositeUniformBuffer.Release()
		r.sceneCompositeUniformBuffer = nil
	}
	if r.sceneCompositeSampler != nil {
		r.sceneCompositeSampler.Release()
		r.sceneCompositeSampler = nil
	}
	if r.sceneCompositeBindGroupLayout != nil {
		r.sceneCompositeBindGroupLayout.Release()
		r.sceneCompositeBindGroupLayout = nil
	}
	if r.sceneCompositePipelineLayout != nil {
		r.sceneCompositePipelineLayout.Release()
		r.sceneCompositePipelineLayout = nil
	}
	if r.sceneCompositePipeline != nil {
		r.sceneCompositePipeline.Release()
		r.sceneCompositePipeline = nil
	}
	if r.sceneCompositeVertexShader != nil {
		r.sceneCompositeVertexShader.Release()
		r.sceneCompositeVertexShader = nil
	}
	if r.sceneCompositeFragmentShader != nil {
		r.sceneCompositeFragmentShader.Release()
		r.sceneCompositeFragmentShader = nil
	}
}

func (r *Renderer) destroyWorldRenderTargetLocked() {
	if r.sceneCompositeBindGroup != nil {
		r.sceneCompositeBindGroup.Release()
		r.sceneCompositeBindGroup = nil
	}
	if r.worldRenderTextureView != nil {
		r.worldRenderTextureView.Release()
		r.worldRenderTextureView = nil
	}
	if r.worldRenderTexture != nil {
		r.worldRenderTexture.Release()
		r.worldRenderTexture = nil
	}
	r.worldRenderWidth = 0
	r.worldRenderHeight = 0
}

func (r *Renderer) ensureSceneCompositeResourcesLocked(device *wgpu.Device) error {
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
		vertexShader.Release()
		return fmt.Errorf("create scene composite fragment shader: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
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
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create scene composite bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Scene Composite Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create scene composite pipeline layout: %w", err)
	}

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
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
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create scene composite sampler: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Scene Composite Uniform Buffer",
		Size:             sceneCompositeUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		sampler.Release()
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create scene composite uniform buffer: %w", err)
	}

	pipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "Scene Composite Pipeline",
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
				Format:    r.sceneSurfaceFormat(),
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		uniformBuffer.Release()
		sampler.Release()
		pipelineLayout.Release()
		bindGroupLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
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

func (r *Renderer) ensureWorldRenderTargetLocked(device *wgpu.Device, width, height int) error {
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

	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "World Scene Texture",
		Size: wgpu.Extent3D{
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

	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
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
		texture.Release()
		return fmt.Errorf("create world scene texture view: %w", err)
	}

	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World Scene Composite BG",
		Layout: r.sceneCompositeBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.sceneCompositeSampler},
			{Binding: 1, TextureView: view},
			{Binding: 2, Buffer: r.sceneCompositeUniformBuffer, Offset: 0, Size: sceneCompositeUniformBufferSize},
		},
	})
	if err != nil {
		view.Release()
		texture.Release()
		return fmt.Errorf("create world scene composite bind group: %w", err)
	}

	r.worldRenderTexture = texture
	r.worldRenderTextureView = view
	r.sceneCompositeBindGroup = bindGroup
	r.worldRenderWidth = width
	r.worldRenderHeight = height
	return nil
}

func (dc *DrawContext) surfaceTextureView() *wgpu.TextureView {
	if dc == nil || dc.ctx == nil {
		return nil
	}
	return dc.ctx.SurfaceView()
}

func (dc *DrawContext) currentWGPURenderTargetView() *wgpu.TextureView {
	if dc == nil {
		return nil
	}
	if dc.sceneRenderActive && dc.sceneRenderTarget != nil {
		return dc.sceneRenderTarget
	}
	return dc.surfaceTextureView()
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
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Scene Target Clear Encoder"})
	if err != nil {
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Scene Target Clear Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       textureView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: float64(clearColor[0]), G: float64(clearColor[1]), B: float64(clearColor[2]), A: float64(clearColor[3])},
		}},
	})
	if err != nil {
		return
	}
	_ = renderPass.End()

	cmdBuffer, err := encoder.Finish()
	if err != nil {
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("clearCurrentHALRenderTarget: failed to submit clear commands", "error", err, "subsystem", "renderer")
	}
}

func (dc *DrawContext) enableSceneRenderTarget() bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	device := dc.renderer.getWGPUDevice()
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
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return false
	}
	surfaceView := dc.surfaceTextureView()
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

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Scene Composite Encoder"})
	if err != nil {
		return false
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Scene Composite Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: float64(clearColor[0]), G: float64(clearColor[1]), B: float64(clearColor[2]), A: float64(clearColor[3])},
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
	if err := queue.WriteBuffer(uniformBuffer, 0, sceneCompositeUniformBytes(warpActive, warpTime)); err != nil {
		return false
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
