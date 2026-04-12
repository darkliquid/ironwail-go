package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	aliasUniformBufferSize      = 80
	aliasSceneUniformBufferSize = 96
	aliasUniformAlign           = 256 // minUniformBufferOffsetAlignment
	aliasInitialDrawCapacity    = 64  // initial capacity for batched draws
	aliasVertexStride           = 44
)

func (r *Renderer) ensureAliasResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.aliasPipeline != nil && r.aliasShadowPipeline != nil && r.aliasUniformBuffer != nil && r.aliasUniformBindGroup != nil && r.aliasSampler != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, worldgogpu.AliasVertexShaderWGSL, "Alias Vertex Shader")
	if err != nil {
		return fmt.Errorf("create alias vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, worldgogpu.AliasFragmentShaderWGSL, "Alias Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create alias fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Alias Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: true,
				MinBindingSize:   aliasSceneUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
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
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias texture layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Alias Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout},
	})
	if err != nil {
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             uint64(aliasInitialDrawCapacity) * aliasUniformAlign,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform buffer: %w", err)
	}

	uniformBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Alias Uniform BG",
		Layout:  uniformLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: aliasSceneUniformBufferSize}},
	})
	if err != nil {
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform bind group: %w", err)
	}

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
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
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias sampler: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := createAliasRenderPipeline(device, vertexShader, fragmentShader, pipelineLayout, surfaceFormat, "Alias Render Pipeline", true)
	if err != nil {
		sampler.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline: %w", err)
	}
	shadowPipeline, err := createAliasRenderPipeline(device, vertexShader, fragmentShader, pipelineLayout, surfaceFormat, "Alias Shadow Render Pipeline", false)
	if err != nil {
		pipeline.Release()
		sampler.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias shadow pipeline: %w", err)
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
	r.aliasShadowPipeline = shadowPipeline
	return nil
}

func createAliasRenderPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat, label string, depthWrite bool) (*wgpu.RenderPipeline, error) {
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  label,
		Layout: layout,
		Vertex: wgpu.VertexState{
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
		DepthStencil: gogpuNonDecalDepthStencilState(depthWrite),
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
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
}

func (r *Renderer) ensureAliasDepthTextureLocked(device *wgpu.Device) {
	if device == nil {
		return
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return
	}
	// Recreate if nil or dimensions changed (e.g. window resize).
	if r.worldDepthTextureView != nil && r.worldDepthWidth == width && r.worldDepthHeight == height {
		return
	}
	if r.worldDepthTextureView != nil {
		slog.Debug("recreating world depth texture for new dimensions",
			"old", fmt.Sprintf("%dx%d", r.worldDepthWidth, r.worldDepthHeight),
			"new", fmt.Sprintf("%dx%d", width, height))
	}
	// Release old resources.
	if r.worldDepthTextureView != nil {
		r.worldDepthTextureView.Release()
	}
	if r.worldDepthTexture != nil {
		r.worldDepthTexture.Release()
	}
	depthTexture, depthView, err := r.createWorldDepthTexture(device, width, height)
	if err != nil {
		slog.Warn("failed to create alias depth texture", "error", err)
		r.worldDepthTexture = nil
		r.worldDepthTextureView = nil
		r.worldDepthWidth = 0
		r.worldDepthHeight = 0
		return
	}
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthView
	r.worldDepthWidth = width
	r.worldDepthHeight = height
}

func (r *Renderer) ensureAliasModelLocked(device *wgpu.Device, queue *wgpu.Queue, modelID string, mdl *model.Model) *gpuAliasModel {
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

	refs := make([]aliasimpl.MeshRef, 0, len(hdr.Triangles)*3)
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
			refs = append(refs, aliasimpl.MeshRef{
				VertexIndex: idx,
				TexCoord: [2]float32{
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

func (r *Renderer) createAliasSkinLocked(device *wgpu.Device, queue *wgpu.Queue, width, height int, pixels []byte) (gpuAliasSkin, error) {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(pixels, r.palette, 0, false)
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Alias Skin Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuAliasSkin{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, baseRGBA, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
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
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create texture view: %w", err)
	}
	fullbrightTexture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Alias Fullbright Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  fullbrightTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, fullbrightRGBA, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("write fullbright texture: %w", err)
	}
	fullbrightView, err := device.CreateTextureView(fullbrightTexture, &wgpu.TextureViewDescriptor{
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
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Alias Skin BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.aliasSampler},
			{Binding: 1, TextureView: view},
			{Binding: 2, TextureView: fullbrightView},
		},
	})
	if err != nil {
		fullbrightView.Release()
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
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

func (r *Renderer) resolveAliasSkinLocked(device *wgpu.Device, queue *wgpu.Queue, alias *gpuAliasModel, entity AliasModelEntity, slot int) *gpuAliasSkin {
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

func (r *Renderer) buildAliasDrawLocked(device *wgpu.Device, queue *wgpu.Queue, entity AliasModelEntity, fullAngles bool) *gpuAliasDraw {
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
	aliasHdr := aliasHeaderFromModel(hdr)
	aliasHdr.Flags = applyAliasNoLerpListFlags(aliasHdr.Flags, entity.ModelID)
	interpData, err := SetupAliasFrame(state, aliasHdr, entity.TimeSeconds, true, false, 1)
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

func (dc *DrawContext) renderAliasEntitiesHAL(entities []AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	draws := dc.collectAliasDraws(entities, false)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, false, fogColor, fogDensity)
}

func (dc *DrawContext) renderViewModelHAL(entity AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil {
		return
	}
	draws := dc.collectAliasDraws([]AliasModelEntity{entity}, true)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, true, fogColor, fogDensity)
}

func (dc *DrawContext) collectAliasDraws(entities []AliasModelEntity, fullAngles bool) []gpuAliasDraw {
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
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

func (dc *DrawContext) renderAliasDrawsHAL(draws []gpuAliasDraw, useViewModelDepthRange bool, fogColor [3]float32, fogDensity float32) {
	if len(draws) == 0 {
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

	// Pre-build all vertex data and compute total scratch buffer size.
	type preparedDraw struct {
		draw        gpuAliasDraw
		skin        *gpuAliasSkin
		alpha       float32
		vertexCount uint32
	}
	prepared := make([]preparedDraw, 0, len(draws))
	vertexScratch := make([]WorldVertex, 0)
	totalVertexBytes := uint64(0)
	for _, draw := range draws {
		if draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}
		vertexScratch = buildAliasVerticesInterpolatedInto(vertexScratch[:0], draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertexScratch) == 0 {
			continue
		}
		prepared = append(prepared, preparedDraw{
			draw:        draw,
			skin:        draw.skin,
			alpha:       draw.alpha,
			vertexCount: uint32(len(vertexScratch)),
		})
		totalVertexBytes += uint64(len(vertexScratch) * aliasVertexStride)
	}
	if len(prepared) == 0 {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer", "error", err)
		return
	}
	if err := r.ensureAliasUniformBufferLocked(device, len(prepared)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	// Pre-upload all uniform data at 256-byte aligned offsets
	// and all vertex data at consecutive offsets.
	vertexOffsets := make([]uint64, len(prepared))
	vertexCounts := make([]uint32, len(prepared))
	uniformOffsets := make([]uint32, len(prepared))
	vertexByteScratch := make([]byte, 0)
	currentVertexOffset := uint64(0)
	for i, pd := range prepared {
		vertexScratch = buildAliasVerticesInterpolatedInto(vertexScratch[:0], pd.draw.alias, pd.draw.model, pd.draw.pose1, pd.draw.pose2, pd.draw.blend, pd.draw.origin, pd.draw.angles, pd.draw.scale, pd.draw.full)
		if len(vertexScratch) == 0 {
			continue
		}
		uniformOffsets[i] = uint32(i) * aliasUniformAlign
		vertexOffsets[i] = currentVertexOffset
		vertexCounts[i] = pd.vertexCount

		if err := queue.WriteBuffer(uniformBuffer, uint64(uniformOffsets[i]), aliasSceneUniformBytes(vpMatrix, cameraOrigin, pd.alpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update alias uniform buffer", "error", err, "draw", i)
			return
		}
		vertexBytes := aliasVertexBytesInto(vertexByteScratch[:0], vertexScratch)
		vertexByteScratch = vertexBytes[:0]
		if err := queue.WriteBuffer(scratchBuffer, currentVertexOffset, vertexBytes); err != nil {
			slog.Warn("failed to upload alias vertices", "error", err, "draw", i)
			return
		}
		currentVertexOffset += uint64(len(vertexBytes))
	}

	// Record a single render pass with all draws.
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Alias Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Alias Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("failed to begin alias render pass", "error", err)
		return
	}
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

	for i, pd := range prepared {
		renderPass.SetVertexBuffer(0, scratchBuffer, vertexOffsets[i])
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{uniformOffsets[i]})
		renderPass.SetBindGroup(1, pd.skin.bindGroup, nil)
		renderPass.Draw(vertexCounts[i], 1, 0, 0)
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderAliasDrawsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alias encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit alias commands", "error", err)
	}
}

func aliasUniformBytes(vp types.Mat4, alpha float32) []byte {
	data := make([]byte, aliasUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	binary.LittleEndian.PutUint32(data[64:68], math.Float32bits(alpha))
	return data
}

func aliasSceneUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, aliasSceneUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	putFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return data
}

func aliasShadowUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	return aliasSceneUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
}

func aliasVertexBytes(vertices []WorldVertex) []byte {
	return aliasVertexBytesInto(nil, vertices)
}

func aliasVertexBytesInto(dst []byte, vertices []WorldVertex) []byte {
	required := len(vertices) * aliasVertexStride
	data := dst[:0]
	if cap(data) < required {
		data = make([]byte, required)
	} else {
		data = data[:required]
	}
	for i, v := range vertices {
		offset := i * aliasVertexStride
		putFloat32s(data[offset:offset+12], v.Position[:])
		putFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32s(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32s(data[offset+28:offset+40], v.Normal[:])
	}
	return data
}

func buildAliasVerticesInterpolated(alias *gpuAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	return buildAliasVerticesInterpolatedInto(nil, alias, mdl, pose1Index, pose2Index, blend, origin, angles, entityScale, fullAngles)
}

func buildAliasVerticesInterpolatedInto(dst []WorldVertex, alias *gpuAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	return aliasimpl.BuildVerticesInterpolatedInto(
		dst,
		aliasimpl.MeshFromRefs(alias.poses, alias.refs),
		mdl.AliasHeader,
		pose1Index,
		pose2Index,
		blend,
		origin,
		angles,
		entityScale,
		fullAngles,
	)
}

// ---- merged from world_sprite_gogpu_root.go ----
