package main

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

	"github.com/ironwail/ironwail-go/internal/bsp"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// gameCallbacks implements host.FrameCallbacks to drive server+client each frame.
type gameCallbacks struct{}

var runtimeProcessClientPhase string

type runtimeLastServerMessageProvider interface {
	LastServerMessage() []byte
}

var loadDemoWorldTree = func(files host.Filesystem, worldModel string) (*bsp.Tree, error) {
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
	if fsys, ok := files.(*fs.FileSystem); ok {
		return fsys.LoadMapBSPAndLit(worldModel)
	}
	data, err := files.LoadFile(worldModel)
	if err != nil {
		return nil, nil, err
	}
	return data, nil, nil
}

func (gameCallbacks) SetProcessClientPhase(phase string) {
	runtimeProcessClientPhase = phase
}

func (gameCallbacks) GetEvents() {
	pollRuntimeInputEvents()
	if g.Subs != nil && g.Subs.Client != nil && g.Host != nil {
		_ = g.Subs.Client.Frame(g.Host.FrameTime())
	}
}

func (gameCallbacks) ProcessConsoleCommands() {
	if g.Subs != nil && g.Subs.Commands != nil {
		g.Subs.Commands.Execute()
	} else {
		cmdsys.Execute()
	}
	host.DispatchLoopbackStuffText(g.Subs)
}

func (gameCallbacks) ProcessServer() {
	if g.Subs == nil || g.Subs.Server == nil {
		return
	}
	dt := g.Host.FrameTime()
	if err := g.Subs.Server.Frame(dt); err != nil {
		slog.Warn("server frame error", "error", err)
	}
}

func (gameCallbacks) ProcessClient() {
	if g.Subs == nil || g.Subs.Client == nil {
		return
	}
	syncHostClientState()
	prevState, prevSignon := currentRuntimeClientActivation()

	// Handle demo playback
	if g.Host != nil && g.Host.DemoState() != nil && g.Host.DemoState().Playback {
		demo := g.Host.DemoState()
		refreshDemoPlaybackSpeed()
		if !demo.ShouldReadFrame(g.Host.FrameCount()) {
			return
		}
		clientState := host.ActiveClientState(g.Subs)
		prevState := cl.StateDisconnected
		prevSignon := 0
		if clientState != nil {
			prevState = clientState.State
			prevSignon = clientState.Signon
			clientState.AdvanceTime(demo, g.Host.FrameTime())
			if !shouldReadNextDemoMessage(clientState, demo) {
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
				_ = demo.StopPlayback()
				clearRuntimeDemoFlags()
				g.Host.SetClientState(0) // caDisconnected
				return
			}
			bootstrapDemoPlaybackWorld(clientState)
			syncHostClientState()
			if clientState.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
				applyStartupGameplayInputMode()
			}
			return
		}

		// Try to read next demo frame
		msgData, viewAngles, err := demo.ReadDemoFrame()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				if demo.TimeDemo && g.Subs != nil && g.Subs.Console != nil {
					frames, seconds, fps := demo.TimeDemoSummary()
					g.Subs.Console.Print(fmt.Sprintf("timedemo: %d frames %.3f seconds %.1f fps\n", frames, seconds, fps))
				}
				// Demo ended, check if we should loop to next demo
				_ = demo.StopPlayback()
				clearRuntimeDemoFlags()
				g.Host.SetClientState(0) // caDisconnected

				// Queue the next attract-mode demo for the next frame instead of
				// starting it inline during EOF teardown. This keeps playback
				// teardown/bootstrap from mutating render state mid-frame.
				if g.Host.DemoNum() >= 0 && len(g.Host.DemoList()) > 0 {
					if g.Subs != nil && g.Subs.Commands != nil {
						g.Subs.Commands.AddText("demos\n")
					} else {
						cmdsys.AddText("demos\n")
					}
				}
				return
			}
			// Other errors - stop playback
			slog.Warn("demo playback error", "error", err)
			_ = demo.StopPlayback()
			clearRuntimeDemoFlags()
			g.Host.SetClientState(0) // caDisconnected
			return
		}

		// Successfully read demo frame - parse the message and apply view angles
		// Get the actual client state to access parser
		if clientState != nil {
			applyDemoPlaybackViewAngles(clientState, viewAngles)

			// Parse the server message from demo
			parser := cl.NewParser(clientState)
			if err := parser.ParseServerMessage(msgData); err != nil {
				slog.Warn("failed to parse demo message", "error", err)
			} else {
				bootstrapDemoPlaybackWorld(clientState)
			}
			host.DispatchLoopbackStuffText(g.Subs)
			syncHostClientState()
			if clientState.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
				applyStartupGameplayInputMode()
			}

		}

		// Don't run normal networked gameplay during demo playback
		return
	}

	// Normal networked gameplay
	switch runtimeProcessClientPhase {
	case "send":
		_ = g.Subs.Client.SendCommand()
	case "read":
		_ = g.Subs.Client.ReadFromServer()
		noteRuntimeServerMessage()
		syncHostClientState()
		applyRuntimeGameplayActivation(prevState, prevSignon)
		recordRuntimeDemoFrame()
		host.DispatchLoopbackStuffText(g.Subs)
	default:
		_ = g.Subs.Client.ReadFromServer()
		noteRuntimeServerMessage()
		syncHostClientState()
		applyRuntimeGameplayActivation(prevState, prevSignon)
		recordRuntimeDemoFrame()
		host.DispatchLoopbackStuffText(g.Subs)
		_ = g.Subs.Client.SendCommand()
	}
}

func currentRuntimeClientActivation() (state cl.ClientState, signon int) {
	if g.Client == nil {
		return cl.StateDisconnected, 0
	}
	return g.Client.State, g.Client.Signon
}

func applyRuntimeGameplayActivation(prevState cl.ClientState, prevSignon int) {
	if g.Client == nil {
		return
	}
	if g.Client.State == cl.StateActive && (prevState != cl.StateActive || prevSignon < cl.Signons) {
		applyStartupGameplayInputMode()
	}
}

func noteRuntimeServerMessage() {
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

func syncHostClientState() {
	if g.Subs == nil || g.Subs.Client == nil {
		return
	}
	prevClient := g.Client
	g.Client = host.ActiveClientState(g.Subs)
	if g.Client != prevClient {
		syncControlCvarsToClient()
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

func clearRuntimeDemoFlags() {
	if clientState := host.LoopbackClientState(g.Subs); clientState != nil {
		clientState.DemoPlayback = false
		clientState.TimeDemoActive = false
	}
}

func bootstrapDemoPlaybackWorld(clientState *cl.Client) {
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
	tree, err := loadDemoWorldTree(g.Subs.Files, worldModel)
	if err != nil {
		slog.Debug("demo world load skipped", "model", worldModel, "error", err)
		return
	}
	g.Server.ModelName = worldModel
	g.Server.WorldTree = tree
	g.WorldUploadKey = ""
}

func syncAudioViewEntity() {
	if g.Audio == nil {
		return
	}

	viewEntity := 0
	if g.Client != nil {
		viewEntity = g.Client.ViewEntity
	}
	g.Audio.SetViewEntity(viewEntity)
}

func (gameCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {
	if g.Audio == nil {
		return
	}
	syncAudioViewEntity()
	g.Audio.SetListener(origin, [3]float32{}, forward, right, up)
}

func headlessGameLoop() {
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
		if err := g.Host.Frame(dt, gameCallbacks{}); err != nil {
			log.Fatal("host frame error", err)
		}
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
	}
}

func dedicatedGameLoop() {
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
	ticker := time.NewTicker(time.Second / 250) // 250 FPS target
	defer ticker.Stop()

	queueConsoleCommand := func(text string) {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if g.Subs != nil && g.Subs.Commands != nil {
			g.Subs.Commands.AddText(text)
			g.Subs.Commands.Execute()
			return
		}
		cmdsys.AddText(text)
		cmdsys.Execute()
	}

	for range ticker.C {
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

		if err := g.Host.Frame(dt, gameCallbacks{}); err != nil {
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

func drawLoadingPlaque(dc renderer.RenderContext, pics picProvider) {
	if pics == nil {
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

func runRuntimeFrame(dt float64, cb gameCallbacks) cl.TransientEvents {
	if g.Host != nil {
		g.Host.Frame(dt, cb)
	}
	syncControlCvarsToClient()
	if g.Client != nil {
		if g.Host != nil && (g.Host.DemoState() == nil || !g.Host.DemoState().Playback) {
			g.Client.AdvanceTime(nil, dt)
		}
		runtimeDebugViewBeginFrame()
		runtimeDebugViewLogRelinkPhase("pre")
		g.Client.UpdateBlend(dt)
		g.Client.UpdateTempEntities()
		// Relink before view/audio consumers so camera, listener, and viewmodel
		// calculations all observe the same interpolated entity state this frame.
		g.Client.RelinkEntities()
		runtimeDebugViewLogRelinkPhase("post")
		// Predict after relink so prediction freshness is stamped against the
		// final post-LerpPoint frame state consumed by camera selection.
		g.Client.PredictPlayers(float32(dt))
		runtimeDebugViewLogPrediction()
	}
	transientEvents := cl.TransientEvents{}
	if g.Client != nil {
		transientEvents = g.Client.ConsumeTransientEvents()
	}
	viewOrigin, viewAngles := runtimeViewState()
	runtimeDebugViewLogState(viewOrigin, viewAngles)
	runtimeDebugViewLogLerp()
	runtimeDebugViewLogOriginSelect()
	syncRuntimeSkybox()
	if g.Audio != nil {
		forward, right, up := runtimeAngleVectors(viewAngles)
		syncAudioViewEntity()
		viewVelocity := [3]float32{}
		if g.Client != nil {
			viewVelocity = g.Client.GetPredictedVelocity()
		}
		g.Audio.SetListener(viewOrigin, viewVelocity, forward, right, up)
		syncRuntimeStaticSounds()
		syncRuntimeAmbientAudio(viewOrigin, float32(dt))
		syncRuntimeMusic()
		processRuntimeAudioEvents(viewOrigin, transientEvents)
		g.Audio.Update(viewOrigin, viewVelocity, forward, right, up)
	}
	return transientEvents
}

func isRendererError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "renderer") ||
		strings.Contains(errStr, "wayland") ||
		strings.Contains(errStr, "configure") ||
		strings.Contains(errStr, "display") ||
		strings.Contains(errStr, "window") ||
		strings.Contains(errStr, "surface") ||
		strings.Contains(errStr, "segv")
}

func captureScreenshot(sspath, _, _ string) error {
	if g.Renderer != nil {
		if capturer, ok := any(g.Renderer).(interface {
			CaptureScreenshot(string) error
		}); ok {
			if err := capturer.CaptureScreenshot(sspath); err != nil {
				return fmt.Errorf("capture renderer screenshot: %w", err)
			}
			slog.Info("Screenshot saved", "path", sspath)
			return nil
		}
	}

	const (
		ssWidth  = 1280
		ssHeight = 720
	)

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
