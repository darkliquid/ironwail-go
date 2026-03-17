//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/image"
)

// RenderFrameState carries per-frame render configuration passed to RenderFrame.
type RenderFrameState struct {
	ClearColor     [4]float32
	DrawWorld      bool
	DrawEntities   bool
	BrushEntities  []BrushEntity
	AliasEntities  []AliasModelEntity
	SpriteEntities []SpriteEntity
	DecalMarks     []DecalMarkEntity
	ViewModel      *AliasModelEntity
	LightStyles    [64]float32
	FogColor       [3]float32
	FogDensity     float32
	DrawParticles  bool
	Draw2DOverlay  bool
	MenuActive     bool
	Particles      *ParticleSystem
	Palette        []byte

	// WaterWarp enables the screen-space sinusoidal post-process warp effect.
	// Set when r_waterwarp == 1 and the camera is in a liquid leaf (or ForceUnderwater is true).
	// Mirrors C Ironwail: water_warp flag fed into R_WarpScaleView().
	WaterWarp bool

	// WaterWarpTime is the time value driving the warp animation.
	// Use cl.time normally; use realtime when ForceUnderwater is true (menu preview).
	// Mirrors C Ironwail: `t = M_ForcedUnderwater() ? realtime : cl.time` in R_WarpScaleView().
	WaterWarpTime float32

	// ForceUnderwater signals that the menu is previewing the underwater warp effect.
	// When true, the warp is active regardless of camera leaf contents.
	// Mirrors C Ironwail: M_ForcedUnderwater() used in R_SetupView() and R_WarpScaleView().
	ForceUnderwater bool

	// VBlend is the composite RGBA screen-tint from v_blend color shifts.
	// Applied as a full-screen alpha-blended quad after the 3D scene and any
	// FBO blit, and before the 2D HUD overlay.
	// Mirrors C Ironwail: view.c V_PolyBlend() / glprogs.viewblend.
	// RGB in 0..1; Alpha is the composite opacity (0 = no tint, 1 = full cover).
	VBlend [4]float32
}

// DrawContext wraps the underlying OpenGL draw context and is the concrete type
// passed to OnDraw callbacks, allowing main.go's type assertion to succeed.
type DrawContext struct {
	gldc *glDrawContext
}

func beginLateTranslucencyStateBlock() {
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
}

func endLateTranslucencyStateBlock() {
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
}

// RenderFrame executes the frame pipeline for the OpenGL path.
//
// When state.WaterWarp is true (r_waterwarp == 1, camera in liquid leaf), the
// 3D scene is rendered to an offscreen FBO and then blitted to the default
// framebuffer through the warpscale post-process shader, producing the
// sinusoidal screen-space distortion. Mirrors C Ironwail R_WarpScaleView().
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}
	var opaqueBrushEntities, translucentBrushEntities []BrushEntity
	var opaqueAliasEntities, translucentAliasEntities []AliasModelEntity
	if state.DrawEntities && len(state.BrushEntities) > 0 {
		opaqueBrushEntities, translucentBrushEntities = splitBrushEntitiesByAlpha(state.BrushEntities)
	}
	if state.DrawEntities && len(state.AliasEntities) > 0 {
		opaqueAliasEntities, translucentAliasEntities = splitAliasEntitiesByAlpha(state.AliasEntities)
	}
	lateTranslucency := shouldRunLateTranslucencyBlock(lateTranslucencyBlockInputs{
		drawWorld:                   state.DrawWorld,
		hasTranslucentWorld:         state.DrawWorld && dc.gldc.renderer != nil && dc.gldc.renderer.hasTranslucentWorldLiquidFaces(),
		drawEntities:                state.DrawEntities,
		hasSpriteEntities:           len(state.SpriteEntities) > 0,
		drawParticles:               state.DrawParticles,
		hasDecalMarks:               len(state.DecalMarks) > 0,
		hasTranslucentBrushEntities: len(translucentBrushEntities) > 0,
		hasTranslucentAliasEntities: len(translucentAliasEntities) > 0,
	})
	if dc.gldc.renderer != nil {
		dc.gldc.renderer.ClearTranslucentCalls()
		dc.gldc.renderer.setLightStyleValues(state.LightStyles)
		dc.gldc.renderer.setFogState(state.FogColor, state.FogDensity)
	}

	// --- Screen-space underwater warp setup (r_waterwarp == 1) ---
	// When active, redirect all 3D scene rendering to the scene FBO.
	// After the 3D scene, apply the warpscale post-process then restore default FBO.
	// Mirrors C Ironwail: R_WarpScaleView() after R_RenderScene().
	warpViewport := dc.gldc.viewport
	if state.WaterWarp && dc.gldc.renderer != nil {
		w, h := warpViewport.width, warpViewport.height
		if w > 0 && h > 0 {
			if err := dc.gldc.renderer.ensureSceneFBO(w, h); err == nil {
				gl.BindFramebuffer(gl.FRAMEBUFFER, dc.gldc.renderer.sceneFBO)
			}
		}
	}

	dc.gldc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])
	if state.DrawWorld && dc.gldc.renderer != nil {
		dc.gldc.renderer.renderWorld(worldBrushPassNonLiquid)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(opaqueBrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(opaqueBrushEntities, worldBrushPassNonLiquid)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(opaqueAliasEntities) > 0 {
		dc.gldc.renderer.renderAliasEntities(opaqueAliasEntities)
	}
	if state.DrawParticles && dc.gldc.renderer != nil && state.Particles != nil {
		dc.gldc.renderer.renderParticles(state.Particles, state.Palette, particlePassOpaque)
	}
	if state.DrawWorld && dc.gldc.renderer != nil {
		dc.gldc.renderer.renderWorld(worldBrushPassSkyOnly)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(state.BrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(state.BrushEntities, worldBrushPassSkyOnly)
	}
	if state.DrawWorld && dc.gldc.renderer != nil {
		dc.gldc.renderer.renderWorld(worldBrushPassLiquidOpaqueOnly)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(opaqueBrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(opaqueBrushEntities, worldBrushPassLiquidOpaqueOnly)
	}
	if lateTranslucency {
		beginLateTranslucencyStateBlock()
	}
	if state.DrawWorld && dc.gldc.renderer != nil {
		dc.gldc.renderer.renderWorld(worldBrushPassLiquidTranslucentOnly)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(opaqueBrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(opaqueBrushEntities, worldBrushPassLiquidTranslucentOnly)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(translucentBrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(translucentBrushEntities, worldBrushPassAll)
	}

	// Draw sorted translucent world/brush model faces
	if dc.gldc.renderer != nil {
		dc.gldc.renderer.DrawTranslucentCalls()
	}

	if dc.gldc.renderer != nil && len(state.DecalMarks) > 0 {
		dc.gldc.renderer.renderDecalMarks(state.DecalMarks)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(translucentAliasEntities) > 0 {
		dc.gldc.renderer.renderAliasEntities(translucentAliasEntities)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(state.SpriteEntities) > 0 {
		dc.gldc.renderer.renderSpriteEntities(state.SpriteEntities)
	}
	if state.DrawParticles && dc.gldc.renderer != nil && state.Particles != nil {
		dc.gldc.renderer.renderParticles(state.Particles, state.Palette, particlePassTranslucent)
	}
	if lateTranslucency {
		endLateTranslucencyStateBlock()
	}
	if state.DrawEntities && dc.gldc.renderer != nil && state.ViewModel != nil {
		dc.gldc.renderer.renderViewModel(*state.ViewModel)
	}

	// --- Screen-space underwater warp post-process ---
	// Apply the warpscale effect and restore the default framebuffer before 2D overlay.
	if state.WaterWarp && dc.gldc.renderer != nil && dc.gldc.renderer.sceneFBO != 0 {
		w, h := warpViewport.width, warpViewport.height
		dc.gldc.renderer.applyWarpScaleEffect(true, state.WaterWarpTime, w, h)
		// Restore viewport for 2D overlay.
		gl.Viewport(0, 0, int32(w), int32(h))
	}

	// --- v_blend polyblend screen tint ---
	// Applied after the 3D scene (and any FBO blit) but before the 2D overlay.
	// Mirrors C Ironwail: view.c V_PolyBlend() / glprogs.viewblend.
	if dc.gldc.renderer != nil && state.VBlend[3] > 0 {
		dc.gldc.renderer.renderPolyBlend(state.VBlend)
	}

	if state.Draw2DOverlay && draw2DOverlay != nil {
		gl.Disable(gl.DEPTH_TEST)
		draw2DOverlay(dc)
		gl.Enable(gl.DEPTH_TEST)
	}
}

// RenderContext interface delegation to the underlying glDrawContext.

func (dc *DrawContext) Clear(r, g, b, a float32)          { dc.gldc.Clear(r, g, b, a) }
func (dc *DrawContext) DrawTriangle(r, g, b, a float32)   { dc.gldc.DrawTriangle(r, g, b, a) }
func (dc *DrawContext) SurfaceView() interface{}          { return dc.gldc.SurfaceView() }
func (dc *DrawContext) Gamma() float32                    { return dc.gldc.Gamma() }
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) { dc.gldc.DrawPic(x, y, pic) }
func (dc *DrawContext) DrawMenuPic(x, y int, pic *image.QPic) {
	dc.gldc.DrawMenuPic(x, y, pic)
}
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) { dc.gldc.DrawFill(x, y, w, h, color) }
func (dc *DrawContext) DrawCharacter(x, y int, num int)     { dc.gldc.DrawCharacter(x, y, num) }
func (dc *DrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.gldc.DrawMenuCharacter(x, y, num)
}

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
