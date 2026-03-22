//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"log/slog"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
)

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

func destroyGoGPUTransientBuffers(buffers []hal.Buffer) {
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Destroy()
		}
	}
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
			renders = append(renders, gogpuTranslucentBrushFaceRender{
				bufferPair: [2]hal.Buffer{vertexBuffer, indexBuffer},
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldTextures, res.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldFullbrightTextures, res.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldTextures, res.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := res.transparentBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face.face, res.worldFullbrightTextures, res.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
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
