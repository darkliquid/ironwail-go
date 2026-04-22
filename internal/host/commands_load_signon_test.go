package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/server"
)

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
	h.Cmd.AddCommand("test_alias_target", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "")
	defer h.Cmd.RemoveCommand("test_alias_target")

	h.CmdAlias([]string{"foo", "test_alias_target", "bar", "baz"}, &subs.Subsystems)
	h.Cmd.ExecuteText("foo")
	if want := []string{"bar", "baz"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("alias execution args = %v, want %v", gotArgs, want)
	}

	h.CmdUnalias([]string{"foo"}, &subs.Subsystems)
	gotArgs = nil
	h.Cmd.ExecuteText("foo")
	if gotArgs != nil {
		t.Fatalf("expected foo alias to be removed, got args %v", gotArgs)
	}

	h.CmdAlias([]string{"one", "test_alias_target", "one"}, &subs.Subsystems)
	h.CmdAlias([]string{"two", "test_alias_target", "two"}, &subs.Subsystems)
	h.CmdUnaliasAll()
	if _, ok := h.Cmd.Alias("one"); ok {
		t.Fatal("expected alias one to be removed by unaliasall")
	}
	if _, ok := h.Cmd.Alias("two"); ok {
		t.Fatal("expected alias two to be removed by unaliasall")
	}
}

func TestAliasCommandSupportsQuotedSemicolonBodies(t *testing.T) {
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
	h.Cmd.AddCommand("test_alias_chain", func(args []string) {
		got = append(got, strings.Join(args, " "))
	}, "")
	defer h.Cmd.RemoveCommand("test_alias_chain")

	h.Cmd.ExecuteText(`alias combo "test_alias_chain one; test_alias_chain two"`)
	h.Cmd.ExecuteText("combo")

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

	h2.Cmd.ExecuteText("quit")

	if !h2.aborted {
		t.Fatal("newest host did not receive refreshed quit binding")
	}
	if h1.aborted {
		t.Fatal("stale host unexpectedly handled refreshed quit binding")
	}
}

func TestRegisterCommands_MenuCommandsTargetExpectedStates(t *testing.T) {
	h := NewHost()
	mgr := menu.NewManager(nil, nil, nil)
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
		{command: "menu_maps", want: menu.MenuMods},
		{command: "menu_load", want: menu.MenuLoad},
		{command: "menu_save", want: menu.MenuSave},
		{command: "menu_multiplayer", want: menu.MenuMultiPlayer},
		{command: "menu_setup", want: menu.MenuSetup},
		{command: "menu_options", want: menu.MenuOptions},
		{command: "menu_keys", want: menu.MenuControls},
		{command: "menu_video", want: menu.MenuVideo},
		{command: "menu_help", want: menu.MenuHelp},
		{command: "menu_quit", want: menu.MenuQuit},
	}

	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			mgr.HideMenu()
			h.Cmd.ExecuteText(tc.command)
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
