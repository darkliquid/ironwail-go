// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"reflect"
	"testing"

	"github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/server"
)

type mockSubsystems struct {
	Subsystems
	server  *mockServer
	client  *mockClient
	console *mockConsole
}

type mockServer struct {
	active  bool
	paused  bool
	mapName string
	spawned []string
}

func (m *mockServer) Init(maxClients int) error     { m.active = true; return nil }
func (m *mockServer) Frame(frameTime float64) error { return nil }
func (m *mockServer) Shutdown()                     { m.active = false }
func (m *mockServer) IsActive() bool                { return m.active }
func (m *mockServer) IsPaused() bool                { return m.paused }
func (m *mockServer) SetLoadGame(v bool)            {}
func (m *mockServer) SetPreserveSpawnParms(v bool)  {}
func (m *mockServer) SpawnServer(mapName string, vfs *fs.FileSystem) error {
	m.mapName = mapName
	m.spawned = append(m.spawned, mapName)
	return nil
}
func (m *mockServer) ConnectClient(clientNum int)                       {}
func (m *mockServer) KillClient(clientNum int) bool                     { return false }
func (m *mockServer) KickClient(clientNum int, who, reason string) bool { return false }
func (m *mockServer) SaveSpawnParms()                                   {}
func (m *mockServer) GetMaxClients() int                                { return 1 }
func (m *mockServer) IsClientActive(clientNum int) bool                 { return clientNum == 0 }
func (m *mockServer) GetClientName(clientNum int) string                { return "Player" }
func (m *mockServer) SetClientName(clientNum int, name string)          {}
func (m *mockServer) GetClientColor(clientNum int) int                  { return 0 }
func (m *mockServer) SetClientColor(clientNum int, color int)           {}
func (m *mockServer) GetClientPing(clientNum int) float32               { return 0 }
func (m *mockServer) EdictNum(n int) *server.Edict                      { return &server.Edict{Vars: &server.EntVars{}} }
func (m *mockServer) GetMapName() string {
	if m.mapName != "" {
		return m.mapName
	}
	return "start"
}
func (m *mockServer) RestoreTextSaveGameState(state *server.TextSaveGameState) error {
	return nil
}

type mockClient struct {
	state ClientState
}

func (m *mockClient) Init() error                    { return nil }
func (m *mockClient) Frame(frameTime float64) error  { return nil }
func (m *mockClient) Shutdown()                      {}
func (m *mockClient) State() ClientState             { return m.state }
func (m *mockClient) ReadFromServer() error          { return nil }
func (m *mockClient) SendCommand() error             { return nil }
func (m *mockClient) SendStringCmd(cmd string) error { return nil }

type mockConsole struct {
	messages []string
}

func (m *mockConsole) Init() error                { return nil }
func (m *mockConsole) Print(msg string)           { m.messages = append(m.messages, msg) }
func (m *mockConsole) Clear()                     { m.messages = nil }
func (m *mockConsole) Dump(filename string) error { return nil }
func (m *mockConsole) Shutdown()                  {}

type shutdownRecorder struct {
	order []string
}

func (r *shutdownRecorder) record(name string) {
	r.order = append(r.order, name)
}

type shutdownTrackingFilesystem struct{ recorder *shutdownRecorder }

func (f *shutdownTrackingFilesystem) Init(baseDir, gameDir string) error { return nil }
func (f *shutdownTrackingFilesystem) Close()                             { f.recorder.record("files") }
func (f *shutdownTrackingFilesystem) LoadFile(filename string) ([]byte, error) {
	return nil, nil
}
func (f *shutdownTrackingFilesystem) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	return "", nil, nil
}
func (f *shutdownTrackingFilesystem) FileExists(filename string) bool { return false }

type shutdownTrackingCommands struct{ recorder *shutdownRecorder }

func (c *shutdownTrackingCommands) Init()                                         {}
func (c *shutdownTrackingCommands) Execute()                                      {}
func (c *shutdownTrackingCommands) ExecuteWithSource(source cmdsys.CommandSource) {}
func (c *shutdownTrackingCommands) AddText(text string)                           {}
func (c *shutdownTrackingCommands) InsertText(text string)                        {}
func (c *shutdownTrackingCommands) Shutdown()                                     { c.recorder.record("commands") }

type shutdownTrackingConsole struct{ recorder *shutdownRecorder }

func (c *shutdownTrackingConsole) Init() error       { return nil }
func (c *shutdownTrackingConsole) Print(string)      {}
func (c *shutdownTrackingConsole) Clear()            {}
func (c *shutdownTrackingConsole) Dump(string) error { return nil }
func (c *shutdownTrackingConsole) Shutdown()         { c.recorder.record("console") }

type shutdownTrackingServer struct{ recorder *shutdownRecorder }

func (s *shutdownTrackingServer) Init(int) error                           { return nil }
func (s *shutdownTrackingServer) SpawnServer(string, *fs.FileSystem) error { return nil }
func (s *shutdownTrackingServer) ConnectClient(int)                        {}
func (s *shutdownTrackingServer) KillClient(int) bool                      { return false }
func (s *shutdownTrackingServer) KickClient(int, string, string) bool      { return false }
func (s *shutdownTrackingServer) Frame(float64) error                      { return nil }
func (s *shutdownTrackingServer) Shutdown()                                { s.recorder.record("server") }
func (s *shutdownTrackingServer) SaveSpawnParms()                          {}
func (s *shutdownTrackingServer) GetMaxClients() int                       { return 1 }
func (s *shutdownTrackingServer) IsClientActive(int) bool                  { return false }
func (s *shutdownTrackingServer) GetClientName(int) string                 { return "" }
func (s *shutdownTrackingServer) SetClientName(int, string)                {}
func (s *shutdownTrackingServer) GetClientColor(int) int                   { return 0 }
func (s *shutdownTrackingServer) SetClientColor(int, int)                  {}
func (s *shutdownTrackingServer) GetClientPing(int) float32                { return 0 }
func (s *shutdownTrackingServer) EdictNum(int) *server.Edict               { return nil }
func (s *shutdownTrackingServer) GetMapName() string                       { return "" }
func (s *shutdownTrackingServer) IsActive() bool                           { return false }
func (s *shutdownTrackingServer) IsPaused() bool                           { return false }
func (s *shutdownTrackingServer) RestoreTextSaveGameState(*server.TextSaveGameState) error {
	return nil
}
func (s *shutdownTrackingServer) SetLoadGame(bool)           {}
func (s *shutdownTrackingServer) SetPreserveSpawnParms(bool) {}

type shutdownTrackingClient struct{ recorder *shutdownRecorder }

func (c *shutdownTrackingClient) Init() error                { return nil }
func (c *shutdownTrackingClient) Frame(float64) error        { return nil }
func (c *shutdownTrackingClient) Shutdown()                  { c.recorder.record("client") }
func (c *shutdownTrackingClient) State() ClientState         { return caDisconnected }
func (c *shutdownTrackingClient) ReadFromServer() error      { return nil }
func (c *shutdownTrackingClient) SendCommand() error         { return nil }
func (c *shutdownTrackingClient) SendStringCmd(string) error { return nil }

type shutdownTrackingAudio struct{ recorder *shutdownRecorder }

func (a *shutdownTrackingAudio) Init() error { return nil }
func (a *shutdownTrackingAudio) Update(origin, velocity, forward, right, up [3]float32) {
}
func (a *shutdownTrackingAudio) StopAllSounds(clear bool) {}
func (a *shutdownTrackingAudio) SoundInfo() string        { return "" }
func (a *shutdownTrackingAudio) SoundList() string        { return "" }
func (a *shutdownTrackingAudio) PlayLocalSound(string, func() ([]byte, error), float32) error {
	return nil
}
func (a *shutdownTrackingAudio) PlayMusic(string, func(string) ([]byte, error), func([]string) (string, []byte, error)) error {
	return nil
}
func (a *shutdownTrackingAudio) PauseMusic()       {}
func (a *shutdownTrackingAudio) ResumeMusic()      {}
func (a *shutdownTrackingAudio) SetMusicLoop(bool) {}
func (a *shutdownTrackingAudio) ToggleMusicLoop() bool {
	return false
}
func (a *shutdownTrackingAudio) MusicLooping() bool   { return false }
func (a *shutdownTrackingAudio) CurrentMusic() string { return "" }
func (a *shutdownTrackingAudio) JumpMusic(int) bool   { return false }
func (a *shutdownTrackingAudio) StopMusic()           {}
func (a *shutdownTrackingAudio) Shutdown()            { a.recorder.record("audio") }

type shutdownTrackingRenderer struct{ recorder *shutdownRecorder }

func (r *shutdownTrackingRenderer) Init() error   { return nil }
func (r *shutdownTrackingRenderer) UpdateScreen() {}
func (r *shutdownTrackingRenderer) Shutdown()     { r.recorder.record("renderer") }

type shutdownTrackingInputBackend struct{ recorder *shutdownRecorder }

func (b *shutdownTrackingInputBackend) Init() error                   { return nil }
func (b *shutdownTrackingInputBackend) Shutdown()                     { b.recorder.record("input") }
func (b *shutdownTrackingInputBackend) PollEvents() bool              { return true }
func (b *shutdownTrackingInputBackend) GetMouseDelta() (int32, int32) { return 0, 0 }
func (b *shutdownTrackingInputBackend) GetMousePosition() (int32, int32, bool) {
	return 0, 0, false
}
func (b *shutdownTrackingInputBackend) GetModifierState() input.ModifierState {
	return input.ModifierState{}
}
func (b *shutdownTrackingInputBackend) SetTextMode(input.TextMode)     {}
func (b *shutdownTrackingInputBackend) SetCursorMode(input.CursorMode) {}
func (b *shutdownTrackingInputBackend) ShowKeyboard(bool)              {}
func (b *shutdownTrackingInputBackend) GetGamepadState(int) input.GamepadState {
	return input.GamepadState{}
}
func (b *shutdownTrackingInputBackend) IsGamepadConnected(int) bool { return false }
func (b *shutdownTrackingInputBackend) SetMouseGrab(bool)           {}
func (b *shutdownTrackingInputBackend) SetWindow(interface{})       {}

type mockCallbacks struct {
	serverCalled bool
	clientCalled bool
}

func (m *mockCallbacks) GetEvents()                                        {}
func (m *mockCallbacks) ProcessConsoleCommands()                           {}
func (m *mockCallbacks) ProcessServer()                                    { m.serverCalled = true }
func (m *mockCallbacks) ProcessClient()                                    { m.clientCalled = true }
func (m *mockCallbacks) UpdateScreen()                                     {}
func (m *mockCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {}

// TestHostInit verifies that the host initializes correctly with mock subsystems.
// Why: The Host is the central coordinator of the engine, and ensuring its
// initialization logic is sound is critical for overall stability.
// Where in C: host.c, Host_Init.
func TestHostInit(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	params := &InitParams{
		BaseDir:    ".",
		MaxClients: 1,
	}

	if err := h.Init(params, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !h.IsInitialized() {
		t.Errorf("Host not initialized")
	}
}

// TestHostInitRegistersDeathmatchRuleCVars verifies that the host correctly
// registers core deathmatch rules (fraglimit, timelimit, teamplay) as serverinfo.
// Why: These cvars are essential for multiplayer game rules and client-side HUD updates.
// Where in C: host.c, Host_Init.
func TestHostInitRegistersDeathmatchRuleCVars(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", MaxClients: 1}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, name := range []string{"fraglimit", "timelimit", "teamplay"} {
		cv := cvar.Get(name)
		if cv == nil {
			t.Fatalf("cvar %q not registered", name)
		}
		if cv.Flags&cvar.FlagServerInfo == 0 {
			t.Fatalf("cvar %q missing serverinfo flag", name)
		}
	}
}

// TestRegisterHostCVarsIncludesDebugTelemetryCVars verifies that the host registers
// its debug telemetry cvars for parity and troubleshooting.
// Why: Enables engine-side event logging and QuakeC tracing for parity investigations.
// Where in C: host.c, Host_Init (and Ironwail-specific extensions).
func TestRegisterHostCVarsIncludesDebugTelemetryCVars(t *testing.T) {
	registerHostCVars()

	for _, name := range []string{
		"sv_debug_telemetry",
		"sv_debug_telemetry_events",
		"sv_debug_telemetry_classname",
		"sv_debug_telemetry_entnum",
		"sv_debug_telemetry_summary",
		"sv_debug_qc_trace",
		"sv_debug_qc_trace_verbosity",
	} {
		if cv := cvar.Get(name); cv == nil {
			t.Fatalf("cvar %q not registered", name)
		}
	}
}

// TestRegisterHostCVarsIncludesAudioCVars verifies that the host registers
// core audio configuration cvars.
// Why: Allows the user to control volume, sampling rate, and other audio parameters.
// Where in C: host.c, Host_Init and snd_dma.c.
func TestRegisterHostCVarsIncludesAudioCVars(t *testing.T) {
	registerHostCVars()

	for _, name := range []string{
		"volume",
		"bgmvolume",
		"ambient_level",
		"ambient_fade",
		"sndspeed",
		"snd_mixspeed",
		"snd_filterquality",
		"snd_waterfx",
		"nosound",
		"precache",
		"loadas8bit",
		"snd_noextraupdate",
		"snd_show",
		"_snd_mixahead",
		"bgm_extmusic",
	} {
		if cv := cvar.Get(name); cv == nil {
			t.Fatalf("cvar %q not registered", name)
		}
	}
}

// TestRegisterHostCVarsIncludesAutosaveCVars verifies that the host registers
// cvars related to the autosave system.
// Why: Provides user control over periodic game state persistence.
// Where in C: host.c, Host_Init (Ironwail extension).
func TestRegisterHostCVarsIncludesAutosaveCVars(t *testing.T) {
	registerHostCVars()

	for _, name := range []string{"sv_autosave", "sv_autosave_interval", "sv_autoload"} {
		if cv := cvar.Get(name); cv == nil {
			t.Fatalf("cvar %q not registered", name)
		}
	}
}

func TestHostInitDedicatedSkipsImplicitLocalClient(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", Dedicated: true, MaxClients: 8}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if subs.Subsystems.Client != nil {
		t.Fatalf("dedicated Host.Init created client %T, want nil", subs.Subsystems.Client)
	}
	if got := h.MaxClients(); got != 8 {
		t.Fatalf("MaxClients = %d, want 8", got)
	}
}

func TestHostShutdownOrdersSubsystemTearDownAndClearsInitialized(t *testing.T) {
	recorder := &shutdownRecorder{}
	h := &Host{initialized: true}
	subs := &Subsystems{
		Files:    &shutdownTrackingFilesystem{recorder: recorder},
		Commands: &shutdownTrackingCommands{recorder: recorder},
		Console:  &shutdownTrackingConsole{recorder: recorder},
		Server:   &shutdownTrackingServer{recorder: recorder},
		Client:   &shutdownTrackingClient{recorder: recorder},
		Input:    input.NewSystem(&shutdownTrackingInputBackend{recorder: recorder}),
		Audio:    &shutdownTrackingAudio{recorder: recorder},
		Renderer: &shutdownTrackingRenderer{recorder: recorder},
	}

	h.Shutdown(subs)

	want := []string{"client", "server", "console", "commands", "audio", "input", "renderer", "files"}
	if !reflect.DeepEqual(recorder.order, want) {
		t.Fatalf("shutdown order = %v, want %v", recorder.order, want)
	}
	if h.initialized {
		t.Fatal("Host.Shutdown left host initialized")
	}
}

// TestMakeServerInfoProviderUsesLiveServerState verifies that the server info
// provider correctly exposes current engine state (hostname, map, player counts).
// Why: Used for server discovery and slist responses.
// Where in C: host.c, SV_Serverinfo_f (and related server metadata logic).
func TestMakeServerInfoProviderUsesLiveServerState(t *testing.T) {
	srv := &mockServer{active: true}
	subs := &Subsystems{Server: srv}
	cvar.Set("hostname", "LAN Party")

	provider := makeServerInfoProvider(subs)
	if provider == nil {
		t.Fatal("makeServerInfoProvider() = nil")
	}
	if got := provider.Hostname(); got != "LAN Party" {
		t.Fatalf("Hostname() = %q, want %q", got, "LAN Party")
	}
	if got := provider.MapName(); got != "start" {
		t.Fatalf("MapName() = %q, want %q", got, "start")
	}
	if got := provider.Players(); got != 1 {
		t.Fatalf("Players() = %d, want 1", got)
	}
	if got := provider.MaxPlayers(); got != 1 {
		t.Fatalf("MaxPlayers() = %d, want 1", got)
	}
}

// TestHostFrame verifies the core host frame loop triggers appropriate callbacks.
// Why: Ensures the main engine loop orchestration is functional.
// Where in C: host.c, Host_Frame.
func TestHostFrame(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	cb := &mockCallbacks{}
	if err := h.Frame(0.016, cb); err != nil {
		t.Fatalf("Frame failed: %v", err)
	}

	if !cb.clientCalled {
		t.Errorf("Client not called")
	}
	if !cb.serverCalled {
		t.Errorf("Server not called")
	}
}

func TestSetMaxFPSDerivesNetIntervalAndFastBypass(t *testing.T) {
	h := NewHost()

	h.SetServerActive(true)
	h.SetMaxFPS(72)
	if got := h.NetInterval(); got != 0 {
		t.Fatalf("NetInterval at 72 FPS = %v, want 0", got)
	}
	if !h.LocalServerFast() {
		t.Fatalf("LocalServerFast at 72 FPS with active server = false, want true")
	}

	h.SetMaxFPS(250)
	if got := h.NetInterval(); got <= 0 {
		t.Fatalf("NetInterval at 250 FPS = %v, want > 0", got)
	}
	if h.LocalServerFast() {
		t.Fatalf("LocalServerFast at 250 FPS with active server = true, want false")
	}

	h.SetServerActive(false)
	h.SetMaxFPS(72)
	if h.LocalServerFast() {
		t.Fatalf("LocalServerFast with inactive server = true, want false")
	}
}

// TestHostFrameAdvancesCompatRNGOnce verifies the compat-RNG is advanced each frame.
// Why: Maintains parity with Quake's frame-based PRNG behavior.
// Where in C: host.c, Host_Frame.
func TestHostFrameAdvancesCompatRNGOnce(t *testing.T) {
	h := NewHost()

	if got := h.compatRNG.Int(); got != 1804289383 {
		t.Fatalf("first compat rand = %d, want 1804289383", got)
	}

	h.compatRNG.Seed(1)
	if err := h.Frame(0.016, nil); err != nil {
		t.Fatalf("Frame failed: %v", err)
	}

	if got := h.compatRNG.Int(); got != 846930886 {
		t.Fatalf("post-frame compat rand = %d, want 846930886", got)
	}
}

// TestHostCommands verifies host-level commands like skill and pause.
// Why: These commands control core engine state and game flow.
// Where in C: host.c, Skill_f and Pause_f.
func TestHostCommands(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)

	h.CmdSkill(2)
	if h.CurrentSkill() != 2 {
		t.Errorf("Expected skill 2, got %d", h.CurrentSkill())
	}

	h.SetServerActive(true)
	h.CmdPause(&subs.Subsystems)
	if !h.ServerPaused() {
		t.Errorf("Expected server paused")
	}
}

// TestLoadingPlaqueAutoExpires verifies that the loading plaque disappears after a minimum duration.
// Why: Provides a consistent user experience during map transitions.
// Where in C: host.c, SCR_UpdateScreen (handling loading state).
func TestLoadingPlaqueAutoExpires(t *testing.T) {
	h := NewHost()
	h.BeginLoadingPlaque(100)

	if !h.LoadingPlaqueActive(100.1) {
		t.Fatal("loading plaque should be active before timeout")
	}
	if h.LoadingPlaqueActive(100.3) {
		t.Fatal("loading plaque should expire after minimum duration")
	}
}

// TestLoadingTransitionPlaqueHoldsUntilSignonComplete verifies that the transition plaque
// persists until the client has fully signed on.
// Why: Ensures the loading screen doesn't flicker or disappear while data is still loading.
// Where in C: host.c, SCR_UpdateScreen (handling loading state).
func TestLoadingTransitionPlaqueHoldsUntilSignonComplete(t *testing.T) {
	h := NewHost()
	h.BeginLoadingTransitionPlaque(100)

	if !h.LoadingPlaqueActive(101) {
		t.Fatal("loading transition plaque should remain active before signon completion")
	}

	h.SetSignOns(client.Signons)

	if !h.LoadingPlaqueActive(100.1) {
		t.Fatal("loading transition plaque should remain active through minimum duration")
	}
	if h.LoadingPlaqueActive(100.3) {
		t.Fatal("loading transition plaque should clear after signon completion and minimum duration")
	}
}

// TestLoadingTransitionPlaqueFailsafeTimeout verifies the failsafe timeout for the loading plaque.
// Why: Prevents the engine from being permanently stuck on a loading screen if networking fails.
// Where in C: host.c (Ironwail specific failsafe).
func TestLoadingTransitionPlaqueFailsafeTimeout(t *testing.T) {
	h := NewHost()
	h.BeginLoadingTransitionPlaque(100)

	if !h.LoadingPlaqueActive(159.9) {
		t.Fatal("loading transition plaque should stay active before failsafe timeout")
	}
	if h.LoadingPlaqueActive(160.1) {
		t.Fatal("loading transition plaque should timeout after failsafe duration")
	}
}
