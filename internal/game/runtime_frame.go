package game

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	cl "github.com/darkliquid/ironwail-go/internal/client"
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

	slog.Debug("frame loop started")
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
		return result, fmt.Errorf("render loop failed: %w", runErr)
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

	g.Host.Cmd.Execute()
	if g.Host != nil && g.Server != nil {
		_ = g.Server.Frame(0.05)
	}
}

func (g *Game) installRuntimeRendererCallbacks(cb gameCallbacks, state *runtimeRendererLoopState) {
	var paritySetupDone bool
	g.Renderer.OnUpdate(func(dt float64) {
		if os.Getenv("PARITY_RUN") == "1" && !paritySetupDone && g.Client != nil && g.Client.State == cl.StateActive && g.Host.SignOns() == 4 {
			if g.Menu != nil && g.Menu.IsActive() {
				g.Menu.HideMenu()
			}
			if g.Input != nil && g.Input.KeyDest() == input.KeyConsole {
				g.Input.SetKeyDest(input.KeyGame)
			}
			g.Host.Cmd.AddText("noclip\n")
			// Wait a few command frames so server fixangle reaches the local client
			// before signaling readiness to the external screenshot harness.
			g.Host.Cmd.AddText(fmt.Sprintf("setpos %s %s\nwait\nwait\nwait\nwait\nwait\nviewpos\necho PARITY_READY\n", os.Getenv("PARITY_POS"), os.Getenv("PARITY_ANGLES")))
			paritySetupDone = true
		}

		g.pollRuntimeInputEvents()
		if g.Input != nil {
			g.syncGameplayInputMode()
			g.applyMenuMouseMove()
			g.applyGameplayMouseLook()
			g.updateRuntimeTextEditRepeat(dt)
		}

		consoleVisible := g.Input != nil && g.Input.KeyDest() == input.KeyConsole
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
		g.runtimeMu.Lock()
		defer g.runtimeMu.Unlock()

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
	g.applyParityViewAnglesOverride()

	g.ApplyQueuedRendererAssets()
	renderDT, renderEvents := state.pendingRendererFrame()
	origin, angles := g.runtimeViewState()
	camera := g.runtimeCameraState(origin, angles)
	g.Renderer.UpdateCamera(camera, 0.1, 65536.0)
	g.applyRuntimeRendererVisualEffects(renderDT, g.Renderer, renderEvents)
	g.applyRuntimeRendererSkybox(g.Renderer)
}

func (g *Game) applyParityViewAnglesOverride() {
	if os.Getenv("PARITY_RUN") != "1" || g.Client == nil || g.Client.State != cl.StateActive {
		return
	}
	angles, ok := parseParityAnglesEnv(os.Getenv("PARITY_ANGLES"))
	if !ok {
		return
	}
	g.Client.ViewAngles = angles
	g.Client.MViewAngles[0] = angles
	g.Client.MViewAngles[1] = angles
	g.Client.PendingCmd.ViewAngles = angles
}

func parseParityAnglesEnv(raw string) ([3]float32, bool) {
	fields := strings.Fields(raw)
	if len(fields) != 3 {
		return [3]float32{}, false
	}
	var pitch, yaw, roll float32
	if _, err := fmt.Sscanf(strings.Join(fields, " "), "%f %f %f", &pitch, &yaw, &roll); err != nil {
		return [3]float32{}, false
	}
	return [3]float32{pitch, yaw, roll}, true
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
	consoleVisible := g.Input != nil && g.Input.KeyDest() == input.KeyConsole
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

	menuActive := g.Menu != nil && g.Menu.IsActive()

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
				// Console fully covers the rest; menu/chat still handled below.
				telemetryState := g.buildRuntimeTelemetryState(conForcedup)
				telemetryState.ViewRect = g.runtimeOverlayViewRect(w, h, false)
				g.drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
				g.drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
				return
			}
		}

		if !g.runtimeConsoleAnimating() {
			g.drawRuntimeConsole(overlay, w, h, false, false)
		}

		if g.Input != nil && g.Input.KeyDest() == input.KeyMessage && !g.runtimeConsoleAnimating() {
			g.drawChatInput(overlay, w, h)
		}
	}

	// Draw menu on top of HUD (mirrors C gl_screen.c: Sbar_Draw() then M_Draw()).
	if menuActive {
		g.drawRuntimeMenu(overlay, w, h, g.Menu.M_Draw)
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
