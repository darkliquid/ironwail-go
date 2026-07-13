package renderer

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

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
					HasDynamicOffset: true,
					MinBindingSize:   worldUniformBufferSize,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   256 * 32,
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

	lightmapLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Lightmap Bind Group Layout",
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
		textureLayout.Release()
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create lightmap bind group layout: %w", err)
	}

	lightsLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Dynamic Lights BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeUint,
					ViewDimension: gputypes.TextureViewDimension3D,
					Multisampled:  false,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeReadOnlyStorage,
					HasDynamicOffset: false,
					MinBindingSize:   gogpuWorldDynamicLightBufferSize,
				},
			},
		},
	})
	if err != nil {
		lightmapLayout.Release()
		textureLayout.Release()
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create dynamic lights bind group layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "World Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{
			uniformLayout,
			textureLayout,
			lightmapLayout,
			textureLayout,
			lightsLayout,
		},
	})
	if err != nil {
		lightsLayout.Release()
		lightmapLayout.Release()
		textureLayout.Release()
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	pipeline, err := r.createWorldOpaquePipeline(device, vertexShader, fragmentShader, pipelineLayout)
	if err != nil {
		lightsLayout.Release()
		textureLayout.Release()
		uniformLayout.Release()
		pipelineLayout.Release()
		return nil, nil, fmt.Errorf("create render pipeline: %w", err)
	}

	r.mu.Lock()
	r.uniformBindGroupLayout = uniformLayout
	r.textureBindGroupLayout = textureLayout
	r.lightmapBindGroupLayout = lightmapLayout
	r.worldDynamicLightsBindGroupLayout = lightsLayout
	r.mu.Unlock()

	slog.Debug("World render pipeline created")
	return pipeline, pipelineLayout, nil
}

func (r *Renderer) createWorldOpaquePipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	// The vertex buffer layout tells the GPU how to read the flat byte buffer.
	// ArrayStride (48) must match the byte size written by every vertex-packing
	// function (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
	// aliasVertexBytesInto). The Offset values must match the field offsets in
	// WorldVertex (world/types.go). See docs/VERTEX_LAYOUT.md.
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
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
			CullMode:  gputypes.CullModeFront,
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
	return r.createWorldSkyPipelineWithDepthState(device, vertexShader, fragmentShader, layout, gogpuNonDecalDepthStencilState(false))
}

func (r *Renderer) createWorldSkyPipelineWithDepthWrite(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, depthWrite bool) (*wgpu.RenderPipeline, error) {
	return r.createWorldSkyPipelineWithDepthState(device, vertexShader, fragmentShader, layout, gogpuNonDecalDepthStencilState(depthWrite))
}

func (r *Renderer) createWorldSkyPipelineWithDepthState(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, depthStencil *wgpu.DepthStencilState) (*wgpu.RenderPipeline, error) {
	// The vertex buffer layout tells the GPU how to read the flat byte buffer.
	// ArrayStride (48) must match the byte size written by every vertex-packing
	// function (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
	// aliasVertexBytesInto). The Offset values must match the field offsets in
	// WorldVertex (world/types.go). See docs/VERTEX_LAYOUT.md.
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
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
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: depthStencil,
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

func (r *Renderer) createWorldExternalSkyPipeline(device *wgpu.Device, vertexShader, overlayVertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || vertexShader == nil || overlayVertexShader == nil || fragmentShader == nil || r.uniformBindGroupLayout == nil || r.textureBindGroupLayout == nil || r.worldDynamicLightsBindGroupLayout == nil {
		return nil, nil, nil, nil, fmt.Errorf("missing external sky pipeline inputs")
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
		return nil, nil, nil, nil, fmt.Errorf("create external sky bind group layout: %w", err)
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "World External Sky Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.uniformBindGroupLayout, textureLayout, r.textureBindGroupLayout, r.textureBindGroupLayout, r.worldDynamicLightsBindGroupLayout},
	})
	if err != nil {
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky pipeline layout: %w", err)
	}
	pipeline, err := r.createWorldSkyPipelineWithDepthWrite(device, vertexShader, fragmentShader, layout, true)
	if err != nil {
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky pipeline: %w", err)
	}
	overlayPipeline, err := r.createWorldSkyPipelineWithDepthState(device, overlayVertexShader, fragmentShader, layout, gogpuNonDecalDepthStencilState(false))
	if err != nil {
		pipeline.Release()
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky overlay pipeline: %w", err)
	}
	return pipeline, overlayPipeline, layout, textureLayout, nil
}

func (r *Renderer) createWorldTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	// The vertex buffer layout tells the GPU how to read the flat byte buffer.
	// ArrayStride (48) must match the byte size written by every vertex-packing
	// function (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
	// aliasVertexBytesInto). The Offset values must match the field offsets in
	// WorldVertex (world/types.go). See docs/VERTEX_LAYOUT.md.
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
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
			CullMode:  gputypes.CullModeFront,
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
	// The vertex buffer layout tells the GPU how to read the flat byte buffer.
	// ArrayStride (48) must match the byte size written by every vertex-packing
	// function (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
	// aliasVertexBytesInto). The Offset values must match the field offsets in
	// WorldVertex (world/types.go). See docs/VERTEX_LAYOUT.md.
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
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
			CullMode:  gputypes.CullModeFront,
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
	// The vertex buffer layout tells the GPU how to read the flat byte buffer.
	// ArrayStride (48) must match the byte size written by every vertex-packing
	// function (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
	// aliasVertexBytesInto). The Offset values must match the field offsets in
	// WorldVertex (world/types.go). See docs/VERTEX_LAYOUT.md.
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 48, // == sizeof(WorldVertex) == goGPUWorldVertexStrideBytes
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},  // Position      [3]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1}, // TexCoord      [2]float32
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2}, // LightmapCoord [2]float32
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3}, // Normal        [3]float32
			{Format: gputypes.VertexFormatFloat32, Offset: 40, ShaderLocation: 4},   // LightmapLayer  float32
			{Format: gputypes.VertexFormatUint32, Offset: 44, ShaderLocation: 5},    // MaterialID     uint32
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
			CullMode:  gputypes.CullModeFront,
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
