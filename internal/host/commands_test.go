// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"bytes"
	"fmt"
	"io"
	stdnet "net"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
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

type spawnFailureTrackingServer struct {
	mockServer
	shutdownCalls int
	spawnErr      error
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

func (s *spawnFailureTrackingServer) SpawnServer(mapName string, vfs *fs.FileSystem) error {
	s.mapName = mapName
	s.spawned = append(s.spawned, mapName)
	if s.spawnErr != nil {
		return s.spawnErr
	}
	return nil
}

func (s *spawnFailureTrackingServer) Shutdown() {
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

func (s *kickTrackingServer) MaxClients() int {
	return len(s.names)
}

func (s *kickTrackingServer) IsClientActive(clientNum int) bool {
	return clientNum >= 0 && clientNum < len(s.active) && s.active[clientNum]
}

func (s *kickTrackingServer) ClientName(clientNum int) string {
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
