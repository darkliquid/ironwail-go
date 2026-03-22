package host

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/server"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestCmdMapStartRealAssetsReachesCaActive(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}
	if got := h.ClientState(); got != caActive {
		t.Fatalf("ClientState = %v, want %v", got, caActive)
	}
	if got := h.SignOns(); got != 4 {
		t.Fatalf("SignOns = %d, want 4", got)
	}
	client := LoopbackClientState(subs)
	if client == nil {
		t.Fatal("loopback client missing")
	}
	if client.State != cl.StateActive {
		t.Fatalf("client.State = %v, want %v", client.State, cl.StateActive)
	}
	if !srv.Static.Clients[0].Spawned {
		t.Fatal("server client not marked spawned")
	}
	if got := srv.GetString(srv.Edicts[0].Vars.ClassName); got != "worldspawn" {
		t.Fatalf("world classname = %q, want %q", got, "worldspawn")
	}
	if got := srv.GetString(srv.Static.Clients[0].Edict.Vars.ClassName); got != "player" {
		t.Fatalf("player classname = %q, want %q", got, "player")
	}
}

func TestCmdMapE2M2RealAssetsKeepsMonstersOutOfSolid(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("e2m2", subs); err != nil {
		t.Fatalf("CmdMap(e2m2): %v", err)
	}
	if got := h.ClientState(); got != caActive {
		t.Fatalf("ClientState = %v, want %v", got, caActive)
	}

	monsterCount := 0
	for entNum := 1; entNum < srv.NumEdicts; entNum++ {
		ent := srv.EdictNum(entNum)
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		className := srv.GetString(ent.Vars.ClassName)
		if len(className) < len("monster_") || className[:len("monster_")] != "monster_" {
			continue
		}
		monsterCount++
		if ent.Vars.Origin == [3]float32{} {
			t.Fatalf("monster %d (%s) spawned at origin after CmdMap", entNum, className)
		}
		if blocker := srv.TestEntityPosition(ent); blocker != nil {
			blockerClass := ""
			if blocker.Vars != nil {
				blockerClass = srv.GetString(blocker.Vars.ClassName)
			}
			t.Fatalf("monster %d (%s) spawned in solid after CmdMap at %v blocker=%d (%s)", entNum, className, ent.Vars.Origin, srv.NumForEdict(blocker), blockerClass)
		}
	}
	if monsterCount == 0 {
		t.Fatal("expected monsters on e2m2")
	}
}

func TestCmdSaveLoadRealAssetsRoundTrip(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	player := srv.Static.Clients[0].Edict
	player.Vars.Health = 61
	player.Vars.Origin = [3]float32{320, 144, 40}
	player.Vars.CurrentAmmo = 12
	player.Vars.AmmoShells = 25
	player.Vars.AmmoNails = 50
	player.Vars.AmmoRockets = 8
	player.Vars.AmmoCells = 31
	player.Vars.Weapon = 8
	player.Vars.Items = 0x0001 | 0x0002 | 0x0040
	player.Vars.ArmorType = 0.6
	player.Vars.ArmorValue = 95
	srv.LightStyles[3] = "az"
	h.SetCurrentSkill(3)
	cvar.SetInt("skill", 3)

	h.CmdSave("roundtrip", subs)

	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "roundtrip.sav")); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	player.Vars.Health = 12
	player.Vars.Origin = [3]float32{0, 0, 0}
	player.Vars.CurrentAmmo = 1
	player.Vars.AmmoShells = 1
	player.Vars.AmmoNails = 1
	player.Vars.AmmoRockets = 1
	player.Vars.AmmoCells = 1
	player.Vars.Weapon = 1
	player.Vars.Items = 0
	player.Vars.ArmorType = 0
	player.Vars.ArmorValue = 0
	h.SetCurrentSkill(0)
	cvar.SetInt("skill", 0)

	h.CmdLoad("roundtrip", subs)

	if got := h.ClientState(); got != caActive {
		t.Fatalf("ClientState = %v, want %v", got, caActive)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Health; got != 61 {
		t.Fatalf("loaded player health = %v, want 61", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Origin; got != ([3]float32{320, 144, 40}) {
		t.Fatalf("loaded player origin = %v, want restored origin", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.CurrentAmmo; got != 12 {
		t.Fatalf("loaded current ammo = %v, want 12", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoShells; got != 25 {
		t.Fatalf("loaded shells = %v, want 25", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoNails; got != 50 {
		t.Fatalf("loaded nails = %v, want 50", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoRockets; got != 8 {
		t.Fatalf("loaded rockets = %v, want 8", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoCells; got != 31 {
		t.Fatalf("loaded cells = %v, want 31", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Weapon; got != 8 {
		t.Fatalf("loaded weapon = %v, want 8", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Items; got != (0x0001 | 0x0002 | 0x0040) {
		t.Fatalf("loaded items = %v, want %v", got, float32(0x0001|0x0002|0x0040))
	}
	if got := srv.Static.Clients[0].Edict.Vars.ArmorType; got != 0.6 {
		t.Fatalf("loaded armor type = %v, want 0.6", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.ArmorValue; got != 95 {
		t.Fatalf("loaded armor value = %v, want 95", got)
	}
	if !srv.Static.Clients[0].Spawned {
		t.Fatal("loaded client not marked spawned")
	}
	clientState := LoopbackClientState(subs)
	if clientState == nil {
		t.Fatal("loopback client state missing after load")
	}
	if got := clientState.LightStyles[3].Map; got != "az" {
		t.Fatalf("loaded lightstyle = %q, want %q", got, "az")
	}
	if got := h.CurrentSkill(); got != 3 {
		t.Fatalf("loaded host skill = %d, want 3", got)
	}
	if got := cvar.IntValue("skill"); got != 3 {
		t.Fatalf("loaded skill cvar = %d, want 3", got)
	}
}

func TestCmdLoadArgsKEXRealAssetsRoundTrip(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)
	baseDir := t.TempDir()
	if err := os.Symlink(filepath.Join(quakeDir, "id1"), filepath.Join(baseDir, "id1")); err != nil {
		t.Fatalf("Symlink(id1): %v", err)
	}

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    baseDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	player := srv.Static.Clients[0].Edict
	player.Vars.Health = 61
	player.Vars.Origin = [3]float32{320, 144, 40}
	player.Vars.ViewOfs = [3]float32{0, 0, 22}
	player.Vars.VAngle = [3]float32{0, 90, 0}
	player.Vars.Angles = [3]float32{0, 90, 0}
	player.Vars.CurrentAmmo = 12
	player.Vars.AmmoShells = 25
	player.Vars.AmmoNails = 50
	player.Vars.AmmoRockets = 8
	player.Vars.AmmoCells = 31
	player.Vars.Weapon = 8
	player.Vars.Items = 0x0001 | 0x0002 | 0x0040
	player.Vars.ArmorType = 0.6
	player.Vars.ArmorValue = 95
	player.Vars.MoveType = float32(server.MoveTypeWalk)
	player.Vars.Solid = float32(server.SolidSlideBox)
	player.Vars.TakeDamage = 1
	player.Vars.Colormap = 1
	player.Vars.Team = 1
	player.Vars.Mins = [3]float32{-16, -16, -24}
	player.Vars.Maxs = [3]float32{16, 16, 32}
	player.Vars.Size = [3]float32{32, 32, 56}
	srv.Static.Clients[0].SpawnParms[0] = 100
	srv.Static.Clients[0].SpawnParms[1] = 250
	srv.LightStyles[3] = "az"
	h.SetCurrentSkill(3)
	cvar.SetInt("skill", 3)

	savePath := filepath.Join(baseDir, "roundtrip.sav")
	saveData := buildKEXTextSave(kexTextSaveFixture{
		gameDir:    "id1",
		mapName:    "start",
		skill:      3,
		time:       srv.Time,
		spawnParms: srv.Static.Clients[0].SpawnParms,
		lightStyles: map[int]string{
			3: "az",
		},
		worldFields: map[string]string{
			"classname": "worldspawn",
		},
		playerFields: map[string]string{
			"classname":    "player",
			"origin":       "320 144 40",
			"health":       "61",
			"view_ofs":     "0 0 22",
			"angles":       "0 90 0",
			"v_angle":      "0 90 0",
			"currentammo":  "12",
			"ammo_shells":  "25",
			"ammo_nails":   "50",
			"ammo_rockets": "8",
			"ammo_cells":   "31",
			"weapon":       "8",
			"items":        "67",
			"armortype":    "0.6",
			"armorvalue":   "95",
			"movetype":     "3",
			"solid":        "3",
			"takedamage":   "1",
			"colormap":     "1",
			"team":         "1",
			"mins":         "-16 -16 -24",
			"maxs":         "16 16 32",
			"size":         "32 32 56",
		},
	})
	if err := os.WriteFile(savePath, []byte(saveData), 0o644); err != nil {
		t.Fatalf("WriteFile(kex save): %v", err)
	}

	player.Vars.Health = 12
	player.Vars.Origin = [3]float32{}
	player.Vars.CurrentAmmo = 1
	player.Vars.AmmoShells = 1
	player.Vars.AmmoNails = 1
	player.Vars.AmmoRockets = 1
	player.Vars.AmmoCells = 1
	player.Vars.Weapon = 1
	player.Vars.Items = 0
	player.Vars.ArmorType = 0
	player.Vars.ArmorValue = 0
	srv.LightStyles[3] = "m"
	h.SetCurrentSkill(0)
	cvar.SetInt("skill", 0)

	h.CmdLoadArgs([]string{"roundtrip", "kex"}, subs)

	if got := h.ClientState(); got != caActive {
		t.Fatalf("ClientState = %v, want %v", got, caActive)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Health; got != 61 {
		t.Fatalf("loaded player health = %v, want 61", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Origin; got != ([3]float32{320, 144, 40}) {
		t.Fatalf("loaded player origin = %v, want restored origin", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.CurrentAmmo; got != 12 {
		t.Fatalf("loaded current ammo = %v, want 12", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoShells; got != 25 {
		t.Fatalf("loaded shells = %v, want 25", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoNails; got != 50 {
		t.Fatalf("loaded nails = %v, want 50", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoRockets; got != 8 {
		t.Fatalf("loaded rockets = %v, want 8", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.AmmoCells; got != 31 {
		t.Fatalf("loaded cells = %v, want 31", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Weapon; got != 8 {
		t.Fatalf("loaded weapon = %v, want 8", got)
	}
	clientState := LoopbackClientState(subs)
	if clientState == nil {
		t.Fatal("loopback client state missing after kex load")
	}
	if got := clientState.LightStyles[3].Map; got != "az" {
		t.Fatalf("loaded lightstyle = %q, want %q", got, "az")
	}
	if got := h.CurrentSkill(); got != 3 {
		t.Fatalf("loaded host skill = %d, want 3", got)
	}
	if got := cvar.IntValue("skill"); got != 3 {
		t.Fatalf("loaded skill cvar = %d, want 3", got)
	}
}

func TestCmdSaveArgsSkipNotifySuppressesSaveMessage(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Console: console,
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}
	console.Clear()

	h.CmdSaveArgs([]string{"autosave/start", "0"}, subs)

	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "autosave", "start.sav")); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if got := strings.Join(console.messages, ""); strings.Contains(got, "Saving game to") {
		t.Fatalf("console output = %q, want no save notification", got)
	}
}

func TestCmdSaveNestedPathPrintsRelativeSaveName(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	console := &mockConsole{}
	subs := &Subsystems{
		Files:   fileSys,
		Console: console,
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}
	console.Clear()

	h.CmdSave("autosave/start", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Saving game to autosave/start.sav...") {
		t.Fatalf("console output = %q, want nested relative save name", got)
	}
}

func TestCmdSaveBlockedPathPrintsCouldNotOpen(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	console := &mockConsole{}
	userDir := t.TempDir()
	subs := &Subsystems{
		Files:   fileSys,
		Console: console,
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    userDir,
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	savesPath := filepath.Join(userDir, "saves")
	if err := os.RemoveAll(savesPath); err != nil {
		t.Fatalf("RemoveAll(%q): %v", savesPath, err)
	}
	if err := os.WriteFile(savesPath, []byte("blocked"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", savesPath, err)
	}
	console.Clear()

	h.CmdSave("slot1", subs)

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "ERROR: couldn't open.") {
		t.Fatalf("console output = %q, want couldn't-open error", got)
	}
}

func TestCmdRestartAutoloadsLastSaveForDeadPlayer(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	previousAutoload := cvar.StringValue("sv_autoload")
	cvar.Set("sv_autoload", "2")
	t.Cleanup(func() {
		cvar.Set("sv_autoload", previousAutoload)
	})

	player := srv.Static.Clients[0].Edict
	player.Vars.Health = 61
	player.Vars.Origin = [3]float32{320, 144, 40}

	h.CmdSave("autoload_restart", subs)

	player.Vars.Health = 0
	player.Vars.Origin = [3]float32{0, 0, 0}

	h.CmdRestart(subs)

	if got := srv.Static.Clients[0].Edict.Vars.Health; got != 61 {
		t.Fatalf("autoloaded restart health = %v, want 61", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Origin; got != ([3]float32{320, 144, 40}) {
		t.Fatalf("autoloaded restart origin = %v, want restored origin", got)
	}
}

func TestCmdChangelevelSameMapAutoloadsLastSaveWhenConfigured(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		UserDir:    t.TempDir(),
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	previousAutoload := cvar.StringValue("sv_autoload")
	cvar.Set("sv_autoload", "3")
	t.Cleanup(func() {
		cvar.Set("sv_autoload", previousAutoload)
	})

	player := srv.Static.Clients[0].Edict
	player.Vars.Health = 61
	player.Vars.Origin = [3]float32{320, 144, 40}

	h.CmdSave("autoload_changelevel", subs)

	player.Vars.Health = 12
	player.Vars.Origin = [3]float32{0, 0, 0}

	h.CmdChangelevel("start", subs)

	if got := srv.Static.Clients[0].Edict.Vars.Health; got != 61 {
		t.Fatalf("autoloaded same-map changelevel health = %v, want 61", got)
	}
	if got := srv.Static.Clients[0].Edict.Vars.Origin; got != ([3]float32{320, 144, 40}) {
		t.Fatalf("autoloaded same-map changelevel origin = %v, want restored origin", got)
	}
}

func TestCmdReconnectRealAssetsRestartsLocalSignon(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer fileSys.Close()

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	client := LoopbackClientState(subs)
	if client == nil {
		t.Fatal("loopback client missing")
	}

	h.CmdReconnect(subs)

	if got := h.ClientState(); got != caActive {
		t.Fatalf("ClientState = %v, want %v", got, caActive)
	}
	if got := h.SignOns(); got != cl.Signons {
		t.Fatalf("SignOns = %d, want %d", got, cl.Signons)
	}
	if client.State != cl.StateActive {
		t.Fatalf("client.State = %v, want %v", client.State, cl.StateActive)
	}
	if client.Signon != cl.Signons {
		t.Fatalf("client.Signon = %d, want %d", client.Signon, cl.Signons)
	}
	if !srv.Static.Clients[0].Spawned {
		t.Fatal("server client not marked spawned after reconnect")
	}
}
