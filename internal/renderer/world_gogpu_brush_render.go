package renderer

import (
	"log/slog"

	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (dc *DrawContext) renderOpaqueBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
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

	scratch := gogpuBrushPrepScratchPool.Get().(*gogpuBrushPrepScratch)
	defer gogpuBrushPrepScratchPool.Put(scratch)
	scratch.classifiedBuild = scratch.classifiedBuild[:0]
	scratch.classifiedDraws = scratch.classifiedDraws[:0]
	scratch.classifiedPrepared = scratch.classifiedPrepared[:0]
	scratch.vertexData = scratch.vertexData[:0]
	scratch.indexData = scratch.indexData[:0]
	for _, entity := range entities {
		geom := dc.renderer.brushEntityGeometry(entity)
		scratch.classifiedBuild = append(scratch.classifiedBuild, worldgogpu.ClassifiedBrushEntityDraw{})
		buildDraw := &scratch.classifiedBuild[len(scratch.classifiedBuild)-1]
		if !worldgogpu.FillClassifiedBrushEntityDraw(buildDraw, gogpuBrushEntityParams(entity), geom, classifyGoGPUBrushEntityFace) {
			scratch.classifiedBuild = scratch.classifiedBuild[:len(scratch.classifiedBuild)-1]
			continue
		}
		extTextures, extFullbright, extAnimations, extBindGroup := dc.renderer.brushEntityTextures(entity)
		scratch.classifiedDraws = append(scratch.classifiedDraws, gogpuClassifiedBrushEntityDraw{
			alpha:                  buildDraw.Alpha,
			frame:                  buildDraw.Frame,
			vertices:               buildDraw.Vertices,
			opaqueIndices:          buildDraw.OpaqueIndices,
			opaqueFaces:            buildDraw.OpaqueFaces,
			opaqueCenters:          buildDraw.OpaqueCenters,
			alphaTestIndices:       buildDraw.AlphaTestIndices,
			alphaTestFaces:         buildDraw.AlphaTestFaces,
			alphaTestCenters:       buildDraw.AlphaTestCenters,
			lightmapArray:          dc.renderer.brushEntityLightmaps(entity, geom),
			textures:               extTextures,
			fullbrightTextures:     extFullbright,
			textureAnimations:      extAnimations,
			uniformBindGroup:       extBindGroup,
			uniformBindGroupFrame1: dc.renderer.brushEntityUniformBindGroupFrame1(entity),
		})
		drawIndex := len(scratch.classifiedDraws) - 1
		draw := &scratch.classifiedDraws[drawIndex]
		vertexOffset := uint64(len(scratch.vertexData))
		scratch.vertexData = appendGoGPUWorldVertexBytes(scratch.vertexData, draw.vertices)
		opaqueIndexOffset := uint64(len(scratch.indexData))
		scratch.indexData = appendGoGPUWorldIndexBytes(scratch.indexData, draw.opaqueIndices)
		alphaTestIndexOffset := uint64(len(scratch.indexData))
		scratch.indexData = appendGoGPUWorldIndexBytes(scratch.indexData, draw.alphaTestIndices)
		scratch.classifiedPrepared = append(scratch.classifiedPrepared, gogpuPreparedClassifiedBrushDraw{
			drawIndex:            drawIndex,
			vertexOffset:         vertexOffset,
			opaqueIndexOffset:    opaqueIndexOffset,
			alphaTestIndexOffset: alphaTestIndexOffset,
		})
	}
	if len(scratch.classifiedPrepared) == 0 {
		return
	}
	totalVertexBytes := uint64(len(scratch.vertexData))
	totalIndexBytes := uint64(len(scratch.indexData))

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureBrushEntityScratchBuffersLocked(device, totalVertexBytes, totalIndexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure brush entity scratch buffers", "error", err)
		return
	}
	pipeline := r.worldPipeline
	alphaTestPipeline := r.worldAlphaTestPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	transparentBindGroup := r.transparentBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	vertexScratchBuffer := r.brushEntityScratchVertexBuffer
	indexScratchBuffer := r.brushEntityScratchIndexBuffer
	depthView := r.worldDepthTextureView
	dynamicLightsBuffer := r.worldDynamicLightsBuffer
	dynamicLightsBindGroup := r.worldDynamicLightsBindGroup
	camera := r.cameraState
	worldTextures := r.worldTextures
	worldFullbrightTextures := r.worldFullbrightTextures

	r.brushTextureAnimationsScratch = append(r.brushTextureAnimationsScratch[:0], r.worldTextureAnimations...)

	worldLightmapArray := r.worldLightmapArray

	r.activeDynamicLightsScratch = r.activeDynamicLightsScratch[:0]
	if r.lightPool != nil {
		r.activeDynamicLightsScratch = append(r.activeDynamicLightsScratch, r.lightPool.ActiveLights()...)
	}
	activeDynamicLights := r.activeDynamicLightsScratch
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || dynamicLightsBuffer == nil || dynamicLightsBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}

	// Create mapped scratch buffers per-pass (bypassing queue.WriteBuffer + stagingBelt)
	if len(scratch.vertexData) > 0 {
		vSize := uint64(len(scratch.vertexData))
		buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Brush Entity Vertex Scratch Buffer",
			Size:             vSize,
			Usage:            gputypes.BufferUsageVertex,
			MappedAtCreation: true,
		})
		if err != nil {
			slog.Warn("failed to create brush scratch vertex buffer", "error", err)
			return
		}
		if mr, mrErr := buf.MappedRange(0, vSize); mrErr == nil && mr != nil {
			copy(mr.Bytes(), scratch.vertexData)
		}
		buf.Unmap()
		vertexScratchBuffer = buf
		defer vertexScratchBuffer.Release()
	}
	if len(scratch.indexData) > 0 {
		iSize := uint64(len(scratch.indexData))
		buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Brush Entity Index Scratch Buffer",
			Size:             iSize,
			Usage:            gputypes.BufferUsageIndex,
			MappedAtCreation: true,
		})
		if err != nil {
			slog.Warn("failed to create brush scratch index buffer", "error", err)
			return
		}
		if mr, mrErr := buf.MappedRange(0, iSize); mrErr == nil && mr != nil {
			copy(mr.Bytes(), scratch.indexData)
		}
		buf.Unmap()
		indexScratchBuffer = buf
		defer indexScratchBuffer.Release()
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Brush Entity Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush entity encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Brush Entity Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderOpaqueBrushEntitiesHAL: Failed to begin render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	passStartUniformOffset := r.uniformOffset
	ptr1, lightData1 := encodeGoGPUWorldDynamicLights(activeDynamicLights)
	err1 := queue.WriteBuffer(dynamicLightsBuffer, 0, lightData1)
	dynamicLightsBytesPool.Put(ptr1)
	if err1 != nil {
		slog.Warn("failed to upload brush dynamic lights", "error", err1)
		return
	}
	renderPass.SetBindGroup(4, dynamicLightsBindGroup, nil)

	vpMatrix := r.ViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	var materialBindState gogpuWorldMaterialBindState
	for _, preparedDraw := range scratch.classifiedPrepared {
		draw := scratch.classifiedDraws[preparedDraw.drawIndex]
		renderPass.SetVertexBuffer(0, vertexScratchBuffer, preparedDraw.vertexOffset)
		drawTextures := worldTextures
		drawFullbright := worldFullbrightTextures
		if draw.textures != nil {
			drawTextures = draw.textures
		}
		if draw.fullbrightTextures != nil {
			drawFullbright = draw.fullbrightTextures
		}

		// Select the frame-1 uniform bind group when the entity's frame != 0
		// so the shader reads alternate texture atlas bounds/layer (pressed
		// button, activated switch textures) from the frame-1 materials buffer.
		frameUniformBindGroup := uniformBindGroup
		if draw.frame != 0 && draw.uniformBindGroupFrame1 != nil {
			frameUniformBindGroup = draw.uniformBindGroupFrame1
		} else if draw.uniformBindGroup != nil {
			frameUniformBindGroup = draw.uniformBindGroup
		}

		if len(draw.opaqueFaces) > 0 {
			// Ensure the opaque pipeline is active (may have been
			// switched to alpha-test by a previous draw).
			renderPass.SetPipeline(pipeline)
			materialBindState.invalidate()
			renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.opaqueIndexOffset)
			for _, face := range draw.opaqueFaces {
				offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
				fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), camera.Time, draw.alpha, 0)

				renderPass.SetBindGroup(0, frameUniformBindGroup, []uint32{offset})
				textureBindGroup := whiteTextureBindGroup
				if drawTextures != nil && drawTextures.bindGroup != nil {
					textureBindGroup = drawTextures.bindGroup
				}
				lightmapBindGroup := whiteLightmapBindGroup
				if face.LightmapIndex >= 0 {
					if draw.lightmapArray != nil && draw.lightmapArray.bindGroup != nil {
						lightmapBindGroup = draw.lightmapArray.bindGroup
					} else if worldLightmapArray != nil && worldLightmapArray.bindGroup != nil {
						lightmapBindGroup = worldLightmapArray.bindGroup
					}
				}
				fullbrightBindGroup := transparentBindGroup
				if drawFullbright != nil && drawFullbright.bindGroup != nil {
					fullbrightBindGroup = drawFullbright.bindGroup
				}
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
		}
		if len(draw.alphaTestFaces) > 0 {
			// Switch to the alpha-test pipeline for cutout/fence faces.
			// The opaque pipeline doesn't have the discard logic, so
			// transparent pixels would render as solid black.
			if alphaTestPipeline != nil {
				renderPass.SetPipeline(alphaTestPipeline)
				materialBindState.invalidate()
			}
			renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.alphaTestIndexOffset)
			for _, face := range draw.alphaTestFaces {
				offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
				fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), camera.Time, draw.alpha, 0)

				renderPass.SetBindGroup(0, frameUniformBindGroup, []uint32{offset})
				textureBindGroup := whiteTextureBindGroup
				if drawTextures != nil && drawTextures.bindGroup != nil {
					textureBindGroup = drawTextures.bindGroup
				}
				lightmapBindGroup := whiteLightmapBindGroup
				if face.LightmapIndex >= 0 {
					if draw.lightmapArray != nil && draw.lightmapArray.bindGroup != nil {
						lightmapBindGroup = draw.lightmapArray.bindGroup
					} else if worldLightmapArray != nil && worldLightmapArray.bindGroup != nil {
						lightmapBindGroup = worldLightmapArray.bindGroup
					}
				}
				fullbrightBindGroup := transparentBindGroup
				if drawFullbright != nil && drawFullbright.bindGroup != nil {
					fullbrightBindGroup = drawFullbright.bindGroup
				}
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
		}
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderOpaqueBrushEntitiesHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush entity encoding", "error", err)
		return
	}
	if r.uniformOffset > passStartUniformOffset {
		_ = queue.WriteBuffer(uniformBuffer, uint64(passStartUniformOffset), r.uniformDataScratch[passStartUniformOffset:r.uniformOffset])
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush entity commands", "error", err)
	}
}

func (dc *DrawContext) renderSkyBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
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

	draws := make([]gogpuOpaqueBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.brushEntityGeometry(entity)
		if draw := buildGoGPUSkyBrushEntityDraw(entity, geom); draw != nil {
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return
	}

	r := dc.renderer
	r.mu.RLock()
	var treeEntities []byte
	skyPipeline := r.worldSkyPipeline
	externalSkyOverlayPipeline := r.worldSkyExternalOverlayPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	transparentBindGroup := r.transparentBindGroup
	dynamicLightsBuffer := r.worldDynamicLightsBuffer
	dynamicLightsBindGroup := r.worldDynamicLightsBindGroup
	worldSkySolidTextures := r.worldSkySolidTextures
	worldSkyAlphaTextures := r.worldSkyAlphaTextures
	externalSkyMode := r.worldSkyExternalMode
	externalSkyBindGroup := r.worldSkyExternalBindGroup
	externalSkyWind := r.worldSkyExternalWind
	externalSkyWindLoaded := r.worldSkyExternalWindLoaded
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if uniformBuffer == nil || uniformBindGroup == nil || dynamicLightsBuffer == nil || dynamicLightsBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}
	useExternalSky := externalSkyMode == externalSkyboxRenderFaces && externalSkyOverlayPipeline != nil && externalSkyBindGroup != nil && whiteTextureBindGroup != nil
	if !useExternalSky && (skyPipeline == nil || whiteTextureBindGroup == nil) {
		return
	}
	skyFogDensity := gogpuWorldSkyFogDensity(treeEntities, fogDensity)

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Brush Sky Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush sky encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Brush Sky Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderSkyBrushEntitiesHAL: Failed to begin render pass", "error", err)
		return
	}
	logExternalSkyDraw := useExternalSky && !r.worldSkyExternalBrushDrawLogged
	if logExternalSkyDraw {
		totalFaces := 0
		totalIndices := uint32(0)
		for _, draw := range draws {
			totalFaces += len(draw.faces)
			for _, face := range draw.faces {
				totalIndices += face.NumIndices
			}
		}
		slog.Info("external sky brush draw begin", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName, "draws", len(draws), "faces", totalFaces, "indices", totalIndices)
	}
	if useExternalSky {
		renderPass.SetPipeline(externalSkyOverlayPipeline)
		renderPass.SetBindGroup(1, externalSkyBindGroup, nil)
		// Keep the external sky pipeline layout compatible with the world
		// pipeline layout; placeholder groups satisfy WebGPU validation.
		renderPass.SetBindGroup(2, whiteTextureBindGroup, nil)
		renderPass.SetBindGroup(3, whiteTextureBindGroup, nil)
		if logExternalSkyDraw {
			slog.Info("external sky brush draw pipeline bound", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
		}
	} else {
		renderPass.SetPipeline(skyPipeline)
	}
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	passStartUniformOffset := r.uniformOffset
	ptr3, lightData3 := encodeGoGPUWorldDynamicLights(activeDynamicLights)
	err3 := queue.WriteBuffer(dynamicLightsBuffer, 0, lightData3)
	dynamicLightsBytesPool.Put(ptr3)
	if err3 != nil {
		slog.Warn("failed to upload brush dynamic lights", "error", err3)
		_ = renderPass.End()
		return
	}
	renderPass.SetBindGroup(4, dynamicLightsBindGroup, nil)

	vpMatrix := r.ViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	buffers := make([]*wgpu.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Sky Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush sky vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to upload brush sky vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Sky Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to create brush sky index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Release()
			vertexBuffer.Release()
			slog.Warn("failed to upload brush sky index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
		if useExternalSky {
			fillWorldSceneUniformBytesWithExternalSkyWind(uData, vpMatrix, cameraOrigin, fogColor, skyFogDensity, camera.Time, externalSkyWind, externalSkyWindLoaded)
		} else {
			fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, skyFogDensity, camera.Time, 1, 0)
		}
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{offset})
		renderPass.SetVertexBuffer(0, vertexBuffer, 0)
		renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
		for _, face := range draw.faces {
			if useExternalSky {
				renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
				continue
			}
			solidBindGroup := whiteTextureBindGroup
			if wt := worldSkySolidTextures[face.TextureIndex]; wt != nil && wt.bindGroup != nil {
				solidBindGroup = wt.bindGroup
			}
			alphaBindGroup := transparentBindGroup
			if wt := worldSkyAlphaTextures[face.TextureIndex]; wt != nil && wt.bindGroup != nil {
				alphaBindGroup = wt.bindGroup
			}
			renderPass.SetBindGroup(1, solidBindGroup, nil)
			renderPass.SetBindGroup(2, alphaBindGroup, nil)
			// Bind group 3 (fullbright/lightmap) is required by the shared pipeline
			// layout even though the sky shader doesn't use it.
			renderPass.SetBindGroup(3, whiteTextureBindGroup, nil)
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		}
	}
	if logExternalSkyDraw {
		slog.Info("external sky brush draw commands encoded", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
		slog.Info("external sky brush render pass end begin", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderSkyBrushEntitiesHAL: render pass end error", "error", err)
	}
	if logExternalSkyDraw {
		slog.Info("external sky brush render pass end complete", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
		slog.Info("external sky brush encoder finish begin", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush sky encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Release()
		}
		return
	}
	if logExternalSkyDraw {
		slog.Info("external sky brush encoder finish complete", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
		slog.Info("external sky brush queue submit begin", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
	}
	if r.uniformOffset > passStartUniformOffset {
		_ = queue.WriteBuffer(uniformBuffer, uint64(passStartUniformOffset), r.uniformDataScratch[passStartUniformOffset:r.uniformOffset])
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush sky commands", "error", err)
	}
	if logExternalSkyDraw {
		slog.Info("external sky brush queue submit complete", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName)
		r.worldSkyExternalBrushDrawLogged = true
	}
	for _, buffer := range buffers {
		buffer.Release()
	}
}

func (dc *DrawContext) renderOpaqueLiquidBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
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
	r.mu.RLock()
	var geom *WorldGeometry
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		geom = r.worldData.Geometry
	}
	r.mu.RUnlock()
	if geom == nil || geom.Tree == nil {
		return
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)

	scratch := gogpuBrushPrepScratchPool.Get().(*gogpuBrushPrepScratch)
	defer gogpuBrushPrepScratchPool.Put(scratch)
	scratch.opaqueBuild = scratch.opaqueBuild[:0]
	scratch.opaqueDraws = scratch.opaqueDraws[:0]
	scratch.opaquePrepared = scratch.opaquePrepared[:0]
	scratch.vertexData = scratch.vertexData[:0]
	scratch.indexData = scratch.indexData[:0]
	for _, entity := range entities {
		geom := dc.renderer.brushEntityGeometry(entity)
		scratch.opaqueBuild = append(scratch.opaqueBuild, worldgogpu.OpaqueBrushEntityDraw{})
		buildDraw := &scratch.opaqueBuild[len(scratch.opaqueBuild)-1]
		if !worldgogpu.FillBrushEntityDraw(buildDraw, gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) bool {
			return shouldDrawGoGPUOpaqueLiquidBrushFace(face, entityAlpha, liquidAlpha)
		}) {
			scratch.opaqueBuild = scratch.opaqueBuild[:len(scratch.opaqueBuild)-1]
			continue
		}
		extTextures, extFullbright, extAnimations, extBindGroup := dc.renderer.brushEntityTextures(entity)
		scratch.opaqueDraws = append(scratch.opaqueDraws, gogpuOpaqueBrushEntityDraw{
			hasLitWater:            buildDraw.HasLitWater,
			alpha:                  buildDraw.Alpha,
			frame:                  buildDraw.Frame,
			vertices:               buildDraw.Vertices,
			indices:                buildDraw.Indices,
			faces:                  buildDraw.Faces,
			centers:                buildDraw.Centers,
			lightmapArray:          dc.renderer.brushEntityLightmaps(entity, geom),
			textures:               extTextures,
			fullbrightTextures:     extFullbright,
			textureAnimations:      extAnimations,
			uniformBindGroup:       extBindGroup,
			uniformBindGroupFrame1: dc.renderer.brushEntityUniformBindGroupFrame1(entity),
		})
		drawIndex := len(scratch.opaqueDraws) - 1
		draw := &scratch.opaqueDraws[drawIndex]
		vertexOffset := uint64(len(scratch.vertexData))
		scratch.vertexData = appendGoGPUWorldVertexBytes(scratch.vertexData, draw.vertices)
		indexOffset := uint64(len(scratch.indexData))
		scratch.indexData = appendGoGPUWorldIndexBytes(scratch.indexData, draw.indices)
		scratch.opaquePrepared = append(scratch.opaquePrepared, gogpuPreparedOpaqueBrushDraw{
			drawIndex:    drawIndex,
			hasLitWater:  draw.hasLitWater,
			vertexOffset: vertexOffset,
			indexOffset:  indexOffset,
		})
	}
	if len(scratch.opaquePrepared) == 0 {
		return
	}
	totalVertexBytes := uint64(len(scratch.vertexData))
	totalIndexBytes := uint64(len(scratch.indexData))

	r.mu.Lock()
	if err := r.ensureBrushEntityScratchBuffersLocked(device, totalVertexBytes, totalIndexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure brush liquid scratch buffers", "error", err)
		return
	}
	pipeline := r.worldTurbulentPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	transparentBindGroup := r.transparentBindGroup
	vertexScratchBuffer := r.brushEntityScratchVertexBuffer
	indexScratchBuffer := r.brushEntityScratchIndexBuffer
	depthView := r.worldDepthTextureView
	dynamicLightsBuffer := r.worldDynamicLightsBuffer
	dynamicLightsBindGroup := r.worldDynamicLightsBindGroup
	camera := r.cameraState
	worldTextures := r.worldTextures
	worldFullbrightTextures := r.worldFullbrightTextures

	r.brushTextureAnimationsScratch = append(r.brushTextureAnimationsScratch[:0], r.worldTextureAnimations...)

	r.activeDynamicLightsScratch = r.activeDynamicLightsScratch[:0]
	if r.lightPool != nil {
		r.activeDynamicLightsScratch = append(r.activeDynamicLightsScratch, r.lightPool.ActiveLights()...)
	}
	activeDynamicLights := r.activeDynamicLightsScratch
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || dynamicLightsBuffer == nil || dynamicLightsBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}
	// Create mapped scratch buffers per-pass (bypassing queue.WriteBuffer + stagingBelt)
	if len(scratch.vertexData) > 0 {
		vSize := uint64(len(scratch.vertexData))
		buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Brush Liquid Vertex Scratch Buffer",
			Size:             vSize,
			Usage:            gputypes.BufferUsageVertex,
			MappedAtCreation: true,
		})
		if err != nil {
			slog.Warn("failed to create brush liquid scratch vertex buffer", "error", err)
			return
		}
		if mr, mrErr := buf.MappedRange(0, vSize); mrErr == nil && mr != nil {
			copy(mr.Bytes(), scratch.vertexData)
		}
		buf.Unmap()
		vertexScratchBuffer = buf
		defer vertexScratchBuffer.Release()
	}
	if len(scratch.indexData) > 0 {
		iSize := uint64(len(scratch.indexData))
		buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Brush Liquid Index Scratch Buffer",
			Size:             iSize,
			Usage:            gputypes.BufferUsageIndex,
			MappedAtCreation: true,
		})
		if err != nil {
			slog.Warn("failed to create brush liquid scratch index buffer", "error", err)
			return
		}
		if mr, mrErr := buf.MappedRange(0, iSize); mrErr == nil && mr != nil {
			copy(mr.Bytes(), scratch.indexData)
		}
		buf.Unmap()
		indexScratchBuffer = buf
		defer indexScratchBuffer.Release()
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Brush Liquid Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush liquid encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Brush Liquid Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderOpaqueLiquidBrushEntitiesHAL: Failed to begin render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	passStartUniformOffset := r.uniformOffset
	ptr2, lightData2 := encodeGoGPUWorldDynamicLights(activeDynamicLights)
	err2 := queue.WriteBuffer(dynamicLightsBuffer, 0, lightData2)
	dynamicLightsBytesPool.Put(ptr2)
	if err2 != nil {
		slog.Warn("failed to upload brush dynamic lights", "error", err2)
		return
	}
	renderPass.SetBindGroup(4, dynamicLightsBindGroup, nil)

	vpMatrix := r.ViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	var materialBindState gogpuWorldMaterialBindState
	for _, preparedDraw := range scratch.opaquePrepared {
		draw := scratch.opaqueDraws[preparedDraw.drawIndex]
		renderPass.SetVertexBuffer(0, vertexScratchBuffer, preparedDraw.vertexOffset)
		renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.indexOffset)
		drawTextures := worldTextures
		drawFullbright := worldFullbrightTextures
		if draw.textures != nil {
			drawTextures = draw.textures
		}
		if draw.fullbrightTextures != nil {
			drawFullbright = draw.fullbrightTextures
		}

		// Select the frame-1 uniform bind group when the entity's frame != 0.
		frameUniformBindGroup := uniformBindGroup
		if draw.frame != 0 && draw.uniformBindGroupFrame1 != nil {
			frameUniformBindGroup = draw.uniformBindGroupFrame1
		} else if draw.uniformBindGroup != nil {
			frameUniformBindGroup = draw.uniformBindGroup
		}

		for _, face := range draw.faces {
			textureBindGroup := whiteTextureBindGroup
			if drawTextures != nil && drawTextures.bindGroup != nil {
				textureBindGroup = drawTextures.bindGroup
			}
			lightmapBindGroup, litWater := gogpuWorldLightmapArrayBindGroupForFace(face, draw.lightmapArray, whiteLightmapBindGroup, preparedDraw.hasLitWater)
			offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
			fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), camera.Time, draw.alpha, litWater)

			renderPass.SetBindGroup(0, frameUniformBindGroup, []uint32{offset})
			fullbrightBindGroup := transparentBindGroup
			if drawFullbright != nil && drawFullbright.bindGroup != nil {
				fullbrightBindGroup = drawFullbright.bindGroup
			}
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
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderOpaqueLiquidBrushEntitiesHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush liquid encoding", "error", err)
		return
	}
	if r.uniformOffset > passStartUniformOffset {
		_ = queue.WriteBuffer(uniformBuffer, uint64(passStartUniformOffset), r.uniformDataScratch[passStartUniformOffset:r.uniformOffset])
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush liquid commands", "error", err)
	}
}

// ---- merged from world_alias_gogpu_root.go ----
