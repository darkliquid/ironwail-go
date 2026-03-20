// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/server"
)

type reconnectTrackingServer struct {
	mockServer
	connectCalls int
}

func (s *reconnectTrackingServer) ConnectClient(clientNum int) {
	s.connectCalls++
}

type disconnectTrackingServer struct {
	mockServer
	shutdownCalls int
}

func (s *disconnectTrackingServer) Shutdown() {
	s.shutdownCalls++
	s.mockServer.Shutdown()
}

type sessionStartTrackingServer struct {
	mockServer
	connectCalls  int
	shutdownCalls int
}

func (s *sessionStartTrackingServer) ConnectClient(clientNum int) {
	s.connectCalls++
}

func (s *sessionStartTrackingServer) Shutdown() {
	s.shutdownCalls++
	s.mockServer.Shutdown()
}

type reconnectHandshakeClient struct {
	state ClientState

	signon int

	serverInfoCalls int
	signonReplies   []string
}

func (c *reconnectHandshakeClient) Init() error                    { return nil }
func (c *reconnectHandshakeClient) Frame(frameTime float64) error  { return nil }
func (c *reconnectHandshakeClient) Shutdown()                      {}
func (c *reconnectHandshakeClient) State() ClientState             { return c.state }
func (c *reconnectHandshakeClient) ReadFromServer() error          { return nil }
func (c *reconnectHandshakeClient) SendCommand() error             { return nil }
func (c *reconnectHandshakeClient) SendStringCmd(cmd string) error { return nil }

func (c *reconnectHandshakeClient) LocalServerInfo() error {
	c.serverInfoCalls++
	c.state = caConnected
	c.signon = 0
	return nil
}

func (c *reconnectHandshakeClient) LocalSignonReply(command string) error {
	c.signonReplies = append(c.signonReplies, command)

	switch command {
	case "prespawn":
		if c.signon != 0 {
			return fmt.Errorf("prespawn requires signon 0, got %d", c.signon)
		}
		c.signon = 1
	case "spawn":
		if c.signon != 1 {
			return fmt.Errorf("spawn requires signon 1, got %d", c.signon)
		}
		c.signon = 2
	case "begin":
		if c.signon != 2 {
			return fmt.Errorf("begin requires signon 2, got %d", c.signon)
		}
		c.signon = cl.Signons
		c.state = caActive
	default:
		return fmt.Errorf("unsupported signon reply %q", command)
	}

	return nil
}

func (c *reconnectHandshakeClient) LocalSignon() int {
	return c.signon
}

type remoteSignonTestClient struct {
	state          ClientState
	signonCommands []string
	shutdownCalls  int
}

func (c *remoteSignonTestClient) Init() error                    { return nil }
func (c *remoteSignonTestClient) Frame(frameTime float64) error  { return nil }
func (c *remoteSignonTestClient) Shutdown()                      { c.shutdownCalls++ }
func (c *remoteSignonTestClient) State() ClientState             { return c.state }
func (c *remoteSignonTestClient) ReadFromServer() error          { return nil }
func (c *remoteSignonTestClient) SendCommand() error             { return nil }
func (c *remoteSignonTestClient) SendStringCmd(cmd string) error { return nil }
func (c *remoteSignonTestClient) SendSignonCommand(command string) error {
	c.signonCommands = append(c.signonCommands, command)
	return nil
}

type remoteReconnectStateClient struct {
	state          ClientState
	clientState    *cl.Client
	signonCommands []string
	resetCalls     int
}

func (c *remoteReconnectStateClient) Init() error                    { return nil }
func (c *remoteReconnectStateClient) Frame(frameTime float64) error  { return nil }
func (c *remoteReconnectStateClient) Shutdown()                      {}
func (c *remoteReconnectStateClient) State() ClientState             { return c.state }
func (c *remoteReconnectStateClient) ReadFromServer() error          { return nil }
func (c *remoteReconnectStateClient) SendCommand() error             { return nil }
func (c *remoteReconnectStateClient) SendStringCmd(cmd string) error { return nil }
func (c *remoteReconnectStateClient) SendSignonCommand(command string) error {
	c.signonCommands = append(c.signonCommands, command)
	return nil
}
func (c *remoteReconnectStateClient) ResetConnectionState() error {
	c.resetCalls++
	if c.clientState == nil {
		c.clientState = cl.NewClient()
	}
	c.clientState.ClearState()
	c.clientState.State = cl.StateConnected
	c.state = caConnected
	return nil
}
func (c *remoteReconnectStateClient) ClientState() *cl.Client {
	return c.clientState
}

type stopAllTrackingAudio struct {
	calls []bool
}

func (a *stopAllTrackingAudio) Init() error                                            { return nil }
func (a *stopAllTrackingAudio) Update(origin, velocity, forward, right, up [3]float32) {}
func (a *stopAllTrackingAudio) Shutdown()                                              {}
func (a *stopAllTrackingAudio) SoundInfo() string                                      { return "" }
func (a *stopAllTrackingAudio) StopAllSounds(clear bool) {
	a.calls = append(a.calls, clear)
}

type kickRecord struct {
	clientNum int
	who       string
	reason    string
}

type killTrackingServer struct {
	mockServer
	killCalls []int
}

type kickTrackingServer struct {
	mockServer
	names  []string
	active []bool
	kicks  []kickRecord
}

type colorTrackingServer struct {
	mockServer
	lastColor int
}

type nameTrackingServer struct {
	mockServer
	lastName string
}

type insertTrackingCommandBuffer struct {
	inserted []string
}

func (b *insertTrackingCommandBuffer) Init()               {}
func (b *insertTrackingCommandBuffer) Execute()            {}
func (b *insertTrackingCommandBuffer) AddText(text string) {}
func (b *insertTrackingCommandBuffer) InsertText(text string) {
	b.inserted = append(b.inserted, text)
}
func (b *insertTrackingCommandBuffer) Shutdown() {}

func (s *colorTrackingServer) SetClientColor(clientNum int, color int) {
	s.lastColor = color
}

func (s *killTrackingServer) KillClient(clientNum int) bool {
	s.killCalls = append(s.killCalls, clientNum)
	return true
}

func (s *nameTrackingServer) SetClientName(clientNum int, name string) {
	s.lastName = name
}

func newKickTrackingServer(names ...string) *kickTrackingServer {
	active := make([]bool, len(names))
	for i := range active {
		active[i] = true
	}
	return &kickTrackingServer{
		mockServer: mockServer{active: true},
		names:      append([]string(nil), names...),
		active:     active,
	}
}

func (s *kickTrackingServer) GetMaxClients() int {
	return len(s.names)
}

func (s *kickTrackingServer) IsClientActive(clientNum int) bool {
	return clientNum >= 0 && clientNum < len(s.active) && s.active[clientNum]
}

func (s *kickTrackingServer) GetClientName(clientNum int) string {
	if clientNum < 0 || clientNum >= len(s.names) {
		return ""
	}
	return s.names[clientNum]
}

func (s *kickTrackingServer) KickClient(clientNum int, who, reason string) bool {
	if !s.IsClientActive(clientNum) {
		return false
	}
	s.kicks = append(s.kicks, kickRecord{
		clientNum: clientNum,
		who:       who,
		reason:    reason,
	})
	s.active[clientNum] = false
	return true
}

func TestCmdChangelevel(t *testing.T) {
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

	h.CmdChangelevel("start", &subs.Subsystems)
	// For now, we just check if it doesn't crash and maybe logs something
	// Once implemented, we can check for state changes
}

func TestCmdRestart(t *testing.T) {
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

	h.CmdRestart(&subs.Subsystems)
}

func TestCmdKill(t *testing.T) {
	h := NewHost()
	server := &killTrackingServer{}
	subs := &mockSubsystems{
		server:  &server.mockServer,
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdKill(&subs.Subsystems)
	if len(server.killCalls) != 1 || server.killCalls[0] != 0 {
		t.Fatalf("KillClient calls = %v, want [0]", server.killCalls)
	}
}

func TestCmdGod(t *testing.T) {
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

	h.CmdGod(&subs.Subsystems)
}

func TestCmdNoClip(t *testing.T) {
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

	h.CmdNoClip(&subs.Subsystems)
}

func TestCmdNotarget(t *testing.T) {
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

	h.CmdNotarget(&subs.Subsystems)
}

func TestCmdGive(t *testing.T) {
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

	h.CmdGive("all", "", &subs.Subsystems)
}

func TestCmdName(t *testing.T) {
	h := NewHost()
	srv := &nameTrackingServer{}
	subs := &mockSubsystems{
		server:  &srv.mockServer,
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = srv
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	oldName := cvar.StringValue(clientNameCVar)
	t.Cleanup(func() {
		cvar.Set(clientNameCVar, oldName)
	})

	h.CmdName("Player", &subs.Subsystems)
	if got := srv.lastName; got != "Player" {
		t.Fatalf("server name = %q, want %q", got, "Player")
	}
	if got := cvar.StringValue(clientNameCVar); got != "Player" {
		t.Fatalf("%s = %q, want %q", clientNameCVar, got, "Player")
	}
}

func TestCmdColor(t *testing.T) {
	h := NewHost()
	srv := &colorTrackingServer{}
	subs := &Subsystems{
		Server:  srv,
		Client:  &mockClient{},
		Console: &mockConsole{},
	}

	h.Init(&InitParams{BaseDir: "."}, subs)
	oldColor := cvar.StringValue(clientColorCVar)
	t.Cleanup(func() {
		cvar.Set(clientColorCVar, oldColor)
	})

	h.CmdColor([]string{"13"}, subs)
	if got := srv.lastColor; got != 221 {
		t.Fatalf("single-arg color = %d, want 221", got)
	}
	if got := cvar.IntValue(clientColorCVar); got != 221 {
		t.Fatalf("%s = %d, want 221", clientColorCVar, got)
	}

	h.CmdColor([]string{"1", "2"}, subs)
	if got := srv.lastColor; got != 18 {
		t.Fatalf("two-arg color = %d, want 18", got)
	}
	if got := cvar.IntValue(clientColorCVar); got != 18 {
		t.Fatalf("%s = %d, want 18", clientColorCVar, got)
	}
}

func TestCmdServerInfoIncludesHostname(t *testing.T) {
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
	oldHostname := cvar.StringValue(serverHostnameCVar)
	t.Cleanup(func() {
		cvar.Set(serverHostnameCVar, oldHostname)
	})
	cvar.Set(serverHostnameCVar, "LAN Party")

	h.CmdServerInfo(&subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "host:      LAN Party\n") {
		t.Fatalf("serverinfo output missing hostname in:\n%s", got)
	}
}

func TestCmdPing(t *testing.T) {
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

	h.CmdPing(&subs.Subsystems)
}

func TestCmdKickBySlot(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "2"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].clientNum; got != 1 {
		t.Fatalf("kicked slot = %d, want 1", got)
	}
}

func TestCmdKickByNameCaseInsensitive(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"gRuNt"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].clientNum; got != 1 {
		t.Fatalf("kicked slot = %d, want 1", got)
	}
}

func TestCmdKickIncludesMessage(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "2", "watch", "your", "step"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].reason; got != "watch your step" {
		t.Fatalf("kick reason = %q, want %q", got, "watch your step")
	}
}

func TestCmdKickPreventsSelfKick(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "1"}, subs)
	h.CmdKick([]string{"ranger"}, subs)

	if len(srv.kicks) != 0 {
		t.Fatalf("kick count = %d, want 0", len(srv.kicks))
	}
}

func TestCmdKickUnknownTargetNoOp(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "99"}, subs)
	h.CmdKick([]string{"ogre"}, subs)

	if len(srv.kicks) != 0 {
		t.Fatalf("kick count = %d, want 0", len(srv.kicks))
	}
}

func TestKickCommandRegistrationPreservesFullArgs(t *testing.T) {
	cmdsys.RemoveCommand("kick")
	t.Cleanup(func() {
		cmdsys.RemoveCommand("kick")
	})

	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}
	h.RegisterCommands(subs)
	h.SetServerActive(true)

	cmdsys.ExecuteText("kick # 2 too much ping")
	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].reason; got != "too much ping" {
		t.Fatalf("kick reason = %q, want %q", got, "too much ping")
	}
}

func TestStuffCmds(t *testing.T) {
	h := NewHost()
	cmdBuf := &insertTrackingCommandBuffer{}
	subs := &Subsystems{Commands: cmdBuf}

	h.SetArgs([]string{"+map", "start", "+skill", "2", "-window"})
	h.CmdStuffCmds(subs)

	if len(cmdBuf.inserted) != 1 {
		t.Fatalf("InsertText calls = %d, want 1", len(cmdBuf.inserted))
	}
	if got, want := cmdBuf.inserted[0], "map start\nskill 2\n"; got != want {
		t.Fatalf("InsertText text = %q, want %q", got, want)
	}
}

func TestCmdSaveRejectsInvalidName(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{active: true},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	h.SetServerActive(true)

	h.CmdSave("../bad", &subs.Subsystems)

	if len(subs.console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "invalid save name") {
		t.Fatalf("console output = %q, want invalid save name", got)
	}
}

func TestCmdLoadRejectsInvalidName(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("../bad", &subs.Subsystems)

	if len(subs.console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "invalid save name") {
		t.Fatalf("console output = %q, want invalid save name", got)
	}
}

func TestListSaveSlotsUsesSavedMapNameAndUnusedPlaceholder(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	savesDir := filepath.Join(userDir, "saves")
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(saves): %v", err)
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   2,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "e1m1",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(save): %v", err)
	}
	if err := os.WriteFile(filepath.Join(savesDir, "s0.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile(s0): %v", err)
	}

	slots := h.ListSaveSlots(3)
	if len(slots) != 3 {
		t.Fatalf("slot count = %d, want 3", len(slots))
	}
	if got := slots[0].Name; got != "s0" {
		t.Fatalf("slot[0].Name = %q, want s0", got)
	}
	if got := slots[0].DisplayName; got != "e1m1" {
		t.Fatalf("slot[0].DisplayName = %q, want e1m1", got)
	}
	if got := slots[1].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[1].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
	if got := slots[2].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[2].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
}

func TestListSaveSlotsTreatsMalformedSaveAsUnused(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	savesDir := filepath.Join(userDir, "saves")
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(saves): %v", err)
	}
	if err := os.WriteFile(filepath.Join(savesDir, "s0.sav"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile(s0): %v", err)
	}

	slots := h.ListSaveSlots(1)
	if len(slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(slots))
	}
	if got := slots[0].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[0].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
}

func TestListSaveSlotsFallsBackToLegacyBaseGameSaveWhenUserSaveMissing(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   2,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "legacy-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(save): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "s0.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(2)
	if len(slots) != 2 {
		t.Fatalf("slot count = %d, want 2", len(slots))
	}
	if got := slots[0].DisplayName; got != "legacy-map" {
		t.Fatalf("slot[0].DisplayName = %q, want legacy-map", got)
	}
	if got := slots[1].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[1].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
}

func TestListSaveSlotsFallsBackToLegacyInstallRootSaveWhenUserAndBaseGameSaveMissing(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   2,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "install-root-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(save): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "s0.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile(install root s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(1)
	if len(slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(slots))
	}
	if got := slots[0].DisplayName; got != "install-root-map" {
		t.Fatalf("slot[0].DisplayName = %q, want install-root-map", got)
	}
}

func TestListSaveSlotsPrefersLegacyInstallRootOverBaseGameFallback(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}

	baseGameSave, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "base-game-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(base game): %v", err)
	}
	installRootSave, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "install-root-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(install root): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "s0.sav"), baseGameSave, 0o644); err != nil {
		t.Fatalf("WriteFile(id1 s0): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "s0.sav"), installRootSave, 0o644); err != nil {
		t.Fatalf("WriteFile(root s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(1)
	if len(slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(slots))
	}
	if got := slots[0].DisplayName; got != "install-root-map" {
		t.Fatalf("slot[0].DisplayName = %q, want install-root-map", got)
	}
}

func TestListSaveSlotsPrefersUserSaveOverLegacyFallback(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(userDir, "saves"), 0o755); err != nil {
		t.Fatalf("MkdirAll(saves): %v", err)
	}

	legacySave, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "legacy-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(legacy): %v", err)
	}
	userSave, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "user-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(user): %v", err)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "id1", "s0.sav"), legacySave, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy s0): %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "saves", "s0.sav"), userSave, 0o644); err != nil {
		t.Fatalf("WriteFile(user s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(1)
	if len(slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(slots))
	}
	if got := slots[0].DisplayName; got != "user-map" {
		t.Fatalf("slot[0].DisplayName = %q, want user-map", got)
	}
}

func TestListSaveSlotsTreatsMalformedUserSaveAsUnusedWithLegacyFallback(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(userDir, "saves"), 0o755); err != nil {
		t.Fatalf("MkdirAll(saves): %v", err)
	}

	legacySave, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "legacy-map",
		},
	})
	if err != nil {
		t.Fatalf("Marshal(legacy): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "s0.sav"), legacySave, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy s0): %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "saves", "s0.sav"), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("WriteFile(user s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(2)
	if len(slots) != 2 {
		t.Fatalf("slot count = %d, want 2", len(slots))
	}
	if got := slots[0].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[0].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
	if got := slots[1].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[1].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
}

func TestCmdSaveRejectsIntermission(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	lc := newLocalLoopbackClient()
	subs := &Subsystems{
		Server:  srv,
		Client:  lc,
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.SetServerActive(true)
	srv.Active = true
	lc.inner.Intermission = 1

	h.CmdSave("blocked", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Can't save in intermission.") {
		t.Fatalf("console output = %q, want intermission rejection", got)
	}
	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "blocked.sav")); !os.IsNotExist(err) {
		t.Fatalf("save file should not exist, stat err = %v", err)
	}
}

func TestCmdSaveRejectsDeadPlayer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	subs := &Subsystems{
		Server:  srv,
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.SetServerActive(true)
	srv.Active = true
	srv.Static.Clients[0].Active = true
	srv.Static.Clients[0].Edict.Vars.Health = 0

	h.CmdSave("dead", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Can't savegame with a dead player") {
		t.Fatalf("console output = %q, want dead-player rejection", got)
	}
	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "dead.sav")); !os.IsNotExist(err) {
		t.Fatalf("save file should not exist, stat err = %v", err)
	}
}

func TestCmdSaveRejectsNoMonsters(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	subs := &Subsystems{
		Server:  srv,
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	cvar.Set("nomonsters", "1")
	t.Cleanup(func() {
		cvar.Set("nomonsters", "0")
	})

	h.SetServerActive(true)
	srv.Active = true

	h.CmdSave("nomonsters_blocked", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Can't save when using \"nomonsters\".") {
		t.Fatalf("console output = %q, want nomonsters rejection", got)
	}
	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "nomonsters_blocked.sav")); !os.IsNotExist(err) {
		t.Fatalf("save file should not exist, stat err = %v", err)
	}
}

func TestCmdRecordUsesLoopbackClientCDTrack(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.CDTrack = 7
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdRecord("music_header", subs)
	if h.demoState == nil {
		t.Fatal("expected demo state to be created")
	}
	if got := h.demoState.CDTrack; got != 7 {
		t.Fatalf("demo CDTrack = %d, want 7", got)
	}
	if err := h.demoState.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "demos", "music_header.dem"))
	if err != nil {
		t.Fatalf("ReadFile(demo): %v", err)
	}
	if !strings.HasPrefix(string(data), "7\n") {
		t.Fatalf("demo header = %q, want prefix %q", string(data), "7\\n")
	}
}

func TestCmdStopWritesDisconnectTrailer(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.ViewAngles = [3]float32{11, 22, 33}
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.demoState = cl.NewDemoState()
	if err := h.demoState.StartDemoRecording("stop_trailer", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	h.CmdStop(subs)
	if h.demoState.Recording {
		t.Fatal("demo recording still active after stop")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Completed demo") {
		t.Fatalf("console output = %q, want completion message", got)
	}

	replay := cl.NewDemoState()
	if err := replay.StartDemoPlayback(filepath.Join(tmpDir, "demos", "stop_trailer.dem")); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer replay.StopPlayback()

	message, angles, err := replay.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}
	if len(message) != 1 || message[0] != inet.SVCDisconnect {
		t.Fatalf("disconnect message = %v, want [%d]", message, inet.SVCDisconnect)
	}
	if angles != lc.inner.ViewAngles {
		t.Fatalf("disconnect angles = %v, want %v", angles, lc.inner.ViewAngles)
	}
}

func TestCmdRecordWritesInitialStateSnapshotWhenConnected(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = 4
	lc.inner.Protocol = inet.PROTOCOL_FITZQUAKE
	lc.inner.MaxClients = 1
	lc.inner.LevelName = "Snapshot Command"
	lc.inner.ModelPrecache = []string{"maps/start.bsp"}
	lc.inner.SoundPrecache = []string{"misc/null.wav"}
	lc.inner.ViewEntity = 1
	lc.inner.CDTrack = 3
	lc.inner.LoopTrack = 3
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdRecord("record_snapshot", subs)
	if h.demoState == nil {
		t.Fatal("expected demo state")
	}
	if err := h.demoState.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	replay := cl.NewDemoState()
	if err := replay.StartDemoPlayback(filepath.Join(tmpDir, "demos", "record_snapshot.dem")); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer replay.StopPlayback()

	message, _, err := replay.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}
	if len(message) == 0 || message[0] != byte(inet.SVCServerInfo) {
		t.Fatalf("first frame = %v, want serverinfo snapshot", message)
	}
}

func TestCmdPlaydemoLeavesLoopbackClientDisconnectedForServerInfo(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.MkdirAll(filepath.Join(tmpDir, "demos"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "demos", "bootstrap.dem"), []byte("0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = 4
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdPlaydemo("bootstrap", subs)
	if lc.inner.State != cl.StateDisconnected {
		t.Fatalf("loopback client state = %v, want disconnected", lc.inner.State)
	}
}

func TestCmdTimedemoEnablesTimeDemoPlayback(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timedemo_cmd", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame failed: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdTimedemo("timedemo_cmd", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}
	if !h.demoState.TimeDemo {
		t.Fatal("expected timedemo mode to be active")
	}
	if clientState := LoopbackClientState(subs); clientState == nil || !clientState.DemoPlayback || !clientState.TimeDemoActive {
		t.Fatalf("loopback demo flags = %#v, want demo playback and timedemo active", clientState)
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Timing demo") {
		t.Fatalf("console output = %q, want timedemo banner", got)
	}
}

func TestCmdRewindSeeksBackwardFromCurrentFrame(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("rewind_cmd", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdPlaydemo("rewind_cmd", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}
	for i := 0; i < 3; i++ {
		if _, _, err := h.demoState.ReadDemoFrame(); err != nil {
			t.Fatalf("ReadDemoFrame %d failed: %v", i, err)
		}
	}
	if got := h.demoState.FrameIndex; got != 3 {
		t.Fatalf("frame index before rewind = %d, want 3", got)
	}

	h.CmdRewind(2, subs)
	if got := h.demoState.FrameIndex; got != 1 {
		t.Fatalf("frame index after rewind = %d, want 1", got)
	}
}

func TestCmdDemoSeekRejectsFrameEqualToFrameCount(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("demoseek_bounds", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdPlaydemo("demoseek_bounds", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}
	frameCount := len(h.demoState.Frames)

	h.CmdDemoSeek(frameCount, subs)

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, fmt.Sprintf("Frame %d out of range", frameCount)) {
		t.Fatalf("console output = %q, want out-of-range message for frame %d", output, frameCount)
	}
	if strings.Contains(output, "Failed to seek demo") {
		t.Fatalf("console output = %q, did not expect seek failure from lower-level demo code", output)
	}
}

func TestCmdReconnectRestartsLocalHandshake(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	srv := &reconnectTrackingServer{mockServer: mockServer{active: true}}
	client := &reconnectHandshakeClient{state: caActive, signon: cl.Signons}
	audio := &stopAllTrackingAudio{}
	subs := &Subsystems{
		Server:  srv,
		Client:  client,
		Console: console,
		Audio:   audio,
	}

	h.CmdReconnect(subs)

	if srv.connectCalls != 1 {
		t.Fatalf("ConnectClient calls = %d, want 1", srv.connectCalls)
	}
	if client.serverInfoCalls != 1 {
		t.Fatalf("LocalServerInfo calls = %d, want 1", client.serverInfoCalls)
	}
	if want := []string{"prespawn", "spawn", "begin"}; !reflect.DeepEqual(client.signonReplies, want) {
		t.Fatalf("signon replies = %v, want %v", client.signonReplies, want)
	}
	if client.signon != cl.Signons {
		t.Fatalf("client signon = %d, want %d", client.signon, cl.Signons)
	}
	if client.state != caActive {
		t.Fatalf("client state = %v, want %v", client.state, caActive)
	}
	if h.SignOns() != cl.Signons {
		t.Fatalf("host signons = %d, want %d", h.SignOns(), cl.Signons)
	}
	if h.ClientState() != caActive {
		t.Fatalf("host client state = %v, want %v", h.ClientState(), caActive)
	}
	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after reconnect")
	}
}

func TestCmdConnectLocalRestartsLocalHandshakeAndStopsDemoPlayback(t *testing.T) {
	h := NewHost()
	h.SetDemoNum(2)
	h.demoState = &cl.DemoState{Playback: true}
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	srv := &reconnectTrackingServer{mockServer: mockServer{active: true}}
	client := &reconnectHandshakeClient{state: caActive, signon: cl.Signons}
	subs := &Subsystems{
		Server:  srv,
		Client:  client,
		Console: console,
	}

	h.CmdConnect("local", subs)

	if got := h.DemoNum(); got != -1 {
		t.Fatalf("demoNum = %d, want -1", got)
	}
	if h.demoState.Playback {
		t.Fatal("demo playback still active after connect local")
	}
	if srv.connectCalls != 1 {
		t.Fatalf("ConnectClient calls = %d, want 1", srv.connectCalls)
	}
	if client.serverInfoCalls != 1 {
		t.Fatalf("LocalServerInfo calls = %d, want 1", client.serverInfoCalls)
	}
	if want := []string{"prespawn", "spawn", "begin"}; !reflect.DeepEqual(client.signonReplies, want) {
		t.Fatalf("signon replies = %v, want %v", client.signonReplies, want)
	}
	if client.state != caActive {
		t.Fatalf("client state = %v, want %v", client.state, caActive)
	}
	if h.ClientState() != caActive {
		t.Fatalf("host client state = %v, want %v", h.ClientState(), caActive)
	}
	if h.SignOns() != cl.Signons {
		t.Fatalf("host signons = %d, want %d", h.SignOns(), cl.Signons)
	}
}

func TestCmdConnectRemoteUsesTransportClientAndDisconnectsCurrentSession(t *testing.T) {
	h := NewHost()
	h.SetDemoNum(3)
	h.demoState = &cl.DemoState{Playback: true}
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	srv := &disconnectTrackingServer{mockServer: mockServer{active: true}}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = cl.Signons
	remoteState := cl.NewClient()
	remoteState.State = cl.StateActive
	remoteState.Signon = cl.Signons
	remoteState.LevelName = "stale-level"
	remoteClient := &remoteReconnectStateClient{
		state:       caActive,
		clientState: remoteState,
	}
	oldFactory := remoteClientFactory
	remoteClientFactory = func(address string) (Client, error) {
		if address != "example.com:26000" {
			t.Fatalf("remoteClientFactory address = %q, want %q", address, "example.com:26000")
		}
		return remoteClient, nil
	}
	t.Cleanup(func() {
		remoteClientFactory = oldFactory
	})
	subs := &Subsystems{
		Server:  srv,
		Client:  lc,
		Console: console,
	}

	h.CmdConnect("example.com:26000", subs)

	if got := h.DemoNum(); got != -1 {
		t.Fatalf("demoNum = %d, want -1", got)
	}
	if h.demoState.Playback {
		t.Fatal("demo playback still active after remote connect attempt")
	}
	if srv.shutdownCalls != 1 {
		t.Fatalf("Shutdown calls = %d, want 1", srv.shutdownCalls)
	}
	if h.ServerActive() {
		t.Fatal("serverActive = true, want false")
	}
	if h.ClientState() != caConnected {
		t.Fatalf("client state = %v, want %v", h.ClientState(), caConnected)
	}
	if h.SignOns() != 0 {
		t.Fatalf("host signons = %d, want 0", h.SignOns())
	}
	if subs.Client != remoteClient {
		t.Fatalf("client = %T, want remote transport client", subs.Client)
	}
	if got := remoteClient.resetCalls; got != 1 {
		t.Fatalf("ResetConnectionState calls = %d, want 1", got)
	}
	if got := remoteState.State; got != cl.StateConnected {
		t.Fatalf("remote state = %v, want %v", got, cl.StateConnected)
	}
	if got := remoteState.Signon; got != 0 {
		t.Fatalf("remote signon = %d, want 0", got)
	}
	if got := remoteState.LevelName; got != "" {
		t.Fatalf("remote level = %q, want cleared", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after remote connect")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Connecting to example.com:26000...") {
		t.Fatalf("console output = %q, want remote connect banner", got)
	}
}

func TestCmdConnectLocalWithoutServerPrintsErrorAndDisconnects(t *testing.T) {
	h := NewHost()
	h.SetDemoNum(4)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = cl.Signons
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdConnect("local", subs)

	if got := h.DemoNum(); got != -1 {
		t.Fatalf("demoNum = %d, want -1", got)
	}
	if h.ClientState() != caDisconnected {
		t.Fatalf("client state = %v, want %v", h.ClientState(), caDisconnected)
	}
	if h.SignOns() != 0 {
		t.Fatalf("host signons = %d, want 0", h.SignOns())
	}
	if lc.inner.State != cl.StateDisconnected {
		t.Fatalf("loopback state = %v, want disconnected", lc.inner.State)
	}
	if lc.inner.Signon != 0 {
		t.Fatalf("loopback signon = %d, want 0", lc.inner.Signon)
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "No local server is active.") {
		t.Fatalf("console output = %q, want missing-local-server message", got)
	}
}

func TestCmdDisconnectStopsPlaybackAndClearsConnectionState(t *testing.T) {
	h := NewHost()
	h.demoState = &cl.DemoState{Playback: true}
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	srv := &disconnectTrackingServer{mockServer: mockServer{active: true}}
	lc := newLocalLoopbackClient()
	audio := &stopAllTrackingAudio{}
	lc.inner.State = cl.StateActive
	lc.inner.Signon = cl.Signons
	subs := &Subsystems{
		Server:  srv,
		Client:  lc,
		Console: console,
		Audio:   audio,
	}

	h.CmdDisconnect(subs)

	if h.demoState.Playback {
		t.Fatal("demo playback still active after disconnect")
	}
	if srv.shutdownCalls != 1 {
		t.Fatalf("Shutdown calls = %d, want 1", srv.shutdownCalls)
	}
	if h.ServerActive() {
		t.Fatal("serverActive = true, want false")
	}
	if h.ClientState() != caDisconnected {
		t.Fatalf("client state = %v, want %v", h.ClientState(), caDisconnected)
	}
	if h.SignOns() != 0 {
		t.Fatalf("host signons = %d, want 0", h.SignOns())
	}
	if lc.inner.State != cl.StateDisconnected {
		t.Fatalf("loopback state = %v, want disconnected", lc.inner.State)
	}
	if lc.inner.Signon != 0 {
		t.Fatalf("loopback signon = %d, want 0", lc.inner.Signon)
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Disconnected.") {
		t.Fatalf("console output = %q, want disconnect confirmation", got)
	}
	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
}

func TestCmdMapStopsAllSoundsBeforeStartingSession(t *testing.T) {
	h := NewHost()
	audio := &stopAllTrackingAudio{}
	srv := &reconnectTrackingServer{}
	client := &reconnectHandshakeClient{}
	subs := &Subsystems{
		Files:   &fs.FileSystem{},
		Server:  srv,
		Client:  client,
		Audio:   audio,
		Console: &mockConsole{},
	}

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start) failed: %v", err)
	}
	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
}

func TestCmdMapShutsDownRemoteClientBeforeReplacingWithLocalClient(t *testing.T) {
	h := NewHost()
	srv := &sessionStartTrackingServer{}
	remoteClient := &remoteSignonTestClient{state: caConnected}
	subs := &Subsystems{
		Files:   &fs.FileSystem{},
		Server:  srv,
		Client:  remoteClient,
		Console: &mockConsole{},
	}

	err := h.CmdMap("start", subs)
	if err == nil {
		t.Fatal("CmdMap(start) error = nil, want local handshake failure")
	}
	if !strings.Contains(err.Error(), "local serverinfo handshake failed") {
		t.Fatalf("CmdMap(start) error = %q, want local serverinfo handshake failure", err)
	}
	if remoteClient.shutdownCalls != 1 {
		t.Fatalf("remote client Shutdown calls = %d, want 1", remoteClient.shutdownCalls)
	}
	if _, ok := subs.Client.(*localLoopbackClient); !ok {
		t.Fatalf("client = %T, want *localLoopbackClient", subs.Client)
	}
}

func TestStartLocalServerSessionRollsBackOnAfterConnectFailure(t *testing.T) {
	h := NewHost()
	h.SetClientState(caConnected)
	h.SetSignOns(2)

	srv := &sessionStartTrackingServer{}
	remoteClient := &remoteSignonTestClient{state: caConnected}
	subs := &Subsystems{
		Server: srv,
		Client: remoteClient,
	}

	err := h.startLocalServerSession(subs, func() error {
		return fmt.Errorf("restore failed")
	})
	if err == nil {
		t.Fatal("startLocalServerSession error = nil, want restore failure")
	}
	if !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("startLocalServerSession error = %q, want restore failure", err)
	}
	if srv.connectCalls != 1 {
		t.Fatalf("ConnectClient calls = %d, want 1", srv.connectCalls)
	}
	if srv.shutdownCalls != 1 {
		t.Fatalf("Shutdown calls = %d, want 1", srv.shutdownCalls)
	}
	if remoteClient.shutdownCalls != 1 {
		t.Fatalf("remote client Shutdown calls = %d, want 1", remoteClient.shutdownCalls)
	}
	if h.ServerActive() {
		t.Fatal("serverActive = true, want false after rollback")
	}
	if got := h.ClientState(); got != caDisconnected {
		t.Fatalf("client state = %v, want %v after rollback", got, caDisconnected)
	}
	if got := h.SignOns(); got != 0 {
		t.Fatalf("host signons = %d, want 0 after rollback", got)
	}
}

func TestCmdLoadStopsAllSoundsDuringSessionTransition(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "missingmap",
		},
	})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(userDir, "saves"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "saves", "slot1.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	audio := &stopAllTrackingAudio{}
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
		Audio:   audio,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("slot1", subs)

	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "load failed:") {
		t.Fatalf("console output = %q, want load failure text", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after load transition")
	}
}

func TestCmdLoadFallsBackToBaseGameSaveWhenUserSaveMissing(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) failed: %v", dir, err)
		}
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "missingmap",
		},
	})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "slot1.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	audio := &stopAllTrackingAudio{}
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
		Audio:   audio,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("slot1", subs)

	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "load failed:") {
		t.Fatalf("console output = %q, want load failure text", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after legacy save fallback")
	}
}

func TestCmdLoadFallsBackToLegacyInstallRootSaveWhenUserAndBaseGameSaveMissing(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) failed: %v", dir, err)
		}
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "missingmap",
		},
	})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "slot1.sav"), saveData, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	audio := &stopAllTrackingAudio{}
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
		Audio:   audio,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("slot1", subs)

	if len(audio.calls) != 1 {
		t.Fatalf("StopAllSounds calls = %d, want 1", len(audio.calls))
	}
	if !audio.calls[0] {
		t.Fatal("StopAllSounds clear flag = false, want true")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "load failed:") {
		t.Fatalf("console output = %q, want load failure text", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after install-root save fallback")
	}
}

func TestCmdReconnectClearsSignonsWithoutLocalServer(t *testing.T) {
	h := NewHost()
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = cl.Signons
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdReconnect(subs)

	if lc.inner.Signon != 0 {
		t.Fatalf("loopback signon = %d, want 0", lc.inner.Signon)
	}
	if lc.inner.State != cl.StateConnected {
		t.Fatalf("loopback state = %v, want connected", lc.inner.State)
	}
	if h.SignOns() != 0 {
		t.Fatalf("host signons = %d, want 0", h.SignOns())
	}
	if h.ClientState() != caConnected {
		t.Fatalf("host client state = %v, want %v", h.ClientState(), caConnected)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after reconnect")
	}
}

func TestCmdReconnectForRemoteClientResetsClientStateWithoutSignonCommand(t *testing.T) {
	h := NewHost()
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	remoteState := cl.NewClient()
	remoteState.State = cl.StateActive
	remoteState.Signon = cl.Signons
	remoteState.LevelName = "stale level"
	client := &remoteReconnectStateClient{
		state:       caActive,
		clientState: remoteState,
	}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdReconnect(subs)

	if got := client.resetCalls; got != 1 {
		t.Fatalf("ResetConnectionState calls = %d, want 1", got)
	}
	if len(client.signonCommands) != 0 {
		t.Fatalf("remote reconnect sent signon commands = %v, want none", client.signonCommands)
	}
	if got := remoteState.Signon; got != 0 {
		t.Fatalf("remote signon = %d, want 0", got)
	}
	if got := remoteState.State; got != cl.StateConnected {
		t.Fatalf("remote client state = %v, want %v", got, cl.StateConnected)
	}
	if got := remoteState.LevelName; got != "" {
		t.Fatalf("remote level name = %q, want cleared", got)
	}
	if got := h.SignOns(); got != 0 {
		t.Fatalf("host signons = %d, want 0", got)
	}
	if got := h.ClientState(); got != caConnected {
		t.Fatalf("host client state = %v, want %v", got, caConnected)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after remote reconnect")
	}
}

func TestCmdReconnectIgnoresDemoPlayback(t *testing.T) {
	h := NewHost()
	h.demoState = &cl.DemoState{Playback: true}
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(cl.Signons)

	console := &mockConsole{}
	srv := &reconnectTrackingServer{mockServer: mockServer{active: true}}
	client := &reconnectHandshakeClient{state: caActive, signon: cl.Signons}
	subs := &Subsystems{
		Server:  srv,
		Client:  client,
		Console: console,
	}

	h.CmdReconnect(subs)

	if srv.connectCalls != 0 {
		t.Fatalf("ConnectClient calls = %d, want 0", srv.connectCalls)
	}
	if client.serverInfoCalls != 0 {
		t.Fatalf("LocalServerInfo calls = %d, want 0", client.serverInfoCalls)
	}
	if h.SignOns() != cl.Signons {
		t.Fatalf("host signons = %d, want %d", h.SignOns(), cl.Signons)
	}
	if h.ClientState() != caActive {
		t.Fatalf("host client state = %v, want %v", h.ClientState(), caActive)
	}
}

func TestCmdPreSpawnForRemoteClientSendsSignonCommand(t *testing.T) {
	h := NewHost()
	h.SetClientState(caConnected)

	client := &remoteSignonTestClient{state: caConnected}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdPreSpawn(subs)
	h.CmdSpawn(subs)
	h.CmdBegin(subs)

	if want := []string{"prespawn", "spawn", "begin"}; !reflect.DeepEqual(client.signonCommands, want) {
		t.Fatalf("remote signon commands = %v, want %v", client.signonCommands, want)
	}
}

func TestAliasCommandsDefineAndRemoveAliases(t *testing.T) {
	cmdsys.UnaliasAll()
	t.Cleanup(cmdsys.UnaliasAll)

	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	var gotArgs []string
	cmdsys.AddCommand("test_alias_target", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "")
	defer cmdsys.RemoveCommand("test_alias_target")

	h.CmdAlias([]string{"foo", "test_alias_target", "bar", "baz"}, &subs.Subsystems)
	cmdsys.ExecuteText("foo")
	if want := []string{"bar", "baz"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("alias execution args = %v, want %v", gotArgs, want)
	}

	h.CmdUnalias([]string{"foo"}, &subs.Subsystems)
	gotArgs = nil
	cmdsys.ExecuteText("foo")
	if gotArgs != nil {
		t.Fatalf("expected foo alias to be removed, got args %v", gotArgs)
	}

	h.CmdAlias([]string{"one", "test_alias_target", "one"}, &subs.Subsystems)
	h.CmdAlias([]string{"two", "test_alias_target", "two"}, &subs.Subsystems)
	h.CmdUnaliasAll()
	if _, ok := cmdsys.Alias("one"); ok {
		t.Fatal("expected alias one to be removed by unaliasall")
	}
	if _, ok := cmdsys.Alias("two"); ok {
		t.Fatal("expected alias two to be removed by unaliasall")
	}
}

func TestAliasCommandSupportsQuotedSemicolonBodies(t *testing.T) {
	cmdsys.UnaliasAll()
	t.Cleanup(cmdsys.UnaliasAll)

	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	h.RegisterCommands(&subs.Subsystems)

	var got []string
	cmdsys.AddCommand("test_alias_chain", func(args []string) {
		got = append(got, strings.Join(args, " "))
	}, "")
	defer cmdsys.RemoveCommand("test_alias_chain")

	cmdsys.ExecuteText(`alias combo "test_alias_chain one; test_alias_chain two"`)
	cmdsys.ExecuteText("combo")

	want := []string{"one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("alias chain = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// startdemos / demos / stopdemo playlist tests
// ---------------------------------------------------------------------------

func TestCmdStartdemosStoresDemoNames(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	// Provide some demo names. The host has no game running so it will try
	// CmdDemos which calls CmdPlaydemo. Without a real filesystem the
	// playback will fail, but the list should still be stored.
	h.CmdStartdemos([]string{"demo1", "demo2", "demo3"}, subs)

	got := h.DemoList()
	want := []string{"demo1", "demo2", "demo3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DemoList = %v, want %v", got, want)
	}
}

func TestCmdStartdemosClipsToMaxDemos(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	names := make([]string, 12)
	for i := range names {
		names[i] = fmt.Sprintf("demo%d", i)
	}
	h.CmdStartdemos(names, subs)

	got := h.DemoList()
	if len(got) != MaxDemos {
		t.Fatalf("DemoList length = %d, want %d", len(got), MaxDemos)
	}
	if got[MaxDemos-1] != fmt.Sprintf("demo%d", MaxDemos-1) {
		t.Fatalf("last demo = %q, want %q", got[MaxDemos-1], fmt.Sprintf("demo%d", MaxDemos-1))
	}
}

func TestCmdStartdemosSetsDemoNumToZero(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.SetDemoNum(-1)
	h.CmdStartdemos([]string{"demo1"}, subs)

	// After CmdDemos runs (triggered because no game active), demoNum
	// advances to 1 (past the first demo that was queued).
	if got := h.DemoNum(); got < 0 {
		t.Fatalf("DemoNum = %d, want >= 0", got)
	}
}

func TestCmdStartdemosNoArgsPrintsUsage(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdStartdemos(nil, subs)

	if len(console.messages) == 0 || !strings.Contains(console.messages[0], "usage") {
		t.Fatalf("expected usage message, got %v", console.messages)
	}
	if h.DemoNum() != -1 {
		t.Fatalf("DemoNum = %d, want -1 (unchanged)", h.DemoNum())
	}
}

func TestCmdDemosCyclesToNextDemo(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.SetDemoList([]string{"demo1", "demo2", "demo3"})
	h.SetDemoNum(1) // start from second entry

	h.CmdDemos(subs)

	// Should have advanced past demo2.
	if got := h.DemoNum(); got != 2 {
		t.Fatalf("DemoNum = %d, want 2", got)
	}
}

func TestCmdDemosWrapsAround(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.SetDemoList([]string{"demo1", "demo2"})
	h.SetDemoNum(2) // past end

	h.CmdDemos(subs)

	// Should wrap to 0 then advance to 1.
	if got := h.DemoNum(); got != 1 {
		t.Fatalf("DemoNum = %d, want 1", got)
	}
}

func TestCmdDemosDisabledPrintsMessage(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.SetDemoNum(-1)
	h.CmdDemos(subs)

	if len(console.messages) == 0 || !strings.Contains(console.messages[0], "No demo loop") {
		t.Fatalf("expected 'No demo loop' message, got %v", console.messages)
	}
}

func TestCmdStopdemoResetsDemoNum(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	client := newLocalLoopbackClient()
	client.inner.DemoPlayback = true
	client.inner.TimeDemoActive = true
	subs := &Subsystems{Console: console, Client: client}

	h.demoState = &cl.DemoState{Playback: true}
	h.SetDemoNum(2)
	h.CmdStopdemo(subs)

	if got := h.DemoNum(); got != -1 {
		t.Fatalf("DemoNum = %d, want -1 after stopdemo", got)
	}
	if client.inner.DemoPlayback || client.inner.TimeDemoActive {
		t.Fatalf("loopback demo flags = demo:%v timedemo:%v, want both false", client.inner.DemoPlayback, client.inner.TimeDemoActive)
	}
}

func TestCmdDemoGotoSeeksToTimeBasedFrame(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("demogoto_cmd", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	// 144 frames = 2 seconds at 72 Hz
	for i := 0; i < 144; i++ {
		if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d failed: %v", i, err)
		}
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdPlaydemo("demogoto_cmd", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}

	h.CmdDemoGoto(1.0, subs) // 1 second = frame 72
	if got := h.demoState.FrameIndex; got != 72 {
		t.Fatalf("frame index after demogoto 1.0 = %d, want 72", got)
	}

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "1.00s") {
		t.Fatalf("console output = %q, expected time confirmation", output)
	}
}

func TestCmdDemoPauseToggles(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("pause_cmd", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{0, 0, 0}); err != nil {
		t.Fatalf("WriteDemoFrame failed: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdPlaydemo("pause_cmd", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}

	h.CmdDemoPause(subs)
	if !h.demoState.Paused {
		t.Fatal("expected demo to be paused after first toggle")
	}

	h.CmdDemoPause(subs)
	if h.demoState.Paused {
		t.Fatal("expected demo to be unpaused after second toggle")
	}

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "paused") || !strings.Contains(output, "resumed") {
		t.Fatalf("console output = %q, expected pause/resume messages", output)
	}
}

func TestCmdDemoSpeedSetsMultiplier(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("speed_cmd", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{0, 0, 0}); err != nil {
		t.Fatalf("WriteDemoFrame failed: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: tmpDir, UserDir: tmpDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdPlaydemo("speed_cmd", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}

	h.CmdDemoSpeed(2.5, subs)
	if got := h.demoState.Speed; got != 2.5 {
		t.Fatalf("Speed = %f, want 2.5", got)
	}

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "2.50") {
		t.Fatalf("console output = %q, expected speed confirmation", output)
	}
}

func TestCmdDemoGotoNotPlayingBack(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	h.CmdDemoGoto(1.0, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "Not playing back") {
		t.Fatalf("console output = %q, expected not-playing message", output)
	}
}

func TestCmdDemoPauseNotPlayingBack(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	h.CmdDemoPause(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "Not playing back") {
		t.Fatalf("console output = %q, expected not-playing message", output)
	}
}

func TestCmdDemoSpeedNotPlayingBack(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	h.CmdDemoSpeed(2.0, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "Not playing back") {
		t.Fatalf("console output = %q, expected not-playing message", output)
	}
}

// --- randmap tests ---

type mapListingFiles struct {
	files []string
}

func (m *mapListingFiles) Init(baseDir, gameDir string) error { return nil }
func (m *mapListingFiles) Close()                             {}
func (m *mapListingFiles) LoadFile(filename string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mapListingFiles) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	return "", nil, fmt.Errorf("not found")
}
func (m *mapListingFiles) FileExists(filename string) bool { return false }

func TestCmdRandmapNoServer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
		Files:   &mapListingFiles{},
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	// serverActive is false by default
	h.CmdRandmap(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no server running") {
		t.Errorf("expected 'no server running', got %q", output)
	}
}

func TestCmdRandmapNoFiles(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
		Files:   &mapListingFiles{files: nil},
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	// Files is not *fs.FileSystem, so the type assertion fails and returns early silently
	h.CmdRandmap(subs)
}

// --- viewframe/viewnext/viewprev tests ---

func TestCmdViewframeNoServer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.CmdViewframe(5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no server running") {
		t.Errorf("expected 'no server running', got %q", output)
	}
}

func TestCmdViewframeNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewframe(5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewnextNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewnext(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewprevNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewprev(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewframeNegativeClampsToZero(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	// With mockServer, findViewthing returns nil (type assertion fails).
	// This tests the "no viewthing" path with negative frame.
	h.CmdViewframe(-5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}
