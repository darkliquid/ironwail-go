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

func (c *demoMessageClient) Init() error               { return nil }
func (c *demoMessageClient) Frame(float64) error       { return nil }
func (c *demoMessageClient) Shutdown()                 {}
func (c *demoMessageClient) State() host.ClientState   { return 0 }
func (c *demoMessageClient) ReadFromServer() error     { return nil }
func (c *demoMessageClient) SendCommand() error        { return nil }
func (c *demoMessageClient) SendStringCmd(string) error { return nil }
func (c *demoMessageClient) LastServerMessage() []byte { return append([]byte(nil), c.message...) }

type activeStateTestClient struct {
	state       host.ClientState
	clientState *cl.Client
}

func (c *activeStateTestClient) Init() error         { return nil }
func (c *activeStateTestClient) Frame(float64) error { return nil }
func (c *activeStateTestClient) Shutdown()           {}
func (c *activeStateTestClient) State() host.ClientState { return c.state }
func (c *activeStateTestClient) ReadFromServer() error   { return nil }
func (c *activeStateTestClient) SendCommand() error      { return nil }
func (c *activeStateTestClient) SendStringCmd(string) error { return nil }
func (c *activeStateTestClient) ClientState() *cl.Client { return c.clientState }

type demoPlaybackNoopServer struct{}

func (s *demoPlaybackNoopServer) Init(int) error                           { return nil }
func (s *demoPlaybackNoopServer) SpawnServer(string, *fs.FileSystem) error { return nil }
func (s *demoPlaybackNoopServer) ConnectClient(int)                        {}
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

func (c *demoPlaybackConsole) Init() error  { return nil }
func (c *demoPlaybackConsole) Print(string) {}
func (c *demoPlaybackConsole) Clear()       {}
func (c *demoPlaybackConsole) Dump(string) error { return nil }
func (c *demoPlaybackConsole) Shutdown()    {}

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
	originalHost := gameHost
	originalClient := gameClient
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
	})

	gameHost = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.Entities[0] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	gameClient.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	runRuntimeFrame(0.016, gameCallbacks{})

	if got := gameClient.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
}

func TestRunRuntimeFrameSyncsAudioViewEntity(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	originalAudio := gameAudio
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
		gameAudio = originalAudio
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	gameHost = nil
	gameAudio = audio.NewAudioAdapter(sys)
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ViewEntity = 3
	gameClient.ViewHeight = 22
	gameClient.Entities[3] = inet.EntityState{Origin: [3]float32{64, 32, 16}}

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 3 {
		t.Fatalf("audio view entity after active client frame = %d, want 3", got)
	}

	gameClient = nil
	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 0 {
		t.Fatalf("audio view entity after clearing client = %d, want 0", got)
	}
}

func TestRunRuntimeFrameUpdatesLeafAmbientAndUnderwaterAudio(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	originalAudio := gameAudio
	originalServer := gameServer
	originalSubs := gameSubs
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
		gameAudio = originalAudio
		gameServer = originalServer
		gameSubs = originalSubs
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	gameAudio = audio.NewAudioAdapter(sys)
	gameAudio.SetAmbientSound(0, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})
	gameAudio.SetAmbientSound(1, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})

	gameHost = nil
	gameSubs = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ViewEntity = 1
	gameClient.ViewHeight = 0
	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{64, 0, 0}}
	gameServer = &server.Server{
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

	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{-64, 0, 0}}
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

	gameServer = nil
	runRuntimeFrame(0.1, gameCallbacks{})
	if sys.AmbientSound(0) != nil || sys.AmbientSound(1) != nil {
		t.Fatalf("ambient channels should clear when no world tree is available")
	}
}

func TestRunRuntimeFrameConsumesTransientEventsOnce(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
	})

	gameHost = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.SoundEvents = []cl.SoundEvent{{Entity: 1, Channel: 2, SoundIndex: 3}}
	gameClient.StopSoundEvents = []cl.StopSoundEvent{{Entity: 4, Channel: 5}}
	gameClient.ParticleEvents = []cl.ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	gameClient.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}}}

	events := runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 1 || len(events.StopSoundEvents) != 1 || len(events.ParticleEvents) != 1 || len(events.TempEntities) != 1 {
		t.Fatalf("runRuntimeFrame consumed = %d sounds, %d stops, %d particles, %d temps; want 1,1,1,1", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
	if len(gameClient.SoundEvents) != 0 || len(gameClient.StopSoundEvents) != 0 || len(gameClient.ParticleEvents) != 0 || len(gameClient.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d sounds %d stops %d particles %d temps", len(gameClient.SoundEvents), len(gameClient.StopSoundEvents), len(gameClient.ParticleEvents), len(gameClient.TempEntities))
	}

	events = runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 0 || len(events.StopSoundEvents) != 0 || len(events.ParticleEvents) != 0 || len(events.TempEntities) != 0 {
		t.Fatalf("second frame consumed = %d sounds, %d stops, %d particles, %d temps; want 0,0,0,0", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
}

func TestRuntimeViewStatePrefersAuthoritativeViewEntityOrigin(t *testing.T) {
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.ViewEntity = 1
	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{128, 64, 32}}
	gameClient.PredictedOrigin = [3]float32{64, 32, 16}
	gameClient.ViewHeight = 30
	gameClient.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if want := [3]float32{128, 64, 62}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != gameClient.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, gameClient.ViewAngles)
	}
}

func TestRuntimeViewStateFallsBackToPredictedOrigin(t *testing.T) {
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.ViewEntity = 1
	gameClient.PredictedOrigin = [3]float32{128, 64, 32}
	gameClient.ViewHeight = 18
	gameClient.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if want := [3]float32{128, 64, 50}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != gameClient.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, gameClient.ViewAngles)
	}
}

func TestRuntimeViewStateUsesPredictedXYOffsetDuringActiveMovement(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameHost = nil
	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ViewEntity = 1
	gameClient.ViewHeight = 22
	gameClient.ViewAngles = [3]float32{10, 20, 0}
	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	gameClient.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := gameClient.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
	if got := gameClient.PredictedOrigin; got[2] >= 300 {
		t.Fatalf("expected collisionless prediction to drift below authoritative Z, got %#v", got)
	}

	origin, _ := runtimeViewState()
	if want := [3]float32{gameClient.PredictedOrigin[0], gameClient.PredictedOrigin[1], 300 + gameClient.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want predicted XY with authoritative Z %v", origin, want)
	}
}

func TestRuntimeViewStateClampsPredictedXYOffset(t *testing.T) {
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ViewEntity = 1
	gameClient.ViewHeight = 22
	gameClient.ViewAngles = [3]float32{10, 20, 0}
	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	gameClient.PredictedOrigin = [3]float32{120, 240, 280}
	gameClient.PendingCmd = cl.UserCmd{Forward: 100}

	offsetScale := float32(runtimeMaxPredictedXYOffset / math.Hypot(20, 40))
	want := [3]float32{
		100 + 20*offsetScale,
		200 + 40*offsetScale,
		300 + gameClient.ViewHeight,
	}

	origin, _ := runtimeViewState()
	if origin != want {
		t.Fatalf("runtimeViewState origin = %v, want clamped predicted XY %v", origin, want)
	}
}

func TestRuntimeViewStateIgnoresPredictedXYOffsetOnLargePredictionError(t *testing.T) {
	originalClient := gameClient
	originalServer := gameServer
	originalRenderer := gameRenderer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
		gameRenderer = originalRenderer
	})

	gameServer = nil
	gameRenderer = nil
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ViewEntity = 1
	gameClient.ViewHeight = 22
	gameClient.ViewAngles = [3]float32{10, 20, 0}
	gameClient.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	gameClient.PredictedOrigin = [3]float32{110, 200, 280}
	gameClient.PredictionError = [3]float32{runtimeMaxPredictedXYOffset + 1, 0, 0}
	gameClient.PendingCmd = cl.UserCmd{Forward: 100}

	origin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 300 + gameClient.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeCameraStateCarriesClientTime(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.Time = 12.5

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{4, 5, 6})
	if camera.Time != 12.5 {
		t.Fatalf("runtimeCameraState time = %v, want 12.5", camera.Time)
	}
}

func TestRuntimeCameraStateAppliesPunchAnglesOutsideIntermission(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 11 || camera.Angles.Y != 18 || camera.Angles.Z != 33 {
		t.Fatalf("runtimeCameraState angles = %v, want {11 18 33}", camera.Angles)
	}
}

func TestRuntimeCameraStateSkipsPunchAnglesDuringIntermission(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.Intermission = 1
	gameClient.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 10 || camera.Angles.Y != 20 || camera.Angles.Z != 30 {
		t.Fatalf("runtimeCameraState angles = %v, want {10 20 30}", camera.Angles)
	}
}

func TestRuntimeViewStateInterpolatesViewAngles(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.ViewHeight = 22
	gameClient.PredictedOrigin = [3]float32{32, 64, 96}
	gameClient.MViewAngles[1] = [3]float32{0, 0, 0}
	gameClient.MViewAngles[0] = [3]float32{10, 20, 30}
	gameClient.MTime[1] = 1.0
	gameClient.MTime[0] = 1.1
	gameClient.Time = 1.05

	_, angles := runtimeViewState()
	if angles != [3]float32{5, 10, 15} {
		t.Fatalf("runtimeViewState angles = %v, want [5 10 15]", angles)
	}
}

func TestRuntimeCameraStateInterpolatesPunchAngles(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.Intermission = 0
	gameClient.PunchAngles[1] = [3]float32{0, 0, 0}
	gameClient.PunchAngles[0] = [3]float32{10, 0, 0}
	gameClient.PunchTime = 1.0
	gameClient.Time = 1.05

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X < 5.9 || camera.Angles.X > 6.1 {
		t.Fatalf("runtimeCameraState punch interpolation = %v, want ~6", camera.Angles.X)
	}
}

func TestRuntimeCameraStateGunKickModeRaw(t *testing.T) {
	originalClient := gameClient
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		gameClient = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "1")
	gameClient = cl.NewClient()
	gameClient.Intermission = 0
	gameClient.PunchAngle = [3]float32{2, -4, 6}
	gameClient.PunchAngles[1] = [3]float32{0, 0, 0}
	gameClient.PunchAngles[0] = [3]float32{10, 0, 0}
	gameClient.PunchTime = 1.0
	gameClient.Time = 1.05

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 3 || camera.Angles.Y != -2 || camera.Angles.Z != 9 {
		t.Fatalf("runtimeCameraState raw punch = %v, want {3 -2 9}", camera.Angles)
	}
}

func TestRuntimeCameraStateGunKickModeOff(t *testing.T) {
	originalClient := gameClient
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		gameClient = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "0")
	gameClient = cl.NewClient()
	gameClient.Intermission = 0
	gameClient.PunchAngle = [3]float32{2, -4, 6}

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 1 || camera.Angles.Y != 2 || camera.Angles.Z != 3 {
		t.Fatalf("runtimeCameraState with gunkick off = %v, want {1 2 3}", camera.Angles)
	}
}

func TestRuntimeViewStateInterpolatesYawAcrossWrap(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.ViewHeight = 22
	gameClient.PredictedOrigin = [3]float32{32, 64, 96}
	gameClient.MViewAngles[1] = [3]float32{0, 350, 0}
	gameClient.MViewAngles[0] = [3]float32{0, 10, 0}
	gameClient.MTime[1] = 1.0
	gameClient.MTime[0] = 1.1
	gameClient.Time = 1.05

	_, angles := runtimeViewState()
	if math.Abs(float64(angles[1]-360)) > 0.01 && math.Abs(float64(angles[1])) > 0.01 {
		t.Fatalf("runtimeViewState wrapped yaw = %v, want 0/360 short-path interpolation", angles[1])
	}
}

func TestCollectViewModelEntityAnchorsToEyeOrigin(t *testing.T) {
	originalClient := gameClient
	originalMenu := gameMenu
	originalSubs := gameSubs
	originalAliasCache := aliasModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameMenu = originalMenu
		gameSubs = originalSubs
		aliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/v_axe.mdl"}
	gameClient.Stats[0] = 100
	gameClient.Stats[2] = 1
	gameClient.Stats[5] = 1
	gameClient.ViewAngles = [3]float32{12, 34, 0}
	gameClient.ViewHeight = 28
	gameClient.PredictedOrigin = [3]float32{100, 200, 300}
	gameMenu = menu.NewManager(nil, nil)
	gameSubs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	aliasModelCache = map[string]*model.Model{
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
	if entity.Angles != [3]float32{-12, 34, 0} {
		t.Fatalf("viewmodel angles = %v, want [-12 34 0]", entity.Angles)
	}
	if entity.Frame != 1 {
		t.Fatalf("viewmodel frame = %d, want 1", entity.Frame)
	}
}

func TestCollectViewModelEntitySuppressesIntermission(t *testing.T) {
	originalClient := gameClient
	originalMenu := gameMenu
	originalSubs := gameSubs
	originalAliasCache := aliasModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameMenu = originalMenu
		gameSubs = originalSubs
		aliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	gameClient = cl.NewClient()
	gameClient.Intermission = 1
	gameClient.ModelPrecache = []string{"progs/v_axe.mdl"}
	gameClient.Stats[2] = 1
	gameClient.Stats[0] = 100
	gameMenu = menu.NewManager(nil, nil)
	gameSubs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	aliasModelCache = map[string]*model.Model{
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
	originalClient := gameClient
	originalMenu := gameMenu
	originalSubs := gameSubs
	originalAliasCache := aliasModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameMenu = originalMenu
		gameSubs = originalSubs
		aliasModelCache = originalAliasCache
		cvar.Set("r_drawviewmodel", "1")
	})

	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/v_axe.mdl"}
	gameClient.Stats[2] = 1
	gameClient.Stats[0] = 100
	gameMenu = menu.NewManager(nil, nil)
	gameSubs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	aliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

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
	originalClient := gameClient
	originalMenu := gameMenu
	originalSubs := gameSubs
	originalAliasCache := aliasModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameMenu = originalMenu
		gameSubs = originalSubs
		aliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/v_axe.mdl"}
	gameClient.Stats[2] = 1
	gameClient.Stats[0] = 100
	gameClient.Items = cl.ItemInvisibility
	gameMenu = menu.NewManager(nil, nil)
	gameSubs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	aliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when invisibility is active", entity)
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
	originalHost := gameHost
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
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

	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	gameHost.CmdPlaydemo("single_step", gameSubs)

	demo := gameHost.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after second host frame = %d, want 2", demo.FrameIndex)
	}
}

func TestPausedDemoPlaybackDoesNotReadFrames(t *testing.T) {
	originalHost := gameHost
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
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

	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	gameHost.CmdPlaydemo("paused", gameSubs)

	demo := gameHost.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.Paused = true

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}
	if demo.FrameIndex != 0 {
		t.Fatalf("frame index while paused = %d, want 0", demo.FrameIndex)
	}
}

func TestDemoPlaybackWaitsForRecordedServerTime(t *testing.T) {
	originalHost := gameHost
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
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

	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	gameHost.CmdPlaydemo("timed", gameSubs)

	clientState := host.LoopbackClientState(gameSubs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}

	demo := gameHost.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index before recorded time elapses = %d, want 1", demo.FrameIndex)
	}

	for i := 0; i < 6; i++ {
		if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
			t.Fatalf("Host.Frame catch-up %d: %v", i, err)
		}
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after recorded time elapses = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackTimeDemoIgnoresRecordedServerTime(t *testing.T) {
	originalHost := gameHost
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
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

	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	gameHost.CmdTimedemo("timedemo", gameSubs)

	clientState := host.LoopbackClientState(gameSubs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	demo := gameHost.DemoState()
	if demo == nil || !demo.Playback || !demo.TimeDemo {
		t.Fatal("expected active timedemo playback")
	}

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after timedemo frames = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackFlushesStuffTextSameFrame(t *testing.T) {
	originalHost := gameHost
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
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
	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	gameHost.CmdPlaydemo("stuffcmd", gameSubs)

	if err := gameHost.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if cmd.executes < 2 {
		t.Fatalf("executes = %d, want at least 2", cmd.executes)
	}
	clientState := host.LoopbackClientState(gameSubs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after same-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientFlushesLiveStuffTextSameFrame(t *testing.T) {
	originalHost := gameHost
	originalSubs := gameSubs
	t.Cleanup(func() {
		gameHost = originalHost
		gameSubs = originalSubs
	})

	cmd := &demoPlaybackCommandBuffer{}
	gameHost = host.NewHost()
	gameSubs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	tmpDir := t.TempDir()
	if err := gameHost.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, gameSubs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}

	clientState := host.LoopbackClientState(gameSubs)
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
	originalHost := gameHost
	originalClient := gameClient
	originalSubs := gameSubs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
		gameSubs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	gameHost = host.NewHost()
	demo := cl.NewDemoState()
	if err := demo.StartDemoRecording("runtime_demo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	t.Cleanup(func() {
		_ = demo.StopRecording()
	})
	gameHost.SetDemoState(demo)

	gameClient = cl.NewClient()
	gameClient.ViewAngles = [3]float32{10, 20, 30}
	gameSubs = &host.Subsystems{Client: &demoMessageClient{message: []byte{1, 2, 3}}}

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
	originalClient := gameClient
	originalMap := soundSFXByIndex
	originalKey := soundPrecacheKey
	t.Cleanup(func() {
		gameClient = originalClient
		soundSFXByIndex = originalMap
		soundPrecacheKey = originalKey
	})

	gameClient = cl.NewClient()
	gameClient.SoundPrecache = []string{"weapons/rocket1.wav"}
	soundPrecacheKey = "weapons/rocket1.wav"
	soundSFXByIndex = map[int]*audio.SFX{1: nil}

	refreshRuntimeSoundCache()
	if got := len(soundSFXByIndex); got != 1 {
		t.Fatalf("same precache unexpectedly reset cache; len = %d, want 1", got)
	}

	gameClient.SoundPrecache = []string{"weapons/shotgn2.wav"}
	refreshRuntimeSoundCache()
	if got := len(soundSFXByIndex); got != 0 {
		t.Fatalf("changed precache should reset cache; len = %d, want 0", got)
	}
}

func TestSyncRuntimeStaticSoundsTracksClientStateAndSnapshotChanges(t *testing.T) {
	originalClient := gameClient
	originalAudio := gameAudio
	originalSubs := gameSubs
	originalMap := soundSFXByIndex
	originalPrecacheKey := soundPrecacheKey
	originalStaticKey := staticSoundKey
	t.Cleanup(func() {
		gameClient = originalClient
		gameAudio = originalAudio
		gameSubs = originalSubs
		soundSFXByIndex = originalMap
		soundPrecacheKey = originalPrecacheKey
		staticSoundKey = originalStaticKey
	})

	gameSubs = nil
	gameAudio = audio.NewAudioAdapter(nil)
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.SoundPrecache = []string{"ambience/drip.wav"}
	gameClient.StaticSounds = []cl.StaticSound{
		{Origin: [3]float32{10, 20, 30}, SoundIndex: 1, Volume: 255, Attenuation: 1},
	}

	syncRuntimeStaticSounds()
	firstKey := staticSoundKey
	if firstKey == "" {
		t.Fatalf("expected static sound snapshot key to be populated")
	}

	syncRuntimeStaticSounds()
	if staticSoundKey != firstKey {
		t.Fatalf("unchanged snapshot should not churn static key; got %q, want %q", staticSoundKey, firstKey)
	}

	gameClient.StaticSounds = append(gameClient.StaticSounds, cl.StaticSound{
		Origin: [3]float32{40, 50, 60}, SoundIndex: 2, Volume: 200, Attenuation: 0.5,
	})
	syncRuntimeStaticSounds()
	secondKey := staticSoundKey
	if secondKey == firstKey {
		t.Fatalf("static sound list change should rebuild snapshot key")
	}

	soundSFXByIndex = map[int]*audio.SFX{1: nil}
	gameClient.SoundPrecache = []string{"ambience/wind2.wav"}
	syncRuntimeStaticSounds()
	if got := len(soundSFXByIndex); got != 0 {
		t.Fatalf("precache change should reset runtime SFX cache before static sync; len = %d, want 0", got)
	}
	if staticSoundKey == secondKey {
		t.Fatalf("precache change should rebuild static snapshot key")
	}

	gameClient.State = cl.StateConnected
	syncRuntimeStaticSounds()
	if staticSoundKey != "" {
		t.Fatalf("non-active client state should clear static snapshot key, got %q", staticSoundKey)
	}
}

func TestSyncRuntimeVisualEffectsEmitsParticlesAndDecals(t *testing.T) {
	originalClient := gameClient
	originalRenderer := gameRenderer
	originalParticles := gameParticles
	originalMarks := gameDecalMarks
	originalRNG := particleRNG
	originalTime := particleTime
	t.Cleanup(func() {
		gameClient = originalClient
		gameRenderer = originalRenderer
		gameParticles = originalParticles
		gameDecalMarks = originalMarks
		particleRNG = originalRNG
		particleTime = originalTime
	})

	gameRenderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ParticleEvents = []cl.ParticleEvent{
		{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 99},
	}
	gameClient.TempEntities = []cl.TempEntityEvent{
		{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}},
	}

	transientEvents := gameClient.ConsumeTransientEvents()
	syncRuntimeVisualEffects(0.1, transientEvents)

	if gameParticles == nil || gameParticles.ActiveCount() == 0 {
		t.Fatalf("expected runtime visual sync to emit particles")
	}
	gotMarks := 0
	if gameDecalMarks != nil {
		gotMarks = gameDecalMarks.ActiveCount()
	}
	if gotMarks != 1 {
		t.Fatalf("expected runtime visual sync to emit one decal mark, got %d", gotMarks)
	}
	if got := particleTime; got <= 0 {
		t.Fatalf("particleTime = %v, want > 0", got)
	}
	if len(gameClient.ParticleEvents) != 0 || len(gameClient.TempEntities) != 0 {
		t.Fatalf("runtime visual sync should consume client effect buffers")
	}
}

func TestSyncRuntimeVisualEffectsEmitsBrightFieldParticles(t *testing.T) {
	originalClient := gameClient
	originalRenderer := gameRenderer
	originalParticles := gameParticles
	originalMarks := gameDecalMarks
	originalRNG := particleRNG
	originalTime := particleTime
	t.Cleanup(func() {
		gameClient = originalClient
		gameRenderer = originalRenderer
		gameParticles = originalParticles
		gameDecalMarks = originalMarks
		particleRNG = originalRNG
		particleTime = originalTime
	})

	gameRenderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameClient.ModelPrecache = []string{"progs/player.mdl"}
	gameClient.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTFIELD},
	}

	syncRuntimeVisualEffects(0.1, cl.TransientEvents{})

	if gameParticles == nil {
		t.Fatalf("expected runtime visual sync to keep particle system initialized")
	}
	if got := gameParticles.ActiveCount(); got != 162 {
		t.Fatalf("brightfield particle count = %d, want 162", got)
	}
}

func TestSyncRuntimeVisualEffectsResetsEffectsWhenClientInactive(t *testing.T) {
	originalClient := gameClient
	originalRenderer := gameRenderer
	originalParticles := gameParticles
	originalMarks := gameDecalMarks
	originalRNG := particleRNG
	originalTime := particleTime
	t.Cleanup(func() {
		gameClient = originalClient
		gameRenderer = originalRenderer
		gameParticles = originalParticles
		gameDecalMarks = originalMarks
		particleRNG = originalRNG
		particleTime = originalTime
	})

	gameRenderer = &renderer.Renderer{}
	resetRuntimeVisualState()
	gameDecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{0, 0, 0},
		Normal: [3]float32{0, 0, 1},
		Size:   8,
		Alpha:  1,
	}, 5, 0)
	gameClient = cl.NewClient()
	gameClient.State = cl.StateConnected
	gameClient.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{1, 1, 1}}}

	transientEvents := gameClient.ConsumeTransientEvents()
	syncRuntimeVisualEffects(0.1, transientEvents)

	gotMarks := 0
	if gameDecalMarks != nil {
		gotMarks = gameDecalMarks.ActiveCount()
	}
	if gotMarks != 0 {
		t.Fatalf("inactive client should clear runtime decal marks")
	}
	if gameParticles == nil {
		t.Fatalf("inactive client reset should leave runtime particle system initialized")
	}
	if len(gameClient.TempEntities) != 0 {
		t.Fatalf("inactive client should consume queued temp entities")
	}
}

func TestBuildRuntimeRenderFrameStateIncludesDecalMarks(t *testing.T) {
	originalClient := gameClient
	originalMenu := gameMenu
	originalDraw := gameDraw
	originalRenderer := gameRenderer
	originalParticles := gameParticles
	originalMarks := gameDecalMarks
	t.Cleanup(func() {
		gameClient = originalClient
		gameMenu = originalMenu
		gameDraw = originalDraw
		gameRenderer = originalRenderer
		gameParticles = originalParticles
		gameDecalMarks = originalMarks
	})

	gameRenderer = &renderer.Renderer{}
	gameClient = cl.NewClient()
	gameClient.FogDensity = 128
	gameClient.FogColor = [3]byte{64, 128, 255}
	gameMenu = nil
	gameDraw = nil
	gameParticles = renderer.NewParticleSystem(renderer.MaxParticles)
	gameDecalMarks = renderer.NewDecalMarkSystem()
	gameDecalMarks.AddMark(renderer.DecalMarkEntity{
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
	originalClient := gameClient
	originalSubs := gameSubs
	originalCache := spriteModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameSubs = originalSubs
		spriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSprite(t, 1, 1),
		},
	}
	gameSubs = &host.Subsystems{Files: testFS}
	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/flame.spr"}
	gameClient.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Origin: [3]float32{7, 8, 9}, Angles: [3]float32{10, 20, 30}, Alpha: 128, Scale: 32},
	}
	spriteModelCache = nil

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
	originalClient := gameClient
	originalSubs := gameSubs
	originalCache := spriteModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameSubs = originalSubs
		spriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSpriteGroup(t, 2, []float32{0.2, 0.4}),
		},
	}
	gameSubs = &host.Subsystems{Files: testFS}
	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/flame.spr"}
	gameClient.Time = 0.25
	gameClient.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0},
	}
	spriteModelCache = nil

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 1 {
		t.Fatalf("collectSpriteEntities grouped frame = %d, want 1", got)
	}
}

func TestCollectSpriteEntitiesResolvesAngledFrameFromViewAngles(t *testing.T) {
	originalClient := gameClient
	originalSubs := gameSubs
	originalCache := spriteModelCache
	t.Cleanup(func() {
		gameClient = originalClient
		gameSubs = originalSubs
		spriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeAngledSprite(t),
		},
	}
	gameSubs = &host.Subsystems{Files: testFS}
	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"progs/flame.spr"}
	gameClient.ViewAngles = [3]float32{0, 90, 0}
	gameClient.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Angles: [3]float32{0, 0, 0}},
	}
	spriteModelCache = nil

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
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{
		"progs/player.mdl",
		"*1",
		"progs/flame.spr",
	}
	gameClient.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{1, 2, 3}, Angles: [3]float32{0, 90, 0}, Effects: inet.EF_MUZZLEFLASH},
		2: {ModelIndex: 2, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTLIGHT},
		3: {ModelIndex: 3, Origin: [3]float32{7, 8, 9}, Effects: inet.EF_DIMLIGHT},
		4: {ModelIndex: 1, Origin: [3]float32{9, 9, 9}},
	}
	gameClient.StaticEntities = []inet.EntityState{
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
	originalClient := gameClient
	originalServer := gameServer
	t.Cleanup(func() {
		gameClient = originalClient
		gameServer = originalServer
	})

	gameClient = cl.NewClient()
	gameClient.ModelPrecache = []string{"maps/start.bsp", "*1"}
	gameClient.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 2,
			Frame:      3,
			Origin:     [3]float32{1, 2, 3},
			Angles:     [3]float32{10, 20, 30},
			Alpha:      128,
			Scale:      32,
		},
	}
	gameServer = &server.Server{WorldTree: &bsp.Tree{Models: []bsp.DModel{{}, {}}}}

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
	originalHUD := gameHUD
	originalClient := gameClient
	originalServer := gameServer
	originalShowScores := gameShowScores
	t.Cleanup(func() {
		gameHUD = originalHUD
		gameClient = originalClient
		gameServer = originalServer
		gameShowScores = originalShowScores
	})

	gameHUD = hud.NewHUD(nil)
	gameClient = cl.NewClient()
	gameClient.Stats[inet.StatHealth] = 111
	gameClient.Stats[inet.StatArmor] = 55
	gameClient.Stats[inet.StatAmmo] = 22
	gameClient.Stats[inet.StatWeapon] = 7
	gameClient.Stats[inet.StatActiveWeapon] = cl.ItemRocketLauncher
	gameClient.Stats[inet.StatShells] = 10
	gameClient.Stats[inet.StatNails] = 20
	gameClient.Stats[inet.StatRockets] = 30
	gameClient.Stats[inet.StatCells] = 40
	gameClient.Stats[inet.StatTotalSecrets] = 9
	gameClient.Stats[inet.StatTotalMonsters] = 66
	gameClient.Stats[inet.StatSecrets] = 3
	gameClient.Stats[inet.StatMonsters] = 12
	gameClient.MaxClients = 4
	gameClient.GameType = 1
	gameClient.ViewEntity = 2
	gameClient.PlayerNames[0] = "alpha"
	gameClient.PlayerNames[1] = "bravo"
	gameClient.PlayerNames[2] = "charlie"
	gameClient.PlayerColors[0] = 0x1f
	gameClient.PlayerColors[1] = 0x2e
	gameClient.PlayerColors[2] = 0x3d
	gameClient.Frags[0] = 4
	gameClient.Frags[1] = 10
	gameClient.Frags[2] = 6
	gameClient.Items = cl.ItemRocketLauncher | cl.ItemRockets | cl.ItemArmor2 | cl.ItemQuad
	gameClient.Intermission = 2
	gameClient.CompletedTime = 123
	gameClient.Time = 124
	gameClient.CenterPrint = "The End"
	gameClient.CenterPrintAt = 120
	gameClient.LevelName = "Unit Test Map"
	gameShowScores = true

	updateHUDFromServer()

	got := gameHUD.State()
	if got.Health != 111 || got.Armor != 55 || got.Ammo != 22 {
		t.Fatalf("hud core stats = %#v, want health=111 armor=55 ammo=22", got)
	}
	if got.WeaponModel != 7 || got.ActiveWeapon != cl.ItemRocketLauncher {
		t.Fatalf("hud weapon state = %#v, want model=7 active=%d", got, cl.ItemRocketLauncher)
	}
	if got.Shells != 10 || got.Nails != 20 || got.Rockets != 30 || got.Cells != 40 {
		t.Fatalf("hud ammo strip = %#v, want [10 20 30 40]", got)
	}
	if got.Items != gameClient.Items {
		t.Fatalf("hud items = %#x, want %#x", got.Items, gameClient.Items)
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
	originalInput := gameInput
	t.Cleanup(func() {
		gameInput = originalInput
	})

	gameInput = input.NewSystem(nil)
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
		if got := gameInput.GetBinding(tc.key); got != tc.want {
			t.Fatalf("binding for key %d = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestGameplayBindCommandsAndDispatch(t *testing.T) {
	originalInput := gameInput
	originalClient := gameClient
	t.Cleanup(func() {
		gameInput = originalInput
		gameClient = originalClient
	})

	gameInput = input.NewSystem(nil)
	gameInput.SetKeyDest(input.KeyGame)
	gameClient = cl.NewClient()
	registerGameplayBindCommands()

	cmdsys.ExecuteText("unbindall")
	cmdsys.ExecuteText("bind w +forward")
	cmdsys.ExecuteText("bind MWHEELUP \"impulse 12\"")

	if got := gameInput.GetBinding(int('w')); got != "+forward" {
		t.Fatalf("bind command did not set w binding, got %q", got)
	}
	if got := gameInput.GetBinding(input.KMWheelUp); got != "impulse 12" {
		t.Fatalf("bind command did not set MWHEELUP binding, got %q", got)
	}

	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	if gameClient.InputForward.State&1 == 0 {
		t.Fatalf("expected +forward to press InputForward")
	}
	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: false})
	if gameClient.InputForward.State&1 != 0 {
		t.Fatalf("expected -forward to release InputForward")
	}

	handleGameKeyEvent(input.KeyEvent{Key: input.KMWheelUp, Down: true})
	if gameClient.InImpulse != 12 {
		t.Fatalf("expected wheel bind to set impulse 12, got %d", gameClient.InImpulse)
	}

	cmdsys.ExecuteText("unbind w")
	if got := gameInput.GetBinding(int('w')); got != "" {
		t.Fatalf("unbind did not clear w binding, got %q", got)
	}

	cmdsys.ExecuteText("unbindall")
	if got := gameInput.GetBinding(input.KMWheelUp); got != "" {
		t.Fatalf("unbindall did not clear MWHEELUP binding, got %q", got)
	}
}

func TestSyncGameplayInputModeClearsHeldScoreboardOutsideGameInput(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	originalClient := gameClient
	originalShowScores := gameShowScores
	originalGrabbed := gameMouseGrabbed
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
		gameClient = originalClient
		gameShowScores = originalShowScores
		gameMouseGrabbed = originalGrabbed
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameClient = cl.NewClient()
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	gameInput.SetKeyDest(input.KeyGame)
	gameMouseGrabbed = false
	syncGameplayInputMode()

	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})
	if !gameShowScores {
		t.Fatalf("+showscores should set held scoreboard state")
	}

	gameInput.SetKeyDest(input.KeyConsole)
	syncGameplayInputMode()
	if gameShowScores {
		t.Fatalf("scoreboard hold should clear when leaving gameplay input")
	}
}

func TestStartupMenuStateSuppressesGameplayMovementInput(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	originalClient := gameClient
	originalGrabbed := gameMouseGrabbed
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
		gameClient = originalClient
		gameMouseGrabbed = originalGrabbed
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameClient = cl.NewClient()
	gameMouseGrabbed = false

	gameInput.OnMenuKey = handleMenuKeyEvent
	gameInput.OnMenuChar = handleMenuCharEvent
	gameInput.OnKey = handleGameKeyEvent
	gameInput.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	// initSubsystems shows the menu at startup; +map start does not close it.
	gameMenu.ShowMenu()
	syncGameplayInputMode()
	if got := gameInput.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination with startup menu active = %v, want menu", got)
	}

	gameInput.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if gameClient.InputForward.State&1 != 0 {
		t.Fatalf("+forward should not activate while key destination is menu")
	}
	if gameClient.InputJump.State&1 != 0 {
		t.Fatalf("+jump should not activate while key destination is menu")
	}

	gameMenu.HideMenu()
	syncGameplayInputMode()
	gameInput.ClearKeyStates()

	gameInput.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if gameClient.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after menu closes")
	}
	if gameClient.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after menu closes")
	}
}

func TestApplyStartupGameplayInputModeHidesMenuAndEnablesMovementInput(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	originalClient := gameClient
	originalGrabbed := gameMouseGrabbed
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
		gameClient = originalClient
		gameMouseGrabbed = originalGrabbed
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameClient = cl.NewClient()
	gameClient.State = cl.StateActive
	gameMouseGrabbed = false

	gameInput.OnMenuKey = handleMenuKeyEvent
	gameInput.OnMenuChar = handleMenuCharEvent
	gameInput.OnKey = handleGameKeyEvent
	gameInput.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	gameMenu.ShowMenu()
	syncGameplayInputMode()
	if got := gameInput.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination before startup transition = %v, want menu", got)
	}

	applyStartupGameplayInputMode()
	if gameMenu.IsActive() {
		t.Fatalf("startup transition should hide menu")
	}
	if got := gameInput.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after startup transition = %v, want game", got)
	}

	gameInput.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if gameClient.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after startup transition")
	}
	if gameClient.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after startup transition")
	}
}

func TestHostInitLoadsBindingOverridesFromConfig(t *testing.T) {
	originalInput := gameInput
	t.Cleanup(func() {
		gameInput = originalInput
	})

	gameInput = input.NewSystem(nil)
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
		Input:    gameInput,
	}
	if err := h.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if got := gameInput.GetBinding(int('w')); got != "+back" {
		t.Fatalf("binding for w after config load = %q, want %q", got, "+back")
	}
	if got := gameInput.GetBinding(input.KF10); got != "+attack" {
		t.Fatalf("binding for F10 after config load = %q, want %q", got, "+attack")
	}
}

func TestQuotedBindingsRoundTripThroughConfig(t *testing.T) {
	originalInput := gameInput
	t.Cleanup(func() {
		gameInput = originalInput
	})

	userDir := t.TempDir()
	gameInput = input.NewSystem(nil)
	registerGameplayBindCommands()

	writerHost := host.NewHost()
	writerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    gameInput,
	}
	if err := writerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, writerSubs); err != nil {
		t.Fatalf("writer Init failed: %v", err)
	}

	want := "say He said \"hello\" \\world\nnext\tline"
	cmdsys.ExecuteText(`bind t "say He said \"hello\" \\world\nnext\tline"`)
	if got := gameInput.GetBinding(int('t')); got != want {
		t.Fatalf("binding before save = %q, want %q", got, want)
	}
	if err := writerHost.WriteConfig(writerSubs); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	gameInput = input.NewSystem(nil)
	registerGameplayBindCommands()
	readerHost := host.NewHost()
	readerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    gameInput,
	}
	if err := readerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, readerSubs); err != nil {
		t.Fatalf("reader Init failed: %v", err)
	}

	if got := gameInput.GetBinding(int('t')); got != want {
		t.Fatalf("binding after reload = %q, want %q", got, want)
	}
}

func TestSyncControlCvarsToClient(t *testing.T) {
	originalClient := gameClient
	t.Cleanup(func() {
		gameClient = originalClient
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	gameClient = cl.NewClient()
	syncControlCvarsToClient()

	if gameClient.AlwaysRun {
		t.Fatalf("AlwaysRun should follow cl_alwaysrun")
	}
	if gameClient.FreeLook {
		t.Fatalf("FreeLook should follow freelook")
	}
	if !gameClient.LookSpring {
		t.Fatalf("LookSpring should follow lookspring")
	}
}

func TestSyncHostClientStateReappliesControlCvarsOnClientReplacement(t *testing.T) {
	originalClient := gameClient
	originalSubs := gameSubs
	t.Cleanup(func() {
		gameClient = originalClient
		gameSubs = originalSubs
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	firstClient := cl.NewClient()
	gameSubs = &host.Subsystems{
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
	gameSubs.Client = &activeStateTestClient{
		state:       host.ClientState(1),
		clientState: replacedClient,
	}
	syncHostClientState()

	if replacedClient.AlwaysRun || replacedClient.FreeLook || !replacedClient.LookSpring {
		t.Fatalf("replaced client controls = %+v, want alwaysrun=false freelook=false lookspring=true", replacedClient)
	}
}

func TestApplyGameplayMouseLookUsesControlCvars(t *testing.T) {
	originalInput := gameInput
	originalClient := gameClient
	t.Cleanup(func() {
		gameInput = originalInput
		gameClient = originalClient
	})

	registerControlCvars()
	backend := &mouseDeltaBackend{}
	gameInput = input.NewSystem(backend)
	gameInput.SetKeyDest(input.KeyGame)
	gameClient = cl.NewClient()

	cvar.Set("sensitivity", "10")
	cvar.Set("m_yaw", "0.01")
	cvar.Set("m_pitch", "0.02")
	cvar.Set("freelook", "1")

	backend.dx = 2
	backend.dy = 3
	applyGameplayMouseLook()
	if got := gameClient.ViewAngles[1]; math.Abs(float64(got-(-0.2))) > 0.0001 {
		t.Fatalf("yaw after mouse look = %.2f, want -0.20", got)
	}
	if got := gameClient.ViewAngles[0]; math.Abs(float64(got-0.6)) > 0.0001 {
		t.Fatalf("pitch after mouse look = %.2f, want 0.60", got)
	}

	gameClient.ViewAngles = [3]float32{}
	cvar.Set("freelook", "0")
	backend.dx = 0
	backend.dy = 5
	applyGameplayMouseLook()
	if got := gameClient.ViewAngles[0]; got != 0 {
		t.Fatalf("pitch should stay unchanged when freelook is off and +mlook inactive, got %.2f", got)
	}

	gameClient.InputMLook.State = 1
	backend.dy = 5
	applyGameplayMouseLook()
	if got := gameClient.ViewAngles[0]; math.Abs(float64(got-1.0)) > 0.0001 {
		t.Fatalf("pitch with +mlook held = %.2f, want 1.00", got)
	}

	gameClient.ViewAngles = [3]float32{}
	gameClient.InputMLook.State = 0
	cvar.Set("freelook", "1")
	cvar.Set("m_pitch", "-0.02")
	backend.dy = 5
	applyGameplayMouseLook()
	if got := gameClient.ViewAngles[0]; math.Abs(float64(got-(-1.0))) > 0.0001 {
		t.Fatalf("pitch with inverted mouse = %.2f, want -1.00", got)
	}
}

func TestToggleConsoleClosesMenuAndSwitchesKeyDest(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	originalGrabbed := gameMouseGrabbed
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
		gameMouseGrabbed = originalGrabbed
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameMenu.ShowMenu()
	gameMouseGrabbed = true

	cmdToggleConsole(nil)

	if gameMenu.IsActive() {
		t.Fatalf("toggleconsole should hide the menu")
	}
	if got := gameInput.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after toggleconsole = %v, want console", got)
	}
	if gameMouseGrabbed {
		t.Fatalf("console mode should release mouse grab")
	}

	cmdToggleConsole(nil)
	if got := gameInput.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestMenuTapDownMovesCursorOnce(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameMenu.ShowMenu()
	gameInput.SetKeyDest(input.KeyMenu)
	gameInput.OnMenuKey = handleMenuKeyEvent

	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := gameMenu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after down+enter tap = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestMenuTapEscapeFromSubmenuReturnsToMain(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameMenu.ShowMenu()
	gameInput.SetKeyDest(input.KeyMenu)
	gameInput.OnMenuKey = handleMenuKeyEvent

	// Enter multiplayer menu.
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := gameMenu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after entering submenu = %v, want %v", got, menu.MenuMultiPlayer)
	}

	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: false})

	if !gameMenu.IsActive() {
		t.Fatalf("menu should remain active after escape tap from submenu")
	}
	if got := gameMenu.GetState(); got != menu.MenuMain {
		t.Fatalf("menu state after escape tap = %v, want %v", got, menu.MenuMain)
	}
}

func TestMenuCharRoutingUpdatesSetupName(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
	})

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameMenu.ShowMenu()
	gameInput.SetKeyDest(input.KeyMenu)
	gameInput.OnMenuKey = handleMenuKeyEvent
	gameInput.OnMenuChar = handleMenuCharEvent

	// Enter multiplayer -> setup.
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := gameMenu.GetState(); got != menu.MenuSetup {
		t.Fatalf("menu state = %v, want %v", got, menu.MenuSetup)
	}

	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // name
	gameInput.HandleCharEvent('x')
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // shirt
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // pants
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // accept
	gameInput.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := gameMenu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after accept = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestConsoleKeyRoutingExecutesCommands(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	originalGrabbed := gameMouseGrabbed
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
		gameMouseGrabbed = originalGrabbed
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	gameInput.SetKeyDest(input.KeyGame)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	var gotArgs []string
	cmdsys.AddCommand("testconsolecmd", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "test console command")

	handleGameKeyEvent(input.KeyEvent{Key: int('`'), Down: true})
	if got := gameInput.GetKeyDest(); got != input.KeyConsole {
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
	if got := gameInput.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestConsoleTabCompletionCompletesCommand(t *testing.T) {
	originalInput := gameInput
	originalMenu := gameMenu
	t.Cleanup(func() {
		gameInput = originalInput
		gameMenu = originalMenu
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	console.ResetCompletion()

	gameInput = input.NewSystem(nil)
	gameMenu = menu.NewManager(nil, gameInput)
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	gameInput.SetKeyDest(input.KeyConsole)

	for _, ch := range "tog" {
		handleGameCharEvent(ch)
	}
	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})

	if got := console.InputLine(); got != "toggleconsole" {
		t.Fatalf("console input line after tab completion = %q, want %q", got, "toggleconsole")
	}
}

func TestHandleGameKeyEventUnboundSpecialKeyFeedback(t *testing.T) {
	originalInput := gameInput
	originalHost := gameHost
	t.Cleanup(func() {
		gameInput = originalInput
		gameHost = originalHost
		console.SetPrintCallback(nil)
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	gameInput = input.NewSystem(nil)
	gameInput.SetKeyDest(input.KeyGame)

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
	gameInput.SetKeyDest(input.KeyMenu)
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("menu destination should not print unbound game hint, got %q", got)
	}

	printed.Reset()
	gameInput.SetKeyDest(input.KeyGame)
	gameHost = host.NewHost()
	gameHost.SetDemoState(&cl.DemoState{Playback: true})
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("demo playback should suppress unbound hint, got %q", got)
	}
}

func TestRuntimeMusicSelectionUsesDemoHeaderFallback(t *testing.T) {
	originalHost := gameHost
	originalClient := gameClient
	t.Cleanup(func() {
		gameHost = originalHost
		gameClient = originalClient
	})

	gameHost = host.NewHost()
	demo := cl.NewDemoState()
	demo.Playback = true
	demo.CDTrack = 5
	gameHost.SetDemoState(demo)
	gameClient = cl.NewClient()

	track, loopTrack := runtimeMusicSelection()
	if track != 5 || loopTrack != 5 {
		t.Fatalf("runtimeMusicSelection() = %d/%d, want 5/5", track, loopTrack)
	}

	gameClient.CDTrack = 2
	gameClient.LoopTrack = 3
	track, loopTrack = runtimeMusicSelection()
	if track != 2 || loopTrack != 3 {
		t.Fatalf("runtimeMusicSelection() with live client track = %d/%d, want 2/3", track, loopTrack)
	}
}

func TestSyncRuntimeMusicLoadsTrackOnceAndStops(t *testing.T) {
	originalAudio := gameAudio
	originalClient := gameClient
	originalHost := gameHost
	originalSubs := gameSubs
	originalKey := musicTrackKey
	t.Cleanup(func() {
		gameAudio = originalAudio
		gameClient = originalClient
		gameHost = originalHost
		gameSubs = originalSubs
		musicTrackKey = originalKey
	})

	sys := &audio.System{}
	sys = audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	gameAudio = audio.NewAudioAdapter(sys)
	gameClient = cl.NewClient()
	gameClient.CDTrack = 2
	gameClient.LoopTrack = 2
	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"music/track02.wav": testRuntimeMusicWAV(t, 44100, 2, 2, 64),
		},
	}
	gameSubs = &host.Subsystems{Files: testFS}

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

	gameClient.CDTrack = 0
	gameClient.LoopTrack = 0
	syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 0 {
		t.Fatalf("CurrentMusicTrack = %d, want 0 after stopping music", got)
	}
}

func TestApplySVolumeUsesCVarAndClamps(t *testing.T) {
	originalAudio := gameAudio
	t.Cleanup(func() {
		gameAudio = originalAudio
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	gameAudio = audio.NewAudioAdapter(sys)

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
