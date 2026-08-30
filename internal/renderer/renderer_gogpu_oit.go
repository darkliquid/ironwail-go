// renderer_gogpu_oit.go implements order-independent transparency (OIT) for the
// GoGPU/WebGPU backend using McGuire's weighted-blended OIT.
//
// # Why MRT?
//
// C Ironwail's default alpha mode (r_oit 1) uses weighted-blended OIT: a
// translucent pass writes each fragment into two targets at once — an HDR
// accumulation buffer (accum.rgb += rgb*a*weight) and an R8 revealage buffer
// (reveal *= 1-a) — and a fullscreen resolve composites
// accum.rgb/max(accum.a,eps) over the scene with coverage 1-reveal. Writing two
// targets in one pass requires multiple render targets (MRT), which gogpu's
// native Vulkan backend did not support when this port was written. Now that
// gogpu (wgpu v0.31.8+) supports MRT, we match C's approach directly instead of
// the previous per-pixel linked-list (A-Buffer) fallback.
//
// # The algorithm
//
// Pass 1 — Accumulation: translucent turbulent water renders into the accum/
// reveal targets (MRT, additive/multiplicative blend) with depth testing
// against the shared world depth (read-only). No color is written to the scene.
//
// Pass 2 — Resolve: a fullscreen triangle samples accum and reveal and
// alpha-composites accum.rgb/accum.a over the opaque scene with coverage
// 1-reveal (C: R_EndTranslucency, gl_shaders.h oit_resove_fragment_shader).
//
// # Bind group strategy
//
// The accumulation pipeline reuses the world pipeline layout unchanged: group 0
// (uniforms + materials + light clusters + dynamic lights), group 1 (texture),
// group 2 (lightmap), group 3 (fullbright). The resolve pipeline has its own
// dedicated layout (sampler + accum texture + reveal texture).

package renderer

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/renderer/pipeline"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// --- WGSL shaders -----------------------------------------------------------

const oitResolveVertexShaderWGSL = `
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

const oitResolveFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var oitSampler: sampler;

@group(0) @binding(1)
var accumTexture: texture_2d<f32>;

@group(0) @binding(2)
var revealTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let revealage = textureSample(revealTexture, oitSampler, input.uv).r;
    var accumulation = textureSample(accumTexture, oitSampler, input.uv);
    // Suppressing the isinf overflow guard from C: naga cannot lower isinf
    // to SPIR-V, and the HDR accumulation well under the float range for
    // Quake's translucent water.
    let average = clamp(accumulation.rgb / max(accumulation.a, 1e-5), vec3<f32>(0.0), vec3<f32>(1.0));
    return vec4<f32>(average, 1.0 - revealage);
}
`



func oitAccumFragEpilogueWGSL() string {
	return `

struct OITOut {
    @location(0) accum: vec4<f32>,
    @location(1) reveal: f32,
}

@fragment
fn fs_main(input: VertexOutput) -> OITOut {
    // Match C's OIT_OUTPUT epilogue (gl_shaders.h): clamp to [0,1], weight by
    // depth, write accum (premultiplied rgb + a*weight) and reveal (a). C uses
    // z = 1/gl_FragCoord.w, and GLSL gl_FragCoord.w == 1/w_clip, so z == w_clip
    // == clipPos.w here.
    let color = clamp(oitColor(input), vec4<f32>(0.0), vec4<f32>(1.0));
    let z = input.clipPos.w;
    let weight = clamp(color.a * color.a * 0.03 / (1e-5 + z / 1e7), 1e-2, 3e3);
    let premul = color.a * weight;
    var o: OITOut;
    o.accum = vec4<f32>(color.rgb * premul, premul);
    o.reveal = color.a;
    return o;
}
`
}

// oitTranslucentWaterFragmentShaderWGSL wraps the world turbulent fragment
// shader as an OIT accumulation shader. The turbulent body becomes oitColor()
// (its @fragment attribute stripped so the epilogue can call it) and the
// epilogue appends the McGuire weight + MRT writes.
func oitTranslucentWaterFragmentShaderWGSL() string {
	body := strings.Replace(worldTurbulentFragmentShaderWGSL,
		"@fragment\nfn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {",
		"fn oitColor(input: VertexOutput) -> vec4<f32> {",
		1)
	return body + oitAccumFragEpilogueWGSL()
}

// --- OIT resource lifecycle ------------------------------------------------

func (r *Renderer) ensureOITResourcesLocked(device *wgpu.Device, width, height int) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid OIT target size %dx%d", width, height)
	}
	if err := r.ensureOITResolveResourcesLocked(device); err != nil {
		return err
	}
	if err := r.ensureOITWorldPipelineLocked(device); err != nil {
		return err
	}
	if r.resources.OITAccumTexture != nil && r.resources.OITRevealTexture != nil &&
		r.resources.OITWidth == width && r.resources.OITHeight == height {
		return nil
	}
	r.destroyOITTargetsLocked()

	accumTexture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "OIT Accum Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA16Float,
		Usage:         gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopySrc,
	})
	if err != nil {
		return fmt.Errorf("create OIT accum texture: %w", err)
	}
	accumView, err := device.CreateTextureView(accumTexture, &wgpu.TextureViewDescriptor{
		Label:           "OIT Accum Texture View",
		Format:          gputypes.TextureFormatRGBA16Float,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		accumTexture.Release()
		return fmt.Errorf("create OIT accum texture view: %w", err)
	}

	revealTexture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "OIT Reveal Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatR8Unorm,
		Usage:         gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopySrc,
	})
	if err != nil {
		accumView.Release()
		accumTexture.Release()
		return fmt.Errorf("create OIT reveal texture: %w", err)
	}
	revealView, err := device.CreateTextureView(revealTexture, &wgpu.TextureViewDescriptor{
		Label:           "OIT Reveal Texture View",
		Format:          gputypes.TextureFormatR8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		revealTexture.Release()
		accumView.Release()
		accumTexture.Release()
		return fmt.Errorf("create OIT reveal texture view: %w", err)
	}

	r.resources.OITAccumTexture = accumTexture
	r.resources.OITAccumTextureView = accumView
	r.resources.OITRevealTexture = revealTexture
	r.resources.OITRevealTextureView = revealView
	r.resources.OITWidth = width
	r.resources.OITHeight = height
	return nil
}

func (r *Renderer) destroyOITTargetsLocked() {
	if r.resources.OITResolveBindGroup != nil {
		r.resources.OITResolveBindGroup.Release()
		r.resources.OITResolveBindGroup = nil
	}
	if r.resources.OITAccumTextureView != nil {
		r.resources.OITAccumTextureView.Release()
		r.resources.OITAccumTextureView = nil
	}
	if r.resources.OITAccumTexture != nil {
		r.resources.OITAccumTexture.Release()
		r.resources.OITAccumTexture = nil
	}
	if r.resources.OITRevealTextureView != nil {
		r.resources.OITRevealTextureView.Release()
		r.resources.OITRevealTextureView = nil
	}
	if r.resources.OITRevealTexture != nil {
		r.resources.OITRevealTexture.Release()
		r.resources.OITRevealTexture = nil
	}
	r.resources.OITWidth = 0
	r.resources.OITHeight = 0
}

func (r *Renderer) destroyOITResourcesLocked() {
	r.destroyOITTargetsLocked()
	if r.resources.OITResolveSampler != nil {
		r.resources.OITResolveSampler.Release()
		r.resources.OITResolveSampler = nil
	}
	if r.resources.OITResolvePipeline != nil {
		r.resources.OITResolvePipeline.Release()
		r.resources.OITResolvePipeline = nil
	}
	if r.resources.OITResolvePipelineLayout != nil {
		r.resources.OITResolvePipelineLayout.Release()
		r.resources.OITResolvePipelineLayout = nil
	}
	if r.resources.OITResolveBindGroupLayout != nil {
		r.resources.OITResolveBindGroupLayout.Release()
		r.resources.OITResolveBindGroupLayout = nil
	}
	if r.resources.OITResolveVertexShader != nil {
		r.resources.OITResolveVertexShader.Release()
		r.resources.OITResolveVertexShader = nil
	}
	if r.resources.OITResolveFragmentShader != nil {
		r.resources.OITResolveFragmentShader.Release()
		r.resources.OITResolveFragmentShader = nil
	}
	if r.resources.OITWorldTranslucentTurbulentPipeline != nil {
		r.resources.OITWorldTranslucentTurbulentPipeline.Release()
		r.resources.OITWorldTranslucentTurbulentPipeline = nil
	}
	if r.resources.OITAccumPipelineLayout != nil {
		r.resources.OITAccumPipelineLayout.Release()
		r.resources.OITAccumPipelineLayout = nil
	}
}

// --- Resolve pipeline -------------------------------------------------------

func (r *Renderer) ensureOITResolveResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.resources.OITResolvePipeline != nil && r.resources.OITResolveBindGroupLayout != nil &&
		r.resources.OITResolveSampler != nil {
		return nil
	}

	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "OIT Resolve BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
		},
	})
	if err != nil {
		return fmt.Errorf("create OIT resolve BGL: %w", err)
	}
	pl, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "OIT Resolve Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		return fmt.Errorf("create OIT resolve pipeline layout: %w", err)
	}
	vs, err := createWorldShaderModule(device, oitResolveVertexShaderWGSL, "OIT Resolve Vertex Shader")
	if err != nil {
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create OIT resolve vertex shader: %w", err)
	}
	fs, err := createWorldShaderModule(device, oitResolveFragmentShaderWGSL, "OIT Resolve Fragment Shader")
	if err != nil {
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create OIT resolve fragment shader: %w", err)
	}
	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "OIT Resolve Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  1,
	})
	if err != nil {
		fs.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create OIT resolve sampler: %w", err)
	}

	resolvePipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "OIT Resolve Pipeline",
		Layout: pl,
		Vertex: wgpu.VertexState{Module: vs, EntryPoint: "vs_main"},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     fs,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: r.sceneSurfaceFormat(),
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		sampler.Release()
		fs.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create OIT resolve pipeline: %w", err)
	}

	r.resources.OITResolveBindGroupLayout = bgl
	r.resources.OITResolvePipelineLayout = pl
	r.resources.OITResolveVertexShader = vs
	r.resources.OITResolveFragmentShader = fs
	r.resources.OITResolveSampler = sampler
	r.resources.OITResolvePipeline = resolvePipeline
	return nil
}

// --- Accumulation pipeline --------------------------------------------------

func (r *Renderer) ensureOITWorldPipelineLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.resources.OITWorldTranslucentTurbulentPipeline != nil {
		return nil
	}
	// The accumulation shader reuses the world pipeline layout (groups 0-3) and
	// only changes the fragment entry point + MRT targets, so it binds exactly
	// like the deferred translucent world pass.
	layout := r.resources.WorldPipelineLayout
	if layout == nil {
		return fmt.Errorf("world pipeline layout not ready")
	}

	vertexShader, err := createWorldShaderModule(device, worldVertexShaderWGSL, "OIT Water Vertex Shader")
	if err != nil {
		return fmt.Errorf("create OIT water vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, oitTranslucentWaterFragmentShaderWGSL(), "OIT Water Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create OIT water fragment shader: %w", err)
	}

	oitPipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "OIT World Translucent Turbulent Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{pipeline.WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: pipeline.NonDecalDepthStencilState(false),
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{
				{
					Format: gputypes.TextureFormatRGBA16Float,
					Blend: &gputypes.BlendState{
						Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOne, Operation: gputypes.BlendOperationAdd},
						Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOne, Operation: gputypes.BlendOperationAdd},
					},
					WriteMask: gputypes.ColorWriteMaskAll,
				},
				{
					Format: gputypes.TextureFormatR8Unorm,
					Blend: &gputypes.BlendState{
						Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorZero, DstFactor: gputypes.BlendFactorOneMinusSrc, Operation: gputypes.BlendOperationAdd},
						Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorZero, DstFactor: gputypes.BlendFactorOneMinusSrc, Operation: gputypes.BlendOperationAdd},
					},
					WriteMask: gputypes.ColorWriteMaskAll,
				},
			},
		},
	})
	if err != nil {
		fragmentShader.Release()
		vertexShader.Release()
		return fmt.Errorf("create OIT world translucent turbulent pipeline: %w", err)
	}
	vertexShader.Release()
	fragmentShader.Release()

	r.resources.OITWorldTranslucentTurbulentPipeline = oitPipeline
	// The pipeline layout is shared with the world pipeline; retain a reference
	// so destroyOITResourcesLocked does not free the world layout while it is in
	// use. We do not own it — the world upload owns it — so hold without release.
	r.resources.OITAccumPipelineLayout = layout
	return nil
}

// --- OIT passes --------------------------------------------------------------

func (dc *DrawContext) renderOITTranslucentWorldLiquidHAL(fogColor types.Vec3, fogDensity float32) bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	r := dc.renderer
	faces := r.deferredTranslucentLiquidFaces
	if !r.deferredTranslucentLiquidValid || len(faces) == 0 {
		return false
	}
	if r.worldIndexBuffer == nil || r.worldVertexBuffer == nil {
		return false
	}
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil {
		return false
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return false
	}

	r.mu.Lock()
	if err := r.ensureOITResourcesLocked(device, width, height); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure OIT resources", "error", err)
		return false
	}
	pipelineObj := r.resources.OITWorldTranslucentTurbulentPipeline
	accumView := r.resources.OITAccumTextureView
	revealView := r.resources.OITRevealTextureView
	depthView := r.resources.WorldDepthTextureView
	uniformBindGroup := r.resources.UniformBindGroup
	dynamicLightsBuffer := r.resources.WorldDynamicLightsBuffer
	camera := r.cameraState
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	r.mu.Unlock()
	if pipelineObj == nil || accumView == nil || revealView == nil || uniformBindGroup == nil || dynamicLightsBuffer == nil {
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "OIT World Translucent Liquid Encoder")
	if err != nil {
		slog.Warn("failed to create OIT water encoder", "error", err)
		return false
	}

	// Accumulation pass: write accum (clear to black) + reveal (clear to 1)
	// into the OIT MRT targets, depth-testing against the shared world depth.
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "OIT World Translucent Liquid Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       accumView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 0},
			},
			{
				View:       revealView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 1, G: 1, B: 1, A: 1},
			},
		},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderOITTranslucentWorldLiquidHAL: Failed to begin render pass", "error", err)
		return false
	}
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetPipeline(pipelineObj)
	renderPass.SetVertexBuffer(0, r.worldVertexBuffer, 0)
	renderPass.SetIndexBuffer(r.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	passStartUniformOffset := r.uniformOffset
	ptr, lightData := encodeGoGPUWorldDynamicLights(activeDynamicLights)
	err = queue.WriteBuffer(dynamicLightsBuffer, 0, lightData)
	dynamicLightsBytesPool.Put(ptr)
	if err != nil {
		slog.Warn("failed to upload OIT water dynamic lights", "error", err)
		_ = renderPass.End()
		return false
	}

	vpMatrix := r.ViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, camera)
	worldHasLitWater := r.deferredTranslucentLiquidLitWater

	var materialBindState gogpuWorldMaterialBindState
	materialBindState.invalidate()
	for _, face := range faces {
		textureBindGroup := r.resources.WhiteTextureBindGroup
		if r.worldTextures != nil && r.worldTextures.bindGroup != nil {
			textureBindGroup = r.worldTextures.bindGroup
		}
		lightmapBindGroup, litWater := gogpuWorldLightmapArrayBindGroupForFace(face, r.worldLightmapArray, r.resources.WhiteLightmapBindGroup, worldHasLitWater)
		fullbrightBindGroup := r.resources.TransparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = r.resources.WhiteTextureBindGroup
		}
		if r.worldFullbrightTextures != nil && r.worldFullbrightTextures.bindGroup != nil {
			fullbrightBindGroup = r.worldFullbrightTextures.bindGroup
		}

		offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
		if uData == nil {
			continue
		}
		faceAlpha := worldFaceAlpha(face.Flags, r.deferredTranslucentLiquidAlpha)
		fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), timeValue, faceAlpha, litWater)
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{offset})

		setTexture, setLightmap, setFullbright := materialBindState.update(textureBindGroup, lightmapBindGroup, fullbrightBindGroup)
		if setTexture {
			renderPass.SetBindGroup(1, textureBindGroup, nil)
		}
		if setLightmap {
			renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		}
		if setFullbright {
			renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		}
		renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderOITTranslucentWorldLiquidHAL: render pass end error", "error", err)
	}
	if r.uniformOffset > passStartUniformOffset {
		_ = queue.WriteBuffer(r.resources.UniformBuffer, uint64(passStartUniformOffset), r.uniformDataScratch[passStartUniformOffset:r.uniformOffset])
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "OIT World Translucent Liquid Encoder")
	return true
}

func (dc *DrawContext) resolveOITHAL() {
	if dc == nil || dc.renderer == nil {
		return
	}
	r := dc.renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	r.mu.RLock()
	resolvePipeline := r.resources.OITResolvePipeline
	resolveBGL := r.resources.OITResolveBindGroupLayout
	accumView := r.resources.OITAccumTextureView
	revealView := r.resources.OITRevealTextureView
	sampler := r.resources.OITResolveSampler
	r.mu.RUnlock()
	if resolvePipeline == nil || resolveBGL == nil || accumView == nil || revealView == nil || sampler == nil {
		return
	}

	r.mu.Lock()
	if r.resources.OITResolveBindGroup == nil {
		resolveBG, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "OIT Resolve BG",
			Layout: resolveBGL,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: sampler},
				{Binding: 1, TextureView: accumView},
				{Binding: 2, TextureView: revealView},
			},
		})
		if err != nil {
			r.mu.Unlock()
			slog.Warn("OIT resolve: failed to create bind group", "error", err)
			return
		}
		r.resources.OITResolveBindGroup = resolveBG
	}
	resolveBG := r.resources.OITResolveBindGroup
	r.mu.Unlock()

	encoder, encoderOwned, err := dc.frameEncoder(device, "OIT Resolve Encoder")
	if err != nil {
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "OIT Resolve Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
	})
	if err != nil {
		return
	}
	renderPass.SetPipeline(resolvePipeline)
	renderPass.SetBindGroup(0, resolveBG, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "OIT Resolve Encoder")
}