package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	aliasUniformBufferSize      = 80
	aliasSceneUniformBufferSize = 96
	aliasInstanceUniformSize    = 80 // per-draw instance params: frame indices, blend, scale, origin, angles, flags
	aliasInitialDrawCapacity    = 64 // initial capacity for batched draws
	aliasVertexStride           = 48 // must match WorldVertex size and every pipeline's ArrayStride — see docs/VERTEX_LAYOUT.md
	aliasRefVertexStride        = 12 // per-vertex stride for GPU ref buffer: uint32 index + vec2 texCoord
)

func (r *Renderer) ensureAliasResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.aliasPipeline != nil && r.aliasUniformBuffer != nil && r.aliasUniformBindGroup != nil && r.aliasSampler != nil {
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

	// Instance bind group layout (group 2):
	//   binding 0: per-draw instance uniform (dynamic offset)
	//   binding 1: per-model pose data (read-only storage buffer)
	instanceLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Alias Instance BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: true,
					MinBindingSize:   aliasInstanceUniformSize,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageVertex,
				Buffer: &gputypes.BufferBindingLayout{
					Type:            gputypes.BufferBindingTypeReadOnlyStorage,
					HasDynamicOffset: false,
					MinBindingSize:  4,
				},
			},
		},
	})
	if err != nil {
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias instance layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Alias Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout, instanceLayout},
	})
	if err != nil {
		instanceLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             uint64(aliasInitialDrawCapacity) * worldUniformAlign,
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

	// Instance uniform buffer: per-draw params (frame indices, blend, transform)
	instanceUniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Instance Uniform Buffer",
		Size:             uint64(aliasInitialDrawCapacity) * worldUniformAlign,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		instanceLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias instance uniform buffer: %w", err)
	}
	// The instance bind group is per-model (binding 1 changes per model),
	// but the instance uniform buffer at binding 0 is shared. The per-model
	// bind groups are created lazily in ensureAliasModelLocked.
	instanceUniformBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Alias Instance Uniform BG",
		Layout: instanceLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: instanceUniformBuffer, Offset: 0, Size: aliasInstanceUniformSize},
			{Binding: 1, Buffer: instanceUniformBuffer, Offset: 0, Size: 4},
		},
	})
	if err != nil {
		instanceUniformBuffer.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		instanceLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias instance uniform bind group: %w", err)
	}

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label: "Alias Sampler",
		// C alias skins are uploaded without TEXPREF_CLAMP (see gl_model.c
		// R_LoadSkin), so the GL default GL_REPEAT wrap applies. Match that
		// here: some v_*.mdl seam/back-facing triangles have UVs that land
		// exactly on the skin edge (u == 1.0), and ClampToEdge would sample
		// the wrong column of the skin (e.g. nailgun grate triangles picking
		// up the dark barrel-cap column).
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeRepeat,
		AddressModeW: gputypes.AddressModeRepeat,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
	if err != nil {
		instanceUniformBindGroup.Release()
		instanceUniformBuffer.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		instanceLayout.Release()
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
		instanceUniformBindGroup.Release()
		instanceUniformBuffer.Release()
		sampler.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		instanceLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline: %w", err)
	}

	r.aliasVertexShader = vertexShader
	r.aliasFragmentShader = fragmentShader
	r.aliasUniformBindGroupLayout = uniformLayout
	r.aliasTextureBindGroupLayout = textureLayout
	r.aliasInstanceBindGroupLayout = instanceLayout
	r.aliasPipelineLayout = pipelineLayout
	r.aliasUniformBuffer = uniformBuffer
	r.aliasUniformBindGroup = uniformBindGroup
	r.aliasInstanceUniformBuffer = instanceUniformBuffer
	r.aliasInstanceUniformBindGroup = instanceUniformBindGroup
	r.aliasSampler = sampler
	r.aliasPipeline = pipeline
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
				ArrayStride: aliasRefVertexStride,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 4, ShaderLocation: 1},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
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
		scale:       hdr.Scale,
		scaleOrigin: hdr.ScaleOrigin,
	}
	r.aliasModels[modelID] = alias

	// Upload all pose data (TriVertX = 4 bytes each) as a single GPU
	// storage buffer. Layout: [pose0_vert0, pose0_vert1, ..., pose1_vert0, ...]
	// Total = numPoses * numVerts * 4 bytes.
	numVerts := len(hdr.STVerts)
	if numVerts > 0 && len(hdr.Poses) > 0 {
		totalPoseBytes := len(hdr.Poses) * numVerts * 4
		poseData := make([]byte, totalPoseBytes)
		for poseIdx, pose := range hdr.Poses {
			off := poseIdx * numVerts * 4
			for vi := 0; vi < len(pose) && vi < numVerts; vi++ {
				copy(poseData[off+vi*4:off+vi*4+4], []byte{pose[vi].V[0], pose[vi].V[1], pose[vi].V[2], pose[vi].LightNormalIndex})
			}
		}
		poseBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Alias Pose Buffer",
			Size:             uint64(totalPoseBytes),
			Usage:            gputypes.BufferUsageStorage | gputypes.BufferUsageCopyDst,
			MappedAtCreation: false,
		})
		if err != nil {
			slog.Warn("failed to create alias pose buffer", "model", modelID, "error", err)
		} else {
			queue.WriteBuffer(poseBuf, 0, poseData)
			alias.poseBuffer = poseBuf
		}
	}

	// Upload vertex reference data as a GPU vertex buffer.
	// Each vertex: uint32 vertexIndex + vec2 texCoord = 12 bytes.
	// We use a custom vertex buffer layout (separate from the WorldVertex
	// 48-byte stride) so the GPU can fetch vertex indices and UVs.
	if len(refs) > 0 {
		vtxStride := uint64(12)
		vtxSize := vtxStride * uint64(len(refs))
		vtxData := make([]byte, vtxSize)
		for i, ref := range refs {
			off := i * 12
			binary.LittleEndian.PutUint32(vtxData[off:off+4], uint32(ref.VertexIndex))
			binary.LittleEndian.PutUint32(vtxData[off+4:off+8], math.Float32bits(ref.TexCoord[0]))
			binary.LittleEndian.PutUint32(vtxData[off+8:off+12], math.Float32bits(ref.TexCoord[1]))
		}
		vtxBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Alias Vertex Ref Buffer",
			Size:             vtxSize,
			Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
			MappedAtCreation: false,
		})
		if err != nil {
			slog.Warn("failed to create alias vertex buffer", "model", modelID, "error", err)
		} else {
			queue.WriteBuffer(vtxBuf, 0, vtxData)
			alias.vertexBuffer = vtxBuf
			alias.vertexCount = uint32(len(refs))
		}
	}

	// Create per-model instance bind group (group 2) referencing the shared
	// instance uniform buffer at binding 0 and this model's pose buffer at binding 1.
	if alias.poseBuffer != nil && r.aliasInstanceUniformBuffer != nil && r.aliasInstanceBindGroupLayout != nil {
		bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Alias Instance BG",
			Layout: r.aliasInstanceBindGroupLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Buffer: r.aliasInstanceUniformBuffer, Offset: 0, Size: aliasInstanceUniformSize},
				{Binding: 1, Buffer: alias.poseBuffer, Offset: 0, Size: alias.poseBuffer.Size()},
			},
		})
		if err != nil {
			slog.Warn("failed to create alias instance bind group", "model", modelID, "error", err)
		} else {
			alias.instanceBindGroup = bg
		}
	}

	// Debug: dump first skin bytes and first N emitted UVs to help diagnose
	// UV mis-mapping on alias models (e.g. nailgun end-segment artifact).
	// Run with -loglvl DEBUG to enable.
	{
		dbgSkinBytes := []byte(nil)
		if len(hdr.Skins) > 0 {
			n := min(64, len(hdr.Skins[0]))
			dbgSkinBytes = hdr.Skins[0][:n]
		}
		dbgUVs := make([][2]float32, min(10, len(refs)))
		for i := range dbgUVs {
			dbgUVs[i] = refs[i].TexCoord
		}
		slog.Debug("alias model built",
			"model", modelID,
			"skinW", hdr.SkinWidth, "skinH", hdr.SkinHeight,
			"numSkins", len(hdr.Skins),
			"numRefs", len(refs),
			"skin0_head64", fmt.Sprintf("%v", dbgSkinBytes),
			"first10UVs", fmt.Sprintf("%v", dbgUVs),
		)
	}

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

	// Quick check: are there any valid draws with GPU buffers?
	hasValidDraw := false
	for _, draw := range draws {
		if draw.skin != nil && draw.skin.bindGroup != nil &&
			draw.alias.vertexBuffer != nil && draw.alias.instanceBindGroup != nil {
			hasValidDraw = true
			break
		}
	}
	if !hasValidDraw {
		return
	}

	r := dc.renderer
	vpMatrix := r.ViewProjectionMatrix()
	r.mu.Lock()
	camera := r.cameraState
	r.mu.Unlock()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	// Reset persistent scratch buffers on DrawContext
	dc.aliasPreparedScratch = dc.aliasPreparedScratch[:0]
	dc.aliasBulkUniformData = dc.aliasBulkUniformData[:0]
	dc.aliasBulkInstanceData = dc.aliasBulkInstanceData[:0]
	dc.aliasVertexCounts = dc.aliasVertexCounts[:0]
	dc.aliasUniformOffsets = dc.aliasUniformOffsets[:0]
	dc.aliasInstanceOffsets = dc.aliasInstanceOffsets[:0]

	// Pack per-draw scene uniforms and instance uniforms.
	for _, draw := range draws {
		if draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}
		if draw.alias.vertexBuffer == nil || draw.alias.instanceBindGroup == nil {
			continue
		}

		drawIdx := uint32(len(dc.aliasPreparedScratch))
		uOffset := drawIdx * worldUniformAlign

		dc.aliasPreparedScratch = append(dc.aliasPreparedScratch, gpuPreparedAliasDraw{
			draw:        draw,
			skin:        draw.skin,
			alpha:       draw.alpha,
			vertexCount: draw.alias.vertexCount,
		})
		dc.aliasUniformOffsets = append(dc.aliasUniformOffsets, uOffset)
		dc.aliasInstanceOffsets = append(dc.aliasInstanceOffsets, uOffset)
		dc.aliasVertexCounts = append(dc.aliasVertexCounts, draw.alias.vertexCount)

		// Pack scene uniform (VP, fog, alpha)
		dc.aliasBulkUniformData = appendAliasSceneUniformBytes(dc.aliasBulkUniformData, uOffset, vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)

		// Pack instance uniform (frame indices, blend, scale, origin, angles)
		dc.aliasBulkInstanceData = appendAliasInstanceUniformBytes(dc.aliasBulkInstanceData, uOffset,
			draw.pose1, draw.pose2, draw.blend, draw.scale,
			draw.alias.scale, draw.alias.scaleOrigin,
			draw.origin, draw.angles, draw.full, len(draw.alias.poses))
	}

	if len(dc.aliasPreparedScratch) == 0 {
		return
	}

	r.mu.Lock()
	if err := r.ensureAliasUniformBufferLocked(device, len(dc.aliasPreparedScratch)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer", "error", err)
		return
	}
	if err := r.ensureAliasInstanceUniformBufferLocked(device, len(dc.aliasPreparedScratch)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias instance uniform buffer", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	instanceUniformBuffer := r.aliasInstanceUniformBuffer
	depthView := r.worldDepthTextureView
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || instanceUniformBuffer == nil {
		return
	}

	// Bulk upload scene uniforms
	if len(dc.aliasBulkUniformData) > 0 {
		if err := queue.WriteBuffer(uniformBuffer, 0, dc.aliasBulkUniformData); err != nil {
			slog.Warn("failed to upload alias uniform buffer in bulk", "error", err)
			return
		}
	}

	// Bulk upload instance uniforms
	if len(dc.aliasBulkInstanceData) > 0 {
		if err := queue.WriteBuffer(instanceUniformBuffer, 0, dc.aliasBulkInstanceData); err != nil {
			slog.Warn("failed to upload alias instance uniform buffer in bulk", "error", err)
			return
		}
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

	for i, pd := range dc.aliasPreparedScratch {
		renderPass.SetVertexBuffer(0, pd.draw.alias.vertexBuffer, 0)
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{dc.aliasUniformOffsets[i]})
		renderPass.SetBindGroup(1, pd.skin.bindGroup, nil)
		renderPass.SetBindGroup(2, pd.draw.alias.instanceBindGroup, []uint32{dc.aliasInstanceOffsets[i]})
		renderPass.Draw(dc.aliasVertexCounts[i], 1, 0, 0)
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

// ---- merged from world_sprite_gogpu_root.go ----
