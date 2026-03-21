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
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

type gpuAliasVertexRef struct {
	vertexIndex int
	texCoord    [2]float32
}

type gpuAliasSkin struct {
	texture           hal.Texture
	view              hal.TextureView
	fullbrightTexture hal.Texture
	fullbrightView    hal.TextureView
	bindGroup         hal.BindGroup
}

type gpuAliasModel struct {
	modelID     string
	flags       int
	skins       []gpuAliasSkin
	playerSkins map[uint32][]gpuAliasSkin
	poses       [][]model.TriVertX
	refs        []gpuAliasVertexRef
}

type gpuAliasDraw struct {
	alias  *gpuAliasModel
	model  *model.Model
	pose1  int
	pose2  int
	blend  float32
	skin   *gpuAliasSkin
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
	full   bool
}

const aliasUniformBufferSize = 80

const aliasVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct AliasUniforms {
    viewProjection: mat4x4<f32>,
    alpha: f32,
    _pad0: vec3<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) normal: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: AliasUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.normal = input.normal;
    return output;
}
`

const aliasFragmentShaderWGSL = `
struct AliasUniforms {
    viewProjection: mat4x4<f32>,
    alpha: f32,
    _pad0: vec3<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) normal: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: AliasUniforms;

@group(1) @binding(0)
var skinSampler: sampler;

@group(1) @binding(1)
var skinTexture: texture_2d<f32>;

@group(1) @binding(2)
var fullbrightTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let sampled = textureSample(skinTexture, skinSampler, input.texCoord);
    if (sampled.a < 0.01) {
        discard;
    }
    let fullbright = textureSample(fullbrightTexture, skinSampler, input.texCoord);

    let lightDir = normalize(vec3<f32>(0.35, -0.45, 0.82));
    let normal = normalize(input.normal);
    let diffuse = max(dot(normal, lightDir), 0.25);
    return vec4<f32>(sampled.rgb * diffuse + fullbright.rgb, sampled.a * uniforms.alpha);
}
`

func (r *Renderer) clearAliasModelsLocked() {
	for key, cached := range r.aliasModels {
		for _, skin := range cached.skins {
			if skin.bindGroup != nil {
				skin.bindGroup.Destroy()
			}
			if skin.fullbrightView != nil {
				skin.fullbrightView.Destroy()
			}
			if skin.fullbrightTexture != nil {
				skin.fullbrightTexture.Destroy()
			}
			if skin.view != nil {
				skin.view.Destroy()
			}
			if skin.texture != nil {
				skin.texture.Destroy()
			}
		}
		for _, variants := range cached.playerSkins {
			for _, skin := range variants {
				if skin.bindGroup != nil {
					skin.bindGroup.Destroy()
				}
				if skin.fullbrightView != nil {
					skin.fullbrightView.Destroy()
				}
				if skin.fullbrightTexture != nil {
					skin.fullbrightTexture.Destroy()
				}
				if skin.view != nil {
					skin.view.Destroy()
				}
				if skin.texture != nil {
					skin.texture.Destroy()
				}
			}
		}
		delete(r.aliasModels, key)
	}
}

func (r *Renderer) destroyAliasResourcesLocked() {
	r.clearAliasModelsLocked()
	if r.aliasShadowSkin != nil {
		if r.aliasShadowSkin.bindGroup != nil {
			r.aliasShadowSkin.bindGroup.Destroy()
		}
		if r.aliasShadowSkin.fullbrightView != nil {
			r.aliasShadowSkin.fullbrightView.Destroy()
		}
		if r.aliasShadowSkin.fullbrightTexture != nil {
			r.aliasShadowSkin.fullbrightTexture.Destroy()
		}
		if r.aliasShadowSkin.view != nil {
			r.aliasShadowSkin.view.Destroy()
		}
		if r.aliasShadowSkin.texture != nil {
			r.aliasShadowSkin.texture.Destroy()
		}
		r.aliasShadowSkin = nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Destroy()
		r.aliasScratchBuffer = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Destroy()
		r.aliasUniformBuffer = nil
	}
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Destroy()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasSampler != nil {
		r.aliasSampler.Destroy()
		r.aliasSampler = nil
	}
	if r.aliasPipeline != nil {
		r.aliasPipeline.Destroy()
		r.aliasPipeline = nil
	}
	if r.aliasPipelineLayout != nil {
		r.aliasPipelineLayout.Destroy()
		r.aliasPipelineLayout = nil
	}
	if r.aliasVertexShader != nil {
		r.aliasVertexShader.Destroy()
		r.aliasVertexShader = nil
	}
	if r.aliasFragmentShader != nil {
		r.aliasFragmentShader.Destroy()
		r.aliasFragmentShader = nil
	}
	if r.aliasUniformBindGroupLayout != nil {
		r.aliasUniformBindGroupLayout.Destroy()
		r.aliasUniformBindGroupLayout = nil
	}
	if r.aliasTextureBindGroupLayout != nil {
		r.aliasTextureBindGroupLayout.Destroy()
		r.aliasTextureBindGroupLayout = nil
	}
	r.aliasScratchBufferSize = 0
}

func (r *Renderer) ensureAliasResourcesLocked(device hal.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.aliasPipeline != nil && r.aliasUniformBuffer != nil && r.aliasUniformBindGroup != nil && r.aliasSampler != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, aliasVertexShaderWGSL, "Alias Vertex Shader")
	if err != nil {
		return fmt.Errorf("create alias vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, aliasFragmentShaderWGSL, "Alias Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		return fmt.Errorf("create alias fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Alias Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   aliasUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Alias Texture BGL",
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
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias texture layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Alias Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout, textureLayout},
	})
	if err != nil {
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             aliasUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform buffer: %w", err)
	}

	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Alias Uniform BG",
		Layout: uniformLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   aliasUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform bind group: %w", err)
	}

	sampler, err := device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "Alias Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
	if err != nil {
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias sampler: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Alias Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 44,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
					{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: &hal.DepthStencilState{
			Format:            worldDepthTextureFormat,
			DepthWriteEnabled: true,
			DepthCompare:      gputypes.CompareFunctionLessEqual,
			StencilReadMask:   0xFFFFFFFF,
			StencilWriteMask:  0xFFFFFFFF,
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
		sampler.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias pipeline: %w", err)
	}

	r.aliasVertexShader = vertexShader
	r.aliasFragmentShader = fragmentShader
	r.aliasUniformBindGroupLayout = uniformLayout
	r.aliasTextureBindGroupLayout = textureLayout
	r.aliasPipelineLayout = pipelineLayout
	r.aliasUniformBuffer = uniformBuffer
	r.aliasUniformBindGroup = uniformBindGroup
	r.aliasSampler = sampler
	r.aliasPipeline = pipeline
	return nil
}

func (r *Renderer) ensureAliasScratchBufferLocked(device hal.Device, size uint64) error {
	if size == 0 {
		size = 44
	}
	if r.aliasScratchBuffer != nil && r.aliasScratchBufferSize >= size {
		return nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Destroy()
		r.aliasScratchBuffer = nil
		r.aliasScratchBufferSize = 0
	}
	buffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Alias Scratch Buffer",
		Size:             size,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create alias scratch buffer: %w", err)
	}
	r.aliasScratchBuffer = buffer
	r.aliasScratchBufferSize = size
	return nil
}

func (r *Renderer) ensureAliasDepthTextureLocked(device hal.Device) {
	if r.worldDepthTextureView != nil || device == nil {
		return
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return
	}
	depthTexture, depthView, err := r.createWorldDepthTexture(device, width, height)
	if err != nil {
		slog.Warn("failed to create alias depth texture", "error", err)
		return
	}
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthView
}

func (r *Renderer) ensureAliasModelLocked(device hal.Device, queue hal.Queue, modelID string, mdl *model.Model) *gpuAliasModel {
	if modelID == "" || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	if cached, ok := r.aliasModels[modelID]; ok {
		return cached
	}
	if r.aliasTextureBindGroupLayout == nil || r.aliasSampler == nil {
		return nil
	}

	hdr := mdl.AliasHeader
	if len(hdr.STVerts) != hdr.NumVerts || len(hdr.Triangles) != hdr.NumTris || len(hdr.Poses) == 0 {
		return nil
	}

	skins := make([]gpuAliasSkin, 0, len(hdr.Skins))
	for _, skinPixels := range hdr.Skins {
		skin, err := r.createAliasSkinLocked(device, queue, hdr.SkinWidth, hdr.SkinHeight, skinPixels)
		if err != nil {
			slog.Warn("failed to upload alias skin", "model", modelID, "error", err)
			continue
		}
		skins = append(skins, skin)
	}
	if len(skins) == 0 {
		fallback, err := r.createAliasSkinLocked(device, queue, 1, 1, []byte{0})
		if err != nil {
			return nil
		}
		skins = append(skins, fallback)
	}

	refs := make([]gpuAliasVertexRef, 0, len(hdr.Triangles)*3)
	for _, tri := range hdr.Triangles {
		for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
			idx := int(tri.VertIndex[vertexIndex])
			if idx < 0 || idx >= len(hdr.STVerts) {
				continue
			}
			st := hdr.STVerts[idx]
			s := float32(st.S) + 0.5
			if tri.FacesFront == 0 && st.OnSeam != 0 {
				s += float32(hdr.SkinWidth) * 0.5
			}
			refs = append(refs, gpuAliasVertexRef{
				vertexIndex: idx,
				texCoord: [2]float32{
					s / float32(hdr.SkinWidth),
					(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
				},
			})
		}
	}

	alias := &gpuAliasModel{
		modelID:     modelID,
		flags:       hdr.Flags,
		skins:       skins,
		playerSkins: make(map[uint32][]gpuAliasSkin),
		poses:       hdr.Poses,
		refs:        refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func (r *Renderer) createAliasSkinLocked(device hal.Device, queue hal.Queue, width, height int, pixels []byte) (gpuAliasSkin, error) {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(pixels, r.palette, 0, false)
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Alias Skin Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuAliasSkin{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, baseRGBA, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "Alias Skin View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create texture view: %w", err)
	}
	fullbrightTexture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Alias Fullbright Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  fullbrightTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, fullbrightRGBA, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("write fullbright texture: %w", err)
	}
	fullbrightView, err := device.CreateTextureView(fullbrightTexture, &hal.TextureViewDescriptor{
		Label:           "Alias Fullbright View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Alias Skin BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: fullbrightView.NativeHandle()}},
		},
	})
	if err != nil {
		fullbrightView.Destroy()
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create bind group: %w", err)
	}
	return gpuAliasSkin{
		texture:           texture,
		view:              view,
		fullbrightTexture: fullbrightTexture,
		fullbrightView:    fullbrightView,
		bindGroup:         bindGroup,
	}, nil
}

func (r *Renderer) resolveAliasSkinLocked(device hal.Device, queue hal.Queue, alias *gpuAliasModel, entity AliasModelEntity, slot int) *gpuAliasSkin {
	if alias == nil || slot < 0 {
		return nil
	}
	if entity.IsPlayer {
		if skins, ok := alias.playerSkins[entity.ColorMap]; ok && slot < len(skins) {
			return &skins[slot]
		}
		hdr := entity.Model.AliasHeader
		playerSkins := make([]gpuAliasSkin, len(hdr.Skins))
		for i, skinPixels := range hdr.Skins {
			topColor, bottomColor := splitPlayerColors(byte(entity.ColorMap))
			translated := TranslatePlayerSkinPixels(skinPixels, topColor, bottomColor)
			skin, err := r.createAliasSkinLocked(device, queue, hdr.SkinWidth, hdr.SkinHeight, translated)
			if err != nil {
				slog.Warn("failed to upload translated alias skin", "model", entity.ModelID, "colormap", entity.ColorMap, "error", err)
				return nil
			}
			playerSkins[i] = skin
		}
		alias.playerSkins[entity.ColorMap] = playerSkins
		if slot < len(playerSkins) {
			return &playerSkins[slot]
		}
	}
	if slot < len(alias.skins) {
		return &alias.skins[slot]
	}
	return nil
}

func (r *Renderer) buildAliasDrawLocked(device hal.Device, queue hal.Queue, entity AliasModelEntity, fullAngles bool) *gpuAliasDraw {
	alias := r.ensureAliasModelLocked(device, queue, entity.ModelID, entity.Model)
	if alias == nil || entity.Model == nil || entity.Model.AliasHeader == nil || len(alias.refs) == 0 {
		return nil
	}

	hdr := entity.Model.AliasHeader
	frame := entity.Frame
	if frame < 0 || frame >= len(hdr.Frames) {
		frame = 0
	}
	state := r.ensureAliasStateLocked(entity)
	state.Frame = frame
	interpData, err := SetupAliasFrame(state, aliasHeaderFromModel(hdr), entity.TimeSeconds, true, false, 1)
	if err != nil {
		return nil
	}
	interpData.Origin, interpData.Angles = SetupEntityTransform(
		state,
		entity.TimeSeconds,
		true,
		entity.EntityKey == AliasViewModelEntityKey,
		false,
		false,
		1,
	)
	pose1 := interpData.Pose1
	pose2 := interpData.Pose2
	if pose1 < 0 || pose1 >= len(alias.poses) {
		pose1 = 0
	}
	if pose2 < 0 || pose2 >= len(alias.poses) {
		pose2 = 0
	}

	var skin *gpuAliasSkin
	if len(alias.skins) > 0 {
		slot := resolveAliasSkinSlot(entity.Model.AliasHeader, entity.SkinNum, entity.TimeSeconds, len(alias.skins))
		skin = r.resolveAliasSkinLocked(device, queue, alias, entity, slot)
	}
	if skin == nil && len(alias.skins) > 0 {
		skin = &alias.skins[0]
	}

	alpha, visible := visibleEntityAlpha(entity.Alpha)
	if !visible {
		return nil
	}

	return &gpuAliasDraw{
		alias:  alias,
		model:  entity.Model,
		pose1:  pose1,
		pose2:  pose2,
		blend:  interpData.Blend,
		skin:   skin,
		origin: interpData.Origin,
		angles: interpData.Angles,
		alpha:  alpha,
		scale:  entity.Scale,
		full:   fullAngles,
	}
}

func (dc *DrawContext) renderAliasEntitiesHAL(entities []AliasModelEntity) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	draws := dc.collectAliasDraws(entities, false)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, false)
}

func (dc *DrawContext) renderViewModelHAL(entity AliasModelEntity) {
	if dc == nil || dc.renderer == nil {
		return
	}
	draws := dc.collectAliasDraws([]AliasModelEntity{entity}, true)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, true)
}

func (dc *DrawContext) collectAliasDraws(entities []AliasModelEntity, fullAngles bool) []gpuAliasDraw {
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return nil
	}

	r := dc.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if !fullAngles {
		r.pruneAliasStatesLocked(entities)
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		slog.Warn("failed to initialize alias resources", "error", err)
		return nil
	}
	r.ensureAliasDepthTextureLocked(device)
	draws := make([]gpuAliasDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildAliasDrawLocked(device, queue, entity, fullAngles); draw != nil {
			draws = append(draws, *draw)
		}
	}
	return draws
}

func (dc *DrawContext) renderAliasDrawsHAL(draws []gpuAliasDraw, useViewModelDepthRange bool) {
	if len(draws) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	surfaceViewAny := dc.ctx.SurfaceView()
	textureView, ok := surfaceViewAny.(hal.TextureView)
	if !ok || textureView == nil {
		return
	}

	maxVertexBytes := uint64(44)
	for _, draw := range draws {
		if draw.alias == nil {
			continue
		}
		size := uint64(len(draw.alias.refs) * 44)
		if size > maxVertexBytes {
			maxVertexBytes = size
		}
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Alias Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("alias"); err != nil {
		slog.Warn("failed to begin alias encoding", "error", err)
		return
	}

	renderPassDesc := &hal.RenderPassDescriptor{
		Label: "Alias Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	}
	renderPass := encoder.BeginRenderPass(renderPassDesc)
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		maxDepth := float32(1.0)
		if useViewModelDepthRange {
			maxDepth = 0.3
		}
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, maxDepth)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	for _, draw := range draws {
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertices) == 0 || draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}
		if err := queue.WriteBuffer(uniformBuffer, 0, aliasUniformBytes(vpMatrix, draw.alpha)); err != nil {
			slog.Warn("failed to update alias uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(vertices)); err != nil {
			slog.Warn("failed to upload alias vertices", "error", err)
			continue
		}
		renderPass.SetBindGroup(1, draw.skin.bindGroup, nil)
		renderPass.Draw(uint32(len(vertices)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish alias encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit alias commands", "error", err)
	}
}

func aliasDepthAttachmentForView(view hal.TextureView) *hal.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &hal.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpLoad,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpLoad,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   true,
	}
}

func aliasUniformBytes(vp types.Mat4, alpha float32) []byte {
	data := make([]byte, aliasUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	binary.LittleEndian.PutUint32(data[64:68], math.Float32bits(alpha))
	return data
}

func aliasVertexBytes(vertices []WorldVertex) []byte {
	data := make([]byte, len(vertices)*44)
	for i, v := range vertices {
		offset := i * 44
		putFloat32s(data[offset:offset+12], v.Position[:])
		putFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32s(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32s(data[offset+28:offset+40], v.Normal[:])
	}
	return data
}

func putFloat32s(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

func interpolateVertexPosition(pose1Vert, pose2Vert model.TriVertX, scale, origin [3]float32, factor float32) [3]float32 {
	pos1 := model.DecodeVertex(pose1Vert, scale, origin)
	pos2 := model.DecodeVertex(pose2Vert, scale, origin)
	return [3]float32{
		pos1[0] + (pos2[0]-pos1[0])*factor,
		pos1[1] + (pos2[1]-pos1[1])*factor,
		pos1[2] + (pos2[2]-pos1[2])*factor,
	}
}

func buildAliasVerticesInterpolated(alias *gpuAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	if pose1Index < 0 || pose1Index >= len(alias.poses) || pose2Index < 0 || pose2Index >= len(alias.poses) {
		return nil
	}
	blend = clamp01(blend)
	if entityScale <= 0 {
		entityScale = 1
	}
	pose1 := alias.poses[pose1Index]
	pose2 := alias.poses[pose2Index]
	vertices := make([]WorldVertex, 0, len(alias.refs))
	hdr := mdl.AliasHeader
	for _, ref := range alias.refs {
		if ref.vertexIndex < 0 || ref.vertexIndex >= len(pose1) || ref.vertexIndex >= len(pose2) {
			continue
		}
		position := interpolateVertexPosition(pose1[ref.vertexIndex], pose2[ref.vertexIndex], hdr.Scale, hdr.ScaleOrigin, blend)
		position[0] *= entityScale
		position[1] *= entityScale
		position[2] *= entityScale
		normal := model.GetNormal(pose1[ref.vertexIndex].LightNormalIndex)
		if fullAngles {
			position = rotateAliasAngles(position, angles)
			normal = rotateAliasAngles(normal, angles)
		} else {
			position = rotateAliasYaw(position, angles[1])
			normal = rotateAliasYaw(normal, angles[1])
		}
		position[0] += origin[0]
		position[1] += origin[1]
		position[2] += origin[2]
		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      ref.texCoord,
			LightmapCoord: [2]float32{},
			Normal:        normal,
		})
	}
	return vertices
}

func setupAliasFrameInterpolation(frameIndex int, frames []model.AliasFrameDesc, timeSeconds float64, lerpModels bool, flags int) InterpolationData {
	var result InterpolationData
	if len(frames) == 0 {
		return result
	}
	if frameIndex < 0 || frameIndex >= len(frames) {
		frameIndex = 0
	}
	frameDesc := frames[frameIndex]
	if frameDesc.NumPoses <= 0 {
		result.Pose1 = frameDesc.FirstPose
		result.Pose2 = frameDesc.FirstPose
		return result
	}
	poseOffset := 0
	if frameDesc.NumPoses > 1 {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		poseOffset = int(timeSeconds/float64(interval)) % frameDesc.NumPoses
	}
	currentPose := frameDesc.FirstPose + poseOffset
	if frameDesc.NumPoses <= 1 {
		result.Pose1 = currentPose
		result.Pose2 = currentPose
		return result
	}
	nextPose := frameDesc.FirstPose + (poseOffset+1)%frameDesc.NumPoses
	if lerpModels && (flags&ModNoLerp == 0) {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		timeInInterval := math.Mod(timeSeconds, float64(interval))
		result.Blend = clamp01(float32(timeInInterval / float64(interval)))
	}
	result.Pose1 = currentPose
	result.Pose2 = nextPose
	return result
}

type InterpolationData struct {
	Pose1 int
	Pose2 int
	Blend float32
}

func rotateAliasAngles(v [3]float32, angles [3]float32) [3]float32 {
	v = rotateAliasYaw(v, angles[1])
	v = rotateAliasPitch(v, angles[0])
	v = rotateAliasRoll(v, angles[2])
	return v
}

func rotateAliasYaw(v [3]float32, yawDegrees float32) [3]float32 {
	if yawDegrees == 0 {
		return v
	}
	yaw := float32(math.Pi) * yawDegrees / 180.0
	sinYaw := float32(math.Sin(float64(yaw)))
	cosYaw := float32(math.Cos(float64(yaw)))
	return [3]float32{
		v[0]*cosYaw - v[1]*sinYaw,
		v[0]*sinYaw + v[1]*cosYaw,
		v[2],
	}
}

func rotateAliasPitch(v [3]float32, pitchDegrees float32) [3]float32 {
	if pitchDegrees == 0 {
		return v
	}
	pitch := float32(math.Pi) * pitchDegrees / 180.0
	sinPitch := float32(math.Sin(float64(pitch)))
	cosPitch := float32(math.Cos(float64(pitch)))
	return [3]float32{
		v[0],
		v[1]*cosPitch - v[2]*sinPitch,
		v[1]*sinPitch + v[2]*cosPitch,
	}
}

func rotateAliasRoll(v [3]float32, rollDegrees float32) [3]float32 {
	if rollDegrees == 0 {
		return v
	}
	roll := float32(math.Pi) * rollDegrees / 180.0
	sinRoll := float32(math.Sin(float64(roll)))
	cosRoll := float32(math.Cos(float64(roll)))
	return [3]float32{
		v[0]*cosRoll + v[2]*sinRoll,
		v[1],
		-v[0]*sinRoll + v[2]*cosRoll,
	}
}
