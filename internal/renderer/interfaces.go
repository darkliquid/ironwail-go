// interfaces.go defines the core contracts (GPUContext, WorldPipeline, AliasBatcher, Compositor2D)
// that decouple the monolithic Renderer struct into standalone, mockable components.
package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/wgpu"
)

// GPUContext defines the contract for accessing low-level WGPU device and queue handles.
type GPUContext interface {
	Device() *wgpu.Device
	Queue() *wgpu.Queue
}

// WorldPipeline defines the contract for uploading and rendering BSP world geometry, lightmaps, and skyboxes.
type WorldPipeline interface {
	UploadWorld(data *WorldRenderData) error
	DrawWorldOpaque(dc RenderContext)
	DrawWorldTranslucent(dc RenderContext)
	DrawSky(dc RenderContext)
}

// AliasBatcher defines the contract for interpolating and drawing Alias models.
type AliasBatcher interface {
	PrepareAliasDraws(entities []AliasModelEntity)
	DrawAliasModels(dc RenderContext)
}

// Compositor2D defines the contract for CPU-side 2D overlay compositing and quad drawing.
type Compositor2D interface {
	DrawPic(x, y int, pic *image.QPic)
	DrawCharacter(x, y int, num int)
	DrawString(x, y int, str string)
	FlushOverlay(dc RenderContext)
}
