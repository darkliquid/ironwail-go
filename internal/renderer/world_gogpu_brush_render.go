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
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		scratch.classifiedBuild = append(scratch.classifiedBuild, worldgogpu.ClassifiedBrushEntityDraw{})
		buildDraw := &scratch.classifiedBuild[len(scratch.classifiedBuild)-1]
		if !worldgogpu.FillClassifiedBrushEntityDraw(buildDraw, gogpuBrushEntityParams(entity), geom, classifyGoGPUBrushEntityFace) {
			scratch.classifiedBuild = scratch.classifiedBuild[:len(scratch.classifiedBuild)-1]
			continue
		}
		scratch.classifiedDraws = append(scratch.classifiedDraws, gogpuClassifiedBrushEntityDraw{
			alpha:            buildDraw.Alpha,
			frame:            buildDraw.Frame,
			vertices:         buildDraw.Vertices,
			opaqueIndices:    buildDraw.OpaqueIndices,
			opaqueFaces:      buildDraw.OpaqueFaces,
			opaqueCenters:    buildDraw.OpaqueCenters,
			alphaTestIndices: buildDraw.AlphaTestIndices,
			alphaTestFaces:   buildDraw.AlphaTestFaces,
			alphaTestCenters: buildDraw.AlphaTestCenters,
			lightmaps:        dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom),
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
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	transparentBindGroup := r.transparentBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	vertexScratchBuffer := r.brushEntityScratchVertexBuffer
	indexScratchBuffer := r.brushEntityScratchIndexBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	worldTextures := make(map[int32]*gpuWorldTexture, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]*gpuWorldTexture, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	worldLightmapPages := append([]*gpuWorldTexture(nil), r.worldLightmapPages...)
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil || vertexScratchBuffer == nil || indexScratchBuffer == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}

	if len(scratch.vertexData) > 0 {
		if err := queue.WriteBuffer(vertexScratchBuffer, 0, scratch.vertexData); err != nil {
			slog.Warn("failed to upload brush scratch vertices", "error", err)
			return
		}
	}
	if len(scratch.indexData) > 0 {
		if err := queue.WriteBuffer(indexScratchBuffer, 0, scratch.indexData); err != nil {
			slog.Warn("failed to upload brush scratch indices", "error", err)
			return
		}
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
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	timeSeconds := float64(camera.Time)
	var uniformData [worldUniformBufferSize]byte
	var materialBindState gogpuWorldMaterialBindState
	for _, preparedDraw := range scratch.classifiedPrepared {
		draw := scratch.classifiedDraws[preparedDraw.drawIndex]
		renderPass.SetVertexBuffer(0, vertexScratchBuffer, preparedDraw.vertexOffset)
		if len(draw.opaqueFaces) > 0 {
			renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.opaqueIndexOffset)
			for faceIndex, face := range draw.opaqueFaces {
				dynamicLight := [3]float32{}
				if faceIndex < len(draw.opaqueCenters) {
					dynamicLight = quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.opaqueCenters[faceIndex]))
				}
				fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.alpha, dynamicLight, 0)
				if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
					slog.Warn("failed to update brush uniform buffer", "error", err)
					continue
				}
				textureBindGroup := whiteTextureBindGroup
				if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					textureBindGroup = worldTexture.bindGroup
				}
				lightmapBindGroup := whiteLightmapBindGroup
				if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(draw.lightmaps) {
					if lightmapPage := draw.lightmaps[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
						lightmapBindGroup = lightmapPage.bindGroup
					}
				} else if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(worldLightmapPages) {
					if lightmapPage := worldLightmapPages[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
						lightmapBindGroup = lightmapPage.bindGroup
					}
				}
				fullbrightBindGroup := transparentBindGroup
				if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					fullbrightBindGroup = worldTexture.bindGroup
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
			renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.alphaTestIndexOffset)
			for faceIndex, face := range draw.alphaTestFaces {
				dynamicLight := [3]float32{}
				if faceIndex < len(draw.alphaTestCenters) {
					dynamicLight = quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.alphaTestCenters[faceIndex]))
				}
				fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.alpha, dynamicLight, 0)
				if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
					slog.Warn("failed to update brush uniform buffer", "error", err)
					continue
				}
				textureBindGroup := whiteTextureBindGroup
				if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					textureBindGroup = worldTexture.bindGroup
				}
				lightmapBindGroup := whiteLightmapBindGroup
				if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(draw.lightmaps) {
					if lightmapPage := draw.lightmaps[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
						lightmapBindGroup = lightmapPage.bindGroup
					}
				} else if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(worldLightmapPages) {
					if lightmapPage := worldLightmapPages[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
						lightmapBindGroup = lightmapPage.bindGroup
					}
				}
				fullbrightBindGroup := transparentBindGroup
				if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					fullbrightBindGroup = worldTexture.bindGroup
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
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
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
	externalSkyPipeline := r.worldSkyExternalPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	transparentBindGroup := r.transparentBindGroup
	worldSkySolidTextures := make(map[int32]*gpuWorldTexture, len(r.worldSkySolidTextures))
	for k, v := range r.worldSkySolidTextures {
		worldSkySolidTextures[k] = v
	}
	worldSkyAlphaTextures := make(map[int32]*gpuWorldTexture, len(r.worldSkyAlphaTextures))
	for k, v := range r.worldSkyAlphaTextures {
		worldSkyAlphaTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	externalSkyMode := r.worldSkyExternalMode
	externalSkyBindGroup := r.worldSkyExternalBindGroup
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if uniformBuffer == nil || uniformBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}
	useExternalSky := externalSkyMode == externalSkyboxRenderFaces && externalSkyPipeline != nil && externalSkyBindGroup != nil
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
	if useExternalSky {
		renderPass.SetPipeline(externalSkyPipeline)
		renderPass.SetBindGroup(1, externalSkyBindGroup, nil)
	} else {
		renderPass.SetPipeline(skyPipeline)
	}
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
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
		var uniformData [worldUniformBufferSize]byte
		fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, skyFogDensity, camera.Time, 1, [3]float32{}, 0)
		if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
			slog.Warn("failed to update brush sky uniform buffer", "error", err)
			continue
		}
		renderPass.SetVertexBuffer(0, vertexBuffer, 0)
		renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
		for _, face := range draw.faces {
			if useExternalSky {
				renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
				continue
			}
			textureIndex := resolveWorldSkyTextureIndex(face, worldTextureAnimations, draw.frame, float64(camera.Time))
			solidBindGroup := whiteTextureBindGroup
			if worldTexture := worldSkySolidTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				solidBindGroup = worldTexture.bindGroup
			}
			alphaBindGroup := transparentBindGroup
			if worldTexture := worldSkyAlphaTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				alphaBindGroup = worldTexture.bindGroup
			}
			renderPass.SetBindGroup(1, solidBindGroup, nil)
			renderPass.SetBindGroup(2, alphaBindGroup, nil)
			// Bind group 3 (fullbright/lightmap) is required by the shared pipeline
			// layout even though the sky shader doesn't use it.
			renderPass.SetBindGroup(3, whiteTextureBindGroup, nil)
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		}
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderSkyBrushEntitiesHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush sky encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Release()
		}
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush sky commands", "error", err)
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
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		scratch.opaqueBuild = append(scratch.opaqueBuild, worldgogpu.OpaqueBrushEntityDraw{})
		buildDraw := &scratch.opaqueBuild[len(scratch.opaqueBuild)-1]
		if !worldgogpu.FillBrushEntityDraw(buildDraw, gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) bool {
			return shouldDrawGoGPUOpaqueLiquidBrushFace(face, entityAlpha, liquidAlpha)
		}) {
			scratch.opaqueBuild = scratch.opaqueBuild[:len(scratch.opaqueBuild)-1]
			continue
		}
		scratch.opaqueDraws = append(scratch.opaqueDraws, gogpuOpaqueBrushEntityDraw{
			hasLitWater: buildDraw.HasLitWater,
			alpha:       buildDraw.Alpha,
			frame:       buildDraw.Frame,
			vertices:    buildDraw.Vertices,
			indices:     buildDraw.Indices,
			faces:       buildDraw.Faces,
			centers:     buildDraw.Centers,
			lightmaps:   dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom),
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
	camera := r.cameraState
	worldTextures := make(map[int32]*gpuWorldTexture, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]*gpuWorldTexture, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil || vertexScratchBuffer == nil || indexScratchBuffer == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}
	if len(scratch.vertexData) > 0 {
		if err := queue.WriteBuffer(vertexScratchBuffer, 0, scratch.vertexData); err != nil {
			slog.Warn("failed to upload brush liquid scratch vertices", "error", err)
			return
		}
	}
	if len(scratch.indexData) > 0 {
		if err := queue.WriteBuffer(indexScratchBuffer, 0, scratch.indexData); err != nil {
			slog.Warn("failed to upload brush liquid scratch indices", "error", err)
			return
		}
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
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	timeSeconds := float64(camera.Time)
	var uniformData [worldUniformBufferSize]byte
	var materialBindState gogpuWorldMaterialBindState
	for _, preparedDraw := range scratch.opaquePrepared {
		draw := scratch.opaqueDraws[preparedDraw.drawIndex]
		renderPass.SetVertexBuffer(0, vertexScratchBuffer, preparedDraw.vertexOffset)
		renderPass.SetIndexBuffer(indexScratchBuffer, gputypes.IndexFormatUint32, preparedDraw.indexOffset)
		for faceIndex, face := range draw.faces {
			textureBindGroup := whiteTextureBindGroup
			if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
				textureBindGroup = worldTexture.bindGroup
			}
			dynamicLight := [3]float32{}
			if faceIndex < len(draw.centers) {
				dynamicLight = quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.centers[faceIndex]))
			}
			lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(face, draw.lightmaps, whiteLightmapBindGroup, preparedDraw.hasLitWater)
			fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, 1, dynamicLight, litWater)
			if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
				slog.Warn("failed to update brush liquid uniform buffer", "error", err)
				continue
			}
			fullbrightBindGroup := transparentBindGroup
			if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
				fullbrightBindGroup = worldTexture.bindGroup
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
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush liquid commands", "error", err)
	}
}

// ---- merged from world_alias_gogpu_root.go ----
