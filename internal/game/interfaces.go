// interfaces.go defines the core contracts (SessionManager, CameraSystem, AssetCache, UIController)
// that decouple the monolithic Game struct into standalone, mockable components.
package game

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// SessionManager defines the contract for coordinating Host, Server, Client, and QuakeC VMs.
type SessionManager interface {
	IsActive() bool
	AdvanceFrame(dt float64)
}

// CameraSystem defines the contract for computing view matrices, chase camera positioning, and FOV zoom.
type CameraSystem interface {
	ComputeView(cameraOrigin, cameraAngles [3]float32)
	UpdateZoom(dt float64)
}

// AssetCache defines the contract for retrieving precached models, sounds, and BSP trees.
type AssetCache interface {
	GetAliasModel(name string) *model.Model
	GetBrushModel(name string) *bsp.Tree
}

// UIController defines the contract for managing HUD, menu, input dispatch, and debug overlays.
type UIController interface {
	RenderHUD()
	ProcessInput()
	DrawOverlays()
}
