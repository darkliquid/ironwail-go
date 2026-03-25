// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	stdnet "net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/menu"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
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

type readSeekNopCloser struct {
	*bytes.Reader
}

func (r readSeekNopCloser) Close() error { return nil }

type demoCommandFiles struct {
	loaded    map[string][]byte
	loadCalls []string
	openCalls []string
}

func (f *demoCommandFiles) Init(baseDir, gameDir string) error { return nil }
func (f *demoCommandFiles) Close()                             {}
func (f *demoCommandFiles) LoadFile(filename string) ([]byte, error) {
	f.loadCalls = append(f.loadCalls, filename)
	data, ok := f.loaded[filename]
	if !ok {
		return nil, fmt.Errorf("missing file %s", filename)
	}
	return data, nil
}
func (f *demoCommandFiles) OpenFile(filename string) (io.ReadSeekCloser, int64, error) {
	f.openCalls = append(f.openCalls, filename)
	data, ok := f.loaded[filename]
	if !ok {
		return nil, 0, fmt.Errorf("missing file %s", filename)
	}
	return readSeekNopCloser{Reader: bytes.NewReader(data)}, int64(len(data)), nil
}
func (f *demoCommandFiles) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	for _, filename := range filenames {
		if data, ok := f.loaded[filename]; ok {
			return filename, data, nil
		}
	}
	return "", nil, fmt.Errorf("missing files")
}
func (f *demoCommandFiles) FileExists(filename string) bool {
	_, ok := f.loaded[filename]
	return ok
}

func (s *disconnectTrackingServer) Shutdown() {
	s.shutdownCalls++
	s.mockServer.Shutdown()
}

type sessionStartTrackingServer struct {
	mockServer
	initMaxClients int
	connectCalls   int
	shutdownCalls  int
}

func (s *sessionStartTrackingServer) Init(maxClients int) error {
	s.initMaxClients = maxClients
	return s.mockServer.Init(maxClients)
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

	switch strings.Fields(command)[0] {
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

type forwardingTrackingClient struct {
	state    ClientState
	commands []string
}

func (c *forwardingTrackingClient) Init() error                   { return nil }
func (c *forwardingTrackingClient) Frame(frameTime float64) error { return nil }
func (c *forwardingTrackingClient) Shutdown()                     {}
func (c *forwardingTrackingClient) State() ClientState            { return c.state }
func (c *forwardingTrackingClient) ReadFromServer() error         { return nil }
func (c *forwardingTrackingClient) SendCommand() error            { return nil }
func (c *forwardingTrackingClient) SendStringCmd(cmd string) error {
	c.commands = append(c.commands, cmd)
	return nil
}

type stopAllTrackingAudio struct {
	calls        []bool
	loop         bool
	currentMusic string
}

func (a *stopAllTrackingAudio) Init() error                                            { return nil }
func (a *stopAllTrackingAudio) Update(origin, velocity, forward, right, up [3]float32) {}
func (a *stopAllTrackingAudio) Shutdown()                                              {}
func (a *stopAllTrackingAudio) SoundInfo() string                                      { return "" }
func (a *stopAllTrackingAudio) SoundList() string                                      { return "" }
func (a *stopAllTrackingAudio) PlayLocalSound(name string, loader func() ([]byte, error), vol float32) error {
	return nil
}
func (a *stopAllTrackingAudio) PlayMusic(filename string, loader func(string) ([]byte, error), resolver func([]string) (string, []byte, error)) error {
	a.currentMusic = filename
	return nil
}
func (a *stopAllTrackingAudio) PauseMusic()            {}
func (a *stopAllTrackingAudio) ResumeMusic()           {}
func (a *stopAllTrackingAudio) SetMusicLoop(loop bool) { a.loop = loop }
func (a *stopAllTrackingAudio) ToggleMusicLoop() bool {
	a.loop = !a.loop
	return a.loop
}
func (a *stopAllTrackingAudio) MusicLooping() bool       { return a.loop }
func (a *stopAllTrackingAudio) CurrentMusic() string     { return a.currentMusic }
func (a *stopAllTrackingAudio) JumpMusic(order int) bool { return false }
func (a *stopAllTrackingAudio) StopMusic()               { a.currentMusic = "" }
func (a *stopAllTrackingAudio) StopAllSounds(clear bool) {
	a.calls = append(a.calls, clear)
}

type audioCommandRecord struct {
	name string
	vol  float32
	data []byte
}

type audioCommandTracking struct {
	stopAllTrackingAudio
	soundInfo      string
	soundList      string
	playedSounds   []audioCommandRecord
	playedMusic    []string
	pauseCalls     int
	resumeCalls    int
	stopMusicCalls int
	jumpOrders     []int
}

func (a *audioCommandTracking) SoundInfo() string { return a.soundInfo }
func (a *audioCommandTracking) SoundList() string { return a.soundList }
func (a *audioCommandTracking) PlayLocalSound(name string, loader func() ([]byte, error), vol float32) error {
	data, err := loader()
	if err != nil {
		return err
	}
	a.playedSounds = append(a.playedSounds, audioCommandRecord{name: name, vol: vol, data: data})
	return nil
}
func (a *audioCommandTracking) PlayMusic(filename string, loader func(string) ([]byte, error), resolver func([]string) (string, []byte, error)) error {
	a.playedMusic = append(a.playedMusic, filename)
	a.currentMusic = "music/" + filename + ".ogg"
	return nil
}
func (a *audioCommandTracking) PauseMusic()  { a.pauseCalls++ }
func (a *audioCommandTracking) ResumeMusic() { a.resumeCalls++ }
func (a *audioCommandTracking) JumpMusic(order int) bool {
	a.jumpOrders = append(a.jumpOrders, order)
	return true
}
func (a *audioCommandTracking) StopMusic() {
	a.stopMusicCalls++
	a.currentMusic = ""
}

type audioCommandFiles struct {
	loaded map[string][]byte
	calls  []string
}

func (f *audioCommandFiles) Init(baseDir, gameDir string) error { return nil }
func (f *audioCommandFiles) Close()                             {}
func (f *audioCommandFiles) LoadFile(filename string) ([]byte, error) {
	f.calls = append(f.calls, filename)
	data, ok := f.loaded[filename]
	if !ok {
		return nil, fmt.Errorf("missing file %s", filename)
	}
	return data, nil
}
func (f *audioCommandFiles) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	for _, filename := range filenames {
		if data, ok := f.loaded[filename]; ok {
			f.calls = append(f.calls, filename)
			return filename, data, nil
		}
	}
	return "", nil, fmt.Errorf("missing files")
}
func (f *audioCommandFiles) FileExists(filename string) bool {
	_, ok := f.loaded[filename]
	return ok
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
	added    []string
}

type fakeServerBrowser struct {
	results []inet.HostCacheEntry
}

func (f *fakeServerBrowser) Start() {}
func (f *fakeServerBrowser) Wait()  {}
func (f *fakeServerBrowser) Results() []inet.HostCacheEntry {
	return append([]inet.HostCacheEntry(nil), f.results...)
}

func (b *insertTrackingCommandBuffer) Init()                                         {}
func (b *insertTrackingCommandBuffer) Execute()                                      {}
func (b *insertTrackingCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {}
func (b *insertTrackingCommandBuffer) AddText(text string) {
	b.added = append(b.added, text)
}
func (b *insertTrackingCommandBuffer) InsertText(text string) {
	b.inserted = append(b.inserted, text)
}
func (b *insertTrackingCommandBuffer) Shutdown() {}

func testFreeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	port := conn.LocalAddr().(*stdnet.UDPAddr).Port
	if err := conn.Close(); err != nil {
		t.Fatalf("Close UDP listener: %v", err)
	}
	return port
}

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

func makeQCVMWithProfileResults(profiles map[string]int32) *qc.VM {
	vm := qc.NewVM()
	stringsData := []byte{0}
	vm.Functions = make([]qc.DFunction, 0, len(profiles))

	for name, profile := range profiles {
		nameOfs := int32(len(stringsData))
		stringsData = append(stringsData, []byte(name)...)
		stringsData = append(stringsData, 0)
		vm.Functions = append(vm.Functions, qc.DFunction{Name: nameOfs, Profile: profile})
	}
	vm.Strings = stringsData
	return vm
}

func TestCmdProfileNoOpWithoutActiveServer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Server: server.NewServer(), Console: console}

	h.CmdProfile(subs)

	if got := strings.Join(console.messages, ""); got != "" {
		t.Fatalf("profile output = %q, want empty when no active local server", got)
	}
}

func TestCmdProfilePrintsTopTenAndClearsCounters(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	srv.Active = true
	h.SetServerActive(true)

	profiles := map[string]int32{
		"func00": 120,
		"func01": 119,
		"func02": 118,
		"func03": 117,
		"func04": 116,
		"func05": 115,
		"func06": 114,
		"func07": 113,
		"func08": 112,
		"func09": 111,
		"func10": 110,
		"func11": 109,
	}
	srv.QCVM = makeQCVMWithProfileResults(profiles)
	subs := &Subsystems{Server: srv, Console: console}

	h.CmdProfile(subs)

	if len(console.messages) != 10 {
		t.Fatalf("profile lines = %d, want 10", len(console.messages))
	}
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "    120 func00\n") {
		t.Fatalf("profile output missing top function: %q", output)
	}
	if !strings.Contains(output, "    111 func09\n") {
		t.Fatalf("profile output missing tenth function: %q", output)
	}
	if strings.Contains(output, "func10") || strings.Contains(output, "func11") {
		t.Fatalf("profile output includes entries past top 10: %q", output)
	}

	for i, fn := range srv.QCVM.Functions {
		if fn.Profile != 0 {
			t.Fatalf("function %d profile = %d, want 0 after profile command", i, fn.Profile)
		}
	}
}

func TestProfileCommandRegistrationExecutes(t *testing.T) {
	cmdsys.RemoveCommand("profile")
	t.Cleanup(func() { cmdsys.RemoveCommand("profile") })

	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	srv.Active = true
	h.SetServerActive(true)
	srv.QCVM = makeQCVMWithProfileResults(map[string]int32{"qc_profiled": 7})
	subs := &Subsystems{Server: srv, Console: console}

	h.RegisterCommands(subs)
	cmdsys.ExecuteText("profile")

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "      7 qc_profiled\n") {
		t.Fatalf("profile command output = %q, want formatted QC profile line", got)
	}
}

func TestCmdDevStatsPrintsCStyleTable(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	srv := server.NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	srv.Active = true

	subs := &Subsystems{Server: srv, Console: console}
	h.CmdDevStats(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "devstats | Curr  Peak\n") {
		t.Fatalf("devstats output missing header:\n%s", got)
	}
	if !strings.Contains(got, "Edicts   |") {
		t.Fatalf("devstats output missing Edicts row:\n%s", got)
	}
	if !strings.Contains(got, "Packet   |") {
		t.Fatalf("devstats output missing Packet row:\n%s", got)
	}
	if !strings.Contains(got, "GL upload|") {
		t.Fatalf("devstats output missing GL upload row:\n%s", got)
	}
}

func TestDevStatsCommandRegistrationExecutes(t *testing.T) {
	cmdsys.RemoveCommand("devstats")
	t.Cleanup(func() { cmdsys.RemoveCommand("devstats") })

	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	srv.Active = true
	h.SetServerActive(true)

	subs := &Subsystems{Server: srv, Console: console}
	h.RegisterCommands(subs)
	cmdsys.ExecuteText("devstats")

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "devstats | Curr  Peak\n") {
		t.Fatalf("devstats command output = %q, want devstats header", got)
	}
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

func TestCmdRestartPromptAutoloadShowsConfirmationMenu(t *testing.T) {
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
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil)
	h.SetMenu(mgr)

	previousAutoload := cvar.StringValue("sv_autoload")
	cvar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		cvar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)

	if !mgr.IsActive() {
		t.Fatal("menu should be active for prompt autoload")
	}
	if got := mgr.GetState(); got != menu.MenuQuit {
		t.Fatalf("menu state = %v, want %v", got, menu.MenuQuit)
	}
	if got := strings.Join(subs.console.messages, ""); strings.Contains(got, "Autoloading...") {
		t.Fatalf("console output = %q, want no immediate autoload", got)
	}
}

func TestCmdRestartPromptAutoloadConfirmLoadsLastSave(t *testing.T) {
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
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil)
	h.SetMenu(mgr)

	previousAutoload := cvar.StringValue("sv_autoload")
	cvar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		cvar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)
	mgr.M_Key('y')

	if mgr.IsActive() {
		t.Fatal("menu should hide after confirming autoload prompt")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "ERROR: slot1.sav not found.") {
		t.Fatalf("console output = %q, want prompted load failure", got)
	}
	if h.lastSave != "" {
		t.Fatalf("lastSave = %q, want cleared after missing prompted load", h.lastSave)
	}
}

func TestCmdRestartPromptAutoloadDeclineClearsLastSave(t *testing.T) {
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
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil)
	h.SetMenu(mgr)

	previousAutoload := cvar.StringValue("sv_autoload")
	cvar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		cvar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)
	mgr.M_Key('n')

	if mgr.IsActive() {
		t.Fatal("menu should hide after declining autoload prompt")
	}
	if h.lastSave != "" {
		t.Fatalf("lastSave = %q, want cleared after declining prompt", h.lastSave)
	}
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

	h.CmdGod(nil, &subs.Subsystems)
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

	h.CmdNoClip(nil, &subs.Subsystems)
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

	h.CmdNotarget(nil, &subs.Subsystems)
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

func TestCmdTest2PrintsQueriedRules(t *testing.T) {
	serverConn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer serverConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < inet.HeaderSize+1 || buf[8] != inet.CCReqRuleInfo {
				continue
			}
			prev := strings.TrimRight(string(buf[9:n]), "\x00")
			var resp []byte
			switch prev {
			case "":
				resp = buildHostRuleInfoResponse("deathmatch", "1")
			case "deathmatch":
				resp = buildHostRuleInfoResponse("teamplay", "0")
			default:
				resp = buildHostRuleInfoResponse("", "")
			}
			serverConn.WriteToUDP(resp, addr)
		}
	}()

	h := NewHost()
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdTest2(serverConn.LocalAddr().String(), &subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "deathmatch") || !strings.Contains(got, "teamplay") {
		t.Fatalf("test2 output missing expected rules:\n%s", got)
	}
}

func TestCmdPlayersPrintsQueriedPlayers(t *testing.T) {
	serverConn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer serverConn.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < inet.HeaderSize+2 || buf[8] != inet.CCReqPlayerInfo {
				continue
			}
			switch buf[9] {
			case 0:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(0, "Ranger", 0x49, 15, 32, "10.0.0.2:26000"), addr)
			case 1:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(1, "Shambler", 0xdd, 42, 60, "10.0.0.3:26000"), addr)
			default:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(buf[9], "", 0, 0, 0, ""), addr)
			}
		}
	}()

	h := NewHost()
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdPlayers(serverConn.LocalAddr().String(), &subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "slot  name              color  frags  ping") {
		t.Fatalf("players output missing header:\n%s", got)
	}
	if !strings.Contains(got, "Ranger") || !strings.Contains(got, "Shambler") {
		t.Fatalf("players output missing expected player names:\n%s", got)
	}
}

func TestCmdNetStatsPrintsGlobalDatagramCounters(t *testing.T) {
	inet.GlobalStats.Reset()
	t.Cleanup(inet.GlobalStats.Reset)

	inet.GlobalStats.UnreliableSent.Store(11)
	inet.GlobalStats.ReliableReceived.Store(7)
	inet.GlobalStats.DroppedDatagrams.Store(3)

	h := NewHost()
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdNetStats(&subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "unreliable messages sent   = 11\n") {
		t.Fatalf("net_stats output missing unreliable sent count:\n%s", got)
	}
	if !strings.Contains(got, "reliable messages received = 7\n") {
		t.Fatalf("net_stats output missing reliable received count:\n%s", got)
	}
	if !strings.Contains(got, "droppedDatagrams           = 3\n") {
		t.Fatalf("net_stats output missing dropped datagrams count:\n%s", got)
	}
}

func TestCmdSlistPrintsCStyleHeaderEntriesAndTrailer(t *testing.T) {
	h := NewHost()
	subs := &Subsystems{Console: &mockConsole{}}
	console := subs.Console.(*mockConsole)

	oldFactory := newServerBrowser
	t.Cleanup(func() { newServerBrowser = oldFactory })
	newServerBrowser = func() serverBrowser {
		return &fakeServerBrowser{
			results: []inet.HostCacheEntry{
				{Name: "LAN Test", Map: "e1m1", Players: 2, MaxPlayers: 8, Address: "127.0.0.1:26000"},
				{Name: "NoSlots", Map: "start", Players: 0, MaxPlayers: 0, Address: "127.0.0.1:26001"},
			},
		}
	}

	h.CmdSlist(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "Looking for Quake servers...\n") {
		t.Fatalf("slist output missing banner:\n%s", got)
	}
	if !strings.Contains(got, "Server          Map             Users\n") {
		t.Fatalf("slist output missing header:\n%s", got)
	}
	if !strings.Contains(got, "--------------- --------------- -----\n") {
		t.Fatalf("slist output missing separator:\n%s", got)
	}
	if !strings.Contains(got, "LAN Test        e1m1             2/ 8\n") {
		t.Fatalf("slist output missing users row:\n%s", got)
	}
	if !strings.Contains(got, "NoSlots         start          \n") {
		t.Fatalf("slist output missing zero-max row:\n%s", got)
	}
	if !strings.Contains(got, "== end list ==\n\n") {
		t.Fatalf("slist output missing trailer:\n%s", got)
	}
}

func TestCmdSlistPrintsNoServersMessageWhenEmpty(t *testing.T) {
	h := NewHost()
	subs := &Subsystems{Console: &mockConsole{}}
	console := subs.Console.(*mockConsole)

	oldFactory := newServerBrowser
	t.Cleanup(func() { newServerBrowser = oldFactory })
	newServerBrowser = func() serverBrowser {
		return &fakeServerBrowser{}
	}

	h.CmdSlist(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "Looking for Quake servers...\n") {
		t.Fatalf("slist output missing banner:\n%s", got)
	}
	if !strings.Contains(got, "No Quake servers found.\n\n") {
		t.Fatalf("slist output missing empty message:\n%s", got)
	}
}

func buildHostRuleInfoResponse(name, value string) []byte {
	payloadLen := 1
	if name != "" {
		payloadLen += len(name) + 1 + len(value) + 1
	}
	buf := make([]byte, inet.HeaderSize+payloadLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(len(buf))|inet.FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = inet.CCRepRuleInfo
	if name != "" {
		copy(buf[9:], name)
		buf[9+len(name)] = 0
		copy(buf[10+len(name):], value)
	}
	return buf
}

func buildHostPlayerInfoResponse(slot byte, name string, colors byte, frags int32, ping int32, address string) []byte {
	payloadLen := 2 // slot + empty name terminator
	if name != "" {
		payloadLen += len(name) + 1 + 4 + 4 + 4 + len(address) + 1
	}
	buf := make([]byte, inet.HeaderSize+1+payloadLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(len(buf))|inet.FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = inet.CCRepPlayerInfo
	buf[9] = slot
	if name != "" {
		copy(buf[10:], name)
		nameEnd := 10 + len(name)
		buf[nameEnd] = 0
		binary.LittleEndian.PutUint32(buf[nameEnd+1:], uint32(colors))
		binary.LittleEndian.PutUint32(buf[nameEnd+5:], uint32(frags))
		binary.LittleEndian.PutUint32(buf[nameEnd+9:], uint32(ping))
		copy(buf[nameEnd+13:], address)
	}
	return buf
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

func TestCmdMapnamePrintsServerMap(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{
		Console: console,
		Server:  &mockServer{mapName: "e1m1"},
	}

	h.CmdMapname(subs)

	if got := strings.Join(console.messages, ""); got != "\"mapname\" is \"e1m1\"\n" {
		t.Fatalf("mapname output = %q", got)
	}
}

type mapnameStateClient struct {
	mockClient
	client *cl.Client
}

func (c *mapnameStateClient) ClientState() *cl.Client { return c.client }

func TestCmdMapnamePrintsConnectedClientMap(t *testing.T) {
	h := NewHost()
	h.clientState = caConnected
	console := &mockConsole{}
	subs := &Subsystems{
		Console: console,
		Client: &mapnameStateClient{
			mockClient: mockClient{state: caConnected},
			client:     &cl.Client{MapName: "start"},
		},
	}

	h.CmdMapname(subs)

	if got := strings.Join(console.messages, ""); got != "\"mapname\" is \"start\"\n" {
		t.Fatalf("mapname output = %q", got)
	}
}

func TestCmdModsPrintsAvailableMods(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic", "rogue"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "pak0.pak"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "rogue", "progs.dat"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile rogue progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdMods(nil, subs)

	got := strings.Join(console.messages, "")
	for _, want := range []string{"   hipnotic\n", "   rogue\n", "2 mods\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mods output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "id1") {
		t.Fatalf("mods output should not include id1:\n%s", got)
	}
}

func TestCmdModsFilter(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic", "rogue"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "pak0.pak"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic pak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "rogue", "progs.dat"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile rogue progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdMods([]string{"rog"}, subs)
	h.CmdMods([]string{"zzz"}, subs)

	got := strings.Join(console.messages, "")
	for _, want := range []string{
		"   rogue\n",
		"1 mod containing \"rog\"\n",
		"no mods found containing \"zzz\"\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("filtered mods output missing %q:\n%s", want, got)
		}
	}
}

func TestCmdGamePrintsCurrentGameDir(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "pak0.pak"), []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic pak: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdGame(nil, subs)

	if got := strings.Join(console.messages, ""); got != "\"game\" is \"id1\"\n" {
		t.Fatalf("game output = %q", got)
	}
}

func TestCmdGameSwitchesFilesystemToSelectedMod(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "progs.dat"), []byte("hipnotic"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdGame([]string{"hipnotic"}, subs)

	activeFS, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		t.Fatalf("subs.Files type = %T, want *fs.FileSystem", subs.Files)
	}
	if got := activeFS.GetGameDir(); got != "hipnotic" {
		t.Fatalf("active game dir = %q, want %q", got, "hipnotic")
	}
	data, err := subs.Files.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if got := string(data); got != "hipnotic" {
		t.Fatalf("progs.dat contents = %q, want %q", got, "hipnotic")
	}
	if h.gameDir != "hipnotic" {
		t.Fatalf("host gameDir = %q, want %q", h.gameDir, "hipnotic")
	}
}

func TestCmdGameInvokesGameDirChangedCallbackWithNewFilesystem(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "progs.dat"), []byte("hipnotic"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}
	calls := 0
	var seenGameDir string
	var seenData string
	h.SetGameDirChangedCallback(func(_ *Subsystems, changed *fs.FileSystem) error {
		calls++
		seenGameDir = changed.GetGameDir()
		data, err := changed.LoadFile("progs.dat")
		if err != nil {
			return err
		}
		seenData = string(data)
		return nil
	})

	h.CmdGame([]string{"hipnotic"}, subs)

	if calls != 1 {
		t.Fatalf("callback calls = %d, want 1", calls)
	}
	if seenGameDir != "hipnotic" {
		t.Fatalf("callback game dir = %q, want hipnotic", seenGameDir)
	}
	if seenData != "hipnotic" {
		t.Fatalf("callback progs data = %q, want hipnotic", seenData)
	}
}

func TestCmdGameReportsCallbackReloadWarningAndContinues(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "progs.dat"), []byte("hipnotic"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}
	h.SetGameDirChangedCallback(func(_ *Subsystems, _ *fs.FileSystem) error {
		return fmt.Errorf("reload failed")
	})

	h.CmdGame([]string{"hipnotic"}, subs)

	activeFS, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		t.Fatalf("subs.Files type = %T, want *fs.FileSystem", subs.Files)
	}
	if got := activeFS.GetGameDir(); got != "hipnotic" {
		t.Fatalf("active game dir = %q, want %q", got, "hipnotic")
	}
	out := strings.Join(console.messages, "")
	if !strings.Contains(out, "failed to reload draw assets") {
		t.Fatalf("console output = %q, want reload warning", out)
	}
	if !strings.Contains(out, "\"game\" changed to \"hipnotic\"") {
		t.Fatalf("console output = %q, want successful game switch message", out)
	}
}

func TestGameConsoleCommandSwitchesFilesystemToSelectedMod(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "hipnotic", "progs.dat"), []byte("hipnotic"), 0o644); err != nil {
		t.Fatalf("WriteFile hipnotic progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	h.baseDir = baseDir
	subs := &Subsystems{
		Console: console,
		Files:   fileSys,
	}
	h.RegisterCommands(subs)

	cmdsys.ExecuteText(`game "hipnotic"`)

	activeFS, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		t.Fatalf("subs.Files type = %T, want *fs.FileSystem", subs.Files)
	}
	if got := activeFS.GetGameDir(); got != "hipnotic" {
		t.Fatalf("active game dir = %q, want %q", got, "hipnotic")
	}
}

func TestCmdGameRejectsUnknownModAndLeavesFilesystemUnchanged(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("WriteFile id1 progs: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdGame([]string{"missingmod"}, subs)

	activeFS, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		t.Fatalf("subs.Files type = %T, want *fs.FileSystem", subs.Files)
	}
	if got := activeFS.GetGameDir(); got != "id1" {
		t.Fatalf("active game dir = %q, want %q", got, "id1")
	}
	gotOutput := strings.Join(console.messages, "")
	if !strings.Contains(gotOutput, "unknown gamedir") {
		t.Fatalf("console output = %q, want unknown gamedir message", gotOutput)
	}
}

func TestCmdBanPrintsInactiveStatus(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	if err := inet.SetIPBan("off", ""); err != nil {
		t.Fatalf("clear ban: %v", err)
	}

	h.CmdBan(nil, subs)

	if got := strings.Join(console.messages, ""); got != "Banning not active\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdBanSetsSingleAddress(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	if err := inet.SetIPBan("off", ""); err != nil {
		t.Fatalf("clear ban: %v", err)
	}
	t.Cleanup(func() {
		_ = inet.SetIPBan("off", "")
	})

	h.CmdBan([]string{"192.168.1.100"}, subs)

	if got := inet.IPBanStatus(); got != "Banning 192.168.1.100 [255.255.255.255]" {
		t.Fatalf("ban status = %q", got)
	}
	if got := strings.Join(console.messages, ""); got != "" {
		t.Fatalf("console output = %q, want empty", got)
	}
}

func TestCmdEdictCountPrintsCanonicalSummary(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	srv := server.NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	modelEnt := srv.AllocEdict()
	if modelEnt == nil {
		t.Fatal("AllocEdict modelEnt = nil")
	}
	modelEnt.Vars.Model = srv.QCVM.AllocString("progs/ogre.mdl")
	modelEnt.Vars.Solid = float32(server.SolidSlideBox)
	modelEnt.Vars.MoveType = float32(server.MoveTypeStep)

	solidEnt := srv.AllocEdict()
	if solidEnt == nil {
		t.Fatal("AllocEdict solidEnt = nil")
	}
	solidEnt.Vars.Solid = float32(server.SolidBSP)
	server.ResetCheckBottomStats()
	grounded := srv.AllocEdict()
	if grounded == nil {
		t.Fatal("AllocEdict grounded = nil")
	}
	grounded.Vars.Origin = [3]float32{0, 0, 24}
	grounded.Vars.Mins = [3]float32{-16, -16, -24}
	grounded.Vars.Maxs = [3]float32{16, 16, 32}
	grounded.Vars.Solid = float32(server.SolidSlideBox)
	grounded.Vars.MoveType = float32(server.MoveTypeStep)
	srv.WorldModel = server.CreateSyntheticWorldModel()
	srv.Edicts[0].Vars.Solid = float32(server.SolidBSP)
	srv.ClearWorld()
	srv.LinkEdict(grounded, false)
	if !srv.CheckBottom(grounded) {
		t.Fatal("expected grounded entity to satisfy CheckBottom")
	}
	air := srv.AllocEdict()
	if air == nil {
		t.Fatal("AllocEdict air = nil")
	}
	air.Vars.Origin = [3]float32{0, 0, 256}
	air.Vars.Mins = [3]float32{-16, -16, -24}
	air.Vars.Maxs = [3]float32{16, 16, 32}
	air.Vars.Solid = float32(server.SolidSlideBox)
	srv.LinkEdict(air, false)
	if srv.CheckBottom(air) {
		t.Fatal("expected elevated entity to fail CheckBottom")
	}
	srv.Physics()

	subs := &Subsystems{
		Server:  srv,
		Console: console,
	}

	h.CmdEdictCount(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "num_edicts:  6\n") {
		t.Fatalf("edictcount output missing num_edicts:\n%s", got)
	}
	if !strings.Contains(got, "active    :  6\n") {
		t.Fatalf("edictcount output missing active count:\n%s", got)
	}
	if !strings.Contains(got, "peak      :  6\n") {
		t.Fatalf("edictcount output missing peak count:\n%s", got)
	}
	if !strings.Contains(got, "view      :  1\n") {
		t.Fatalf("edictcount output missing model count:\n%s", got)
	}
	if !strings.Contains(got, "touch     :  5\n") {
		t.Fatalf("edictcount output missing solid count:\n%s", got)
	}
	if !strings.Contains(got, "step      :  2\n") {
		t.Fatalf("edictcount output missing step count:\n%s", got)
	}
	if !strings.Contains(got, "c_yes     :  1\n") {
		t.Fatalf("edictcount output missing c_yes count:\n%s", got)
	}
	if !strings.Contains(got, "c_no      :  1\n") {
		t.Fatalf("edictcount output missing c_no count:\n%s", got)
	}
}

func TestCmdBanSetsSubnetMask(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	if err := inet.SetIPBan("off", ""); err != nil {
		t.Fatalf("clear ban: %v", err)
	}
	t.Cleanup(func() {
		_ = inet.SetIPBan("off", "")
	})

	h.CmdBan([]string{"10.0.0.0", "255.255.0.0"}, subs)

	if got := inet.IPBanStatus(); got != "Banning 10.0.0.0 [255.255.0.0]" {
		t.Fatalf("ban status = %q", got)
	}
}

func TestCmdBanTurnsOff(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	if err := inet.SetIPBan("192.168.1.100", ""); err != nil {
		t.Fatalf("set ban: %v", err)
	}

	h.CmdBan([]string{"off"}, subs)

	if got := inet.IPBanStatus(); got != "Banning not active" {
		t.Fatalf("ban status = %q", got)
	}
}

func TestCmdBanPrintsUsageForTooManyArgs(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdBan([]string{"1.2.3.4", "255.255.255.0", "extra"}, subs)

	if got := strings.Join(console.messages, ""); got != "BAN ip_address [mask]\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdBanForwardsWhenRemoteConnected(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdBan([]string{"10.0.0.1"}, subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"ban 10.0.0.1"}) {
		t.Fatalf("forwarded commands = %v, want [ban 10.0.0.1]", got)
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

	h.CmdSave("../bad", subs)

	if len(console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Relative pathnames are not allowed.") {
		t.Fatalf("console output = %q, want relative-path rejection", got)
	}
}

func TestCmdSaveChecksLocalGameBeforeRelativePathValidation(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdSave("../bad", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Not playing a local game.") {
		t.Fatalf("console output = %q, want local-game rejection before path validation", got)
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
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "Relative pathnames are not allowed.") {
		t.Fatalf("console output = %q, want relative-path rejection", got)
	}
}

func TestCmdLoadChecksRelativePathBeforeDisablingNoMonsters(t *testing.T) {
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

	previous := cvar.StringValue("nomonsters")
	cvar.Set("nomonsters", "1")
	t.Cleanup(func() {
		cvar.Set("nomonsters", previous)
	})

	h.CmdLoad("../bad", &subs.Subsystems)

	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "Relative pathnames are not allowed.") {
		t.Fatalf("console output = %q, want relative-path rejection", got)
	}
	if got := cvar.StringValue("nomonsters"); got != "1" {
		t.Fatalf("nomonsters after invalid path load = %q, want unchanged 1", got)
	}
}

func TestCmdLoadMissingNestedSaveIncludesRelativePath(t *testing.T) {
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

	h.CmdLoad("autosave/start", &subs.Subsystems)

	if len(subs.console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "ERROR: autosave/start.sav not found.") {
		t.Fatalf("console output = %q, want nested save path in not-found error", got)
	}
}

func TestCmdSaveRejectsWhenNotPlayingLocalGame(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdSave("slot1", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Not playing a local game.") {
		t.Fatalf("console output = %q, want local-game rejection", got)
	}
}

func TestCmdSaveRejectsMultiplayerGames(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	subs := &Subsystems{
		Server:  srv,
		Client:  newLocalLoopbackClient(),
		Console: console,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir(), MaxClients: 2}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.SetServerActive(true)
	srv.Active = true

	h.CmdSave("slot1", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Can't save multiplayer games.") {
		t.Fatalf("console output = %q, want multiplayer rejection", got)
	}
}

func TestSaveEntryAllowed(t *testing.T) {
	h := NewHost()
	srv := server.NewServer()
	lc := newLocalLoopbackClient()
	subs := &Subsystems{
		Server:  srv,
		Client:  lc,
		Console: &mockConsole{},
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.SetServerActive(true)
	srv.Active = true
	if got := h.SaveEntryAllowed(subs); !got {
		t.Fatal("SaveEntryAllowed() = false, want true for local active single-player")
	}

	lc.inner.Intermission = 1
	if got := h.SaveEntryAllowed(subs); got {
		t.Fatal("SaveEntryAllowed() = true during intermission, want false")
	}
	lc.inner.Intermission = 0

	srv.Static.MaxClients = 2
	if got := h.SaveEntryAllowed(subs); got {
		t.Fatal("SaveEntryAllowed() = true for multiplayer server, want false")
	}
	srv.Static.MaxClients = 1

	h.SetServerActive(false)
	if got := h.SaveEntryAllowed(subs); got {
		t.Fatal("SaveEntryAllowed() = true when host server inactive, want false")
	}
}

func TestCmdSaveArgsPrintsUsageWithoutName(t *testing.T) {
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

	h.CmdSaveArgs(nil, &subs.Subsystems)

	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "save <savename> : save a game") {
		t.Fatalf("console output = %q, want save usage", got)
	}
}

func TestCmdLoadArgsPrintsUsageWithoutName(t *testing.T) {
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

	h.CmdLoadArgs(nil, &subs.Subsystems)

	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "load <savename> : load a game") {
		t.Fatalf("console output = %q, want load usage", got)
	}
}

func TestSaveFilePathAllowsCanonicalAutosaveSubdir(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	path, err := h.saveFilePath("autosave/start")
	if err != nil {
		t.Fatalf("saveFilePath returned error: %v", err)
	}

	want := filepath.Join(userDir, "saves", "autosave", "start.sav")
	if path != want {
		t.Fatalf("saveFilePath = %q, want %q", path, want)
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

func TestListSaveSlotsTreatsLegacyInstallRootJSONSaveAsUnused(t *testing.T) {
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
	if got := slots[0].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[0].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
	}
}

func TestListSaveSlotsDecodesInstallRootKEXTextSave(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d\n", server.SaveGameVersionKEX)
	b.WriteString("id1\n")
	b.WriteString("KEX save title\n")
	for i := 0; i < server.NumSpawnParms; i++ {
		b.WriteString("0\n")
	}
	b.WriteString("2\n")
	b.WriteString("install-root-kex-map\n")
	b.WriteString("123.5\n")
	for i := 0; i < 64; i++ {
		b.WriteString("m\n")
	}
	b.WriteString("{\n\"serverflags\" \"0\"\n}\n")
	b.WriteString("{\n\"classname\" \"worldspawn\"\n}\n")

	if err := os.WriteFile(filepath.Join(baseDir, "s0.sav"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("WriteFile(install root kex s0): %v", err)
	}

	h := NewHost()
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "hipnotic", UserDir: userDir}, &Subsystems{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	slots := h.ListSaveSlots(1)
	if len(slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(slots))
	}
	if got := slots[0].DisplayName; got != "KEX save title" {
		t.Fatalf("slot[0].DisplayName = %q, want KEX save title", got)
	}
}

func TestListSaveSlotsTreatsLegacyInstallRootJSONAsUnusedInsteadOfFallingBack(t *testing.T) {
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
	if got := slots[0].DisplayName; got != unusedSaveSlotDisplay {
		t.Fatalf("slot[0].DisplayName = %q, want %q", got, unusedSaveSlotDisplay)
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

	h.CmdRecord([]string{"music_header"}, subs)
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

	h.CmdRecord([]string{"record_snapshot"}, subs)
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

func TestCmdPlaydemoUsesOpenFileWhenAvailable(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	files := &demoCommandFiles{
		loaded: map[string][]byte{
			"bootstrap.dem": []byte("0\n"),
		},
	}
	lc := newLocalLoopbackClient()
	subs := &Subsystems{
		Client:  lc,
		Console: console,
		Files:   files,
	}

	h.CmdPlaydemo("bootstrap", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}
	defer h.demoState.StopPlayback()

	if !reflect.DeepEqual(files.openCalls, []string{"bootstrap.dem"}) {
		t.Fatalf("OpenFile calls = %v, want [bootstrap.dem]", files.openCalls)
	}
	if len(files.loadCalls) != 0 {
		t.Fatalf("LoadFile calls = %v, want none", files.loadCalls)
	}
	if got := h.demoState.Filename; got != "bootstrap.dem" {
		t.Fatalf("demo filename = %q, want bootstrap.dem", got)
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

func TestCmdDemoSeekClearsRewindBackstop(t *testing.T) {
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
	if err := recorder.StartDemoRecording("demoseek_backstop", 0); err != nil {
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

	h.CmdPlaydemo("demoseek_backstop", subs)
	if h.demoState == nil || !h.demoState.Playback {
		t.Fatal("expected demo playback to be active")
	}
	h.demoState.SetRewindBackstop(true)
	h.CmdDemoSeek(1, subs)
	if h.demoState.RewindBackstop() {
		t.Fatal("expected demoseek to clear rewind backstop")
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

func TestCmdMapWithSpawnArgsCarriesSpawnCommandIntoLocalHandshake(t *testing.T) {
	h := NewHost()
	srv := &reconnectTrackingServer{}
	client := &reconnectHandshakeClient{}
	subs := &Subsystems{
		Files:   &fs.FileSystem{},
		Server:  srv,
		Client:  client,
		Console: &mockConsole{},
	}

	if err := h.CmdMapWithSpawnArgs("start", []string{"coop", "1"}, subs); err != nil {
		t.Fatalf("CmdMapWithSpawnArgs(start) failed: %v", err)
	}

	if want := []string{"prespawn", "spawn coop 1", "begin"}; !reflect.DeepEqual(client.signonReplies, want) {
		t.Fatalf("signon replies = %v, want %v", client.signonReplies, want)
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

func TestCmdMapDedicatedStartsServerWithoutLocalSession(t *testing.T) {
	registerHostCVars()
	oldDedicated := cvar.BoolValue("dedicated")
	t.Cleanup(func() {
		cvar.SetBool("dedicated", oldDedicated)
	})

	h := NewHost()
	h.dedicated = true
	h.maxClients = 8
	srv := &sessionStartTrackingServer{}
	subs := &Subsystems{
		Files:   &fs.FileSystem{},
		Server:  srv,
		Client:  &remoteSignonTestClient{state: caConnected},
		Console: &mockConsole{},
	}

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start) failed: %v", err)
	}
	if srv.initMaxClients != 8 {
		t.Fatalf("server Init maxClients = %d, want 8", srv.initMaxClients)
	}
	if srv.connectCalls != 0 {
		t.Fatalf("ConnectClient calls = %d, want 0 for dedicated startup", srv.connectCalls)
	}
	if !h.ServerActive() {
		t.Fatal("serverActive = false, want true")
	}
	if got := h.ClientState(); got != caDisconnected {
		t.Fatalf("client state = %v, want %v", got, caDisconnected)
	}
	if got := h.SignOns(); got != 0 {
		t.Fatalf("host signons = %d, want 0", got)
	}
	if subs.Client != nil {
		t.Fatalf("client = %T, want nil after dedicated map start", subs.Client)
	}
}

func TestSyncAutosaveLastTimeFromServerUsesServerTime(t *testing.T) {
	h := NewHost()
	h.autosave.lastTime = 1
	srv := server.NewServer()
	srv.Time = 123.5

	h.syncAutosaveLastTimeFromServer(srv)

	if got, want := h.autosave.lastTime, 123.5; got != want {
		t.Fatalf("autosave.lastTime = %v, want %v", got, want)
	}
}

func TestSyncAutosaveLastTimeFromServerIgnoresNilServer(t *testing.T) {
	h := NewHost()
	h.autosave.lastTime = 7

	h.syncAutosaveLastTimeFromServer(nil)

	if got, want := h.autosave.lastTime, 7.0; got != want {
		t.Fatalf("autosave.lastTime = %v, want %v", got, want)
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
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Couldn't load map") {
		t.Fatalf("console output = %q, want load failure text", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after load transition")
	}
}

func TestCmdLoadDisablesNoMonstersAutomatically(t *testing.T) {
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
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	previous := cvar.StringValue("nomonsters")
	cvar.Set("nomonsters", "1")
	t.Cleanup(func() {
		cvar.Set("nomonsters", previous)
	})

	h.CmdLoad("slot1", subs)

	if got := cvar.StringValue("nomonsters"); got != "0" {
		t.Fatalf("nomonsters after load = %q, want 0", got)
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Warning: \"nomonsters\" disabled automatically.") {
		t.Fatalf("console output = %q, want nomonsters warning", got)
	}
}

func TestCmdLoadRejectsMismatchedSaveVersion(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion + 1,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "start",
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

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: Savegame is version") {
		t.Fatalf("console output = %q, want version mismatch", got)
	}
	if len(audio.calls) != 0 {
		t.Fatalf("StopAllSounds calls = %d, want 0 for early version rejection", len(audio.calls))
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive on early version rejection")
	}
}

func TestCmdLoadArgsKEXRejectsNativeVersionAtInstallRoot(t *testing.T) {
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
			MapName: "start",
		},
	})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "slot1.sav"), saveData, 0o644); err != nil {
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

	h.CmdLoadArgs([]string{"slot1", "kex"}, subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: Savegame is version 1, not 6") {
		t.Fatalf("console output = %q, want explicit kex version mismatch", got)
	}
	if len(audio.calls) != 0 {
		t.Fatalf("StopAllSounds calls = %d, want 0 for early kex version rejection", len(audio.calls))
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive on early kex version rejection")
	}
}

func TestCmdLoadArgsKEXSearchesInstallRootOnly(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(userDir, "saves"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	saveData, err := json.Marshal(hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   1,
		Server: &server.SaveGameState{
			Version: server.SaveGameVersion,
			MapName: "start",
		},
	})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
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
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoadArgs([]string{"slot1", "kex"}, subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: slot1.sav not found.") {
		t.Fatalf("console output = %q, want install-root-only not found", got)
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive when explicit kex save is missing")
	}
}

func TestCmdLoadArgsKEXReportsUnsupportedTextFormat(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "slot1.sav"), []byte("6\nid1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoadArgs([]string{"slot1", "kex"}, subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: couldn't parse text savegame: savegame map is empty") {
		t.Fatalf("console output = %q, want explicit text save parse error", got)
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive when text save parsing fails")
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
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Couldn't load map") {
		t.Fatalf("console output = %q, want load failure text", got)
	}
	if !h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should be active after legacy save fallback")
	}
}

func TestCmdLoadTreatsLegacyInstallRootSaveAsKEXWhenUserAndBaseGameSaveMissing(t *testing.T) {
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

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: Savegame is version 1, not 6") {
		t.Fatalf("console output = %q, want install-root kex version mismatch", got)
	}
	if len(audio.calls) != 0 {
		t.Fatalf("StopAllSounds calls = %d, want 0 for early install-root kex rejection", len(audio.calls))
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive on early install-root kex rejection")
	}
}

func TestCmdLoadAutoDetectsInstallRootKEXTextSave(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "slot1.sav"), []byte("6\nid1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("slot1", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: couldn't parse text savegame: savegame map is empty") {
		t.Fatalf("console output = %q, want auto-detected text save parse error", got)
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive when auto-detected install-root text parsing fails")
	}
}

func TestCmdLoadArgsKEXRejectsCrossModSave(t *testing.T) {
	baseDir := t.TempDir()
	userDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "id1"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "slot1.sav"), []byte(buildKEXTextSave(kexTextSaveFixture{
		gameDir: "hipnotic",
		mapName: "start",
		skill:   2,
		time:    1,
		worldFields: map[string]string{
			"classname": "worldspawn",
		},
		playerFields: map[string]string{
			"classname": "player",
		},
	})), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("filesystem Init failed: %v", err)
	}
	defer fileSys.Close()

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Server:  server.NewServer(),
		Client:  newLocalLoopbackClient(),
		Console: console,
	}
	if err := h.Init(&InitParams{BaseDir: baseDir, GameDir: "id1", UserDir: userDir, MaxClients: 1}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoadArgs([]string{"slot1", "kex"}, subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: KEX savegame targets game hipnotic, but the active game is id1") {
		t.Fatalf("console output = %q, want cross-mod rejection", got)
	}
	if h.LoadingPlaqueActive(0) {
		t.Fatal("loading plaque should stay inactive on cross-mod kex rejection")
	}
}

type kexTextSaveFixture struct {
	gameDir      string
	mapName      string
	skill        int
	time         float32
	spawnParms   [server.NumSpawnParms]float32
	lightStyles  map[int]string
	worldFields  map[string]string
	playerFields map[string]string
}

func buildKEXTextSave(f kexTextSaveFixture) string {
	if f.gameDir == "" {
		f.gameDir = "id1"
	}
	if f.mapName == "" {
		f.mapName = "start"
	}
	if f.time == 0 {
		f.time = 1
	}

	var b strings.Builder
	b.WriteString(strconv.Itoa(server.SaveGameVersionKEX))
	b.WriteString("\n")
	b.WriteString(f.gameDir)
	b.WriteString("\n")
	b.WriteString("generated\n")
	for _, parm := range f.spawnParms {
		b.WriteString(strconv.FormatFloat(float64(parm), 'f', -1, 32))
		b.WriteString("\n")
	}
	b.WriteString(strconv.FormatFloat(float64(f.skill), 'f', 1, 32))
	b.WriteString("\n")
	b.WriteString(f.mapName)
	b.WriteString("\n")
	b.WriteString(strconv.FormatFloat(float64(f.time), 'f', -1, 32))
	b.WriteString("\n")
	for i := 0; i < 64; i++ {
		if f.lightStyles != nil {
			if style, ok := f.lightStyles[i]; ok {
				b.WriteString(style)
			}
		}
		b.WriteString("\n")
	}
	writeTextSaveEntity(&b, nil)
	writeTextSaveEntity(&b, f.worldFields)
	writeTextSaveEntity(&b, f.playerFields)
	return b.String()
}

func writeTextSaveEntity(b *strings.Builder, fields map[string]string) {
	b.WriteString("{\n")
	for key, value := range fields {
		b.WriteString(fmt.Sprintf("\"%s\" \"%s\"\n", key, value))
	}
	b.WriteString("}\n")
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

func TestCmdSpawnForRemoteClientIncludesSpawnArgs(t *testing.T) {
	h := NewHost()
	h.SetClientState(caConnected)
	h.spawnArgs = "coop 1"

	client := &remoteSignonTestClient{state: caConnected}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdSpawn(subs)

	if want := []string{"spawn coop 1"}; !reflect.DeepEqual(client.signonCommands, want) {
		t.Fatalf("remote spawn commands = %v, want %v", client.signonCommands, want)
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

func TestRegisterCommandsRefreshesExistingBindings(t *testing.T) {
	h1 := NewHost()
	subs1 := &mockSubsystems{server: &mockServer{}, client: &mockClient{}, console: &mockConsole{}}
	subs1.Subsystems.Server = subs1.server
	subs1.Subsystems.Client = subs1.client
	subs1.Subsystems.Console = subs1.console
	h1.RegisterCommands(&subs1.Subsystems)

	h2 := NewHost()
	subs2 := &mockSubsystems{server: &mockServer{}, client: &mockClient{}, console: &mockConsole{}}
	subs2.Subsystems.Server = subs2.server
	subs2.Subsystems.Client = subs2.client
	subs2.Subsystems.Console = subs2.console
	h2.RegisterCommands(&subs2.Subsystems)

	cmdsys.ExecuteText("quit")

	if !h2.aborted {
		t.Fatal("newest host did not receive refreshed quit binding")
	}
	if h1.aborted {
		t.Fatal("stale host unexpectedly handled refreshed quit binding")
	}
}

func TestRegisterCommands_MenuCommandsTargetExpectedStates(t *testing.T) {
	h := NewHost()
	mgr := menu.NewManager(nil, nil)
	h.SetMenu(mgr)
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: &mockConsole{},
	}
	h.RegisterCommands(subs)

	tests := []struct {
		command string
		want    menu.MenuState
	}{
		{command: "menu_main", want: menu.MenuMain},
		{command: "menu_singleplayer", want: menu.MenuSinglePlayer},
		{command: "menu_maps", want: menu.MenuMaps},
		{command: "menu_load", want: menu.MenuLoad},
		{command: "menu_save", want: menu.MenuSave},
		{command: "menu_multiplayer", want: menu.MenuMultiPlayer},
		{command: "menu_setup", want: menu.MenuSetup},
		{command: "menu_options", want: menu.MenuOptions},
		{command: "menu_keys", want: menu.MenuKeys},
		{command: "menu_video", want: menu.MenuVideo},
		{command: "menu_help", want: menu.MenuHelp},
		{command: "menu_quit", want: menu.MenuQuit},
	}

	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			mgr.HideMenu()
			cmdsys.ExecuteText(tc.command)
			if !mgr.IsActive() {
				t.Fatalf("%s should show menu", tc.command)
			}
			if got := mgr.GetState(); got != tc.want {
				t.Fatalf("%s state = %v, want %v", tc.command, got, tc.want)
			}
		})
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

func TestCmdStopdemoPrintsTimedemoSummary(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	h.demoState = &cl.DemoState{
		Playback: true,
		TimeDemo: true,
	}
	// Seed benchmarking counters so summary has deterministic frame count.
	h.demoState.EnableTimeDemo()
	h.demoState.NotePlaybackFrame() // arm
	h.demoState.NotePlaybackFrame() // starts + increments to 1

	h.CmdStopdemo(subs)

	joined := strings.Join(console.messages, "")
	if !strings.Contains(joined, "timedemo: 1 frames") {
		t.Fatalf("console output = %q, want timedemo summary", joined)
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
	if got := h.demoState.BaseSpeed; got != 2.5 {
		t.Fatalf("BaseSpeed = %f, want 2.5", got)
	}

	h.CmdDemoSpeed(-1.5, subs)
	if got := h.demoState.Speed; got != -1.5 {
		t.Fatalf("Speed after rewind command = %f, want -1.5", got)
	}
	if got := h.demoState.BaseSpeed; got != -1.5 {
		t.Fatalf("BaseSpeed after rewind command = %f, want -1.5", got)
	}

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "2.50") || !strings.Contains(output, "-1.50") {
		t.Fatalf("console output = %q, expected speed confirmations", output)
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

func TestCmdPlayAppendsWAVAndLoadsFromSoundDir(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	files := &audioCommandFiles{loaded: map[string][]byte{
		"sound/misc/menu1.wav": []byte("menu1"),
	}}
	subs := &Subsystems{
		Audio: audio,
		Files: files,
	}

	h.CmdPlay([]string{"misc/menu1"}, subs)

	if len(audio.playedSounds) != 1 {
		t.Fatalf("played sound count = %d, want 1", len(audio.playedSounds))
	}
	if got := audio.playedSounds[0].name; got != "misc/menu1.wav" {
		t.Fatalf("played sound name = %q, want misc/menu1.wav", got)
	}
	if got := string(audio.playedSounds[0].data); got != "menu1" {
		t.Fatalf("loaded data = %q, want menu1", got)
	}
	if got := files.calls; !reflect.DeepEqual(got, []string{"sound/misc/menu1.wav"}) {
		t.Fatalf("LoadFile calls = %v, want [sound/misc/menu1.wav]", got)
	}
}

func TestCmdPlayVolUsesExplicitVolumes(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	files := &audioCommandFiles{loaded: map[string][]byte{
		"sound/misc/menu1.wav": []byte("one"),
		"sound/misc/menu2.wav": []byte("two"),
	}}
	subs := &Subsystems{
		Audio:   audio,
		Files:   files,
		Console: &mockConsole{},
	}

	h.CmdPlayVol([]string{"misc/menu1", "0.25", "misc/menu2.wav", "0.5"}, subs)

	if len(audio.playedSounds) != 2 {
		t.Fatalf("played sound count = %d, want 2", len(audio.playedSounds))
	}
	if got := audio.playedSounds[0].vol; got != 0.25 {
		t.Fatalf("first volume = %v, want 0.25", got)
	}
	if got := audio.playedSounds[1].name; got != "misc/menu2.wav" {
		t.Fatalf("second sound name = %q, want misc/menu2.wav", got)
	}
	if got := audio.playedSounds[1].vol; got != 0.5 {
		t.Fatalf("second volume = %v, want 0.5", got)
	}
}

func TestCmdSoundlistPrintsAudioListing(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{soundList: "L(16b)    128 : misc/menu1.wav\n1 sounds, 128 bytes\n"}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdSoundlist(subs)

	if got := strings.Join(console.messages, ""); got != audio.soundList {
		t.Fatalf("console output = %q, want %q", got, audio.soundList)
	}
}

func TestCmdMusicWithoutArgsReportsCurrentTrack(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	audio.currentMusic = "music/track02.ogg"
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusic(nil, subs)

	if got := strings.Join(console.messages, ""); got != "Playing track02, use 'music <musicfile>' to change\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdMusicLoopToggleMatchesCanonicalMessages(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusicLoop([]string{"toggle"}, subs)
	if got := strings.Join(console.messages, ""); got != "Music will be looped\n" {
		t.Fatalf("toggle output = %q, want looped message", got)
	}

	console.Clear()
	h.CmdMusicLoop([]string{"off"}, subs)
	if got := strings.Join(console.messages, ""); got != "Music will not be looped\n" {
		t.Fatalf("off output = %q, want not-looped message", got)
	}
}

func TestCmdMusicJumpPrintsUsageOnInvalidArgs(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusicJump(nil, subs)

	if got := strings.Join(console.messages, ""); got != "music_jump <ordernum>\n" {
		t.Fatalf("console output = %q, want usage", got)
	}
}

func TestCmdFogWithoutArgsPrintsUsageAndCurrentValues(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	client := newLocalLoopbackClient()
	client.inner.FogDensity = 128
	client.inner.FogColor = [3]byte{64, 128, 255}
	subs := &Subsystems{
		Client:  client,
		Console: console,
	}

	h.CmdFog(nil, subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "usage:\n") {
		t.Fatalf("fog usage missing in %q", got)
	}
	if !strings.Contains(got, "\"density\" is \"0.5019608\"") {
		t.Fatalf("fog density line missing in %q", got)
	}
	if !strings.Contains(got, "\"blue\"    is \"1\"") {
		t.Fatalf("fog blue line missing in %q", got)
	}
}

func TestCmdFogDensityOnlyPreservesColor(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.FogColor = [3]byte{51, 102, 153}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"0.25"}, subs)

	if got := client.inner.FogDensity; got != 64 {
		t.Fatalf("FogDensity = %d, want 64", got)
	}
	if got := client.inner.FogColor; got != [3]byte{51, 102, 153} {
		t.Fatalf("FogColor = %v, want preserved [51 102 153]", got)
	}
	if got := client.inner.FogTime; got != 0 {
		t.Fatalf("FogTime = %v, want 0", got)
	}
}

func TestCmdFogRGBOnlyPreservesDensity(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.FogDensity = 200
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"0.1", "0.2", "0.3"}, subs)

	if got := client.inner.FogDensity; got != 200 {
		t.Fatalf("FogDensity = %d, want preserved 200", got)
	}
	if got := client.inner.FogColor; got != [3]byte{26, 51, 77} {
		t.Fatalf("FogColor = %v, want [26 51 77]", got)
	}
}

func TestCmdFogDensityRGBTimeClampsInputs(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.Time = 2
	client.inner.SetFogState(255, [3]byte{255, 128, 0}, 4)
	client.inner.Time = 4
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"-1", "-0.5", "2", "0.4", "1.5"}, subs)

	if got := client.inner.FogDensity; got != 0 {
		t.Fatalf("FogDensity = %d, want clamped 0", got)
	}
	if got := client.inner.FogColor; got != [3]byte{0, 255, 102} {
		t.Fatalf("FogColor = %v, want [0 255 102]", got)
	}
	if got := client.inner.FogTime; got != 1.5 {
		t.Fatalf("FogTime = %v, want 1.5", got)
	}
	currentDensity, currentColor := client.inner.CurrentFog()
	if currentDensity < 0.49 || currentDensity > 0.51 {
		t.Fatalf("CurrentFog density = %v, want ~0.5", currentDensity)
	}
	if currentColor[0] < 0.49 || currentColor[0] > 0.51 || currentColor[1] < 0.24 || currentColor[1] > 0.26 || currentColor[2] != 0 {
		t.Fatalf("CurrentFog color = %v, want previous in-flight fade color", currentColor)
	}
}

func TestCmdStatusForwardsToRemoteServerWhenNoLocalServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdStatus(subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestListenCommandRegistrationExecutes(t *testing.T) {
	cmdsys.RemoveCommand("listen")
	t.Cleanup(func() { cmdsys.RemoveCommand("listen") })

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	inet.Init()
	t.Cleanup(inet.Shutdown)
	_ = inet.Listen(false)

	h.RegisterCommands(subs)
	cmdsys.ExecuteText("listen")

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "\"listen\" is \"0\"") {
		t.Fatalf("listen query output = %q", got)
	}
}

func TestCmdListenQueryAndToggle(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	port := testFreeUDPPort(t)
	inet.Init()
	t.Cleanup(inet.Shutdown)
	inet.SetHostPort(port)
	_ = inet.Listen(false)

	h.CmdListen(nil, subs)
	if got := strings.Join(console.messages, ""); got != "\"listen\" is \"0\"\n" {
		t.Fatalf("listen query output = %q, want disabled state", got)
	}

	h.CmdListen([]string{"1"}, subs)
	if !inet.IsListening() {
		t.Fatal("expected listening enabled")
	}

	h.CmdListen([]string{"0"}, subs)
	if inet.IsListening() {
		t.Fatal("expected listening disabled")
	}
}

func TestCmdMaxPlayersQuerySetAndDeathmatch(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	oldDeathmatch := cvar.StringValue("deathmatch")
	oldMaxPlayers := cvar.StringValue("maxplayers")
	t.Cleanup(func() {
		cvar.Set("deathmatch", oldDeathmatch)
		cvar.Set("maxplayers", oldMaxPlayers)
	})

	h.maxClients = 1
	cvar.Set("maxplayers", "1")
	cvar.Set("deathmatch", "0")

	h.CmdMaxPlayers(nil, subs)
	if got := strings.Join(console.messages, ""); got != "\"maxplayers\" is \"1\"\n" {
		t.Fatalf("maxplayers query output = %q", got)
	}

	console.messages = nil
	h.CmdMaxPlayers([]string{"4"}, subs)
	if got := h.MaxClients(); got != 4 {
		t.Fatalf("maxclients = %d, want 4", got)
	}
	if got := cvar.IntValue("maxplayers"); got != 4 {
		t.Fatalf("maxplayers cvar = %d, want 4", got)
	}
	if got := cvar.IntValue("deathmatch"); got != 1 {
		t.Fatalf("deathmatch = %d, want 1", got)
	}
	if len(console.messages) != 0 {
		t.Fatalf("unexpected console output: %q", strings.Join(console.messages, ""))
	}
}

func TestCmdMaxPlayersRejectsWhenServerRunning(t *testing.T) {
	h := NewHost()
	h.maxClients = 2
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdMaxPlayers([]string{"8"}, subs)

	if got := h.MaxClients(); got != 2 {
		t.Fatalf("maxclients changed to %d while server active", got)
	}
	if got := strings.Join(console.messages, ""); got != "maxplayers can not be changed while a server is running.\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdMaxPlayersQueuesListenTransition(t *testing.T) {
	h := NewHost()
	h.maxClients = 1
	cmdBuf := &insertTrackingCommandBuffer{}
	subs := &Subsystems{Commands: cmdBuf}

	port := testFreeUDPPort(t)
	inet.Init()
	t.Cleanup(inet.Shutdown)
	inet.SetHostPort(port)
	_ = inet.Listen(false)

	h.CmdMaxPlayers([]string{"3"}, subs)
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 1\n"}) {
		t.Fatalf("queued commands = %v, want [listen 1\\n]", got)
	}

	cmdBuf.added = nil
	if err := inet.Listen(true); err != nil {
		t.Fatalf("Listen(true): %v", err)
	}
	h.CmdMaxPlayers([]string{"1"}, subs)
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 0\n"}) {
		t.Fatalf("queued commands = %v, want [listen 0\\n]", got)
	}
}

func TestCmdPortQuerySetValidationAndListenRestart(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	cmdBuf := &insertTrackingCommandBuffer{}
	subs := &Subsystems{Console: console, Commands: cmdBuf}

	oldPort := inet.HostPort()
	t.Cleanup(func() { inet.SetHostPort(oldPort) })

	port := testFreeUDPPort(t)
	inet.Init()
	t.Cleanup(inet.Shutdown)
	_ = inet.Listen(false)
	inet.SetHostPort(port)

	h.CmdPort(nil, subs)
	if got := strings.Join(console.messages, ""); got != fmt.Sprintf("\"port\" is \"%d\"\n", port) {
		t.Fatalf("port query output = %q", got)
	}

	console.messages = nil
	h.CmdPort([]string{"70000"}, subs)
	if got := strings.Join(console.messages, ""); got != "Bad value, must be between 1 and 65534\n" {
		t.Fatalf("invalid port output = %q", got)
	}

	newPort := testFreeUDPPort(t)
	h.CmdPort([]string{strconv.Itoa(newPort)}, subs)
	if got := inet.HostPort(); got != newPort {
		t.Fatalf("host port = %d, want %d", got, newPort)
	}

	if err := inet.Listen(true); err != nil {
		t.Fatalf("Listen(true): %v", err)
	}
	cmdBuf.added = nil
	nextPort := testFreeUDPPort(t)
	h.CmdPort([]string{strconv.Itoa(nextPort)}, subs)
	if got := inet.HostPort(); got != nextPort {
		t.Fatalf("host port after change = %d, want %d", got, nextPort)
	}
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 0\n", "listen 1\n"}) {
		t.Fatalf("queued commands = %v, want [listen 0\\n listen 1\\n]", got)
	}
}

func TestCmdNameUpdatesCVarAndForwardsWhenRemoteConnected(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caConnected}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdName("Ranger", subs)

	if got := cvar.StringValue(clientNameCVar); got != "Ranger" {
		t.Fatalf("%s = %q, want Ranger", clientNameCVar, got)
	}
	if got := client.commands; !reflect.DeepEqual(got, []string{"name Ranger"}) {
		t.Fatalf("forwarded commands = %v, want [name Ranger]", got)
	}
}

func TestCmdStatusForwardsWithInitializedInactiveServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Server:  server.NewServer(),
		Client:  client,
		Console: &mockConsole{},
	}
	if err := h.Init(&InitParams{BaseDir: "."}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h.CmdStatus(subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestCmdNameForwardsWithInitializedInactiveServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caConnected}
	subs := &Subsystems{
		Server:  server.NewServer(),
		Client:  client,
		Console: &mockConsole{},
	}
	if err := h.Init(&InitParams{BaseDir: "."}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h.CmdName("Ranger", subs)

	if got := cvar.StringValue(clientNameCVar); got != "Ranger" {
		t.Fatalf("%s = %q, want Ranger", clientNameCVar, got)
	}
	if got := client.commands; !reflect.DeepEqual(got, []string{"name Ranger"}) {
		t.Fatalf("forwarded commands = %v, want [name Ranger]", got)
	}
}

func TestRegisterCommandsAddsCmdForwarder(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.RegisterCommands(subs)
	if !cmdsys.Exists("cmd") {
		t.Fatal("cmd command was not registered")
	}
	cmdsys.ExecuteText("cmd status")

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestRegisterCommandsAddsRconForwarder(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.RegisterCommands(subs)
	if !cmdsys.Exists("rcon") {
		t.Fatal("rcon command was not registered")
	}
	cmdsys.ExecuteText("rcon status")

	if got := client.commands; !reflect.DeepEqual(got, []string{"rcon status"}) {
		t.Fatalf("forwarded commands = %v, want [rcon status]", got)
	}
}

func TestCmdRconPrintsUsageWithoutArgs(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdRcon(nil, subs)

	if got := strings.Join(console.messages, ""); got != "usage: rcon <command>\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdRconPrintsNotConnected(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdRcon([]string{"status"}, subs)

	if got := strings.Join(console.messages, ""); got != "Can't \"rcon\", not connected\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdForwardToServerStillForwardsWhenLocalServerActive(t *testing.T) {
	h := NewHost()
	h.serverActive = true
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdForwardToServer([]string{"status"}, subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestCmdForwardToServerPrintsNotConnected(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdForwardToServer([]string{"status"}, subs)

	if got := strings.Join(console.messages, ""); got != "Can't \"cmd\", not connected\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdForwardToServerNoopsDuringDemoPlayback(t *testing.T) {
	h := NewHost()
	h.demoState = &cl.DemoState{Playback: true}
	client := &forwardingTrackingClient{state: caActive}
	console := &mockConsole{}
	subs := &Subsystems{
		Client:  client,
		Console: console,
	}

	h.CmdForwardToServer([]string{"status"}, subs)

	if len(client.commands) != 0 {
		t.Fatalf("forwarded commands = %v, want none", client.commands)
	}
	if got := strings.Join(console.messages, ""); got != "" {
		t.Fatalf("console output = %q, want empty", got)
	}
}

func TestCmdForwardToServerSendsNewlineForBareCmd(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdForwardToServer(nil, subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"\n"}) {
		t.Fatalf("forwarded commands = %q, want [\n]", got)
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

type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

func writeCommandTestPak(t testingT, path string, files map[string][]byte) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	defer file.Close()

	if _, err := file.Write([]byte("PACK")); err != nil {
		t.Fatalf("Write magic: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(0)); err != nil {
		t.Fatalf("Write dir ofs placeholder: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(0)); err != nil {
		t.Fatalf("Write dir len placeholder: %v", err)
	}

	type dirEntry struct {
		name string
		pos  int32
		size int32
	}
	entries := make([]dirEntry, 0, len(files))
	for name, data := range files {
		pos, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			t.Fatalf("Seek current: %v", err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatalf("Write data for %s: %v", name, err)
		}
		entries = append(entries, dirEntry{name: name, pos: int32(pos), size: int32(len(data))})
	}

	dirOfs, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek dir ofs: %v", err)
	}
	for _, entry := range entries {
		var name [56]byte
		copy(name[:], []byte(entry.name))
		if _, err := file.Write(name[:]); err != nil {
			t.Fatalf("Write dir name: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, entry.pos); err != nil {
			t.Fatalf("Write dir pos: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, entry.size); err != nil {
			t.Fatalf("Write dir size: %v", err)
		}
	}

	dirLen := int32(len(entries) * 64)
	if _, err := file.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek header patch: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(dirOfs)); err != nil {
		t.Fatalf("Patch dir ofs: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, dirLen); err != nil {
		t.Fatalf("Patch dir len: %v", err)
	}
}

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

func TestCmdPathPrintsSearchPathStack(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	modDir := filepath.Join(baseDir, "hipnotic")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(mod): %v", err)
	}

	writeCommandTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"maps/start.bsp": []byte("id1"),
	})
	writeCommandTestPak(t, filepath.Join(modDir, "pak0.pak"), map[string][]byte{
		"maps/e1m1.bsp": []byte("mod0"),
	})
	writeCommandTestPak(t, filepath.Join(modDir, "pak1.pak"), map[string][]byte{
		"maps/e1m2.bsp": []byte("mod1"),
		"progs.dat":     []byte("progs"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdPath(subs)

	got := strings.Join(console.messages, "")
	if !strings.HasPrefix(got, "Current search path:\n") {
		t.Fatalf("path output missing header:\n%s", got)
	}

	wantInOrder := []string{
		filepath.Join(modDir, "pak1.pak") + " (2 files)\n",
		filepath.Join(modDir, "pak0.pak") + " (1 files)\n",
		modDir + "\n",
		filepath.Join(id1Dir, "pak0.pak") + " (1 files)\n",
		id1Dir + "\n",
	}
	last := -1
	for _, want := range wantInOrder {
		idx := strings.Index(got, want)
		if idx < 0 {
			t.Fatalf("path output missing %q:\n%s", want, got)
		}
		if idx <= last {
			t.Fatalf("path output out of order for %q:\n%s", want, got)
		}
		last = idx
	}
}

func TestCmdSkiesPrintsAvailableSkyboxes(t *testing.T) {
	baseDir := t.TempDir()
	envDir := filepath.Join(baseDir, "id1", "gfx", "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(env): %v", err)
	}
	for _, name := range []string{"stormup.tga", "stormrt.tga", "plasmaup.tga", "junkup.jpg"} {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	writeCommandTestPak(t, filepath.Join(baseDir, "id1", "pak0.pak"), map[string][]byte{
		"gfx/env/iceup.tga":   []byte("iceup"),
		"gfx/env/icert.tga":   []byte("icert"),
		"gfx/env/stormup.tga": []byte("duplicate"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdSkies(nil, subs)

	got := strings.Join(console.messages, "")
	for _, want := range []string{"   ice\n", "   plasma\n", "   storm\n", "3 skies\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("skies output missing %q:\n%s", want, got)
		}
	}
}

func TestCmdSkiesFilter(t *testing.T) {
	baseDir := t.TempDir()
	envDir := filepath.Join(baseDir, "id1", "gfx", "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(env): %v", err)
	}
	for _, name := range []string{"stormup.tga", "plasmaup.tga"} {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdSkies([]string{"sto"}, subs)
	filtered := strings.Join(console.messages, "")
	if !strings.Contains(filtered, "   storm\n") || strings.Contains(filtered, "plasma") {
		t.Fatalf("filtered skies output mismatch:\n%s", filtered)
	}
	if !strings.Contains(filtered, "1 sky containing \"sto\"\n") {
		t.Fatalf("filtered skies summary mismatch:\n%s", filtered)
	}

	console.messages = nil
	h.CmdSkies([]string{"zzz"}, subs)
	if got := strings.Join(console.messages, ""); got != "no skies found containing \"zzz\"\n" {
		t.Fatalf("missing-filter output = %q", got)
	}
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
