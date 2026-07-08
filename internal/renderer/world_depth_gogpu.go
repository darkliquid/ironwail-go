package renderer

import (
	"fmt"
	"log/slog"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
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
		if r.worldDynamicLightsBuffer != nil {
			r.worldDynamicLightsBuffer.Release()
		}
		if r.worldSkyPipeline != nil {
			r.worldSkyPipeline.Release()
		}
		if r.worldSkyExternalPipeline != nil {
			r.worldSkyExternalPipeline.Release()
		}
		if r.worldSkyExternalOverlayPipeline != nil {
			r.worldSkyExternalOverlayPipeline.Release()
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
		if r.worldShader != nil {
			r.worldShader.Release()
		}
		if r.uniformBindGroup != nil {
			r.uniformBindGroup.Release()
		}
		if r.worldDynamicLightsBindGroup != nil {
			r.worldDynamicLightsBindGroup.Release()
		}
		if r.uniformBindGroupLayout != nil {
			r.uniformBindGroupLayout.Release()
		}
		if r.worldDynamicLightsBindGroupLayout != nil {
			r.worldDynamicLightsBindGroupLayout.Release()
		}
		if r.worldClusterComputeUniformBuffer != nil {
			r.worldClusterComputeUniformBuffer.Release()
		}
		if r.worldClusterComputeBindGroup != nil {
			r.worldClusterComputeBindGroup.Release()
		}
		if r.worldClusterComputeBindGroupLayout != nil {
			r.worldClusterComputeBindGroupLayout.Release()
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
		if r.worldTextures != nil {
			if r.worldTextures.bindGroup != nil {
				r.worldTextures.bindGroup.Release()
			}
			if r.worldTextures.view != nil {
				r.worldTextures.view.Release()
			}
			if r.worldTextures.texture != nil {
				r.worldTextures.texture.Release()
			}
			r.worldTextures = nil
		}
		for _, skyTex := range r.worldSkySolidTextures {
			if skyTex == nil {
				continue
			}
			if skyTex.bindGroup != nil {
				skyTex.bindGroup.Release()
			}
			if skyTex.view != nil {
				skyTex.view.Release()
			}
			if skyTex.texture != nil {
				skyTex.texture.Release()
			}
		}
		r.worldSkySolidTextures = nil
		for _, skyTex := range r.worldSkyAlphaTextures {
			if skyTex == nil {
				continue
			}
			if skyTex.bindGroup != nil {
				skyTex.bindGroup.Release()
			}
			if skyTex.view != nil {
				skyTex.view.Release()
			}
			if skyTex.texture != nil {
				skyTex.texture.Release()
			}
		}
		r.worldSkyAlphaTextures = nil
		if r.worldFullbrightTextures != nil {
			if r.worldFullbrightTextures.bindGroup != nil {
				r.worldFullbrightTextures.bindGroup.Release()
			}
			if r.worldFullbrightTextures.view != nil {
				r.worldFullbrightTextures.view.Release()
			}
			if r.worldFullbrightTextures.texture != nil {
				r.worldFullbrightTextures.texture.Release()
			}
			r.worldFullbrightTextures = nil
		}
		if r.worldLightmapArray != nil {
			if r.worldLightmapArray.bindGroup != nil {
				r.worldLightmapArray.bindGroup.Release()
			}
			if r.worldLightmapArray.view != nil {
				r.worldLightmapArray.view.Release()
			}
			if r.worldLightmapArray.texture != nil {
				r.worldLightmapArray.texture.Release()
			}
			r.worldLightmapArray = nil
		}
		if r.whiteTextureView != nil {
			r.whiteTextureView.Release()
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
		if r.worldDepthTextureView != nil {
			r.worldDepthTextureView.Release()
		}
		if r.worldDepthTexture != nil {
			r.worldDepthTexture.Release()
		}
		if r.worldMaterialsBuffer != nil {
			r.worldMaterialsBuffer.Release()
		}
		for _, buf := range r.externalBrushMaterialsBuffers {
			if buf != nil {
				buf.Release()
			}
		}
		for _, bg := range r.externalBrushUniformBindGroups {
			if bg != nil {
				bg.Release()
			}
		}
		for _, tex := range r.externalBrushTextures {
			if tex == nil {
				continue
			}
			if tex.bindGroup != nil {
				tex.bindGroup.Release()
			}
			if tex.view != nil {
				tex.view.Release()
			}
			if tex.texture != nil {
				tex.texture.Release()
			}
		}
		for _, tex := range r.externalBrushFullbright {
			if tex == nil {
				continue
			}
			if tex.bindGroup != nil {
				tex.bindGroup.Release()
			}
			if tex.view != nil {
				tex.view.Release()
			}
			if tex.texture != nil {
				tex.texture.Release()
			}
		}
		for _, lightmap := range r.brushModelLightmaps {
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
		r.worldSkyExternalOverlayPipeline = nil
		r.worldPipelineLayout = nil
		r.worldSkyExternalPipelineLayout = nil
		r.worldShader = nil
		r.uniformBuffer = nil
		r.worldMaterialsBuffer = nil
		r.worldBaseMaterials = nil
		r.worldDynamicLightsBuffer = nil
		r.uniformBindGroup = nil
		r.worldDynamicLightsBindGroup = nil
		r.uniformBindGroupLayout = nil
		r.worldDynamicLightsBindGroupLayout = nil
		r.worldClusterComputeUniformBuffer = nil
		r.worldClusterComputeBindGroup = nil
		r.worldClusterComputeBindGroupLayout = nil
		r.textureBindGroupLayout = nil
		r.lightmapBindGroupLayout = nil
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
		r.worldLightmapArray = nil
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
		r.brushModelLightmaps = make(map[int]*gpuWorldTexture)
		r.externalBrushTextures = make(map[string]*gpuWorldTexture)
		r.externalBrushFullbright = make(map[string]*gpuWorldTexture)
		r.externalBrushAnimations = make(map[string][]*surfacepkg.SurfaceTexture)
		r.externalBrushBaseMaterials = make(map[string][]WorldMaterialData)
		r.externalBrushMaterialsBuffers = make(map[string]*wgpu.Buffer)
		r.externalBrushUniformBindGroups = make(map[string]*wgpu.BindGroup)

		slog.Debug("World geometry cleared")
	}
}

// WorldData returns the current world render data (for debugging).
func (r *Renderer) WorldData() *WorldRenderData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData
}

// WorldBounds returns uploaded world geometry bounds when available.
func (r *Renderer) WorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}

	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}
