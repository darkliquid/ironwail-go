//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
)

// CameraState holds camera position and orientation.
// This is a simplified version for the OpenGL/CGO path; 3D world rendering
// is not yet implemented here so the values are stored but not acted on.
type CameraState struct {
	Origin [3]float32
	Angles [3]float32
	FOV    float32
}

// RenderFrameState carries per-frame render configuration passed to RenderFrame.
type RenderFrameState struct {
	ClearColor    [4]float32
	DrawWorld     bool
	DrawEntities  bool
	DrawParticles bool
	Draw2DOverlay bool
	MenuActive    bool
	Particles     *ParticleSystem
	Palette       []byte
}

// DrawContext wraps the underlying OpenGL draw context and is the concrete type
// passed to OnDraw callbacks, allowing main.go's type assertion to succeed.
type DrawContext struct {
	gldc *glDrawContext
}

// RenderFrame executes the frame pipeline. On the OpenGL path, 3D world
// rendering is not yet implemented; only the 2D overlay is drawn.
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}
	dc.gldc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])
	if state.Draw2DOverlay && draw2DOverlay != nil {
		draw2DOverlay(dc)
	}
}

// RenderContext interface delegation to the underlying glDrawContext.

func (dc *DrawContext) Clear(r, g, b, a float32)              { dc.gldc.Clear(r, g, b, a) }
func (dc *DrawContext) DrawTriangle(r, g, b, a float32)       { dc.gldc.DrawTriangle(r, g, b, a) }
func (dc *DrawContext) SurfaceView() interface{}               { return dc.gldc.SurfaceView() }
func (dc *DrawContext) Gamma() float32                         { return dc.gldc.Gamma() }
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic)     { dc.gldc.DrawPic(x, y, pic) }
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte)   { dc.gldc.DrawFill(x, y, w, h, color) }
func (dc *DrawContext) DrawCharacter(x, y int, num int)       { dc.gldc.DrawCharacter(x, y, num) }

// DefaultRenderFrameState returns a sensible default RenderFrameState.
func DefaultRenderFrameState() *RenderFrameState {
	return &RenderFrameState{
		ClearColor:    [4]float32{0, 0, 0, 1},
		DrawWorld:     false,
		DrawEntities:  false,
		DrawParticles: false,
		Draw2DOverlay: true,
		MenuActive:    true,
	}
}

// ConvertClientStateToCamera converts client prediction state to a CameraState.
// On the OpenGL path this is a passthrough; matrices are not yet computed here.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{Origin: origin, Angles: angles, FOV: fov}
}

// GetWorldBounds returns false; BSP world rendering is not yet implemented for the OpenGL path.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	return [3]float32{}, [3]float32{}, false
}

// UpdateCamera stores the camera state. Not yet acted on for the OpenGL path.
func (r *Renderer) UpdateCamera(_ CameraState, _, _ float32) {}

// HasWorldData returns false; BSP world upload is not yet implemented for the OpenGL path.
func (r *Renderer) HasWorldData() bool { return false }

// UploadWorld is a no-op on the OpenGL path.
func (r *Renderer) UploadWorld(_ *bsp.Tree) error { return nil }
