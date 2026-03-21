// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/server"
)

type mockSubsystems struct {
	Subsystems
	server  *mockServer
	client  *mockClient
	console *mockConsole
}

type mockServer struct {
	active bool
	paused bool
}

func (m *mockServer) Init(maxClients int) error                            { m.active = true; return nil }
func (m *mockServer) Frame(frameTime float64) error                        { return nil }
func (m *mockServer) Shutdown()                                            { m.active = false }
func (m *mockServer) IsActive() bool                                       { return m.active }
func (m *mockServer) IsPaused() bool                                       { return m.paused }
func (m *mockServer) SetLoadGame(v bool)                                   {}
func (m *mockServer) SetPreserveSpawnParms(v bool)                         {}
func (m *mockServer) SpawnServer(mapName string, vfs *fs.FileSystem) error { return nil }
func (m *mockServer) ConnectClient(clientNum int)                          {}
func (m *mockServer) KillClient(clientNum int) bool                        { return false }
func (m *mockServer) KickClient(clientNum int, who, reason string) bool    { return false }
func (m *mockServer) SaveSpawnParms()                                      {}
func (m *mockServer) GetMaxClients() int                                   { return 1 }
func (m *mockServer) IsClientActive(clientNum int) bool                    { return clientNum == 0 }
func (m *mockServer) GetClientName(clientNum int) string                   { return "Player" }
func (m *mockServer) SetClientName(clientNum int, name string)             {}
func (m *mockServer) GetClientColor(clientNum int) int                     { return 0 }
func (m *mockServer) SetClientColor(clientNum int, color int)              {}
func (m *mockServer) GetClientPing(clientNum int) float32                  { return 0 }
func (m *mockServer) EdictNum(n int) *server.Edict                         { return &server.Edict{Vars: &server.EntVars{}} }
func (m *mockServer) GetMapName() string                                   { return "start" }

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

func TestRegisterHostCVarsIncludesAutosaveCVars(t *testing.T) {
	registerHostCVars()

	for _, name := range []string{"sv_autosave", "sv_autosave_interval", "sv_autoload"} {
		if cv := cvar.Get(name); cv == nil {
			t.Fatalf("cvar %q not registered", name)
		}
	}
}

func TestMakeServerInfoProviderUsesLiveServerState(t *testing.T) {
	srv := &mockServer{active: true}
	subs := &Subsystems{Server: srv}
	cvar.Set(serverHostnameCVar, "LAN Party")

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
