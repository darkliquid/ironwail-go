package renderer

import (
	"fmt"
	"log/slog"

	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

type gpuDecalVertex struct {
	Position [3]float32
	TexCoord [2]float32
	Color    [4]float32
}

func (r *Renderer) ensureDecalResourcesLocked(device *wgpu.Device, queue *wgpu.Queue) error {
	if device == nil || queue == nil {
		return fmt.Errorf("nil device or queue")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.decalPipeline != nil && r.decalBindGroup != nil && r.decalUniformBindGroup != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, worldgogpu.DecalVertexShaderWGSL, "Decal Vertex Shader")
	if err != nil {
		return fmt.Errorf("create decal vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, worldgogpu.DecalFragmentShaderWGSL, "Decal Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create decal fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Decal Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   worldgogpu.DecalUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal uniform layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Decal Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, r.aliasTextureBindGroupLayout},
	})
	if err != nil {
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Decal Uniform Buffer",
		Size:             worldgogpu.DecalUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Decal Uniform BG",
		Layout:  uniformLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: worldgogpu.DecalUniformBufferSize}},
	})
	if err != nil {
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal uniform bind group: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	pipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "Decal Render Pipeline",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 36,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x4, Offset: 20, ShaderLocation: 2},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: decalDepthStencilState(),
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
	if err != nil {
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal pipeline: %w", err)
	}

	atlasData := generateDecalAtlasData()
	atlasTexture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Decal Atlas Texture",
		Size:          wgpu.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		pipeline.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal atlas texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  atlasTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, atlasData, &wgpu.ImageDataLayout{BytesPerRow: 256 * 4, RowsPerImage: 256}, &wgpu.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1}); err != nil {
		atlasTexture.Release()
		pipeline.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("write decal atlas texture: %w", err)
	}
	atlasView, err := device.CreateTextureView(atlasTexture, &wgpu.TextureViewDescriptor{
		Label:           "Decal Atlas View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		atlasTexture.Release()
		pipeline.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal atlas view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Decal Atlas BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.aliasSampler},
			{Binding: 1, TextureView: atlasView},
			{Binding: 2, TextureView: atlasView},
		},
	})
	if err != nil {
		atlasView.Release()
		atlasTexture.Release()
		pipeline.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create decal bind group: %w", err)
	}

	r.decalVertexShader = vertexShader
	r.decalFragmentShader = fragmentShader
	r.decalUniformLayout = uniformLayout
	r.decalPipelineLayout = pipelineLayout
	r.decalUniformBuffer = uniformBuffer
	r.decalUniformBindGroup = uniformBindGroup
	r.decalPipeline = pipeline
	r.decalAtlasTextureHAL = atlasTexture
	r.decalAtlasView = atlasView
	r.decalBindGroup = bindGroup
	return nil
}

func (dc *DrawContext) renderDecalMarksHAL(marks []DecalMarkEntity) {
	if dc == nil || dc.renderer == nil || len(marks) == 0 {
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

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureDecalResourcesLocked(device, queue); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure decal resources", "error", err)
		return
	}
	if err := r.ensureAliasScratchBufferLocked(device, 36*6); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure decal scratch buffer", "error", err)
		return
	}
	pipeline := r.decalPipeline
	uniformBuffer := r.decalUniformBuffer
	uniformBindGroup := r.decalUniformBindGroup
	bindGroup := r.decalBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()

	draws := prepareDecalDraws(marks, camera)
	if len(draws) == 0 || pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || bindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Decal Render Encoder"})
	if err != nil {
		slog.Warn("failed to create decal encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Decal Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: decalDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderDecalMarksHAL: Failed to begin render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)
	renderPass.SetBindGroup(1, bindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	if err := queue.WriteBuffer(uniformBuffer, 0, worldgogpu.DecalUniformBytes(vpMatrix, 1)); err != nil {
		slog.Warn("failed to upload decal uniform buffer", "error", err)
		return
	}

	preparedDraws := prepareGoGPUDecalHALDraws(draws)
	for _, prepared := range preparedDraws {
		if prepared.VertexCount == 0 {
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, prepared.VertexBytes); err != nil {
			slog.Warn("failed to upload decal vertices", "error", err)
			continue
		}
		renderPass.Draw(prepared.VertexCount, 1, 0, 0)
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderDecalMarksHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish decal encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit decal commands", "error", err)
	}
}

func prepareGoGPUDecalHALDraws(draws []decalDraw) []worldgogpu.PreparedDecalDraw {
	return worldgogpu.PrepareDecalDrawsWithAdapter(draws, gogpuDecalPreparedMark, gogpuDecalQuad)
}

func gogpuDecalPreparedMark(draw decalDraw) (worldgogpu.DecalPreparedMark, bool) {
	return worldgogpu.DecalPreparedMark{
		Params: gogpuDecalMarkParams(draw.mark),
		Color: [4]float32{
			clamp01(draw.mark.Color[0]),
			clamp01(draw.mark.Color[1]),
			clamp01(draw.mark.Color[2]),
			clamp01(draw.mark.Alpha),
		},
	}, true
}

func gogpuDecalMarkParams(mark DecalMarkEntity) worldgogpu.DecalMarkParams {
	return worldgogpu.DecalMarkParams{
		Origin:   mark.Origin,
		Normal:   mark.Normal,
		Size:     mark.Size,
		Rotation: mark.Rotation,
		Variant:  int(mark.Variant),
	}
}

func gogpuDecalQuad(params worldgogpu.DecalMarkParams) ([4][3]float32, bool) {
	return buildDecalQuad(DecalMarkEntity{
		Origin:   params.Origin,
		Normal:   params.Normal,
		Size:     params.Size,
		Rotation: params.Rotation,
	})
}

func decalDepthStencilState() *wgpu.DepthStencilState {
	stencilFace := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionEqual,
		FailOp:      wgpu.StencilOperationKeep,
		DepthFailOp: wgpu.StencilOperationKeep,
		PassOp:      wgpu.StencilOperationIncrementClamp,
	}
	return &wgpu.DepthStencilState{
		Format:              worldDepthTextureFormat,
		DepthWriteEnabled:   false,
		DepthCompare:        gputypes.CompareFunctionLessEqual,
		StencilFront:        stencilFace,
		StencilBack:         stencilFace,
		StencilReadMask:     0xFFFFFFFF,
		StencilWriteMask:    0xFFFFFFFF,
		DepthBias:           -1,
		DepthBiasSlopeScale: -2,
		DepthBiasClamp:      0,
	}
}

func spriteDepthOffsetDepthStencilState() *wgpu.DepthStencilState {
	stencilFace := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      wgpu.StencilOperationKeep,
		DepthFailOp: wgpu.StencilOperationKeep,
		PassOp:      wgpu.StencilOperationKeep,
	}
	return &wgpu.DepthStencilState{
		Format:              worldDepthTextureFormat,
		DepthWriteEnabled:   true,
		DepthCompare:        gputypes.CompareFunctionLessEqual,
		StencilFront:        stencilFace,
		StencilBack:         stencilFace,
		StencilReadMask:     0,
		StencilWriteMask:    0,
		DepthBias:           -1,
		DepthBiasSlopeScale: -2,
		DepthBiasClamp:      0,
	}
}

func decalDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpLoad,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpLoad,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false,
	}
}

// ---- merged from world_late_translucent_gogpu_root.go ----
