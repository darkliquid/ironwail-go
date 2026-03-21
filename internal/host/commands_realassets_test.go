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
