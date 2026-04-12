package renderer

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) ensureSpriteResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.spritePipeline != nil && r.spriteDepthOffsetPipeline != nil && r.spriteUniformBuffer != nil && r.spriteUniformBindGroup != nil {
		return nil
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Sprite Uniform Buffer",
		Size:             worldgogpu.SpriteUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create sprite uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Sprite Uniform BG",
		Layout:  r.aliasUniformBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: worldgogpu.SpriteUniformBufferSize}},
	})
	if err != nil {
		uniformBuffer.Release()
		return fmt.Errorf("create sprite uniform bind group: %w", err)
	}

	vertexShader, err := createWorldShaderModule(device, worldgogpu.SpriteVertexShaderWGSL, "Sprite Vertex Shader")
	if err != nil {
		uniformBindGroup.Release()
		uniformBuffer.Release()
		return fmt.Errorf("create sprite vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, worldgogpu.SpriteFragmentShaderWGSL, "Sprite Fragment Shader")
	if err != nil {
		vertexShader.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		return fmt.Errorf("create sprite fragment shader: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "Sprite Render Pipeline",
		Layout: r.aliasPipelineLayout,
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
		DepthStencil: gogpuNonDecalDepthStencilState(false),
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
		vertexShader.Release()
		fragmentShader.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		return fmt.Errorf("create sprite pipeline: %w", err)
	}

	depthOffsetPipeline, err := validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "Sprite Depth Offset Render Pipeline",
		Layout: r.aliasPipelineLayout,
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
			CullMode:  gputypes.CullModeBack,
		},
		DepthStencil: spriteDepthOffsetDepthStencilState(),
		Multisample:  gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    surfaceFormat,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		pipeline.Release()
		vertexShader.Release()
		fragmentShader.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		return fmt.Errorf("create sprite depth-offset pipeline: %w", err)
	}

	r.spriteUniformBuffer = uniformBuffer
	r.spriteUniformBindGroup = uniformBindGroup
	r.spriteVertexShader = vertexShader
	r.spriteFragmentShader = fragmentShader
	r.spritePipeline = pipeline
	r.spriteDepthOffsetPipeline = depthOffsetPipeline
	return nil
}

func (r *Renderer) createSpriteFrameLocked(device *wgpu.Device, queue *wgpu.Queue, frame spriteRenderFrame) (gpuSpriteFrame, error) {
	width, height := frame.width, frame.height
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	pixels := frame.pixels
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	rgba := ConvertPaletteToRGBA(pixels, r.palette)
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Sprite Frame Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuSpriteFrame{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return gpuSpriteFrame{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "Sprite Frame View",
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
		return gpuSpriteFrame{}, fmt.Errorf("create texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Sprite Frame BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.aliasSampler},
			{Binding: 1, TextureView: view},
			{Binding: 2, TextureView: view},
		},
	})
	if err != nil {
		view.Release()
		texture.Release()
		return gpuSpriteFrame{}, fmt.Errorf("create bind group: %w", err)
	}
	return gpuSpriteFrame{
		meta:      frame,
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
	}, nil
}

func (r *Renderer) ensureSpriteModelLocked(device *wgpu.Device, queue *wgpu.Queue, modelID string, spr *model.MSprite) *gpuSpriteModel {
	if modelID == "" || spr == nil {
		return nil
	}
	if cached, ok := r.spriteModels[modelID]; ok {
		return cached
	}
	meta := buildSpriteRenderModel(modelID, spr)
	if meta == nil {
		return nil
	}
	frames := make([]gpuSpriteFrame, 0, len(meta.frames))
	for _, frame := range meta.frames {
		gpuFrame, err := r.createSpriteFrameLocked(device, queue, frame)
		if err != nil {
			slog.Warn("failed to upload sprite frame", "model", modelID, "error", err)
			for _, uploaded := range frames {
				if uploaded.bindGroup != nil {
					uploaded.bindGroup.Release()
				}
				if uploaded.view != nil {
					uploaded.view.Release()
				}
				if uploaded.texture != nil {
					uploaded.texture.Release()
				}
			}
			return nil
		}
		frames = append(frames, gpuFrame)
	}
	model := &gpuSpriteModel{
		modelID:    meta.modelID,
		spriteType: meta.spriteType,
		frames:     frames,
		maxWidth:   meta.maxWidth,
		maxHeight:  meta.maxHeight,
		bounds:     meta.bounds,
	}
	r.spriteModels[modelID] = model
	return model
}

func (r *Renderer) buildSpriteDrawLocked(device *wgpu.Device, queue *wgpu.Queue, entity SpriteEntity) *gpuSpriteDraw {
	if entity.ModelID == "" || entity.Model == nil || entity.Model.Type != model.ModSprite {
		return nil
	}
	draw, ok := worldgogpu.BuildSpriteDraw(worldgogpu.SpriteDrawParams{
		ModelID:    entity.ModelID,
		SpriteData: spriteDataForEntity(entity),
		Frame:      entity.Frame,
		Origin:     entity.Origin,
		Angles:     entity.Angles,
		Alpha:      entity.Alpha,
		Scale:      entity.Scale,
	}, func(modelID string, spriteData *model.MSprite) (worldgogpu.ResolvedSpriteModel[*gpuSpriteModel], bool) {
		sprite := r.ensureSpriteModelLocked(device, queue, modelID, spriteData)
		if sprite == nil {
			return worldgogpu.ResolvedSpriteModel[*gpuSpriteModel]{}, false
		}
		return worldgogpu.ResolvedSpriteModel[*gpuSpriteModel]{
			Handle:     sprite,
			FrameCount: len(sprite.frames),
		}, true
	})
	if !ok {
		return nil
	}
	return &gpuSpriteDraw{
		sprite: draw.Sprite,
		frame:  draw.Frame,
		origin: draw.Origin,
		angles: draw.Angles,
		alpha:  draw.Alpha,
		scale:  draw.Scale,
	}
}

func (dc *DrawContext) collectSpriteDraws(entities []SpriteEntity) []gpuSpriteDraw {
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return nil
	}

	r := dc.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureSpriteResourcesLocked(device); err != nil {
		slog.Warn("failed to ensure sprite resources", "error", err)
		return nil
	}

	draws := make([]gpuSpriteDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildSpriteDrawLocked(device, queue, entity); draw != nil {
			draws = append(draws, *draw)
		}
	}
	return draws
}

func (dc *DrawContext) renderSpriteEntitiesHAL(entities []SpriteEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	draws := dc.collectSpriteDraws(entities)
	if len(draws) == 0 {
		return
	}
	dc.renderSpriteDrawsHAL(draws, fogColor, fogDensity)
}

func (dc *DrawContext) renderSpriteDrawsHAL(draws []gpuSpriteDraw, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(draws) == 0 {
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

	maxVertexBytes := uint64(44 * 6)
	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure sprite scratch buffer", "error", err)
		return
	}
	pipeline := r.spritePipeline
	depthOffsetPipeline := r.spriteDepthOffsetPipeline
	uniformBuffer := r.spriteUniformBuffer
	uniformBindGroup := r.spriteUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || depthOffsetPipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Sprite Render Encoder"})
	if err != nil {
		slog.Warn("failed to create sprite encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Sprite Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderSpriteDrawsHAL: Failed to begin render pass", "error", err)
		return
	}
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	// HAL backends require an active pipeline layout before SetBindGroup.
	renderPass.SetPipeline(pipeline)
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, []uint32{0})

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraAngles := [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
	cameraForward, cameraRight, cameraUp := spriteCameraBasis(cameraAngles)
	currentPipeline := pipeline

	for _, draw := range draws {
		if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
			continue
		}
		frame := draw.sprite.frames[draw.frame]
		if frame.bindGroup == nil {
			continue
		}
		targetPipeline := pipeline
		if spriteUsesOpaqueCutout(draw.sprite.spriteType, draw.alpha) {
			targetPipeline = depthOffsetPipeline
		}
		if currentPipeline != targetPipeline {
			renderPass.SetPipeline(targetPipeline)
			currentPipeline = targetPipeline
		}
		vertices := buildSpriteQuadVertices(&spriteRenderModel{
			modelID:    draw.sprite.modelID,
			spriteType: draw.sprite.spriteType,
			frames:     []spriteRenderFrame{draw.sprite.frames[draw.frame].meta},
			maxWidth:   draw.sprite.maxWidth,
			maxHeight:  draw.sprite.maxHeight,
			bounds:     draw.sprite.bounds,
		}, 0, cameraOrigin, draw.origin, draw.angles, cameraForward, cameraRight, cameraUp, draw.scale)
		if len(vertices) == 0 {
			continue
		}
		triangleVertices := expandSpriteQuadVertices(vertices)
		if len(triangleVertices) == 0 {
			continue
		}
		worldVertices := worldgogpu.ProjectSpriteQuadVerticesToWorldVertices(triangleVertices, func(vertex spriteQuadVertex) worldgogpu.SpriteQuadVertex {
			return worldgogpu.SpriteQuadVertex{
				Position: vertex.Position,
				TexCoord: vertex.TexCoord,
			}
		})
		if err := queue.WriteBuffer(uniformBuffer, 0, worldgogpu.SpriteUniformBytes(vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update sprite uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(worldVertices)); err != nil {
			slog.Warn("failed to upload sprite vertices", "error", err)
			continue
		}
		renderPass.SetBindGroup(1, frame.bindGroup, nil)
		renderPass.Draw(uint32(len(worldVertices)), 1, 0, 0)
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderSpriteDrawsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish sprite encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit sprite commands", "error", err)
	}
}

// ---- merged from world_decal_gogpu_root.go ----
