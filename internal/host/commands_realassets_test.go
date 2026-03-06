package host

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
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

	h.CmdSave("roundtrip", subs)

	if _, err := os.Stat(filepath.Join(h.UserDir(), "saves", "roundtrip.sav")); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	player.Vars.Health = 12
	player.Vars.Origin = [3]float32{0, 0, 0}

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
	if !srv.Static.Clients[0].Spawned {
		t.Fatal("loaded client not marked spawned")
	}
}
