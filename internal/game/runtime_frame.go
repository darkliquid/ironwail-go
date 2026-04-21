package game

import (
	"fmt"
	"log/slog"
	"sync"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

type runtimeRendererLoopResult struct {
	ScreenshotCaptured bool
	ScreenshotErr      error
	HandledFallback    bool
}

type runtimeRendererLoopState struct {
	startupOpts        startupOptions
	screenshotPath     string
	screenshotMode     bool
	screenshotCaptured bool
	screenshotErr      error

	pendingRendererMu     sync.Mutex
	pendingRendererDT     float64
	pendingRendererEvents cl.TransientEvents
}

func (s *runtimeRendererLoopState) storePendingRendererFrame(dt float64, transientEvents cl.TransientEvents) {
	s.pendingRendererMu.Lock()
	s.pendingRendererDT = dt
	s.pendingRendererEvents = transientEvents
	s.pendingRendererMu.Unlock()
}

func (s *runtimeRendererLoopState) pendingRendererFrame() (float64, cl.TransientEvents) {
	s.pendingRendererMu.Lock()
	defer s.pendingRendererMu.Unlock()
	return s.pendingRendererDT, s.pendingRendererEvents
}

func (g *Game) RunRuntimeRendererLoop(startupOpts StartupOptions, screenshotPath string) (runtimeRendererLoopResult, error) {
	result := runtimeRendererLoopResult{}
	state := &runtimeRendererLoopState{
		startupOpts:    startupOpts,
		screenshotPath: screenshotPath,
		screenshotMode: screenshotPath != "",
	}

	g.installRuntimeRendererCallbacks(gameCallbacks{g: g}, state)
	g.prepareRuntimeRendererScreenshot(state.screenshotMode)

	slog.Info("frame loop started")
	runErr := g.Renderer.Run()
	if runErr != nil {
		if g.Renderer != nil {
			g.Renderer.Stop()
		}
		if g.isRendererError(runErr) {
			fmt.Println("WARNING: Render loop failed. Falling back to headless mode.")
			fmt.Printf("Error: %v\n", runErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			g.HeadlessGameLoop()
			result.HandledFallback = true
			return result, nil
		}
		return result, fmt.Errorf("Render loop failed: %w", runErr)
	}

	if state.screenshotMode {
		result.ScreenshotCaptured = true
		result.ScreenshotErr = state.screenshotErr
	}
	return result, nil
}

func (g *Game) prepareRuntimeRendererScreenshot(screenshotMode bool) {
	if !screenshotMode {
		return
	}

	cmdsys.Execute()
	if g.Host != nil && g.Server != nil {
		_ = g.Server.Frame(0.05)
	}
}

func (g *Game) installRuntimeRendererCallbacks(cb gameCallbacks, state *runtimeRendererLoopState) {
	g.Renderer.OnUpdate(func(dt float64) {
		g.pollRuntimeInputEvents()
		if g.Input != nil {
			g.syncGameplayInputMode()
			g.applyMenuMouseMove()
			g.applyGameplayMouseLook()
			g.updateRuntimeTextEditRepeat(dt)
		}

		consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole
		g.updateRuntimeConsoleSlide(dt, consoleVisible, g.runtimeConsoleForcedUp())

		transientEvents := g.RunRuntimeFrame(dt, cb)
		if g.Host != nil && g.Host.IsAborted() {
			if g.Renderer != nil {
				g.Renderer.Stop()
			}
			return
		}

		g.syncRuntimeVisualEffects(dt, transientEvents)
		state.storePendingRendererFrame(dt, transientEvents)
	})

	g.Renderer.OnDraw(func(dc renderer.RenderContext) {
		runtimeStateMu.Lock()
		defer runtimeStateMu.Unlock()

		if state.screenshotMode && !state.screenshotCaptured {
			defer g.captureRuntimeRendererScreenshot(state)
		}

		g.applyRuntimeRendererState(state)
		g.uploadDeferredRuntimeWorld()
		g.drawRuntimeRendererFrame(dc)
	})
}

func (g *Game) captureRuntimeRendererScreenshot(state *runtimeRendererLoopState) {
	state.screenshotCaptured = true
	state.screenshotErr = g.CaptureScreenshot(state.screenshotPath, state.startupOpts.BaseDir, state.startupOpts.GameDir)
	if g.Renderer != nil {
		g.Renderer.Stop()
	}
}

func (g *Game) applyRuntimeRendererState(state *runtimeRendererLoopState) {
	if g.Renderer == nil {
		return
	}

	g.ApplyQueuedRendererAssets()
	renderDT, renderEvents := state.pendingRendererFrame()
	origin, angles := g.runtimeViewState()
	camera := g.runtimeCameraState(origin, angles)
	g.Renderer.UpdateCamera(camera, 0.1, 4096.0)
	g.applyRuntimeRendererVisualEffects(renderDT, g.Renderer, renderEvents)
	g.applyRuntimeRendererSkybox(g.Renderer)
}

func (g *Game) uploadDeferredRuntimeWorld() {
	if g.Renderer == nil || g.Server == nil || g.Server.WorldTree == nil {
		return
	}
	if !g.shouldUploadRuntimeWorld(g.WorldUploadKey, g.Server.ModelName, g.Renderer.HasWorldData()) {
		return
	}

	if err := g.Renderer.UploadWorld(g.Server.WorldTree); err != nil {
		slog.Warn("deferred world upload failed", "error", err)
		return
	}
	g.WorldUploadKey = g.Server.ModelName
}

func (g *Game) shouldUploadRuntimeWorld(uploadedKey, targetKey string, hasWorldData bool) bool {
	if targetKey == "" {
		return false
	}
	if !hasWorldData {
		return true
	}
	return uploadedKey != targetKey
}

func (g *Game) drawRuntimeRendererFrame(dc renderer.RenderContext) {
	brushEntities := g.collectBrushEntities()
	aliasEntities := g.collectAliasEntities()
	spriteEntities := g.collectSpriteEntities()
	viewModel := g.collectViewModelEntity()

	if drawCtx, ok := dc.(*renderer.DrawContext); ok {
		state := g.buildRuntimeRenderFrameState(brushEntities, aliasEntities, spriteEntities, viewModel)
		drawCtx.RenderFrame(state, func(overlay renderer.RenderContext) {
			g.drawRuntimeOverlayFrame(overlay)
		})
		return
	}

	g.drawRuntimeFallbackFrame(dc)
}

func (g *Game) drawRuntimeOverlayFrame(overlay renderer.RenderContext) {
	w, h := g.Renderer.Size()
	consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole
	if setter, ok := overlay.(CanvasParamSetter); ok {
		setter.SetCanvasParams(g.runtimeOverlayCanvasParams(w, h))
	}

	conForcedup := g.runtimeConsoleForcedUp()
	overlay.SetCanvas(renderer.CanvasDefault)

	if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
		overlay.SetCanvas(renderer.CanvasMenu)
		g.drawLoadingPlaque(overlay, g.Draw)
		if consoleVisible {
			g.drawRuntimeConsole(overlay, w, h, true, false)
		}
		return
	}

	if conForcedup {
		g.drawRuntimeConsole(overlay, w, h, true, true)
	}

	if g.Menu != nil && g.Menu.IsActive() {
		g.drawRuntimeMenu(overlay, w, h, g.Menu.M_Draw)
		telemetryState := g.buildRuntimeTelemetryState(conForcedup)
		telemetryState.ViewRect = g.runtimeOverlayViewRect(w, h, false)
		g.drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
		g.drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
		return
	}

	if !conForcedup {
		telemetryState := g.buildRuntimeTelemetryState(conForcedup)
		g.drawRuntimeHUDLayer(overlay, w, h, &telemetryState)
		g.drawRuntimeClock(overlay, telemetryState)
		g.drawRuntimeDemoControls(overlay, g.Draw, telemetryState, &g.DemoOverlay)
		g.drawRuntimeSpeed(overlay, telemetryState, &g.SpeedOverlay)
		g.drawRuntimeNet(overlay, g.Draw, telemetryState)
		g.drawRuntimeTurtle(overlay, g.Draw, telemetryState, &g.TurtleOverlayCount)
		if g.runtimePauseActive() {
			g.drawPauseOverlay(overlay, g.Draw)
		}

		if consoleVisible || g.runtimeConsoleAnimating() {
			g.drawRuntimeConsole(overlay, w, h, true, false)
			if consoleVisible {
				return
			}
		}

		if !g.runtimeConsoleAnimating() {
			g.drawRuntimeConsole(overlay, w, h, false, false)
		}

		if g.Input != nil && g.Input.GetKeyDest() == input.KeyMessage && !g.runtimeConsoleAnimating() {
			g.drawChatInput(overlay, w, h)
		}
	}

	telemetryState := g.buildRuntimeTelemetryState(conForcedup)
	telemetryState.ViewRect = g.runtimeOverlayViewRect(w, h, false)
	g.drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
	g.drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
}

func (g *Game) drawRuntimeFallbackFrame(dc renderer.RenderContext) {
	dc.Clear(0, 0, 0, 1)
	dc.SetCanvas(renderer.CanvasDefault)

	w, h := g.Renderer.Size()
	if setter, ok := dc.(CanvasParamSetter); ok {
		setter.SetCanvasParams(g.runtimeOverlayCanvasParams(w, h))
	}
	if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
		g.drawLoadingPlaque(dc, g.Draw)
		return
	}

	conForcedup := g.runtimeConsoleForcedUp()
	if g.Menu != nil && g.Menu.IsActive() {
		g.drawRuntimeMenu(dc, w, h, g.Menu.M_Draw)
	} else if !conForcedup && g.runtimePauseActive() {
		g.drawPauseOverlay(dc, g.Draw)
	}
}
