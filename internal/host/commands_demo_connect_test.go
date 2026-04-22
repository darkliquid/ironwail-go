package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/server"
)

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
	oldFactory := h.RemoteClientFactory
	h.RemoteClientFactory = func(address string) (Client, error) {
		if address != "example.com:26000" {
			t.Fatalf("remoteClientFactory address = %q, want %q", address, "example.com:26000")
		}
		return remoteClient, nil
	}
	t.Cleanup(func() {
		h.RemoteClientFactory = oldFactory
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

func TestCmdMapSpawnFailureCleansUpDisconnectedSessionState(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	h.SetClientState(caActive)
	h.SetSignOns(4)

	srv := &spawnFailureTrackingServer{spawnErr: fmt.Errorf("spawn failed")}
	remoteClient := &remoteSignonTestClient{state: caActive}
	subs := &Subsystems{
		Files:   &fs.FileSystem{},
		Server:  srv,
		Client:  remoteClient,
		Console: &mockConsole{},
	}

	err := h.CmdMap("start", subs)
	if err == nil {
		t.Fatal("CmdMap(start) error = nil, want spawn failure")
	}
	if !strings.Contains(err.Error(), "spawn failed") {
		t.Fatalf("CmdMap(start) error = %q, want spawn failure", err)
	}
	if srv.shutdownCalls != 1 {
		t.Fatalf("Shutdown calls = %d, want 1", srv.shutdownCalls)
	}
	if remoteClient.shutdownCalls != 1 {
		t.Fatalf("remote client Shutdown calls = %d, want 1", remoteClient.shutdownCalls)
	}
	if h.ServerActive() {
		t.Fatal("ServerActive = true, want false after failed map spawn")
	}
	if got := h.ClientState(); got != caDisconnected {
		t.Fatalf("ClientState = %v, want disconnected", got)
	}
	if got := h.SignOns(); got != 0 {
		t.Fatalf("SignOns = %d, want 0", got)
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
	h := NewHost()
	h.registerHostCVars()
	h.CVar.SetBool("dedicated", false)
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

	previous := h.CVar.StringValue("nomonsters")
	h.CVar.Set("nomonsters", "1")
	t.Cleanup(func() {
		h.CVar.Set("nomonsters", previous)
	})

	h.CmdLoad("slot1", subs)

	if got := h.CVar.StringValue("nomonsters"); got != "0" {
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
