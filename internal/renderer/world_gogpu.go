//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	aliasimpl "github.com/ironwail/ironwail-go/internal/renderer/alias"
	worldgogpu "github.com/ironwail/ironwail-go/internal/renderer/world/gogpu"
	"github.com/ironwail/ironwail-go/pkg/types"
	"log/slog"
	"math"
	"sort"
	"strings"
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

func buildGoGPUTranslucentLiquidBrushEntityDraw(entity BrushEntity, geom *WorldGeometry, liquidAlpha worldLiquidAlphaSettings, camera CameraState) *gogpuTranslucentLiquidBrushEntityDraw {
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return nil
	}
	alpha := clamp01(entity.Alpha)
	if !isFullyOpaqueAlpha(alpha) {
		return nil
	}
	rotation := buildBrushRotationMatrix(entity.Angles)
	vertices := make([]WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = transformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	faces := make([]gogpuTranslucentLiquidFaceDraw, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
	for _, face := range geom.Faces {
		if !shouldDrawGoGPUTranslucentLiquidBrushFace(face, alpha, liquidAlpha) {
			continue
		}
		first := int(face.FirstIndex)
		last := first + int(face.NumIndices)
		if first < 0 {
			first = 0
		}
		if last > len(geom.Indices) {
			last = len(geom.Indices)
		}
		if first >= last {
			continue
		}
		drawFace := face
		drawFace.FirstIndex = uint32(len(indices))
		drawFace.NumIndices = uint32(last - first)
		center := transformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale)
		faces = append(faces, gogpuTranslucentLiquidFaceDraw{
			face:       drawFace,
			alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
			center:     center,
			distanceSq: worldFaceDistanceSq(center, camera),
		})
		indices = append(indices, geom.Indices[first:last]...)
	}
	if len(faces) == 0 || len(indices) == 0 {
		return nil
	}
	return &gogpuTranslucentLiquidBrushEntityDraw{
		frame:    entity.Frame,
		vertices: vertices,
		indices:  indices,
		faces:    faces,
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
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return nil
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 || alpha >= 1 {
		return nil
	}
	rotation := buildBrushRotationMatrix(entity.Angles)
	vertices := make([]WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = transformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	alphaTestFaces := make([]WorldFace, 0, len(geom.Faces))
	alphaTestCenters := make([][3]float32, 0, len(geom.Faces))
	translucentFaces := make([]gogpuTranslucentLiquidFaceDraw, 0, len(geom.Faces))
	liquidFaces := make([]gogpuTranslucentLiquidFaceDraw, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
	for _, face := range geom.Faces {
		if !shouldDrawGoGPUTranslucentBrushEntityFace(face, alpha, liquidAlpha) {
			continue
		}
		first := int(face.FirstIndex)
		last := first + int(face.NumIndices)
		if first < 0 {
			first = 0
		}
		if last > len(geom.Indices) {
			last = len(geom.Indices)
		}
		if first >= last {
			continue
		}
		drawFace := face
		drawFace.FirstIndex = uint32(len(indices))
		drawFace.NumIndices = uint32(last - first)
		indices = append(indices, geom.Indices[first:last]...)
		faceAlpha := worldFaceAlpha(face.Flags, liquidAlpha) * alpha
		center := transformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale)
		switch worldFacePass(face.Flags, faceAlpha) {
		case worldPassAlphaTest:
			alphaTestFaces = append(alphaTestFaces, drawFace)
			alphaTestCenters = append(alphaTestCenters, center)
		case worldPassTranslucent:
			draw := gogpuTranslucentLiquidFaceDraw{
				face:       drawFace,
				alpha:      faceAlpha,
				center:     center,
				distanceSq: worldFaceDistanceSq(center, camera),
			}
			if worldFaceIsLiquid(face.Flags) {
				liquidFaces = append(liquidFaces, draw)
				continue
			}
			translucentFaces = append(translucentFaces, draw)
		}
	}
	if len(alphaTestFaces) == 0 && len(translucentFaces) == 0 && len(liquidFaces) == 0 {
		return nil
	}
	return &gogpuTranslucentBrushEntityDraw{
		frame:            entity.Frame,
		vertices:         vertices,
		indices:          indices,
		alphaTestFaces:   alphaTestFaces,
		alphaTestCenters: alphaTestCenters,
		translucentFaces: translucentFaces,
		liquidFaces:      liquidFaces,
	}
}

func (dc *DrawContext) renderOpaqueBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Brush Entity Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush entity encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_entities"); err != nil {
		slog.Warn("failed to begin brush entity encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Brush Entity Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
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
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Entity Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Entity Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create brush index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		renderPass.SetVertexBuffer(0, vertexBuffer, 0)
		renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)
		for faceIndex, face := range draw.faces {
			dynamicLight := [3]float32{}
			if faceIndex < len(draw.centers) {
				dynamicLight = evaluateDynamicLightsAtPoint(activeDynamicLights, draw.centers[faceIndex])
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
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish brush entity encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Destroy()
		}
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit brush entity commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Destroy()
	}
}

func (dc *DrawContext) renderSkyBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Brush Sky Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush sky encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_sky"); err != nil {
		slog.Warn("failed to begin brush sky encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Brush Sky Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
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
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Sky Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush sky vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush sky vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Sky Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create brush sky index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
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
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		}
	}
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish brush sky encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Destroy()
		}
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit brush sky commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Destroy()
	}
}

func (dc *DrawContext) renderOpaqueLiquidBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Brush Liquid Render Encoder"})
	if err != nil {
		slog.Warn("failed to create brush liquid encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_liquid"); err != nil {
		slog.Warn("failed to begin brush liquid encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Brush Liquid Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
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
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Liquid Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create brush liquid index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
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
				dynamicLight = evaluateDynamicLightsAtPoint(activeDynamicLights, draw.centers[faceIndex])
			}
			lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(face, draw.lightmaps, whiteLightmapBindGroup)
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
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish brush liquid encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Destroy()
		}
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit brush liquid commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Destroy()
	}
}

type gogpuTranslucentLiquidBrushFaceRender struct {
	bufferPair [2]hal.Buffer
	frame      int
	face       gogpuTranslucentLiquidFaceDraw
	lightmaps  []*gpuWorldTexture
}

func sortGoGPUTranslucentLiquidBrushFaceRenders(mode AlphaMode, renders []gogpuTranslucentLiquidBrushFaceRender) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(renders, func(i, j int) bool {
		return renders[i].face.distanceSq > renders[j].face.distanceSq
	})
}

func (dc *DrawContext) renderTranslucentLiquidBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	r := dc.renderer
	r.mu.RLock()
	var treeEntities []byte
	var tree *bsp.Tree
	camera := r.cameraState
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		tree = r.worldData.Geometry.Tree
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if tree == nil {
		return
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(treeEntities), tree)
	draws := make([]gogpuTranslucentLiquidBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentLiquidBrushEntityDraw(entity, geom, liquidAlpha, camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return
	}

	r.mu.RLock()
	pipeline := r.worldTranslucentTurbulentPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	transparentBindGroup := r.transparentBindGroup
	depthView := r.worldDepthTextureView
	camera = r.cameraState
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Brush Translucent Liquid Render Encoder"})
	if err != nil {
		slog.Warn("failed to create translucent brush liquid encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_liquid_translucent"); err != nil {
		slog.Warn("failed to begin translucent brush liquid encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Brush Translucent Liquid Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
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
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	faceRenders := make([]gogpuTranslucentLiquidBrushFaceRender, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Liquid Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create translucent brush liquid index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush liquid index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		for _, face := range draw.faces {
			faceRenders = append(faceRenders, gogpuTranslucentLiquidBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				lightmaps:  draw.lightmaps,
			})
		}
	}
	sortGoGPUTranslucentLiquidBrushFaceRenders(effectiveGoGPUAlphaMode(GetAlphaMode()), faceRenders)
	for _, draw := range faceRenders {
		dynamicLight := evaluateDynamicLightsAtPoint(activeDynamicLights, draw.face.center)
		lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(draw.face.face, draw.lightmaps, whiteLightmapBindGroup)
		if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.face.alpha, dynamicLight, litWater)); err != nil {
			slog.Warn("failed to update translucent brush liquid uniform buffer", "error", err)
			continue
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup := whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish translucent brush liquid encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Destroy()
		}
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit translucent brush liquid commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Destroy()
	}
}

func (dc *DrawContext) renderTranslucentBrushEntitiesHAL(entities []BrushEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	r := dc.renderer
	r.mu.RLock()
	var treeEntities []byte
	var tree *bsp.Tree
	camera := r.cameraState
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		tree = r.worldData.Geometry.Tree
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if tree == nil {
		return
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(treeEntities), tree)

	draws := make([]gogpuTranslucentBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentBrushEntityDraw(entity, geom, liquidAlpha, camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return
	}

	r.mu.RLock()
	alphaTestPipeline := r.worldPipeline
	translucentPipeline := r.worldTranslucentPipeline
	liquidPipeline := r.worldTranslucentTurbulentPipeline
	uniformBuffer := r.uniformBuffer
	uniformBindGroup := r.uniformBindGroup
	whiteTextureBindGroup := r.whiteTextureBindGroup
	whiteLightmapBindGroup := r.whiteLightmapBindGroup
	transparentBindGroup := r.transparentBindGroup
	depthView := r.worldDepthTextureView
	camera = r.cameraState
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
	if alphaTestPipeline == nil || translucentPipeline == nil || liquidPipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || whiteTextureBindGroup == nil || whiteLightmapBindGroup == nil {
		return
	}
	if transparentBindGroup == nil {
		transparentBindGroup = whiteTextureBindGroup
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Brush Translucent Render Encoder"})
	if err != nil {
		slog.Warn("failed to create translucent brush encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_translucent"); err != nil {
		slog.Warn("failed to begin translucent brush encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Brush Translucent Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	timeSeconds := float64(camera.Time)
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	alphaTestRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws))
	translucentRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create translucent brush index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		for faceIndex, face := range draw.alphaTestFaces {
			center := [3]float32{}
			if faceIndex < len(draw.alphaTestCenters) {
				center = draw.alphaTestCenters[faceIndex]
			}
			alphaTestRenders = append(alphaTestRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
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
			translucentRenders = append(translucentRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				lightmaps:  draw.lightmaps,
			})
		}
		for _, face := range draw.liquidFaces {
			translucentRenders = append(translucentRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				liquid:     true,
				lightmaps:  draw.lightmaps,
			})
		}
	}
	renderPass.SetPipeline(alphaTestPipeline)
	for _, draw := range alphaTestRenders {
		dynamicLight := evaluateDynamicLightsAtPoint(activeDynamicLights, draw.center)
		if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.face.alpha, dynamicLight, 0)); err != nil {
			slog.Warn("failed to update translucent brush alpha-test uniform buffer", "error", err)
			continue
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup := whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		lightmapBindGroup := whiteLightmapBindGroup
		if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(draw.lightmaps) {
			if lightmapPage := draw.lightmaps[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
				lightmapBindGroup = lightmapPage.bindGroup
			}
		} else if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(worldLightmapPages) {
			if lightmapPage := worldLightmapPages[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
				lightmapBindGroup = lightmapPage.bindGroup
			}
		}
		fullbrightBindGroup := transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}

	sortGoGPUTranslucentBrushFaceRenders(effectiveGoGPUAlphaMode(GetAlphaMode()), translucentRenders)
	for _, draw := range translucentRenders {
		dynamicLight := evaluateDynamicLightsAtPoint(activeDynamicLights, draw.face.center)
		litWater := float32(0)
		lightmapBindGroup := whiteLightmapBindGroup
		if draw.liquid {
			lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(draw.face.face, draw.lightmaps, whiteLightmapBindGroup)
			if lightmapBindGroup == whiteLightmapBindGroup {
				lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(draw.face.face, worldLightmapPages, whiteLightmapBindGroup)
			}
		} else {
			if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(draw.lightmaps) {
				if lightmapPage := draw.lightmaps[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
					lightmapBindGroup = lightmapPage.bindGroup
				}
			} else if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(worldLightmapPages) {
				if lightmapPage := worldLightmapPages[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
					lightmapBindGroup = lightmapPage.bindGroup
				}
			}
		}
		if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, draw.face.alpha, dynamicLight, litWater)); err != nil {
			slog.Warn("failed to update translucent brush uniform buffer", "error", err)
			continue
		}
		if draw.liquid {
			renderPass.SetPipeline(liquidPipeline)
		} else {
			renderPass.SetPipeline(translucentPipeline)
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup := whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish translucent brush encoding", "error", err)
		for _, buffer := range buffers {
			buffer.Destroy()
		}
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit translucent brush commands", "error", err)
	}
	for _, buffer := range buffers {
		buffer.Destroy()
	}
}

// ---- merged from world_alias_gogpu_root.go ----
const (
	aliasUniformBufferSize      = 80
	aliasSceneUniformBufferSize = 96
)

func (r *Renderer) ensureAliasResourcesLocked(device hal.Device) error {
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
		vertexShader.Destroy()
		return fmt.Errorf("create alias fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Alias Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   aliasSceneUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias texture layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Alias Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout, textureLayout},
	})
	if err != nil {
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             aliasSceneUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform buffer: %w", err)
	}

	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Alias Uniform BG",
		Layout: uniformLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   aliasSceneUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias uniform bind group: %w", err)
	}

	sampler, err := device.CreateSampler(&hal.SamplerDescriptor{
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
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
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
		sampler.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create alias pipeline: %w", err)
	}
	shadowPipeline, err := createAliasRenderPipeline(device, vertexShader, fragmentShader, pipelineLayout, surfaceFormat, "Alias Shadow Render Pipeline", false)
	if err != nil {
		pipeline.Destroy()
		sampler.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		textureLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
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

func createAliasRenderPipeline(device hal.Device, vertexShader, fragmentShader hal.ShaderModule, layout hal.PipelineLayout, surfaceFormat gputypes.TextureFormat, label string, depthWrite bool) (hal.RenderPipeline, error) {
	return device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  label,
		Layout: layout,
		Vertex: hal.VertexState{
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
		Fragment: &hal.FragmentState{
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

func (r *Renderer) ensureAliasDepthTextureLocked(device hal.Device) {
	if r.worldDepthTextureView != nil || device == nil {
		return
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return
	}
	depthTexture, depthView, err := r.createWorldDepthTexture(device, width, height)
	if err != nil {
		slog.Warn("failed to create alias depth texture", "error", err)
		return
	}
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthView
}

func (r *Renderer) ensureAliasModelLocked(device hal.Device, queue hal.Queue, modelID string, mdl *model.Model) *gpuAliasModel {
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

	refs := make([]gpuAliasVertexRef, 0, len(hdr.Triangles)*3)
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
			refs = append(refs, gpuAliasVertexRef{
				vertexIndex: idx,
				texCoord: [2]float32{
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

func (r *Renderer) createAliasSkinLocked(device hal.Device, queue hal.Queue, width, height int, pixels []byte) (gpuAliasSkin, error) {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(pixels, r.palette, 0, false)
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Alias Skin Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuAliasSkin{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, baseRGBA, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
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
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create texture view: %w", err)
	}
	fullbrightTexture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Alias Fullbright Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  fullbrightTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, fullbrightRGBA, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("write fullbright texture: %w", err)
	}
	fullbrightView, err := device.CreateTextureView(fullbrightTexture, &hal.TextureViewDescriptor{
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
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
		return gpuAliasSkin{}, fmt.Errorf("create fullbright texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Alias Skin BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: fullbrightView.NativeHandle()}},
		},
	})
	if err != nil {
		fullbrightView.Destroy()
		fullbrightTexture.Destroy()
		view.Destroy()
		texture.Destroy()
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

func (r *Renderer) resolveAliasSkinLocked(device hal.Device, queue hal.Queue, alias *gpuAliasModel, entity AliasModelEntity, slot int) *gpuAliasSkin {
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

func (r *Renderer) buildAliasDrawLocked(device hal.Device, queue hal.Queue, entity AliasModelEntity, fullAngles bool) *gpuAliasDraw {
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	maxVertexBytes := uint64(44)
	for _, draw := range draws {
		if draw.alias == nil {
			continue
		}
		size := uint64(len(draw.alias.refs) * 44)
		if size > maxVertexBytes {
			maxVertexBytes = size
		}
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer", "error", err)
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Alias Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("alias"); err != nil {
		slog.Warn("failed to begin alias encoding", "error", err)
		return
	}

	renderPassDesc := &hal.RenderPassDescriptor{
		Label: "Alias Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	}
	renderPass := encoder.BeginRenderPass(renderPassDesc)
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
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	for _, draw := range draws {
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertices) == 0 || draw.skin == nil || draw.skin.bindGroup == nil {
			continue
		}
		if err := queue.WriteBuffer(uniformBuffer, 0, aliasSceneUniformBytes(vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update alias uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(vertices)); err != nil {
			slog.Warn("failed to upload alias vertices", "error", err)
			continue
		}
		renderPass.SetBindGroup(1, draw.skin.bindGroup, nil)
		renderPass.Draw(uint32(len(vertices)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish alias encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
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
	return aliasimpl.BuildVerticesInterpolated(goGPUAliasMesh(alias), mdl.AliasHeader, pose1Index, pose2Index, blend, origin, angles, entityScale, fullAngles)
}

func setupAliasFrameInterpolation(frameIndex int, frames []model.AliasFrameDesc, timeSeconds float64, lerpModels bool, flags int) InterpolationData {
	var result InterpolationData
	if len(frames) == 0 {
		return result
	}
	if frameIndex < 0 || frameIndex >= len(frames) {
		frameIndex = 0
	}
	frameDesc := frames[frameIndex]
	if frameDesc.NumPoses <= 0 {
		result.Pose1 = frameDesc.FirstPose
		result.Pose2 = frameDesc.FirstPose
		return result
	}
	poseOffset := 0
	if frameDesc.NumPoses > 1 {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		poseOffset = int(timeSeconds/float64(interval)) % frameDesc.NumPoses
	}
	currentPose := frameDesc.FirstPose + poseOffset
	if frameDesc.NumPoses <= 1 {
		result.Pose1 = currentPose
		result.Pose2 = currentPose
		return result
	}
	nextPose := frameDesc.FirstPose + (poseOffset+1)%frameDesc.NumPoses
	if lerpModels && (flags&ModNoLerp == 0) {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		timeInInterval := math.Mod(timeSeconds, float64(interval))
		result.Blend = clamp01(float32(timeInInterval / float64(interval)))
	}
	result.Pose1 = currentPose
	result.Pose2 = nextPose
	return result
}

type InterpolationData struct {
	Pose1 int
	Pose2 int
	Blend float32
}

func goGPUAliasMesh(alias *gpuAliasModel) aliasimpl.Mesh {
	return aliasimpl.Mesh{
		Poses:    alias.poses,
		RefCount: len(alias.refs),
		RefAt: func(index int) aliasimpl.MeshRef {
			ref := alias.refs[index]
			return aliasimpl.MeshRef{VertexIndex: ref.vertexIndex, TexCoord: ref.texCoord}
		},
	}
}

// ---- merged from world_sprite_gogpu_root.go ----

func (r *Renderer) ensureSpriteResourcesLocked(device hal.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.spritePipeline != nil && r.spriteUniformBuffer != nil && r.spriteUniformBindGroup != nil {
		return nil
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Sprite Uniform Buffer",
		Size:             worldgogpu.SpriteUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create sprite uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Sprite Uniform BG",
		Layout: r.aliasUniformBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   worldgogpu.SpriteUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite uniform bind group: %w", err)
	}

	vertexShader, err := createWorldShaderModule(device, worldgogpu.SpriteVertexShaderWGSL, "Sprite Vertex Shader")
	if err != nil {
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, worldgogpu.SpriteFragmentShaderWGSL, "Sprite Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite fragment shader: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Sprite Render Pipeline",
		Layout: r.aliasPipelineLayout,
		Vertex: hal.VertexState{
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
		Fragment: &hal.FragmentState{
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
		vertexShader.Destroy()
		fragmentShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite pipeline: %w", err)
	}

	r.spriteUniformBuffer = uniformBuffer
	r.spriteUniformBindGroup = uniformBindGroup
	r.spriteVertexShader = vertexShader
	r.spriteFragmentShader = fragmentShader
	r.spritePipeline = pipeline
	return nil
}

func (r *Renderer) createSpriteFrameLocked(device hal.Device, queue hal.Queue, frame spriteRenderFrame) (gpuSpriteFrame, error) {
	width, height := frame.width, frame.height
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	pixels := frame.pixels
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	rgba := ConvertPaletteToRGBA(pixels, r.palette)
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Sprite Frame Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuSpriteFrame{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return gpuSpriteFrame{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
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
		texture.Destroy()
		return gpuSpriteFrame{}, fmt.Errorf("create texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Sprite Frame BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
		},
	})
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return gpuSpriteFrame{}, fmt.Errorf("create bind group: %w", err)
	}
	return gpuSpriteFrame{
		meta:      frame,
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
	}, nil
}

func (r *Renderer) ensureSpriteModelLocked(device hal.Device, queue hal.Queue, modelID string, spr *model.MSprite) *gpuSpriteModel {
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
					uploaded.bindGroup.Destroy()
				}
				if uploaded.view != nil {
					uploaded.view.Destroy()
				}
				if uploaded.texture != nil {
					uploaded.texture.Destroy()
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

func (r *Renderer) buildSpriteDrawLocked(device hal.Device, queue hal.Queue, entity SpriteEntity) *gpuSpriteDraw {
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Sprite Render Encoder"})
	if err != nil {
		slog.Warn("failed to create sprite encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("sprite"); err != nil {
		slog.Warn("failed to begin sprite encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Sprite Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

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

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish sprite encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit sprite commands", "error", err)
	}
}

// ---- merged from world_decal_gogpu_root.go ----
type gpuDecalVertex struct {
	Position [3]float32
	TexCoord [2]float32
	Color    [4]float32
}

func (r *Renderer) ensureDecalResourcesLocked(device hal.Device, queue hal.Queue) error {
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
		vertexShader.Destroy()
		return fmt.Errorf("create decal fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
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
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Decal Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout, r.aliasTextureBindGroupLayout},
	})
	if err != nil {
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Decal Uniform Buffer",
		Size:             worldgogpu.DecalUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Decal Uniform BG",
		Layout: uniformLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   worldgogpu.DecalUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform bind group: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Decal Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
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
		Fragment: &hal.FragmentState{
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
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal pipeline: %w", err)
	}

	atlasData := generateDecalAtlasData()
	atlasTexture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Decal Atlas Texture",
		Size:          hal.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal atlas texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  atlasTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, atlasData, &hal.ImageDataLayout{BytesPerRow: 256 * 4, RowsPerImage: 256}, &hal.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1}); err != nil {
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("write decal atlas texture: %w", err)
	}
	atlasView, err := device.CreateTextureView(atlasTexture, &hal.TextureViewDescriptor{
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
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal atlas view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Decal Atlas BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: atlasView.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: atlasView.NativeHandle()}},
		},
	})
	if err != nil {
		atlasView.Destroy()
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Decal Render Encoder"})
	if err != nil {
		slog.Warn("failed to create decal encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("decal"); err != nil {
		slog.Warn("failed to begin decal encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Decal Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: decalDepthAttachmentForView(depthView),
	})
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

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish decal encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
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

func decalDepthStencilState() *hal.DepthStencilState {
	stencilFace := hal.StencilFaceState{
		Compare:     gputypes.CompareFunctionEqual,
		FailOp:      hal.StencilOperationKeep,
		DepthFailOp: hal.StencilOperationKeep,
		PassOp:      hal.StencilOperationIncrementClamp,
	}
	return &hal.DepthStencilState{
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

func decalDepthAttachmentForView(view hal.TextureView) *hal.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &hal.RenderPassDepthStencilAttachment{
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
	device                  hal.Device
	queue                   hal.Queue
	textureView             hal.TextureView
	alphaTestPipeline       hal.RenderPipeline
	translucentPipeline     hal.RenderPipeline
	liquidPipeline          hal.RenderPipeline
	uniformBuffer           hal.Buffer
	uniformBindGroup        hal.BindGroup
	whiteTextureBindGroup   hal.BindGroup
	whiteLightmapBindGroup  hal.BindGroup
	transparentBindGroup    hal.BindGroup
	depthView               hal.TextureView
	camera                  CameraState
	worldTextures           map[int32]*gpuWorldTexture
	worldFullbrightTextures map[int32]*gpuWorldTexture
	worldTextureAnimations  []*SurfaceTexture
	worldLightmapPages      []*gpuWorldTexture
	activeDynamicLights     []DynamicLight
}

func (dc *DrawContext) loadGoGPULateTranslucentFaceResources() (gogpuLateTranslucentFaceResources, bool) {
	if dc == nil || dc.renderer == nil {
		return gogpuLateTranslucentFaceResources{}, false
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	textureView := dc.currentHALRenderTargetView()
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
		uniformBindGroup:        r.uniformBindGroup,
		whiteTextureBindGroup:   r.whiteTextureBindGroup,
		whiteLightmapBindGroup:  r.whiteLightmapBindGroup,
		transparentBindGroup:    r.transparentBindGroup,
		depthView:               r.worldDepthTextureView,
		camera:                  r.cameraState,
		worldTextures:           make(map[int32]*gpuWorldTexture, len(r.worldTextures)),
		worldFullbrightTextures: make(map[int32]*gpuWorldTexture, len(r.worldFullbrightTextures)),
		worldTextureAnimations:  append([]*SurfaceTexture(nil), r.worldTextureAnimations...),
		worldLightmapPages:      append([]*gpuWorldTexture(nil), r.worldLightmapPages...),
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
	r.mu.RUnlock()
	if res.translucentPipeline == nil || res.liquidPipeline == nil || res.uniformBuffer == nil || res.uniformBindGroup == nil || res.whiteTextureBindGroup == nil || res.whiteLightmapBindGroup == nil {
		return gogpuLateTranslucentFaceResources{}, false
	}
	if res.transparentBindGroup == nil {
		res.transparentBindGroup = res.whiteTextureBindGroup
	}
	return res, true
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
	renders := make([]gogpuTranslucentBrushFaceRender, 0, 8)
	for _, face := range worldData.Geometry.Faces {
		if !shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha) {
			continue
		}
		renders = append(renders, gogpuTranslucentBrushFaceRender{
			bufferPair: [2]hal.Buffer{worldVertexBuffer, worldIndexBuffer},
			face: gogpuTranslucentLiquidFaceDraw{
				face:       face,
				alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
				center:     face.Center,
				distanceSq: worldFaceDistanceSq(face.Center, camera),
			},
			liquid:    true,
			lightmaps: worldLightmapPages,
		})
	}
	return renders
}

func (dc *DrawContext) collectGoGPUTranslucentLiquidBrushFaceRenders(entities []BrushEntity) ([]gogpuTranslucentBrushFaceRender, []hal.Buffer) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return nil, nil
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return nil, nil
	}

	r := dc.renderer
	r.mu.RLock()
	var treeEntities []byte
	var tree *bsp.Tree
	camera := r.cameraState
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		tree = r.worldData.Geometry.Tree
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if tree == nil {
		return nil, nil
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(treeEntities), tree)
	draws := make([]gogpuTranslucentLiquidBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentLiquidBrushEntityDraw(entity, geom, liquidAlpha, camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil
	}

	renders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Liquid Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create translucent brush liquid index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush liquid index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		for _, face := range draw.faces {
			renders = append(renders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				liquid:     true,
				lightmaps:  draw.lightmaps,
			})
		}
	}
	return renders, buffers
}

func (dc *DrawContext) collectGoGPUTranslucentBrushEntityFaceRenders(entities []BrushEntity) ([]gogpuTranslucentBrushFaceRender, []gogpuTranslucentBrushFaceRender, []hal.Buffer) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return nil, nil, nil
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return nil, nil, nil
	}

	r := dc.renderer
	r.mu.RLock()
	var treeEntities []byte
	var tree *bsp.Tree
	camera := r.cameraState
	if r.worldData != nil && r.worldData.Geometry != nil && r.worldData.Geometry.Tree != nil {
		tree = r.worldData.Geometry.Tree
		treeEntities = r.worldData.Geometry.Tree.Entities
	}
	r.mu.RUnlock()
	if tree == nil {
		return nil, nil, nil
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(parseWorldspawnLiquidAlphaOverrides(treeEntities), tree)
	draws := make([]gogpuTranslucentBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentBrushEntityDraw(entity, geom, liquidAlpha, camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil, nil
	}

	alphaTestRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws))
	translucentRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	buffers := make([]hal.Buffer, 0, len(draws)*2)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		vertexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := worldgogpu.CreateBrushBuffer(device, "Brush Translucent Indices", gputypes.BufferUsageIndex, indexData)
		if err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to create translucent brush index buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(indexBuffer, 0, indexData); err != nil {
			indexBuffer.Destroy()
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush index buffer", "error", err)
			continue
		}
		buffers = append(buffers, vertexBuffer, indexBuffer)
		for faceIndex, face := range draw.alphaTestFaces {
			center := [3]float32{}
			if faceIndex < len(draw.alphaTestCenters) {
				center = draw.alphaTestCenters[faceIndex]
			}
			alphaTestRenders = append(alphaTestRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
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
			translucentRenders = append(translucentRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				lightmaps:  draw.lightmaps,
			})
		}
		for _, face := range draw.liquidFaces {
			translucentRenders = append(translucentRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
				frame:      draw.frame,
				face:       face,
				liquid:     true,
				lightmaps:  draw.lightmaps,
			})
		}
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
	encoder, err := res.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "GoGPU Alpha-Test Brush Encoder"})
	if err != nil {
		slog.Warn("failed to create alpha-test brush encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("brush_alpha_test"); err != nil {
		slog.Warn("failed to begin alpha-test brush encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "GoGPU Alpha-Test Brush Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    res.textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(res.depthView),
	})
	renderPass.SetPipeline(res.alphaTestPipeline)
	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetBindGroup(0, res.uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	for _, draw := range renders {
		dynamicLight := evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.center)
		if err := res.queue.WriteBuffer(res.uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, timeValue, draw.face.alpha, dynamicLight, 0)); err != nil {
			slog.Warn("failed to update alpha-test brush uniform buffer", "error", err)
			continue
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], 0)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, 0)
		textureBindGroup := res.whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
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
		fullbrightBindGroup := res.transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldFullbrightTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish alpha-test brush encoding", "error", err)
		return
	}
	if err := res.queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
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
	encoder, err := res.device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "GoGPU Late Translucent Encoder"})
	if err != nil {
		slog.Warn("failed to create late translucent encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("late_translucent"); err != nil {
		slog.Warn("failed to begin late translucent encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "GoGPU Late Translucent Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    res.textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(res.depthView),
	})
	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetBindGroup(0, res.uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	for _, draw := range renders {
		dynamicLight := evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.face.center)
		litWater := float32(0)
		lightmapBindGroup := res.whiteLightmapBindGroup
		if draw.liquid {
			lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(draw.face.face, draw.lightmaps, res.whiteLightmapBindGroup)
			if lightmapBindGroup == res.whiteLightmapBindGroup {
				lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(draw.face.face, res.worldLightmapPages, res.whiteLightmapBindGroup)
			}
		} else {
			if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(draw.lightmaps) {
				if lightmapPage := draw.lightmaps[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
					lightmapBindGroup = lightmapPage.bindGroup
				}
			} else if draw.face.face.LightmapIndex >= 0 && int(draw.face.face.LightmapIndex) < len(res.worldLightmapPages) {
				if lightmapPage := res.worldLightmapPages[draw.face.face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
					lightmapBindGroup = lightmapPage.bindGroup
				}
			}
		}
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
		textureBindGroup := res.whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := res.transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldFullbrightTextures, res.worldTextureAnimations, nil, draw.frame, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish late translucent encoding", "error", err)
		return
	}
	if err := res.queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
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

	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
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
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	maxVertexBytes := uint64(44)
	for _, draw := range draws {
		size := uint64(len(draw.vertices) * 44)
		if size > maxVertexBytes {
			maxVertexBytes = size
		}
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer for shadows", "error", err)
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

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Alias Shadow Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias shadow encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("alias-shadow"); err != nil {
		slog.Warn("failed to begin alias shadow encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Alias Shadow Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)
	renderPass.SetBindGroup(1, shadowSkin.bindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	for _, draw := range draws {
		if len(draw.vertices) == 0 {
			continue
		}
		if err := queue.WriteBuffer(uniformBuffer, 0, aliasShadowUniformBytes(vpMatrix, cameraOrigin, aliasShadowAlpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update alias shadow uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(draw.vertices)); err != nil {
			slog.Warn("failed to upload alias shadow vertices", "error", err)
			continue
		}
		renderPass.Draw(uint32(len(draw.vertices)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish alias shadow encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit alias shadow commands", "error", err)
	}
}

func (r *Renderer) ensureAliasShadowSkinLocked(device hal.Device, queue hal.Queue) error {
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
				skin.bindGroup.Destroy()
			}
			if skin.fullbrightView != nil {
				skin.fullbrightView.Destroy()
			}
			if skin.fullbrightTexture != nil {
				skin.fullbrightTexture.Destroy()
			}
			if skin.view != nil {
				skin.view.Destroy()
			}
			if skin.texture != nil {
				skin.texture.Destroy()
			}
		}
		for _, variants := range cached.playerSkins {
			for _, skin := range variants {
				if skin.bindGroup != nil {
					skin.bindGroup.Destroy()
				}
				if skin.fullbrightView != nil {
					skin.fullbrightView.Destroy()
				}
				if skin.fullbrightTexture != nil {
					skin.fullbrightTexture.Destroy()
				}
				if skin.view != nil {
					skin.view.Destroy()
				}
				if skin.texture != nil {
					skin.texture.Destroy()
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
			r.aliasShadowSkin.bindGroup.Destroy()
		}
		if r.aliasShadowSkin.fullbrightView != nil {
			r.aliasShadowSkin.fullbrightView.Destroy()
		}
		if r.aliasShadowSkin.fullbrightTexture != nil {
			r.aliasShadowSkin.fullbrightTexture.Destroy()
		}
		if r.aliasShadowSkin.view != nil {
			r.aliasShadowSkin.view.Destroy()
		}
		if r.aliasShadowSkin.texture != nil {
			r.aliasShadowSkin.texture.Destroy()
		}
		r.aliasShadowSkin = nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Destroy()
		r.aliasScratchBuffer = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Destroy()
		r.aliasUniformBuffer = nil
	}
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Destroy()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasSampler != nil {
		r.aliasSampler.Destroy()
		r.aliasSampler = nil
	}
	if r.aliasPipeline != nil {
		r.aliasPipeline.Destroy()
		r.aliasPipeline = nil
	}
	if r.aliasShadowPipeline != nil {
		r.aliasShadowPipeline.Destroy()
		r.aliasShadowPipeline = nil
	}
	if r.aliasPipelineLayout != nil {
		r.aliasPipelineLayout.Destroy()
		r.aliasPipelineLayout = nil
	}
	if r.aliasVertexShader != nil {
		r.aliasVertexShader.Destroy()
		r.aliasVertexShader = nil
	}
	if r.aliasFragmentShader != nil {
		r.aliasFragmentShader.Destroy()
		r.aliasFragmentShader = nil
	}
	if r.aliasUniformBindGroupLayout != nil {
		r.aliasUniformBindGroupLayout.Destroy()
		r.aliasUniformBindGroupLayout = nil
	}
	if r.aliasTextureBindGroupLayout != nil {
		r.aliasTextureBindGroupLayout.Destroy()
		r.aliasTextureBindGroupLayout = nil
	}
}

func (r *Renderer) clearSpriteModelsLocked() {
	for key, cached := range r.spriteModels {
		for _, frame := range cached.frames {
			if frame.bindGroup != nil {
				frame.bindGroup.Destroy()
			}
			if frame.view != nil {
				frame.view.Destroy()
			}
			if frame.texture != nil {
				frame.texture.Destroy()
			}
		}
		delete(r.spriteModels, key)
	}
}

func (r *Renderer) destroySpriteResourcesLocked() {
	r.clearSpriteModelsLocked()
	if r.spriteUniformBuffer != nil {
		r.spriteUniformBuffer.Destroy()
		r.spriteUniformBuffer = nil
	}
	if r.spriteUniformBindGroup != nil {
		r.spriteUniformBindGroup.Destroy()
		r.spriteUniformBindGroup = nil
	}
	if r.spritePipeline != nil {
		r.spritePipeline.Destroy()
		r.spritePipeline = nil
	}
	if r.spriteVertexShader != nil {
		r.spriteVertexShader.Destroy()
		r.spriteVertexShader = nil
	}
	if r.spriteFragmentShader != nil {
		r.spriteFragmentShader.Destroy()
		r.spriteFragmentShader = nil
	}
}

func (r *Renderer) destroyDecalResourcesLocked() {
	if r.decalBindGroup != nil {
		r.decalBindGroup.Destroy()
		r.decalBindGroup = nil
	}
	if r.decalAtlasView != nil {
		r.decalAtlasView.Destroy()
		r.decalAtlasView = nil
	}
	if r.decalAtlasTextureHAL != nil {
		r.decalAtlasTextureHAL.Destroy()
		r.decalAtlasTextureHAL = nil
	}
	if r.decalUniformBuffer != nil {
		r.decalUniformBuffer.Destroy()
		r.decalUniformBuffer = nil
	}
	if r.decalUniformBindGroup != nil {
		r.decalUniformBindGroup.Destroy()
		r.decalUniformBindGroup = nil
	}
	if r.decalUniformLayout != nil {
		r.decalUniformLayout.Destroy()
		r.decalUniformLayout = nil
	}
	if r.decalPipelineLayout != nil {
		r.decalPipelineLayout.Destroy()
		r.decalPipelineLayout = nil
	}
	if r.decalPipeline != nil {
		r.decalPipeline.Destroy()
		r.decalPipeline = nil
	}
	if r.decalVertexShader != nil {
		r.decalVertexShader.Destroy()
		r.decalVertexShader = nil
	}
	if r.decalFragmentShader != nil {
		r.decalFragmentShader.Destroy()
		r.decalFragmentShader = nil
	}
}

// ---- merged from world_support_gogpu_root.go ----
type gpuAliasVertexRef struct {
	vertexIndex int
	texCoord    [2]float32
}

type gpuAliasSkin struct {
	texture           hal.Texture
	view              hal.TextureView
	fullbrightTexture hal.Texture
	fullbrightView    hal.TextureView
	bindGroup         hal.BindGroup
}

type gpuAliasModel struct {
	modelID     string
	flags       int
	skins       []gpuAliasSkin
	playerSkins map[uint32][]gpuAliasSkin
	poses       [][]model.TriVertX
	refs        []gpuAliasVertexRef
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
	texture   hal.Texture
	view      hal.TextureView
	bindGroup hal.BindGroup
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

func (r *Renderer) ensureAliasScratchBufferLocked(device hal.Device, size uint64) error {
	if size == 0 {
		size = 44
	}
	if r.aliasScratchBuffer != nil && r.aliasScratchBufferSize >= size {
		return nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Destroy()
		r.aliasScratchBuffer = nil
		r.aliasScratchBufferSize = 0
	}
	buffer, err := device.CreateBuffer(&hal.BufferDescriptor{
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

func aliasDepthAttachmentForView(view hal.TextureView) *hal.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &hal.RenderPassDepthStencilAttachment{
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
func destroyGoGPUTransientBuffers(buffers []hal.Buffer) {
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Destroy()
		}
	}
}

type gogpuTranslucentBrushFaceRender struct {
	bufferPair [2]hal.Buffer
	frame      int
	face       gogpuTranslucentLiquidFaceDraw
	liquid     bool
	center     [3]float32
	lightmaps  []*gpuWorldTexture
}

func sortGoGPUTranslucentBrushFaceRenders(mode AlphaMode, renders []gogpuTranslucentBrushFaceRender) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(renders, func(i, j int) bool {
		return renders[i].face.distanceSq > renders[j].face.distanceSq
	})
}
