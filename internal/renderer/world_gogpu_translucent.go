package renderer

import (
	"log/slog"
	"sort"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
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
		alphaTestPipeline:       r.worldAlphaTestPipeline,
		translucentPipeline:     r.worldTranslucentPipeline,
		liquidPipeline:          r.worldTranslucentTurbulentPipeline,
		uniformBuffer:           r.uniformBuffer,
		uniformBindGroup:        r.uniformBindGroup,
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
	if res.translucentPipeline == nil || res.liquidPipeline == nil || res.uniformBuffer == nil || res.uniformBindGroup == nil || res.uniformBindGroupLayout == nil || res.whiteTextureBindGroup == nil || res.whiteLightmapBindGroup == nil {
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
			bufferPair:   bufferPair,
			vertexOffset: vertexOffset,
			indexOffset:  indexOffset,
			frame:        draw.frame,
			face:         face,
			liquid:       true,
			hasLitWater:  hasLitWater,
			lightmaps:    draw.lightmaps,
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
			center:    center,
			lightmaps: draw.lightmaps,
		})
	}
	for _, face := range draw.translucentFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair:   bufferPair,
			vertexOffset: vertexOffset,
			indexOffset:  indexOffset,
			frame:        draw.frame,
			face:         face,
			lightmaps:    draw.lightmaps,
		})
	}
	for _, face := range draw.liquidFaces {
		translucentDst = append(translucentDst, gogpuTranslucentBrushFaceRender{
			bufferPair:   bufferPair,
			vertexOffset: vertexOffset,
			indexOffset:  indexOffset,
			frame:        draw.frame,
			face:         face,
			liquid:       true,
			hasLitWater:  hasLitWater,
			lightmaps:    draw.lightmaps,
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

func gogpuWorldTranslucentLiquidFaceRenders(
	faces []WorldFace,
	camera CameraState,
	worldVertexBuffer *wgpu.Buffer,
	worldIndexBuffer *wgpu.Buffer,
	worldLightmapPages []*gpuWorldTexture,
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
			liquid:      true,
			hasLitWater: worldHasLitWater,
			lightmaps:   worldLightmapPages,
		})
	}
	return renders
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
	var activeDynamicLights []DynamicLight
	if r.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, r.lightPool.ActiveLights()...)
	}
	cameraOriginWorld := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraLeafIndex := -1
	if worldData != nil && worldData.Geometry != nil && worldData.Geometry.Tree != nil {
		cameraLeafIndex = worldLeafIndex(worldData.Geometry.Tree, cameraOriginWorld)
	}
	dynamicLightSig := gogpuWorldDynamicLightSignature(activeDynamicLights)
	var cachedFaces []WorldFace
	if worldData != nil && worldData.Geometry != nil {
		if cacheEntry := r.gogpuWorldBatchCacheEntry(cameraLeafIndex, dynamicLightSig); cacheEntry != nil {
			cachedFaces = cacheEntry.translucentLiquid
		}
	}
	worldHasLitWater := worldData != nil && worldData.Geometry != nil && worldData.Geometry.HasLitWater
	r.mu.RUnlock()
	if worldData == nil || worldData.Geometry == nil || worldVertexBuffer == nil || worldIndexBuffer == nil {
		return nil
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	if cachedFaces != nil {
		return gogpuWorldTranslucentLiquidFaceRenders(cachedFaces, camera, worldVertexBuffer, worldIndexBuffer, worldLightmapPages, liquidAlpha, worldHasLitWater)
	}
	visibleFaces := r.worldVisibleFacesScratch.selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		cameraOriginWorld,
	)
	translucentFaces := make([]WorldFace, 0, len(visibleFaces))
	for _, face := range visibleFaces {
		if shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha) {
			translucentFaces = append(translucentFaces, face)
		}
	}
	return gogpuWorldTranslucentLiquidFaceRenders(translucentFaces, camera, worldVertexBuffer, worldIndexBuffer, worldLightmapPages, liquidAlpha, worldHasLitWater)
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
		geom := dc.renderer.ensureBrushModelGeometry(entity.SubmodelIndex)
		if draw := buildGoGPUTranslucentBrushEntityDraw(entity, geom, state.liquidAlpha, state.camera); draw != nil {
			draw.lightmaps = dc.renderer.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
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
	renderPass.SetBindGroup(0, res.uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	var uniformData [worldUniformBufferSize]byte
	var materialBindState gogpuWorldMaterialBindState
	for _, draw := range renders {
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.center))
		fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, fogDensity, timeValue, draw.face.alpha, dynamicLight, 0)
		if err := res.queue.WriteBuffer(res.uniformBuffer, 0, uniformData[:]); err != nil {
			slog.Warn("failed to update alpha-test brush uniform buffer", "error", err)
			continue
		}
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
	renderPass.SetBindGroup(0, res.uniformBindGroup, nil)

	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	cameraOrigin, _, timeValue := gogpuWorldUniformInputs(&RenderFrameState{FogDensity: fogDensity}, res.camera)
	timeSeconds := float64(timeValue)
	var uniformData [worldUniformBufferSize]byte
	currentPipeline := res.translucentPipeline
	var materialBindState gogpuWorldMaterialBindState
	for _, draw := range renders {
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(res.activeDynamicLights, draw.face.center))
		lightmapBindGroup, litWater := gogpuLateTranslucentLightmapBindGroup(res, draw)
		fillWorldSceneUniformBytes(uniformData[:], vpMatrix, cameraOrigin, fogColor, fogDensity, timeValue, draw.face.alpha, dynamicLight, litWater)
		if err := res.queue.WriteBuffer(res.uniformBuffer, 0, uniformData[:]); err != nil {
			slog.Warn("failed to update late translucent uniform buffer", "error", err)
			continue
		}
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
	if _, err := res.queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit late translucent commands", "error", err)
	}
}

// ---- merged from world_alias_shadow_gogpu_root.go ----

type gogpuTranslucentBrushFaceRender struct {
	bufferPair   [2]*wgpu.Buffer
	vertexOffset uint64
	indexOffset  uint64
	frame        int
	face         gogpuTranslucentLiquidFaceDraw
	liquid       bool
	hasLitWater  bool
	center       [3]float32
	lightmaps    []*gpuWorldTexture
}

func sortGoGPUTranslucentBrushFaceRenders(mode AlphaMode, renders []gogpuTranslucentBrushFaceRender) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(renders, func(i, j int) bool {
		return renders[i].face.distanceSq > renders[j].face.distanceSq
	})
}
