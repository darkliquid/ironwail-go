package game

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// RendererFrameLoop defines frame-level rendering callbacks.
type RendererFrameLoop interface {
	OnDraw(func(renderer.RenderContext))
	OnUpdate(func(dt float64))
	RequestRedraw()
	StepWasmFrame(dt float64)
	Size() (width, height int)
	SetConfig(renderer.Config)
	Run() error
	Stop()
	Shutdown()
}

// RendererAssets defines asset management methods.
type RendererAssets interface {
	SetPalette([]byte)
	SetConchars([]byte)
	SetExternalSkybox(string, func(string) ([]byte, error))
	UploadPendingExternalSkybox() error
}

// RendererWorld defines world rendering methods.
type RendererWorld interface {
	UpdateCamera(renderer.CameraState, float32, float32)
	UploadWorld(*bsp.Tree) error
	HasWorldData() bool
	WorldBounds() (min types.Vec3, max types.Vec3, ok bool)
	PreloadBrushEntities([]renderer.BrushEntity)
}

// RendererLights defines dynamic light methods.
type RendererLights interface {
	SpawnDynamicLight(renderer.DynamicLight) bool
	SpawnKeyedDynamicLight(renderer.DynamicLight) bool
	UpdateLights(float32)
	ClearDynamicLights()
}

// RendererInput defines input backend methods.
type RendererInput interface {
	InputBackendForSystem(*input.System) input.Backend
}

// RendererGPUView defines the gogpu/ui world-texture integration surface
// (ADR-0009): the engine exposes its gogpu.App (desktop.Run's loop owner) and
// can render the world into a gpuview texture view (M1.4b). The quakeui Host
// routes to these using gogpu/gpucontext types only.
type RendererGPUView interface {
	// GogpuApp returns the underlying gogpu application.
	GogpuApp() *gogpu.App
	// RenderWorldIntoView renders the world into an offscreen gpuview texture.
	RenderWorldIntoView(gpucontext.TextureView, *renderer.RenderFrameState) error
}

// Renderer is the composite interface for all renderer functionality.
type Renderer interface {
	RendererFrameLoop
	RendererAssets
	RendererWorld
	RendererLights
	RendererInput
	RendererGPUView
}

// CanvasParamSetter allows setting canvas transformation parameters.
type CanvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}
