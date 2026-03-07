package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/audio"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

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

func TestRuntimeViewStateUsesPredictedClientView(t *testing.T) {
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
	gameClient.PredictedOrigin = [3]float32{64, 32, 16}
	gameClient.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := runtimeViewState()
	if origin != gameClient.PredictedOrigin {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, gameClient.PredictedOrigin)
	}
	if angles != gameClient.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, gameClient.ViewAngles)
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

	syncRuntimeVisualEffects(0.1)

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

	syncRuntimeVisualEffects(0.1)

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
		1: {ModelIndex: 1, Frame: 0, Origin: [3]float32{7, 8, 9}, Alpha: 128},
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
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after first collect = %d, want 1", got)
	}

	_ = collectSpriteEntities()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after cached collect = %d, want 1", got)
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
