package renderer

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

type gogpuLateTranslucentFaceResources struct {
	device                  *wgpu.Device
	queue                   *wgpu.Queue
	textureView             *wgpu.TextureView
	alphaTestPipeline       *wgpu.RenderPipeline
	translucentPipeline     *wgpu.RenderPipeline
	liquidPipeline          *wgpu.RenderPipeline
	uniformBuffer           *wgpu.Buffer
	uniformBindGroup        *wgpu.BindGroup
	uniformBindGroupLayout  *wgpu.BindGroupLayout
	dynamicLightsBuffer     *wgpu.Buffer
	dynamicLightsBindGroup  *wgpu.BindGroup
	whiteTextureBindGroup   *wgpu.BindGroup
	whiteLightmapBindGroup  *wgpu.BindGroup
	transparentBindGroup    *wgpu.BindGroup
	depthView               *wgpu.TextureView
	camera                  CameraState
	worldTextures           *gpuWorldTexture
	worldFullbrightTextures *gpuWorldTexture
	worldTextureAnimations  []*surfacepkg.SurfaceTexture
	worldLightmapArray      *gpuWorldTexture
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
		alphaTestPipeline:       r.resources.WorldAlphaTestPipeline,
		translucentPipeline:     r.resources.WorldTranslucentPipeline,
		liquidPipeline:          r.resources.WorldTranslucentTurbulentPipeline,
		uniformBuffer:           r.resources.UniformBuffer,
		uniformBindGroup:        r.resources.UniformBindGroup,
		uniformBindGroupLayout:  r.resources.UniformBindGroupLayout,
		dynamicLightsBuffer:     r.resources.WorldDynamicLightsBuffer,
		dynamicLightsBindGroup:  r.resources.WorldDynamicLightsBindGroup,
		whiteTextureBindGroup:   r.resources.WhiteTextureBindGroup,
		whiteLightmapBindGroup:  r.resources.WhiteLightmapBindGroup,
		transparentBindGroup:    r.resources.TransparentBindGroup,
		depthView:               r.resources.WorldDepthTextureView,
		camera:                  r.cameraState,
		worldTextures:           r.worldTextures,
		worldFullbrightTextures: r.worldFullbrightTextures,
		worldTextureAnimations:  r.worldTextureAnimations,
		worldLightmapArray:      r.worldLightmapArray,
		unlock:                  r.mu.RUnlock,
	}
	if r.lightPool != nil {
		res.activeDynamicLights = r.lightPool.ActiveLights()
	}
	if res.translucentPipeline == nil || res.liquidPipeline == nil || res.uniformBuffer == nil || res.uniformBindGroup == nil || res.uniformBindGroupLayout == nil || res.dynamicLightsBuffer == nil || res.dynamicLightsBindGroup == nil || res.whiteTextureBindGroup == nil || res.whiteLightmapBindGroup == nil {
		res.unlock()
		return gogpuLateTranslucentFaceResources{}, false
	}
	if res.transparentBindGroup == nil {
		res.transparentBindGroup = res.whiteTextureBindGroup
	}
	return res, true
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
		liquidAlpha: worldLiquidAlphaSettingsForGeometry(r.worldData.Geometry),
	}, true
}

func createGoGPUTranslucentBrushBuffers(device *wgpu.Device, queue *wgpu.Queue, vertexLabel, indexLabel string, vertexData, indexData []byte) ([2]*wgpu.Buffer, []*wgpu.Buffer, bool) {
	if device == nil || queue == nil || len(vertexData) == 0 || len(indexData) == 0 {
		return [2]*wgpu.Buffer{}, nil, false
	}

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

func appendGoGPUTranslucentLiquidBrushFaceRenders(dst []gogpuTranslucentBrushFaceRender, bufferPair [2]*wgpu.Buffer, vertexOffset, indexOffset uint64, draw gogpuTranslucentLiquidBrushEntityDraw) []gogpuTranslucentBrushFaceRender {
	hasLitWater := gogpuTranslucentFacesHaveLitWater(draw.faces)
	for _, face := range draw.faces {
		dst = append(dst, gogpuTranslucentBrushFaceRender{
			bufferPair:             bufferPair,
			vertexOffset:           vertexOffset,
			indexOffset:            indexOffset,
			frame:                  draw.frame,
			face:                   face,
			liquid:                 true,
			hasLitWater:            hasLitWater,
			lightmapArray:          draw.lightmapArray,
			textures:               draw.textures,
			fullbrightTextures:     draw.fullbrightTextures,
			textureAnimations:      draw.textureAnimations,
			uniformBindGroup:       draw.uniformBindGroup,
			uniformBindGroupFrame1: draw.uniformBindGroupFrame1,
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

func appendGoGPUTranslucentBrushEntityFaceRenders(alphaTestDst, translucentDst []gogpuTranslucentBrushFaceRender, bufferPair [2]*wgpu.Buffer, vertexOffset, indexOffset uint64, draw gogpuTranslucentBrushEntityDraw) ([]gogpuTranslucentBrushFaceRender, []gogpuTranslucentBrushFaceRender) {
	hasLitWater := gogpuTranslucentFacesHaveLitWater(draw.liquidFaces)
	for faceIndex, face := range draw.alphaTestFaces {
		center := [3]float32{}
		if faceIndex < len(draw.alphaTestCenters) {
			center = draw.alphaTestCenters[faceIndex]
		}
		alphaTestDst = append(alphaTestDst, gogpuTranslucentBrushFaceRender{
			bufferPair:   bufferPair,
			vertexOffset: vertexOffset,
			indexOffset:  indexOffset,
			frame:        draw.frame,
			face: gogpuTranslucentLiquidFaceDraw{
				face:  face,
				alpha: 1,
			},
			center:                 center,
			lightmapArray:          draw.lightmapArray,
			textures:               draw.textures,
			fullbrightTextures:     draw.fullbrightTextures,
			textureAnimations:      draw.textureAnimations,
			uniformBindGroup:       draw.uniformBindGroup,
			uniformBindGroupFrame1: draw.uniformBindGroupFrame1,
		})
	}
	for _, face := range draw.translucentFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair:             bufferPair,
			vertexOffset:           vertexOffset,
			indexOffset:            indexOffset,
			frame:                  draw.frame,
			face:                   face,
			lightmapArray:          draw.lightmapArray,
			textures:               draw.textures,
			fullbrightTextures:     draw.fullbrightTextures,
			textureAnimations:      draw.textureAnimations,
			uniformBindGroup:       draw.uniformBindGroup,
			uniformBindGroupFrame1: draw.uniformBindGroupFrame1,
		})
	}
	for _, face := range draw.liquidFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair:             bufferPair,
			vertexOffset:           vertexOffset,
			indexOffset:            indexOffset,
			frame:                  draw.frame,
			face:                   face,
			liquid:                 true,
			hasLitWater:            hasLitWater,
			lightmapArray:          draw.lightmapArray,
			textures:               draw.textures,
			fullbrightTextures:     draw.fullbrightTextures,
			textureAnimations:      draw.textureAnimations,
			uniformBindGroup:       draw.uniformBindGroup,
			uniformBindGroupFrame1: draw.uniformBindGroupFrame1,
		})
	}
	return alphaTestDst, translucentDst
}

func gogpuLateTranslucentTextureBindGroups(res gogpuLateTranslucentFaceResources, draw gogpuTranslucentBrushFaceRender, timeSeconds float64) (*wgpu.BindGroup, *wgpu.BindGroup) {
	textures := res.worldTextures
	fullbright := res.worldFullbrightTextures

	if draw.textures != nil {
		textures = draw.textures
	}
	if draw.fullbrightTextures != nil {
		fullbright = draw.fullbrightTextures
	}
	textureBindGroup := res.whiteTextureBindGroup
	if textures != nil && textures.bindGroup != nil {
		textureBindGroup = textures.bindGroup
	}

	fullbrightBindGroup := res.transparentBindGroup
	if fullbright != nil && fullbright.bindGroup != nil {
		fullbrightBindGroup = fullbright.bindGroup
	}

	return textureBindGroup, fullbrightBindGroup
}

func gogpuLateTranslucentLightmapBindGroup(res gogpuLateTranslucentFaceResources, draw gogpuTranslucentBrushFaceRender) (*wgpu.BindGroup, float32) {
	if draw.liquid {
		lightmapBindGroup, litWater := gogpuWorldLightmapArrayBindGroupForFace(draw.face.face, draw.lightmapArray, res.whiteLightmapBindGroup, draw.hasLitWater)
		if lightmapBindGroup == res.whiteLightmapBindGroup {
			lightmapBindGroup, litWater = gogpuWorldLightmapArrayBindGroupForFace(draw.face.face, res.worldLightmapArray, res.whiteLightmapBindGroup, draw.hasLitWater)
		}
		return lightmapBindGroup, litWater
	}

	lightmapBindGroup := res.whiteLightmapBindGroup
	if draw.face.face.LightmapIndex >= 0 {
		if draw.lightmapArray != nil && draw.lightmapArray.bindGroup != nil {
			lightmapBindGroup = draw.lightmapArray.bindGroup
		} else if res.worldLightmapArray != nil && res.worldLightmapArray.bindGroup != nil {
			lightmapBindGroup = res.worldLightmapArray.bindGroup
		}
	}
	return lightmapBindGroup, 0
}

func gogpuWorldTranslucentLiquidFaceRenders(
	faces []WorldFace,
	camera CameraState,
	worldVertexBuffer *wgpu.Buffer,
	worldIndexBuffer *wgpu.Buffer,
	worldLightmapArray *gpuWorldTexture,
	liquidAlpha worldLiquidAlphaSettings,
	worldHasLitWater bool,
) []gogpuTranslucentBrushFaceRender {
	if len(faces) == 0 || worldVertexBuffer == nil || worldIndexBuffer == nil {
		return nil
	}
	renders := make([]gogpuTranslucentBrushFaceRender, 0, len(faces))
	for _, face := range faces {
		renders = append(renders, gogpuTranslucentBrushFaceRender{
			bufferPair: [2]*wgpu.Buffer{worldVertexBuffer, worldIndexBuffer},
			face: gogpuTranslucentLiquidFaceDraw{
				face:       face,
				alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
				center:     face.Center,
				distanceSq: worldFaceDistanceSq(face.Center, camera),
			},
			liquid:        true,
			hasLitWater:   worldHasLitWater,
			lightmapArray: worldLightmapArray,
		})
	}
	return renders
}

func (dc *DrawContext) collectGoGPUWorldTranslucentLiquidFaceRenders() []gogpuTranslucentBrushFaceRender {
	return nil
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
		geom := dc.renderer.brushEntityGeometry(entity)
		if draw := buildGoGPUTranslucentLiquidBrushEntityDraw(entity, geom, state.liquidAlpha, state.camera); draw != nil {
			draw.lightmapArray = dc.renderer.brushEntityLightmaps(entity, geom)
			draw.textures, draw.fullbrightTextures, draw.textureAnimations, draw.uniformBindGroup = dc.renderer.brushEntityTextures(entity)
			draw.uniformBindGroupFrame1 = dc.renderer.brushEntityUniformBindGroupFrame1(entity)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil
	}

	type preparedTranslucentBrushDraw struct {
		draw         gogpuTranslucentLiquidBrushEntityDraw
		vertexData   []byte
		indexData    []byte
		vertexOffset uint64
		indexOffset  uint64
	}
	prepared := make([]preparedTranslucentBrushDraw, 0, len(draws))
	totalVertexBytes := uint64(0)
	totalIndexBytes := uint64(0)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		prepared = append(prepared, preparedTranslucentBrushDraw{
			draw:         draw,
			vertexData:   vertexData,
			indexData:    indexData,
			vertexOffset: totalVertexBytes,
			indexOffset:  totalIndexBytes,
		})
		totalVertexBytes += uint64(len(vertexData))
		totalIndexBytes += uint64(len(indexData))
	}
	if len(prepared) == 0 {
		return nil, nil
	}
	combinedVertexData := make([]byte, int(totalVertexBytes))
	combinedIndexData := make([]byte, int(totalIndexBytes))
	for _, preparedDraw := range prepared {
		copy(combinedVertexData[preparedDraw.vertexOffset:], preparedDraw.vertexData)
		copy(combinedIndexData[preparedDraw.indexOffset:], preparedDraw.indexData)
	}
	bufferPair, owned, ok := createGoGPUTranslucentBrushBuffers(state.device, state.queue, "Brush Translucent Liquid Vertices", "Brush Translucent Liquid Indices", combinedVertexData, combinedIndexData)
	if !ok {
		return nil, nil
	}
	renders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	for _, preparedDraw := range prepared {
		renders = appendGoGPUTranslucentLiquidBrushFaceRenders(renders, bufferPair, preparedDraw.vertexOffset, preparedDraw.indexOffset, preparedDraw.draw)
	}
	return renders, owned
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
		geom := dc.renderer.brushEntityGeometry(entity)
		if draw := buildGoGPUTranslucentBrushEntityDraw(entity, geom, state.liquidAlpha, state.camera); draw != nil {
			draw.lightmapArray = dc.renderer.brushEntityLightmaps(entity, geom)
			draw.textures, draw.fullbrightTextures, draw.textureAnimations, draw.uniformBindGroup = dc.renderer.brushEntityTextures(entity)
			draw.uniformBindGroupFrame1 = dc.renderer.brushEntityUniformBindGroupFrame1(entity)
			draws = append(draws, *draw)
		}
	}
	if len(draws) == 0 {
		return nil, nil, nil
	}

	type preparedTranslucentBrushDraw struct {
		draw         gogpuTranslucentBrushEntityDraw
		vertexData   []byte
		indexData    []byte
		vertexOffset uint64
		indexOffset  uint64
	}
	prepared := make([]preparedTranslucentBrushDraw, 0, len(draws))
	totalVertexBytes := uint64(0)
	totalIndexBytes := uint64(0)
	for _, draw := range draws {
		vertexData := worldgogpu.VertexBytes(draw.vertices)
		indexData := worldgogpu.IndexBytes(draw.indices)
		prepared = append(prepared, preparedTranslucentBrushDraw{
			draw:         draw,
			vertexData:   vertexData,
			indexData:    indexData,
			vertexOffset: totalVertexBytes,
			indexOffset:  totalIndexBytes,
		})
		totalVertexBytes += uint64(len(vertexData))
		totalIndexBytes += uint64(len(indexData))
	}
	if len(prepared) == 0 {
		return nil, nil, nil
	}
	combinedVertexData := make([]byte, int(totalVertexBytes))
	combinedIndexData := make([]byte, int(totalIndexBytes))
	for _, preparedDraw := range prepared {
		copy(combinedVertexData[preparedDraw.vertexOffset:], preparedDraw.vertexData)
		copy(combinedIndexData[preparedDraw.indexOffset:], preparedDraw.indexData)
	}
	bufferPair, owned, ok := createGoGPUTranslucentBrushBuffers(state.device, state.queue, "Brush Translucent Vertices", "Brush Translucent Indices", combinedVertexData, combinedIndexData)
	if !ok {
		return nil, nil, nil
	}
	alphaTestRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws))
	translucentRenders := make([]gogpuTranslucentBrushFaceRender, 0, len(draws)*2)
	for _, preparedDraw := range prepared {
		alphaTestRenders, translucentRenders = appendGoGPUTranslucentBrushEntityFaceRenders(alphaTestRenders, translucentRenders, bufferPair, preparedDraw.vertexOffset, preparedDraw.indexOffset, preparedDraw.draw)
	}
	return alphaTestRenders, translucentRenders, owned
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
	passStartUniformOffset := dc.renderer.uniformOffset
	ptr, lightData := encodeGoGPUWorldDynamicLights(res.activeDynamicLights)
	err = res.queue.WriteBuffer(res.dynamicLightsBuffer, 0, lightData)
	dynamicLightsBytesPool.Put(ptr)
	if err != nil {
		slog.Warn("failed to upload alpha-test brush dynamic lights", "error", err)
		return
	}
	renderPass.SetBindGroup(4, res.dynamicLightsBindGroup, nil)

	vpMatrix := dc.renderer.ViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	var materialBindState gogpuWorldMaterialBindState
	for _, draw := range renders {
		offset, uData := dc.renderer.allocateUniformBuffer(worldUniformBufferSize)
		fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), timeValue, draw.face.alpha, 0)

		// Select the frame-1 uniform bind group when the entity's frame != 0.
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
		lightmapBindGroup, _ := gogpuLateTranslucentLightmapBindGroup(res, draw)
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
	if err := renderPass.End(); err != nil {
		slog.Warn("renderGoGPUAlphaTestBrushFaceRendersHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alpha-test brush encoding", "error", err)
		return
	}
	if dc.renderer.uniformOffset > passStartUniformOffset {
		_ = res.queue.WriteBuffer(res.uniformBuffer, uint64(passStartUniformOffset), dc.renderer.uniformDataScratch[passStartUniformOffset:dc.renderer.uniformOffset])
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
	width, height := dc.renderer.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	// GoGPU's Vulkan backend resolves descriptor-set binding through the active
	// pipeline layout, so the sorted late-translucent pass must select a pipeline
	// before its first SetBindGroup call.
	renderPass.SetPipeline(res.translucentPipeline)
	passStartUniformOffset := dc.renderer.uniformOffset
	ptr, lightData := encodeGoGPUWorldDynamicLights(res.activeDynamicLights)
	err = res.queue.WriteBuffer(res.dynamicLightsBuffer, 0, lightData)
	dynamicLightsBytesPool.Put(ptr)
	if err != nil {
		slog.Warn("failed to upload translucent brush dynamic lights", "error", err)
		return
	}
	renderPass.SetBindGroup(4, res.dynamicLightsBindGroup, nil)

	vpMatrix := dc.renderer.ViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	currentPipeline := res.translucentPipeline
	var materialBindState gogpuWorldMaterialBindState
	for _, draw := range renders {
		lightmapBindGroup, litWater := gogpuLateTranslucentLightmapBindGroup(res, draw)
		offset, uData := dc.renderer.allocateUniformBuffer(worldUniformBufferSize)
		fillWorldSceneUniformBytes(uData, vpMatrix, cameraOrigin, fogColor, worldFogUniformDensity(fogDensity), res.camera.Time, draw.face.alpha, litWater)

		// Select the frame-1 uniform bind group when the entity's frame != 0.
		activeUniformBindGroup := res.uniformBindGroup
		if draw.frame != 0 && draw.uniformBindGroupFrame1 != nil {
			activeUniformBindGroup = draw.uniformBindGroupFrame1
		} else if draw.uniformBindGroup != nil {
			activeUniformBindGroup = draw.uniformBindGroup
		}

		renderPass.SetBindGroup(0, activeUniformBindGroup, []uint32{offset})
		pipeline := res.translucentPipeline
		if draw.liquid {
			pipeline = res.liquidPipeline
		}
		if pipeline != currentPipeline {
			renderPass.SetPipeline(pipeline)
			currentPipeline = pipeline
			materialBindState = gogpuWorldMaterialBindState{}
		}
		renderPass.SetVertexBuffer(0, draw.bufferPair[0], draw.vertexOffset)
		renderPass.SetIndexBuffer(draw.bufferPair[1], gputypes.IndexFormatUint32, draw.indexOffset)
		textureBindGroup, fullbrightBindGroup := gogpuLateTranslucentTextureBindGroups(res, draw, timeSeconds)
		if rDebugWaterEnabled() && draw.liquid {
			slog.Debug("[rwater] late translucent gpu draw",
				"face_idx", draw.face.face.FirstIndex,
				"num_indices", draw.face.face.NumIndices,
				"alpha", draw.face.alpha,
				"lit_water", litWater,
				"has_lit_water", draw.hasLitWater,
				"dynamic_uniform_offset", offset,
				"uniform_alpha_bytes", fmt.Sprintf("%x", uData[96:100]),
				"uniform_litwater_bytes", fmt.Sprintf("%x", uData[100:104]),
				"pipeline_ptr", fmt.Sprintf("%p", pipeline),
				"liquid_pipeline_ptr", fmt.Sprintf("%p", res.liquidPipeline),
				"translucent_pipeline_ptr", fmt.Sprintf("%p", res.translucentPipeline),
				"texture_bg", fmt.Sprintf("%p", textureBindGroup),
				"lightmap_bg", fmt.Sprintf("%p", lightmapBindGroup),
			)
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
	if dc.renderer.uniformOffset > passStartUniformOffset {
		_ = res.queue.WriteBuffer(res.uniformBuffer, uint64(passStartUniformOffset), dc.renderer.uniformDataScratch[passStartUniformOffset:dc.renderer.uniformOffset])
	}
	if _, err := res.queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit late translucent commands", "error", err)
	}
}

// ---- merged from world_alias_shadow_gogpu_root.go ----

type gogpuTranslucentBrushFaceRender struct {
	bufferPair             [2]*wgpu.Buffer
	vertexOffset           uint64
	indexOffset            uint64
	frame                  int
	face                   gogpuTranslucentLiquidFaceDraw
	liquid                 bool
	hasLitWater            bool
	center                 [3]float32
	lightmapArray          *gpuWorldTexture
	textures               *gpuWorldTexture
	fullbrightTextures     *gpuWorldTexture
	textureAnimations      []*surfacepkg.SurfaceTexture
	uniformBindGroup       *wgpu.BindGroup
	uniformBindGroupFrame1 *wgpu.BindGroup
}

func sortGoGPUTranslucentBrushFaceRenders(mode AlphaMode, renders []gogpuTranslucentBrushFaceRender) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(renders, func(i, j int) bool {
		return renders[i].face.distanceSq > renders[j].face.distanceSq
	})
}
