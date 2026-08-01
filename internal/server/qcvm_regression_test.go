package server

import (
	"bytes"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// newProductionOrderServer simulates the production init order:
// 1. NewServer() — QCVM created, EdictSize=0, Edicts=nil
// 2. LoadProgs() — sets EdictSize from progs.dat, Edicts still nil
// 3. Init(maxClients) — calls ensureDefaultQCVMEdictStorage which returns
//    early because EdictSize > 0, so Edicts is NOT allocated here
// 4. SpawnServer(mapName) — sets world fields via accessors, then
//    reloadProgs, then syncQCVMState (which calls ensureQCVMEdictStorage)
//
// This matches the real game flow: game_init.go creates the server,
// loadRuntimePrograms calls LoadProgs, then CmdMap calls Init+SpawnServer.
func newProductionOrderServer(t *testing.T, mapName string) *Server {
	t.Helper()
	testutil.SkipIfNoPak0(t)
	baseDir, err := testutil.LocateQuakeDir()
	if err != nil {
		t.Skipf("no quake dir: %v", err)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	t.Cleanup(vfs.Close)

	// Step 1: NewServer — EdictSize=0, Edicts=nil
	s := NewServer()

	// Step 2: LoadProgs — sets EdictSize, Edicts still nil
	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	// Step 3: Init — ensureDefaultQCVMEdictStorage returns early (EdictSize > 0)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	// Step 4: SpawnServer
	if err := s.SpawnServer(mapName, vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	return s
}

// TestQCVMWorldEntitySurvivesReloadProgs verifies that world entity fields
// (Solid, MoveType, ModelIndex) are correctly written to the QCVM Edicts
// byte array during SpawnServer, even when the production init order leaves
// Edicts unallocated until syncQCVMState. If the world's Solid field reads
// as 0 (SolidNot) after spawn, the player falls through the floor because
// hullForEntity skips the BSP hull path.
func TestQCVMWorldEntitySurvivesReloadProgs(t *testing.T) {
	s := newProductionOrderServer(t, "start")

	world := s.Edicts[0]
	if world == nil {
		t.Fatal("world entity is nil after spawn")
	}

	// The world must be SolidBSP for collision to work.
	if got := world.Solid(s); got != float32(SolidBSP) {
		t.Errorf("world Solid = %v, want %v (SolidBSP) — player will fall through floor", got, SolidBSP)
	}
	if got := world.MoveType(s); got != float32(MoveTypePush) {
		t.Errorf("world MoveType = %v, want %v (MoveTypePush)", got, MoveTypePush)
	}
	if got := world.ModelIndex(s); got != 1 {
		t.Errorf("world ModelIndex = %v, want 1", got)
	}
}

// TestQCVMPlayerDoesNotFallThroughFloor simulates the production init order
// and verifies the player entity ends up resting on solid ground rather
// than falling indefinitely after spawn.
func TestQCVMPlayerDoesNotFallThroughFloor(t *testing.T) {
	s := newProductionOrderServer(t, "start")

	// Connect and spawn the client (matches host loopback session flow).
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC: %v", err)
	}
	client.Spawned = true

	player := client.Edict
	if player == nil {
		t.Fatal("player edict is nil")
	}

	// Verify player has solid and movetype set correctly.
	if got := player.Solid(s); got != float32(SolidSlideBox) {
		t.Errorf("player Solid = %v, want %v (SolidSlideBox)", got, SolidSlideBox)
	}
	if got := player.MoveType(s); got != float32(MoveTypeWalk) {
		t.Errorf("player MoveType = %v, want %v (MoveTypeWalk)", got, MoveTypeWalk)
	}

	// Record initial origin.
	originBefore := player.Origin(s)

	// Run several physics frames with client think. In Quake, the player
	// should rest on the floor, not fall through it. After a few frames
	// with gravity, the origin should stabilize (FlagOnGround set).
	for i := 0; i < 10; i++ {
		s.FrameTime = 0.1
		s.Time += s.FrameTime
		s.ClientThink(client)
		s.Physics()
	}

	originAfter := player.Origin(s)

	// If the player fell through the floor, the Z coordinate will have
	// decreased dramatically (gravity accelerates downward). A player
	// resting on the ground should have a stable or near-stable Z.
	zDrop := originBefore[2] - originAfter[2]
	if zDrop > 100 {
		t.Errorf("player fell through floor: Z dropped %.1f units in 10 frames (before=%v, after=%v)",
			zDrop, originBefore, originAfter)
	}

	// The player should have FlagOnGround set after physics settles.
	flags := uint32(player.Flags(s))
	if flags&FlagOnGround == 0 {
		t.Errorf("player FlagOnGround not set after physics (flags=%d)", flags)
	}
}
