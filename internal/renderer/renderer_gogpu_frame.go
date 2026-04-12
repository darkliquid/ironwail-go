package renderer

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/wgpu"
)

type RenderFrameState struct {
	// ClearColor is the RGBA clear color (typically dark gray/black)
	ClearColor [4]float32

	// DrawWorld enables 3D world rendering
	DrawWorld bool

	// DrawEntities enables entity rendering
	DrawEntities bool

	// BrushEntities contains inline BSP submodels for renderer parity.
	BrushEntities []BrushEntity

	// AliasEntities contains world-space MDL entities for renderer parity.
	AliasEntities []AliasModelEntity

	// SpriteEntities contains sprite (billboard) entities for renderer parity.
	SpriteEntities []SpriteEntity

	// DecalMarks contains projected world-space mark entities for renderer parity.
	DecalMarks []DecalMarkEntity

	// ViewModel contains the first-person weapon model when active.
	ViewModel *AliasModelEntity

	// LightStyles contains evaluated lightstyle scalars for the current frame.
	LightStyles [64]float32

	// FogColor and FogDensity mirror the authoritative renderer state for parity tracking.
	FogColor   [3]float32
	FogDensity float32

	// DrawParticles enables particle rendering
	DrawParticles bool

	// Draw2DOverlay enables 2D overlay (HUD, menu, console)
	Draw2DOverlay bool

	// MenuActive indicates if menu is currently displayed
	MenuActive bool

	// CSQCDrawHud indicates CSQC is drawing HUD this frame.
	CSQCDrawHud bool

	// Particles is the active particle system to render
	Particles *ParticleSystem

	// Palette for color conversion
	Palette []byte

	// WaterWarp, WaterWarpTime, and ForceUnderwater drive scene-target waterwarp,
	// composite behavior, and related runtime parity checks.
	WaterWarp       bool
	WaterWarpTime   float32
	ForceUnderwater bool

	// VBlend is the composite RGBA screen tint from client color shifts.
	// Applied after the 3D scene and entity passes, before the 2D overlay.
	VBlend [4]float32
}

// RenderFrame executes the complete frame pipeline in order:
// 1. Clear screen
// 2. Draw 3D world / scene target
// 3. Draw entities
// 4. Draw particles / viewmodel / polyblend
// 5. Draw 2D overlay (HUD, menu, console)
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}
	frameStart := time.Now()
	hostSpeeds := cvar.BoolValue("host_speeds")
	phaseStart := time.Time{}
	phaseBegin := func() {
		if hostSpeeds {
			phaseStart = time.Now()
		}
	}
	phaseEnd := func(total *float64) {
		if hostSpeeds {
			*total += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		}
	}
	var clearMS float64
	var worldMS float64
	var entitiesMS float64
	var viewModelMS float64
	var sceneCompositeMS float64
	var polyBlendMS float64
	var overlayMS float64

	slog.Debug("RenderFrame called", "draw_world", state.DrawWorld, "draw_particles", state.DrawParticles, "draw_2d_overlay", state.Draw2DOverlay)
	slog.Debug("RenderFrame: surface view (start)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Debug("RenderFrame: gogpu frame state (start)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
	}

	// Phase 1: Clear screen
	// Skip clear when world rendering is active - the world render pass will handle clearing,
	// and gogpu will use LoadOpLoad to preserve our world rendering when drawing the overlay.
	sceneTargetActive := shouldUseSceneRenderTarget(state) && dc.enableSceneRenderTarget()
	phaseBegin()
	if !state.DrawWorld && !sceneTargetActive {
		// When the in-game menu is up without an active world pass, preserve the
		// previously rendered scene behind the menu instead of force-clearing to black.
		// This matches Quake-style "menu over frozen gameplay" behavior.
		if !state.MenuActive {
			dc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])
		}
	} else if sceneTargetActive && !state.DrawWorld {
		dc.clearCurrentHALRenderTarget(state.ClearColor)
	}
	phaseEnd(&clearMS)

	// Phase 2: Draw 3D world directly to surface view (zero-copy)
	// HAL renders to dc.ctx.SurfaceView() which is the current frame's swapchain texture.
	// Then gogpu draws 2D overlay on top with LoadOpLoad to preserve the world.
	if state.DrawWorld {
		dc.renderer.setGoGPUWorldLightStyleValues(state.LightStyles)
		slog.Debug("RenderFrame: rendering world to surface")
		phaseBegin()
		dc.renderWorld(state)
		phaseEnd(&worldMS)
		slog.Debug("RenderFrame: surface view (after world)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
		if !sceneTargetActive && dc.markGoGPUFrameContentForOverlay() {
			slog.Debug("RenderFrame: marked gogpu frame as pre-populated (HAL world rendered)")
			if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
				slog.Debug("RenderFrame: gogpu frame state (after mark)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
			}
		} else if !sceneTargetActive {
			slog.Warn("RenderFrame: unable to mark gogpu frame state; first 2D draw may clear world")
		}
	}

	// Nuclear debug option: skip all gogpu-side drawing for exactly one frame.
	// This isolates "HAL world render + present" from all 2D/overlay paths.
	if state.DrawWorld && shouldRunHALOnlyFrame() {
		slog.Warn("RenderFrame: HAL-only debug frame enabled; skipping entities/particles/2D overlay")
		dc.logPrePresentState("hal-only")
		return
	}

	var translucentBrushEntities []BrushEntity
	var translucentAliasEntities []AliasModelEntity
	if state.DrawEntities {
		_, translucentBrushEntities = splitBrushEntitiesByAlpha(state.BrushEntities)
		_, translucentAliasEntities = splitAliasEntitiesByAlpha(state.AliasEntities)
	}
	lateTranslucency := shouldRunLateTranslucencyBlock(lateTranslucencyBlockInputs{
		drawWorld:                   state.DrawWorld,
		hasTranslucentWorld:         state.DrawWorld && dc.renderer != nil && dc.renderer.hasTranslucentWorldLiquidFacesGoGPU(),
		drawEntities:                state.DrawEntities,
		hasSpriteEntities:           len(state.SpriteEntities) > 0,
		drawParticles:               state.DrawParticles,
		hasDecalMarks:               len(state.DecalMarks) > 0,
		hasTranslucentBrushEntities: len(translucentBrushEntities) > 0,
		hasTranslucentAliasEntities: len(translucentAliasEntities) > 0,
	})
	dc.maybeLogGoGPUFirstWorldFrameStats(state, lateTranslucency, translucentBrushEntities, translucentAliasEntities)

	if shouldClearGoGPUSharedDepthStencil(gogpuSharedDepthStencilClearInputs{
		drawWorld:         state.DrawWorld,
		drawEntities:      state.DrawEntities,
		hasBrushEntities:  len(state.BrushEntities) > 0,
		hasAliasEntities:  len(state.AliasEntities) > 0,
		hasSpriteEntities: len(state.SpriteEntities) > 0,
		hasParticles:      state.DrawParticles && particleCount(state.Particles) > 0,
		hasDecalMarks:     len(state.DecalMarks) > 0,
		hasViewModel:      state.ViewModel != nil,
	}) {
		dc.clearGoGPUSharedDepthStencil()
	}

	// Phase 3: Draw entities, decals, and mode-placed particles.
	if lateTranslucency || state.DrawEntities || len(state.DecalMarks) > 0 || (state.DrawParticles && state.Particles != nil) {
		phaseBegin()
		dc.renderEntities(state)
		phaseEnd(&entitiesMS)
	}

	if state.DrawEntities && state.ViewModel != nil {
		phaseBegin()
		dc.renderViewModelHAL(*state.ViewModel, state.FogColor, state.FogDensity)
		phaseEnd(&viewModelMS)
	}
	if sceneTargetActive {
		phaseBegin()
		if dc.compositeSceneRenderTarget(state.WaterWarp, state.WaterWarpTime, state.ClearColor) {
			if dc.markGoGPUFrameContentForOverlay() {
				slog.Debug("RenderFrame: marked gogpu frame as pre-populated (scene composite rendered)")
			} else {
				slog.Warn("RenderFrame: unable to mark gogpu frame state after scene composite")
			}
		} else {
			slog.Warn("RenderFrame: failed to composite scene render target")
		}
		phaseEnd(&sceneCompositeMS)
		dc.disableSceneRenderTarget()
	}
	if state.VBlend[3] > 0 {
		phaseBegin()
		dc.renderPolyBlendHAL(state.VBlend)
		phaseEnd(&polyBlendMS)
	}

	// Phase 5: Draw 2D overlay (HUD, menu, console)
	// Re-enable to show menu + fallback dots
	// IMPORTANT: When we skip dc.Clear() above (because state.DrawWorld=true),
	// gogpu should use LoadOpLoad for its internal 2D render pass, preserving
	// the world rendering we just submitted via HAL. This relies on gogpu's
	// internal behavior to detect that Clear() was not called.
	if state.Draw2DOverlay && draw2DOverlay != nil {
		if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
			slog.Debug("RenderFrame: gogpu frame state (pre-overlay)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
		}
		if shouldDrawWorldFallbackDots() {
			slog.Debug("RenderFrame: debug world fallback dots enabled")
			dc.renderWorldFallbackTopDown()
		}
		slog.Debug("Drawing 2D overlay on top of world", "menu_active", state.MenuActive)
		phaseBegin()
		draw2DOverlay(dc)
		dc.flush2DOverlay()
		phaseEnd(&overlayMS)
	}

	dc.logPrePresentState("normal")
	if hostSpeeds {
		slog.Info("render_thread_speeds",
			"clear_ms", clearMS,
			"world_ms", worldMS,
			"entities_ms", entitiesMS,
			"viewmodel_ms", viewModelMS,
			"scene_composite_ms", sceneCompositeMS,
			"polyblend_ms", polyBlendMS,
			"overlay_ms", overlayMS,
			"total_ms", float64(time.Since(frameStart))/float64(time.Millisecond),
			"draw_world", state.DrawWorld,
			"draw_entities", state.DrawEntities,
			"draw_particles", state.DrawParticles,
			"draw_overlay", state.Draw2DOverlay,
			"menu_active", state.MenuActive,
			"water_warp", state.WaterWarp,
		)
	}
}

func (dc *DrawContext) maybeLogGoGPUFirstWorldFrameStats(state *RenderFrameState, lateTranslucency bool, translucentBrushEntities []BrushEntity, translucentAliasEntities []AliasModelEntity) {
	if dc == nil || dc.renderer == nil || state == nil || !state.DrawWorld {
		return
	}
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil {
		return
	}
	if !dc.renderer.worldFirstFrameStatsLogged.CompareAndSwap(false, true) {
		return
	}

	camera := dc.renderer.cameraState
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	visibleFaces := selectVisibleWorldFaces(
		worldData.Geometry.Tree,
		worldData.Geometry.Faces,
		worldData.Geometry.LeafFaces,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
	)
	visibleStats := summarizeGoGPUWorldFaceStats(visibleFaces, liquidAlpha)

	dynamicLightCount := 0
	dc.renderer.mu.RLock()
	if dc.renderer.lightPool != nil {
		dynamicLightCount = dc.renderer.lightPool.ActiveCount()
	}
	dc.renderer.mu.RUnlock()

	particleTotal := 0
	if state.DrawParticles {
		particleTotal = particleCount(state.Particles)
	}
	opaqueBrushEntities := len(state.BrushEntities) - len(translucentBrushEntities)
	if opaqueBrushEntities < 0 {
		opaqueBrushEntities = 0
	}
	opaqueAliasEntities := len(state.AliasEntities) - len(translucentAliasEntities)
	if opaqueAliasEntities < 0 {
		opaqueAliasEntities = 0
	}

	slog.Info("GoGPU first frame stats",
		"alpha_mode", effectiveGoGPUAlphaMode(GetAlphaMode()).String(),
		"visible_faces", visibleStats.TotalFaces,
		"visible_triangles", visibleStats.TotalTriangles,
		"visible_lightmapped_faces", visibleStats.LightmappedFaces,
		"visible_lit_water_faces", visibleStats.LitWaterFaces,
		"visible_turbulent_faces", visibleStats.TurbulentFaces,
		"visible_sky_faces", visibleStats.SkyFaces,
		"visible_opaque_faces", visibleStats.OpaqueFaces,
		"visible_alpha_test_faces", visibleStats.AlphaTestFaces,
		"visible_opaque_liquid_faces", visibleStats.OpaqueLiquidFaces,
		"visible_translucent_liquid_faces", visibleStats.TranslucentLiquidFaces,
		"world_faces_total", worldData.TotalFaces,
		"world_triangles_total", worldData.TotalIndices/3,
		"lightmap_pages", len(worldData.Geometry.Lightmaps),
		"brush_entities", len(state.BrushEntities),
		"brush_entities_opaque", opaqueBrushEntities,
		"brush_entities_translucent", len(translucentBrushEntities),
		"alias_entities", len(state.AliasEntities),
		"alias_entities_opaque", opaqueAliasEntities,
		"alias_entities_translucent", len(translucentAliasEntities),
		"sprite_entities", len(state.SpriteEntities),
		"decal_marks", len(state.DecalMarks),
		"particles", particleTotal,
		"dynamic_lights", dynamicLightCount,
		"late_translucency", lateTranslucency,
		"menu_active", state.MenuActive,
		"view_model", state.ViewModel != nil,
	)
}

func shouldRunHALOnlyFrame() bool {
	if os.Getenv("IRONWAIL_DEBUG_HAL_ONLY_FRAME") != "1" {
		return false
	}

	return !halOnlyFrameConsumed.Swap(true)
}

func (dc *DrawContext) logPrePresentState(mode string) {
	if dc == nil || dc.ctx == nil {
		return
	}

	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Debug("RenderFrame: gogpu frame state (pre-present)",
			"mode", mode,
			"frameCleared", frameCleared,
			"hasPendingClear", hasPendingClear,
		)
	} else {
		slog.Warn("RenderFrame: unable to read gogpu frame state (pre-present)", "mode", mode)
	}

	slog.Debug("RenderFrame: surface view (pre-present)", "mode", mode, "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
}

func debugSurfaceViewID(view any) string {
	if view == nil {
		return "nil"
	}

	value := reflect.ValueOf(view)
	if value.Kind() == reflect.Pointer || value.Kind() == reflect.UnsafePointer {
		if value.IsNil() {
			return fmt.Sprintf("%T@nil", view)
		}
		return fmt.Sprintf("%T@0x%x", view, value.Pointer())
	}

	return fmt.Sprintf("%T@%p", view, &view)
}

func shouldDrawWorldFallbackDots() bool {
	return os.Getenv("IRONWAIL_DEBUG_WORLD_DOTS") == "1"
}

// markGoGPUFrameContentForOverlay sets gogpu renderer's internal frame state
// so the first 2D draw pass uses LoadOpLoad (preserve existing surface content)
// rather than defaulting to LoadOpClear.
func (dc *DrawContext) markGoGPUFrameContentForOverlay() bool {
	if dc == nil || dc.ctx == nil {
		return false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false
	}

	frameClearedWritable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearWritable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()

	if frameClearedWritable.Kind() != reflect.Bool || hasPendingClearWritable.Kind() != reflect.Bool {
		return false
	}

	frameClearedWritable.SetBool(true)
	hasPendingClearWritable.SetBool(false)

	return true
}

func (dc *DrawContext) getGoGPUFrameStateForDebug() (frameCleared bool, hasPendingClear bool, ok bool) {
	if dc == nil || dc.ctx == nil {
		return false, false, false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false, false, false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false, false, false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false, false, false
	}

	frameClearedReadable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearReadable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()
	if frameClearedReadable.Kind() != reflect.Bool || hasPendingClearReadable.Kind() != reflect.Bool {
		return false, false, false
	}

	return frameClearedReadable.Bool(), hasPendingClearReadable.Bool(), true
}

// renderWorldFallbackTopDown draws a sparse top-down projection of world
// vertices using the 2D API. It is used as a visibility fallback while the
// GPU world pass is being stabilized across platforms.
func (dc *DrawContext) renderWorldFallbackTopDown() {
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil || len(worldData.Geometry.Vertices) == 0 {
		return
	}

	verts := worldData.Geometry.Vertices
	minX, maxX := verts[0].Position[0], verts[0].Position[0]
	minY, maxY := verts[0].Position[1], verts[0].Position[1]
	for index := 1; index < len(verts); index++ {
		x := verts[index].Position[0]
		y := verts[index].Position[1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1 || rangeY < 1 {
		return
	}

	screenW, screenH := dc.renderer.Size()
	if screenW <= 0 || screenH <= 0 {
		return
	}

	const margin = 24.0
	drawW := float32(screenW) - margin*2
	drawH := float32(screenH) - margin*2
	if drawW <= 0 || drawH <= 0 {
		return
	}

	scaleX := drawW / rangeX
	scaleY := drawH / rangeY
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	step := len(verts) / 5000
	if step < 1 {
		step = 1
	}

	// Draw a visible frame so non-black background is obvious.
	frameColor := byte(250)
	panelColor := byte(48)
	dc.DrawFill(int(margin), int(margin), int(drawW), int(drawH), panelColor)
	dc.DrawFill(int(margin), int(margin), int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin+drawH)-2, int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin), 2, int(drawH), frameColor)
	dc.DrawFill(int(margin+drawW)-2, int(margin), 2, int(drawH), frameColor)

	for index := 0; index < len(verts); index += step {
		v := verts[index]
		sx := int(margin + (v.Position[0]-minX)*scale)
		sy := int(float32(screenH) - (margin + (v.Position[1]-minY)*scale))
		if sx < 0 || sx >= screenW || sy < 0 || sy >= screenH {
			continue
		}
		dc.DrawFill(sx, sy, 2, 2, 251)
	}
}

// renderWorld draws the 3D world geometry (BSP surfaces).
// Implementation delegates to renderWorldInternal in world.go.
func (dc *DrawContext) renderWorld(state *RenderFrameState) {
	// Render world directly to the current surface view (zero-copy approach)
	// The surface view is available from dc.ctx.SurfaceView() per gogpu's design
	dc.renderWorldInternal(state)
}

// Note: ensureWorldRenderTarget removed - we now render directly to the surface view
// provided by gogpu via dc.ctx.SurfaceView() for zero-copy rendering.

// renderEntities draws runtime entities. Alias models, sprites, and decals use
// HAL-backed paths.
func (dc *DrawContext) renderEntities(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}
	hasTranslucentWorld := state.DrawWorld && dc.renderer.hasTranslucentWorldLiquidFacesGoGPU()
	hostSpeeds := cvar.BoolValue("host_speeds")
	particlePhase, hasParticlePhase := classifyGoGPUParticlePhase(readGoGPUParticleModeCvar(), particleCount(state.Particles))
	plan := planGoGPUEntityDrawOrder(state.DrawEntities, hasTranslucentWorld, state.BrushEntities, state.AliasEntities, state.SpriteEntities, state.DecalMarks, particlePhase, hasParticlePhase)
	var (
		opaqueBrushMS          float64
		opaqueAliasMS          float64
		opaqueParticlesMS      float64
		skyBrushMS             float64
		opaqueLiquidBrushMS    float64
		translucentWorldMS     float64
		translucentLiquidMS    float64
		alphaTestBrushMS       float64
		translucentBrushMS     float64
		translucencyFlushMS    float64
		decalsMS               float64
		translucentAliasMS     float64
		spritesMS              float64
		translucentParticlesMS float64
	)
	var pendingTranslucentRenders []gogpuTranslucentBrushFaceRender
	var pendingTransientBuffers []*wgpu.Buffer
	flushPendingTranslucency := func() {
		if len(pendingTranslucentRenders) == 0 {
			destroyGoGPUTransientBuffers(pendingTransientBuffers)
			pendingTransientBuffers = nil
			return
		}
		sortGoGPUTranslucentBrushFaceRenders(effectiveGoGPUAlphaMode(GetAlphaMode()), pendingTranslucentRenders)
		dc.renderGoGPUSortedTranslucentFaceRendersHAL(pendingTranslucentRenders, state.FogColor, state.FogDensity)
		destroyGoGPUTransientBuffers(pendingTransientBuffers)
		pendingTranslucentRenders = nil
		pendingTransientBuffers = nil
	}
	for _, phase := range plan.phases {
		switch phase {
		case gogpuEntityPhaseTranslucentWorldLiquid, gogpuEntityPhaseTranslucentLiquidBrush, gogpuEntityPhaseTranslucentBrush:
		default:
			flushStart := time.Now()
			flushPendingTranslucency()
			translucencyFlushMS += float64(time.Since(flushStart)) / float64(time.Millisecond)
		}
		switch phase {
		case gogpuEntityPhaseOpaqueBrush:
			phaseStart := time.Now()
			dc.renderOpaqueBrushEntitiesHAL(plan.opaqueBrush, state.FogColor, state.FogDensity)
			opaqueBrushMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseOpaqueAlias:
			phaseStart := time.Now()
			for _, step := range gogpuOpaqueAliasPassSteps() {
				switch step {
				case gogpuOpaqueAliasStepEntities:
					dc.renderAliasEntitiesHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				case gogpuOpaqueAliasStepShadows:
					dc.renderAliasShadowsHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				}
			}
			opaqueAliasMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseOpaqueParticles:
			if state.DrawParticles && state.Particles != nil {
				phaseStart := time.Now()
				dc.renderParticlesHAL(state, false)
				opaqueParticlesMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
			}
		case gogpuEntityPhaseSkyBrush:
			phaseStart := time.Now()
			dc.renderSkyBrushEntitiesHAL(plan.skyBrush, state.FogColor, state.FogDensity)
			skyBrushMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseOpaqueLiquidBrush:
			phaseStart := time.Now()
			dc.renderOpaqueLiquidBrushEntitiesHAL(plan.opaqueBrush, state.FogColor, state.FogDensity)
			opaqueLiquidBrushMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseTranslucentWorldLiquid:
			phaseStart := time.Now()
			pendingTranslucentRenders = append(pendingTranslucentRenders, dc.collectGoGPUWorldTranslucentLiquidFaceRenders()...)
			translucentWorldMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseTranslucentLiquidBrush:
			phaseStart := time.Now()
			renders, buffers := dc.collectGoGPUTranslucentLiquidBrushFaceRenders(plan.opaqueBrush)
			pendingTranslucentRenders = append(pendingTranslucentRenders, renders...)
			pendingTransientBuffers = append(pendingTransientBuffers, buffers...)
			translucentLiquidMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseTranslucentBrush:
			collectStart := time.Now()
			alphaTestRenders, renders, buffers := dc.collectGoGPUTranslucentBrushEntityFaceRenders(plan.translucentBrush)
			translucentBrushMS += float64(time.Since(collectStart)) / float64(time.Millisecond)
			alphaTestStart := time.Now()
			dc.renderGoGPUAlphaTestBrushFaceRendersHAL(alphaTestRenders, state.FogColor, state.FogDensity)
			alphaTestBrushMS += float64(time.Since(alphaTestStart)) / float64(time.Millisecond)
			pendingTranslucentRenders = append(pendingTranslucentRenders, renders...)
			pendingTransientBuffers = append(pendingTransientBuffers, buffers...)
		case gogpuEntityPhaseDecals:
			phaseStart := time.Now()
			dc.renderDecalMarksHAL(state.DecalMarks)
			decalsMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseTranslucentAlias:
			phaseStart := time.Now()
			dc.renderAliasEntitiesHAL(plan.translucentAlias, state.FogColor, state.FogDensity)
			translucentAliasMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseSprites:
			phaseStart := time.Now()
			dc.renderSpriteEntitiesHAL(state.SpriteEntities, state.FogColor, state.FogDensity)
			spritesMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		case gogpuEntityPhaseTranslucentParticles:
			if state.DrawParticles && state.Particles != nil {
				phaseStart := time.Now()
				dc.renderParticlesHAL(state, true)
				translucentParticlesMS += float64(time.Since(phaseStart)) / float64(time.Millisecond)
			}
		}
	}
	flushStart := time.Now()
	flushPendingTranslucency()
	translucencyFlushMS += float64(time.Since(flushStart)) / float64(time.Millisecond)
	if hostSpeeds {
		slog.Info("render_entities_speeds",
			"opaque_brush_ms", opaqueBrushMS,
			"opaque_alias_ms", opaqueAliasMS,
			"opaque_particles_ms", opaqueParticlesMS,
			"sky_brush_ms", skyBrushMS,
			"opaque_liquid_brush_ms", opaqueLiquidBrushMS,
			"translucent_world_collect_ms", translucentWorldMS,
			"translucent_liquid_collect_ms", translucentLiquidMS,
			"alpha_test_brush_ms", alphaTestBrushMS,
			"translucent_brush_collect_ms", translucentBrushMS,
			"translucency_flush_ms", translucencyFlushMS,
			"decals_ms", decalsMS,
			"translucent_alias_ms", translucentAliasMS,
			"sprites_ms", spritesMS,
			"translucent_particles_ms", translucentParticlesMS,
			"opaque_brush_count", len(plan.opaqueBrush),
			"opaque_alias_count", len(plan.opaqueAlias),
			"translucent_brush_count", len(plan.translucentBrush),
			"translucent_alias_count", len(plan.translucentAlias),
			"sprite_count", len(state.SpriteEntities),
			"decal_count", len(state.DecalMarks),
			"particle_count", particleCount(state.Particles),
		)
	}
}

func projectWorldPointToScreen(pos [3]float32, vp types.Mat4, screenW, screenH int) (x int, y int, ok bool) {
	if screenW <= 0 || screenH <= 0 {
		return 0, 0, false
	}

	clipPos := TransformVertex(pos, vp)
	if clipPos.W <= 0.001 {
		return 0, 0, false
	}

	invW := float32(1.0) / clipPos.W
	ndcX := clipPos.X * invW
	ndcY := clipPos.Y * invW
	ndcZ := clipPos.Z * invW

	if ndcX < -1 || ndcX > 1 || ndcY < -1 || ndcY > 1 || ndcZ < -1 || ndcZ > 1 {
		return 0, 0, false
	}

	screenX := (ndcX*0.5 + 0.5) * float32(screenW-1)
	screenY := (1 - (ndcY*0.5 + 0.5)) * float32(screenH-1)

	return int(screenX), int(screenY), true
}

type projectedParticleMarker struct {
	x     int
	y     int
	color byte
	size  int
	alpha float32
}

func projectParticleMarkers(particles []Particle, verts []ParticleVertex, vp types.Mat4, screenW, screenH int) []projectedParticleMarker {
	if len(particles) == 0 || len(verts) == 0 {
		return nil
	}
	count := len(particles)
	if len(verts) < count {
		count = len(verts)
	}
	markers := make([]projectedParticleMarker, 0, count)
	for i := 0; i < count; i++ {
		x, y, ok := projectWorldPointToScreen(verts[i].Pos, vp, screenW, screenH)
		if !ok {
			continue
		}
		markers = append(markers, projectedParticleMarker{
			x:     x,
			y:     y,
			color: particles[i].Color,
			size:  4,
			alpha: 1,
		})
	}
	return markers
}

func readGoGPUParticleModeCvar() int {
	cv := cvar.Get(CvarRParticles)
	if cv == nil {
		return 1
	}
	return cv.Int
}

func shouldDrawGoGPUParticles(mode, activeParticles int) bool {
	return ShouldDrawParticles(mode, false, false, activeParticles) || ShouldDrawParticles(mode, true, false, activeParticles)
}

func particleCount(ps *ParticleSystem) int {
	if ps == nil {
		return 0
	}
	return ps.ActiveCount()
}

// buildParticlePalette converts a 768-byte palette to the [256][4]byte format
// expected by BuildParticleVertices.
func buildParticlePalette(palette []byte) [256][4]byte {
	var p [256][4]byte
	if len(palette) < 768 {
		// Grayscale fallback
		for i := range p {
			p[i] = [4]byte{byte(i), byte(i), byte(i), 255}
		}
		return p
	}
	for i := range p {
		offset := i * 3
		p[i] = [4]byte{
			palette[offset],
			palette[offset+1],
			palette[offset+2],
			255,
		}
	}
	return p
}

// DefaultRenderFrameState returns a sensible default RenderFrameState.
func DefaultRenderFrameState() *RenderFrameState {
	return &RenderFrameState{
		ClearColor:    [4]float32{0.0, 0.0, 0.0, 1.0}, // Black
		DrawWorld:     false,                          // Disabled until M4.3
		DrawEntities:  false,                          // Disabled until M4.4
		DrawParticles: false,                          // Opt-in per frame like the primary renderer
		Draw2DOverlay: true,                           // Always draw 2D overlay
		MenuActive:    true,                           // Menu is active at startup
	}
}
