package renderer

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (dc *DrawContext) renderWorldSkyPass(
	renderPass *wgpu.RenderPassEncoder,
	skyFaces []WorldFace,
	skyFogDensity float32,
	timeSeconds float64,
	writeWorldUniformWithFog func(float32, float32, float32) bool,
	writeExternalSkyUniform func(float32) bool,
) (skyDrawnIndices uint32, err error) {
	if dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces && dc.renderer.worldSkyExternalPipeline != nil && dc.renderer.worldSkyExternalBindGroup != nil {
		logExternalSkyDraw := !dc.renderer.worldSkyExternalWorldDrawLogged
		if logExternalSkyDraw {
			slog.Info("external sky world draw begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "sky_faces", len(skyFaces))
		}
		if !writeExternalSkyUniform(skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			return 0, fmt.Errorf("failed to update sky fog uniform")
		}
		if logExternalSkyDraw {
			slog.Info("external sky world draw uniform written", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.worldSkyExternalName, "sky_fog_density", skyFogDensity, "wind_loaded", dc.renderer.worldSkyExternalWindLoaded, "wind_dist", dc.renderer.worldSkyExternalWind.Dist, "wind_period", dc.renderer.worldSkyExternalWind.Period)
		}
		renderPass.SetPipeline(dc.renderer.worldSkyExternalPipeline)
		renderPass.SetBindGroup(1, dc.renderer.worldSkyExternalBindGroup, nil)
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
			return 0, fmt.Errorf("failed to update sky fog uniform")
		}
		renderPass.SetPipeline(dc.renderer.worldSkyPipeline)
		var materialBindState gogpuWorldMaterialBindState
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
			if setFullbright {
				renderPass.SetBindGroup(3, dc.renderer.whiteTextureBindGroup, nil)
			}
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
	}
	return skyDrawnIndices, nil
}

func (dc *DrawContext) renderWorldOpaquePasses(
	renderPass *wgpu.RenderPassEncoder,
	opaqueBatches, alphaTestBatches, opaqueLiquidBatches []gogpuWorldFaceBatch,
	opaqueBatchBuffer *wgpu.Buffer,
	writeWorldUniform func(float32, float32) bool,
) (drawnIndices, alphaTestDrawnIndices, liquidDrawnIndices uint32, err error) {
	var materialBindState gogpuWorldMaterialBindState
	renderPass.SetPipeline(dc.renderer.worldPipeline)
	materialBindState.invalidate()
	if opaqueBatchBuffer != nil {
		renderPass.SetIndexBuffer(opaqueBatchBuffer, gputypes.IndexFormatUint32, 0)
	}
	for _, batch := range opaqueBatches {
		if !writeWorldUniform(1, batch.key.litWater) {
			return 0, 0, 0, fmt.Errorf("failed to update world dynamic-light uniform")
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
				return 0, 0, 0, fmt.Errorf("failed to update alpha-test world dynamic-light uniform")
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
		for i, batch := range opaqueLiquidBatches {
			if rDebugWaterEnabled() {
				slog.Debug("[rwater] opaque liquid batch",
					"batch_idx", i,
					"num_indices", batch.numIndices,
					"lit_water", batch.key.litWater,
					"alpha_uniform", 1,
					"texture_bind_group", fmt.Sprintf("%p", batch.key.textureBindGroup),
					"lightmap_bind_group", fmt.Sprintf("%p", batch.key.lightmapBindGroup),
				)
			}
			if !writeWorldUniform(1, batch.key.litWater) {
				return 0, 0, 0, fmt.Errorf("failed to update liquid lighting uniform")
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
	return drawnIndices, alphaTestDrawnIndices, liquidDrawnIndices, nil
}

func (dc *DrawContext) renderWorldTranslucentPass(
	renderPass *wgpu.RenderPassEncoder,
	translucentLiquidFaces []WorldFace,
	worldHasLitWater bool,
	liquidAlpha worldLiquidAlphaSettings,
	vpMatrix [16]float32,
	cameraOrigin [3]float32,
	state *RenderFrameState,
	fogDensity float32,
	timeValue float32,
	queue *wgpu.Queue,
	worldIndices []uint32,
	batchedIndices []uint32,
) ([]uint32, error) {
	if dc.renderer.worldTranslucentTurbulentPipeline == nil || len(translucentLiquidFaces) == 0 {
		return batchedIndices, nil
	}
	dynUniformStart := dc.renderer.uniformOffset
	translucentLiquidDraws := dc.renderer.worldLiquidDrawsScratch[:0]
	for _, face := range translucentLiquidFaces {
		textureBindGroup := dc.renderer.whiteTextureBindGroup
		if dc.renderer.worldTextures != nil && dc.renderer.worldTextures.bindGroup != nil {
			textureBindGroup = dc.renderer.worldTextures.bindGroup
		}
		lightmapBindGroup, litWater := gogpuWorldLightmapArrayBindGroupForFace(face, dc.renderer.worldLightmapArray, dc.renderer.whiteLightmapBindGroup, worldHasLitWater)
		fullbrightBindGroup := dc.renderer.transparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = dc.renderer.whiteTextureBindGroup
		}
		if dc.renderer.worldFullbrightTextures != nil && dc.renderer.worldFullbrightTextures.bindGroup != nil {
			fullbrightBindGroup = dc.renderer.worldFullbrightTextures.bindGroup
		}
		translucentLiquidDraws = append(translucentLiquidDraws, gogpuWorldFaceDraw{
			face:                face,
			textureBindGroup:    textureBindGroup,
			lightmapBindGroup:   lightmapBindGroup,
			fullbrightBindGroup: fullbrightBindGroup,
			litWater:            litWater,
		})
	}
	translucentLiquidBatches := dc.renderer.worldLiquidBatchScratch[:0]
	batchedIndices, translucentLiquidBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, translucentLiquidBatches, translucentLiquidDraws, worldIndices)

	translucentAlpha := liquidAlpha.water
	for _, batch := range translucentLiquidBatches {
		tOffset, tUData := dc.renderer.allocateUniformBuffer(worldUniformBufferSize)
		fillWorldSceneUniformBytes(tUData, vpMatrix, cameraOrigin, state.FogColor, worldFogUniformDensity(fogDensity), timeValue, translucentAlpha, batch.key.litWater)
		_ = tOffset
	}
	if dc.renderer.uniformOffset > dynUniformStart {
		dynData := dc.renderer.uniformDataScratch[dynUniformStart:dc.renderer.uniformOffset]
		_ = queue.WriteBuffer(dc.renderer.uniformBuffer, uint64(dynUniformStart), dynData)
	}

	renderPass.SetPipeline(dc.renderer.worldTranslucentTurbulentPipeline)
	var materialBindState gogpuWorldMaterialBindState
	materialBindState.invalidate()
	tOffset := dynUniformStart
	for i, batch := range translucentLiquidBatches {
		if rDebugWaterEnabled() {
			slog.Debug("[rwater] translucent liquid batch (in-world-pass)",
				"batch_idx", i,
				"num_indices", batch.numIndices,
				"lit_water", batch.key.litWater,
				"alpha", translucentAlpha,
				"uniform_offset", tOffset,
			)
		}
		renderPass.SetBindGroup(0, dc.renderer.uniformBindGroup, []uint32{tOffset})
		tOffset += worldUniformAlign
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
	}
	return batchedIndices, nil
}
