package main

import (
	"bufio"
	"fmt"
	"image/png"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/host"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// gameCallbacks implements host.FrameCallbacks to drive server+client each frame.
type gameCallbacks struct{}

func (gameCallbacks) GetEvents() {
	if gameInput != nil {
		gameInput.PollEvents()
	}
	if gameSubs != nil && gameSubs.Client != nil && gameHost != nil {
		_ = gameSubs.Client.Frame(gameHost.FrameTime())
	}
}

func (gameCallbacks) ProcessConsoleCommands() {
	host.DispatchLoopbackStuffText(gameSubs)
}

func (gameCallbacks) ProcessServer() {
	if gameSubs == nil || gameSubs.Server == nil {
		return
	}
	dt := gameHost.FrameTime()
	if err := gameSubs.Server.Frame(dt); err != nil {
		slog.Warn("server frame error", "error", err)
	}
}

func (gameCallbacks) ProcessClient() {
	if gameSubs == nil || gameSubs.Client == nil {
		return
	}
	syncHostClientState()

	// Handle demo playback
	if gameHost != nil && gameHost.DemoState() != nil && gameHost.DemoState().Playback {
		demo := gameHost.DemoState()
		if !demo.ShouldReadFrame(gameHost.FrameCount()) {
			return
		}
		clientState := host.ActiveClientState(gameSubs)
		if clientState != nil {
			clientState.AdvanceTime(demo, gameHost.FrameTime())
			if !shouldReadNextDemoMessage(clientState, demo) {
				return
			}
		}

		// Try to read next demo frame
		msgData, viewAngles, err := demo.ReadDemoFrame()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "unexpected EOF" {
				if demo.TimeDemo && gameSubs != nil && gameSubs.Console != nil {
					frames, seconds, fps := demo.TimeDemoSummary()
					gameSubs.Console.Print(fmt.Sprintf("timedemo: %d frames %.3f seconds %.1f fps\n", frames, seconds, fps))
				}
				// Demo ended, check if we should loop to next demo
				_ = demo.StopPlayback()
				gameHost.SetClientState(0) // caDisconnected

				// Demo loop: play next demo if demo loop is active
				if gameHost.DemoNum() >= 0 && len(gameHost.DemoList()) > 0 {
					demoNum := gameHost.DemoNum()
					demos := gameHost.DemoList()

					// Wrap around to start
					if demoNum >= len(demos) {
						demoNum = 0
						gameHost.SetDemoNum(demoNum)
					}

					if demoNum < len(demos) && demos[demoNum] != "" {
						// Play the next demo
						gameHost.CmdPlaydemo(demos[demoNum], gameSubs)
						// Advance for next time
						gameHost.SetDemoNum(demoNum + 1)
					} else {
						// No more demos
						gameHost.SetDemoNum(-1)
					}
				}
				return
			}
			// Other errors - stop playback
			slog.Warn("demo playback error", "error", err)
			_ = demo.StopPlayback()
			gameHost.SetClientState(0) // caDisconnected
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
			}
			host.DispatchLoopbackStuffText(gameSubs)

		}

		// Don't run normal networked gameplay during demo playback
		return
	}

	// Normal networked gameplay
	_ = gameSubs.Client.ReadFromServer()
	syncHostClientState()
	recordRuntimeDemoFrame()
	host.DispatchLoopbackStuffText(gameSubs)
	_ = gameSubs.Client.SendCommand()
}

func (gameCallbacks) UpdateScreen() {}

func syncHostClientState() {
	if gameSubs == nil || gameSubs.Client == nil {
		return
	}
	prevClient := gameClient
	gameClient = host.ActiveClientState(gameSubs)
	if gameClient != prevClient {
		syncControlCvarsToClient()
	}
	if gameHost == nil {
		return
	}
	gameHost.SetClientState(gameSubs.Client.State())
	if gameClient != nil {
		gameHost.SetSignOns(gameClient.Signon)
	}
}

func syncAudioViewEntity() {
	if gameAudio == nil {
		return
	}

	viewEntity := 0
	if gameClient != nil {
		viewEntity = gameClient.ViewEntity
	}
	gameAudio.SetViewEntity(viewEntity)
}

func (gameCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {
	if gameAudio == nil {
		return
	}
	syncAudioViewEntity()
	gameAudio.SetListener(origin, [3]float32{}, forward, right, up)
}

func headlessGameLoop() {
	slog.Info("Starting headless game loop")

	// Simple game loop without rendering
	slog.Info("frame loop started")
	lastTime := time.Now()
	ticker := time.NewTicker(time.Second / 250) // 250 FPS target
	defer ticker.Stop()

	for range ticker.C {
		if gameHost != nil && gameHost.IsAborted() {
			return
		}
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Update game state
		if err := gameHost.Frame(dt, gameCallbacks{}); err != nil {
			log.Fatal("host frame error", err)
		}
		if gameHost != nil && gameHost.IsAborted() {
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
		if gameSubs != nil && gameSubs.Commands != nil {
			gameSubs.Commands.AddText(text)
			gameSubs.Commands.Execute()
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
				if gameHost != nil && gameHost.IsAborted() {
					return
				}
			default:
				goto frame
			}
		}

	frame:
		if gameHost != nil && gameHost.IsAborted() {
			return
		}
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		if err := gameHost.Frame(dt, gameCallbacks{}); err != nil {
			log.Fatal("host frame error", err)
		}
		if gameHost != nil && gameHost.IsAborted() {
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

	if plaque := pics.GetPic("gfx/qplaque.lmp"); plaque != nil {
		dc.DrawMenuPic(16, 4, plaque)
	}
	if loading := pics.GetPic("gfx/loading.lmp"); loading != nil {
		dc.DrawMenuPic((320-int(loading.Width))/2, (240-48-int(loading.Height))/2, loading)
	}
}

func runRuntimeFrame(dt float64, cb gameCallbacks) cl.TransientEvents {
	if gameHost != nil {
		gameHost.Frame(dt, cb)
	}
	syncControlCvarsToClient()
	if gameClient != nil {
		gameClient.PredictPlayers(float32(dt))
		gameClient.UpdateBlend(dt)
	}
	transientEvents := cl.TransientEvents{}
	if gameClient != nil {
		transientEvents = gameClient.ConsumeTransientEvents()
	}
	viewOrigin, viewAngles := runtimeViewState()
	syncRuntimeSkybox()
	if gameAudio != nil {
		forward, right, up := runtimeAngleVectors(viewAngles)
		syncAudioViewEntity()
		viewVelocity := [3]float32{}
		if gameClient != nil {
			viewVelocity = gameClient.GetPredictedVelocity()
		}
		gameAudio.SetListener(viewOrigin, viewVelocity, forward, right, up)
		syncRuntimeStaticSounds()
		syncRuntimeAmbientAudio(viewOrigin, float32(dt))
		syncRuntimeMusic()
		processRuntimeAudioEvents(viewOrigin, transientEvents)
		gameAudio.Update(viewOrigin, viewVelocity, forward, right, up)
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
	if gameRenderer != nil {
		if capturer, ok := any(gameRenderer).(interface {
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
	if gameDraw != nil {
		palette = gameDraw.Palette()
	}
	soft := renderer.NewSoftwareRenderer(ssWidth, ssHeight, 1.0, palette)

	// Sky-blue background
	soft.Clear(0.08, 0.08, 0.18, 1.0)

	// Render BSP world geometry if a map is loaded
	if gameServer != nil && gameServer.WorldTree != nil {
		soft.DrawBSPWorld(gameServer.WorldTree)
	}

	// Render 2D overlay (menu if active)
	if gameMenu != nil && gameMenu.IsActive() {
		gameMenu.M_Draw(soft)
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
