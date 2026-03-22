//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"sort"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

type gogpuOpaqueBrushEntityDraw struct {
	alpha     float32
	vertices  []WorldVertex
	indices   []uint32
	faces     []WorldFace
	centers   [][3]float32
	lightmaps []*gpuWorldTexture
}

func shouldDrawGoGPUOpaqueBrushFace(face WorldFace, entityAlpha float32) bool {
	return isFullyOpaqueAlpha(clamp01(entityAlpha)) && shouldDrawGoGPUOpaqueWorldFace(face)
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
	if geom == nil || len(geom.Vertices) == 0 || len(geom.Indices) == 0 || len(geom.Faces) == 0 {
		return nil
	}
	alpha := clamp01(entity.Alpha)
	if alpha <= 0 {
		return nil
	}
	rotation := buildBrushRotationMatrix(entity.Angles)
	vertices := make([]WorldVertex, len(geom.Vertices))
	for i, vertex := range geom.Vertices {
		vertices[i] = vertex
		vertices[i].Position = transformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
	}
	faces := make([]WorldFace, 0, len(geom.Faces))
	centers := make([][3]float32, 0, len(geom.Faces))
	indices := make([]uint32, 0, len(geom.Indices))
	for _, face := range geom.Faces {
		if !includeFace(face, alpha) {
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
		faces = append(faces, drawFace)
		centers = append(centers, transformModelSpacePoint(face.Center, entity.Origin, rotation, entity.Scale))
		indices = append(indices, geom.Indices[first:last]...)
	}
	if len(faces) == 0 || len(indices) == 0 {
		return nil
	}
	return &gogpuOpaqueBrushEntityDraw{
		alpha:    alpha,
		vertices: vertices,
		indices:  indices,
		faces:    faces,
		centers:  centers,
	}
}

func buildGoGPUOpaqueBrushEntityDraw(entity BrushEntity, geom *WorldGeometry) *gogpuOpaqueBrushEntityDraw {
	return buildGoGPUBrushEntityDraw(entity, geom, shouldDrawGoGPUOpaqueBrushFace)
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
		vertices: vertices,
		indices:  indices,
		faces:    faces,
	}
}

type gogpuTranslucentBrushEntityDraw struct {
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
		vertices:         vertices,
		indices:          indices,
		alphaTestFaces:   alphaTestFaces,
		alphaTestCenters: alphaTestCenters,
		translucentFaces: translucentFaces,
		liquidFaces:      liquidFaces,
	}
}

func worldVertexBytes(vertices []WorldVertex) []byte {
	data := make([]byte, len(vertices)*44)
	for i, v := range vertices {
		offset := i * 44
		copy(data[offset:offset+12], float32ToBytes(v.Position[:]))
		copy(data[offset+12:offset+20], float32ToBytes(v.TexCoord[:]))
		copy(data[offset+20:offset+28], float32ToBytes(v.LightmapCoord[:]))
		copy(data[offset+28:offset+40], float32ToBytes(v.Normal[:]))
	}
	return data
}

func worldIndexBytes(indices []uint32) []byte {
	data := make([]byte, len(indices)*4)
	for i, idx := range indices {
		binary.LittleEndian.PutUint32(data[i*4:], idx)
	}
	return data
}

func createGoGPUBrushBuffer(device hal.Device, label string, usage gputypes.BufferUsage, data []byte) (hal.Buffer, error) {
	if device == nil || len(data) == 0 {
		return nil, fmt.Errorf("invalid brush buffer upload")
	}
	buffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            label,
		Size:             uint64(len(data)),
		Usage:            usage | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, err
	}
	return buffer, nil
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

	draws := make([]gogpuOpaqueBrushEntityDraw, 0, len(entities))
	for _, entity := range entities {
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUOpaqueBrushEntityDraw(entity, geom); draw != nil {
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
		vertexData := worldVertexBytes(draw.vertices)
		indexData := worldIndexBytes(draw.indices)
		vertexBuffer, err := createGoGPUBrushBuffer(device, "Brush Entity Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := createGoGPUBrushBuffer(device, "Brush Entity Indices", gputypes.BufferUsageIndex, indexData)
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
			if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
			if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
	externalSkyMode := r.worldSkyExternalMode
	externalSkyBindGroup := r.worldSkyExternalBindGroup
	depthView := r.worldDepthTextureView
	camera := r.cameraState
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
		vertexData := worldVertexBytes(draw.vertices)
		indexData := worldIndexBytes(draw.indices)
		vertexBuffer, err := createGoGPUBrushBuffer(device, "Brush Sky Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush sky vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush sky vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := createGoGPUBrushBuffer(device, "Brush Sky Indices", gputypes.BufferUsageIndex, indexData)
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
		if err := queue.WriteBuffer(uniformBuffer, 0, worldSceneUniformBytes(vpMatrix, cameraOrigin, fogColor, fogDensity, camera.Time, 1, [3]float32{}, 0)); err != nil {
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
			solidBindGroup := whiteTextureBindGroup
			if worldTexture := worldSkySolidTextures[face.TextureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				solidBindGroup = worldTexture.bindGroup
			}
			alphaBindGroup := transparentBindGroup
			if worldTexture := worldSkyAlphaTextures[face.TextureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
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
		vertexData := worldVertexBytes(draw.vertices)
		indexData := worldIndexBytes(draw.indices)
		vertexBuffer, err := createGoGPUBrushBuffer(device, "Brush Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := createGoGPUBrushBuffer(device, "Brush Liquid Indices", gputypes.BufferUsageIndex, indexData)
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
			if worldTexture := gogpuWorldTextureForFace(face, worldTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
			if worldTexture := gogpuWorldTextureForFace(face, worldFullbrightTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
		vertexData := worldVertexBytes(draw.vertices)
		indexData := worldIndexBytes(draw.indices)
		vertexBuffer, err := createGoGPUBrushBuffer(device, "Brush Translucent Liquid Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush liquid vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush liquid vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := createGoGPUBrushBuffer(device, "Brush Translucent Liquid Indices", gputypes.BufferUsageIndex, indexData)
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
				face:       face,
				lightmaps:  draw.lightmaps,
			})
		}
	}
	sortGoGPUTranslucentLiquidBrushFaceRenders(GetAlphaMode(), faceRenders)
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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

type gogpuTranslucentBrushFaceRender struct {
	bufferPair [2]hal.Buffer
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
		vertexData := worldVertexBytes(draw.vertices)
		indexData := worldIndexBytes(draw.indices)
		vertexBuffer, err := createGoGPUBrushBuffer(device, "Brush Translucent Vertices", gputypes.BufferUsageVertex, vertexData)
		if err != nil {
			slog.Warn("failed to create translucent brush vertex buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(vertexBuffer, 0, vertexData); err != nil {
			vertexBuffer.Destroy()
			slog.Warn("failed to upload translucent brush vertex buffer", "error", err)
			continue
		}
		indexBuffer, err := createGoGPUBrushBuffer(device, "Brush Translucent Indices", gputypes.BufferUsageIndex, indexData)
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
				face:       face,
				lightmaps:  draw.lightmaps,
			})
		}
		for _, face := range draw.liquidFaces {
			translucentRenders = append(translucentRenders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.face.NumIndices, 1, draw.face.face.FirstIndex, 0, 0)
	}

	sortGoGPUTranslucentBrushFaceRenders(GetAlphaMode(), translucentRenders)
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, worldFullbrightTextures, worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
