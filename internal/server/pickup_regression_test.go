package server

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// spawnPickupServer boots a headless server on a real map so pickup behavior
// can be observed end-to-end. Skips when pak assets are unavailable.
func spawnPickupServer(t *testing.T, gameDir, mapName string) (*Server, *fs.FileSystem) {
	t.Helper()
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}
	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, gameDir); err != nil {
		t.Fatalf("vfs.Init: %v", err)
	}
	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
	if err := s.SpawnServer(mapName, vfs); err != nil {
		t.Skipf("SpawnServer(%s): %v", mapName, err)
	}
	return s, vfs
}

// signonLoopbackClient performs the prespawn/spawn/begin handshake and drains
// messages using the same pattern as the multiplayer e2e helper.
func signonLoopbackClient(t *testing.T, s *Server) *Client {
	t.Helper()
	s.ConnectClient(0)
	cl := s.Static.Clients[0]
	cl.Name = "Tester"
	drainClientMessages(s, 0)
	for _, step := range []string{"prespawn", "spawn", "begin"} {
		if err := s.SubmitLoopbackStringCommand(0, step); err != nil {
			t.Fatalf("signon %q: %v", step, err)
		}
		drainClientMessages(s, 0)
	}
	if !cl.Spawned {
		t.Fatal("client not spawned after signon")
	}
	return cl
}

// firstWeaponClass returns the first live weapon edict matching cls, or nil.
func firstWeaponClass(t *testing.T, s *Server, cls string) (*Edict, int) {
	t.Helper()
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		if s.QCVM.String(s.QCVM.EString(i, qc.EntFieldClassName)) == cls {
			return ent, i
		}
	}
	return nil, 0
}

// stepPickupFrames runs n physics frames with a stationary player and drains
// the loopback client messages after each.
func stepPickupFrames(t *testing.T, s *Server, n int) {
	t.Helper()
	const frameTime = 1.0 / 72.0
	for i := 0; i < n; i++ {
		_ = s.SubmitLoopbackCmd(0, [3]float32{0, 0, 0}, 0, 0, 0, 0, 0, float64(s.Time))
		if err := s.Frame(frameTime); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		drainClientMessages(s, 0)
	}
}

// TestPickupWeaponGrantsItemAndHidesFromClient reproduces the id1 e1m1
// regression where picking up a weapon granted it to the player but the
// weapon model stayed visible: the server kept sending the entity because
// its modelindex remained set while QuakeC cleared the model string.
//
// Where in C: SV_WriteEntitiesToClient in sv_main.c skips entities with an
// empty model string ("ignore ents without visible models").
func TestPickupWeaponGrantsItemAndHidesFromClient(t *testing.T) {
	s, vfs := spawnPickupServer(t, "id1", "maps/e1m1.bsp")
	defer vfs.Close()
	cl := signonLoopbackClient(t, s)
	player := cl.Edict

	weapon, weaponNum := firstWeaponClass(t, s, "weapon_nailgun")
	if weapon == nil {
		t.Fatal("weapon_nailgun not found on e1m1")
	}
	if int(weapon.Solid(s)) != int(SolidTrigger) {
		t.Fatalf("weapon_nailgun solid = %d, want SOLID_TRIGGER", int(weapon.Solid(s)))
	}

	// Teleport the player onto the weapon so the touch fires deterministically.
	// C's SV_LinkEdict(ent, true) runs SV_TouchLinks immediately, so the
	// pickup applies during LinkEdict itself; snapshot items beforehand.
	before := uint32(player.Items(s))
	org := weapon.Origin(s)
	player.SetOrigin(s, [3]float32{org[0], org[1], org[2]})
	s.LinkEdict(player, true)

	stepPickupFrames(t, s, 3)

	after := uint32(player.Items(s))
	if after == before {
		t.Fatalf("player items unchanged after touching weapon: 0x%x -> 0x%x", before, after)
	}

	// The picked-up item must no longer be sent to the client: QuakeC clears
	// its model string (self.model = string_null) while modelindex stays set.
	// C's SV_WriteEntitiesToClient skips entities with an empty model string,
	// so the server must not write the weapon into the client datagram.
	ent := s.EdictNum(weaponNum)
	if ent == nil || ent.Free {
		return // freed items are also fine
	}
	if s.String(ent.Model(s)) != "" {
		t.Fatalf("picked-up weapon #%d model string not cleared: %q", weaponNum, s.String(ent.Model(s)))
	}
	// With full PVS coverage the only reason the weapon would be omitted is
	// the empty-model skip; confirm the send never tracks it client-side.
	client := s.Static.Clients[0]
	client.FatPVS = make([]byte, 4096)
	for i := range client.FatPVS {
		client.FatPVS[i] = 0xff
	}
	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)
	if _, tracked := client.EntityStates[weaponNum]; tracked {
		t.Fatalf("picked-up weapon #%d still written into the client datagram", weaponNum)
	}
}

// TestPickupItemSettlesThenPickable verifies qbj3's deferred item placement:
// items spawn invisible/SOLID_NOT (StartItem sets model=string_null and a
// delayed nextthink=ItemPlace) and become pickable once placed.
func TestPickupItemSettlesThenPickable(t *testing.T) {
	s, vfs := spawnPickupServer(t, "qbj3", "maps/qbj3_pixeldud.bsp")
	defer vfs.Close()
	cl := signonLoopbackClient(t, s)

	flak, flakNum := firstWeaponClass(t, s, "weapon_flak")
	if flak == nil {
		t.Fatal("weapon_flak not found on qbj3_pixeldud")
	}
	// Before its ItemPlace think runs, the item is deferred (SOLID_NOT).
	t.Logf("flak #%d initial solid=%d model=%q modelindex=%d",
		flakNum, int(flak.Solid(s)), s.String(flak.Model(s)), int(flak.ModelIndex(s)))

	// Wait out the deferred placement (~0.2-0.3s) so the item becomes solid.
	stepPickupFrames(t, s, 40)

	settled := s.EdictNum(flakNum)
	if settled == nil || settled.Free {
		t.Fatal("flak was removed instead of settling")
	}
	if int(settled.Solid(s)) != int(SolidTrigger) {
		t.Fatalf("flak solid = %d after settle, want %d (SOLID_TRIGGER)", int(settled.Solid(s)), int(SolidTrigger))
	}
	if s.String(settled.Model(s)) == "" {
		t.Fatal("flak model string empty after settle")
	}

	// Now placing the player on it must grant the weapon (the touch fires
	// during LinkEdict itself, so snapshot items beforehand).
	player := cl.Edict
	before := uint32(player.Items(s))
	org := settled.Origin(s)
	player.SetOrigin(s, [3]float32{org[0], org[1], org[2]})
	s.LinkEdict(player, true)
	stepPickupFrames(t, s, 4)
	if uint32(player.Items(s)) == before {
		t.Fatalf("flak touch did not grant the weapon: items 0x%x -> 0x%x", before, uint32(player.Items(s)))
	}
}
