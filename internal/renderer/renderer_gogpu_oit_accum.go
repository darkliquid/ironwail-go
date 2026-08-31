// renderer_gogpu_oit_accum.go executes the unified Pass 2 (Translucent Accumulation)
// for McGuire's weighted-blended order-independent transparency (OIT) in GoGPU/WebGPU.

package renderer

import (
	"log/slog"

	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// renderOITTranslucentPassHAL executes the unified Pass 2 (Translucent Accumulation)
// into the OIT multiple render targets (accum RGBA16Float + reveal R8Unorm).
func (dc *DrawContext) renderOITTranslucentPassHAL(state *RenderFrameState, plan gogpuEntityDrawPlan, pendingTranslucentRenders []gogpuTranslucentBrushFaceRender) bool {
	if dc == nil || dc.renderer == nil || state == nil {
		return false
	}
	r := dc.renderer
	hasWorldLiquid := state.DrawWorld && r.hasTranslucentWorldLiquidFacesGoGPU() && IsGlobalPassEnabled(PassTranslucentLiquids)
	hasBrushTranslucent := len(pendingTranslucentRenders) > 0 && IsGlobalPassEnabled(PassBrushEntities)
	hasDecals := len(state.DecalMarks) > 0 && IsGlobalPassEnabled(PassDecals)
	hasAliasTranslucent := len(plan.translucentAlias) > 0 && IsGlobalPassEnabled(PassAliasEntities)
	hasSprites := len(state.SpriteEntities) > 0 && IsGlobalPassEnabled(PassAliasEntities)
	hasParticles := state.DrawParticles && state.Particles != nil && state.Particles.ActiveCount() > 0 && IsGlobalPassEnabled(PassParticles)

	slog.Info("[oit_debug] renderOITTranslucentPassHAL",
		"hasWorldLiquid", hasWorldLiquid,
		"hasTranslucentWorldLiquidFaces", r.hasTranslucentWorldLiquidFacesGoGPU(),
		"hasBrushTranslucent", hasBrushTranslucent,
		"hasDecals", hasDecals,
		"hasAliasTranslucent", hasAliasTranslucent,
		"hasSprites", hasSprites,
		"hasParticles", hasParticles,
	)

	if !hasWorldLiquid && !hasBrushTranslucent && !hasDecals && !hasAliasTranslucent && !hasSprites && !hasParticles {
		return false
	}

	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil {
		return false
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return false
	}

	r.mu.Lock()
	if err := r.ensureOITResourcesLocked(device, width, height); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure OIT resources", "error", err)
		return false
	}
	accumView := r.resources.OITAccumTextureView
	revealView := r.resources.OITRevealTextureView
	depthView := r.resources.WorldDepthTextureView
	dynamicLightsBuffer := r.resources.WorldDynamicLightsBuffer
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	r.mu.Unlock()

	if accumView == nil || revealView == nil || depthView == nil {
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "OIT Translucent Accumulation Encoder")
	if err != nil {
		slog.Warn("failed to create OIT accumulation encoder", "error", err)
		return false
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "OIT Translucent Accumulation Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       accumView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 0},
			},
			{
				View:       revealView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 1, G: 1, B: 1, A: 1},
			},
		},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("renderOITTranslucentPassHAL: Failed to begin render pass", "error", err)
		return false
	}

	renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
	renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))

	passStartUniformOffset := r.uniformOffset
	if dynamicLightsBuffer != nil && len(activeDynamicLights) > 0 {
		ptr, lightData := encodeGoGPUWorldDynamicLights(activeDynamicLights)
		err = queue.WriteBuffer(dynamicLightsBuffer, 0, lightData)
		dynamicLightsBytesPool.Put(ptr)
		if err != nil {
			slog.Warn("failed to upload OIT dynamic lights", "error", err)
		}
	}

	if hasWorldLiquid && r.resources.OITWorldUniformBuffer != nil {
		var uniformBytes [worldUniformBufferSize]byte
		vpMatrix := r.ViewProjectionMatrix()
		cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: state.FogDensity}, r.cameraState)
		faceAlpha := r.deferredTranslucentLiquidAlpha.water
		litWater := float32(0)
		if r.deferredTranslucentLiquidLitWater {
			litWater = 1
		}
		fillWorldSceneUniformBytes(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, worldFogUniformDensity(state.FogDensity), timeValue, faceAlpha, litWater)
		if err := queue.WriteBuffer(r.resources.OITWorldUniformBuffer, 0, uniformBytes[:]); err != nil {
			slog.Warn("failed to write OIT liquid uniforms", "error", err)
		}
	}

	if hasWorldLiquid {
		dc.recordOITWorldTranslucentLiquid(renderPass, queue, state.FogColor, state.FogDensity)
	}
	if hasBrushTranslucent {
		dc.recordOITTranslucentBrushFaces(renderPass, queue, pendingTranslucentRenders, state.FogColor, state.FogDensity)
	}
	if hasDecals {
		dc.recordOITDecalMarks(renderPass, queue, state.DecalMarks)
	}
	if hasAliasTranslucent {
		dc.recordOITTranslucentAliasModels(renderPass, queue, device, plan.translucentAlias, state.FogColor, state.FogDensity)
	}
	if hasSprites {
		dc.recordOITSprites(renderPass, queue, state.SpriteEntities, state.FogColor, state.FogDensity)
	}
	if hasParticles {
		dc.recordOITParticles(renderPass, queue, device, state)
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderOITTranslucentPassHAL: render pass end error", "error", err)
	}

	if r.uniformOffset > passStartUniformOffset {
		err := queue.WriteBuffer(r.resources.UniformBuffer, uint64(passStartUniformOffset), r.uniformDataScratch[passStartUniformOffset:r.uniformOffset])
		if err != nil {
			slog.Warn("failed to write OIT uniforms", "error", err)
		}
	}

	dc.frameSubmit(queue, encoder, encoderOwned, "OIT Translucent Accumulation Encoder")
	return true
}

func (dc *DrawContext) recordOITWorldTranslucentLiquid(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, fogColor types.Vec3, fogDensity float32) {
	r := dc.renderer
	faces := r.deferredTranslucentLiquidFaces
	if !r.deferredTranslucentLiquidValid || len(faces) == 0 || r.worldIndexBuffer == nil || r.worldVertexBuffer == nil {
		return
	}
	pipelineObj := r.resources.OITWorldTranslucentTurbulentPipeline
	if pipelineObj == nil {
		return
	}

	renderPass.SetPipeline(pipelineObj)
	renderPass.SetVertexBuffer(0, r.worldVertexBuffer, 0)
	renderPass.SetIndexBuffer(r.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	uniformBG := r.resources.OITWorldUniformBindGroup
	if uniformBG == nil {
		uniformBG = r.resources.UniformBindGroup
	}
	renderPass.SetBindGroup(0, uniformBG, nil)

	worldHasLitWater := r.deferredTranslucentLiquidLitWater

	var materialBindState gogpuWorldMaterialBindState
	materialBindState.invalidate()
	for _, face := range faces {
		textureBindGroup := r.resources.WhiteTextureBindGroup
		if r.worldTextures != nil && r.worldTextures.bindGroup != nil {
			textureBindGroup = r.worldTextures.bindGroup
		}
		lightmapBindGroup, _ := gogpuWorldLightmapArrayBindGroupForFace(face, r.worldLightmapArray, r.resources.WhiteLightmapBindGroup, worldHasLitWater)
		fullbrightBindGroup := r.resources.TransparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = r.resources.WhiteTextureBindGroup
		}
		if r.worldFullbrightTextures != nil && r.worldFullbrightTextures.bindGroup != nil {
			fullbrightBindGroup = r.worldFullbrightTextures.bindGroup
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

func (dc *DrawContext) recordOITTranslucentBrushFaces(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, renders []gogpuTranslucentBrushFaceRender, fogColor types.Vec3, fogDensity float32) {
	if len(renders) == 0 {
		return
	}
	r := dc.renderer
	vpMatrix := r.ViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, r.cameraState)
	timeSeconds := float64(timeValue)

	oitTurbPipeline := r.resources.OITWorldTranslucentTurbulentPipeline
	oitTransPipeline := r.resources.OITWorldTranslucentPipeline
	if oitTurbPipeline == nil || oitTransPipeline == nil {
		return
	}

	res := gogpuLateTranslucentFaceResources{
		translucentPipeline:     oitTransPipeline,
		liquidPipeline:          oitTurbPipeline,
		uniformBindGroup:        r.resources.UniformBindGroup,
		whiteTextureBindGroup:   r.resources.WhiteTextureBindGroup,
		whiteLightmapBindGroup:  r.resources.WhiteLightmapBindGroup,
		transparentBindGroup:    r.resources.TransparentBindGroup,
		worldTextures:           r.worldTextures,
		worldFullbrightTextures: r.worldFullbrightTextures,
		worldLightmapArray:      r.worldLightmapArray,
	}
	if res.transparentBindGroup == nil {
		res.transparentBindGroup = res.whiteTextureBindGroup
	}

	var currentPipeline *wgpu.RenderPipeline
	var materialBindState gogpuWorldMaterialBindState
	for _, draw := range renders {
		pipeline := oitTransPipeline
		if draw.liquid {
			pipeline = oitTurbPipeline
		}
		if pipeline != currentPipeline {
			renderPass.SetPipeline(pipeline)
			currentPipeline = pipeline
			materialBindState = gogpuWorldMaterialBindState{}
		}

		lightmapBindGroup, litWater := gogpuLateTranslucentLightmapBindGroup(res, draw)
		offset, uData := r.allocateUniformBuffer(worldUniformBufferSize)
		if uData == nil {
			continue
		}
		fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), r.cameraState.Time, draw.face.alpha, litWater)

		activeUniformBindGroup := res.uniformBindGroup
		if draw.frame != 0 && draw.uniformBindGroupFrame1 != nil {
			activeUniformBindGroup = draw.uniformBindGroupFrame1
		} else if draw.uniformBindGroup != nil {
			activeUniformBindGroup = draw.uniformBindGroup
		}

		renderPass.SetBindGroup(0, activeUniformBindGroup, []uint32{offset})
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], draw.vertexOffset)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, draw.indexOffset)

		textureBindGroup, fullbrightBindGroup := gogpuLateTranslucentTextureBindGroups(res, draw, timeSeconds)
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
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
}

func (dc *DrawContext) recordOITTranslucentAliasModels(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, device *wgpu.Device, entities []AliasModelEntity, fogColor types.Vec3, fogDensity float32) {
	if len(entities) == 0 {
		return
	}
	draws := dc.collectAliasDraws(entities, false)
	if len(draws) == 0 {
		return
	}

	r := dc.renderer
	vpMatrix := r.ViewProjectionMatrix()
	camera := r.cameraState
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	dc.aliasPreparedScratch = dc.aliasPreparedScratch[:0]
	dc.aliasVertexScratch = dc.aliasVertexScratch[:0]
	dc.aliasBulkVertexData = dc.aliasBulkVertexData[:0]
	dc.aliasBulkUniformData = dc.aliasBulkUniformData[:0]
	dc.aliasVertexOffsets = dc.aliasVertexOffsets[:0]
	dc.aliasVertexCounts = dc.aliasVertexCounts[:0]
	dc.aliasUniformOffsets = dc.aliasUniformOffsets[:0]

	currentVertexOffset := uint64(0)
	for _, draw := range draws {
		if draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}

		dc.aliasVertexScratch = buildAliasVerticesInterpolatedInto(
			dc.aliasVertexScratch[:0],
			draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend,
			draw.origin, draw.angles, draw.scale, draw.full,
		)
		if len(dc.aliasVertexScratch) == 0 {
			continue
		}

		vertexCount := uint32(len(dc.aliasVertexScratch))
		uOffset := uint32(len(dc.aliasPreparedScratch)) * worldUniformAlign

		dc.aliasPreparedScratch = append(dc.aliasPreparedScratch, gpuPreparedAliasDraw{
			draw:        draw,
			skin:        draw.skin,
			alpha:       draw.alpha,
			vertexCount: vertexCount,
		})
		dc.aliasUniformOffsets = append(dc.aliasUniformOffsets, uOffset)
		dc.aliasVertexOffsets = append(dc.aliasVertexOffsets, currentVertexOffset)
		dc.aliasVertexCounts = append(dc.aliasVertexCounts, vertexCount)

		dc.aliasBulkUniformData = appendAliasSceneUniformBytes(dc.aliasBulkUniformData, uOffset, vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)
		dc.aliasBulkVertexData = appendAliasVertexBytes(dc.aliasBulkVertexData, dc.aliasVertexScratch)
		currentVertexOffset += uint64(len(dc.aliasVertexScratch) * aliasVertexStride)
	}

	if len(dc.aliasPreparedScratch) == 0 {
		return
	}

	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, currentVertexOffset); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer", "error", err)
		return
	}
	if err := r.ensureAliasUniformBufferLocked(device, len(dc.aliasPreparedScratch)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer", "error", err)
		return
	}
	pipelineObj := r.oitAliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	r.mu.Unlock()

	if pipelineObj == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	if len(dc.aliasBulkUniformData) > 0 {
		if err := queue.WriteBuffer(uniformBuffer, 0, dc.aliasBulkUniformData); err != nil {
			slog.Warn("failed to upload alias uniform buffer in bulk", "error", err)
			return
		}
	}
	if len(dc.aliasBulkVertexData) > 0 {
		if err := queue.WriteBuffer(scratchBuffer, 0, dc.aliasBulkVertexData); err != nil {
			slog.Warn("failed to upload alias vertices in bulk", "error", err)
			return
		}
	}

	renderPass.SetPipeline(pipelineObj)
	for i, pd := range dc.aliasPreparedScratch {
		renderPass.SetVertexBuffer(0, scratchBuffer, dc.aliasVertexOffsets[i])
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{dc.aliasUniformOffsets[i]})
		renderPass.SetBindGroup(1, pd.skin.bindGroup, nil)
		renderPass.Draw(dc.aliasVertexCounts[i], 1, 0, 0)
	}
}

func (dc *DrawContext) recordOITDecalMarks(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, marks []DecalMarkEntity) {
	if len(marks) == 0 {
		return
	}
	r := dc.renderer
	camera := r.cameraState
	draws := prepareDecalDraws(marks, camera)
	preparedDraws := prepareGoGPUDecalHALDraws(draws)

	totalVertexBytes := uint64(0)
	for _, prepared := range preparedDraws {
		totalVertexBytes += uint64(len(prepared.VertexBytes))
	}
	if totalVertexBytes == 0 {
		return
	}

	device := r.getWGPUDevice()
	r.mu.Lock()
	if err := r.ensureDecalScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure decal scratch buffer", "error", err)
		return
	}
	pipelineObj := r.oitDecalPipeline
	uniformBuffer := r.decalUniformBuffer
	uniformBindGroup := r.decalUniformBindGroup
	bindGroup := r.decalBindGroup
	scratchBuffer := r.decalScratchBuffer
	r.mu.Unlock()

	if pipelineObj == nil || uniformBuffer == nil || uniformBindGroup == nil || bindGroup == nil || scratchBuffer == nil {
		return
	}

	vpMatrix := r.ViewProjectionMatrix()
	if err := queue.WriteBuffer(uniformBuffer, 0, worldgogpu.DecalUniformBytes(vpMatrix, 1)); err != nil {
		slog.Warn("failed to upload decal uniform buffer", "error", err)
		return
	}

	bulkVertexData := make([]byte, 0, totalVertexBytes)
	totalVertices := uint32(0)
	for _, prepared := range preparedDraws {
		if prepared.VertexCount == 0 {
			continue
		}
		bulkVertexData = append(bulkVertexData, prepared.VertexBytes...)
		totalVertices += prepared.VertexCount
	}

	if err := queue.WriteBuffer(scratchBuffer, 0, bulkVertexData); err != nil {
		slog.Warn("failed to upload decal vertices", "error", err)
		return
	}

	renderPass.SetPipeline(pipelineObj)
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)
	renderPass.SetBindGroup(1, bindGroup, nil)
	renderPass.Draw(totalVertices, 1, 0, 0)
}

func (dc *DrawContext) recordOITSprites(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, entities []SpriteEntity, fogColor types.Vec3, fogDensity float32) {
	if len(entities) == 0 {
		return
	}
	draws := dc.collectSpriteDraws(entities)
	if len(draws) == 0 {
		return
	}

	totalVertexBytes := uint64(len(draws) * 6 * 48)
	r := dc.renderer
	device := r.getWGPUDevice()
	r.mu.Lock()
	if err := r.ensureSpriteScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure sprite scratch buffer", "error", err)
		return
	}
	pipelineObj := r.oitSpritePipeline
	uniformBuffer := r.spriteUniformBuffer
	uniformBindGroup := r.spriteUniformBindGroup
	scratchBuffer := r.spriteScratchBuffer
	r.mu.Unlock()

	if pipelineObj == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	renderPass.SetPipeline(pipelineObj)
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)

	vpMatrix := r.ViewProjectionMatrix()
	camera := r.cameraState
	cameraOrigin := camera.Origin
	cameraAngles := camera.Angles
	cameraForward, cameraRight, cameraUp := spriteCameraBasis(cameraAngles)

	bulkUniformData := make([]byte, uint64(len(draws))*worldUniformAlign)
	bulkVertexData := make([]byte, 0, len(draws)*6*48)

	uniformOffsets := make([]uint32, len(draws))
	vertexCounts := make([]uint32, len(draws))
	vertexOffsets := make([]uint32, len(draws))

	currentVertexOffset := uint32(0)
	for i, draw := range draws {
		if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
			continue
		}
		frame := draw.sprite.frames[draw.frame]
		if frame.bindGroup == nil {
			continue
		}
		vertices := buildSpriteQuadVertices(&spriteRenderModel{
			modelID:    draw.sprite.modelID,
			spriteType: draw.sprite.spriteType,
			frames:     []spriteRenderFrame{draw.sprite.frames[draw.frame].meta},
			maxWidth:   draw.sprite.maxWidth,
			maxHeight:  draw.sprite.maxHeight,
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

		uOffset := uint32(i) * worldUniformAlign
		uniformOffsets[i] = uOffset

		uBytes := worldgogpu.SpriteUniformBytes(vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)
		copy(bulkUniformData[uOffset:], uBytes)

		vertexBytes := aliasVertexBytes(worldVertices)
		bulkVertexData = append(bulkVertexData, vertexBytes...)

		vertexCounts[i] = uint32(len(worldVertices))
		vertexOffsets[i] = currentVertexOffset
		currentVertexOffset += uint32(len(worldVertices))
	}

	if len(draws) > 0 {
		bufSize := int(uint64(aliasInitialDrawCapacity) * worldUniformAlign)
		writeSize := min(len(bulkUniformData), bufSize)
		if err := queue.WriteBuffer(uniformBuffer, 0, bulkUniformData[:writeSize]); err != nil {
			slog.Warn("failed to upload sprite uniform buffer in bulk", "error", err)
			return
		}
	}
	if len(bulkVertexData) > 0 {
		if err := queue.WriteBuffer(scratchBuffer, 0, bulkVertexData); err != nil {
			slog.Warn("failed to upload sprite vertices in bulk", "error", err)
			return
		}
	}

	for i, draw := range draws {
		if vertexCounts[i] == 0 {
			continue
		}
		frame := draw.sprite.frames[draw.frame]
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{uniformOffsets[i]})
		renderPass.SetBindGroup(1, frame.bindGroup, nil)
		renderPass.Draw(vertexCounts[i], 1, vertexOffsets[i], 0)
	}
}

func (dc *DrawContext) recordOITParticles(renderPass *wgpu.RenderPassEncoder, queue *wgpu.Queue, device *wgpu.Device, state *RenderFrameState) {
	if state.Particles == nil || state.Particles.ActiveCount() == 0 {
		return
	}
	r := dc.renderer
	mode := readGoGPUParticleModeCvar()
	particles := state.Particles.ActiveParticles()
	if len(particles) == 0 {
		return
	}
	palette := buildParticlePalette(state.Palette)
	vertices := BuildParticleVertices(particles, palette, false)
	drawVertices := particleVerticesForGoGPUPass(vertices, mode, true)
	if len(drawVertices) == 0 {
		return
	}

	r.mu.Lock()
	pipelineObj := r.oitParticlePipeline
	uniformBuffer := r.particleUniformBuffer
	uniformBindGroup := r.particleUniformBindGroup
	scratchBuffer := r.particleScratchBuffer
	camera := r.cameraState
	r.mu.Unlock()

	if pipelineObj == nil || uniformBuffer == nil || uniformBindGroup == nil {
		return
	}

	vpMatrix := r.ViewProjectionMatrix()
	projectionMatrix := r.ProjectionMatrix()
	uvScale, textureScaleFactor := ParticleTexture(mode)
	scaleX, scaleY := ParticleProjection(textureScaleFactor, projectionMatrix)
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	uData := particleUniformBytes(vpMatrix, [2]float32{scaleX, scaleY}, uvScale, cameraOrigin, state.FogColor, state.FogDensity)
	if err := queue.WriteBuffer(uniformBuffer, 0, uData); err != nil {
		slog.Warn("failed to upload particle uniforms", "error", err)
		return
	}

	totalVertices := len(drawVertices)
	if totalVertices == 0 {
		return
	}
	needBytes := uint64(totalVertices) * particleVertexStride
	r.mu.Lock()
	if r.particleScratchBufferSize < needBytes {
		if r.particleScratchBuffer != nil {
			old := r.particleScratchBuffer
			r.enqueueReleaseLocked(func() { old.Release() })
			r.particleScratchBuffer = nil
		}
		buf, bErr := device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Particle Scratch Buffer",
			Size:             needBytes,
			Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
			MappedAtCreation: false,
		})
		if bErr != nil {
			r.mu.Unlock()
			slog.Warn("failed to grow particle scratch buffer", "error", bErr)
			return
		}
		r.particleScratchBuffer = buf
		r.particleScratchBufferSize = needBytes
		scratchBuffer = buf
	}
	r.mu.Unlock()

	batchByteOffsets := make([]uint64, 0, (totalVertices+particleBatchCapacity-1)/particleBatchCapacity)
	writeOffset := uint64(0)
	for remaining := drawVertices; len(remaining) > 0; {
		batch := remaining
		if len(batch) > particleBatchCapacity {
			batch = remaining[:particleBatchCapacity]
		}
		batchBytes := particleVertexBytes(batch)
		if err := queue.WriteBuffer(scratchBuffer, writeOffset, batchBytes); err != nil {
			slog.Warn("failed to upload particle vertices", "error", err)
			break
		}
		batchByteOffsets = append(batchByteOffsets, writeOffset)
		writeOffset += uint64(len(batchBytes))
		remaining = remaining[len(batch):]
	}

	renderPass.SetPipeline(pipelineObj)
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	for i := range batchByteOffsets {
		firstVertex := uint32(batchByteOffsets[i] / particleVertexStride)
		batchCount := particleBatchCapacity
		if i == len(batchByteOffsets)-1 {
			batchCount = totalVertices - i*particleBatchCapacity
		}
		renderPass.Draw(4, uint32(batchCount), firstVertex, 0)
	}
}
