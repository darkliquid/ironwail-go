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
		drawParticles:               state.DrawParticles,
		hasDecalMarks:               len(state.DecalMarks) > 0,
		hasTranslucentBrushEntities: len(translucentBrushEntities) > 0,
		hasTranslucentAliasEntities: len(translucentAliasEntities) > 0,
	})
	if dc.gldc.renderer != nil {
		dc.gldc.renderer.setLightStyleValues(state.LightStyles)
		dc.gldc.renderer.setFogState(state.FogColor, state.FogDensity)
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
	if state.DrawEntities && dc.gldc.renderer != nil && len(state.SpriteEntities) > 0 {
		dc.gldc.renderer.renderSpriteEntities(state.SpriteEntities)
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
		dc.gldc.renderer.renderBrushEntities(translucentBrushEntities, worldBrushPassLiquidTranslucentOnly)
	}
	if dc.gldc.renderer != nil && len(state.DecalMarks) > 0 {
		dc.gldc.renderer.renderDecalMarks(state.DecalMarks)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(translucentBrushEntities) > 0 {
		dc.gldc.renderer.renderBrushEntities(translucentBrushEntities, worldBrushPassNonLiquid)
	}
	if state.DrawEntities && dc.gldc.renderer != nil && len(translucentAliasEntities) > 0 {
		dc.gldc.renderer.renderAliasEntities(translucentAliasEntities)
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
	if state.Draw2DOverlay && draw2DOverlay != nil {
		gl.Disable(gl.DEPTH_TEST)
		draw2DOverlay(dc)
		gl.Enable(gl.DEPTH_TEST)
	}
}

// RenderContext interface delegation to the underlying glDrawContext.

func (dc *DrawContext) Clear(r, g, b, a float32)            { dc.gldc.Clear(r, g, b, a) }
func (dc *DrawContext) DrawTriangle(r, g, b, a float32)     { dc.gldc.DrawTriangle(r, g, b, a) }
func (dc *DrawContext) SurfaceView() interface{}            { return dc.gldc.SurfaceView() }
func (dc *DrawContext) Gamma() float32                      { return dc.gldc.Gamma() }
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic)   { dc.gldc.DrawPic(x, y, pic) }
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
