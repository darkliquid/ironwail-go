package renderer

import (
	"encoding/binary"
	"fmt"
	stdimage "image"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// createWorldVertexBuffer uploads vertex data to GPU
func (r *Renderer) createWorldVertexBuffer(device *wgpu.Device, queue *wgpu.Queue, vertices []WorldVertex) (*wgpu.Buffer, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("no vertices to upload")
	}

	// Calculate size
	vertexSize := uint64(len(vertices)) * 44 // sizeof(WorldVertex) = 44 bytes

	slog.Debug("Creating world vertex buffer",
		"vertexCount", len(vertices),
		"sizeBytes", vertexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Vertices",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create vertex buffer: %w", err)
	}

	// Write vertex data to buffer
	vertexData := make([]byte, vertexSize)
	for i, v := range vertices {
		offset := uint64(i) * 44

		// Write position (3 float32 = 12 bytes)
		posBytes := float32ToBytes(v.Position[:])
		copy(vertexData[offset:offset+12], posBytes)

		// Write texCoord (2 float32 = 8 bytes)
		texBytes := float32ToBytes(v.TexCoord[:])
		copy(vertexData[offset+12:offset+20], texBytes)

		// Write lightmapCoord (2 float32 = 8 bytes)
		lightBytes := float32ToBytes(v.LightmapCoord[:])
		copy(vertexData[offset+20:offset+28], lightBytes)

		// Write normal (3 float32 = 12 bytes)
		normBytes := float32ToBytes(v.Normal[:])
		copy(vertexData[offset+28:offset+40], normBytes)
	}

	queue.WriteBuffer(buffer, 0, vertexData)

	slog.Debug("World vertex buffer uploaded", "vertices", len(vertices))

	return buffer, nil
}

// createWorldIndexBuffer uploads index data to GPU
func (r *Renderer) createWorldIndexBuffer(device *wgpu.Device, queue *wgpu.Queue, indices []uint32) (*wgpu.Buffer, uint32, error) {
	if len(indices) == 0 {
		return nil, 0, fmt.Errorf("no indices to upload")
	}

	indexSize := uint64(len(indices)) * 4 // uint32 = 4 bytes

	slog.Debug("Creating world index buffer",
		"indexCount", len(indices),
		"sizeBytes", indexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Indices",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create index buffer: %w", err)
	}

	// Write index data to buffer
	indexData := make([]byte, indexSize)
	for i, idx := range indices {
		offset := uint64(i) * 4
		binary.LittleEndian.PutUint32(indexData[offset:offset+4], idx)
	}

	queue.WriteBuffer(buffer, 0, indexData)

	slog.Debug("World index buffer uploaded", "indices", len(indices))

	return buffer, uint32(len(indices)), nil
}

func (r *Renderer) ensureGoGPUWorldDynamicIndexBuffer(device *wgpu.Device, size uint64) (*wgpu.Buffer, error) {
	if size == 0 {
		return nil, nil
	}
	if r.worldDynamicIndexBuffer != nil && r.worldDynamicIndexBufferSize >= size {
		return r.worldDynamicIndexBuffer, nil
	}
	if r.worldDynamicIndexBuffer != nil {
		r.worldDynamicIndexBuffer.Release()
		r.worldDynamicIndexBuffer = nil
		r.worldDynamicIndexBufferSize = 0
	}
	allocSize := size
	if allocSize < 4096 {
		allocSize = 4096
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Dynamic Indices",
		Size:             allocSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create dynamic world index buffer: %w", err)
	}
	r.worldDynamicIndexBuffer = buffer
	r.worldDynamicIndexBufferSize = allocSize
	return buffer, nil
}

// createWorldRenderTarget ensures the GoGPU world scene target exists for the current framebuffer size.
func (r *Renderer) createWorldRenderTarget() error {
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid window size: %dx%d", width, height)
	}
	device := r.getWGPUDevice()
	if device == nil {
		return fmt.Errorf("nil wgpu device")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureWorldRenderTargetLocked(device, width, height)
}

// createWorldPipeline creates the render pipeline for world rendering.
// Configures all pipeline state: vertex layout, shaders, depth-stencil, primitive topology, etc.
func (r *Renderer) createWorldPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil {
		return nil, nil, fmt.Errorf("invalid shader modules or device")
	}

	// Create bind group layout for @group(0) @binding(0) uniform buffer.
	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   worldUniformBufferSize,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create uniform bind group layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Texture BGL",
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
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create texture bind group layout: %w", err)
	}

	// Create pipeline layout with the uniform bind group layout.
	pipelineLayoutDesc := &wgpu.PipelineLayoutDescriptor{
		Label:            "World Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout, textureLayout, textureLayout},
	}

	pipelineLayout, err := device.CreatePipelineLayout(pipelineLayoutDesc)
	if err != nil {
		textureLayout.Release()
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	r.mu.Lock()
	r.uniformBindGroupLayout = uniformLayout
	r.textureBindGroupLayout = textureLayout
	r.mu.Unlock()

	pipeline, err := r.createWorldOpaquePipeline(device, vertexShader, fragmentShader, pipelineLayout)
	if err != nil {
		textureLayout.Release()
		uniformLayout.Release()
		pipelineLayout.Release()
		return nil, nil, fmt.Errorf("create render pipeline: %w", err)
	}

	slog.Debug("World render pipeline created")
	return pipeline, pipelineLayout, nil
}

func (r *Renderer) createWorldOpaquePipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(true),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Sky Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldExternalSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil || r.uniformBindGroupLayout == nil {
		return nil, nil, nil, fmt.Errorf("missing external sky pipeline inputs")
	}
	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World External Sky Texture BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 3, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 4, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 5, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 6, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create external sky bind group layout: %w", err)
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "World External Sky Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.uniformBindGroupLayout, textureLayout},
	})
	if err != nil {
		textureLayout.Release()
		return nil, nil, nil, fmt.Errorf("create external sky pipeline layout: %w", err)
	}
	pipeline, err := r.createWorldSkyPipeline(device, vertexShader, fragmentShader, layout)
	if err != nil {
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, fmt.Errorf("create external sky pipeline: %w", err)
	}
	return pipeline, layout, textureLayout, nil
}

func (r *Renderer) createWorldTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(true),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldTranslucentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldTranslucentTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldSolidTexture(device *wgpu.Device, queue *wgpu.Queue, label string, pixel [4]byte) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid device or queue")
	}

	// Create 1x1 RGBA texture descriptor
	textureDesc := &wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}

	// Create the texture
	texture, err := device.CreateTexture(textureDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", label, err)
	}

	err = queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		pixel[:],
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  4, // 1 pixel × 4 bytes
			RowsPerImage: 1,
		},
		&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write %s data: %w", label, err)
	}

	// Create texture view
	textureViewDesc := &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	}

	textureView, err := device.CreateTextureView(texture, textureViewDesc)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create %s view: %w", label, err)
	}

	slog.Debug("World solid texture created", "label", label)
	return texture, textureView, nil
}

// createWorldWhiteTexture creates a simple 1x1 white texture for fallback.
// Used when actual textures are not yet available for rendering.
func (r *Renderer) createWorldWhiteTexture(device *wgpu.Device, queue *wgpu.Queue) (*wgpu.Texture, *wgpu.TextureView, error) {
	return r.createWorldSolidTexture(device, queue, "World White Texture", [4]byte{255, 255, 255, 255})
}

func worldLightmapFallbackView(blackView, whiteView *wgpu.TextureView) *wgpu.TextureView {
	if blackView != nil {
		return blackView
	}
	return whiteView
}

func (r *Renderer) createWorldTextureFromRGBA(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, label string, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, fmt.Errorf("invalid world texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
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
		return nil, fmt.Errorf("create world texture view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Release()
		texture.Release()
		return nil, fmt.Errorf("create world texture bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func shouldDrawGoGPUOpaqueWorldFace(face WorldFace) bool {
	if face.NumIndices == 0 {
		return false
	}
	if face.Flags&(model.SurfDrawSky|model.SurfDrawTurb|model.SurfDrawFence) != 0 {
		return false
	}
	return true
}

func shouldDrawGoGPUAlphaTestWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && worldFacePass(face.Flags, 1) == worldPassAlphaTest
}

func shouldDrawGoGPUSkyWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && face.Flags&model.SurfDrawSky != 0
}

func shouldDrawGoGPUOpaqueLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassOpaque
}

func shouldDrawGoGPUTranslucentLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassTranslucent
}

func (r *Renderer) createWorldTextureSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Texture Sampler",
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeRepeat,
		AddressModeW: gputypes.AddressModeRepeat,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldLightmapSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Lightmap Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldTextureBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, view *wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || view == nil || r.textureBindGroupLayout == nil {
		return nil, fmt.Errorf("missing world texture bind group resources")
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World Texture BG",
		Layout: r.textureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: view},
		},
	})
}

func (r *Renderer) createWorldExternalSkyBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, views [6]*wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || r.worldSkyExternalBindGroupLayout == nil {
		return nil, fmt.Errorf("missing external sky bind group resources")
	}
	for i, view := range views {
		if view == nil {
			return nil, fmt.Errorf("missing external sky texture view %d", i)
		}
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World External Sky BG",
		Layout: r.worldSkyExternalBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: views[0]},
			{Binding: 2, TextureView: views[1]},
			{Binding: 3, TextureView: views[2]},
			{Binding: 4, TextureView: views[3]},
			{Binding: 5, TextureView: views[4]},
			{Binding: 6, TextureView: views[5]},
		},
	})
}

func (r *Renderer) createWorldExternalSkyFaceTexture(device *wgpu.Device, queue *wgpu.Queue, label string, rgba []byte, width, height int) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid external sky texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, nil, fmt.Errorf("invalid external sky texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create external sky texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write external sky texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
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
		return nil, nil, fmt.Errorf("create external sky texture view: %w", err)
	}
	return texture, view, nil
}

func (r *Renderer) ensureGoGPUExternalSkyboxLocked(device *wgpu.Device, queue *wgpu.Queue) error {
	if r.worldSkyExternalMode != externalSkyboxRenderFaces || r.worldSkyExternalLoaded == 0 {
		return nil
	}
	if device == nil || queue == nil || r.worldLightmapSampler == nil || r.worldSkyExternalBindGroupLayout == nil {
		return fmt.Errorf("external sky resources not ready")
	}
	r.destroyGoGPUExternalSkyboxResourcesLocked()
	fallbackPixel := [4]byte{0, 0, 0, 255}
	var views [6]*wgpu.TextureView
	for i, face := range r.worldSkyExternalFaces {
		width := face.Width
		height := face.Height
		data := face.RGBA
		if width <= 0 || height <= 0 || len(data) != width*height*4 {
			width, height = 1, 1
			data = fallbackPixel[:]
		}
		texture, view, err := r.createWorldExternalSkyFaceTexture(device, queue, fmt.Sprintf("World External Sky %s", skyboxFaceSuffixes[i]), data, width, height)
		if err != nil {
			r.destroyGoGPUExternalSkyboxResourcesLocked()
			return err
		}
		r.worldSkyExternalTextures[i] = texture
		r.worldSkyExternalViews[i] = view
		views[i] = view
	}
	bindGroup, err := r.createWorldExternalSkyBindGroup(device, r.worldLightmapSampler, views)
	if err != nil {
		r.destroyGoGPUExternalSkyboxResourcesLocked()
		return fmt.Errorf("create external sky bind group: %w", err)
	}
	r.worldSkyExternalBindGroup = bindGroup
	return nil
}

func (r *Renderer) createWorldDiffuseTexture(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, textureType model.TextureType, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid world texture size %dx%d", width, height)
	}
	if textureType == model.TexTypeCutout {
		cutout := &stdimage.RGBA{
			Pix:    rgba,
			Stride: width * 4,
			Rect:   stdimage.Rect(0, 0, width, height),
		}
		image.AlphaEdgeFix(cutout)
	}
	return r.createWorldTextureFromRGBA(device, queue, sampler, "World Diffuse Texture", rgba, width, height)
}

func (r *Renderer) uploadWorldMaterialTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture, []*SurfaceTexture) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil, nil
	}
	textures := make(map[int32]*gpuWorldTexture, textureCount)
	fullbright := make(map[int32]*gpuWorldTexture)
	textureNames := make([]string, textureCount)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		textureNames[i] = miptex.Name
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		textureType := classifyWorldTextureName(miptex.Name)
		materialRGBA := worldimpl.BuildMaterialTextureRGBA(pixels, r.palette, textureType)
		worldTexture, err := r.createWorldDiffuseTexture(device, queue, sampler, textureType, materialRGBA.DiffuseRGBA, width, height)
		if err != nil {
			slog.Warn("failed to upload world diffuse texture", "texture", miptex.Name, "error", err)
			continue
		}
		textures[int32(i)] = worldTexture
		if !materialRGBA.HasFullbright {
			continue
		}
		fullbrightTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Fullbright Texture", materialRGBA.FullbrightRGBA, width, height)
		if err != nil {
			slog.Warn("failed to upload world fullbright texture", "texture", miptex.Name, "error", err)
			continue
		}
		fullbright[int32(i)] = fullbrightTexture
	}
	animations, err := BuildTextureAnimations(textureNames)
	if err != nil {
		slog.Warn("failed to build world texture animations", "error", err)
	}
	return textures, fullbright, animations
}

func shouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return worldimpl.ShouldSplitAsQuake64Sky(treeVersion, width, height)
}

func extractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	return worldimpl.ExtractEmbeddedSkyLayers(pixels, width, height, palette, quake64)
}

func (r *Renderer) uploadWorldEmbeddedSkyTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil
	}
	solid := make(map[int32]*gpuWorldTexture)
	alpha := make(map[int32]*gpuWorldTexture)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil || classifyWorldTextureName(miptex.Name) != model.TexTypeSky {
			continue
		}
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil {
			continue
		}
		solidRGBA, alphaRGBA, layerWidth, layerHeight, ok := extractEmbeddedSkyLayers(pixels, width, height, r.palette, shouldSplitAsQuake64Sky(tree.Version, width, height))
		if !ok {
			continue
		}
		solidTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Solid Texture", solidRGBA, layerWidth, layerHeight)
		if err != nil {
			slog.Warn("failed to upload world sky solid texture", "texture", miptex.Name, "error", err)
			continue
		}
		alphaTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Alpha Texture", alphaRGBA, layerWidth, layerHeight)
		if err != nil {
			if solidTexture.bindGroup != nil {
				solidTexture.bindGroup.Release()
			}
			if solidTexture.view != nil {
				solidTexture.view.Release()
			}
			if solidTexture.texture != nil {
				solidTexture.texture.Release()
			}
			slog.Warn("failed to upload world sky alpha texture", "texture", miptex.Name, "error", err)
			continue
		}
		solid[int32(i)] = solidTexture
		alpha[int32(i)] = alphaTexture
	}
	return solid, alpha
}

func gogpuWorldTextureForFace(face WorldFace, textures map[int32]*gpuWorldTexture, textureAnimations []*SurfaceTexture, fallback *gpuWorldTexture, frame int, timeSeconds float64) *gpuWorldTexture {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}
	worldTexture := textures[textureIndex]
	if worldTexture == nil && textureIndex != face.TextureIndex {
		worldTexture = textures[face.TextureIndex]
	}
	if worldTexture == nil {
		return fallback
	}
	return worldTexture
}
