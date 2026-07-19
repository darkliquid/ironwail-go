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

	if dc.renderer.worldPipeline == nil {
		slog.Debug("renderWorldInternal: World pipeline not ready")
		return
	}

	// Get HAL device and queue (device already fetched above, just need queue)
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		slog.Debug("renderWorldInternal: HAL device or queue not available for world rendering")
		return
	}

	// Create command encoder
	slog.Debug("renderWorldInternal: creating command encoder")
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "World Render Command Encoder",
	})
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
		DepthStencilAttachment: worldDepthAttachmentForView(dc.renderer.worldDepthTextureView),
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
	slog.Debug("renderWorldInternal: setting pipeline", "pipeline", fmt.Sprintf("%T", dc.renderer.worldPipeline))
	renderPass.SetPipeline(dc.renderer.worldPipeline)

	// Explicit viewport/scissor to avoid backend defaults that can yield zero-area rasterization.
	w, h := dc.renderer.Size()
	if w > 0 && h > 0 {
		slog.Debug("renderWorldInternal: setting viewport", "x", 0, "y", 0, "w", w, "h", h)
		renderPass.SetViewport(0, 0, float32(w), float32(h), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	} else {
		slog.Warn("renderWorldInternal: invalid viewport size", "w", w, "h", h)
	}

	// Update uniform buffer with VP matrix
	vpMatrix := dc.renderer.ViewProjectionMatrix()
	camera := dc.renderer.cameraState
	cameraOrigin, fogDensity, timeValue := gogpuWorldUniformInputs(state, camera)
	currentLitWater := float32(0)
	var uniformBytes [worldUniformBufferSize]byte
	fillWorldSceneUniformBytes(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, worldFogUniformDensity(fogDensity), timeValue, 1, currentLitWater)
	slog.Debug("renderWorldInternal: VP matrix",
		"m00", vpMatrix[0], "m11", vpMatrix[5], "m22", vpMatrix[10], "m33", vpMatrix[15])
	slog.Debug("renderWorldInternal: writing uniform buffer", "bytes_len", len(uniformBytes))
	err = queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes[:])
	if err != nil {
		slog.Error("renderWorldInternal: Failed to update uniform buffer", "error", err)
		_ = renderPass.End()
		return
	}

	// Set vertex buffer
	slog.Debug("renderWorldInternal: setting vertex buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldVertexBuffer))
	renderPass.SetVertexBuffer(0, dc.renderer.worldVertexBuffer, 0)

	// Set index buffer (uint32 format for indices)
	slog.Debug("renderWorldInternal: setting index buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldIndexBuffer), "count", dc.renderer.worldIndexCount)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	// Set uniform bind group.
	if dc.renderer.uniformBindGroup != nil {
		slog.Debug("renderWorldInternal: setting bind group", "group", fmt.Sprintf("%T", dc.renderer.uniformBindGroup))
		renderPass.SetBindGroup(0, dc.renderer.uniformBindGroup, nil)
	} else {
		slog.Warn("renderWorldInternal: NO uniform bind group set")
	}
	if dc.renderer.worldDynamicLightsBindGroup == nil || dc.renderer.worldDynamicLightsBuffer == nil {
		slog.Warn("renderWorldInternal: no dynamic light bind group available")
		_ = renderPass.End()
		return
	}
	// Light buffer was already uploaded in dispatchWorldClusterCompute
	renderPass.SetBindGroup(4, dc.renderer.worldDynamicLightsBindGroup, nil)

	if dc.renderer.whiteTextureBindGroup == nil || dc.renderer.whiteLightmapBindGroup == nil {
		slog.Warn("renderWorldInternal: no world texture/lightmap bind group available")
		_ = renderPass.End()
		return
	}
	timeSeconds := float64(camera.Time)
	// Update animated world texture material layers before drawing.
	if dc.renderer.worldMaterialsBuffer != nil && queue != nil {
		_ = dc.renderer.updateWorldMaterialsBuffer(queue, float32(timeSeconds))
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	worldHasLitWater := worldData.Geometry.HasLitWater
	skyFogDensity := gogpuWorldSkyFogDensity(worldData.Geometry.Tree.Entities, fogDensity)
	currentAlpha := float32(1)
	currentFogDensity := worldFogUniformDensity(fogDensity)
	writeWorldUniformWithFog := func(alpha float32, litWater float32, activeFogDensity float32) bool {
		if currentAlpha == alpha && currentLitWater == litWater && currentFogDensity == activeFogDensity {
			return true
		}
		currentAlpha = alpha
		currentLitWater = litWater
		currentFogDensity = activeFogDensity
		fillWorldSceneUniformBytes(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, activeFogDensity, timeValue, alpha, litWater)
		return queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes[:]) == nil
	}
	writeWorldUniform := func(alpha float32, litWater float32) bool {
		return writeWorldUniformWithFog(alpha, litWater, worldFogUniformDensity(fogDensity))
	}
	writeExternalSkyUniform := func(activeFogDensity float32) bool {
		currentAlpha = 1
		currentLitWater = 0
		currentFogDensity = activeFogDensity
		fillWorldSceneUniformBytesWithExternalSkyWind(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, activeFogDensity, timeValue, dc.renderer.worldSkyExternalWind, dc.renderer.worldSkyExternalWindLoaded)
		return queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes[:]) == nil
	}
	cameraOriginWorld := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraLeafIndex := worldLeafIndex(worldData.Geometry.Tree, cameraOriginWorld)
	cacheEntry := dc.renderer.gogpuWorldBatchCacheEntry(cameraLeafIndex)
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
			cameraOriginWorld,
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
				textureBindGroup := dc.renderer.whiteTextureBindGroup
				if dc.renderer.worldTextures != nil && dc.renderer.worldTextures.bindGroup != nil {
					textureBindGroup = dc.renderer.worldTextures.bindGroup
				}
				lightmapBindGroup := dc.renderer.whiteLightmapBindGroup
				litWater := float32(0)
				if shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha) {
					lightmapBindGroup, litWater = gogpuWorldLightmapArrayBindGroupForFace(face, dc.renderer.worldLightmapArray, dc.renderer.whiteLightmapBindGroup, worldHasLitWater)
				} else if face.LightmapIndex >= 0 {
					if dc.renderer.worldLightmapArray != nil && dc.renderer.worldLightmapArray.bindGroup != nil {
						lightmapBindGroup = dc.renderer.worldLightmapArray.bindGroup
					}
				}
				fullbrightBindGroup := dc.renderer.transparentBindGroup
				if fullbrightBindGroup == nil {
					fullbrightBindGroup = dc.renderer.whiteTextureBindGroup
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
		dc.renderer.storeGoGPUWorldBatchCacheEntry(cameraLeafIndex, visibleFaceCount, skyFaces, translucentLiquidFaces, batchedIndices, opaqueBatches, alphaTestBatches, opaqueLiquidBatches)
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
		if err := queue.WriteBuffer(opaqueBatchBuffer, 0, uint32SliceToBytes(batchedIndices)); err != nil {
			slog.Error("renderWorldInternal: Failed to upload batched world indices", "error", err)
			_ = renderPass.End()
			return
		}
		batchUploadMS = float64(time.Since(batchUploadStart)) / float64(time.Millisecond)
	}

	skyDrawnIndices := uint32(0)
	var materialBindState gogpuWorldMaterialBindState
	skyDrawStart := time.Now()
	if dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces && dc.renderer.worldSkyExternalPipeline != nil && dc.renderer.worldSkyExternalBindGroup != nil {
		logExternalSkyDraw := !dc.renderer.worldSkyExternalWorldDrawLogged
		if logExternalSkyDraw {
			slog.Info("external sky world draw begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "sky_faces", len(skyFaces))
		}
		if !writeExternalSkyUniform(skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			_ = renderPass.End()
			return
		}
		if logExternalSkyDraw {
			slog.Info("external sky world draw uniform written", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "sky_fog_density", skyFogDensity, "wind_loaded", dc.renderer.worldSkyExternalWindLoaded, "wind_dist", dc.renderer.worldSkyExternalWind.Dist, "wind_period", dc.renderer.worldSkyExternalWind.Period)
		}
		renderPass.SetPipeline(dc.renderer.worldSkyExternalPipeline)
		renderPass.SetBindGroup(1, dc.renderer.worldSkyExternalBindGroup, nil)
		// The external sky shader only samples bind group 1, but the pipeline
		// layout intentionally matches the shared world layout so group 4
		// (dynamic lights) can remain bound across sky and brush draws.
		// WebGPU still requires every lower bind group in that layout to be set.
		renderPass.SetBindGroup(2, dc.renderer.whiteTextureBindGroup, nil)
		renderPass.SetBindGroup(3, dc.renderer.whiteTextureBindGroup, nil)
		if logExternalSkyDraw {
			slog.Info("external sky world draw pipeline bound", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
		}
		for _, face := range skyFaces {
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
		if logExternalSkyDraw {
			slog.Info("external sky world draw commands encoded", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "drawn_indices", skyDrawnIndices, "triangles", skyDrawnIndices/3)
		}
	} else if dc.renderer.worldSkyPipeline != nil {
		if !writeWorldUniformWithFog(1, 0, skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			_ = renderPass.End()
			return
		}
		renderPass.SetPipeline(dc.renderer.worldSkyPipeline)
		materialBindState.invalidate()
		for _, face := range skyFaces {
			textureIndex := resolveWorldSkyTextureIndex(face, dc.renderer.worldTextureAnimations, 0, timeSeconds)
			solidBindGroup := dc.renderer.whiteTextureBindGroup
			if worldTexture := dc.renderer.worldSkySolidTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				solidBindGroup = worldTexture.bindGroup
			}
			alphaBindGroup := dc.renderer.transparentBindGroup
			if alphaBindGroup == nil {
				alphaBindGroup = dc.renderer.whiteTextureBindGroup
			}
			if worldTexture := dc.renderer.worldSkyAlphaTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				alphaBindGroup = worldTexture.bindGroup
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(solidBindGroup, alphaBindGroup, dc.renderer.whiteTextureBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, solidBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, alphaBindGroup, nil)
			}
			// Bind group 3 (fullbright/lightmap) is required by the shared pipeline
			// layout even though the sky shader doesn't use it.
			if setFullbright {
				renderPass.SetBindGroup(3, dc.renderer.whiteTextureBindGroup, nil)
			}
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
	}
	skyDrawMS = float64(time.Since(skyDrawStart)) / float64(time.Millisecond)

	if !writeWorldUniform(1, 0) {
		slog.Error("renderWorldInternal: Failed to restore world fog uniform after sky pass")
		_ = renderPass.End()
		return
	}

	renderPass.SetPipeline(dc.renderer.worldPipeline)
	materialBindState.invalidate()
	drawnIndices := uint32(0)
	alphaTestDrawnIndices := uint32(0)
	liquidDrawnIndices := uint32(0)
	if opaqueBatchBuffer != nil {
		renderPass.SetIndexBuffer(opaqueBatchBuffer, gputypes.IndexFormatUint32, 0)
	}
	opaqueDrawStart := time.Now()
	for _, batch := range opaqueBatches {
		if !writeWorldUniform(1, batch.key.litWater) {
			slog.Error("renderWorldInternal: Failed to update world dynamic-light uniform")
			_ = renderPass.End()
			return
		}
		setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
		if setTexture {
			renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
		}
		if setLightmap {
			renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
		}
		if setFullbright {
			renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
		}
		renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
		drawnIndices += batch.numIndices
	}
	if dc.renderer.worldAlphaTestPipeline != nil {
		renderPass.SetPipeline(dc.renderer.worldAlphaTestPipeline)
		materialBindState.invalidate()
		for _, batch := range alphaTestBatches {
			if !writeWorldUniform(1, batch.key.litWater) {
				slog.Error("renderWorldInternal: Failed to update alpha-test world dynamic-light uniform")
				_ = renderPass.End()
				return
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
			}
			if setFullbright {
				renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
			}
			renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
			alphaTestDrawnIndices += batch.numIndices
		}
	} else if len(alphaTestBatches) > 0 {
		slog.Warn("renderWorldInternal: alpha-test faces exist but alpha-test pipeline is nil",
			"alpha_test_batches", len(alphaTestBatches))
	}
	if dc.renderer.worldTurbulentPipeline != nil {
		renderPass.SetPipeline(dc.renderer.worldTurbulentPipeline)
		materialBindState.invalidate()
		for _, batch := range opaqueLiquidBatches {
			if !writeWorldUniform(1, batch.key.litWater) {
				slog.Error("renderWorldInternal: Failed to update liquid lighting uniform")
				_ = renderPass.End()
				return
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
			}
			if setFullbright {
				renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
			}
			renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
			liquidDrawnIndices += batch.numIndices
		}
	}
	opaqueDrawMS = float64(time.Since(opaqueDrawStart)) / float64(time.Millisecond)
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

	// End render pass
	slog.Debug("renderWorldInternal: ending render pass")
	logExternalSkySubmit := skyDrawnIndices > 0 &&
		dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces &&
		dc.renderer.worldSkyExternalBindGroup != nil &&
		!dc.renderer.worldSkyExternalWorldDrawLogged
	if logExternalSkySubmit {
		slog.Info("external sky world render pass end begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderWorldInternal: render pass end error", "error", err)
	}
	if logExternalSkySubmit {
		slog.Info("external sky world render pass end complete", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
	}

	// Finish encoding and get command buffer
	if logExternalSkySubmit {
		slog.Info("external sky world encoder finish begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Error("renderWorldInternal: Failed to finish command encoding", "error", err)
		return
	}
	if logExternalSkySubmit {
		slog.Info("external sky world encoder finish complete", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
	}

	// Submit to queue
	slog.Debug("renderWorldInternal: submitting to queue")
	if logExternalSkySubmit {
		slog.Info("external sky world queue submit begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName)
	}
	submitStart := time.Now()
	_, err = queue.Submit(cmdBuffer)
	submitMS = float64(time.Since(submitStart)) / float64(time.Millisecond)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to submit render commands", "error", err)
		return
	}
	if logExternalSkySubmit {
		slog.Info("external sky world queue submit complete", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "submit_ms", submitMS)
		dc.renderer.worldSkyExternalWorldDrawLogged = true
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

func (dc *DrawContext) renderExternalWorldSkyOverlayHAL(fogColor [3]float32, fogDensity float32) {
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
		dc.renderer.worldSkyExternalOverlayPipeline == nil ||
		dc.renderer.worldSkyExternalBindGroup == nil ||
		dc.renderer.worldVertexBuffer == nil ||
		dc.renderer.worldIndexBuffer == nil ||
		dc.renderer.uniformBuffer == nil ||
		dc.renderer.uniformBindGroup == nil ||
		dc.renderer.whiteTextureBindGroup == nil ||
		dc.renderer.worldDynamicLightsBindGroup == nil ||
		dc.renderer.worldDepthTextureView == nil {
		dc.renderer.mu.RUnlock()
		return
	}
	pipeline := dc.renderer.worldSkyExternalOverlayPipeline
	externalSkyBindGroup := dc.renderer.worldSkyExternalBindGroup
	whiteTextureBindGroup := dc.renderer.whiteTextureBindGroup
	dynamicLightsBindGroup := dc.renderer.worldDynamicLightsBindGroup
	uniformBuffer := dc.renderer.uniformBuffer
	uniformBindGroup := dc.renderer.uniformBindGroup
	vertexBuffer := dc.renderer.worldVertexBuffer
	indexBuffer := dc.renderer.worldIndexBuffer
	depthView := dc.renderer.worldDepthTextureView
	camera := dc.renderer.cameraState
	vpMatrix := dc.renderer.viewMatrices.VP
	externalSkyWind := dc.renderer.worldSkyExternalWind
	externalSkyWindLoaded := dc.renderer.worldSkyExternalWindLoaded
	name := dc.renderer.worldSkyExternalName
	dc.renderer.mu.RUnlock()

	visibleFaces := selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
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

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "External World Sky Overlay Encoder"})
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
	renderPass.SetBindGroup(0, uniformBindGroup, nil)
	renderPass.SetBindGroup(1, externalSkyBindGroup, nil)
	renderPass.SetBindGroup(2, whiteTextureBindGroup, nil)
	renderPass.SetBindGroup(3, whiteTextureBindGroup, nil)
	renderPass.SetBindGroup(4, dynamicLightsBindGroup, nil)
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
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("external world sky overlay: failed to finish encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("external world sky overlay: failed to submit", "error", err)
		return
	}
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

func (r *Renderer) gogpuWorldBatchCacheEntry(leaf int) *gogpuWorldBatchCacheEntry {
	for i := range r.worldBatchCacheEntries {
		entry := &r.worldBatchCacheEntries[i]
		if entry.valid && entry.leaf == leaf {
			return entry
		}
	}
	return nil
}

func (r *Renderer) storeGoGPUWorldBatchCacheEntry(leaf int, faceCount int, skyFaces, translucentLiquid []WorldFace, batchedIndices []uint32, opaqueBatches, alphaTestBatches, opaqueLiquidBatches []gogpuWorldFaceBatch) {
	if leaf < 0 {
		return
	}
	entry := r.gogpuWorldBatchCacheEntry(leaf)
	if entry == nil {
		entry = &r.worldBatchCacheEntries[r.worldBatchCacheNext]
		r.worldBatchCacheNext = (r.worldBatchCacheNext + 1) % len(r.worldBatchCacheEntries)
	}
	entry.valid = true
	entry.leaf = leaf
	entry.faceCount = faceCount
	entry.skyFaces = append(entry.skyFaces[:0], skyFaces...)
	entry.translucentLiquid = append(entry.translucentLiquid[:0], translucentLiquid...)
	entry.indices = append(entry.indices[:0], batchedIndices...)
	entry.opaque = append(entry.opaque[:0], opaqueBatches...)
	entry.alpha = append(entry.alpha[:0], alphaTestBatches...)
	entry.liquid = append(entry.liquid[:0], opaqueLiquidBatches...)
}

func fillWorldSceneUniformBytes(dst []byte, vp types.Mat4, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32, time float32, alpha float32, litWater float32) {
	clear(dst[:worldUniformBufferSize])
	matrixBytes := matrixToBytes(vp)
	copy(dst[:64], matrixBytes)
	putFloat32s(dst[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(dst[76:80], math.Float32bits(fogDensity))
	putFloat32s(dst[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(dst[92:96], math.Float32bits(time))
	binary.LittleEndian.PutUint32(dst[96:100], math.Float32bits(alpha))
	binary.LittleEndian.PutUint32(dst[100:104], math.Float32bits(litWater))
}

func fillWorldSceneUniformBytesWithExternalSkyWind(dst []byte, vp types.Mat4, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32, timeValue float32, wind externalSkyboxWind, windLoaded bool) {
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
	depthView := dc.renderer.worldDepthTextureView
	dc.renderer.mu.Unlock()
	attachment := gogpuSharedDepthStencilClearAttachmentForView(depthView)
	if attachment == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoGPU Shared Depth Clear Encoder"})
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
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to finish encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to submit clear pass", "error", err)
	}
}
