//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// ---- merged from world_brush_gogpu_root.go ----
type gogpuOpaqueBrushEntityDraw struct {
	alpha     float32
	frame     int
	vertices  []WorldVertex
	indices   []uint32
	faces     []WorldFace
	centers   [][3]float32
	lightmaps []*gpuWorldTexture
}

func shouldDrawGoGPUOpaqueBrushFace(face WorldFace, entityAlpha float32) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUOpaqueWorldFace(face)
}

func shouldDrawGoGPUAlphaTestBrushFace(face WorldFace, entityAlpha float32) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUAlphaTestWorldFace(face)
}

func shouldDrawGoGPUSkyBrushFace(face WorldFace, entityAlpha float32) bool {
	return clamp01(entityAlpha) > 0 && shouldDrawGoGPUSkyWorldFace(face)
}

func shouldDrawGoGPUOpaqueLiquidBrushFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha)
}

func shouldDrawGoGPUTranslucentLiquidBrushFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha)
}

func shouldDrawGoGPUTranslucentBrushEntityFace(face WorldFace, entityAlpha float32, liquidAlpha worldLiquidAlphaSettings) bool {
	if !(clamp01(entityAlpha) > 0 && clamp01(entityAlpha) < 1) {
		return false
	}
	if face.Flags&model.SurfDrawSky != 0 {
		return false
	}
	pass := worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)*clamp01(entityAlpha))
	return pass == worldPassAlphaTest || pass == worldPassTranslucent
}

func buildGoGPUBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, includeFace func(WorldFace, float32) bool) *gogpuOpaqueBrushEntityDraw {
	draw := worldgogpu.BuildBrushEntityDraw(gogpuBrushEntityParams(entity), geom, includeFace)
	if draw == nil {
		return nil
	}
	return &gogpuOpaqueBrushEntityDraw{
		alpha:    draw.Alpha,
		frame:    draw.Frame,
		vertices: draw.Vertices,
		indices:  draw.Indices,
		faces:    draw.Faces,
		centers:  draw.Centers,
	}
}

func gogpuBrushEntityParams(entity BrushEntity) worldgogpu.BrushEntityParams {
	return worldgogpu.BrushEntityParams{
		Alpha:  entity.Alpha,
		Frame:  entity.Frame,
		Origin: entity.Origin,
		Angles: entity.Angles,
		Scale:  entity.Scale,
	}
}

func buildGoGPUOpaqueBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUOpaqueBrushFace)
}

func buildGoGPUAlphaTestBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUAlphaTestBrushFace)
}

func buildGoGPUSkyBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUSkyBrushFace)
}

func buildGoGPUOpaqueLiquidBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, func(face WorldFace, entityAlpha float32) bool {
		return shouldDrawGoGPUOpaqueLiquidBrushFace(face, entityAlpha, liquidAlpha)
	})
}

type gogpuTranslucentLiquidBrushEntityDraw struct {
	frame     int
	vertices  []WorldVertex
	indices   []uint32
	faces     []gogpuTranslucentLiquidFaceDraw
	lightmaps []*gpuWorldTexture
}

func convertGoGPUTranslucentFaceDraws(src []worldgogpu.TranslucentFaceDraw) []gogpuTranslucentLiquidFaceDraw {
	dst := make([]gogpuTranslucentLiquidFaceDraw, 0, len(src))
	for _, face := range src {
		dst = append(dst, gogpuTranslucentLiquidFaceDraw{
			face:       face.Face,
			alpha:      face.Alpha,
			center:     face.Center,
			distanceSq: face.DistanceSq,
		})
	}
	return dst
}

func buildGoGPUTranslucentLiquidBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings, camera CameraState) *gogpuTranslucentLiquidBrushEntityDraw {
	draw := worldgogpu.BuildTranslucentLiquidBrushEntityDraw(gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) (float32, bool) {
		if !shouldDrawGoGPUTranslucentLiquidBrushFace(face, entityAlpha, liquidAlpha) {
			return 0, false
		}
		return worldFaceAlpha(face.Flags, liquidAlpha), true
	}, func(center [3]float32) float32 {
		return worldFaceDistanceSq(center, camera)
	})
	if draw == nil {
		return nil
	}
	return &gogpuTranslucentLiquidBrushEntityDraw{
		frame:    draw.Frame,
		vertices: draw.Vertices,
		indices:  draw.Indices,
		faces:    convertGoGPUTranslucentFaceDraws(draw.Faces),
	}
}

type gogpuTranslucentBrushEntityDraw struct {
	frame            int
	vertices         []WorldVertex
	indices          []uint32
	alphaTestFaces   []WorldFace
	alphaTestCenters [][3]float32
	translucentFaces []gogpuTranslucentLiquidFaceDraw
	liquidFaces      []gogpuTranslucentLiquidFaceDraw
	lightmaps        []*gpuWorldTexture
}

func buildGoGPUTranslucentBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings, camera CameraState) *gogpuTranslucentBrushEntityDraw {
	draw := worldgogpu.BuildTranslucentBrushEntityDraw(gogpuBrushEntityParams(entity), geom, func(face WorldFace, entityAlpha float32) (worldgogpu.TranslucentFacePlan, bool) {
		if !shouldDrawGoGPUTranslucentBrushEntityFace(face, entityAlpha, liquidAlpha) {
			return worldgogpu.TranslucentFacePlan{}, false
		}
		faceAlpha := worldFaceAlpha(face.Flags, liquidAlpha) * entityAlpha
		switch worldFacePass(face.Flags, faceAlpha) {
		case worldPassAlphaTest:
			return worldgogpu.TranslucentFacePlan{
				Pass:  worldgogpu.TranslucentFacePassAlphaTest,
				Alpha: faceAlpha,
			}, true
		case worldPassTranslucent:
			return worldgogpu.TranslucentFacePlan{
				Pass:   worldgogpu.TranslucentFacePassTranslucent,
				Alpha:  faceAlpha,
				Liquid: worldFaceIsLiquid(face.Flags),
			}, true
		default:
			return worldgogpu.TranslucentFacePlan{}, false
		}
	}, func(center [3]float32) float32 {
		return worldFaceDistanceSq(center, camera)
	})
	if draw == nil {
		return nil
	}
	return &gogpuTranslucentBrushEntityDraw{
		frame:            draw.Frame,
		vertices:         draw.Vertices,
		indices:          draw.Indices,
		alphaTestFaces:   draw.AlphaTestFaces,
		alphaTestCenters: draw.AlphaTestCenters,
		translucentFaces: convertGoGPUTranslucentFaceDraws(draw.TranslucentFaces),
		liquidFaces:      convertGoGPUTranslucentFaceDraws(draw.LiquidFaces),
	}
}

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

	draws := make([]gogpuOpaqueBrushEntityDraw, 0, len(entities)*2)
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUOpaqueBrushEntityDraw(entity, geom); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
		if draw := buildGoGPUAlphaTestBrushEntityDraw(entity, geom); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return
	}

	r := dc.renderer
	r.mu.RLock()
	pipeline := r.worldPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	transparentBindGroup := r.transparentBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
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
	r.mu.RUnlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
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
	buffers := make([]*wgpu.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Entity Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to upload brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Entity Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to create brush index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Release()
			vertexBuffer.Release()
			slog.Warn("failed to upload brush index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		renderPass.SetVertexBuffer(0, vertexBuffer, 0)
		renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
		for faceIndex, face := range draw.faces {
			dynamicLight := [3]float32{}
			if faceIndex < len(draw.centers) {
				dynamicLight = quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.centers[faceIndex]))
			}
			if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.alpha, dynamicLight, 0)); err != nil {
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
			renderPass.SetBindGroup(1, textureBindGroup, nil)
			renderPass.SetBindGroup(2, lightmapBindGroup, nil)
			renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		}
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderOpaqueBrushEntitiesHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush entity encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Release()
		}
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush entity commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Release()
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
		if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, skyFogDensity, camera.Time, 1, [3]float32{}, 0)); err != nil {
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
	var treeEntities []byte
	var tree *bsp.Tree
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		tree = r.worldData.Geometry.Tree
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if tree == nil {
		return
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(treeEntities), tree)

	draws := make([]gogpuOpaqueBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUOpaqueLiquidBrushEntityDraw(entity, geom, liquidAlpha); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return
	}

	r.mu.RLock()
	pipeline := r.worldTurbulentPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	transparentBindGroup := r.transparentBindGroup
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
	r.mu.RUnlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
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
	var activeDynamicLights []DynamicLight
	r.mu.RLock()
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	r.mu.RUnlock()
	buffers := make([]*wgpu.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		hasLitWater := gogpuFacesHaveLitWater(draw.faces)
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to upload brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Liquid Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Release()
			slog.Warn("failed to create brush liquid index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Release()
			vertexBuffer.Release()
			slog.Warn("failed to upload brush liquid index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		renderPass.SetVertexBuffer(0, vertexBuffer, 0)
		renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
		for faceIndex, face := range draw.faces {
			textureBindGroup := whiteTextureBindGroup
			if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
				textureBindGroup = worldTexture.bindGroup
			}
			dynamicLight := [3]float32{}
			if faceIndex < len(draw.centers) {
				dynamicLight = quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.centers[faceIndex]))
			}
			lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(face, draw.lightmaps, whiteLightmapBindGroup, hasLitWater)
			if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, 1, dynamicLight, litWater)); err != nil {
				slog.Warn("failed to update brush liquid uniform buffer", "error", err)
				continue
			}
			fullbrightBindGroup := transparentBindGroup
			if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
				fullbrightBindGroup = worldTexture.bindGroup
			}
			renderPass.SetBindGroup(1, textureBindGroup, nil)
			renderPass.SetBindGroup(2, lightmapBindGroup, nil)
			renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		}
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderOpaqueLiquidBrushEntitiesHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish brush liquid encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Release()
		}
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit brush liquid commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Release()
	}
}

// ---- merged from world_alias_gogpu_root.go ----
const (
	aliasUniformBufferSize      = 80
	aliasSceneUniformBufferSize = 96
	aliasUniformAlign           = 256 // minUniformBufferOffsetAlignment
	aliasInitialDrawCapacity    = 64  // initial capacity for batched draws
)

func (r *Renderer) ensureAliasResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.aliasPipeline != nil && r.aliasShadowPipeline != nil && r.aliasUniformBuffer != nil && r.aliasUniformBindGroup != nil && r.aliasSampler != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, worldgogpu.AliasVertexShaderWGSL, "Alias Vertex Shader")
	if err != nil {
		return fmt.Errorf("create alias vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, worldgogpu.AliasFragmentShaderWGSL, "Alias Fragment Shader")
	if err != nil {
		vertexShader.Release()
		return fmt.Errorf("create alias fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Alias Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: true,
				MinBindingSize:   aliasSceneUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Alias Texture BGL",
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
			{
				Binding:    2,
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
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias texture layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Alias Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout},
	})
	if err != nil {
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             uint64(aliasInitialDrawCapacity) * aliasUniformAlign,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform buffer: %w", err)
	}

	uniformBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Alias Uniform BG",
		Layout:  uniformLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: aliasSceneUniformBufferSize}},
	})
	if err != nil {
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias uniform bind group: %w", err)
	}

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "Alias Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
	if err != nil {
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias sampler: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := createAliasRenderPipeline(device, vertexShader, fragmentShader, pipelineLayout, surfaceFormat, "Alias Render Pipeline", true)
	if err != nil {
		sampler.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias pipeline: %w", err)
	}
	shadowPipeline, err := createAliasRenderPipeline(device, vertexShader, fragmentShader, pipelineLayout, surfaceFormat, "Alias Shadow Render Pipeline", false)
	if err != nil {
		pipeline.Release()
		sampler.Release()
		uniformBindGroup.Release()
		uniformBuffer.Release()
		pipelineLayout.Release()
		uniformLayout.Release()
		textureLayout.Release()
		vertexShader.Release()
		fragmentShader.Release()
		return fmt.Errorf("create alias shadow pipeline: %w", err)
	}

	r.aliasVertexShader = vertexShader
	r.aliasFragmentShader = fragmentShader
	r.aliasUniformBindGroupLayout = uniformLayout
	r.aliasTextureBindGroupLayout = textureLayout
	r.aliasPipelineLayout = pipelineLayout
	r.aliasUniformBuffer = uniformBuffer
	r.aliasUniformBindGroup = uniformBindGroup
	r.aliasSampler = sampler
	r.aliasPipeline = pipeline
	r.aliasShadowPipeline = shadowPipeline
	return nil
}

func createAliasRenderPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout, surfaceFormat gputypes.TextureFormat, label string, depthWrite bool) (*wgpu.RenderPipeline, error) {
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  label,
		Layout: layout,
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
		DepthStencil: gogpuNonDecalDepthStencilState(depthWrite),
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
}

func (r *Renderer) ensureAliasDepthTextureLocked(device *wgpu.Device) {
	if device == nil {
		return
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return
	}
	// Recreate if nil or dimensions changed (e.g. window resize).
	if r.worldDepthTextureView != nil && r.worldDepthWidth == width && r.worldDepthHeight == height {
		return
	}
	if r.worldDepthTextureView != nil {
		slog.Debug("recreating world depth texture for new dimensions",
			"old", fmt.Sprintf("%dx%d", r.worldDepthWidth, r.worldDepthHeight),
			"new", fmt.Sprintf("%dx%d", width, height))
	}
	// Release old resources.
	if r.worldDepthTextureView != nil {
		r.worldDepthTextureView.Release()
	}
	if r.worldDepthTexture != nil {
		r.worldDepthTexture.Release()
	}
	depthTexture, depthView, err := r.createWorldDepthTexture(device, width, height)
	if err != nil {
		slog.Warn("failed to create alias depth texture", "error", err)
		r.worldDepthTexture = nil
		r.worldDepthTextureView = nil
		r.worldDepthWidth = 0
		r.worldDepthHeight = 0
		return
	}
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthView
	r.worldDepthWidth = width
	r.worldDepthHeight = height
}

func (r *Renderer) ensureAliasModelLocked(device *wgpu.Device, queue *wgpu.Queue, modelID string, mdl *model.Model) *gpuAliasModel {
	if modelID == "" || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	if cached, ok := r.aliasModels[modelID]; ok {
		return cached
	}
	if r.aliasTextureBindGroupLayout == nil || r.aliasSampler == nil {
		return nil
	}

	hdr := mdl.AliasHeader
	if len(hdr.STVerts) != hdr.NumVerts || len(hdr.Triangles) != hdr.NumTris || len(hdr.Poses) == 0 {
		return nil
	}

	skins := make([]gpuAliasSkin, 0, len(hdr.Skins))
	for _, skinPixels := range hdr.Skins {
		skin, err := r.createAliasSkinLocked(device, queue, hdr.SkinWidth, hdr.SkinHeight, skinPixels)
		if err != nil {
			slog.Warn("failed to upload alias skin", "model", modelID, "error", err)
			continue
		}
		skins = append(skins, skin)
	}
	if len(skins) == 0 {
		fallback, err := r.createAliasSkinLocked(device, queue, 1, 1, []byte{0})
		if err != nil {
			return nil
		}
		skins = append(skins, fallback)
	}

	refs := make([]aliasimpl.MeshRef, 0, len(hdr.Triangles)*3)
	for _, tri := range hdr.Triangles {
		for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
			idx := int(tri.VertIndex[vertexIndex])
			if idx < 0 || idx >= len(hdr.STVerts) {
				continue
			}
			st := hdr.STVerts[idx]
			s := float32(st.S) + 0.5
			if tri.FacesFront == 0 && st.OnSeam != 0 {
				s += float32(hdr.SkinWidth) * 0.5
			}
			refs = append(refs, aliasimpl.MeshRef{
				VertexIndex: idx,
				TexCoord: [2]float32{
					s / float32(hdr.SkinWidth),
					(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
				},
			})
		}
	}

	alias := &gpuAliasModel{
		modelID:     modelID,
		flags:       hdr.Flags,
		skins:       skins,
		playerSkins: make(map[uint32][]gpuAliasSkin),
		poses:       hdr.Poses,
		refs:        refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func (r *Renderer) createAliasSkinLocked(device *wgpu.Device, queue *wgpu.Queue, width, height int, pixels []byte) (gpuAliasSkin, error) {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(pixels, r.palette, 0, false)
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Alias Skin Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuAliasSkin{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, baseRGBA, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "Alias Skin View",
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
		return gpuAliasSkin{}, fmt.Errorf("create texture view: %w", err)
	}
	fullbrightTexture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "Alias Fullbright Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  fullbrightTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, fullbrightRGBA, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("write fullbright texture: %w", err)
	}
	fullbrightView, err := device.CreateTextureView(fullbrightTexture, &wgpu.TextureViewDescriptor{
		Label:           "Alias Fullbright View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Alias Skin BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: r.aliasSampler},
			{Binding: 1, TextureView: view},
			{Binding: 2, TextureView: fullbrightView},
		},
	})
	if err != nil {
		fullbrightView.Release()
		fullbrightTexture.Release()
		view.Release()
		texture.Release()
		return gpuAliasSkin{}, fmt.Errorf("create bind group: %w", err)
	}
	return gpuAliasSkin{
		texture:           texture,
		view:              view,
		fullbrightTexture: fullbrightTexture,
		fullbrightView:    fullbrightView,
		bindGroup:         bindGroup,
	}, nil
}

func (r *Renderer) resolveAliasSkinLocked(device *wgpu.Device, queue *wgpu.Queue, alias *gpuAliasModel, entity AliasModelEntity, slot int) *gpuAliasSkin {
	if alias == nil || slot < 0 {
		return nil
	}
	if entity.IsPlayer {
		if skins, ok := alias.playerSkins[entity.ColorMap]; ok && slot < len(skins) {
			return &skins[slot]
		}
		hdr := entity.Model.AliasHeader
		playerSkins := make([]gpuAliasSkin, len(hdr.Skins))
		for i, skinPixels := range hdr.Skins {
			topColor, bottomColor := splitPlayerColors(byte(entity.ColorMap))
			translated := TranslatePlayerSkinPixels(skinPixels, topColor, bottomColor)
			skin, err := r.createAliasSkinLocked(device, queue, hdr.SkinWidth, hdr.SkinHeight, translated)
			if err != nil {
				slog.Warn("failed to upload translated alias skin", "model", entity.ModelID, "colormap", entity.ColorMap, "error", err)
				return nil
			}
			playerSkins[i] = skin
		}
		alias.playerSkins[entity.ColorMap] = playerSkins
		if slot < len(playerSkins) {
			return &playerSkins[slot]
		}
	}
	if slot < len(alias.skins) {
		return &alias.skins[slot]
	}
	return nil
}

func (r *Renderer) buildAliasDrawLocked(device *wgpu.Device, queue *wgpu.Queue, entity AliasModelEntity, fullAngles bool) *gpuAliasDraw {
	alias := r.ensureAliasModelLocked(device, queue, entity.ModelID, entity.Model)
	if alias == nil || entity.Model == nil || entity.Model.AliasHeader == nil || len(alias.refs) == 0 {
		return nil
	}

	hdr := entity.Model.AliasHeader
	frame := entity.Frame
	if frame < 0 || frame >= len(hdr.Frames) {
		frame = 0
	}
	state := r.ensureAliasStateLocked(entity)
	state.Frame = frame
	aliasHdr := aliasHeaderFromModel(hdr)
	aliasHdr.Flags = applyAliasNoLerpListFlags(aliasHdr.Flags, entity.ModelID)
	interpData, err := SetupAliasFrame(state, aliasHdr, entity.TimeSeconds, true, false, 1)
	if err != nil {
		return nil
	}
	interpData.Origin, interpData.Angles = SetupEntityTransform(
		state,
		entity.TimeSeconds,
		true,
		entity.EntityKey == AliasViewModelEntityKey,
		false,
		false,
		1,
	)
	pose1 := interpData.Pose1
	pose2 := interpData.Pose2
	if pose1 < 0 || pose1 >= len(alias.poses) {
		pose1 = 0
	}
	if pose2 < 0 || pose2 >= len(alias.poses) {
		pose2 = 0
	}

	var skin *gpuAliasSkin
	if len(alias.skins) > 0 {
		slot := resolveAliasSkinSlot(entity.Model.AliasHeader, entity.SkinNum, entity.TimeSeconds, len(alias.skins))
		skin = r.resolveAliasSkinLocked(device, queue, alias, entity, slot)
	}
	if skin == nil && len(alias.skins) > 0 {
		skin = &alias.skins[0]
	}

	alpha, visible := visibleEntityAlpha(entity.Alpha)
	if !visible {
		return nil
	}

	return &gpuAliasDraw{
		alias:  alias,
		model:  entity.Model,
		pose1:  pose1,
		pose2:  pose2,
		blend:  interpData.Blend,
		skin:   skin,
		origin: interpData.Origin,
		angles: interpData.Angles,
		alpha:  alpha,
		scale:  entity.Scale,
		full:   fullAngles,
	}
}

func (dc *DrawContext) renderAliasEntitiesHAL(entities []AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	draws := dc.collectAliasDraws(entities, false)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, false, fogColor, fogDensity)
}

func (dc *DrawContext) renderViewModelHAL(entity AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil {
		return
	}
	draws := dc.collectAliasDraws([]AliasModelEntity{entity}, true)
	if len(draws) == 0 {
		return
	}
	dc.renderAliasDrawsHAL(draws, true, fogColor, fogDensity)
}

func (dc *DrawContext) collectAliasDraws(entities []AliasModelEntity, fullAngles bool) []gpuAliasDraw {
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return nil
	}

	r := dc.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if !fullAngles {
		r.pruneAliasStatesLocked(entities)
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		slog.Warn("failed to initialize alias resources", "error", err)
		return nil
	}
	r.ensureAliasDepthTextureLocked(device)
	draws := make([]gpuAliasDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildAliasDrawLocked(device, queue, entity, fullAngles); draw != nil {
			draws = append(draws, *draw)
		}
	}
	return draws
}

func (dc *DrawContext) renderAliasDrawsHAL(draws []gpuAliasDraw, useViewModelDepthRange bool, fogColor [3]float32, fogDensity float32) {
	if len(draws) == 0 {
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

	// Pre-build all vertex data and compute total scratch buffer size.
	type preparedDraw struct {
		vertices []WorldVertex
		skin     *gpuAliasSkin
		alpha    float32
	}
	prepared := make([]preparedDraw, 0, len(draws))
	totalVertexBytes := uint64(0)
	for _, draw := range draws {
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertices) == 0 || draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}
		prepared = append(prepared, preparedDraw{vertices: vertices, skin: draw.skin, alpha: draw.alpha})
		totalVertexBytes += uint64(len(vertices) * 44)
	}
	if len(prepared) == 0 {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer", "error", err)
		return
	}
	if err := r.ensureAliasUniformBufferLocked(device, len(prepared)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	// Pre-upload all uniform data at 256-byte aligned offsets
	// and all vertex data at consecutive offsets.
	vertexOffsets := make([]uint64, len(prepared))
	vertexCounts := make([]uint32, len(prepared))
	uniformOffsets := make([]uint32, len(prepared))
	currentVertexOffset := uint64(0)
	for i, pd := range prepared {
		uniformOffsets[i] = uint32(i) * aliasUniformAlign
		vertexOffsets[i] = currentVertexOffset
		vertexCounts[i] = uint32(len(pd.vertices))

		if err := queue.WriteBuffer(uniformBuffer, uint64(uniformOffsets[i]), aliasSceneUniformBytes(vpMatrix, cameraOrigin, pd.alpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update alias uniform buffer", "error", err, "draw", i)
			return
		}
		if err := queue.WriteBuffer(scratchBuffer, currentVertexOffset, aliasVertexBytes(pd.vertices)); err != nil {
			slog.Warn("failed to upload alias vertices", "error", err, "draw", i)
			return
		}
		currentVertexOffset += uint64(len(pd.vertices) * 44)
	}

	// Record a single render pass with all draws.
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Alias Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Alias Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("failed to begin alias render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		maxDepth := float32(1.0)
		if useViewModelDepthRange {
			maxDepth = 0.3
		}
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, maxDepth)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}

	for i, pd := range prepared {
		renderPass.SetVertexBuffer(0, scratchBuffer, vertexOffsets[i])
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{uniformOffsets[i]})
		renderPass.SetBindGroup(1, pd.skin.bindGroup, nil)
		renderPass.Draw(vertexCounts[i], 1, 0, 0)
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderAliasDrawsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alias encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit alias commands", "error", err)
	}
}

func aliasUniformBytes(vp types.Mat4, alpha float32) []byte {
	data := make([]byte, aliasUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	binary.LittleEndian.PutUint32(data[64:68], math.Float32bits(alpha))
	return data
}

func aliasSceneUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, aliasSceneUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	putFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return data
}

func aliasShadowUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	return aliasSceneUniformBytes(vp, cameraOrigin, alpha, fogColor, fogDensity)
}

func aliasVertexBytes(vertices []WorldVertex) []byte {
	data := make([]byte, len(vertices)*44)
	for i, v := range vertices {
		offset := i * 44
		putFloat32s(data[offset:offset+12], v.Position[:])
		putFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32s(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32s(data[offset+28:offset+40], v.Normal[:])
	}
	return data
}

func buildAliasVerticesInterpolated(alias *gpuAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	return aliasimpl.BuildVerticesInterpolated(
		aliasimpl.MeshFromRefs(alias.poses, alias.refs),
		mdl.AliasHeader,
		pose1Index,
		pose2Index,
		blend,
		origin,
		angles,
		entityScale,
		fullAngles,
	)
}

// ---- merged from world_sprite_gogpu_root.go ----

func (r *Renderer) ensureSpriteResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.spritePipeline != nil && r.spriteUniformBuffer != nil && r.spriteUniformBindGroup != nil {
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

	r.spriteUniformBuffer = uniformBuffer
	r.spriteUniformBindGroup = uniformBindGroup
	r.spriteVertexShader = vertexShader
	r.spriteFragmentShader = fragmentShader
	r.spritePipeline = pipeline
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
	uniformBuffer := r.spriteUniformBuffer
	uniformBindGroup := r.spriteUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
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
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, []uint32{0})

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraAngles := [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
	cameraForward, cameraRight, cameraUp := spriteCameraBasis(cameraAngles)

	for _, draw := range draws {
		if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
			continue
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
		renderPass.SetBindGroup(1, draw.sprite.frames[draw.frame].bindGroup, nil)
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
type gogpuLateTranslucentFaceResources struct {
	device                  *wgpu.Device
	queue                   *wgpu.Queue
	textureView             *wgpu.TextureView
	alphaTestPipeline       *wgpu.RenderPipeline
	translucentPipeline     *wgpu.RenderPipeline
	liquidPipeline          *wgpu.RenderPipeline
	uniformBuffer           *wgpu.Buffer
	uniformBindGroupLayout  *wgpu.BindGroupLayout
	whiteTextureBindGroup   *wgpu.BindGroup
	whiteLightmapBindGroup  *wgpu.BindGroup
	transparentBindGroup    *wgpu.BindGroup
	depthView               *wgpu.TextureView
	camera                  CameraState
	worldTextures           map[int32]*gpuWorldTexture
	worldFullbrightTextures map[int32]*gpuWorldTexture
	worldTextureAnimations  []*SurfaceTexture
	worldLightmapPages      []*gpuWorldTexture
	activeDynamicLights     []DynamicLight
	unlock                  func()
}

func (dc *DrawContext) loadGoGPULateTranslucentFaceResources() (gogpuLateTranslucentFaceResources, bool) {
	if dc == nil || dc.renderer == nil {
		return gogpuLateTranslucentFaceResources{}, false
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return gogpuLateTranslucentFaceResources{}, false
	}
	r := dc.renderer
	r.mu.RLock()
	res := gogpuLateTranslucentFaceResources{
		device:                  device,
		queue:                   queue,
		textureView:             textureView,
		alphaTestPipeline:       r.worldPipeline,
		translucentPipeline:     r.worldTranslucentPipeline,
		liquidPipeline:          r.worldTranslucentTurbulentPipeline,
		uniformBuffer:           r.uniformBuffer,
		uniformBindGroupLayout:  r.uniformBindGroupLayout,
		whiteTextureBindGroup:   r.whiteTextureBindGroup,
		whiteLightmapBindGroup:  r.whiteLightmapBindGroup,
		transparentBindGroup:    r.transparentBindGroup,
		depthView:               r.worldDepthTextureView,
		camera:                  r.cameraState,
		worldTextures:           make(map[int32]*gpuWorldTexture, len(r.worldTextures)),
		worldFullbrightTextures: make(map[int32]*gpuWorldTexture, len(r.worldFullbrightTextures)),
		worldTextureAnimations:  append([]*SurfaceTexture(nil), r.worldTextureAnimations...),
		worldLightmapPages:      append([]*gpuWorldTexture(nil), r.worldLightmapPages...),
		unlock:                  r.mu.RUnlock,
	}
	for k, v := range r.worldTextures {
		res.worldTextures[k] = v
	}
	for k, v := range r.worldFullbrightTextures {
		res.worldFullbrightTextures[k] = v
	}
	if r.lightPool != nil {
		res.activeDynamicLights = append(res.activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	if res.translucentPipeline == nil || res.liquidPipeline == nil || res.uniformBuffer == nil || res.uniformBindGroupLayout == nil || res.whiteTextureBindGroup == nil || res.whiteLightmapBindGroup == nil {
		res.unlock()
		return gogpuLateTranslucentFaceResources{}, false
	}
	if res.transparentBindGroup == nil {
		res.transparentBindGroup = res.whiteTextureBindGroup
	}
	return res, true
}

func createGoGPULateTranslucentUniformBindGroup(res gogpuLateTranslucentFaceResources) (*wgpu.BindGroup, error) {
	return res.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "GoGPU Late Translucent Uniform BG",
		Layout: res.uniformBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: res.uniformBuffer, Offset: 0, Size: worldUniformBufferSize},
		},
	})
}

type gogpuTranslucentBrushCollectState struct {
	device      *wgpu.Device
	queue       *wgpu.Queue
	camera      CameraState
	liquidAlpha worldLiquidAlphaSettings
}

func (dc *DrawContext) loadGoGPUTranslucentBrushCollectState() (gogpuTranslucentBrushCollectState, bool) {
	if dc == nil || dc.renderer == nil {
		return gogpuTranslucentBrushCollectState{}, false
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return gogpuTranslucentBrushCollectState{}, false
	}

	r := dc.renderer
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.worldData == nil || r.worldData.Geometry == nil || r.worldData.Geometry.Tree == nil {
		return gogpuTranslucentBrushCollectState{}, false
	}

	return gogpuTranslucentBrushCollectState{
		device:      device,
		queue:       queue,
		camera:      r.cameraState,
		liquidAlpha: worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(r.worldData.Geometry.Tree.Entities), r.worldData.Geometry.Tree),
	}, true
}

func createGoGPUTranslucentBrushBuffers(device *wgpu.Device, queue *wgpu.Queue, vertexLabel, indexLabel string, vertices []WorldVertex, indices []uint32) ([2]*wgpu.Buffer, []*wgpu.Buffer, bool) {
	if device == nil || queue == nil || len(vertices) == 0 || len(indices) == 0 {
		return [2]*wgpu.Buffer{}, nil, false
	}
	vertexData := worldgogpu.VertexBytes(vertices)
	indexData := worldgogpu.IndexBytes(indices)

	vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, vertexLabel, gputypes.BufferUsageVertex, vertexData)
	if err != nil {
		slog.Warn("failed to create translucent brush vertex buffer", "label", vertexLabel, "error", err)
		return [2]*wgpu.Buffer{}, nil, false
	}
	if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
		vertexBuffer.Release()
		slog.Warn("failed to upload translucent brush vertex buffer", "label", vertexLabel, "error", err)
		return [2]*wgpu.Buffer{}, nil, false
	}

	indexBuffer, err := worldgogpu.CreateBrushBuffer(device, indexLabel, gputypes.BufferUsageIndex, indexData)
	if err != nil {
		vertexBuffer.Release()
		slog.Warn("failed to create translucent brush index buffer", "label", indexLabel, "error", err)
		return [2]*wgpu.Buffer{}, nil, false
	}
	if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
		indexBuffer.Release()
		vertexBuffer.Release()
		slog.Warn("failed to upload translucent brush index buffer", "label", indexLabel, "error", err)
		return [2]*wgpu.Buffer{}, nil, false
	}

	return [2]*wgpu.Buffer{vertexBuffer, indexBuffer}, []*wgpu.Buffer{vertexBuffer, indexBuffer}, true
}

func appendGoGPUTranslucentLiquidBrushFaceRenders(dst []gogpuTranslucentBrushFaceRender, bufferPair [2]*wgpu.Buffer, draw gogpuTranslucentLiquidBrushEntityDraw) []gogpuTranslucentBrushFaceRender {
	hasLitWater := gogpuTranslucentFacesHaveLitWater(draw.faces)
	for _, face := range draw.faces {
		dst = append(dst, gogpuTranslucentBrushFaceRender{
			bufferPair:  bufferPair,
			frame:       draw.frame,
			face:        face,
			liquid:      true,
			hasLitWater: hasLitWater,
			lightmaps:   draw.lightmaps,
		})
	}
	return dst
}

func gogpuTranslucentFacesHaveLitWater(faces []gogpuTranslucentLiquidFaceDraw) bool {
	for _, face := range faces {
		if face.face.Flags&model.SurfDrawTurb != 0 && face.face.Flags&model.SurfDrawSky == 0 && face.face.LightmapIndex >= 0 {
			return true
		}
	}
	return false
}

func appendGoGPUTranslucentBrushEntityFaceRenders(alphaTestDst, translucentDst []gogpuTranslucentBrushFaceRender, bufferPair [2]*wgpu.Buffer, draw gogpuTranslucentBrushEntityDraw) ([]gogpuTranslucentBrushFaceRender, []gogpuTranslucentBrushFaceRender) {
	hasLitWater := gogpuTranslucentFacesHaveLitWater(draw.liquidFaces)
	for faceIndex, face := range draw.alphaTestFaces {
		center := [3]float32{}
		if faceIndex < len(draw.alphaTestCenters) {
			center = draw.alphaTestCenters[faceIndex]
		}
		alphaTestDst = append(alphaTestDst, gogpuTranslucentBrushFaceRender{
			bufferPair: bufferPair,
			frame:      draw.frame,
			face: gogpuTranslucentLiquidFaceDraw{
				face:  face,
				alpha: 1,
			},
			center:    center,
			lightmaps: draw.lightmaps,
		})
	}
	for _, face := range draw.translucentFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair: bufferPair,
			frame:      draw.frame,
			face:       face,
			lightmaps:  draw.lightmaps,
		})
	}
	for _, face := range draw.liquidFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair:  bufferPair,
			frame:       draw.frame,
			face:        face,
			liquid:      true,
			hasLitWater: hasLitWater,
			lightmaps:   draw.lightmaps,
		})
	}
	return alphaTestDst, translucentDst
}

func gogpuLateTranslucentTextureBindGroups(res gogpuLateTranslucentFaceResources, draw gogpuTranslucentBrushFaceRender, timeSeconds float64) (*wgpu.BindGroup, *wgpu.BindGroup) {
	textureBindGroup := res.whiteTextureBindGroup
	if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
		textureBindGroup = worldTexture.bindGroup
	}

	fullbrightBindGroup := res.transparentBindGroup
	if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldFullbrightTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
		fullbrightBindGroup = worldTexture.bindGroup
	}

	return textureBindGroup, fullbrightBindGroup
}

func gogpuLateTranslucentLightmapBindGroup(res gogpuLateTranslucentFaceResources, draw gogpuTranslucentBrushFaceRender) (*wgpu.BindGroup, float32) {
	if draw.liquid {
		lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(draw.face.face, draw.lightmaps, res.whiteLightmapBindGroup, draw.hasLitWater)
		if lightmapBindGroup == res.whiteLightmapBindGroup {
			lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(draw.face.face, res.worldLightmapPages, res.whiteLightmapBindGroup, draw.hasLitWater)
		}
		return lightmapBindGroup, litWater
	}

	lightmapBindGroup := res.whiteLightmapBindGroup
	if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(draw.lightmaps) {
		if lightmapPage := draw.lightmaps[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
			lightmapBindGroup = lightmapPage.bindGroup
		}
	} else if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(res.worldLightmapPages) {
		if lightmapPage := res.worldLightmapPages[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
			lightmapBindGroup = lightmapPage.bindGroup
		}
	}
	return lightmapBindGroup, 0
}

func (dc *DrawContext) collectGoGPUWorldTranslucentLiquidFaceRenders() []gogpuTranslucentBrushFaceRender {
	if dc == nil || dc.renderer == nil {
		return nil
	}
	r := dc.renderer
	r.mu.RLock()
	worldData := r.worldData
	camera := r.cameraState
	worldVertexBuffer := r.worldVertexBuffer
	worldIndexBuffer := r.worldIndexBuffer
	worldLightmapPages := append([]*gpuWorldTexture(nil), r.worldLightmapPages...)
	r.mu.RUnlock()
	if worldData == nil || worldData.Geometry == nil || worldVertexBuffer == nil || worldIndexBuffer == nil {
		return nil
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(worldData.Geometry.Tree.Entities), worldData.Geometry.Tree)
	visibleFaces := r.worldVisibleFacesScratch.selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
	)
	renders := make([]gogpuTranslucentBrushFaceRender, 0, 8)
	worldHasLitWater := gogpuFacesHaveLitWater(worldData.Geometry.Faces)
	for _, face := range visibleFaces {
		if !shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha) {
			continue
		}
		renders = append(renders, gogpuTranslucentBrushFaceRender{
			bufferPair: [2]*wgpu.Buffer{worldVertexBuffer, worldIndexBuffer},
			face: gogpuTranslucentLiquidFaceDraw{
				face:       face,
				alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
				center:     face.Center,
				distanceSq: worldFaceDistanceSq(face.Center, camera),
			},
			liquid:      true,
			hasLitWater: worldHasLitWater,
			lightmaps:   worldLightmapPages,
		})
	}
	return renders
}

func (dc *DrawContext) collectGoGPUTranslucentLiquidBrushFaceRenders(entities []BrushEntity) ([]gogpuTranslucentBrushFaceRender, []*wgpu.Buffer) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return nil, nil
	}
	state, ok := dc.loadGoGPUTranslucentBrushCollectState()
	if !ok {
		return nil, nil
	}
	draws := make([]gogpuTranslucentLiquidBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentLiquidBrushEntityDraw(entity, geom, state.liquidAlpha, state.camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil
	}

	renders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	buffers := make([]*wgpu.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		bufferPair, owned, ok := createGoGPUTranslucentBrushBuffers(state.device, state.queue, "Brush Translucent Liquid Vertices", "Brush Translucent Liquid Indices", draw.vertices, draw.indices)
		if !ok {
			continue
		}
		buffers = append(buffers, owned...)
		renders = appendGoGPUTranslucentLiquidBrushFaceRenders(renders, bufferPair, draw)
	}
	return renders, buffers
}

func (dc *DrawContext) collectGoGPUTranslucentBrushEntityFaceRenders(entities []BrushEntity) ([]gogpuTranslucentBrushFaceRender, []gogpuTranslucentBrushFaceRender, []*wgpu.Buffer) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return nil, nil, nil
	}
	state, ok := dc.loadGoGPUTranslucentBrushCollectState()
	if !ok {
		return nil, nil, nil
	}
	draws := make([]gogpuTranslucentBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentBrushEntityDraw(entity, geom, state.liquidAlpha, state.camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil, nil
	}

	alphaTestRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws))
	translucentRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	buffers := make([]*wgpu.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		bufferPair, owned, ok := createGoGPUTranslucentBrushBuffers(state.device, state.queue, "Brush Translucent Vertices", "Brush Translucent Indices", draw.vertices, draw.indices)
		if !ok {
			continue
		}
		buffers = append(buffers, owned...)
		alphaTestRenders, translucentRenders = appendGoGPUTranslucentBrushEntityFaceRenders(alphaTestRenders, translucentRenders, bufferPair, draw)
	}
	return alphaTestRenders, translucentRenders, buffers
}

func (dc *DrawContext) renderGoGPUAlphaTestBrushFaceRendersHAL(renders []gogpuTranslucentBrushFaceRender, fogColor [3]float32, fogDensity float32) {
	if len(renders) == 0 {
		return
	}
	res, ok := dc.loadGoGPULateTranslucentFaceResources()
	if !ok || res.alphaTestPipeline == nil {
		return
	}
	defer res.unlock()
	encoder, err := res.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoGPU Alpha-Test Brush Encoder"})
	if err != nil {
		slog.Warn("failed to create alpha-test brush encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoGPU Alpha-Test Brush Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    res.textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(res.depthView),
	})
	if err != nil {
		slog.Warn("renderGoGPUAlphaTestBrushFaceRendersHAL: Failed to begin render pass", "error", err)
		return
	}
	uniformBindGroup, err := createGoGPULateTranslucentUniformBindGroup(res)
	if err != nil {
		slog.Warn("failed to create alpha-test brush uniform bind group", "error", err)
		_ = renderPass.End()
		return
	}
	defer uniformBindGroup.Release()
	renderPass.SetPipeline(res.alphaTestPipeline)
	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	// GoGPU's Vulkan render-pass backend resolves descriptor-set binding through the
	// currently bound pipeline layout, so a known-good world pipeline must be selected
	// before the first SetBindGroup call in this pass.
	renderPass.SetPipeline(res.alphaTestPipeline)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	for _, draw := range renders {
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.center))
		if err := res.queue.WriteBuffer(res.uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, timeValue, draw.face.alpha, dynamicLight, 0)); err != nil {
			slog.Warn("failed to update alpha-test brush uniform buffer", "error", err)
			continue
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup, fullbrightBindGroup := gogpuLateTranslucentTextureBindGroups(res, draw, timeSeconds)
		lightmapBindGroup, _ := gogpuLateTranslucentLightmapBindGroup(res, draw)
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderGoGPUAlphaTestBrushFaceRendersHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alpha-test brush encoding", "error", err)
		return
	}
	if _, err := res.queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit alpha-test brush commands", "error", err)
	}
}

func (dc *DrawContext) renderGoGPUSortedTranslucentFaceRendersHAL(renders []gogpuTranslucentBrushFaceRender, fogColor [3]float32, fogDensity float32) {
	if len(renders) == 0 {
		return
	}
	res, ok := dc.loadGoGPULateTranslucentFaceResources()
	if !ok {
		return
	}
	defer res.unlock()
	encoder, err := res.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoGPU Late Translucent Encoder"})
	if err != nil {
		slog.Warn("failed to create late translucent encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoGPU Late Translucent Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    res.textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(res.depthView),
	})
	if err != nil {
		slog.Warn("renderGoGPUSortedTranslucentFaceRendersHAL: Failed to begin render pass", "error", err)
		return
	}
	uniformBindGroup, err := createGoGPULateTranslucentUniformBindGroup(res)
	if err != nil {
		slog.Warn("failed to create late translucent uniform bind group", "error", err)
		_ = renderPass.End()
		return
	}
	defer uniformBindGroup.Release()
	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	// GoGPU's Vulkan backend resolves descriptor-set binding through the active
	// pipeline layout, so the sorted late-translucent pass must select a pipeline
	// before its first SetBindGroup call.
	renderPass.SetPipeline(res.translucentPipeline)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	for _, draw := range renders {
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.face.center))
		lightmapBindGroup, litWater := gogpuLateTranslucentLightmapBindGroup(res, draw)
		if err := res.queue.WriteBuffer(res.uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, timeValue, draw.face.alpha, dynamicLight, litWater)); err != nil {
			slog.Warn("failed to update late translucent uniform buffer", "error", err)
			continue
		}
		if draw.liquid {
			renderPass.SetPipeline(res.liquidPipeline)
		} else {
			renderPass.SetPipeline(res.translucentPipeline)
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup, fullbrightBindGroup := gogpuLateTranslucentTextureBindGroups(res, draw, timeSeconds)
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("renderGoGPUSortedTranslucentFaceRendersHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish late translucent encoding", "error", err)
		return
	}
	if _, err := res.queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit late translucent commands", "error", err)
	}
}

// ---- merged from world_alias_shadow_gogpu_root.go ----
const (
	aliasShadowSegments = 16
	aliasShadowAlpha    = 0.5
	aliasShadowLift     = 0.1
	aliasShadowMinSize  = 8.0
	aliasShadowMaxSize  = 48.0
)

func (dc *DrawContext) renderAliasShadowsHAL(entities []AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	if cvar.FloatValue(CvarRShadows) <= 0 {
		return
	}

	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}

	excludedModels := parseAliasShadowExclusionsGO(cvar.StringValue(CvarRNoshadowList))
	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias resources for shadows", "error", err)
		return
	}
	if err := r.ensureAliasShadowSkinLocked(device, queue); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias shadow skin", "error", err)
		return
	}
	shadowSkin := r.aliasShadowSkin
	r.mu.Unlock()
	if shadowSkin == nil {
		return
	}

	draws := make([]gpuAliasShadowDraw, 0, len(entities))
	for _, entity := range entities {
		modelID := strings.ToLower(entity.ModelID)
		if _, skip := excludedModels[modelID]; skip {
			continue
		}
		if _, visible := visibleEntityAlpha(entity.Alpha); !visible {
			continue
		}
		vertices := buildAliasShadowVertices(entity)
		if len(vertices) == 0 {
			continue
		}
		draws = append(draws, gpuAliasShadowDraw{vertices: vertices})
	}
	if len(draws) == 0 {
		return
	}

	dc.renderAliasShadowDrawsHAL(draws, shadowSkin, fogColor, fogDensity)
}

type gpuAliasShadowDraw struct {
	vertices []WorldVertex
}

func (dc *DrawContext) renderAliasShadowDrawsHAL(draws []gpuAliasShadowDraw, shadowSkin *gpuAliasSkin, fogColor [3]float32, fogDensity float32) {
	if len(draws) == 0 || shadowSkin == nil || shadowSkin.bindGroup == nil {
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

	// Filter and compute total vertex bytes.
	type preparedShadow struct {
		vertices []WorldVertex
	}
	prepared := make([]preparedShadow, 0, len(draws))
	totalVertexBytes := uint64(0)
	for _, draw := range draws {
		if len(draw.vertices) == 0 {
			continue
		}
		prepared = append(prepared, preparedShadow{vertices: draw.vertices})
		totalVertexBytes += uint64(len(draw.vertices) * 44)
	}
	if len(prepared) == 0 {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer for shadows", "error", err)
		return
	}
	if err := r.ensureAliasUniformBufferLocked(device, len(prepared)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer for shadows", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	shadowPipeline := r.aliasShadowPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}
	if shadowPipeline != nil {
		pipeline = shadowPipeline
	}

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	// Pre-upload all uniform and vertex data.
	// Shadows all share the same uniform data, but we still use dynamic offsets
	// so the bind group layout is consistent.
	shadowUniform := aliasShadowUniformBytes(vpMatrix, cameraOrigin, aliasShadowAlpha, fogColor, fogDensity)
	vertexOffsets := make([]uint64, len(prepared))
	vertexCounts := make([]uint32, len(prepared))
	uniformOffsets := make([]uint32, len(prepared))
	currentVertexOffset := uint64(0)
	for i, pd := range prepared {
		uniformOffsets[i] = uint32(i) * aliasUniformAlign
		vertexOffsets[i] = currentVertexOffset
		vertexCounts[i] = uint32(len(pd.vertices))

		if err := queue.WriteBuffer(uniformBuffer, uint64(uniformOffsets[i]), shadowUniform); err != nil {
			slog.Warn("failed to update alias shadow uniform buffer", "error", err, "draw", i)
			return
		}
		if err := queue.WriteBuffer(scratchBuffer, currentVertexOffset, aliasVertexBytes(pd.vertices)); err != nil {
			slog.Warn("failed to upload alias shadow vertices", "error", err, "draw", i)
			return
		}
		currentVertexOffset += uint64(len(pd.vertices) * 44)
	}

	// Record a single render pass with all shadow draws.
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Alias Shadow Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias shadow encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Alias Shadow Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("failed to begin alias shadow render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}

	for i, pd := range prepared {
		renderPass.SetVertexBuffer(0, scratchBuffer, vertexOffsets[i])
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{uniformOffsets[i]})
		renderPass.SetBindGroup(1, shadowSkin.bindGroup, nil)
		renderPass.Draw(vertexCounts[i], 1, 0, 0)
		_ = pd // used via vertexOffsets/vertexCounts
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderAliasShadowsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alias shadow encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit alias shadow commands", "error", err)
	}
}

func (r *Renderer) ensureAliasShadowSkinLocked(device *wgpu.Device, queue *wgpu.Queue) error {
	if r.aliasShadowSkin != nil {
		return nil
	}
	if r.aliasTextureBindGroupLayout == nil || r.aliasSampler == nil {
		return fmt.Errorf("alias texture bind group layout not ready")
	}
	skin, err := r.createAliasSkinLocked(device, queue, 1, 1, []byte{0})
	if err != nil {
		return err
	}
	r.aliasShadowSkin = &skin
	return nil
}

func buildAliasShadowVertices(entity AliasModelEntity) []WorldVertex {
	if entity.Model == nil || entity.Model.AliasHeader == nil {
		return nil
	}

	modelScale := entity.Scale
	if modelScale == 0 {
		modelScale = 1
	}
	mins := entity.Model.Mins
	maxs := entity.Model.Maxs
	spanX := (maxs[0] - mins[0]) * modelScale
	spanY := (maxs[1] - mins[1]) * modelScale
	shadowRadius := 0.5 * float32(math.Max(float64(spanX), float64(spanY)))
	if shadowRadius < aliasShadowMinSize {
		shadowRadius = aliasShadowMinSize
	}
	if shadowRadius > aliasShadowMaxSize {
		shadowRadius = aliasShadowMaxSize
	}

	shadowZ := entity.Origin[2] + mins[2]*modelScale + aliasShadowLift
	center := WorldVertex{
		Position:      [3]float32{entity.Origin[0], entity.Origin[1], shadowZ},
		TexCoord:      [2]float32{0.5, 0.5},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices := make([]WorldVertex, 0, aliasShadowSegments*3)
	for i := 0; i < aliasShadowSegments; i++ {
		a0 := float32(i) * 2 * float32(math.Pi) / aliasShadowSegments
		a1 := float32(i+1) * 2 * float32(math.Pi) / aliasShadowSegments
		p0 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a0)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a0)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{0, 0},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		p1 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a1)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a1)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{1, 1},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		vertices = append(vertices, center, p0, p1)
	}
	return vertices
}

func parseAliasShadowExclusionsGO(value string) map[string]struct{} {
	return parseAliasModelList(value)
}

// ---- merged from world_cleanup_gogpu_root.go ----
func (r *Renderer) clearAliasModelsLocked() {
	for key, cached := range r.aliasModels {
		for _, skin := range cached.skins {
			if skin.bindGroup != nil {
				skin.bindGroup.Release()
			}
			if skin.fullbrightView != nil {
				skin.fullbrightView.Release()
			}
			if skin.fullbrightTexture != nil {
				skin.fullbrightTexture.Release()
			}
			if skin.view != nil {
				skin.view.Release()
			}
			if skin.texture != nil {
				skin.texture.Release()
			}
		}
		for _, variants := range cached.playerSkins {
			for _, skin := range variants {
				if skin.bindGroup != nil {
					skin.bindGroup.Release()
				}
				if skin.fullbrightView != nil {
					skin.fullbrightView.Release()
				}
				if skin.fullbrightTexture != nil {
					skin.fullbrightTexture.Release()
				}
				if skin.view != nil {
					skin.view.Release()
				}
				if skin.texture != nil {
					skin.texture.Release()
				}
			}
		}
		delete(r.aliasModels, key)
	}
}

func (r *Renderer) destroyAliasResourcesLocked() {
	r.clearAliasModelsLocked()
	if r.aliasShadowSkin != nil {
		if r.aliasShadowSkin.bindGroup != nil {
			r.aliasShadowSkin.bindGroup.Release()
		}
		if r.aliasShadowSkin.fullbrightView != nil {
			r.aliasShadowSkin.fullbrightView.Release()
		}
		if r.aliasShadowSkin.fullbrightTexture != nil {
			r.aliasShadowSkin.fullbrightTexture.Release()
		}
		if r.aliasShadowSkin.view != nil {
			r.aliasShadowSkin.view.Release()
		}
		if r.aliasShadowSkin.texture != nil {
			r.aliasShadowSkin.texture.Release()
		}
		r.aliasShadowSkin = nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Release()
		r.aliasScratchBuffer = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Release()
		r.aliasUniformBuffer = nil
	}
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Release()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasSampler != nil {
		r.aliasSampler.Release()
		r.aliasSampler = nil
	}
	if r.aliasPipeline != nil {
		r.aliasPipeline.Release()
		r.aliasPipeline = nil
	}
	if r.aliasShadowPipeline != nil {
		r.aliasShadowPipeline.Release()
		r.aliasShadowPipeline = nil
	}
	if r.aliasPipelineLayout != nil {
		r.aliasPipelineLayout.Release()
		r.aliasPipelineLayout = nil
	}
	if r.aliasVertexShader != nil {
		r.aliasVertexShader.Release()
		r.aliasVertexShader = nil
	}
	if r.aliasFragmentShader != nil {
		r.aliasFragmentShader.Release()
		r.aliasFragmentShader = nil
	}
	if r.aliasUniformBindGroupLayout != nil {
		r.aliasUniformBindGroupLayout.Release()
		r.aliasUniformBindGroupLayout = nil
	}
	if r.aliasTextureBindGroupLayout != nil {
		r.aliasTextureBindGroupLayout.Release()
		r.aliasTextureBindGroupLayout = nil
	}
}

func (r *Renderer) clearSpriteModelsLocked() {
	for key, cached := range r.spriteModels {
		for _, frame := range cached.frames {
			if frame.bindGroup != nil {
				frame.bindGroup.Release()
			}
			if frame.view != nil {
				frame.view.Release()
			}
			if frame.texture != nil {
				frame.texture.Release()
			}
		}
		delete(r.spriteModels, key)
	}
}

func (r *Renderer) destroySpriteResourcesLocked() {
	r.clearSpriteModelsLocked()
	if r.spriteUniformBuffer != nil {
		r.spriteUniformBuffer.Release()
		r.spriteUniformBuffer = nil
	}
	if r.spriteUniformBindGroup != nil {
		r.spriteUniformBindGroup.Release()
		r.spriteUniformBindGroup = nil
	}
	if r.spritePipeline != nil {
		r.spritePipeline.Release()
		r.spritePipeline = nil
	}
	if r.spriteVertexShader != nil {
		r.spriteVertexShader.Release()
		r.spriteVertexShader = nil
	}
	if r.spriteFragmentShader != nil {
		r.spriteFragmentShader.Release()
		r.spriteFragmentShader = nil
	}
}

func (r *Renderer) destroyDecalResourcesLocked() {
	if r.decalBindGroup != nil {
		r.decalBindGroup.Release()
		r.decalBindGroup = nil
	}
	if r.decalAtlasView != nil {
		r.decalAtlasView.Release()
		r.decalAtlasView = nil
	}
	if r.decalAtlasTextureHAL != nil {
		r.decalAtlasTextureHAL.Release()
		r.decalAtlasTextureHAL = nil
	}
	if r.decalUniformBuffer != nil {
		r.decalUniformBuffer.Release()
		r.decalUniformBuffer = nil
	}
	if r.decalUniformBindGroup != nil {
		r.decalUniformBindGroup.Release()
		r.decalUniformBindGroup = nil
	}
	if r.decalUniformLayout != nil {
		r.decalUniformLayout.Release()
		r.decalUniformLayout = nil
	}
	if r.decalPipelineLayout != nil {
		r.decalPipelineLayout.Release()
		r.decalPipelineLayout = nil
	}
	if r.decalPipeline != nil {
		r.decalPipeline.Release()
		r.decalPipeline = nil
	}
	if r.decalVertexShader != nil {
		r.decalVertexShader.Release()
		r.decalVertexShader = nil
	}
	if r.decalFragmentShader != nil {
		r.decalFragmentShader.Release()
		r.decalFragmentShader = nil
	}
}

// ---- merged from world_support_gogpu_root.go ----
type gpuAliasSkin struct {
	texture           *wgpu.Texture
	view              *wgpu.TextureView
	fullbrightTexture *wgpu.Texture
	fullbrightView    *wgpu.TextureView
	bindGroup         *wgpu.BindGroup
}

type gpuAliasModel struct {
	modelID     string
	flags       int
	skins       []gpuAliasSkin
	playerSkins map[uint32][]gpuAliasSkin
	poses       [][]model.TriVertX
	refs        []aliasimpl.MeshRef
}

type gpuAliasDraw struct {
	alias  *gpuAliasModel
	model  *model.Model
	pose1  int
	pose2  int
	blend  float32
	skin   *gpuAliasSkin
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
	full   bool
}

type gpuSpriteFrame struct {
	meta      spriteRenderFrame
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
}

type gpuSpriteModel struct {
	modelID    string
	spriteType int
	frames     []gpuSpriteFrame
	maxWidth   int
	maxHeight  int
	bounds     [3][2]float32
}

type gpuSpriteDraw struct {
	sprite *gpuSpriteModel
	frame  int
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
}

func (r *Renderer) ensureAliasScratchBufferLocked(device *wgpu.Device, size uint64) error {
	if size == 0 {
		size = 44
	}
	if r.aliasScratchBuffer != nil && r.aliasScratchBufferSize >= size {
		return nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Release()
		r.aliasScratchBuffer = nil
		r.aliasScratchBufferSize = 0
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Scratch Buffer",
		Size:             size,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create alias scratch buffer: %w", err)
	}
	r.aliasScratchBuffer = buffer
	r.aliasScratchBufferSize = size
	return nil
}

// ensureAliasUniformBufferLocked grows the alias uniform buffer and rebuilds
// the bind group when the current buffer is too small for numDraws draws.
func (r *Renderer) ensureAliasUniformBufferLocked(device *wgpu.Device, numDraws int) error {
	needed := uint64(numDraws) * aliasUniformAlign
	if needed < aliasSceneUniformBufferSize {
		needed = aliasSceneUniformBufferSize
	}
	if r.aliasUniformBuffer != nil && r.aliasUniformBuffer.Size() >= needed {
		return nil
	}
	// Release old resources.
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Release()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Release()
		r.aliasUniformBuffer = nil
	}
	buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             needed,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("grow alias uniform buffer: %w", err)
	}
	bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Alias Uniform BG",
		Layout:  r.aliasUniformBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: buf, Offset: 0, Size: aliasSceneUniformBufferSize}},
	})
	if err != nil {
		buf.Release()
		return fmt.Errorf("recreate alias uniform bind group: %w", err)
	}
	r.aliasUniformBuffer = buf
	r.aliasUniformBindGroup = bg
	return nil
}

func aliasDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
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
		StencilReadOnly:   true,
	}
}

func putFloat32s(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

// ---- merged from world_translucent_sort_gogpu_root.go ----
func destroyGoGPUTransientBuffers(buffers []*wgpu.Buffer) {
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Release()
		}
	}
}

type gogpuTranslucentBrushFaceRender struct {
	bufferPair  [2]*wgpu.Buffer
	frame       int
	face        gogpuTranslucentLiquidFaceDraw
	liquid      bool
	hasLitWater bool
	center      [3]float32
	lightmaps   []*gpuWorldTexture
}

func sortGoGPUTranslucentBrushFaceRenders(mode AlphaMode, renders []gogpuTranslucentBrushFaceRender) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(renders, func(i, j int) bool {
		return renders[i].face.distanceSq > renders[j].face.distanceSq
	})
}
