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
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestCmdSaveArgsPrintsUsageWithoutName(t *testing.T) {
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
	subs.Server = subs.server
	subs.Client = subs.client
	subs.Console = subs.console

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

	h.CVar.Set("nomonsters", "1")
	t.Cleanup(func() {
		h.CVar.Set("nomonsters", "0")
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
