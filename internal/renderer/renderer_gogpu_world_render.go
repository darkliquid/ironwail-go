// world_render_gogpu.go implements the world geometry render pass.
//
// Pipeline: prepare (depth, RT, encoder) → sky → opaque batched →
// alpha-test → opaque liquid. Translucent liquid deferred to late pass.
//
// C lineage: R_RenderView, R_DrawWorld in gl_rmain.c, gl_rsurf.c.
// C drew faces one-at-a-time; Go batches by bind group.
//
// Scene render target: when water warp or translucent liquids are
// active, world renders offscreen then composites via warpscale_gogpu.go.

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (dc *DrawContext) renderWorldInternal(state *RenderFrameState) {
	worldData := dc.renderer.WorldData()
	if worldData == nil || worldData.Geometry == nil {
		slog.Debug("renderWorldInternal: no world data")
		return
	}
	hostSpeeds := pkgCVars != nil && pkgCVars.BoolValue("host_speeds")
	var (
		visibleSelectMS float64
		classifyFacesMS float64
		batchBuildMS    float64
		batchUploadMS   float64
		skyDrawMS       float64
		opaqueDrawMS    float64
		submitMS        float64
	)

	slog.Debug("renderWorldInternal: starting world render")

	// Ensure depth texture matches current surface dimensions (handles window resize).
	// Must happen before the RLock below since ensureAliasDepthTextureLocked needs a write lock.
	device := dc.renderer.getWGPUDevice()
	if device != nil {
		dc.renderer.mu.Lock()
		dc.renderer.ensureAliasDepthTextureLocked(device)
		dc.renderer.mu.Unlock()
	}

	dc.renderer.mu.RLock()
	defer dc.renderer.mu.RUnlock()

	// Check if GPU resources are ready
	if dc.renderer.worldVertexBuffer == nil || dc.renderer.worldIndexBuffer == nil {
		if worldData.TotalFaces > 0 {
			slog.Debug("renderWorldInternal: World GPU buffers not ready",
				"faces", worldData.TotalFaces,
				"triangles", worldData.TotalIndices/3)
		}
		return
	}

	if dc.renderer.resources.WorldPipeline == nil {
		slog.Debug("renderWorldInternal: World pipeline not ready")
		return
	}

	// Get HAL device and queue (device already fetched above, just need queue)
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		slog.Debug("renderWorldInternal: HAL device or queue not available for world rendering")
		return
	}

	// Obtain the frame's command encoder. When the frame graph is live this is
	// the single shared encoder every stage records into; otherwise we create a
	// private encoder so this stage still works standalone.
	slog.Debug("renderWorldInternal: obtaining command encoder")
	encoder, encoderOwned, err := dc.frameEncoder(device, "World Render Command Encoder")
	if err != nil {
		slog.Error("renderWorldInternal: Failed to create command encoder", "error", err)
		return
	}

	slog.Debug("renderWorldInternal: command encoder started")

	// Use the current surface view for zero-copy rendering (per gogpu design)
	// This allows HAL to render directly to the same surface that gogpu will composite onto
	slog.Debug("renderWorldInternal: getting surface view from gogpu context")
	textureView := dc.currentWGPURenderTargetView()
	if textureView == nil {
		slog.Debug("renderWorldInternal: Render target view not available, skipping world rendering")
		return
	}
	slog.Debug("renderWorldInternal: render target view acquired", "view_type", fmt.Sprintf("%T", textureView), "queue_type", fmt.Sprintf("%T", queue))

	var activeDynamicLights []DynamicLight
	dc.renderer.mu.RLock()
	if dc.renderer.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, dc.renderer.lightPool.ActiveLights()...)
	}
	dc.renderer.mu.RUnlock()

	// Dispatch cluster compute shader for dynamic lights
	if err := dc.renderer.dispatchWorldClusterCompute(device, queue, encoder, activeDynamicLights, dc.renderer.ViewMatrix(), dc.renderer.ProjectionMatrix()); err != nil {
		slog.Error("renderWorldInternal: Failed to dispatch world cluster compute", "error", err)
	}

	// Create render pass descriptor with color and depth attachments.
	// Use LoadOpClear to handle the clear ourselves since we skip gogpu's Clear().
	clearColor := gogpuWorldClearColor(state.ClearColor)
	slog.Debug("renderWorldInternal: creating render pass descriptor")
	renderPassDesc := &wgpu.RenderPassDescriptor{
		Label: "World Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       textureView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearColor,
			},
		},
		DepthStencilAttachment: worldDepthAttachmentForView(dc.renderer.resources.WorldDepthTextureView),
	}

	// Begin render pass
	slog.Debug("renderWorldInternal: beginning render pass")
	renderPass, err := encoder.BeginRenderPass(renderPassDesc)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to begin render pass", "error", err)
		return
	}
	slog.Debug("renderWorldInternal: render pass created", "pass", fmt.Sprintf("%T", renderPass))

	// Set pipeline
	slog.Debug("renderWorldInternal: setting pipeline", "pipeline", fmt.Sprintf("%T", dc.renderer.resources.WorldPipeline))
	renderPass.SetPipeline(dc.renderer.resources.WorldPipeline)

	// Explicit viewport/scissor to avoid backend defaults that can yield zero-area rasterization.
	w, h := dc.renderer.Size()
	if w > 0 && h > 0 {
		slog.Debug("renderWorldInternal: setting viewport", "x", 0, "y", 0, "w", w, "h", h)
		renderPass.SetViewport(0, 0, float32(w), float32(h), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	} else {
		slog.Warn("renderWorldInternal: invalid viewport size", "w", w, "h", h)
	}

	// SINGLE-SUBMIT UNIFORM MODEL: the whole frame submits once, so we cannot
	// overwrite the uniform buffer at a fixed offset between passes the way the
	// old per-pass-submit code did — gogpu flushes every queued WriteBuffer
	// before the single command buffer runs, so only the LAST write to offset 0
	// would be visible to ALL draws. Instead each distinct uniform state gets its
	// OWN dynamic offset via allocateUniformBuffer, and we re-bind group 0 to that
	// offset. The scratch is flushed to the GPU buffer once at the end of the
	// pass, and because every draw references its own offset the values stay
	// correct.
	vpMatrix := dc.renderer.ViewProjectionMatrix()
	camera := dc.renderer.cameraState
	cameraOrigin, fogDensity, timeValue := gogpuWorldUniformInputs(state, camera)
	currentLitWater := float32(0)

	// worldPassUniformBase records where this pass's uniform writes begin in the
	// shared scratch so we can flush exactly that region before submitting.
	worldPassUniformBase := dc.renderer.uniformOffset

	// bindWorldUniformOffset binds group 0 to a freshly-allocated dynamic offset
	// holding one uniform state. Returns false if the buffer could not grow.
	bindWorldUniformOffset := func(alpha, litWater, activeFogDensity float32, externalSky bool) bool {
		offset, uData := dc.renderer.allocateUniformBuffer(worldUniformBufferSize)
		if uData == nil {
			return false
		}
		if externalSky {
			fillWorldSceneUniformBytesWithExternalSkyWind(uData, vpMatrix, cameraOrigin, state.FogColor, activeFogDensity, timeValue, dc.renderer.worldSkyExternalWind, dc.renderer.resources.WorldSkyExternalWindLoaded)
		} else {
			fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, state.FogColor, activeFogDensity, timeValue, alpha, litWater)
		}
		renderPass.SetBindGroup(0, dc.renderer.resources.UniformBindGroup, []uint32{offset})
		return true
	}

	// Set vertex buffer
	slog.Debug("renderWorldInternal: setting vertex buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldVertexBuffer))
	renderPass.SetVertexBuffer(0, dc.renderer.worldVertexBuffer, 0)

	// Set index buffer (uint32 format for indices)
	slog.Debug("renderWorldInternal: setting index buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldIndexBuffer), "count", dc.renderer.worldIndexCount)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	// Bind the base uniform state (alpha=1, no lit water, default fog) at a
	// freshly allocated dynamic offset. Binding 0 is a dynamic-uniform buffer;
	// strict-validating browsers reject SetBindGroup with 0 offsets for a layout
	// with 1 dynamic buffer, and under single-submit every state must live at its
	// own offset rather than overwriting offset 0.
	if dc.renderer.resources.UniformBindGroup != nil {
		if !bindWorldUniformOffset(1, currentLitWater, worldFogUniformDensity(fogDensity), false) {
			slog.Warn("renderWorldInternal: failed to bind base world uniform offset")
		}
	} else {
		slog.Warn("renderWorldInternal: NO uniform bind group set")
	}
	// Dynamic lights ride in bind group 0 (bindings 2/3) after consolidation;
	// the light buffer must still exist because the cluster compute pass fills
	// the cluster texture that the fragment shader reads.
	if dc.renderer.resources.WorldDynamicLightsBuffer == nil {
		slog.Warn("renderWorldInternal: no dynamic light buffer available")
		_ = renderPass.End()
		return
	}

	if dc.renderer.resources.WhiteTextureBindGroup == nil || dc.renderer.resources.WhiteLightmapBindGroup == nil {
		slog.Warn("renderWorldInternal: no world texture/lightmap bind group available")
		_ = renderPass.End()
		return
	}
	timeSeconds := float64(camera.Time)
	// Update animated world texture material layers before drawing.
	if dc.renderer.resources.WorldMaterialsBuffer != nil && queue != nil {
		_ = dc.renderer.updateWorldMaterialsBuffer(queue, float32(timeSeconds))
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	worldHasLitWater := worldData.Geometry.HasLitWater
	if rDebugWaterEnabled() {
		slog.Debug("[rwater] world alpha settings",
			"water", liquidAlpha.water,
			"lava", liquidAlpha.lava,
			"slime", liquidAlpha.slime,
			"tele", liquidAlpha.tele,
			"has_lit_water", worldHasLitWater,
			"geom_transparent_water_safe", worldData.Geometry.TransparentWaterSafe,
			"geom_has_water_override", worldData.Geometry.LiquidAlphaOverrides.HasWater,
			"geom_water_override", worldData.Geometry.LiquidAlphaOverrides.Water,
		)
	}
	skyFogDensity := gogpuWorldSkyFogDensity(worldData.Geometry.Tree.Entities, fogDensity)
	currentAlpha := float32(1)
	currentFogDensity := worldFogUniformDensity(fogDensity)
	// Each writer allocates a NEW dynamic offset for the new state and re-binds
	// group 0 to it. The dedup guard (currentAlpha/litWater/fogDensity) still
	// avoids redundant rebinds when the state has not actually changed.
	writeWorldUniformWithFog := func(alpha float32, litWater float32, activeFogDensity float32) bool {
		if currentAlpha == alpha && currentLitWater == litWater && currentFogDensity == activeFogDensity {
			return true
		}
		currentAlpha = alpha
		currentLitWater = litWater
		currentFogDensity = activeFogDensity
		return bindWorldUniformOffset(alpha, litWater, activeFogDensity, false)
	}
	writeWorldUniform := func(alpha float32, litWater float32) bool {
		return writeWorldUniformWithFog(alpha, litWater, worldFogUniformDensity(fogDensity))
	}
	writeExternalSkyUniform := func(activeFogDensity float32) bool {
		currentAlpha = 1
		currentLitWater = 0
		currentFogDensity = activeFogDensity
		return bindWorldUniformOffset(1, 0, activeFogDensity, true)
	}
	cameraLeafIndex := worldLeafIndex(worldData.Geometry.Tree, camera.Origin)
	cacheEntry := dc.renderer.gogpuWorldBatchCacheEntry(cameraLeafIndex, liquidAlpha)
	cacheHit := cacheEntry != nil
	visibleFaceCount := 0
	var skyFaces []WorldFace
	var translucentLiquidFaces []WorldFace
	var batchedIndices []uint32
	var opaqueBatches []gogpuWorldFaceBatch
	var alphaTestBatches []gogpuWorldFaceBatch
	var opaqueLiquidBatches []gogpuWorldFaceBatch
	if cacheHit {
		visibleFaceCount = cacheEntry.faceCount
		skyFaces = cacheEntry.skyFaces
		translucentLiquidFaces = cacheEntry.translucentLiquid
		batchedIndices = cacheEntry.indices
		opaqueBatches = cacheEntry.opaque
		alphaTestBatches = cacheEntry.alpha
		opaqueLiquidBatches = cacheEntry.liquid
	} else {
		selectStart := time.Now()
		visibleFaces := dc.renderer.worldVisibleFacesScratch.selectVisibleWorldFaces(
			worldData.Geometry.Tree,
			worldData.Geometry.Faces,
			worldData.Geometry.LeafFaces,
			camera.Origin,
		)
		visibleSelectMS = float64(time.Since(selectStart)) / float64(time.Millisecond)
		visibleFaceCount = len(visibleFaces)
		skyFaces = dc.renderer.worldSkyFacesScratch[:0]
		translucentLiquidFaces = dc.renderer.worldTranslucentLiquidScratch[:0]
		opaqueDraws := dc.renderer.worldOpaqueDrawsScratch[:0]
		alphaTestDraws := dc.renderer.worldAlphaDrawsScratch[:0]
		opaqueLiquidDraws := dc.renderer.worldLiquidDrawsScratch[:0]
		batchedIndices = dc.renderer.worldBatchedIndexScratch[:0]
		opaqueBatches = dc.renderer.worldOpaqueBatchScratch[:0]
		alphaTestBatches = dc.renderer.worldAlphaBatchScratch[:0]
		opaqueLiquidBatches = dc.renderer.worldLiquidBatchScratch[:0]
		defer func() {
			dc.renderer.worldSkyFacesScratch = skyFaces[:0]
			dc.renderer.worldTranslucentLiquidScratch = translucentLiquidFaces[:0]
			dc.renderer.worldOpaqueDrawsScratch = opaqueDraws[:0]
			dc.renderer.worldAlphaDrawsScratch = alphaTestDraws[:0]
			dc.renderer.worldLiquidDrawsScratch = opaqueLiquidDraws[:0]
			dc.renderer.worldBatchedIndexScratch = batchedIndices[:0]
			dc.renderer.worldOpaqueBatchScratch = opaqueBatches[:0]
			dc.renderer.worldAlphaBatchScratch = alphaTestBatches[:0]
			dc.renderer.worldLiquidBatchScratch = opaqueLiquidBatches[:0]
		}()
		classifyStart := time.Now()
		for _, face := range visibleFaces {
			switch {
			case shouldDrawGoGPUSkyWorldFace(face):
				skyFaces = append(skyFaces, face)
			case shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha):
				translucentLiquidFaces = append(translucentLiquidFaces, face)
			case shouldDrawGoGPUOpaqueWorldFace(face), shouldDrawGoGPUAlphaTestWorldFace(face), shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha):
				textureBindGroup := dc.renderer.resources.WhiteTextureBindGroup
				if dc.renderer.worldTextures != nil && dc.renderer.worldTextures.bindGroup != nil {
					textureBindGroup = dc.renderer.worldTextures.bindGroup
				}
				lightmapBindGroup := dc.renderer.resources.WhiteLightmapBindGroup
				litWater := float32(0)
				if !IsGlobalPassEnabled(PassLightmaps) {
					lightmapBindGroup = dc.renderer.resources.WhiteLightmapBindGroup
					litWater = 0
				} else if shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha) {
					lightmapBindGroup, litWater = gogpuWorldLightmapArrayBindGroupForFace(face, dc.renderer.worldLightmapArray, dc.renderer.resources.WhiteLightmapBindGroup, worldHasLitWater)
					if rDebugWaterEnabled() {
						slog.Debug("[rwater] face classified as OPAQUE liquid",
							"face_idx", face.FirstIndex,
							"flags", face.Flags,
							"texture_idx", face.TextureIndex,
							"lightmap_idx", face.LightmapIndex,
							"face_alpha", worldFaceAlpha(face.Flags, liquidAlpha),
						)
					}
				} else if face.LightmapIndex >= 0 {
					if dc.renderer.worldLightmapArray != nil && dc.renderer.worldLightmapArray.bindGroup != nil {
						lightmapBindGroup = dc.renderer.worldLightmapArray.bindGroup
					}
				}
				fullbrightBindGroup := dc.renderer.resources.TransparentBindGroup
				if fullbrightBindGroup == nil {
					fullbrightBindGroup = dc.renderer.resources.WhiteTextureBindGroup
				}
				if dc.renderer.worldFullbrightTextures != nil && dc.renderer.worldFullbrightTextures.bindGroup != nil {
					fullbrightBindGroup = dc.renderer.worldFullbrightTextures.bindGroup
				}
				draw := gogpuWorldFaceDraw{
					face:                face,
					textureBindGroup:    textureBindGroup,
					lightmapBindGroup:   lightmapBindGroup,
					fullbrightBindGroup: fullbrightBindGroup,
					litWater:            litWater,
				}
				switch {
				case shouldDrawGoGPUOpaqueWorldFace(face):
					opaqueDraws = append(opaqueDraws, draw)
				case shouldDrawGoGPUAlphaTestWorldFace(face):
					alphaTestDraws = append(alphaTestDraws, draw)
				default:
					opaqueLiquidDraws = append(opaqueLiquidDraws, draw)
				}
			}
		}
		classifyFacesMS = float64(time.Since(classifyStart)) / float64(time.Millisecond)
		batchBuildStart := time.Now()
		batchedIndices, opaqueBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, opaqueBatches, opaqueDraws, worldData.Geometry.Indices)
		batchedIndices, alphaTestBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, alphaTestBatches, alphaTestDraws, worldData.Geometry.Indices)
		batchedIndices, opaqueLiquidBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, opaqueLiquidBatches, opaqueLiquidDraws, worldData.Geometry.Indices)
		batchBuildMS = float64(time.Since(batchBuildStart)) / float64(time.Millisecond)
		dc.renderer.storeGoGPUWorldBatchCacheEntry(cameraLeafIndex, liquidAlpha, visibleFaceCount, skyFaces, translucentLiquidFaces, batchedIndices, opaqueBatches, alphaTestBatches, opaqueLiquidBatches)
	}
	if rDebugWaterEnabled() {
		slog.Debug("[rwater] face classification",
			"visible_faces", visibleFaceCount,
			"translucent_liquid_faces", len(translucentLiquidFaces),
			"opaque_liquid_batches", len(opaqueLiquidBatches),
			"cache_hit", cacheHit,
		)
	}
	var opaqueBatchBuffer *wgpu.Buffer
	if len(batchedIndices) > 0 {
		batchUploadStart := time.Now()
		opaqueBatchBuffer, err = dc.renderer.ensureGoGPUWorldDynamicIndexBuffer(device, uint64(len(batchedIndices))*4)
		if err != nil {
			slog.Error("renderWorldInternal: Failed to allocate batched world index buffer", "error", err)
			_ = renderPass.End()
			return
		}
		if !cacheHit || dc.renderer.worldDynamicIndexBufferLeaf != cameraLeafIndex {
			if err := queue.WriteBuffer(opaqueBatchBuffer, 0, uint32SliceToBytes(batchedIndices)); err != nil {
				slog.Error("renderWorldInternal: Failed to upload batched world indices", "error", err)
				_ = renderPass.End()
				return
			}
			dc.renderer.worldDynamicIndexBufferLeaf = cameraLeafIndex
		}
		batchUploadMS = float64(time.Since(batchUploadStart)) / float64(time.Millisecond)
	}

	var skyDrawnIndices uint32
	if IsGlobalPassEnabled(PassSky) {
		skyDrawStart := time.Now()
		skyDrawnIndices, err = dc.renderWorldSkyPass(renderPass, skyFaces, skyFogDensity, timeSeconds, writeWorldUniformWithFog, writeExternalSkyUniform)
		if err != nil {
			_ = renderPass.End()
			return
		}
		skyDrawMS = float64(time.Since(skyDrawStart)) / float64(time.Millisecond)

		if !writeWorldUniform(1, 0) {
			slog.Error("renderWorldInternal: Failed to restore world fog uniform after sky pass")
			_ = renderPass.End()
			return
		}
	}

	var drawnIndices, alphaTestDrawnIndices, liquidDrawnIndices uint32
	if IsGlobalPassEnabled(PassWorldOpaque) {
		opaqueDrawStart := time.Now()
		drawnIndices, alphaTestDrawnIndices, liquidDrawnIndices, err = dc.renderWorldOpaquePasses(renderPass, opaqueBatches, alphaTestBatches, opaqueLiquidBatches, opaqueBatchBuffer, writeWorldUniform)
		if err != nil {
			_ = renderPass.End()
			return
		}
		slog.Debug("renderWorldInternal_stats", "drawn_indices", drawnIndices, "visible_faces", visibleFaceCount, "opaque_batches", len(opaqueBatches))
		opaqueDrawMS = float64(time.Since(opaqueDrawStart)) / float64(time.Millisecond)
	}

	if drawnIndices > 0 {
		slog.Debug("World rendered",
			"indices", drawnIndices,
			"triangles", drawnIndices/3,
			"vertices", worldData.TotalVertices)
	} else {
		slog.Debug("renderWorldInternal: No opaque world faces selected for textured draw")
	}
	if skyDrawnIndices > 0 {
		slog.Debug("GoGPU world sky rendered", "indices", skyDrawnIndices, "triangles", skyDrawnIndices/3)
	}
	if alphaTestDrawnIndices > 0 {
		slog.Debug("GoGPU alpha-test world faces rendered", "indices", alphaTestDrawnIndices, "triangles", alphaTestDrawnIndices/3, "batches", len(alphaTestBatches))
	} else if len(alphaTestBatches) > 0 {
		slog.Warn("GoGPU alpha-test batches exist but no indices drawn", "batches", len(alphaTestBatches))
	}
	if liquidDrawnIndices > 0 {
		slog.Debug("GoGPU opaque liquids rendered", "indices", liquidDrawnIndices, "triangles", liquidDrawnIndices/3)
	}

	// Translucent world liquid faces are NOT drawn here. They are deferred to
	// the entity translucent phase so they render AFTER opaque entities — C
	// Ironwail draws R_DrawWater(true) after R_DrawEntitiesOnList(false). If we
	// drew them now (before entities), opaque brushes/models would overwrite the
	// already-blended water and appear in front of it. Stash them for the
	// deferred pass (renderDeferredTranslucentWorldLiquidHAL) instead.
	if IsGlobalPassEnabled(PassTranslucentLiquids) && len(translucentLiquidFaces) > 0 {
		dc.renderer.deferredTranslucentLiquidFaces = append(dc.renderer.deferredTranslucentLiquidFaces[:0], translucentLiquidFaces...)
		dc.renderer.deferredTranslucentLiquidAlpha = liquidAlpha
		dc.renderer.deferredTranslucentLiquidLitWater = worldHasLitWater
		dc.renderer.deferredTranslucentLiquidValid = true
	} else {
		dc.renderer.deferredTranslucentLiquidValid = false
	}

	// End render pass
	slog.Debug("renderWorldInternal: ending render pass")
	logExternalSkySubmit := skyDrawnIndices > 0 &&
		dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces &&
		dc.renderer.resources.WorldSkyExternalBindGroup != nil &&
		!dc.renderer.resources.WorldSkyExternalWorldDrawLogged
	if logExternalSkySubmit {
		slog.Debug("external sky world render pass end begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderWorldInternal: render pass end error", "error", err)
	}
	if logExternalSkySubmit {
		slog.Debug("external sky world render pass end complete", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName)
	}

	// Flush this pass's uniform scratch region to the GPU buffer. Every draw
	// recorded above references a distinct dynamic offset within this region, so
	// one contiguous WriteBuffer covering [base, offset) makes all of those
	// states visible to the GPU before the single frame submit executes them.
	if dc.renderer.uniformOffset > worldPassUniformBase {
		_ = queue.WriteBuffer(dc.renderer.resources.UniformBuffer, uint64(worldPassUniformBase), dc.renderer.uniformDataScratch[worldPassUniformBase:dc.renderer.uniformOffset])
	}

	// Finish encoding and submit. In frame-graph mode this stage's commands are
	// already in the shared encoder and endFrameGraph submits them together with
	// every other stage; standalone mode finishes and submits the private
	// encoder. Either way the recorded draw work is identical.
	if logExternalSkySubmit {
		slog.Debug("external sky world encoder finish begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName)
	}
	submitStart := time.Now()
	dc.frameSubmit(queue, encoder, encoderOwned, "World Render Command Encoder")
	submitMS = float64(time.Since(submitStart)) / float64(time.Millisecond)
	if logExternalSkySubmit {
		slog.Debug("external sky world queue submit complete", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName, "submit_ms", submitMS)
		dc.renderer.resources.WorldSkyExternalWorldDrawLogged = true
	}

	if hostSpeeds {
		slog.Debug("render_world_speeds",
			"visible_select_ms", visibleSelectMS,
			"classify_faces_ms", classifyFacesMS,
			"batch_build_ms", batchBuildMS,
			"batch_upload_ms", batchUploadMS,
			"sky_draw_ms", skyDrawMS,
			"opaque_draw_ms", opaqueDrawMS,
			"submit_ms", submitMS,
			"cache_hit", cacheHit,
			"visible_faces", visibleFaceCount,
			"sky_faces", len(skyFaces),
			"opaque_batches", len(opaqueBatches),
			"alpha_test_batches", len(alphaTestBatches),
			"opaque_liquid_batches", len(opaqueLiquidBatches),
			"batched_indices", len(batchedIndices),
		)
	}
	slog.Debug("World render commands submitted successfully")
}

func (dc *DrawContext) renderExternalWorldSkyOverlayHAL(fogColor types.Vec3, fogDensity float32) {
	if dc == nil || dc.renderer == nil {
		return
	}
	worldData := dc.renderer.WorldData()
	if worldData == nil || worldData.Geometry == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	dc.renderer.mu.RLock()
	if dc.renderer.worldSkyExternalMode != externalSkyboxRenderFaces ||
		dc.renderer.resources.WorldSkyExternalOverlayPipeline == nil ||
		dc.renderer.resources.WorldSkyExternalBindGroup == nil ||
		dc.renderer.worldVertexBuffer == nil ||
		dc.renderer.worldIndexBuffer == nil ||
		dc.renderer.resources.UniformBuffer == nil ||
		dc.renderer.resources.UniformBindGroup == nil ||
		dc.renderer.resources.WhiteTextureBindGroup == nil ||
		dc.renderer.resources.WorldDepthTextureView == nil {
		dc.renderer.mu.RUnlock()
		return
	}
	pipeline := dc.renderer.resources.WorldSkyExternalOverlayPipeline
	externalSkyBindGroup := dc.renderer.resources.WorldSkyExternalBindGroup
	uniformBuffer := dc.renderer.resources.UniformBuffer
	uniformBindGroup := dc.renderer.resources.UniformBindGroup
	vertexBuffer := dc.renderer.worldVertexBuffer
	indexBuffer := dc.renderer.worldIndexBuffer
	depthView := dc.renderer.resources.WorldDepthTextureView
	camera := dc.renderer.cameraState
	vpMatrix := dc.renderer.viewMatrices.VP
	externalSkyWind := dc.renderer.worldSkyExternalWind
	externalSkyWindLoaded := dc.renderer.resources.WorldSkyExternalWindLoaded
	name := dc.renderer.resources.WorldSkyExternalName
	dc.renderer.mu.RUnlock()

	visibleFaces := selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		camera.Origin,
	)
	skyFaces := make([]WorldFace, 0, len(visibleFaces))
	for _, face := range visibleFaces {
		if shouldDrawGoGPUSkyWorldFace(face) {
			skyFaces = append(skyFaces, face)
		}
	}
	if len(skyFaces) == 0 {
		return
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "External World Sky Overlay Encoder")
	if err != nil {
		slog.Warn("external world sky overlay: failed to create encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "External World Sky Overlay Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("external world sky overlay: failed to begin render pass", "error", err)
		return
	}

	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	var uniformBytes [worldUniformBufferSize]byte
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	skyFogDensity := gogpuWorldSkyFogDensity(worldData.Geometry.Tree.Entities, fogDensity)
	fillWorldSceneUniformBytesWithExternalSkyWind(uniformBytes[:], vpMatrix, cameraOrigin, fogColor, skyFogDensity, camera.Time, externalSkyWind, externalSkyWindLoaded)
	if err := queue.WriteBuffer(uniformBuffer, 0, uniformBytes[:]); err != nil {
		slog.Warn("external world sky overlay: failed to upload uniforms", "error", err)
		_ = renderPass.End()
		return
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, uniformBindGroup, []uint32{0})
	renderPass.SetBindGroup(1, externalSkyBindGroup, nil)
	renderPass.SetVertexBuffer(0, vertexBuffer, 0)
	renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
	var drawnIndices uint32
	for _, face := range skyFaces {
		renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		drawnIndices += face.NumIndices
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("external world sky overlay: render pass end error", "error", err)
		return
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "External World Sky Overlay Encoder")
	slog.Debug("external world sky overlay rendered", "subsystem", externalSkyboxLogSubsystem, "name", name, "sky_faces", len(skyFaces), "indices", drawnIndices)
}

// matrixToBytes converts a types.Mat4 to bytes (column-major, little-endian).
func matrixToBytes(m types.Mat4) []byte {
	b := types.Mat4ToBytes(m)
	return b[:]
}

func (r *Renderer) resetGoGPUWorldBatchCache() {
	for i := range r.worldBatchCacheEntries {
		entry := &r.worldBatchCacheEntries[i]
		entry.valid = false
		entry.leaf = 0
		entry.faceCount = 0
		entry.skyFaces = nil
		entry.translucentLiquid = nil
		entry.indices = nil
		entry.opaque = nil
		entry.alpha = nil
		entry.liquid = nil
	}
	r.worldBatchCacheNext = 0
}

func (r *Renderer) gogpuWorldBatchCacheEntry(leaf int, liquidAlpha worldLiquidAlphaSettings) *gogpuWorldBatchCacheEntry {
	for i := range r.worldBatchCacheEntries {
		entry := &r.worldBatchCacheEntries[i]
		if entry.valid && entry.leaf == leaf && entry.liquidAlpha == liquidAlpha {
			return entry
		}
	}
	return nil
}

func (r *Renderer) storeGoGPUWorldBatchCacheEntry(leaf int, liquidAlpha worldLiquidAlphaSettings, faceCount int, skyFaces, translucentLiquid []WorldFace, batchedIndices []uint32, opaqueBatches, alphaTestBatches, opaqueLiquidBatches []gogpuWorldFaceBatch) {
	if leaf < 0 {
		return
	}
	entry := r.gogpuWorldBatchCacheEntry(leaf, liquidAlpha)
	if entry == nil {
		entry = &r.worldBatchCacheEntries[r.worldBatchCacheNext]
		r.worldBatchCacheNext = (r.worldBatchCacheNext + 1) % len(r.worldBatchCacheEntries)
	}
	entry.valid = true
	entry.leaf = leaf
	entry.liquidAlpha = liquidAlpha
	entry.faceCount = faceCount
	entry.skyFaces = append(entry.skyFaces[:0], skyFaces...)
	entry.translucentLiquid = append(entry.translucentLiquid[:0], translucentLiquid...)
	entry.indices = append(entry.indices[:0], batchedIndices...)
	entry.opaque = append(entry.opaque[:0], opaqueBatches...)
	entry.alpha = append(entry.alpha[:0], alphaTestBatches...)
	entry.liquid = append(entry.liquid[:0], opaqueLiquidBatches...)
}

func fillWorldSceneUniformBytes(dst []byte, vp types.Mat4, cameraOrigin [3]float32, fogColor types.Vec3, fogDensity float32, time float32, alpha float32, litWater float32) {
	clear(dst[:worldUniformBufferSize])
	matrixBytes := matrixToBytes(vp)
	copy(dst[:64], matrixBytes)
	putFloat32s(dst[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(dst[76:80], math.Float32bits(fogDensity))
	putFloat32s(dst[80:92], fogColor.Slice())
	binary.LittleEndian.PutUint32(dst[92:96], math.Float32bits(time))
	binary.LittleEndian.PutUint32(dst[96:100], math.Float32bits(alpha))
	binary.LittleEndian.PutUint32(dst[100:104], math.Float32bits(litWater))
}

func fillWorldSceneUniformBytesWithExternalSkyWind(dst []byte, vp types.Mat4, cameraOrigin [3]float32, fogColor types.Vec3, fogDensity float32, timeValue float32, wind externalSkyboxWind, windLoaded bool) {
	fillWorldSceneUniformBytes(dst, vp, cameraOrigin, fogColor, fogDensity, timeValue, 1, 0)
	if !windLoaded || wind.Dist == 0 {
		return
	}
	yaw := float64(wind.Yaw) * math.Pi / 180
	pitch := float64(wind.Pitch) * math.Pi / 180
	sy, cy := math.Sin(yaw), math.Cos(yaw)
	sp, cp := math.Sin(pitch), math.Cos(pitch)
	dist := float64(clampExternalSkyWindDist(wind.Dist))
	period := float64(wind.Period)
	phase := 0.5
	if period != 0 {
		phase = float64(timeValue) * 0.5 / period
	}
	phase -= math.Floor(phase) + 0.5
	windDir := [3]float32{
		float32(dist * cp * sy),
		float32(dist * sp),
		float32(-dist * cp * cy),
	}
	binary.LittleEndian.PutUint32(dst[104:108], math.Float32bits(float32(phase)))
	putFloat32s(dst[112:124], windDir[:])
	binary.LittleEndian.PutUint32(dst[124:128], math.Float32bits(1))
}

func clampExternalSkyWindDist(dist float32) float32 {
	if dist < -2 {
		return -2
	}
	if dist > 2 {
		return 2
	}
	return dist
}

func gogpuWorldUniformInputs(state *RenderFrameState, camera CameraState) ([3]float32, float32, float32) {
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	return cameraOrigin, state.FogDensity, camera.Time
}

func gogpuWorldClearColor(clear [4]float32) gputypes.Color {
	if os.Getenv("IRONWAIL_DEBUG_WORLD_CLEAR_GREEN") == "1" {
		return gputypes.Color{R: 0.0, G: 1.0, B: 0.0, A: 1.0}
	}
	return gputypes.Color{
		R: float64(clear[0]),
		G: float64(clear[1]),
		B: float64(clear[2]),
		A: float64(clear[3]),
	}
}

func (dc *DrawContext) clearGoGPUSharedDepthStencil() {
	if dc == nil || dc.renderer == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	dc.renderer.mu.Lock()
	dc.renderer.ensureAliasDepthTextureLocked(device)
	depthView := dc.renderer.resources.WorldDepthTextureView
	dc.renderer.mu.Unlock()
	attachment := gogpuSharedDepthStencilClearAttachmentForView(depthView)
	if attachment == nil {
		return
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "GoGPU Shared Depth Clear Encoder")
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to create encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoGPU Shared Depth Clear Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: attachment,
	})
	if err != nil {
		slog.Error("clearGoGPUSharedDepthStencil: Failed to begin render pass", "error", err)
		return
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: render pass end error", "error", err)
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "GoGPU Shared Depth Clear Encoder")
}
