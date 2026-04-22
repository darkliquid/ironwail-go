package game

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// gameCallbacks implements host.FrameCallbacks to drive server+client each frame.
type gameCallbacks struct {
	g *Game
}

type runtimeLastServerMessageProvider interface {
	LastServerMessage() []byte
}

func defaultLoadDemoWorldTree(files host.Filesystem, worldModel string) (*bsp.Tree, error) {
	data, litData, err := loadWorldModelAndLit(files, worldModel)
	if err != nil {
		return nil, err
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := bsp.ApplyLitFile(tree, litData); err != nil {
		slog.Warn("ignoring invalid .lit sidecar", "map", worldModel, "error", err)
	}
	return tree, nil
}

type litWorldLoader interface {
	LoadMapBSPAndLit(worldModel string) ([]byte, []byte, error)
}

func loadWorldModelAndLit(files host.Filesystem, worldModel string) ([]byte, []byte, error) {
	if loader, ok := files.(litWorldLoader); ok {
		return loader.LoadMapBSPAndLit(worldModel)
	}

	data, err := files.LoadFile(worldModel)
	if err != nil {
		return nil, nil, err
	}

	return data, nil, nil
}

func (cb gameCallbacks) SetProcessClientPhase(phase string) {
	if cb.g == nil {
		return
	}
	cb.g.processClientPhase = phase
}

func (cb gameCallbacks) GetEvents() {
	g := cb.g
	if g == nil {
		return
	}
	g.pollRuntimeInputEvents()
	if g.Subs != nil && g.Subs.Client != nil && g.Host != nil {
		_ = g.Subs.Client.Frame(g.Host.FrameTime())
	}
}

func (cb gameCallbacks) ProcessConsoleCommands() {
	g := cb.g
	if g == nil {
		return
	}
	if g.Subs != nil && g.Subs.Commands != nil {
		g.Subs.Commands.Execute()
	}
	host.DispatchLoopbackStuffText(g.Subs)
}

func (cb gameCallbacks) ProcessServer() {
	g := cb.g
	if g == nil {
		return
	}
	if g.Subs == nil || g.Subs.Server == nil {
		return
	}
	dt := g.Host.FrameTime()
	if err := g.Subs.Server.Frame(dt); err != nil {
		slog.Warn("server frame error", "error", err)
	}
}

func (cb gameCallbacks) ProcessClient() {
	g := cb.g
	if g == nil {
		return
	}
	if g.Subs == nil || g.Subs.Client == nil {
		return
	}
	g.syncHostClientState()
	prevState, prevSignon := g.currentRuntimeClientActivation()

	// Handle demo playback
	if g.Host != nil && g.Host.DemoState() != nil && g.Host.DemoState().Playback {
		demo := g.Host.DemoState()
		g.refreshDemoPlaybackSpeed()
		if !demo.ShouldReadFrame(g.Host.FrameCount()) {
			return
		}
		clientState := host.ActiveClientState(g.Subs)
		prevState := cl.StateDisconnected
		prevSignon := 0
		if clientState != nil {
			prevState = clientState.State
			prevSignon = clientState.Signon
			// Use the simulation frame delta for demo time advancement.
			// Unlike FrameTime, SimFrameTime is not temporarily overwritten
			// to accumTime during Host.Frame's send/net-tick block, so
			// cl.time advances by the same amount C Ironwail's
			// host_frametime (as seen outside the send block) does. Using
			// raw wall-clock dt here would bypass the CLAMP(0.0001, 0.1)
			// and host_framerate/host_timescale overrides, breaking
			// deterministic demo playback used by the parity harness.
			// Mirrors C Quake's cl.time += cls.demospeed * host_frametime
			// in cl_demo.c::CL_GetMessage.
			clientState.AdvanceTime(demo, g.Host.SimFrameTime())
			if !g.shouldReadNextDemoMessage(clientState, demo) {
				return
			}
		}

		if demo.Speed < 0 && clientState != nil && clientState.Signon >= cl.Signons {
			if demo.FrameIndex <= 1 {
				demo.SetRewindBackstop(true)
				return
			}
			if err := g.Host.SeekDemoFrame(demo.FrameIndex-1, g.Subs); err != nil {
				slog.Warn("demo rewind error", "error", err)
				_ = demo.StopPlaybackWithSummary(func(msg string) {
					if g.Subs != nil && g.Subs.Console != nil {
						g.Subs.Console.Print(msg)
					}
				})
				g.clearRuntimeDemoFlags()
				g.Host.SetClientState(0) // caDisconnected
				return
			}
			g.bootstrapDemoPlaybackWorld(clientState)
			g.syncHostClientState()
			if clientState.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
				g.ApplyStartupGameplayInputMode()
			}
			return
		}

		// Try to read next demo frame
		msgData, viewAngles, err := demo.ReadDemoFrame()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				// Demo ended, check if we should loop to next demo
				_ = demo.StopPlaybackWithSummary(func(msg string) {
					if g.Subs != nil && g.Subs.Console != nil {
						g.Subs.Console.Print(msg)
					}
				})
				g.clearRuntimeDemoFlags()
				g.Host.SetClientState(0) // caDisconnected

				// Queue the next attract-mode demo for the next frame instead of
				// starting it inline during EOF teardown. This keeps playback
				// teardown/bootstrap from mutating render state mid-frame.
				if g.Host.DemoNum() >= 0 && len(g.Host.DemoList()) > 0 {
					if g.Subs != nil && g.Subs.Commands != nil {
						g.Subs.Commands.AddText("demos\n")
					}
				}
				return
			}
			// Other errors - stop playback
			slog.Warn("demo playback error", "error", err)
			_ = demo.StopPlaybackWithSummary(func(msg string) {
				if g.Subs != nil && g.Subs.Console != nil {
					g.Subs.Console.Print(msg)
				}
			})
			g.clearRuntimeDemoFlags()
			g.Host.SetClientState(0) // caDisconnected
			return
		}

		// Successfully read demo frame - parse the message and apply view angles
		// Get the actual client state to access parser
		if clientState != nil {
			g.applyDemoPlaybackViewAngles(clientState, viewAngles)

			// Parse the server message from demo
			parser := cl.NewParser(clientState)
			if err := parser.ParseServerMessage(msgData); err != nil {
				slog.Warn("failed to parse demo message", "error", err)
			} else {
				g.bootstrapDemoPlaybackWorld(clientState)
			}
			host.DispatchLoopbackStuffText(g.Subs)
			g.syncHostClientState()
			if clientState.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
				g.ApplyStartupGameplayInputMode()
			}

		}

		// Don't run normal networked gameplay during demo playback
		return
	}

	// Normal networked gameplay
	switch g.processClientPhase {
	case "send":
		_ = g.Subs.Client.SendCommand()
	case "read":
		_ = g.Subs.Client.ReadFromServer()
		g.noteRuntimeServerMessage()
		g.syncHostClientState()
		g.applyRuntimeGameplayActivation(prevState, prevSignon)
		g.recordRuntimeDemoFrame()
		host.DispatchLoopbackStuffText(g.Subs)
	default:
		_ = g.Subs.Client.ReadFromServer()
		g.noteRuntimeServerMessage()
		g.syncHostClientState()
		g.applyRuntimeGameplayActivation(prevState, prevSignon)
		g.recordRuntimeDemoFrame()
		host.DispatchLoopbackStuffText(g.Subs)
		_ = g.Subs.Client.SendCommand()
	}
}

func (g *Game) currentRuntimeClientActivation() (state cl.ClientState, signon int) {
	if g.Client == nil {
		return cl.StateDisconnected, 0
	}
	return g.Client.State, g.Client.Signon
}

func (g *Game) applyRuntimeGameplayActivation(prevState cl.ClientState, prevSignon int) {
	if g.Client == nil {
		return
	}
	if g.Client.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
		g.ApplyStartupGameplayInputMode()
	}
}

func (g *Game) noteRuntimeServerMessage() {
	if g.Subs == nil || g.Subs.Client == nil || g.Host == nil {
		return
	}
	provider, ok := g.Subs.Client.(runtimeLastServerMessageProvider)
	if !ok {
		return
	}
	if len(provider.LastServerMessage()) == 0 {
		return
	}
	g.LastServerMessageAt = g.Host.RealTime()
}

func (gameCallbacks) UpdateScreen() {}

func (g *Game) syncHostClientState() {
	if g.Subs == nil || g.Subs.Client == nil {
		return
	}
	prevClient := g.Client
	g.Client = host.ActiveClientState(g.Subs)
	if g.Client != prevClient {
		g.syncControlCvarsToClient()
	}
	if g.Host == nil {
		return
	}
	g.Host.SetClientState(g.Subs.Client.State())
	if g.Client != nil {
		g.Host.SetSignOns(g.Client.Signon)
		g.Client.LocalServerFast = g.Host.LocalServerFast()
	}
}

func (g *Game) clearRuntimeDemoFlags() {
	if clientState := host.LoopbackClientState(g.Subs); clientState != nil {
		clientState.DemoPlayback = false
		clientState.TimeDemoActive = false
	}
}

func (g *Game) bootstrapDemoPlaybackWorld(clientState *cl.Client) {
	if clientState == nil || g.Host == nil || g.Server == nil || g.Subs == nil || g.Subs.Files == nil {
		return
	}
	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback || len(clientState.ModelPrecache) == 0 {
		return
	}
	worldModel := clientState.ModelPrecache[0]
	if worldModel == "" || (g.Server.WorldTree != nil && g.Server.ModelName == worldModel) {
		return
	}
	tree, err := g.loadDemoWorldTree(g.Subs.Files, worldModel)
	if err != nil {
		slog.Debug("demo world load skipped", "model", worldModel, "error", err)
		return
	}
	g.Server.ModelName = worldModel
	g.Server.WorldTree = tree
	g.WorldUploadKey = ""
}

func (g *Game) syncAudioViewEntity() {
	if g.Audio == nil {
		return
	}

	viewEntity := 0
	if g.Client != nil {
		viewEntity = g.Client.ViewEntity
	}
	g.Audio.SetViewEntity(viewEntity)
}

func (cb gameCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {
	g := cb.g
	if g == nil {
		return
	}
	if g.Audio == nil {
		return
	}
	g.syncAudioViewEntity()
	g.Audio.SetListener(origin, [3]float32{}, forward, right, up)
}

func (g *Game) HeadlessGameLoop() {
	slog.Info("Starting headless game loop")

	// Simple game loop without rendering
	slog.Info("frame loop started")
	lastTime := time.Now()
	ticker := time.NewTicker(time.Second / 250) // 250 FPS target
	defer ticker.Stop()

	for range ticker.C {
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Update game state
		if err := g.Host.Frame(dt, gameCallbacks{g: g}); err != nil {
			log.Fatal("host frame error", err)
		}
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
	}
}

func (g *Game) DedicatedGameLoop() {
	slog.Info("Starting dedicated game loop")
	slog.Info("frame loop started")

	consoleCommands := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}
			consoleCommands <- text
		}
	}()

	lastTime := time.Now()

	queueConsoleCommand := func(text string) {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if g.Subs != nil && g.Subs.Commands != nil {
			g.Subs.Commands.AddText(text)
			g.Subs.Commands.Execute()
		}
	}

	for {
		ticrate := g.Host.CVar.FloatValue("sys_ticrate")
		if ticrate <= 0 {
			ticrate = 0.05
		}
		time.Sleep(time.Duration(ticrate * float64(time.Second)))
		for {
			select {
			case command := <-consoleCommands:
				queueConsoleCommand(command)
				if g.Host != nil && g.Host.IsAborted() {
					return
				}
			default:
				goto frame
			}
		}

	frame:
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		if err := g.Host.Frame(dt, gameCallbacks{g: g}); err != nil {
			log.Fatal("host frame error", err)
		}
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
	}
}

type picProvider interface {
	GetPic(name string) *qimage.QPic
}

func (g *Game) drawLoadingPlaque(dc renderer.RenderContext, pics picProvider) {
	if dc == nil || pics == nil {
		return
	}
	dc.SetCanvas(renderer.CanvasMenu)

	if plaque := pics.GetPic("gfx/qplaque.lmp"); plaque != nil {
		dc.DrawMenuPic(16, 4, plaque)
	}
	if loading := pics.GetPic("gfx/loading.lmp"); loading != nil {
		dc.DrawMenuPic((320-int(loading.Width))/2, (240-48-int(loading.Height))/2, loading)
	}
}

func (g *Game) RunRuntimeFrame(dt float64, cb gameCallbacks) cl.TransientEvents {
	if g.Host != nil {
		g.Host.Frame(dt, cb)
	}
	g.syncControlCvarsToClient()
	if g.Client != nil {
		if g.Host != nil && (g.Host.DemoState() == nil || !g.Host.DemoState().Playback) {
			g.Client.AdvanceTime(nil, dt)
		}
		g.runtimeDebugViewBeginFrame()
		g.runtimeDebugViewLogRelinkPhase("pre")
		g.Client.UpdateBlend(dt)
		g.Client.UpdateTempEntities()
		// Relink before view/audio consumers so camera, listener, and viewmodel
		// calculations all observe the same interpolated entity state this frame.
		g.Client.RelinkEntities()
		g.runtimeDebugViewLogRelinkPhase("post")
		// Predict after relink so prediction freshness is stamped against the
		// final post-LerpPoint frame state consumed by camera selection.
		g.Client.PredictPlayers(float32(dt))
		g.runtimeDebugViewLogPrediction()
	}
	transientEvents := cl.TransientEvents{}
	if g.Client != nil {
		transientEvents = g.Client.ConsumeTransientEvents()
	}
	viewOrigin, viewAngles := g.runtimeViewState()
	g.runtimeDebugViewLogState(viewOrigin, viewAngles)
	g.runtimeDebugViewLogLerp()
	g.runtimeDebugViewLogOriginSelect()
	g.syncRuntimeSkybox()
	if g.Audio != nil {
		forward, right, up := g.runtimeAngleVectors(viewAngles)
		g.syncAudioViewEntity()
		viewVelocity := [3]float32{}
		if g.Client != nil {
			viewVelocity = g.Client.GetPredictedVelocity()
		}
		g.Audio.SetListener(viewOrigin, viewVelocity, forward, right, up)
		g.syncRuntimeStaticSounds()
		g.syncRuntimeAmbientAudio(viewOrigin, float32(dt))
		g.syncRuntimeMusic()
		g.processRuntimeAudioEvents(viewOrigin, transientEvents)
		g.Audio.Update(viewOrigin, viewVelocity, forward, right, up)
	}
	return transientEvents
}

func (g *Game) isRendererError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "renderer") ||
		strings.Contains(errStr, "wayland") ||
		strings.Contains(errStr, "configure") ||
		strings.Contains(errStr, "display") ||
		strings.Contains(errStr, "window") ||
		strings.Contains(errStr, "surface") ||
		strings.Contains(errStr, "segv")
}

func (g *Game) CaptureScreenshot(sspath, _, _ string) error {
	if g.Renderer != nil {
		if capturer, ok := any(g.Renderer).(interface {
			CaptureScreenshot(string) error
		}); ok {
			if err := capturer.CaptureScreenshot(sspath); err != nil {
				slog.Warn("renderer screenshot capture failed, falling back to software path", "error", err)
			} else {
				slog.Info("Screenshot saved", "path", sspath)
				return nil
			}
		}
	}

	ssWidth, ssHeight := 1280, 720
	if g.Renderer != nil {
		if w, h := g.Renderer.Size(); w > 0 && h > 0 {
			ssWidth = w
			ssHeight = h
		}
	}

	var palette []byte
	if g.Draw != nil {
		palette = g.Draw.Palette()
	}
	soft := renderer.NewSoftwareRenderer(ssWidth, ssHeight, 1.0, palette)

	// Sky-blue background
	soft.Clear(0.08, 0.08, 0.18, 1.0)

	// Render BSP world geometry if a map is loaded
	if g.Server != nil && g.Server.WorldTree != nil {
		soft.DrawBSPWorld(g.Server.WorldTree)
	}

	// Render 2D overlay (menu if active)
	if g.Menu != nil && g.Menu.IsActive() {
		g.Menu.M_Draw(soft)
	}

	f, err := os.Create(sspath)
	if err != nil {
		return fmt.Errorf("create screenshot file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, soft.Image()); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	slog.Info("Screenshot saved", "path", sspath)
	return nil
}
