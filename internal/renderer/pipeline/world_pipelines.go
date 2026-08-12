package pipeline

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// WorldPipelineParams are the layouts and formats every world pipeline needs.
// The parent supplies its owned layouts; nothing here holds Renderer state.
type WorldPipelineParams struct {
	UniformBindGroupLayout  *wgpu.BindGroupLayout
	TextureBindGroupLayout  *wgpu.BindGroupLayout
	LightmapBindGroupLayout *wgpu.BindGroupLayout
	SurfaceFormat           gputypes.TextureFormat
}

// CreateWorldPipeline builds the main world pipeline, its shared pipeline
// layout, and the three shared bind group layouts (uniform-with-lights,
// texture, lightmap) the parent stores and reuses for the sub-pipelines and
// the external sky pipeline layout. The fullbright group reuses the lightmap
// layout and the dynamic lights live in the uniform group, so the pipeline
// fits the browser's 4-bind-group cap.
func CreateWorldPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, params WorldPipelineParams) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, *wgpu.BindGroupLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("invalid shader modules or device")
	}

	// Create bind group layout for @group(0): uniforms + materials +
	// dynamic-lights cluster (fragment reads the 3D cluster texture and the
	// dynamic light storage buffer from bindings 2 and 3). The dynamic
	// lights were previously a separate @group(4); folding them into group 0
	// keeps the pipeline layout at 4 bind groups, which strict-validating
	// browsers cap at.
	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: true,
					MinBindingSize:   WorldUniformBufferSize,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeReadOnlyStorage,
					HasDynamicOffset: false,
					MinBindingSize:   32,
				},
			},
			{
				Binding:    2,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeUint,
					ViewDimension: gputypes.TextureViewDimension3D,
					Multisampled:  false,
				},
			},
			{
				Binding:    3,
				Visibility: gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeReadOnlyStorage,
					HasDynamicOffset: false,
					MinBindingSize:   WorldDynamicLightBufferSize,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("create uniform bind group layout: %w", err)
	}

	// Create bind group layouts for @group(1) textures.
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
		return nil, nil, nil, nil, nil, fmt.Errorf("create texture bind group layout: %w", err)
	}

	// Sky lightmaps bind group layout.
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
		return nil, nil, nil, nil, nil, fmt.Errorf("create lightmap bind group layout: %w", err)
	}

	// Create pipeline layout combining all bind group layouts. Group 3
	// (fullbright, previously a separate group) reuses the lightmap layout
	// shape (sampler + 2D texture), and the dynamic lights live in group 0,
	// so the layout fits the browser's 4-bind-group cap.
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "World Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout, lightmapLayout, lightmapLayout},
	})
	if err != nil {
		uniformLayout.Release()
		textureLayout.Release()
		lightmapLayout.Release()
		return nil, nil, nil, nil, nil, fmt.Errorf("create world pipeline layout: %w", err)
	}

	pipeline, err := CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: NonDecalDepthStencilState(true),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: params.SurfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		layout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		lightmapLayout.Release()
		return nil, nil, nil, nil, nil, fmt.Errorf("create world pipeline: %w", err)
	}

	return pipeline, layout, uniformLayout, textureLayout, lightmapLayout, nil
}

// CreateWorldOpaquePipeline builds the alpha-test (opaque-pass) variant.
func CreateWorldOpaquePipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: NonDecalDepthStencilState(true),
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

// CreateWorldSkyPipeline builds the sky variant with depth-write disabled
// for the opaque skybox pass.
func CreateWorldSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreateWorldSkyPipelineWithDepthState(device, vertexShader, fragmentShader, layout, NonDecalDepthStencilState(false), surfaceFormat)
}

// CreateWorldSkyPipelineWithDepthWrite builds the sky variant, toggling the
// depth write flag.
func CreateWorldSkyPipelineWithDepthWrite(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, depthWrite bool, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreateWorldSkyPipelineWithDepthState(device, vertexShader, fragmentShader, layout, NonDecalDepthStencilState(depthWrite), surfaceFormat)
}

// CreateWorldSkyPipelineWithDepthState builds the sky variant with a fully
// custom depth/stencil state.
func CreateWorldSkyPipelineWithDepthState(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, depthStencil *wgpu.DepthStencilState, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Sky Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
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

// CreateWorldExternalSkyPipeline builds the two external-skybox pipelines
// (full 3D camera-space sky + overlay), their shared pipeline layout, and
// the sky texture bind group layout. It needs the parent's shared layouts
// because the external sky's pipeline layout chains group(0) uniform and
// group(1) sky textures. The external-sky shaders reference only those two
// groups, and strict-validating browsers reject pipeline layouts whose
// bind-group layouts are not all referenced by the shader.
func CreateWorldExternalSkyPipeline(device *wgpu.Device, vertexShader, overlayVertexShader, fragmentShader *wgpu.ShaderModule, params WorldPipelineParams) (*wgpu.RenderPipeline, *wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || vertexShader == nil || overlayVertexShader == nil || fragmentShader == nil ||
		params.UniformBindGroupLayout == nil {
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
		BindGroupLayouts: []*wgpu.BindGroupLayout{params.UniformBindGroupLayout, textureLayout},
	})
	if err != nil {
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky pipeline layout: %w", err)
	}
	pipeline, err := CreateWorldSkyPipelineWithDepthWrite(device, vertexShader, fragmentShader, layout, true, params.SurfaceFormat)
	if err != nil {
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky pipeline: %w", err)
	}
	overlayPipeline, err := CreateWorldSkyPipelineWithDepthState(device, overlayVertexShader, fragmentShader, layout, NonDecalDepthStencilState(false), params.SurfaceFormat)
	if err != nil {
		pipeline.Release()
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, nil, fmt.Errorf("create external sky overlay pipeline: %w", err)
	}
	return pipeline, overlayPipeline, layout, textureLayout, nil
}

// CreateWorldTurbulentPipeline builds the turb (water/lava) variant.
func CreateWorldTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: NonDecalDepthStencilState(true),
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

// CreateWorldTranslucentPipeline builds the alpha-blended translucent
// variant (fences etc.).
func CreateWorldTranslucentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: NonDecalDepthStencilState(false),
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

// CreateWorldTranslucentTurbulentPipeline builds the translucent turb
// variant (underwater surfaces etc.).
func CreateWorldTranslucentTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	return CreatePipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{WorldVertexBufferLayout()},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeFront,
		},
		DepthStencil: NonDecalDepthStencilState(false),
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
