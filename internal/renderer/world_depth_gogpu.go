package renderer

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// TransformVertex applies model-view-projection transformation to a vertex.
// This is a helper for software rendering fallback.
func TransformVertex(pos [3]float32, mvp types.Mat4) types.Vec4 {
	v := types.Vec4{X: pos[0], Y: pos[1], Z: pos[2], W: 1.0}
	return types.Mat4MulVec4(mvp, v)
}

// createWorldDepthTexture allocates a depth attachment used by multi-pass world rendering so later passes can depth-test against the opaque world.
func (r *Renderer) createWorldDepthTexture(device *wgpu.Device, width, height int) (*wgpu.Texture, *wgpu.TextureView, error) {
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Depth Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        worldDepthTextureFormat,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create depth texture: %w", err)
	}

	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "World Depth Texture View",
		Format:          worldDepthTextureFormat,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create depth texture view: %w", err)
	}

	return texture, view, nil
}

func (dc *DrawContext) renderWorldTranslucentLiquidsHAL(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}

	dc.renderer.mu.RLock()
	worldData := dc.renderer.worldData
	textureView := dc.currentWGPURenderTargetView()
	depthView := dc.renderer.worldDepthTextureView
	uniformBuffer := dc.renderer.uniformBuffer
	uniformBindGroup := dc.renderer.uniformBindGroup
	translucentPipeline := dc.renderer.worldTranslucentTurbulentPipeline
	vertexBuffer := dc.renderer.worldVertexBuffer
	indexBuffer := dc.renderer.worldIndexBuffer
	worldTextures := dc.renderer.worldTextures
	worldFullbrightTextures := dc.renderer.worldFullbrightTextures
	worldTextureAnimations := dc.renderer.worldTextureAnimations
	worldLightmapPages := dc.renderer.worldLightmapPages
	whiteTextureBindGroup := dc.renderer.whiteTextureBindGroup
	transparentBindGroup := dc.renderer.transparentBindGroup
	whiteLightmapBindGroup := dc.renderer.whiteLightmapBindGroup
	var activeDynamicLights []DynamicLight
	if dc.renderer.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, dc.renderer.lightPool.ActiveLights()...)
	}
	dc.renderer.mu.RUnlock()

	if worldData == nil || textureView == nil || uniformBuffer == nil || uniformBindGroup == nil || translucentPipeline == nil || vertexBuffer == nil || indexBuffer == nil {
		return
	}

	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	worldHasLitWater := worldData.Geometry.HasLitWater
	if !hasTranslucentWorldLiquidFaceType(worldData.Geometry.LiquidFaceTypes, liquidAlpha) {
		return
	}

	renderPassDescriptor := &wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       textureView,
			LoadOp:     gputypes.LoadOpLoad,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{},
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	}
	encoder, err := device.CreateCommandEncoder(nil)
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to create command encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(renderPassDescriptor)
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: Failed to begin render pass", "error", err)
		return
	}
	w, h := dc.renderer.Size()
	renderPass.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	renderPass.SetPipeline(translucentPipeline)
	renderPass.SetVertexBuffer(0, vertexBuffer, 0)
	renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)

	cameraState := dc.renderer.cameraState
	camera, fogDensity, timeValue := gogpuWorldUniformInputs(state, cameraState)
	vp := dc.renderer.GetViewProjectionMatrix()
	var uniformData [worldUniformBufferSize]byte
	writeWorldUniform := func(alpha float32, dynamicLight [3]float32, litWater float32) bool {
		fillWorldSceneUniformBytes(uniformData[:], vp, camera, state.FogColor, fogDensity, timeValue, alpha, dynamicLight, litWater)
		if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
			slog.Error("renderWorldTranslucentLiquidsHAL: failed to update world uniform", "error", err)
			return false
		}
		renderPass.SetBindGroup(0, uniformBindGroup, nil)
		return true
	}

	translucentFaces := make([]gogpuTranslucentLiquidFaceDraw, 0, 8)
	for _, face := range worldData.Geometry.Faces {
		if !shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha) {
			continue
		}
		translucentFaces = append(translucentFaces, gogpuTranslucentLiquidFaceDraw{
			face:       face,
			alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
			center:     face.Center,
			distanceSq: worldFaceDistanceSq(face.Center, cameraState),
		})
	}
	sortGoGPUTranslucentLiquidFaces(effectiveGoGPUAlphaMode(GetAlphaMode()), translucentFaces)

	translucentLiquidDrawnIndices := uint32(0)
	for _, draw := range translucentFaces {
		lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(draw.face, worldLightmapPages, whiteLightmapBindGroup, worldHasLitWater)
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.center))
		if !writeWorldUniform(draw.alpha, dynamicLight, litWater) {
			renderPass.End()
			return
		}
		textureBindGroup := whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face, worldTextures, worldTextureAnimations, nil, 0, float64(timeValue)); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = whiteTextureBindGroup
		}
		if worldTexture := gogpuWorldTextureForFace(draw.face, worldFullbrightTextures, worldTextureAnimations, nil, 0, float64(timeValue)); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.NumIndices, 1, draw.face.FirstIndex, 0, 0)
		translucentLiquidDrawnIndices += draw.face.NumIndices
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderWorldTranslucentLiquidsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to finish encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to submit render commands", "error", err)
		return
	}
	if translucentLiquidDrawnIndices > 0 {
		slog.Debug("GoGPU translucent liquids rendered", "indices", translucentLiquidDrawnIndices, "triangles", translucentLiquidDrawnIndices/3)
	}
}

func (r *Renderer) hasTranslucentWorldLiquidFacesGoGPU() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	worldData := r.worldData
	r.mu.RUnlock()
	if worldData == nil {
		return false
	}
	return hasTranslucentWorldLiquidFaceType(
		worldData.Geometry.LiquidFaceTypes,
		worldLiquidAlphaSettingsForGeometry(worldData.Geometry),
	)
}

// worldDepthAttachmentForView picks the correct depth target for the current view configuration and pass sequence.
func worldDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false, // Must be false when StencilLoadOp is Clear (WebGPU spec)
	}
}

func gogpuSharedDepthStencilClearAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false,
	}
}

// ClearWorld releases world geometry resources.
// Called when switching maps or shutting down.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.worldFirstFrameStatsLogged.Store(false)

	if r.worldData != nil {
		// Release GPU buffers
		if r.worldVertexBuffer != nil {
			r.worldVertexBuffer.Release()
		}
		if r.worldIndexBuffer != nil {
			r.worldIndexBuffer.Release()
		}
		if r.worldDynamicIndexBuffer != nil {
			r.worldDynamicIndexBuffer.Release()
		}
		if r.uniformBuffer != nil {
			r.uniformBuffer.Release()
		}
		if r.worldSkyPipeline != nil {
			r.worldSkyPipeline.Release()
		}
		if r.worldSkyExternalPipeline != nil {
			r.worldSkyExternalPipeline.Release()
		}
		if r.worldTurbulentPipeline != nil {
			r.worldTurbulentPipeline.Release()
		}
		if r.worldTranslucentPipeline != nil {
			r.worldTranslucentPipeline.Release()
		}
		if r.worldTranslucentTurbulentPipeline != nil {
			r.worldTranslucentTurbulentPipeline.Release()
		}
		if r.worldPipeline != nil {
			r.worldPipeline.Release()
		}
		if r.worldAlphaTestPipeline != nil {
			r.worldAlphaTestPipeline.Release()
		}
		if r.worldPipelineLayout != nil {
			r.worldPipelineLayout.Release()
		}
		if r.worldSkyExternalPipelineLayout != nil {
			r.worldSkyExternalPipelineLayout.Release()
		}
		if r.uniformBindGroup != nil {
			r.uniformBindGroup.Release()
		}
		if r.uniformBindGroupLayout != nil {
			r.uniformBindGroupLayout.Release()
		}
		if r.textureBindGroupLayout != nil {
			r.textureBindGroupLayout.Release()
		}
		if r.worldSkyExternalBindGroupLayout != nil {
			r.worldSkyExternalBindGroupLayout.Release()
		}
		if r.whiteTextureBindGroup != nil {
			r.whiteTextureBindGroup.Release()
		}
		if r.whiteLightmapBindGroup != nil {
			r.whiteLightmapBindGroup.Release()
		}
		if r.worldTextureSampler != nil {
			r.worldTextureSampler.Release()
		}
		if r.worldLightmapSampler != nil {
			r.worldLightmapSampler.Release()
		}
		for textureIndex, worldTexture := range r.worldTextures {
			if worldTexture == nil {
				delete(r.worldTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldSkySolidTextures {
			if worldTexture == nil {
				delete(r.worldSkySolidTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldSkySolidTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldSkyAlphaTextures {
			if worldTexture == nil {
				delete(r.worldSkyAlphaTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldSkyAlphaTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldFullbrightTextures {
			if worldTexture == nil {
				delete(r.worldFullbrightTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldFullbrightTextures, textureIndex)
		}
		for index, worldLightmap := range r.worldLightmapPages {
			if worldLightmap == nil {
				continue
			}
			if worldLightmap.bindGroup != nil {
				worldLightmap.bindGroup.Release()
			}
			if worldLightmap.view != nil {
				worldLightmap.view.Release()
			}
			if worldLightmap.texture != nil {
				worldLightmap.texture.Release()
			}
			r.worldLightmapPages[index] = nil
		}
		if r.whiteTexture != nil {
			r.whiteTexture.Release()
		}
		if r.transparentBindGroup != nil {
			r.transparentBindGroup.Release()
		}
		if r.transparentTextureView != nil {
			r.transparentTextureView.Release()
		}
		if r.transparentTexture != nil {
			r.transparentTexture.Release()
		}
		if r.worldDepthTexture != nil {
			r.worldDepthTexture.Release()
		}
		for submodelIndex, lightmaps := range r.brushModelLightmaps {
			for _, lightmap := range lightmaps {
				if lightmap == nil {
					continue
				}
				if lightmap.bindGroup != nil {
					lightmap.bindGroup.Release()
				}
				if lightmap.view != nil {
					lightmap.view.Release()
				}
				if lightmap.texture != nil {
					lightmap.texture.Release()
				}
			}
			delete(r.brushModelLightmaps, submodelIndex)
		}
		r.destroyGoGPUExternalSkyboxResourcesLocked()

		r.worldData = nil
		r.worldVertexBuffer = nil
		r.worldIndexBuffer = nil
		r.worldDynamicIndexBuffer = nil
		r.worldDynamicIndexBufferSize = 0
		r.worldPipeline = nil
		r.worldAlphaTestPipeline = nil
		r.worldTranslucentPipeline = nil
		r.worldTurbulentPipeline = nil
		r.worldTranslucentTurbulentPipeline = nil
		r.worldSkyPipeline = nil
		r.worldSkyExternalPipeline = nil
		r.worldPipelineLayout = nil
		r.worldSkyExternalPipelineLayout = nil
		r.worldShader = nil
		r.uniformBuffer = nil
		r.uniformBindGroup = nil
		r.uniformBindGroupLayout = nil
		r.textureBindGroupLayout = nil
		r.worldSkyExternalBindGroupLayout = nil
		r.worldTextureSampler = nil
		r.worldTextures = nil
		r.worldFullbrightTextures = nil
		r.worldSkySolidTextures = nil
		r.worldSkyAlphaTextures = nil
		r.worldTextureAnimations = nil
		r.whiteTextureBindGroup = nil
		r.transparentTexture = nil
		r.transparentTextureView = nil
		r.transparentBindGroup = nil
		r.worldLightmapSampler = nil
		r.worldLightmapPages = nil
		r.whiteLightmapBindGroup = nil
		r.worldBindGroup = nil
		r.worldSkyExternalBindGroup = nil
		r.whiteTexture = nil
		r.whiteTextureView = nil
		r.worldDepthTexture = nil
		r.worldDepthTextureView = nil
		r.worldDepthWidth = 0
		r.worldDepthHeight = 0
		r.worldVisibleFacesScratch = worldVisibilityScratch{}
		r.worldSkyFacesScratch = nil
		r.worldTranslucentLiquidScratch = nil
		r.worldOpaqueDrawsScratch = nil
		r.worldAlphaDrawsScratch = nil
		r.worldLiquidDrawsScratch = nil
		r.worldBatchedIndexScratch = nil
		r.worldOpaqueBatchScratch = nil
		r.worldAlphaBatchScratch = nil
		r.worldLiquidBatchScratch = nil
		r.resetGoGPUWorldBatchCache()
		r.brushModelGeometry = make(map[int]*WorldGeometry)
		r.brushModelLightmaps = make(map[int][]*gpuWorldTexture)

		slog.Debug("World geometry cleared")
	}
}

// GetWorldData returns the current world render data (for debugging).
func (r *Renderer) GetWorldData() *WorldRenderData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData
}

// GetWorldBounds returns uploaded world geometry bounds when available.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}

	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}
