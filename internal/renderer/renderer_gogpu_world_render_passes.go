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
	if dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces && dc.renderer.resources.WorldSkyExternalPipeline != nil && dc.renderer.resources.WorldSkyExternalBindGroup != nil {
		logExternalSkyDraw := !dc.renderer.resources.WorldSkyExternalWorldDrawLogged
		if logExternalSkyDraw {
			slog.Debug("external sky world draw begin", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName, "sky_faces", len(skyFaces))
		}
		if !writeExternalSkyUniform(skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			return 0, fmt.Errorf("failed to update sky fog uniform")
		}
		if logExternalSkyDraw {
			slog.Debug("external sky world draw uniform written", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName, "sky_fog_density", skyFogDensity, "wind_loaded", dc.renderer.resources.WorldSkyExternalWindLoaded, "wind_dist", dc.renderer.worldSkyExternalWind.Dist, "wind_period", dc.renderer.worldSkyExternalWind.Period)
		}
		renderPass.SetPipeline(dc.renderer.resources.WorldSkyExternalPipeline)
		renderPass.SetBindGroup(1, dc.renderer.resources.WorldSkyExternalBindGroup, nil)
		// The external sky pipeline layout chains only groups 0 (uniform,
		// already bound by the caller) and 1 (sky faces); do not bind
		// placeholder groups 2-3, which strict-validating browsers reject.
		if logExternalSkyDraw {
			slog.Debug("external sky world draw pipeline bound", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName)
		}
		for _, face := range skyFaces {
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
		if logExternalSkyDraw {
			slog.Debug("external sky world draw commands encoded", "subsystem", externalSkyboxLogSubsystem, "name", dc.renderer.resources.WorldSkyExternalName, "drawn_indices", skyDrawnIndices, "triangles", skyDrawnIndices/3)
		}
	} else if dc.renderer.resources.WorldSkyPipeline != nil {
		if !writeWorldUniformWithFog(1, 0, skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			return 0, fmt.Errorf("failed to update sky fog uniform")
		}
		renderPass.SetPipeline(dc.renderer.resources.WorldSkyPipeline)
		var materialBindState gogpuWorldMaterialBindState
		materialBindState.invalidate()
		for _, face := range skyFaces {
			textureIndex := resolveWorldSkyTextureIndex(face, dc.renderer.worldTextureAnimations, 0, timeSeconds)
			solidBindGroup := dc.renderer.resources.WhiteTextureBindGroup
			if worldTexture := dc.renderer.worldSkySolidTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				solidBindGroup = worldTexture.bindGroup
			}
			alphaBindGroup := dc.renderer.resources.TransparentBindGroup
			if alphaBindGroup == nil {
				alphaBindGroup = dc.renderer.resources.WhiteTextureBindGroup
			}
			if worldTexture := dc.renderer.worldSkyAlphaTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				alphaBindGroup = worldTexture.bindGroup
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(solidBindGroup, alphaBindGroup, dc.renderer.resources.WhiteTextureBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, solidBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, alphaBindGroup, nil)
			}
			if setFullbright {
				renderPass.SetBindGroup(3, dc.renderer.resources.WhiteTextureBindGroup, nil)
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
	renderPass.SetPipeline(dc.renderer.resources.WorldPipeline)
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
	if dc.renderer.resources.WorldAlphaTestPipeline != nil {
		renderPass.SetPipeline(dc.renderer.resources.WorldAlphaTestPipeline)
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
	if dc.renderer.resources.WorldTurbulentPipeline != nil {
		renderPass.SetPipeline(dc.renderer.resources.WorldTurbulentPipeline)
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
	writeWorldUniform func(float32, float32) bool,
) error {
	if dc.renderer.resources.WorldTranslucentTurbulentPipeline == nil || len(translucentLiquidFaces) == 0 {
		return nil
	}
	if dc.renderer.worldIndexBuffer == nil {
		return nil
	}

	renderPass.SetPipeline(dc.renderer.resources.WorldTranslucentTurbulentPipeline)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	var materialBindState gogpuWorldMaterialBindState
	materialBindState.invalidate()

	translucentAlpha := liquidAlpha.water
	for _, face := range translucentLiquidFaces {
		textureBindGroup := dc.renderer.resources.WhiteTextureBindGroup
		if dc.renderer.worldTextures != nil && dc.renderer.worldTextures.bindGroup != nil {
			textureBindGroup = dc.renderer.worldTextures.bindGroup
		}
		lightmapBindGroup, litWater := gogpuWorldLightmapArrayBindGroupForFace(face, dc.renderer.worldLightmapArray, dc.renderer.resources.WhiteLightmapBindGroup, worldHasLitWater)
		fullbrightBindGroup := dc.renderer.resources.TransparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = dc.renderer.resources.WhiteTextureBindGroup
		}
		if dc.renderer.worldFullbrightTextures != nil && dc.renderer.worldFullbrightTextures.bindGroup != nil {
			fullbrightBindGroup = dc.renderer.worldFullbrightTextures.bindGroup
		}

		if writeWorldUniform != nil {
			if !writeWorldUniform(translucentAlpha, litWater) {
				return fmt.Errorf("failed to write translucent liquid uniform")
			}
		}
		renderPass.SetBindGroup(0, dc.renderer.resources.UniformBindGroup, []uint32{0})

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

	return nil
}


