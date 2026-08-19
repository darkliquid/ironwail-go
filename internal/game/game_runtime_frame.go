package game

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/quakeui"
	quakeconsole "github.com/darkliquid/ironwail-go/internal/quakeui/console"
	quakehud "github.com/darkliquid/ironwail-go/internal/quakeui/hud"
	quakemenu "github.com/darkliquid/ironwail-go/internal/quakeui/menu"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gogpu"
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

	// Browser (js/wasm): gogpu's App.Run() initializes the WebGPU device,
	// queue, and swapchain on the canvas. On WASM, App.Run() is launched in a
	// background goroutine so that gogpu initializes the GPU context while
	// StepWasmFrame and requestAnimationFrame drive host frames and redraws.
	// The engine starts paused so the world loads frozen; the walkthrough
	// Play/Step controls advance frames.
	if runtime.GOOS == "js" {
		g.wasmStartPaused()
		if g.Renderer != nil {
			ready := make(chan struct{})
			go func() {
				go func() {
					for i := 0; i < 100; i++ {
						if dp, ok := g.Renderer.(interface{ DeviceProvider() any }); ok && dp.DeviceProvider() != nil {
							close(ready)
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
					select {
					case <-ready:
					default:
						close(ready)
					}
				}()
				if err := g.Renderer.Run(); err != nil {
					slog.Warn("WASM gogpu renderer loop exited", "error", err)
				}
			}()
			<-ready
		}
		g.StartWasmRendererFrameLoop()
		select {}
	}
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
	var paritySettleCountdown = 15
	g.Renderer.OnUpdate(func(dt float64) {
		if os.Getenv("PARITY_RUN") == "1" && g.Client != nil && g.Client.State == cl.StateActive && g.Host.SignOns() == 4 {
			if !paritySetupDone {
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
			} else if paritySettleCountdown > 0 {
				paritySettleCountdown--
			}
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

		transientEvents := g.RunRuntimeFrameUnlessPaused(dt, cb)
		if g.Host != nil && g.Host.IsAborted() {
			if g.Renderer != nil {
				g.Renderer.Stop()
			}
			return
		}

		g.syncRuntimeVisualEffects(dt, transientEvents)
		state.storePendingRendererFrame(dt, transientEvents)
	})

	var lastDrawTime time.Time
	g.Renderer.OnDraw(func(dc renderer.RenderContext) {
		if !g.runtimeMu.TryLock() {
			return
		}
		defer g.runtimeMu.Unlock()

		now := time.Now()
		if !lastDrawTime.IsZero() && now.Sub(lastDrawTime) > 5*time.Second {
			slog.Warn("frame stall detected", "gap_seconds", now.Sub(lastDrawTime).Seconds())
		}
		lastDrawTime = now

		if state.screenshotMode && !state.screenshotCaptured {
			if os.Getenv("PARITY_RUN") != "1" || (paritySetupDone && paritySettleCountdown == 0) {
				defer g.captureRuntimeRendererScreenshot(state)
			}
		}

		g.applyRuntimeRendererState(state)
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
	g.uploadDeferredRuntimeWorld()
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

func parseParityAnglesEnv(raw string) (types.Vec3, bool) {
	fields := strings.Fields(raw)
	if len(fields) != 3 {
		return types.Vec3{}, false
	}
	var pitch, yaw, roll float32
	if _, err := fmt.Sscanf(strings.Join(fields, " "), "%f %f %f", &pitch, &yaw, &roll); err != nil {
		return types.Vec3{}, false
	}
	return types.Vec3{X: pitch, Y: yaw, Z: roll}, true
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

	g.Renderer.PreloadBrushEntities(brushEntities)

	if drawCtx, ok := dc.(*renderer.DrawContext); ok {
		state := g.buildRuntimeRenderFrameState(brushEntities, aliasEntities, spriteEntities, viewModel)
		drawCtx.RenderFrame(state, func(overlay renderer.RenderContext) {
			if quakeui.IsGogpuUIPath(g.Host.CVar) {
				// Path 1: gogpu/ui widget tree (spec §3.3). The host draws
				// every surface (HUD, console, menu) into a widget canvas and
				// composites it onto the engine surface.
				g.drawRuntimeOverlayFrameGogpuUI(overlay)
				return
			}
			g.drawRuntimeOverlayFrame(overlay)
		})
		return
	}

	g.drawRuntimeFallbackFrame(dc)
}

func (g *Game) drawRuntimeOverlayFrameGogpuUI(overlay renderer.RenderContext) {
	w, h := g.Renderer.Size()
	if setter, ok := overlay.(CanvasParamSetter); ok {
		setter.SetCanvasParams(g.runtimeOverlayCanvasParams(w, h))
	}

	// Path 1: run the gogpu/ui widget host (spec §5.1, ADR-0002). The host
	// draws the active surface(s) into its widget canvas and composites the
	// result onto the engine surface via the gogpu.ContextRenderTarget.
	if g.UIHost == nil {
		return
	}

	// Set the root per active surface (spec §3.2). The menu widget is set as
	// the host root when the menu is active; otherwise the host keeps an empty
	// root (HUD/console widgets land in M4/M5).
	g.syncUIHostRoot()

	// CSQC fallback (AC7, spec §1.2): when a mod's CSQC_DrawHud draws, the
	// HUD falls back to the legacy CSQC canvas path (drawn here, before the
	// widget composite) and the HUD widget is hidden by syncUIHostRoot.
	if g.CSQC != nil && g.CSQC.IsLoaded() {
		showScores := g.ShowScores && g.Client != nil && g.Client.MaxClients > 1
		g.drawRuntimeCSQCHUD(overlay, showScores)
	}

	g.UIHost.Frame()
	if dc, ok := overlay.(interface{ GogpuContext() *gogpu.Context }); ok {
		if ctx := dc.GogpuContext(); ctx != nil {
			if err := g.UIHost.DrawTo(ctx.RenderTarget()); err != nil {
				slog.Debug("quakeui DrawTo", "error", err)
			}
		}
	}
}

// syncUIHostRoot sets the gogpu/ui host root to the stacked surface tree
// (ADR-0002, G.14): HUD (bottom), menu, console (top). Surface visibility is
// toggled per frame so overlapping surfaces (console forced-up over menu at
// boot, menu over frozen world + HUD) are layered, not mutually exclusive.
func (g *Game) syncUIHostRoot() {
	if g.UIHost == nil {
		return
	}
	consoleActive := g.Input != nil && g.Input.KeyDest() == input.KeyConsole
	menuActive := g.Menu != nil && g.Menu.IsActive()
	inGame := g.Server != nil && g.Server.Active

	// Build the surface widgets lazily.
	if g.HUD != nil {
		if g.hudRoot == nil {
			g.hudRoot = quakehud.NewStatusBarWidget(g.HUD.State(), g.HUD.Style(), g.quakeUIText())
		} else {
			g.hudRoot.SetState(g.HUD.State())
		}
	}
	if g.menuRoot == nil {
		g.menuRoot = quakemenu.NewMenuRoot(g.Menu, g.quakeUIText())
		g.menuRoot.SetCVars(g.Host.CVar)
	}
	if g.consoleRoot == nil {
		g.consoleRoot = quakeconsole.NewConsoleWidget(console.Global(), g.quakeUIText())
	}

	// Build the stack once; toggle surface visibility per frame.
	if g.uiStack == nil {
		g.uiStack = quakeui.NewStack(g.hudRoot, g.menuRoot, g.consoleRoot)
		g.UIHost.SetRoot(g.uiStack)
	}

	// CSQC fallback (AC7, spec §1.2): when a mod's CSQC_DrawHud draws, the
	// HUD falls back to the legacy CSQC canvas path and the HUD widget is
	// hidden. Menu and console stay on the widget path.
	csqcHUD := g.csqcHUDWidgetHidden()
	if g.hudRoot != nil {
		g.hudRoot.SetVisible(inGame && g.HUD != nil && !csqcHUD)
	}
	g.menuRoot.SetVisible(menuActive)
	g.consoleRoot.SetVisible(consoleActive)
}

// csqcHUDWidgetHidden reports whether the path-1 HUD widget should be hidden
// because a CSQC mod owns the HUD (AC7, spec §1.2): when CSQC progs are
// loaded, the HUD falls back to the legacy CSQC canvas path.
func (g *Game) csqcHUDWidgetHidden() bool {
	return g.CSQC != nil && g.CSQC.IsLoaded()
}

// quakeUIText builds the QuakeText widget used by the path-1 UI, backed by
// the engine's conchars atlas and palette.
func (g *Game) quakeUIText() *widgets.QuakeText {
	var conchars []byte
	var palette []byte
	if g.Draw != nil {
		conchars = g.Draw.ConcharsData()
		palette = g.Draw.Palette()
	}
	return widgets.NewQuakeText(conchars, palette)
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
