// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"testing"
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

func (m *mockServer) Init(maxClients int) error     { m.active = true; return nil }
func (m *mockServer) Frame(frameTime float64) error { return nil }
func (m *mockServer) Shutdown()                     { m.active = false }
func (m *mockServer) IsActive() bool                { return m.active }
func (m *mockServer) IsPaused() bool                { return m.paused }
func (m *mockServer) SpawnServer(mapName string, vfs Filesystem) error { return nil }
func (m *mockServer) SaveSpawnParms()                                     {}
func (m *mockServer) GetMaxClients() int                                  { return 1 }
func (m *mockServer) GetClientName(clientNum int) string                  { return "Player" }
func (m *mockServer) SetClientName(clientNum int, name string)            {}
func (m *mockServer) GetClientColor(clientNum int) int                     { return 0 }
func (m *mockServer) SetClientColor(clientNum int, color int)             {}
func (m *mockServer) GetClientPing(clientNum int) float32                 { return 0 }
func (m *mockServer) EdictNum(n int) *server.Edict                        { return &server.Edict{Vars: &server.EntVars{}} }
func (m *mockServer) GetMapName() string                                  { return "start" }


type mockClient struct {
	state ClientState
}

func (m *mockClient) Init() error                   { return nil }
func (m *mockClient) Frame(frameTime float64) error { return nil }
func (m *mockClient) Shutdown()                     {}
func (m *mockClient) State() ClientState            { return m.state }
func (m *mockClient) ReadFromServer() error         { return nil }
func (m *mockClient) SendCommand() error            { return nil }

type mockConsole struct {
	messages []string
}

func (m *mockConsole) Init() error      { return nil }
func (m *mockConsole) Print(msg string) { m.messages = append(m.messages, msg) }
func (m *mockConsole) Shutdown()        {}

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
	h.CmdPause()
	if !h.ServerPaused() {
		t.Errorf("Expected server paused")
	}
}
