package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/renderer/pipeline"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// createWorldPipeline creates the render pipeline for world rendering.
// Configures all pipeline state: vertex layout, shaders, depth-stencil, primitive topology, etc.
// It delegates the construction to the pipeline subpackage (Pattern A seam);
// the four created bind group layouts are stored on the Renderer for reuse
// by the external-sky pipeline and bind-group creation.
func (r *Renderer) createWorldPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, error) {
	// Pick the world depth format from the device's enabled features before
	// any depth-state pipeline is created (browsers reject pipelines that use
	// an unrequested depth feature at strict validation).
	r.updateWorldDepthFormatForDevice()
	pipelineObj, pipelineLayout, uniformLayout, textureLayout, lightmapLayout, err := pipeline.CreateWorldPipeline(device, vertexShader, fragmentShader, r.worldPipelineParams())
	if err != nil {
		return nil, nil, err
	}
	r.mu.Lock()
	r.resources.UniformBindGroupLayout = uniformLayout
	r.resources.TextureBindGroupLayout = textureLayout
	r.resources.LightmapBindGroupLayout = lightmapLayout
	r.resources.WorldDynamicLightsBindGroupLayout = nil
	r.mu.Unlock()

	slog.Debug("World render pipeline created")
	return pipelineObj, pipelineLayout, nil
}

// worldPipelineParams returns the surface format + shared layouts the
// pipeline subpackage needs. Layouts are populated by createWorldPipeline
// when it stores them on the Renderer; the external-sky wrapper fills them in.
func (r *Renderer) worldPipelineParams() pipeline.WorldPipelineParams {
	return pipeline.WorldPipelineParams{
		SurfaceFormat:           r.surfaceFormat(),
		UniformBindGroupLayout:  r.resources.UniformBindGroupLayout,
		TextureBindGroupLayout:  r.resources.TextureBindGroupLayout,
		LightmapBindGroupLayout: r.resources.LightmapBindGroupLayout,
	}
}

func (r *Renderer) createWorldOpaquePipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	return pipeline.CreateWorldOpaquePipeline(device, vertexShader, fragmentShader, layout, r.surfaceFormat())
}

func (r *Renderer) createWorldSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	return pipeline.CreateWorldSkyPipeline(device, vertexShader, fragmentShader, layout, r.surfaceFormat())
}

func (r *Renderer) createWorldExternalSkyPipeline(device *wgpu.Device, vertexShader, overlayVertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	return pipeline.CreateWorldExternalSkyPipeline(device, vertexShader, overlayVertexShader, fragmentShader, r.worldPipelineParams())
}

func (r *Renderer) createWorldTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	return pipeline.CreateWorldTurbulentPipeline(device, vertexShader, fragmentShader, layout, r.surfaceFormat())
}

func (r *Renderer) createWorldTranslucentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	return pipeline.CreateWorldTranslucentPipeline(device, vertexShader, fragmentShader, layout, r.surfaceFormat())
}

func (r *Renderer) createWorldTranslucentTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	return pipeline.CreateWorldTranslucentTurbulentPipeline(device, vertexShader, fragmentShader, layout, r.surfaceFormat())
}

// surfaceFormat returns the GPU swapchain surface format, defaulting to
// BGRA8Unorm when no app/device provider is available.
func (r *Renderer) surfaceFormat() gputypes.TextureFormat {
	return r.sceneSurfaceFormat()
}
