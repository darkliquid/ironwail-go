package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/renderer/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const sceneCompositeUniformBufferSize = 32

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
        // WebGPU textures are Y=0-at-top; clip Y=-1 is screen bottom.
        // Flipping UV Y maps screen-bottom → texture-bottom (scene bottom),
        // which produces a right-side-up composite instead of upside-down.
        vec2<f32>(0.0,  1.0),
        vec2<f32>(2.0,  1.0),
        vec2<f32>(0.0, -1.0),
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
    postProcess: vec4<f32>,
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
    let warpTime = uniforms.uvScaleWarpTime.w;

    if (warpAmp > 0.0) {
        let dx = max(abs(dpdx(uv.x)), 0.00001);
        let dy = max(abs(dpdy(uv.y)), 0.00001);
        let aspect = dy / dx;
        let warpV = vec2<f32>(warpAmp, warpAmp * aspect);
        let remapped = warpV + uv * (1.0 - 2.0 * warpV);
        uv = remapped + warpV * sin(vec2<f32>(remapped.y / aspect, remapped.x) * (3.14159265 * 8.0) + warpTime);
    }

    var color = textureSample(sceneTexture, sceneSampler, uv * uvScale);

    let contrast = uniforms.postProcess.x;
    let gamma = uniforms.postProcess.y;
    let corrected = pow(color.rgb * contrast, vec3<f32>(gamma));

    return vec4<f32>(corrected, color.a);
}
`

func (r *Renderer) sceneSurfaceFormat() gputypes.TextureFormat {
	if r != nil && r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			if fmt := provider.SurfaceFormat(); fmt != gputypes.TextureFormatUndefined {
				return fmt
			}
		}
	}
	if fmt := gogpu.GetBrowserPreferredCanvasFormat(); fmt != gputypes.TextureFormatUndefined {
		return fmt
	}
	return gputypes.TextureFormatBGRA8Unorm
}

func (r *Renderer) destroySceneCompositeResourcesLocked() {
	if r.resources.SceneCompositeBindGroup != nil {
		r.resources.SceneCompositeBindGroup.Release()
		r.resources.SceneCompositeBindGroup = nil
	}
	if r.resources.SceneCompositeUniformBuffer != nil {
		r.resources.SceneCompositeUniformBuffer.Release()
		r.resources.SceneCompositeUniformBuffer = nil
	}
	if r.resources.SceneCompositeSampler != nil {
		r.resources.SceneCompositeSampler.Release()
		r.resources.SceneCompositeSampler = nil
	}
	if r.resources.SceneCompositeBindGroupLayout != nil {
		r.resources.SceneCompositeBindGroupLayout.Release()
		r.resources.SceneCompositeBindGroupLayout = nil
	}
	if r.resources.SceneCompositePipelineLayout != nil {
		r.resources.SceneCompositePipelineLayout.Release()
		r.resources.SceneCompositePipelineLayout = nil
	}
	if r.resources.SceneCompositePipeline != nil {
		r.resources.SceneCompositePipeline.Release()
		r.resources.SceneCompositePipeline = nil
	}
	if r.resources.SceneCompositeVertexShader != nil {
		r.resources.SceneCompositeVertexShader.Release()
		r.resources.SceneCompositeVertexShader = nil
	}
	if r.resources.SceneCompositeFragmentShader != nil {
		r.resources.SceneCompositeFragmentShader.Release()
		r.resources.SceneCompositeFragmentShader = nil
	}
}

func (r *Renderer) destroyWorldRenderTargetLocked() {
	if r.resources.SceneCompositeBindGroup != nil {
		r.resources.SceneCompositeBindGroup.Release()
		r.resources.SceneCompositeBindGroup = nil
	}
	if r.resources.WorldRenderTextureView != nil {
		r.resources.WorldRenderTextureView.Release()
		r.resources.WorldRenderTextureView = nil
	}
	if r.resources.WorldRenderTexture != nil {
		r.resources.WorldRenderTexture.Release()
		r.resources.WorldRenderTexture = nil
	}
	r.resources.WorldRenderWidth = 0
	r.resources.WorldRenderHeight = 0
}

func (r *Renderer) ensureSceneCompositeResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.resources.SceneCompositePipeline != nil && r.resources.SceneCompositeBindGroupLayout != nil && r.resources.SceneCompositeSampler != nil && r.resources.SceneCompositeUniformBuffer != nil {
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

	r.resources.SceneCompositeVertexShader = vertexShader
	r.resources.SceneCompositeFragmentShader = fragmentShader
	r.resources.SceneCompositeBindGroupLayout = bindGroupLayout
	r.resources.SceneCompositePipelineLayout = pipelineLayout
	r.resources.SceneCompositeSampler = sampler
	r.resources.SceneCompositeUniformBuffer = uniformBuffer
	r.resources.SceneCompositePipeline = pipeline
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
	if r.resources.WorldRenderTexture != nil && r.resources.WorldRenderTextureView != nil &&
		r.resources.SceneCompositeBindGroup != nil &&
		r.resources.WorldRenderWidth == width && r.resources.WorldRenderHeight == height {
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
		Usage:         gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopySrc,
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
		Layout: r.resources.SceneCompositeBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.resources.SceneCompositeSampler},
			{Binding: 1, TextureView: view},
			{Binding: 2, Buffer: r.resources.SceneCompositeUniformBuffer, Offset: 0, Size: sceneCompositeUniformBufferSize},
		},
	})
	if err != nil {
		view.Release()
		texture.Release()
		return fmt.Errorf("create world scene composite bind group: %w", err)
	}

	r.resources.WorldRenderTexture = texture
	r.resources.WorldRenderTextureView = view
	r.resources.SceneCompositeBindGroup = bindGroup
	r.resources.WorldRenderWidth = width
	r.resources.WorldRenderHeight = height
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

func (dc *DrawContext) shouldUseSceneRenderTarget(state *RenderFrameState) bool {
	if dc != nil && dc.renderer != nil && dc.renderer.hasTranslucentWorldLiquidFacesGoGPU() && state != nil && (state.DrawWorld || state.DrawEntities) {
		slog.Debug("[rwater] scene render target enabled for translucent liquid")
		return true
	}
	return shouldUseSceneRenderTarget(state)
}

func shouldUseSceneRenderTarget(state *RenderFrameState) bool {
	if GetPassIsolateMode() != PassIsolateNormal {
		return true
	}
	if state == nil {
		return false
	}
	return state.DrawWorld || state.DrawEntities || len(state.DecalMarks) > 0 || state.ViewModel != nil || (state.DrawParticles && state.Particles != nil)
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

	encoder, encoderOwned, err := dc.frameEncoder(device, "Scene Target Clear Encoder")
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
	dc.frameSubmit(queue, encoder, encoderOwned, "Scene Target Clear Encoder")
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
		dc.sceneRenderTarget = r.resources.WorldRenderTextureView
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
	pipeline := r.resources.SceneCompositePipeline
	bindGroup := r.resources.SceneCompositeBindGroup
	uniformBuffer := r.resources.SceneCompositeUniformBuffer
	r.mu.RUnlock()
	if pipeline == nil || bindGroup == nil || uniformBuffer == nil {
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "Scene Composite Encoder")
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
	if err := queue.WriteBuffer(uniformBuffer, 0, sceneCompositeUniformBytes(warpActive, warpTime, dc.contrast, dc.gamma)); err != nil {
		_ = renderPass.End()
		return false
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return false
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "Scene Composite Encoder")
	return true
}

func sceneCompositeUniformBytes(warpActive bool, warpTime float32, contrast, gamma float32) []byte {
	buf := make([]byte, sceneCompositeUniformBufferSize)
	warpAmp := float32(0)
	if warpActive {
		warpAmp = 1.0 / 256.0
	}
	uvScaleWarp := [4]float32{1, 1, warpAmp, warpTime}
	for i, v := range uvScaleWarp {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	postProcess := [4]float32{contrast, gamma, 0, 0}
	for i, v := range postProcess {
		binary.LittleEndian.PutUint32(buf[16+i*4:], math.Float32bits(v))
	}
	return buf
}
