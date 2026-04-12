package main

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

func runRuntimeRendererLoop(startupOpts startupOptions, screenshotPath string) (runtimeRendererLoopResult, error) {
	result := runtimeRendererLoopResult{}
	state := &runtimeRendererLoopState{
		startupOpts:    startupOpts,
		screenshotPath: screenshotPath,
		screenshotMode: screenshotPath != "",
	}

	installRuntimeRendererCallbacks(gameCallbacks{}, state)
	prepareRuntimeRendererScreenshot(state.screenshotMode)

	slog.Info("frame loop started")
	runErr := g.Renderer.Run()
	if runErr != nil {
		releaseRuntimeRenderer()
		if isRendererError(runErr) {
			fmt.Println("WARNING: Render loop failed. Falling back to headless mode.")
			fmt.Printf("Error: %v\n", runErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headlessGameLoop()
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

func prepareRuntimeRendererScreenshot(screenshotMode bool) {
	if !screenshotMode {
		return
	}

	cmdsys.Execute()
	if g.Host != nil && g.Server != nil {
		_ = g.Server.Frame(0.05)
	}
}

func installRuntimeRendererCallbacks(cb gameCallbacks, state *runtimeRendererLoopState) {
	g.Renderer.OnUpdate(func(dt float64) {
		pollRuntimeInputEvents()
		if g.Input != nil {
			syncGameplayInputMode()
			applyMenuMouseMove()
			applyGameplayMouseLook()
			updateRuntimeTextEditRepeat(dt)
		}

		consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole
		updateRuntimeConsoleSlide(dt, consoleVisible, runtimeConsoleForcedUp())

		transientEvents := runRuntimeFrame(dt, cb)
		if g.Host != nil && g.Host.IsAborted() {
			if g.Renderer != nil {
				g.Renderer.Stop()
			}
			return
		}

		syncRuntimeVisualEffects(dt, transientEvents)
		state.storePendingRendererFrame(dt, transientEvents)
	})

	g.Renderer.OnDraw(func(dc renderer.RenderContext) {
		runtimeStateMu.Lock()
		defer runtimeStateMu.Unlock()

		if state.screenshotMode && !state.screenshotCaptured {
			defer captureRuntimeRendererScreenshot(state)
		}

		applyRuntimeRendererState(state)
		uploadDeferredRuntimeWorld()
		drawRuntimeRendererFrame(dc)
	})
}

func captureRuntimeRendererScreenshot(state *runtimeRendererLoopState) {
	state.screenshotCaptured = true
	state.screenshotErr = captureScreenshot(state.screenshotPath, state.startupOpts.BaseDir, state.startupOpts.GameDir)
	if g.Renderer != nil {
		g.Renderer.Stop()
	}
}

func applyRuntimeRendererState(state *runtimeRendererLoopState) {
	if g.Renderer == nil {
		return
	}

	applyQueuedRuntimeRendererAssets(g.Renderer)
	renderDT, renderEvents := state.pendingRendererFrame()
	origin, angles := runtimeViewState()
	camera := runtimeCameraState(origin, angles)
	g.Renderer.UpdateCamera(camera, 0.1, 4096.0)
	applyRuntimeRendererVisualEffects(renderDT, g.Renderer, renderEvents)
	applyRuntimeRendererSkybox(g.Renderer)
}

func uploadDeferredRuntimeWorld() {
	if g.Renderer == nil || g.Server == nil || g.Server.WorldTree == nil {
		return
	}
	if !shouldUploadRuntimeWorld(g.WorldUploadKey, g.Server.ModelName, g.Renderer.HasWorldData()) {
		return
	}

	if err := g.Renderer.UploadWorld(g.Server.WorldTree); err != nil {
		slog.Warn("deferred world upload failed", "error", err)
		return
	}
	g.WorldUploadKey = g.Server.ModelName
}

func shouldUploadRuntimeWorld(uploadedKey, targetKey string, hasWorldData bool) bool {
	if targetKey == "" {
		return false
	}
	if !hasWorldData {
		return true
	}
	return uploadedKey != targetKey
}

func drawRuntimeRendererFrame(dc renderer.RenderContext) {
	brushEntities := collectBrushEntities()
	aliasEntities := collectAliasEntities()
	spriteEntities := collectSpriteEntities()
	viewModel := collectViewModelEntity()

	if drawCtx, ok := dc.(*renderer.DrawContext); ok {
		state := buildRuntimeRenderFrameState(brushEntities, aliasEntities, spriteEntities, viewModel)
		drawCtx.RenderFrame(state, func(overlay renderer.RenderContext) {
			drawRuntimeOverlayFrame(overlay)
		})
		return
	}

	drawRuntimeFallbackFrame(dc)
}

func drawRuntimeOverlayFrame(overlay renderer.RenderContext) {
	w, h := g.Renderer.Size()
	consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole
	if setter, ok := overlay.(canvasParamSetter); ok {
		setter.SetCanvasParams(runtimeOverlayCanvasParams(w, h))
	}

	conForcedup := runtimeConsoleForcedUp()
	overlay.SetCanvas(renderer.CanvasDefault)

	if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
		overlay.SetCanvas(renderer.CanvasMenu)
		drawLoadingPlaque(overlay, g.Draw)
		if consoleVisible {
			drawRuntimeConsole(overlay, w, h, true, false)
		}
		return
	}

	if conForcedup {
		drawRuntimeConsole(overlay, w, h, true, true)
	}

	if g.Menu != nil && g.Menu.IsActive() {
		drawRuntimeMenu(overlay, w, h, g.Menu.M_Draw)
		telemetryState := buildRuntimeTelemetryState(conForcedup)
		telemetryState.ViewRect = runtimeOverlayViewRect(w, h, false)
		drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
		drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
		return
	}

	if !conForcedup {
		telemetryState := buildRuntimeTelemetryState(conForcedup)
		drawRuntimeHUDLayer(overlay, w, h, &telemetryState)
		drawRuntimeClock(overlay, telemetryState)
		drawRuntimeDemoControls(overlay, g.Draw, telemetryState, &g.DemoOverlay)
		drawRuntimeSpeed(overlay, telemetryState, &g.SpeedOverlay)
		drawRuntimeNet(overlay, g.Draw, telemetryState)
		drawRuntimeTurtle(overlay, g.Draw, telemetryState, &g.TurtleOverlayCount)
		if runtimePauseActive() {
			drawPauseOverlay(overlay, g.Draw)
		}

		if consoleVisible || runtimeConsoleAnimating() {
			drawRuntimeConsole(overlay, w, h, true, false)
			if consoleVisible {
				return
			}
		}

		if !runtimeConsoleAnimating() {
			drawRuntimeConsole(overlay, w, h, false, false)
		}

		if g.Input != nil && g.Input.GetKeyDest() == input.KeyMessage && !runtimeConsoleAnimating() {
			drawChatInput(overlay, w, h)
		}
	}

	telemetryState := buildRuntimeTelemetryState(conForcedup)
	telemetryState.ViewRect = runtimeOverlayViewRect(w, h, false)
	drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
	drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
}

func drawRuntimeFallbackFrame(dc renderer.RenderContext) {
	dc.Clear(0, 0, 0, 1)
	dc.SetCanvas(renderer.CanvasDefault)

	w, h := g.Renderer.Size()
	if setter, ok := dc.(canvasParamSetter); ok {
		setter.SetCanvasParams(runtimeOverlayCanvasParams(w, h))
	}
	if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
		drawLoadingPlaque(dc, g.Draw)
		return
	}

	conForcedup := runtimeConsoleForcedUp()
	if g.Menu != nil && g.Menu.IsActive() {
		drawRuntimeMenu(dc, w, h, g.Menu.M_Draw)
	} else if !conForcedup && runtimePauseActive() {
		drawPauseOverlay(dc, g.Draw)
	}
}
