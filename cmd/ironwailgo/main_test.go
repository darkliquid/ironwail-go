package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/bsp"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
)

type demoMessageClient struct {
	message []byte
}

func (c *demoMessageClient) Init() error                { return nil }
func (c *demoMessageClient) Frame(float64) error        { return nil }
func (c *demoMessageClient) Shutdown()                  {}
func (c *demoMessageClient) State() host.ClientState    { return 0 }
func (c *demoMessageClient) ReadFromServer() error      { return nil }
func (c *demoMessageClient) SendCommand() error         { return nil }
func (c *demoMessageClient) SendStringCmd(string) error { return nil }
func (c *demoMessageClient) LastServerMessage() []byte  { return append([]byte(nil), c.message...) }

type activeStateTestClient struct {
	state       host.ClientState
	clientState *cl.Client
}

func (c *activeStateTestClient) Init() error                { return nil }
func (c *activeStateTestClient) Frame(float64) error        { return nil }
func (c *activeStateTestClient) Shutdown()                  {}
func (c *activeStateTestClient) State() host.ClientState    { return c.state }
func (c *activeStateTestClient) ReadFromServer() error      { return nil }
func (c *activeStateTestClient) SendCommand() error         { return nil }
func (c *activeStateTestClient) SendStringCmd(string) error { return nil }
func (c *activeStateTestClient) ClientState() *cl.Client    { return c.clientState }

type demoPlaybackNoopServer struct{}

func (s *demoPlaybackNoopServer) Init(int) error                           { return nil }
func (s *demoPlaybackNoopServer) SpawnServer(string, *fs.FileSystem) error { return nil }
func (s *demoPlaybackNoopServer) ConnectClient(int)                        {}
func (s *demoPlaybackNoopServer) KillClient(int) bool                      { return false }
func (s *demoPlaybackNoopServer) KickClient(int, string, string) bool      { return false }
func (s *demoPlaybackNoopServer) Frame(float64) error                      { return nil }
func (s *demoPlaybackNoopServer) Shutdown()                                {}
func (s *demoPlaybackNoopServer) SaveSpawnParms()                          {}
func (s *demoPlaybackNoopServer) GetMaxClients() int                       { return 1 }
func (s *demoPlaybackNoopServer) IsClientActive(int) bool                  { return false }
func (s *demoPlaybackNoopServer) GetClientName(int) string                 { return "" }
func (s *demoPlaybackNoopServer) SetClientName(int, string)                {}
func (s *demoPlaybackNoopServer) GetClientColor(int) int                   { return 0 }
func (s *demoPlaybackNoopServer) SetClientColor(int, int)                  {}
func (s *demoPlaybackNoopServer) GetClientPing(int) float32                { return 0 }
func (s *demoPlaybackNoopServer) EdictNum(int) *server.Edict               { return nil }
func (s *demoPlaybackNoopServer) GetMapName() string                       { return "" }
func (s *demoPlaybackNoopServer) IsActive() bool                           { return false }
func (s *demoPlaybackNoopServer) IsPaused() bool                           { return false }

type demoPlaybackConsole struct{}

func (c *demoPlaybackConsole) Init() error       { return nil }
func (c *demoPlaybackConsole) Print(string)      {}
func (c *demoPlaybackConsole) Clear()            {}
func (c *demoPlaybackConsole) Dump(string) error { return nil }
func (c *demoPlaybackConsole) Shutdown()         {}

type demoPlaybackCommandBuffer struct {
	added    []string
	executes int
}

func (c *demoPlaybackCommandBuffer) Init()               {}
func (c *demoPlaybackCommandBuffer) Execute()            { c.executes++ }
func (c *demoPlaybackCommandBuffer) AddText(text string) { c.added = append(c.added, text) }
func (c *demoPlaybackCommandBuffer) InsertText(string)   {}
func (c *demoPlaybackCommandBuffer) Shutdown()           {}

type loadingPlaqueTestPics struct {
	pics map[string]*qimage.QPic
}

func (p *loadingPlaqueTestPics) GetPic(name string) *qimage.QPic {
	return p.pics[name]
}

type loadingPlaqueDrawCall struct {
	x   int
	y   int
	pic *qimage.QPic
}

type loadingPlaqueDrawContext struct {
	pics     []loadingPlaqueDrawCall
	menuPics []loadingPlaqueDrawCall
	canvas   renderer.CanvasState
}

func (dc *loadingPlaqueDrawContext) Clear(r, g, b, a float32)            {}
func (dc *loadingPlaqueDrawContext) DrawTriangle(r, g, b, a float32)     {}
func (dc *loadingPlaqueDrawContext) SurfaceView() interface{}            { return nil }
func (dc *loadingPlaqueDrawContext) Gamma() float32                      { return 1 }
func (dc *loadingPlaqueDrawContext) DrawFill(x, y, w, h int, color byte) {}
func (dc *loadingPlaqueDrawContext) DrawCharacter(x, y int, num int)     {}
func (dc *loadingPlaqueDrawContext) DrawMenuCharacter(x, y int, num int) {}
func (dc *loadingPlaqueDrawContext) DrawPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *loadingPlaqueDrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
	dc.menuPics = append(dc.menuPics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *loadingPlaqueDrawContext) SetCanvas(ct renderer.CanvasType) {
	dc.canvas.Type = ct
}
func (dc *loadingPlaqueDrawContext) Canvas() renderer.CanvasState { return dc.canvas }

type mouseDeltaBackend struct {
	dx int32
	dy int32
}

func (b *mouseDeltaBackend) Init() error                            { return nil }
func (b *mouseDeltaBackend) Shutdown()                              {}
func (b *mouseDeltaBackend) PollEvents() bool                       { return true }
func (b *mouseDeltaBackend) GetMouseDelta() (dx, dy int32)          { return b.dx, b.dy }
func (b *mouseDeltaBackend) GetModifierState() input.ModifierState  { return input.ModifierState{} }
func (b *mouseDeltaBackend) SetTextMode(input.TextMode)             {}
func (b *mouseDeltaBackend) SetCursorMode(input.CursorMode)         {}
func (b *mouseDeltaBackend) ShowKeyboard(bool)                      {}
func (b *mouseDeltaBackend) GetGamepadState(int) input.GamepadState { return input.GamepadState{} }
func (b *mouseDeltaBackend) IsGamepadConnected(int) bool            { return false }
func (b *mouseDeltaBackend) SetMouseGrab(bool)                      {}
func (b *mouseDeltaBackend) SetWindow(interface{})                  {}

func TestStartupMapArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "plus map", args: []string{"+map", "start"}, want: "start"},
		{name: "positional map", args: []string{"start"}, want: "start"},
		{name: "plus map wins", args: []string{"start", "+map", "e1m1"}, want: "e1m1"},
		{name: "no map", args: []string{"+skill", "2"}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := startupMapArg(tc.args); got != tc.want {
				t.Fatalf("startupMapArg(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestRegisterConsoleCompletionProvidersIncludesAliases(t *testing.T) {
	cmdsys.UnaliasAll()
	t.Cleanup(cmdsys.UnaliasAll)
	console.ResetCompletion()
	t.Cleanup(console.ResetCompletion)

	cmdsys.AddAlias("zz_alias_test", "echo hi\n")
	registerConsoleCompletionProviders()

	got, matches := console.CompleteInput("zz_al", true)
	if got != "zz_alias_test" {
		t.Fatalf("CompleteInput = %q, want %q", got, "zz_alias_test")
	}
	found := false
	for _, match := range matches {
		if match == "zz_alias_test (alias)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("matches = %v, want zz_alias_test (alias)", matches)
	}
}

func TestDrawLoadingPlaqueDrawsPlaqueAndCenteredLoadingPic(t *testing.T) {
	plaque := &qimage.QPic{Width: 320, Height: 20}
	loading := &qimage.QPic{Width: 160, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/qplaque.lmp": plaque,
			"gfx/loading.lmp": loading,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	drawLoadingPlaque(dc, pics)

	if len(dc.pics) != 0 {
		t.Fatalf("screen-space draw call count = %d, want 0", len(dc.pics))
	}
	if len(dc.menuPics) != 2 {
		t.Fatalf("menu draw call count = %d, want 2", len(dc.menuPics))
	}
	if dc.menuPics[0].x != 16 || dc.menuPics[0].y != 4 || dc.menuPics[0].pic != plaque {
		t.Fatalf("plaque draw = %+v, want x=16 y=4 plaque", dc.menuPics[0])
	}
	if dc.menuPics[1].x != 80 || dc.menuPics[1].y != 84 || dc.menuPics[1].pic != loading {
		t.Fatalf("loading draw = %+v, want centered loading pic", dc.menuPics[1])
	}
}

func TestDrawLoadingPlaqueNoopWithoutPics(t *testing.T) {
	dc := &loadingPlaqueDrawContext{}
	drawLoadingPlaque(dc, nil)
	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0", len(dc.pics), len(dc.menuPics))
	}
}

func TestRunRuntimeFrameRunsClientPrediction(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.Entities[0] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	runRuntimeFrame(0.016, gameCallbacks{})

	if got := g.Client.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
}

func TestRunRuntimeFrameSyncsAudioViewEntity(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalAudio := g.Audio
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Audio = originalAudio
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	g.Host = nil
	g.Audio = audio.NewAudioAdapter(sys)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 3
	g.Client.ViewHeight = 22
	g.Client.Entities[3] = inet.EntityState{Origin: [3]float32{64, 32, 16}}

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 3 {
		t.Fatalf("audio view entity after active client frame = %d, want 3", got)
	}

	g.Client = nil
	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 0 {
		t.Fatalf("audio view entity after clearing client = %d, want 0", got)
	}
}

func TestRunRuntimeFrameUpdatesLeafAmbientAndUnderwaterAudio(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalAudio := g.Audio
	originalServer := g.Server
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Audio = originalAudio
		g.Server = originalServer
		g.Subs = originalSubs
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	g.Audio = audio.NewAudioAdapter(sys)
	g.Audio.SetAmbientSound(0, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})
	g.Audio.SetAmbientSound(1, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})

	g.Host = nil
	g.Subs = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 0
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{64, 0, 0}}
	g.Server = &server.Server{
		WorldTree: &bsp.Tree{
			Planes: []bsp.DPlane{
				{Normal: [3]float32{1, 0, 0}, Dist: 0},
			},
			Nodes: []bsp.TreeNode{
				{
					PlaneNum: 0,
					Children: [2]bsp.TreeChild{
						{IsLeaf: true, Index: 1},
						{IsLeaf: true, Index: 2},
					},
				},
			},
			Leafs: []bsp.TreeLeaf{
				{Contents: bsp.ContentsSolid},
				{Contents: bsp.ContentsWater, AmbientLevel: [bsp.NumAmbients]uint8{80, 80, 0, 0}},
				{Contents: bsp.ContentsEmpty, AmbientLevel: [bsp.NumAmbients]uint8{0, 0, 0, 0}},
			},
		},
	}

	runRuntimeFrame(0.1, gameCallbacks{})
	if got := sys.UnderwaterIntensity(); got <= 0 {
		t.Fatalf("underwater intensity in water leaf = %v, want > 0", got)
	}
	if got := sys.ViewEntity(); got != 1 {
		t.Fatalf("audio view entity after leaf update = %d, want 1", got)
	}
	if got := sys.AmbientVolume(0); got != 10 {
		t.Fatalf("ambient channel 0 volume = %d, want 10", got)
	}
	if got := sys.AmbientVolume(1); got != 10 {
		t.Fatalf("ambient channel 1 volume = %d, want 10", got)
	}

	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{-64, 0, 0}}
	runRuntimeFrame(0.1, gameCallbacks{})
	if got := sys.UnderwaterIntensity(); got != 0 {
		t.Fatalf("underwater intensity in dry leaf = %v, want 0", got)
	}
	if got := sys.AmbientVolume(0); got != 0 {
		t.Fatalf("ambient channel 0 volume in dry leaf = %d, want 0", got)
	}
	if got := sys.AmbientVolume(1); got != 0 {
		t.Fatalf("ambient channel 1 volume in dry leaf = %d, want 0", got)
	}

	g.Server = nil
	runRuntimeFrame(0.1, gameCallbacks{})
	if sys.AmbientSound(0) != nil || sys.AmbientSound(1) != nil {
		t.Fatalf("ambient channels should clear when no world tree is available")
	}
}

func TestRunRuntimeFrameConsumesTransientEventsOnce(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.SoundEvents = []cl.SoundEvent{{Entity: 1, Channel: 2, SoundIndex: 3}}
	g.Client.StopSoundEvents = []cl.StopSoundEvent{{Entity: 4, Channel: 5}}
	g.Client.ParticleEvents = []cl.ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	g.Client.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}}}

	events := runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 1 || len(events.StopSoundEvents) != 1 || len(events.ParticleEvents) != 1 || len(events.TempEntities) != 1 {
		t.Fatalf("runRuntimeFrame consumed = %d sounds, %d stops, %d particles, %d temps; want 1,1,1,1", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
	if len(g.Client.SoundEvents) != 0 || len(g.Client.StopSoundEvents) != 0 || len(g.Client.ParticleEvents) != 0 || len(g.Client.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d sounds %d stops %d particles %d temps", len(g.Client.SoundEvents), len(g.Client.StopSoundEvents), len(g.Client.ParticleEvents), len(g.Client.TempEntities))
	}

	events = runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 0 || len(events.StopSoundEvents) != 0 || len(events.ParticleEvents) != 0 || len(events.TempEntities) != 0 {
		t.Fatalf("second frame consumed = %d sounds, %d stops, %d particles, %d temps; want 0,0,0,0", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
}

func TestRuntimeViewStatePrefersAuthoritativeViewEntityOrigin(t *testing.T) {
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{128, 64, 32}}
	g.Client.PredictedOrigin = [3]float32{64, 32, 16}
	g.Client.ViewHeight = 30
	g.Client.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if want := [3]float32{128, 64, 62}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateFallsBackToPredictedOrigin(t *testing.T) {
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.PredictedOrigin = [3]float32{128, 64, 32}
	g.Client.ViewHeight = 18
	g.Client.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if want := [3]float32{128, 64, 50}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesPredictedXYOffsetDuringActiveMovement(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Host = nil
	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := g.Client.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
	if got := g.Client.PredictedOrigin; got[2] >= 300 {
		t.Fatalf("expected collisionless prediction to drift below authoritative Z, got %#v", got)
	}

	origin, _ := runtimeViewState()
	if want := [3]float32{g.Client.PredictedOrigin[0], g.Client.PredictedOrigin[1], 300 + g.Client.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want predicted XY with authoritative Z %v", origin, want)
	}
}

func TestRuntimeViewStateClampsPredictedXYOffset(t *testing.T) {
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{120, 240, 280}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}

	offsetScale := float32(runtimeMaxPredictedXYOffset / math.Hypot(20, 40))
	want := [3]float32{
		100 + 20*offsetScale,
		200 + 40*offsetScale,
		300 + g.Client.ViewHeight,
	}

	origin, _ := runtimeViewState()
	if origin != want {
		t.Fatalf("runtimeViewState origin = %v, want clamped predicted XY %v", origin, want)
	}
}

func TestRuntimeViewStateIgnoresPredictedXYOffsetOnLargePredictionError(t *testing.T) {
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{110, 200, 280}
	g.Client.PredictionError = [3]float32{runtimeMaxPredictedXYOffset + 1, 0, 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}

	origin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 300 + g.Client.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeCameraStateCarriesClientTime(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Time = 12.5

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{4, 5, 6})
	if camera.Time != 12.5 {
		t.Fatalf("runtimeCameraState time = %v, want 12.5", camera.Time)
	}
}

func TestRuntimeCameraStateAppliesPunchAnglesOutsideIntermission(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 11 || camera.Angles.Y != 18 || camera.Angles.Z != 33 {
		t.Fatalf("runtimeCameraState angles = %v, want {11 18 33}", camera.Angles)
	}
}

func TestRuntimeCameraStateSkipsPunchAnglesDuringIntermission(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 10 || camera.Angles.Y != 20 || camera.Angles.Z != 30 {
		t.Fatalf("runtimeCameraState angles = %v, want {10 20 30}", camera.Angles)
	}
}

func TestRuntimeViewStateInterpolatesViewAngles(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.MViewAngles[1] = [3]float32{0, 0, 0}
	g.Client.MViewAngles[0] = [3]float32{10, 20, 30}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05

	_, angles := runtimeViewState()
	if angles != [3]float32{5, 10, 15} {
		t.Fatalf("runtimeViewState angles = %v, want [5 10 15]", angles)
	}
}

func TestRuntimeViewStateUsesForcedAnglesWithoutInterpolation(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.ViewAngles = [3]float32{45, 135, 225}
	g.Client.MViewAngles[1] = [3]float32{0, 0, 0}
	g.Client.MViewAngles[0] = [3]float32{10, 20, 30}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	g.Client.FixAngle = true

	_, angles := runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want forced angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeCameraStateInterpolatesPunchAngles(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngles[1] = [3]float32{0, 0, 0}
	g.Client.PunchAngles[0] = [3]float32{10, 0, 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X < 5.9 || camera.Angles.X > 6.1 {
		t.Fatalf("runtimeCameraState punch interpolation = %v, want ~6", camera.Angles.X)
	}
}

func TestRuntimeCameraStateGunKickModeRaw(t *testing.T) {
	originalClient := g.Client
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "1")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{2, -4, 6}
	g.Client.PunchAngles[1] = [3]float32{0, 0, 0}
	g.Client.PunchAngles[0] = [3]float32{10, 0, 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 3 || camera.Angles.Y != -2 || camera.Angles.Z != 9 {
		t.Fatalf("runtimeCameraState raw punch = %v, want {3 -2 9}", camera.Angles)
	}
}

func TestRuntimeCameraStateGunKickModeOff(t *testing.T) {
	originalClient := g.Client
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "0")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{2, -4, 6}

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 1 || camera.Angles.Y != 2 || camera.Angles.Z != 3 {
		t.Fatalf("runtimeCameraState with gunkick off = %v, want {1 2 3}", camera.Angles)
	}
}

func TestRuntimeCameraStateDeadPlayerRoll(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 0 // Dead player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{10, 10, 10}

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	// Dead players should have roll = 80 and ignore other view effects.
	if camera.Angles.Z != 80 {
		t.Fatalf("runtimeCameraState dead player roll = %v, want 80", camera.Angles.Z)
	}
}

func TestRuntimeCameraStateAppliesChaseCameraWhenActive(t *testing.T) {
	originalClient := g.Client
	if cvar.Get("chase_active") == nil {
		cvar.Register("chase_active", "0", 0, "")
	}
	if cvar.Get("chase_back") == nil {
		cvar.Register("chase_back", "100", 0, "")
	}
	if cvar.Get("chase_up") == nil {
		cvar.Register("chase_up", "16", 0, "")
	}
	if cvar.Get("chase_right") == nil {
		cvar.Register("chase_right", "0", 0, "")
	}
	originalActive := cvar.StringValue("chase_active")
	originalBack := cvar.StringValue("chase_back")
	originalUp := cvar.StringValue("chase_up")
	originalRight := cvar.StringValue("chase_right")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("chase_active", originalActive)
		cvar.Set("chase_back", originalBack)
		cvar.Set("chase_up", originalUp)
		cvar.Set("chase_right", originalRight)
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100
	cvar.Set("chase_active", "1")
	cvar.Set("chase_back", "100")
	cvar.Set("chase_up", "16")
	cvar.Set("chase_right", "0")

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{0, 0, 0})
	if math.Abs(float64(camera.Origin.X+100)) > 0.001 || math.Abs(float64(camera.Origin.Y)) > 0.001 || math.Abs(float64(camera.Origin.Z-16)) > 0.001 {
		t.Fatalf("runtimeCameraState chase origin = %v, want {-100 0 16}", camera.Origin)
	}
	if math.Abs(float64(camera.Angles.Y)) > 0.001 {
		t.Fatalf("runtimeCameraState chase yaw = %v, want 0", camera.Angles.Y)
	}
	if camera.Angles.X <= 0 {
		t.Fatalf("runtimeCameraState chase pitch = %v, want positive down-look pitch", camera.Angles.X)
	}
}

func TestRuntimeViewStateInterpolatesYawAcrossWrap(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.MViewAngles[1] = [3]float32{0, 350, 0}
	g.Client.MViewAngles[0] = [3]float32{0, 10, 0}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05

	_, angles := runtimeViewState()
	if math.Abs(float64(angles[1]-360)) > 0.01 && math.Abs(float64(angles[1])) > 0.01 {
		t.Fatalf("runtimeViewState wrapped yaw = %v, want 0/360 short-path interpolation", angles[1])
	}
}

func TestCollectViewModelEntityAnchorsToEyeOrigin(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	// Register view-calc cvars needed by collectViewModelEntity.
	cvar.Set("cl_bob", "0")      // disable bob so origin is predictable
	cvar.Set("cl_bobcycle", "0") // zero cycle → bob returns 0
	cvar.Set("cl_bobup", "0.5")
	cvar.Set("v_idlescale", "0") // no idle sway
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 1
	g.Client.ViewAngles = [3]float32{12, 34, 0}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = [3]float32{100, 200, 300}
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 2},
		},
	}

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != [3]float32{100, 200, 328} {
		t.Fatalf("viewmodel origin = %v, want eye origin [100 200 328]", entity.Origin)
	}
	// viewCalcGunAngle negates pitch: -(12 + 0) = -12.
	if entity.Angles[0] != -12 {
		t.Fatalf("viewmodel pitch = %v, want -12", entity.Angles[0])
	}
	if entity.Angles[1] != 34 {
		t.Fatalf("viewmodel yaw = %v, want 34", entity.Angles[1])
	}
	if entity.Frame != 1 {
		t.Fatalf("viewmodel frame = %d, want 1", entity.Frame)
	}
}

func TestCollectViewModelEntitySuppressesIntermission(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil during intermission", entity)
	}
}

func TestCollectViewModelEntityHonorsDrawViewModelCvar(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		cvar.Set("r_drawviewmodel", "1")
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	cvar.Set("cl_bobcycle", "0") // disable bob for predictable test
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	cvar.Set("r_drawviewmodel", "0")
	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when r_drawviewmodel=0", entity)
	}

	cvar.Set("r_drawviewmodel", "1")
	if entity := collectViewModelEntity(); entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity when r_drawviewmodel=1")
	}
}

func TestCollectViewModelEntitySuppressesWhenInvisible(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Items = cl.ItemInvisibility
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when invisibility is active", entity)
	}
}

func TestCollectViewModelEntitySuppressesDuringChaseCamera(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		cvar.Set("chase_active", "0")
	})

	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("chase_active", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when chase_active=1", entity)
	}
}

func TestCollectViewModelEntityAppliesPunchAndDamageKickAngles(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("cl_bobup", "0.5")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")
	cvar.Set("v_gunkick", "1")
	cvar.Set("v_kicktime", "1")

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.ViewAngles = [3]float32{12, 34, 0}
	g.Client.PunchAngle = [3]float32{2, 3, 4}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = [3]float32{100, 200, 300}
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	globalViewCalc.dmgTime = 0.5
	globalViewCalc.dmgPitch = 6
	globalViewCalc.dmgRoll = 8

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
	}
	if entity.Angles[0] != -17 {
		t.Fatalf("viewmodel pitch = %v, want -17", entity.Angles[0])
	}
	if entity.Angles[1] != 37 {
		t.Fatalf("viewmodel yaw = %v, want 37", entity.Angles[1])
	}
	if entity.Angles[2] != 8 {
		t.Fatalf("viewmodel roll = %v, want 8", entity.Angles[2])
	}
}

func TestApplyDemoPlaybackViewAnglesUpdatesCurrentAndPreviousAngles(t *testing.T) {
	clientState := cl.NewClient()
	clientState.MViewAngles[0] = [3]float32{1, 2, 3}
	clientState.ViewAngles = [3]float32{4, 5, 6}

	applyDemoPlaybackViewAngles(clientState, [3]float32{10, 20, 30})

	if clientState.MViewAngles[1] != [3]float32{1, 2, 3} {
		t.Fatalf("previous demo angles = %v, want [1 2 3]", clientState.MViewAngles[1])
	}
	if clientState.MViewAngles[0] != [3]float32{10, 20, 30} {
		t.Fatalf("current demo angles = %v, want [10 20 30]", clientState.MViewAngles[0])
	}
	if clientState.ViewAngles != [3]float32{10, 20, 30} {
		t.Fatalf("view angles = %v, want [10 20 30]", clientState.ViewAngles)
	}
}

func TestDemoPlaybackReadsOneFramePerHostFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("single_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("single_step", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after second host frame = %d, want 2", demo.FrameIndex)
	}
}

func TestPausedDemoPlaybackDoesNotReadFrames(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("paused", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("paused", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.Paused = true

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}
	if demo.FrameIndex != 0 {
		t.Fatalf("frame index while paused = %d, want 0", demo.FrameIndex)
	}
}

func TestDemoPlaybackWaitsForRecordedServerTime(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var frame bytes.Buffer
		frame.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&frame, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("binary.Write(time): %v", err)
		}
		frame.WriteByte(0xff)
		return frame.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timed", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.2), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("timed", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index before recorded time elapses = %d, want 1", demo.FrameIndex)
	}

	for i := 0; i < 6; i++ {
		if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
			t.Fatalf("Host.Frame catch-up %d: %v", i, err)
		}
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after recorded time elapses = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackTimeDemoIgnoresRecordedServerTime(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var msg bytes.Buffer
		msg.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&msg, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("Write(time): %v", err)
		}
		msg.WriteByte(0xff)
		return msg.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timedemo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(2.0), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdTimedemo("timedemo", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback || !demo.TimeDemo {
		t.Fatal("expected active timedemo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after timedemo frames = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackFlushesStuffTextSameFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	message := bytes.NewBuffer(nil)
	message.WriteByte(byte(inet.SVCStuffText))
	message.WriteString("bf\n")
	message.WriteByte(0)
	message.WriteByte(0xff)

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("stuffcmd", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(message.Bytes(), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("stuffcmd", g.Subs)

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if cmd.executes < 2 {
		t.Fatalf("executes = %d, want at least 2", cmd.executes)
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after same-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientFlushesLiveStuffTextSameFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
	})

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	tmpDir := t.TempDir()
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.StuffCmdBuf = "bf\n"

	gameCallbacks{}.ProcessClient()

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after live-frame flush", clientState.StuffCmdBuf)
	}
}

func TestRecordRuntimeDemoFrameWritesLatestServerMessage(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	g.Host = host.NewHost()
	demo := cl.NewDemoState()
	if err := demo.StartDemoRecording("runtime_demo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	t.Cleanup(func() {
		_ = demo.StopRecording()
	})
	g.Host.SetDemoState(demo)

	g.Client = cl.NewClient()
	g.Client.ViewAngles = [3]float32{10, 20, 30}
	g.Subs = &host.Subsystems{Client: &demoMessageClient{message: []byte{1, 2, 3}}}

	recordRuntimeDemoFrame()
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "demos", "runtime_demo.dem"))
	if err != nil {
		t.Fatalf("ReadFile(demo): %v", err)
	}
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 || string(data[:newline+1]) != "0\n" {
		t.Fatalf("demo header = %q, want %q", string(data), "0\\n")
	}

	reader := bytes.NewReader(data[newline+1:])
	var msgSize int32
	if err := binary.Read(reader, binary.LittleEndian, &msgSize); err != nil {
		t.Fatalf("Read(msgSize): %v", err)
	}
	if msgSize != 3 {
		t.Fatalf("msgSize = %d, want 3", msgSize)
	}
	for i, want := range [3]float32{10, 20, 30} {
		var got float32
		if err := binary.Read(reader, binary.LittleEndian, &got); err != nil {
			t.Fatalf("Read(viewAngle %d): %v", i, err)
		}
		if got != want {
			t.Fatalf("view angle %d = %v, want %v", i, got, want)
		}
	}
	frame := make([]byte, msgSize)
	if _, err := reader.Read(frame); err != nil {
		t.Fatalf("Read(frame): %v", err)
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Fatalf("frame = %v, want [1 2 3]", frame)
	}
}

func TestRuntimeAngleVectorsYawNinety(t *testing.T) {
	forward, right, up := runtimeAngleVectors([3]float32{0, 90, 0})
	if math.Abs(float64(forward[0])) > 0.0001 || math.Abs(float64(forward[1]-1)) > 0.0001 || math.Abs(float64(forward[2])) > 0.0001 {
		t.Fatalf("forward = %v, want [0 1 0]", forward)
	}
	if math.Abs(float64(right[0]-1)) > 0.0001 || math.Abs(float64(right[1])) > 0.0001 || math.Abs(float64(right[2])) > 0.0001 {
		t.Fatalf("right = %v, want [1 0 0]", right)
	}
	if math.Abs(float64(up[0])) > 0.0001 || math.Abs(float64(up[1])) > 0.0001 || math.Abs(float64(up[2]-1)) > 0.0001 {
		t.Fatalf("up = %v, want [0 0 1]", up)
	}
}

func TestRefreshRuntimeSoundCacheResetsOnPrecacheChange(t *testing.T) {
	originalClient := g.Client
	originalMap := g.SoundSFXByIndex
	originalKey := g.SoundPrecacheKey
	t.Cleanup(func() {
		g.Client = originalClient
		g.SoundSFXByIndex = originalMap
		g.SoundPrecacheKey = originalKey
	})

	g.Client = cl.NewClient()
	g.Client.SoundPrecache = []string{"weapons/rocket1.wav"}
	g.SoundPrecacheKey = "weapons/rocket1.wav"
	g.SoundSFXByIndex = map[int]*audio.SFX{1: nil}

	refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 1 {
		t.Fatalf("same precache unexpectedly reset cache; len = %d, want 1", got)
	}

	g.Client.SoundPrecache = []string{"weapons/shotgn2.wav"}
	refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("changed precache should reset cache; len = %d, want 0", got)
	}
}

func TestSyncRuntimeStaticSoundsTracksClientStateAndSnapshotChanges(t *testing.T) {
	originalClient := g.Client
	originalAudio := g.Audio
	originalSubs := g.Subs
	originalMap := g.SoundSFXByIndex
	originalPrecacheKey := g.SoundPrecacheKey
	originalStaticKey := g.StaticSoundKey
	t.Cleanup(func() {
		g.Client = originalClient
		g.Audio = originalAudio
		g.Subs = originalSubs
		g.SoundSFXByIndex = originalMap
		g.SoundPrecacheKey = originalPrecacheKey
		g.StaticSoundKey = originalStaticKey
	})

	g.Subs = nil
	g.Audio = audio.NewAudioAdapter(nil)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.SoundPrecache = []string{"ambience/drip.wav"}
	g.Client.StaticSounds = []cl.StaticSound{
		{Origin: [3]float32{10, 20, 30}, SoundIndex: 1, Volume: 255, Attenuation: 1},
	}

	syncRuntimeStaticSounds()
	firstKey := g.StaticSoundKey
	if firstKey == "" {
		t.Fatalf("expected static sound snapshot key to be populated")
	}

	syncRuntimeStaticSounds()
	if g.StaticSoundKey != firstKey {
		t.Fatalf("unchanged snapshot should not churn static key; got %q, want %q", g.StaticSoundKey, firstKey)
	}

	g.Client.StaticSounds = append(g.Client.StaticSounds, cl.StaticSound{
		Origin: [3]float32{40, 50, 60}, SoundIndex: 2, Volume: 200, Attenuation: 0.5,
	})
	syncRuntimeStaticSounds()
	secondKey := g.StaticSoundKey
	if secondKey == firstKey {
		t.Fatalf("static sound list change should rebuild snapshot key")
	}

	g.SoundSFXByIndex = map[int]*audio.SFX{1: nil}
	g.Client.SoundPrecache = []string{"ambience/wind2.wav"}
	syncRuntimeStaticSounds()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("precache change should reset runtime SFX cache before static sync; len = %d, want 0", got)
	}
	if g.StaticSoundKey == secondKey {
		t.Fatalf("precache change should rebuild static snapshot key")
	}

	g.Client.State = cl.StateConnected
	syncRuntimeStaticSounds()
	if g.StaticSoundKey != "" {
		t.Fatalf("non-active client state should clear static snapshot key, got %q", g.StaticSoundKey)
	}
}

func TestSyncRuntimeVisualEffectsEmitsParticlesAndDecals(t *testing.T) {
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ParticleEvents = []cl.ParticleEvent{
		{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 99},
	}
	g.Client.TempEntities = []cl.TempEntityEvent{
		{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}},
	}

	transientEvents := g.Client.ConsumeTransientEvents()
	syncRuntimeVisualEffects(0.1, transientEvents)

	if g.Particles == nil || g.Particles.ActiveCount() == 0 {
		t.Fatalf("expected runtime visual sync to emit particles")
	}
	gotMarks := 0
	if g.DecalMarks != nil {
		gotMarks = g.DecalMarks.ActiveCount()
	}
	if gotMarks != 1 {
		t.Fatalf("expected runtime visual sync to emit one decal mark, got %d", gotMarks)
	}
	if got := g.ParticleTime; got <= 0 {
		t.Fatalf("g.ParticleTime = %v, want > 0", got)
	}
	if len(g.Client.ParticleEvents) != 0 || len(g.Client.TempEntities) != 0 {
		t.Fatalf("runtime visual sync should consume client effect buffers")
	}
}

func TestSyncRuntimeVisualEffectsEmitsBrightFieldParticles(t *testing.T) {
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ModelPrecache = []string{"progs/player.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTFIELD},
	}

	syncRuntimeVisualEffects(0.1, cl.TransientEvents{})

	if g.Particles == nil {
		t.Fatalf("expected runtime visual sync to keep particle system initialized")
	}
	if got := g.Particles.ActiveCount(); got != 162 {
		t.Fatalf("brightfield particle count = %d, want 162", got)
	}
}

func TestSyncRuntimeVisualEffectsResetsEffectsWhenClientInactive(t *testing.T) {
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	g.DecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{0, 0, 0},
		Normal: [3]float32{0, 0, 1},
		Size:   8,
		Alpha:  1,
	}, 5, 0)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateConnected
	g.Client.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{1, 1, 1}}}

	transientEvents := g.Client.ConsumeTransientEvents()
	syncRuntimeVisualEffects(0.1, transientEvents)

	gotMarks := 0
	if g.DecalMarks != nil {
		gotMarks = g.DecalMarks.ActiveCount()
	}
	if gotMarks != 0 {
		t.Fatalf("inactive client should clear runtime decal marks")
	}
	if g.Particles == nil {
		t.Fatalf("inactive client reset should leave runtime particle system initialized")
	}
	if len(g.Client.TempEntities) != 0 {
		t.Fatalf("inactive client should consume queued temp entities")
	}
}

func TestBuildRuntimeRenderFrameStateIncludesDecalMarks(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalDraw := g.Draw
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Draw = originalDraw
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
	})

	g.Renderer = &renderer.Renderer{}
	g.Client = cl.NewClient()
	g.Client.FogDensity = 128
	g.Client.FogColor = [3]byte{64, 128, 255}
	g.Menu = nil
	g.Draw = nil
	g.Particles = renderer.NewParticleSystem(renderer.MaxParticles)
	g.DecalMarks = renderer.NewDecalMarkSystem()
	g.DecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{1, 2, 3},
		Normal: [3]float32{0, 0, 1},
		Size:   12,
		Alpha:  1,
	}, 5, 0)

	state := buildRuntimeRenderFrameState(nil, nil, []renderer.SpriteEntity{{
		ModelID: "progs/flame.spr",
		Model:   &model.Model{Type: model.ModSprite},
		Scale:   1,
	}}, nil)
	if got := len(state.DecalMarks); got != 1 {
		t.Fatalf("DecalMarks len = %d, want 1", got)
	}
	if got := len(state.SpriteEntities); got != 1 {
		t.Fatalf("SpriteEntities len = %d, want 1", got)
	}
	if !state.DrawEntities {
		t.Fatalf("DrawEntities = false, want true when sprite entities are present")
	}
	if !state.Draw2DOverlay {
		t.Fatalf("Draw2DOverlay = false, want true")
	}
	if math.Abs(float64(state.FogDensity-float32(128)/255.0)) > 0.0001 {
		t.Fatalf("FogDensity = %v, want %v", state.FogDensity, float32(128)/255.0)
	}
	if state.FogColor != [3]float32{64.0 / 255.0, 128.0 / 255.0, 1} {
		t.Fatalf("FogColor = %v, want [64/255 128/255 1]", state.FogColor)
	}
}

func TestCollectSpriteEntitiesLoadsRuntimeSprites(t *testing.T) {
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSprite(t, 1, 1),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Origin: [3]float32{7, 8, 9}, Angles: [3]float32{10, 20, 30}, Alpha: 128, Scale: 32},
	}
	g.SpriteModelCache = nil

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if entities[0].Model == nil || entities[0].Model.Type != model.ModSprite {
		t.Fatalf("collectSpriteEntities model = %#v, want sprite model", entities[0].Model)
	}
	if entities[0].SpriteData == nil || entities[0].SpriteData.NumFrames != 1 {
		t.Fatalf("collectSpriteEntities sprite data = %#v, want loaded sprite data", entities[0].SpriteData)
	}
	if got := entities[0].Alpha; math.Abs(float64(got-inet.ENTALPHA_DECODE(128))) > 0.0001 {
		t.Fatalf("collectSpriteEntities alpha = %v, want %v", got, inet.ENTALPHA_DECODE(128))
	}
	if got := entities[0].Scale; math.Abs(float64(got-inet.ENTSCALE_DECODE(32))) > 0.0001 {
		t.Fatalf("collectSpriteEntities scale = %v, want %v", got, inet.ENTSCALE_DECODE(32))
	}
	if got := entities[0].Angles; got != [3]float32{10, 20, 30} {
		t.Fatalf("collectSpriteEntities angles = %v, want [10 20 30]", got)
	}
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after first collect = %d, want 1", got)
	}

	_ = collectSpriteEntities()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after cached collect = %d, want 1", got)
	}
}

func TestResolveRuntimeSpriteFrameGroupTimingWraps(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{
			{
				Type: model.SpriteFrameGroup,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 3,
					Intervals: []float32{0.1, 0.3, 0.6},
					Frames: []*model.MSpriteFrame{
						{},
						{},
						{},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		clientTime float64
		want       int
	}{
		{name: "first interval", clientTime: 0.05, want: 0},
		{name: "second interval", clientTime: 0.20, want: 1},
		{name: "third interval", clientTime: 0.45, want: 2},
		{name: "wrap interval", clientTime: 0.65, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveRuntimeSpriteFrame(sprite, 0, [3]float32{}, viewForward, viewRight, tc.clientTime); got != tc.want {
				t.Fatalf("resolveRuntimeSpriteFrame(time=%v) = %d, want %d", tc.clientTime, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForGroupedFrames(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 3,
		Frames: []model.MSpriteFrameDesc{
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
			{
				Type: model.SpriteFrameGroup,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 2,
					Intervals: []float32{0.2, 0.4},
					Frames: []*model.MSpriteFrame{
						{},
						{},
					},
				},
			},
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
		},
	}

	if got := resolveRuntimeSpriteFrame(sprite, 1, [3]float32{}, viewForward, viewRight, 0.05); got != 1 {
		t.Fatalf("resolveRuntimeSpriteFrame(group first) = %d, want 1", got)
	}
	if got := resolveRuntimeSpriteFrame(sprite, 1, [3]float32{}, viewForward, viewRight, 0.25); got != 2 {
		t.Fatalf("resolveRuntimeSpriteFrame(group second) = %d, want 2", got)
	}
	if got := resolveRuntimeSpriteFrame(sprite, 2, [3]float32{}, viewForward, viewRight, 0.25); got != 3 {
		t.Fatalf("resolveRuntimeSpriteFrame(single after group) = %d, want 3", got)
	}
}

func TestResolveRuntimeSpriteFrameAngledUsesViewDirection(t *testing.T) {
	sprite := &model.MSprite{
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{
			{
				Type: model.SpriteFrameAngled,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 8,
					Intervals: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
					Frames: []*model.MSpriteFrame{
						{}, {}, {}, {}, {}, {}, {}, {},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		viewAngles [3]float32
		want       int
	}{
		{name: "front", viewAngles: [3]float32{0, 0, 0}, want: 4},
		{name: "right", viewAngles: [3]float32{0, 90, 0}, want: 6},
		{name: "back", viewAngles: [3]float32{0, 180, 0}, want: 0},
		{name: "left", viewAngles: [3]float32{0, 270, 0}, want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			viewForward, viewRight, _ := runtimeAngleVectors(tc.viewAngles)
			if got := resolveRuntimeSpriteFrame(sprite, 0, [3]float32{}, viewForward, viewRight, 0.35); got != tc.want {
				t.Fatalf("resolveRuntimeSpriteFrame(view=%v) = %d, want %d", tc.viewAngles, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForAngledFrames(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 2,
		Frames: []model.MSpriteFrameDesc{
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
			{
				Type: model.SpriteFrameAngled,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 8,
					Intervals: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
					Frames: []*model.MSpriteFrame{
						{}, {}, {}, {}, {}, {}, {}, {},
					},
				},
			},
		},
	}

	if got := resolveRuntimeSpriteFrame(sprite, 1, [3]float32{}, viewForward, viewRight, 0.35); got != 5 {
		t.Fatalf("resolveRuntimeSpriteFrame(angled offset) = %d, want 5", got)
	}
}

func TestCollectSpriteEntitiesResolvesGroupedFrameFromClientTime(t *testing.T) {
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSpriteGroup(t, 2, []float32{0.2, 0.4}),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Time = 0.25
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0},
	}
	g.SpriteModelCache = nil

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 1 {
		t.Fatalf("collectSpriteEntities grouped frame = %d, want 1", got)
	}
}

func TestCollectSpriteEntitiesResolvesAngledFrameFromViewAngles(t *testing.T) {
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeAngledSprite(t),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.ViewAngles = [3]float32{0, 90, 0}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Angles: [3]float32{0, 0, 0}},
	}
	g.SpriteModelCache = nil

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 6 {
		t.Fatalf("collectSpriteEntities angled frame = %d, want 6", got)
	}
}

func TestEntityStateScaleDecodesProtocolScale(t *testing.T) {
	if got := entityStateScale(inet.EntityState{Scale: inet.ENTSCALE_DEFAULT}); got != 1 {
		t.Fatalf("entityStateScale(default) = %v, want 1", got)
	}
	if got := entityStateScale(inet.EntityState{Scale: 32}); got != 2 {
		t.Fatalf("entityStateScale(32) = %v, want 2", got)
	}
	if got := entityStateScale(inet.EntityState{}); got != 1 {
		t.Fatalf("entityStateScale(zero) = %v, want 1 fallback", got)
	}
}

func TestCollectEntityEffectSourcesKeepsAliasEffectsOnly(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{
		"progs/player.mdl",
		"*1",
		"progs/flame.spr",
	}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{1, 2, 3}, Angles: [3]float32{0, 90, 0}, Effects: inet.EF_MUZZLEFLASH},
		2: {ModelIndex: 2, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTLIGHT},
		3: {ModelIndex: 3, Origin: [3]float32{7, 8, 9}, Effects: inet.EF_DIMLIGHT},
		4: {ModelIndex: 1, Origin: [3]float32{9, 9, 9}},
	}
	g.Client.StaticEntities = []inet.EntityState{
		{ModelIndex: 1, Origin: [3]float32{10, 11, 12}, Effects: inet.EF_DIMLIGHT},
	}

	sources := collectEntityEffectSources()
	if got := len(sources); got != 2 {
		t.Fatalf("collectEntityEffectSources len = %d, want 2", got)
	}
	if sources[0].Origin != [3]float32{1, 2, 3} || sources[0].Effects != inet.EF_MUZZLEFLASH {
		t.Fatalf("first effect source = %#v, want alias muzzle-flash source", sources[0])
	}
	if sources[1].Origin != [3]float32{10, 11, 12} || sources[1].Effects != inet.EF_DIMLIGHT {
		t.Fatalf("second effect source = %#v, want static alias dim-light source", sources[1])
	}
}

func TestCollectBrushEntitiesDecodesProtocolAlphaAndScale(t *testing.T) {
	originalClient := g.Client
	originalServer := g.Server
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"maps/start.bsp", "*1"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 2,
			Frame:      3,
			Origin:     [3]float32{1, 2, 3},
			Angles:     [3]float32{10, 20, 30},
			Alpha:      128,
			Scale:      32,
		},
	}
	g.Server = &server.Server{WorldTree: &bsp.Tree{Models: []bsp.DModel{{}, {}}}}

	brushEntities := collectBrushEntities()
	if got := len(brushEntities); got != 1 {
		t.Fatalf("collectBrushEntities len = %d, want 1", got)
	}
	if brushEntities[0].SubmodelIndex != 1 || brushEntities[0].Origin != [3]float32{1, 2, 3} {
		t.Fatalf("brush entity = %#v, want submodel 1 at origin [1 2 3]", brushEntities[0])
	}
	if brushEntities[0].Frame != 3 {
		t.Fatalf("brush frame = %d, want 3", brushEntities[0].Frame)
	}
	if got := brushEntities[0].Alpha; math.Abs(float64(got-inet.ENTALPHA_DECODE(128))) > 0.0001 {
		t.Fatalf("brush alpha = %v, want %v", got, inet.ENTALPHA_DECODE(128))
	}
	if got := brushEntities[0].Scale; math.Abs(float64(got-inet.ENTSCALE_DECODE(32))) > 0.0001 {
		t.Fatalf("brush scale = %v, want %v", got, inet.ENTSCALE_DECODE(32))
	}
}

func TestUpdateHUDFromServerUsesClientState(t *testing.T) {
	originalHUD := g.HUD
	originalClient := g.Client
	originalServer := g.Server
	originalShowScores := g.ShowScores
	t.Cleanup(func() {
		g.HUD = originalHUD
		g.Client = originalClient
		g.Server = originalServer
		g.ShowScores = originalShowScores
	})

	g.HUD = hud.NewHUD(nil)
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 111
	g.Client.Stats[inet.StatArmor] = 55
	g.Client.Stats[inet.StatAmmo] = 22
	g.Client.Stats[inet.StatWeapon] = 7
	g.Client.Stats[inet.StatActiveWeapon] = cl.ItemRocketLauncher
	g.Client.Stats[inet.StatShells] = 10
	g.Client.Stats[inet.StatNails] = 20
	g.Client.Stats[inet.StatRockets] = 30
	g.Client.Stats[inet.StatCells] = 40
	g.Client.Stats[inet.StatTotalSecrets] = 9
	g.Client.Stats[inet.StatTotalMonsters] = 66
	g.Client.Stats[inet.StatSecrets] = 3
	g.Client.Stats[inet.StatMonsters] = 12
	g.Client.MaxClients = 4
	g.Client.GameType = 1
	g.Client.ViewEntity = 2
	g.Client.PlayerNames[0] = "alpha"
	g.Client.PlayerNames[1] = "bravo"
	g.Client.PlayerNames[2] = "charlie"
	g.Client.PlayerColors[0] = 0x1f
	g.Client.PlayerColors[1] = 0x2e
	g.Client.PlayerColors[2] = 0x3d
	g.Client.Frags[0] = 4
	g.Client.Frags[1] = 10
	g.Client.Frags[2] = 6
	g.Client.Items = cl.ItemRocketLauncher | cl.ItemRockets | cl.ItemArmor2 | cl.ItemQuad
	g.Client.Intermission = 2
	g.Client.CompletedTime = 123
	g.Client.Time = 124
	g.Client.CenterPrint = "The End"
	g.Client.CenterPrintAt = 120
	g.Client.LevelName = "Unit Test Map"
	g.ShowScores = true

	updateHUDFromServer()

	got := g.HUD.State()
	if got.Health != 111 || got.Armor != 55 || got.Ammo != 22 {
		t.Fatalf("hud core stats = %#v, want health=111 armor=55 ammo=22", got)
	}
	if got.WeaponModel != 7 || got.ActiveWeapon != cl.ItemRocketLauncher {
		t.Fatalf("hud weapon state = %#v, want model=7 active=%d", got, cl.ItemRocketLauncher)
	}
	if got.Shells != 10 || got.Nails != 20 || got.Rockets != 30 || got.Cells != 40 {
		t.Fatalf("hud ammo strip = %#v, want [10 20 30 40]", got)
	}
	if got.Items != g.Client.Items {
		t.Fatalf("hud items = %#x, want %#x", got.Items, g.Client.Items)
	}
	if got.Intermission != 2 || got.CompletedTime != 123 || got.Time != 124 {
		t.Fatalf("hud intermission state = %#v", got)
	}
	if got.CenterPrint != "The End" || got.CenterPrintAt != 120 || got.LevelName != "Unit Test Map" {
		t.Fatalf("hud center/intermission text state = %#v", got)
	}
	if got.Secrets != 3 || got.TotalSecrets != 9 || got.Monsters != 12 || got.TotalMonsters != 66 {
		t.Fatalf("hud intermission stats = %#v", got)
	}
	if !got.ShowScores || got.GameType != 1 || got.MaxClients != 4 {
		t.Fatalf("hud multiplayer state = %#v", got)
	}
	if len(got.Scoreboard) != 3 {
		t.Fatalf("hud scoreboard len = %d, want 3", len(got.Scoreboard))
	}
	if got.Scoreboard[0].Name != "bravo" || got.Scoreboard[0].Frags != 10 || !got.Scoreboard[0].IsCurrent {
		t.Fatalf("hud scoreboard top row = %#v, want bravo/10/current", got.Scoreboard[0])
	}
}

func TestApplyDefaultGameplayBindings(t *testing.T) {
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	g.Input = input.NewSystem(nil)
	applyDefaultGameplayBindings()

	cases := []struct {
		key  int
		want string
	}{
		{key: int('`'), want: "toggleconsole"},
		{key: int('w'), want: "+forward"},
		{key: input.KUpArrow, want: "+forward"},
		{key: input.KMouse1, want: "+attack"},
		{key: input.KMouse2, want: "+jump"},
		{key: input.KTab, want: "+showscores"},
		{key: input.KMWheelUp, want: "impulse 10"},
		{key: input.KMWheelDown, want: "impulse 12"},
	}

	for _, tc := range cases {
		if got := g.Input.GetBinding(tc.key); got != tc.want {
			t.Fatalf("binding for key %d = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestGameplayBindCommandsAndDispatch(t *testing.T) {
	originalInput := g.Input
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Client = originalClient
	})

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client = cl.NewClient()
	registerGameplayBindCommands()

	cmdsys.ExecuteText("unbindall")
	cmdsys.ExecuteText("bind w +forward")
	cmdsys.ExecuteText("bind MWHEELUP \"impulse 12\"")

	if got := g.Input.GetBinding(int('w')); got != "+forward" {
		t.Fatalf("bind command did not set w binding, got %q", got)
	}
	if got := g.Input.GetBinding(input.KMWheelUp); got != "impulse 12" {
		t.Fatalf("bind command did not set MWHEELUP binding, got %q", got)
	}

	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("expected +forward to press InputForward")
	}
	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: false})
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("expected -forward to release InputForward")
	}

	handleGameKeyEvent(input.KeyEvent{Key: input.KMWheelUp, Down: true})
	if g.Client.InImpulse != 12 {
		t.Fatalf("expected wheel bind to set impulse 12, got %d", g.Client.InImpulse)
	}

	cmdsys.ExecuteText("unbind w")
	if got := g.Input.GetBinding(int('w')); got != "" {
		t.Fatalf("unbind did not clear w binding, got %q", got)
	}

	cmdsys.ExecuteText("unbindall")
	if got := g.Input.GetBinding(input.KMWheelUp); got != "" {
		t.Fatalf("unbindall did not clear MWHEELUP binding, got %q", got)
	}
}

func TestSyncGameplayInputModeClearsHeldScoreboardOutsideGameInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalShowScores := g.ShowScores
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.ShowScores = originalShowScores
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	g.Input.SetKeyDest(input.KeyGame)
	g.MouseGrabbed = false
	syncGameplayInputMode()

	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})
	if !g.ShowScores {
		t.Fatalf("+showscores should set held scoreboard state")
	}

	g.Input.SetKeyDest(input.KeyConsole)
	syncGameplayInputMode()
	if g.ShowScores {
		t.Fatalf("scoreboard hold should clear when leaving gameplay input")
	}
}

func TestStartupMenuStateSuppressesGameplayMovementInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	// initSubsystems shows the menu at startup; +map start does not close it.
	g.Menu.ShowMenu()
	syncGameplayInputMode()
	if got := g.Input.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination with startup menu active = %v, want menu", got)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("+forward should not activate while key destination is menu")
	}
	if g.Client.InputJump.State&1 != 0 {
		t.Fatalf("+jump should not activate while key destination is menu")
	}

	g.Menu.HideMenu()
	syncGameplayInputMode()
	g.Input.ClearKeyStates()

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after menu closes")
	}
	if g.Client.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after menu closes")
	}
}

func TestApplyStartupGameplayInputModeHidesMenuAndEnablesMovementInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	g.Menu.ShowMenu()
	syncGameplayInputMode()
	if got := g.Input.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination before startup transition = %v, want menu", got)
	}

	applyStartupGameplayInputMode()
	if g.Menu.IsActive() {
		t.Fatalf("startup transition should hide menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after startup transition = %v, want game", got)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after startup transition")
	}
	if g.Client.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after startup transition")
	}
}

func TestHostInitLoadsBindingOverridesFromConfig(t *testing.T) {
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	userDir := t.TempDir()
	configPath := filepath.Join(userDir, "config.cfg")
	if err := os.WriteFile(configPath, []byte("bind w +back\nbind F10 +attack\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configPath, err)
	}

	h := host.NewHost()
	subs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := h.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if got := g.Input.GetBinding(int('w')); got != "+back" {
		t.Fatalf("binding for w after config load = %q, want %q", got, "+back")
	}
	if got := g.Input.GetBinding(input.KF10); got != "+attack" {
		t.Fatalf("binding for F10 after config load = %q, want %q", got, "+attack")
	}
}

func TestQuotedBindingsRoundTripThroughConfig(t *testing.T) {
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	userDir := t.TempDir()
	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()

	writerHost := host.NewHost()
	writerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := writerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, writerSubs); err != nil {
		t.Fatalf("writer Init failed: %v", err)
	}

	want := "say He said \"hello\" \\world\nnext\tline"
	cmdsys.ExecuteText(`bind t "say He said \"hello\" \\world\nnext\tline"`)
	if got := g.Input.GetBinding(int('t')); got != want {
		t.Fatalf("binding before save = %q, want %q", got, want)
	}
	if err := writerHost.WriteConfig(writerSubs); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	readerHost := host.NewHost()
	readerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := readerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, readerSubs); err != nil {
		t.Fatalf("reader Init failed: %v", err)
	}

	if got := g.Input.GetBinding(int('t')); got != want {
		t.Fatalf("binding after reload = %q, want %q", got, want)
	}
}

func TestSyncControlCvarsToClient(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	g.Client = cl.NewClient()
	syncControlCvarsToClient()

	if g.Client.AlwaysRun {
		t.Fatalf("AlwaysRun should follow cl_alwaysrun")
	}
	if g.Client.FreeLook {
		t.Fatalf("FreeLook should follow freelook")
	}
	if !g.Client.LookSpring {
		t.Fatalf("LookSpring should follow lookspring")
	}
}

func TestSyncHostClientStateReappliesControlCvarsOnClientReplacement(t *testing.T) {
	originalClient := g.Client
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	firstClient := cl.NewClient()
	g.Subs = &host.Subsystems{
		Client: &activeStateTestClient{
			state:       host.ClientState(1),
			clientState: firstClient,
		},
	}
	syncHostClientState()

	if firstClient.AlwaysRun || firstClient.FreeLook || !firstClient.LookSpring {
		t.Fatalf("first client controls = %+v, want alwaysrun=false freelook=false lookspring=true", firstClient)
	}

	replacedClient := cl.NewClient()
	replacedClient.AlwaysRun = true
	replacedClient.FreeLook = true
	replacedClient.LookSpring = false
	g.Subs.Client = &activeStateTestClient{
		state:       host.ClientState(1),
		clientState: replacedClient,
	}
	syncHostClientState()

	if replacedClient.AlwaysRun || replacedClient.FreeLook || !replacedClient.LookSpring {
		t.Fatalf("replaced client controls = %+v, want alwaysrun=false freelook=false lookspring=true", replacedClient)
	}
}

func TestApplyGameplayMouseLookUsesControlCvars(t *testing.T) {
	originalInput := g.Input
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Client = originalClient
	})

	registerControlCvars()
	backend := &mouseDeltaBackend{}
	g.Input = input.NewSystem(backend)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client = cl.NewClient()

	cvar.Set("sensitivity", "10")
	cvar.Set("m_yaw", "0.01")
	cvar.Set("m_pitch", "0.02")
	cvar.Set("freelook", "1")

	backend.dx = 2
	backend.dy = 3
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[1]; math.Abs(float64(got-(-0.2))) > 0.0001 {
		t.Fatalf("yaw after mouse look = %.2f, want -0.20", got)
	}
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-0.6)) > 0.0001 {
		t.Fatalf("pitch after mouse look = %.2f, want 0.60", got)
	}

	g.Client.ViewAngles = [3]float32{}
	cvar.Set("freelook", "0")
	backend.dx = 0
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; got != 0 {
		t.Fatalf("pitch should stay unchanged when freelook is off and +mlook inactive, got %.2f", got)
	}

	g.Client.InputMLook.State = 1
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-1.0)) > 0.0001 {
		t.Fatalf("pitch with +mlook held = %.2f, want 1.00", got)
	}

	g.Client.ViewAngles = [3]float32{}
	g.Client.InputMLook.State = 0
	cvar.Set("freelook", "1")
	cvar.Set("m_pitch", "-0.02")
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-(-1.0))) > 0.0001 {
		t.Fatalf("pitch with inverted mouse = %.2f, want -1.00", got)
	}
}

func TestToggleConsoleClosesMenuAndSwitchesKeyDest(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.MouseGrabbed = true

	cmdToggleConsole(nil)

	if g.Menu.IsActive() {
		t.Fatalf("toggleconsole should hide the menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after toggleconsole = %v, want console", got)
	}
	if g.MouseGrabbed {
		t.Fatalf("console mode should release mouse grab")
	}

	cmdToggleConsole(nil)
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestMenuTapDownMovesCursorOnce(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after down+enter tap = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestMenuTapEscapeFromSubmenuReturnsToMain(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent

	// Enter multiplayer menu.
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after entering submenu = %v, want %v", got, menu.MenuMultiPlayer)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: false})

	if !g.Menu.IsActive() {
		t.Fatalf("menu should remain active after escape tap from submenu")
	}
	if got := g.Menu.GetState(); got != menu.MenuMain {
		t.Fatalf("menu state after escape tap = %v, want %v", got, menu.MenuMain)
	}
}

func TestMenuCharRoutingUpdatesSetupName(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent

	// Enter multiplayer -> setup.
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := g.Menu.GetState(); got != menu.MenuSetup {
		t.Fatalf("menu state = %v, want %v", got, menu.MenuSetup)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // name
	g.Input.HandleCharEvent('x')
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // shirt
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // pants
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // accept
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after accept = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestConsoleKeyRoutingExecutesCommands(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Input.SetKeyDest(input.KeyGame)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	var gotArgs []string
	cmdsys.AddCommand("testconsolecmd", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "test console command")

	handleGameKeyEvent(input.KeyEvent{Key: int('`'), Down: true})
	if got := g.Input.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after console bind = %v, want console", got)
	}

	for _, ch := range "testconsolecmd 42" {
		handleGameCharEvent(ch)
	}
	if got := console.InputLine(); got != "testconsolecmd 42" {
		t.Fatalf("console input line = %q, want %q", got, "testconsolecmd 42")
	}

	handleGameKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	if len(gotArgs) != 1 || gotArgs[0] != "42" {
		t.Fatalf("console command args = %v, want [42]", gotArgs)
	}
	if got := console.InputLine(); got != "" {
		t.Fatalf("console input line after enter = %q, want empty", got)
	}

	handleGameKeyEvent(input.KeyEvent{Key: int('`'), Down: true})
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestConsoleTabCompletionCompletesCommand(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	console.ResetCompletion()

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	g.Input.SetKeyDest(input.KeyConsole)

	for _, ch := range "tog" {
		handleGameCharEvent(ch)
	}
	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})

	if got := console.InputLine(); got != "toggleconsole" {
		t.Fatalf("console input line after tab completion = %q, want %q", got, "toggleconsole")
	}
}

func TestHandleGameKeyEventUnboundSpecialKeyFeedback(t *testing.T) {
	originalInput := g.Input
	originalHost := g.Host
	t.Cleanup(func() {
		g.Input = originalInput
		g.Host = originalHost
		console.SetPrintCallback(nil)
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyGame)

	var printed strings.Builder
	console.SetPrintCallback(func(msg string) {
		printed.WriteString(msg)
	})

	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	output := printed.String()
	if !strings.Contains(output, "MOUSE4 is unbound, use Options menu to set.") {
		t.Fatalf("unbound special-key feedback = %q, missing expected hint", output)
	}

	printed.Reset()
	g.Input.SetKeyDest(input.KeyMenu)
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("menu destination should not print unbound game hint, got %q", got)
	}

	printed.Reset()
	g.Input.SetKeyDest(input.KeyGame)
	g.Host = host.NewHost()
	g.Host.SetDemoState(&cl.DemoState{Playback: true})
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("demo playback should suppress unbound hint, got %q", got)
	}
}

func TestRuntimeMusicSelectionUsesDemoHeaderFallback(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = host.NewHost()
	demo := cl.NewDemoState()
	demo.Playback = true
	demo.CDTrack = 5
	g.Host.SetDemoState(demo)
	g.Client = cl.NewClient()

	track, loopTrack := runtimeMusicSelection()
	if track != 5 || loopTrack != 5 {
		t.Fatalf("runtimeMusicSelection() = %d/%d, want 5/5", track, loopTrack)
	}

	g.Client.CDTrack = 2
	g.Client.LoopTrack = 3
	track, loopTrack = runtimeMusicSelection()
	if track != 2 || loopTrack != 3 {
		t.Fatalf("runtimeMusicSelection() with live client track = %d/%d, want 2/3", track, loopTrack)
	}
}

func TestSyncRuntimeMusicLoadsTrackOnceAndStops(t *testing.T) {
	originalAudio := g.Audio
	originalClient := g.Client
	originalHost := g.Host
	originalSubs := g.Subs
	originalKey := g.MusicTrackKey
	t.Cleanup(func() {
		g.Audio = originalAudio
		g.Client = originalClient
		g.Host = originalHost
		g.Subs = originalSubs
		g.MusicTrackKey = originalKey
	})

	sys := &audio.System{}
	sys = audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	g.Audio = audio.NewAudioAdapter(sys)
	g.Client = cl.NewClient()
	g.Client.CDTrack = 2
	g.Client.LoopTrack = 2
	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"music/track02.wav": testRuntimeMusicWAV(t, 44100, 2, 2, 64),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}

	syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 2 {
		t.Fatalf("CurrentMusicTrack = %d, want 2", got)
	}
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want 1 after first sync", got)
	}

	syncRuntimeMusic()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want no reload for unchanged request", got)
	}

	g.Client.CDTrack = 0
	g.Client.LoopTrack = 0
	syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 0 {
		t.Fatalf("CurrentMusicTrack = %d, want 0 after stopping music", got)
	}
}

func TestApplySVolumeUsesCVarAndClamps(t *testing.T) {
	originalAudio := g.Audio
	t.Cleanup(func() {
		g.Audio = originalAudio
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	g.Audio = audio.NewAudioAdapter(sys)

	cv := cvar.Get("s_volume")
	if cv == nil {
		cv = cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	}
	originalValue := cv.String
	originalCallback := cv.Callback
	t.Cleanup(func() {
		cv.Callback = originalCallback
		cvar.Set("s_volume", originalValue)
	})

	cvar.Set("s_volume", "0.25")
	applySVolume()
	if got := sys.Volume(); math.Abs(got-0.25) > 0.0001 {
		t.Fatalf("volume after s_volume=0.25 = %v, want 0.25", got)
	}

	cvar.Set("s_volume", "2.5")
	applySVolume()
	if got := sys.Volume(); math.Abs(got-1.0) > 0.0001 {
		t.Fatalf("volume after s_volume=2.5 = %v, want clamped 1.0", got)
	}
}

type runtimeMusicTestFS struct {
	files map[string][]byte
	loads int
}

func (fsys *runtimeMusicTestFS) Init(baseDir, gameDir string) error { return nil }
func (fsys *runtimeMusicTestFS) Close()                             {}

func (fsys *runtimeMusicTestFS) LoadFile(filename string) ([]byte, error) {
	fsys.loads++
	if data, ok := fsys.files[filename]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("missing %s", filename)
}

func (fsys *runtimeMusicTestFS) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	fsys.loads++
	for _, filename := range filenames {
		if data, ok := fsys.files[filename]; ok {
			return filename, data, nil
		}
	}
	return "", nil, fmt.Errorf("missing files: %v", filenames)
}

func (fsys *runtimeMusicTestFS) FileExists(filename string) bool {
	_, ok := fsys.files[filename]
	return ok
}

func testRuntimeMusicWAV(t *testing.T, sampleRate, channels, width, frames int) []byte {
	t.Helper()

	blockAlign := channels * width
	dataSize := frames * blockAlign
	var data bytes.Buffer
	for frame := 0; frame < frames; frame++ {
		for channel := 0; channel < channels; channel++ {
			sample := int16((frame + 1) * 128)
			if channel%2 == 1 {
				sample = -sample
			}
			if err := binary.Write(&data, binary.LittleEndian, sample); err != nil {
				t.Fatalf("binary.Write sample: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	writeString := func(value string) {
		if _, err := wav.WriteString(value); err != nil {
			t.Fatalf("WriteString(%q): %v", value, err)
		}
	}

	writeString("RIFF")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatalf("binary.Write RIFF size: %v", err)
	}
	writeString("WAVE")
	writeString("fmt ")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatalf("binary.Write fmt size: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatalf("binary.Write format: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(channels)); err != nil {
		t.Fatalf("binary.Write channels: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatalf("binary.Write sample rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*blockAlign)); err != nil {
		t.Fatalf("binary.Write byte rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(blockAlign)); err != nil {
		t.Fatalf("binary.Write block align: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(width*8)); err != nil {
		t.Fatalf("binary.Write bits: %v", err)
	}
	writeString("data")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatalf("binary.Write data size: %v", err)
	}
	if _, err := wav.Write(data.Bytes()); err != nil {
		t.Fatalf("Write data: %v", err)
	}
	return wav.Bytes()
}

func testRuntimeSprite(t *testing.T, width, height int32) []byte {
	t.Helper()

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(width))
	write(width)
	write(height)
	write(int32(1))
	write(float32(0))
	write(int32(0))
	write(int32(model.SpriteFrameSingle))
	write([2]int32{0, 0})
	write(width)
	write(height)
	if _, err := spr.Write([]byte{1}); err != nil {
		t.Fatalf("Write pixel data: %v", err)
	}

	return spr.Bytes()
}

func testRuntimeSpriteGroup(t *testing.T, frames int32, intervals []float32) []byte {
	t.Helper()
	if frames <= 0 {
		t.Fatalf("invalid frame count: %d", frames)
	}
	if len(intervals) != int(frames) {
		t.Fatalf("interval count = %d, want %d", len(intervals), frames)
	}

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(1))
	write(int32(1))
	write(int32(1))
	write(int32(1))
	write(float32(0))
	write(int32(0))

	write(int32(model.SpriteFrameGroup))
	write(frames)
	for _, interval := range intervals {
		write(interval)
	}
	for i := int32(0); i < frames; i++ {
		write([2]int32{0, 0})
		write(int32(1))
		write(int32(1))
		if err := spr.WriteByte(byte(i + 1)); err != nil {
			t.Fatalf("Write pixel data: %v", err)
		}
	}

	return spr.Bytes()
}

func testRuntimeAngledSprite(t *testing.T) []byte {
	t.Helper()

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(1))
	write(int32(1))
	write(int32(1))
	write(int32(1))
	write(float32(0))
	write(int32(0))

	write(int32(model.SpriteFrameAngled))
	write(int32(8))
	for i := 0; i < 8; i++ {
		write(float32(i+1) * 0.1)
	}
	for i := 0; i < 8; i++ {
		write([2]int32{0, 0})
		write(int32(1))
		write(int32(1))
		if err := spr.WriteByte(byte(i + 1)); err != nil {
			t.Fatalf("Write pixel data: %v", err)
		}
	}

	return spr.Bytes()
}
