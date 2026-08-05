// This file belongs to the pipeline subpackage: the Resources struct that
// owns the renderer's wgpu object graph (plan 16+2a, the deep
// state-object inversion that splits the Renderer struct).
//
// Resources holds every pipeline/layout/buffer/sampler/texture the renderer
// allocates; the parent Renderer keeps only the fields whose types live in
// renderer-root (gpuWorldTexture, surface textures, world material data,
// external-skybox bookkeeping) because moving those would create an import
// cycle. The parent reaches the wgpu graph through r.resources.
//
// Field names are unexported to match the historical Renderer fields they
// replace (r.worldPipeline → r.resources.worldPipeline); access is under the
// parent's mutex at the same call sites as before.

package pipeline

import (
	"github.com/gogpu/wgpu"
)

// Resources owns the renderer's wgpu objects. It is created once per
// Renderer and populated during world upload / resource ensure; destroyed by
// releasing the individual handles on shutdown.
type Resources struct {
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

	WorldBindGroup                       *wgpu.BindGroup
	WorldShader                          *wgpu.ShaderModule
	UniformBuffer                        *wgpu.Buffer
	WorldMaterialsBuffer                 *wgpu.Buffer
	WorldMaterialsBufferFrame1           *wgpu.Buffer
	WorldUniformBindGroupFrame1          *wgpu.BindGroup
	UniformBindGroup                     *wgpu.BindGroup
	UniformBindGroupLayout               *wgpu.BindGroupLayout
	TextureBindGroupLayout               *wgpu.BindGroupLayout
	LightmapBindGroupLayout              *wgpu.BindGroupLayout
	WorldSkyExternalBindGroupLayout      *wgpu.BindGroupLayout
	WorldTextureSampler                  *wgpu.Sampler

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

	SceneCompositePipeline          *wgpu.RenderPipeline
	SceneCompositePipelineLayout    *wgpu.PipelineLayout
	SceneCompositeVertexShader      *wgpu.ShaderModule
	SceneCompositeFragmentShader    *wgpu.ShaderModule
	SceneCompositeBindGroupLayout   *wgpu.BindGroupLayout
	SceneCompositeSampler           *wgpu.Sampler
	SceneCompositeUniformBuffer     *wgpu.Buffer
	SceneCompositeBindGroup         *wgpu.BindGroup

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
