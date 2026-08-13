package server

import (
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj3PixeldudRegression verifies that on qbj3_pixeldud:
// 1. Player weaponmodel is assigned and resolved ("progs/v_wrench.mdl").
// 2. Client FatPVS is properly populated so entities (keycard, pickups) are not vis-culled.
func TestQbj3PixeldudRegression(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "qbj3"); err != nil {
		t.Skipf("qbj3 mod unavailable: %v", err)
	}
	defer vfs.Close()

	srv := NewServer()
	qc.RegisterBuiltins(srv.QCVM)
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init server: %v", err)
	}
	if err := srv.SpawnServer("qbj3_pixeldud", vfs); err != nil {
		t.Skipf("SpawnServer qbj3_pixeldud: %v", err)
	}

	srv.ConnectClient(0)
	client := srv.Static.Clients[0]
	client.Name = "Player"

	connectFunc := srv.QCVM.FindFunction("ClientConnect")
	if connectFunc >= 0 {
		srv.QCVM.SetGlobal("self", 1)
		if err := srv.executeQCFunction(connectFunc); err != nil {
			t.Fatalf("ClientConnect: %v", err)
		}
	}
	putFunc := srv.QCVM.FindFunction("PutClientInServer")
	if putFunc >= 0 {
		srv.QCVM.SetGlobal("self", 1)
		if err := srv.executeQCFunction(putFunc); err != nil {
			t.Fatalf("PutClientInServer: %v", err)
		}
	}

	// Step physics frames to settle items
	for frame := 1; frame <= 10; frame++ {
		srv.Time = srv.PhysicsSys.StepFrame(srv, srv.Time, 0.1)
	}

	player := srv.EdictNum(1)
	wm := player.WeaponModel(srv)
	wmStr := srv.String(wm)
	if wmStr == "" {
		t.Fatalf("player weaponmodel string is empty after PutClientInServer; want progs/v_wrench.mdl")
	}

	// Verify writeEntitiesToClient populates FatPVS and writes world entities
	msg := NewMessageBuffer(4096)
	srv.writeEntitiesToClient(client, msg)

	if client.FatPVS == nil {
		t.Fatalf("client.FatPVS was not populated by writeEntitiesToClient")
	}

	key1 := srv.EdictNum(16)
	if key1 != nil && !key1.Free {
		if !srv.SV_VisibleToClient(key1, client) {
			t.Fatalf("item_key1 (#16) reported invisible to client FatPVS")
		}
	}
}
