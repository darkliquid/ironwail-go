package host

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

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
	if got := activeFS.GameDir(); got != "hipnotic" {
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
	h.SetGameDirChangedCallback(func(changedSubs *Subsystems, changed *fs.FileSystem) error {
		calls++
		if changedSubs != subs {
			t.Fatalf("callback subs pointer changed")
		}
		if changedSubs.Files != changed {
			t.Fatalf("callback saw stale subs filesystem")
		}
		seenGameDir = changed.GameDir()
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

func TestCmdGameKeepsPreviousFilesystemAliveDuringCallback(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	writeCommandTestPak(t, filepath.Join(baseDir, "id1", "pak0.pak"), map[string][]byte{
		"gfx.wad": []byte("base-gfx"),
	})
	writeCommandTestPak(t, filepath.Join(baseDir, "hipnotic", "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("hipnotic"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	console := &mockConsole{}
	h := NewHost()
	subs := &Subsystems{Console: console, Files: fileSys}

	var callbackErr error
	h.SetGameDirChangedCallback(func(_ *Subsystems, changed *fs.FileSystem) error {
		if _, err := fileSys.LoadFile("gfx.wad"); err != nil {
			callbackErr = err
		}
		if _, err := changed.LoadFile("progs.dat"); err != nil {
			return err
		}
		return nil
	})

	h.CmdGame([]string{"hipnotic"}, subs)

	if callbackErr != nil {
		t.Fatalf("previous filesystem should remain readable during callback: %v", callbackErr)
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
	if got := activeFS.GameDir(); got != "hipnotic" {
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

	h.Cmd.ExecuteText(`game "hipnotic"`)

	activeFS, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		t.Fatalf("subs.Files type = %T, want *fs.FileSystem", subs.Files)
	}
	if got := activeFS.GameDir(); got != "hipnotic" {
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
	if got := activeFS.GameDir(); got != "id1" {
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
	if err := inet.DefaultNetwork().SetIPBan("off", ""); err != nil {
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
	if err := h.Net.SetIPBan("off", ""); err != nil {
		t.Fatalf("clear ban: %v", err)
	}
	t.Cleanup(func() {
		_ = h.Net.SetIPBan("off", "")
	})

	h.CmdBan([]string{"192.168.1.100"}, subs)

	if got := h.Net.IPBanStatus(); got != "Banning 192.168.1.100 [255.255.255.255]" {
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
	modelEnt.SetModel(srv, srv.QCVM.AllocString("progs/ogre.mdl"))
	modelEnt.SetSolid(srv, float32(server.SolidSlideBox))
	modelEnt.SetMoveType(srv, float32(server.MoveTypeStep))

	solidEnt := srv.AllocEdict()
	if solidEnt == nil {
		t.Fatal("AllocEdict solidEnt = nil")
	}
	solidEnt.SetSolid(srv, float32(server.SolidBSP))
	server.ResetCheckBottomStats()
	grounded := srv.AllocEdict()
	if grounded == nil {
		t.Fatal("AllocEdict grounded = nil")
	}
	grounded.SetOrigin(srv, [3]float32{0, 0, 24})
	grounded.SetMins(srv, [3]float32{-16, -16, -24})
	grounded.SetMaxs(srv, [3]float32{16, 16, 32})
	grounded.SetSolid(srv, float32(server.SolidSlideBox))
	grounded.SetMoveType(srv, float32(server.MoveTypeStep))
	srv.WorldModel = server.CreateSyntheticWorldModel()
	srv.Edicts[0].SetSolid(srv, float32(server.SolidBSP))
	srv.ClearWorld()
	srv.LinkEdict(grounded, false)
	if !srv.CheckBottom(grounded) {
		t.Fatal("expected grounded entity to satisfy CheckBottom")
	}
	air := srv.AllocEdict()
	if air == nil {
		t.Fatal("AllocEdict air = nil")
	}
	air.SetOrigin(srv, [3]float32{0, 0, 256})
	air.SetMins(srv, [3]float32{-16, -16, -24})
	air.SetMaxs(srv, [3]float32{16, 16, 32})
	air.SetSolid(srv, float32(server.SolidSlideBox))
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
	if err := h.Net.SetIPBan("off", ""); err != nil {
		t.Fatalf("clear ban: %v", err)
	}
	t.Cleanup(func() {
		_ = h.Net.SetIPBan("off", "")
	})

	h.CmdBan([]string{"10.0.0.0", "255.255.0.0"}, subs)

	if got := h.Net.IPBanStatus(); got != "Banning 10.0.0.0 [255.255.0.0]" {
		t.Fatalf("ban status = %q", got)
	}
}

func TestCmdBanTurnsOff(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}
	if err := h.Net.SetIPBan("192.168.1.100", ""); err != nil {
		t.Fatalf("set ban: %v", err)
	}

	h.CmdBan([]string{"off"}, subs)

	if got := h.Net.IPBanStatus(); got != "Banning not active" {
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
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}
	h.RegisterCommands(subs)
	h.SetServerActive(true)

	h.Cmd.ExecuteText("kick # 2 too much ping")
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
	subs.Server = subs.server
	subs.Client = subs.client
	subs.Console = subs.console

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
	subs.Server = subs.server
	subs.Client = subs.client
	subs.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	previous := h.CVar.StringValue("nomonsters")
	h.CVar.Set("nomonsters", "1")
	t.Cleanup(func() {
		h.CVar.Set("nomonsters", previous)
	})

	h.CmdLoad("../bad", &subs.Subsystems)

	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "Relative pathnames are not allowed.") {
		t.Fatalf("console output = %q, want relative-path rejection", got)
	}
	if got := h.CVar.StringValue("nomonsters"); got != "1" {
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
	subs.Server = subs.server
	subs.Client = subs.client
	subs.Console = subs.console

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
