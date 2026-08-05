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
		if r.resources.UniformBuffer != nil {
			r.resources.UniformBuffer.Release()
		}
		if r.resources.WorldDynamicLightsBuffer != nil {
			r.resources.WorldDynamicLightsBuffer.Release()
		}
		if r.resources.WorldSkyPipeline != nil {
			r.resources.WorldSkyPipeline.Release()
		}
		if r.resources.WorldSkyExternalPipeline != nil {
			r.resources.WorldSkyExternalPipeline.Release()
		}
		if r.resources.WorldSkyExternalOverlayPipeline != nil {
			r.resources.WorldSkyExternalOverlayPipeline.Release()
		}
		if r.resources.WorldTurbulentPipeline != nil {
			r.resources.WorldTurbulentPipeline.Release()
		}
		if r.resources.WorldTranslucentPipeline != nil {
			r.resources.WorldTranslucentPipeline.Release()
		}
		if r.resources.WorldTranslucentTurbulentPipeline != nil {
			r.resources.WorldTranslucentTurbulentPipeline.Release()
		}
		if r.resources.WorldPipeline != nil {
			r.resources.WorldPipeline.Release()
		}
		if r.resources.WorldAlphaTestPipeline != nil {
			r.resources.WorldAlphaTestPipeline.Release()
		}
		if r.resources.WorldPipelineLayout != nil {
			r.resources.WorldPipelineLayout.Release()
		}
		if r.resources.WorldSkyExternalPipelineLayout != nil {
			r.resources.WorldSkyExternalPipelineLayout.Release()
		}
		if r.resources.WorldShader != nil {
			r.resources.WorldShader.Release()
		}
		if r.resources.UniformBindGroup != nil {
			r.resources.UniformBindGroup.Release()
		}
		if r.resources.WorldDynamicLightsBindGroup != nil {
			r.resources.WorldDynamicLightsBindGroup.Release()
		}
		if r.resources.UniformBindGroupLayout != nil {
			r.resources.UniformBindGroupLayout.Release()
		}
		if r.resources.WorldDynamicLightsBindGroupLayout != nil {
			r.resources.WorldDynamicLightsBindGroupLayout.Release()
		}
		if r.resources.WorldClusterComputeUniformBuffer != nil {
			r.resources.WorldClusterComputeUniformBuffer.Release()
		}
		if r.resources.WorldClusterComputeBindGroup != nil {
			r.resources.WorldClusterComputeBindGroup.Release()
		}
		if r.resources.WorldClusterComputeBindGroupLayout != nil {
			r.resources.WorldClusterComputeBindGroupLayout.Release()
		}
		if r.resources.TextureBindGroupLayout != nil {
			r.resources.TextureBindGroupLayout.Release()
		}
		if r.resources.WorldSkyExternalBindGroupLayout != nil {
			r.resources.WorldSkyExternalBindGroupLayout.Release()
		}
		if r.resources.WhiteTextureBindGroup != nil {
			r.resources.WhiteTextureBindGroup.Release()
		}
		if r.resources.WhiteLightmapBindGroup != nil {
			r.resources.WhiteLightmapBindGroup.Release()
		}
		if r.resources.BlackLightmapView != nil {
			r.resources.BlackLightmapView.Release()
		}
		if r.resources.BlackLightmapTexture != nil {
			r.resources.BlackLightmapTexture.Release()
		}
		if r.resources.WorldTextureSampler != nil {
			r.resources.WorldTextureSampler.Release()
		}
		if r.resources.WorldLightmapSampler != nil {
			r.resources.WorldLightmapSampler.Release()
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
		if r.resources.WhiteTextureView != nil {
			r.resources.WhiteTextureView.Release()
		}
		if r.resources.WhiteTexture != nil {
			r.resources.WhiteTexture.Release()
		}
		if r.resources.TransparentBindGroup != nil {
			r.resources.TransparentBindGroup.Release()
		}
		if r.resources.TransparentTextureView != nil {
			r.resources.TransparentTextureView.Release()
		}
		if r.resources.TransparentTexture != nil {
			r.resources.TransparentTexture.Release()
		}
		if r.resources.WorldDepthTextureView != nil {
			r.resources.WorldDepthTextureView.Release()
		}
		if r.resources.WorldDepthTexture != nil {
			r.resources.WorldDepthTexture.Release()
		}
		if r.resources.WorldMaterialsBuffer != nil {
			r.resources.WorldMaterialsBuffer.Release()
		}
		if r.resources.WorldMaterialsBufferFrame1 != nil {
			r.resources.WorldMaterialsBufferFrame1.Release()
		}
		if r.resources.WorldUniformBindGroupFrame1 != nil {
			r.resources.WorldUniformBindGroupFrame1.Release()
		}
		for _, buf := range r.externalBrushMaterialsBuffers {
			if buf != nil {
				buf.Release()
			}
		}
		for _, buf := range r.externalBrushMaterialsBuffersFrame1 {
			if buf != nil {
				buf.Release()
			}
		}
		for _, bg := range r.externalBrushUniformBindGroups {
			if bg != nil {
				bg.Release()
			}
		}
		for _, bg := range r.externalBrushUniformBindGroupsFrame1 {
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
		r.resources.WorldPipeline = nil
		r.resources.WorldAlphaTestPipeline = nil
		r.resources.WorldTranslucentPipeline = nil
		r.resources.WorldTurbulentPipeline = nil
		r.resources.WorldTranslucentTurbulentPipeline = nil
		r.resources.WorldSkyPipeline = nil
		r.resources.WorldSkyExternalPipeline = nil
		r.resources.WorldSkyExternalOverlayPipeline = nil
		r.resources.WorldPipelineLayout = nil
		r.resources.WorldSkyExternalPipelineLayout = nil
		r.resources.WorldShader = nil
		r.resources.UniformBuffer = nil
		r.resources.WorldMaterialsBuffer = nil
		r.resources.WorldMaterialsBufferFrame1 = nil
		r.resources.WorldUniformBindGroupFrame1 = nil
		r.worldBaseMaterials = nil
		r.resources.WorldDynamicLightsBuffer = nil
		r.resources.UniformBindGroup = nil
		r.resources.WorldDynamicLightsBindGroup = nil
		r.resources.UniformBindGroupLayout = nil
		r.resources.WorldDynamicLightsBindGroupLayout = nil
		r.resources.WorldClusterComputeUniformBuffer = nil
		r.resources.WorldClusterComputeBindGroup = nil
		r.resources.WorldClusterComputeBindGroupLayout = nil
		r.resources.TextureBindGroupLayout = nil
		r.resources.LightmapBindGroupLayout = nil
		r.resources.WorldSkyExternalBindGroupLayout = nil
		r.resources.WorldTextureSampler = nil
		r.worldTextures = nil
		r.worldFullbrightTextures = nil
		r.worldSkySolidTextures = nil
		r.worldSkyAlphaTextures = nil
		r.worldTextureAnimations = nil
		r.resources.WhiteTextureBindGroup = nil
		r.resources.TransparentTexture = nil
		r.resources.TransparentTextureView = nil
		r.resources.TransparentBindGroup = nil
		r.resources.WorldLightmapSampler = nil
		r.worldLightmapArray = nil
		r.resources.WhiteLightmapBindGroup = nil
		r.resources.BlackLightmapTexture = nil
		r.resources.BlackLightmapView = nil
		r.resources.WorldBindGroup = nil
		r.resources.WorldSkyExternalBindGroup = nil
		r.resources.WhiteTexture = nil
		r.resources.WhiteTextureView = nil
		r.resources.WorldDepthTexture = nil
		r.resources.WorldDepthTextureView = nil
		r.resources.WorldDepthWidth = 0
		r.resources.WorldDepthHeight = 0
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
		r.externalBrushMaterialsBuffersFrame1 = make(map[string]*wgpu.Buffer)
		r.externalBrushUniformBindGroups = make(map[string]*wgpu.BindGroup)
		r.externalBrushUniformBindGroupsFrame1 = make(map[string]*wgpu.BindGroup)

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
