package game

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// RendererFrameLoop defines frame-level rendering callbacks.
type RendererFrameLoop interface {
	OnDraw(func(renderer.RenderContext))
	OnUpdate(func(dt float64))
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
	WorldBounds() (min [3]float32, max [3]float32, ok bool)
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

// Renderer is the composite interface for all renderer functionality.
type Renderer interface {
	RendererFrameLoop
	RendererAssets
	RendererWorld
	RendererLights
	RendererInput
}

// CanvasParamSetter allows setting canvas transformation parameters.
type CanvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}
