// Package pipeline implements render-pipeline constructors and WGPU resource encapsulation.
package pipeline

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// WorldResourceProvider defines the interface for accessing GPU resources required by render pipelines.
type WorldResourceProvider interface {
	Device() *wgpu.Device
	DepthFormat() gputypes.TextureFormat
	UniformBuffer() *wgpu.Buffer
}

// Resources owns the renderer's wgpu object graph (plan 16+2a).
type Resources struct {
	provider WorldResourceProvider
	layouts  []*wgpu.BindGroupLayout

	WorldPipeline                     *wgpu.RenderPipeline
	WorldAlphaTestPipeline            *wgpu.RenderPipeline
	WorldTranslucentPipeline          *wgpu.RenderPipeline
	WorldTurbulentPipeline            *wgpu.RenderPipeline
	WorldTranslucentTurbulentPipeline *wgpu.RenderPipeline
	WorldSkyPipeline                  *wgpu.RenderPipeline
	WorldSkyExternalPipeline          *wgpu.RenderPipeline
	WorldSkyExternalOverlayPipeline   *wgpu.RenderPipeline
	WorldPipelineLayout               *wgpu.PipelineLayout
	WorldSkyExternalPipelineLayout    *wgpu.PipelineLayout

	WorldDynamicLightsBuffer          *wgpu.Buffer
	WorldDynamicLightsBindGroup       *wgpu.BindGroup
	WorldDynamicLightsBindGroupLayout *wgpu.BindGroupLayout

	WorldClusterComputePipeline        *wgpu.ComputePipeline
	WorldClusterComputePipelineLayout  *wgpu.PipelineLayout
	WorldClusterComputeBindGroup       *wgpu.BindGroup
	WorldClusterComputeBindGroupLayout *wgpu.BindGroupLayout
	WorldClusterComputeUniformBuffer   *wgpu.Buffer
	WorldClusterComputeTexture         *wgpu.Texture
	WorldClusterComputeTextureView     *wgpu.TextureView

	WorldBindGroup                  *wgpu.BindGroup
	WorldShader                     *wgpu.ShaderModule
	UniformBuffer                   *wgpu.Buffer
	WorldMaterialsBuffer            *wgpu.Buffer
	WorldMaterialsBufferFrame1      *wgpu.Buffer
	WorldUniformBindGroupFrame1     *wgpu.BindGroup
	UniformBindGroup                *wgpu.BindGroup
	UniformBindGroupLayout          *wgpu.BindGroupLayout
	TextureBindGroupLayout          *wgpu.BindGroupLayout
	LightmapBindGroupLayout         *wgpu.BindGroupLayout
	WorldSkyExternalBindGroupLayout *wgpu.BindGroupLayout
	WorldTextureSampler             *wgpu.Sampler

	WorldSkyExternalTextures  [6]*wgpu.Texture
	WorldSkyExternalViews     [6]*wgpu.TextureView
	WorldSkyExternalBindGroup *wgpu.BindGroup

	WorldSkyExternalWindLoaded      bool
	WorldSkyExternalLoaded          int
	WorldSkyExternalName            string
	WorldSkyExternalRequestID       uint64
	WorldSkyExternalLoading         bool
	WorldSkyExternalUploadCursor    int
	WorldSkyExternalWorldDrawLogged bool
	WorldSkyExternalBrushDrawLogged bool

	WhiteTextureBindGroup  *wgpu.BindGroup
	TransparentTexture     *wgpu.Texture
	TransparentTextureView *wgpu.TextureView
	TransparentBindGroup   *wgpu.BindGroup

	WorldLightmapSampler   *wgpu.Sampler
	WhiteLightmapBindGroup *wgpu.BindGroup
	BlackLightmapTexture   *wgpu.Texture
	BlackLightmapView      *wgpu.TextureView
	WorldLightStyleValues  [256]float32

	WhiteTexture          *wgpu.Texture
	WhiteTextureView      *wgpu.TextureView
	WorldDepthTexture     *wgpu.Texture
	WorldDepthTextureView *wgpu.TextureView
	WorldDepthWidth       int
	WorldDepthHeight      int

	WorldRenderTexture     *wgpu.Texture
	WorldRenderTextureView *wgpu.TextureView
	WorldRenderWidth       int
	WorldRenderHeight      int

	SceneCompositePipeline        *wgpu.RenderPipeline
	SceneCompositePipelineLayout  *wgpu.PipelineLayout
	SceneCompositeVertexShader    *wgpu.ShaderModule
	SceneCompositeFragmentShader  *wgpu.ShaderModule
	SceneCompositeBindGroupLayout *wgpu.BindGroupLayout
	SceneCompositeSampler         *wgpu.Sampler
	SceneCompositeUniformBuffer   *wgpu.Buffer
	SceneCompositeBindGroup       *wgpu.BindGroup

	OverlayCompositePipeline        *wgpu.RenderPipeline
	OverlayCompositePipelineLayout  *wgpu.PipelineLayout
	OverlayCompositeVertexShader    *wgpu.ShaderModule
	OverlayCompositeFragmentShader  *wgpu.ShaderModule
	OverlayCompositeBindGroupLayout *wgpu.BindGroupLayout
	OverlayCompositeBindGroup       *wgpu.BindGroup
	OverlayCompositeTextureView     *wgpu.TextureView
}

// NewResources returns an empty wgpu resource container.
func NewResources() *Resources {
	return &Resources{}
}

// NewResourcesWithProvider constructs a new pipeline Resources container over a WorldResourceProvider.
func NewResourcesWithProvider(provider WorldResourceProvider, layouts []*wgpu.BindGroupLayout) (*Resources, error) {
	if provider == nil {
		return nil, fmt.Errorf("nil provider")
	}
	return &Resources{
		provider: provider,
		layouts:  layouts,
	}, nil
}

// Layouts returns the registered bind group layouts.
func (r *Resources) Layouts() []*wgpu.BindGroupLayout {
	if r == nil {
		return nil
	}
	return r.layouts
}

// Provider returns the underlying WorldResourceProvider.
func (r *Resources) Provider() WorldResourceProvider {
	if r == nil {
		return nil
	}
	return r.provider
}
