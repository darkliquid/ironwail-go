// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"math"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// drainClientMessages consumes all pending loopback messages for the given
// client slot. Before the client is spawned (during signon handshake) this
// loops until the message queue is empty. After spawning, each call to
// GetClientLoopbackMessage would generate a fresh per-frame datagram, so we
// call it exactly once to avoid an infinite loop.
func drainClientMessages(s *Server, clientNum int) {
	if clientNum < 0 || clientNum >= len(s.Static.Clients) || s.Static.Clients[clientNum] == nil {
		return
	}
	if s.Static.Clients[clientNum].Spawned {
		s.GetClientLoopbackMessage(clientNum)
		return
	}
	for s.GetClientLoopbackMessage(clientNum) != nil {
	}
}

// signonHeadlessClient performs the full loopback signon handshake
// (prespawn → spawn → begin) for a single headless client slot, and
// verifies the slot is marked spawned before returning.
func signonHeadlessClient(t *testing.T, s *Server, clientNum int, name string) {
	t.Helper()
	cl := s.Static.Clients[clientNum]
	if cl == nil {
		t.Fatalf("signonHeadlessClient: client slot %d is nil", clientNum)
	}
	cl.Name = name

	// ConnectClient may have written server-info into the message buffer; drain it.
	drainClientMessages(s, clientNum)

	for _, step := range []string{"prespawn", "spawn", "begin"} {
		if err := s.SubmitLoopbackStringCommand(clientNum, step); err != nil {
			t.Fatalf("signon %q for client %d (%s): %v", step, clientNum, name, err)
		}
		drainClientMessages(s, clientNum)
	}

	if !cl.Spawned {
		t.Fatalf("client %d (%s) not spawned after begin handshake", clientNum, name)
	}
}

// newDeathmatchServer creates and initialises a server configured for two
// players in deathmatch mode. The caller must close vfs when done.
func newDeathmatchServer(t *testing.T) (*Server, *fs.FileSystem) {
	t.Helper()

	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	t.Cleanup(vfs.Close)

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("Init(2): %v", err)
	}
	s.CVar.SetInt("deathmatch", 1)
	// Disable fraglimit so CheckRules never terminates the match mid-test.
	s.CVar.SetInt("fraglimit", 0)

	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	// dm3 is the canonical small deathmatch map; fall back to start when absent.
	if err := s.SpawnServer("dm3", vfs); err != nil {
		if err2 := s.SpawnServer("start", vfs); err2 != nil {
			t.Fatalf("SpawnServer(dm3): %v; SpawnServer(start): %v", err, err2)
		}
		t.Logf("dm3 not available – using start map")
	}

	return s, vfs
}

// TestMultiplayerDedicatedServerConnectsTwoHeadlessClients verifies that a
// dedicated server running with two maximum clients can accept signon
// handshakes from two independent headless loopback clients and leave both
// in the spawned/active state.
func TestMultiplayerDedicatedServerConnectsTwoHeadlessClients(t *testing.T) {
	s, _ := newDeathmatchServer(t)

	s.ConnectClient(0)
	s.ConnectClient(1)

	signonHeadlessClient(t, s, 0, "BotA")
	signonHeadlessClient(t, s, 1, "BotB")

	cl0 := s.Static.Clients[0]
	cl1 := s.Static.Clients[1]

	if !cl0.Active || !cl0.Spawned {
		t.Errorf("BotA: Active=%v Spawned=%v, want both true", cl0.Active, cl0.Spawned)
	}
	if !cl1.Active || !cl1.Spawned {
		t.Errorf("BotB: Active=%v Spawned=%v, want both true", cl1.Active, cl1.Spawned)
	}
	if cl0.Edict == nil || cl0.Edict.Vars == nil {
		t.Error("BotA edict is nil after signon")
	}
	if cl1.Edict == nil || cl1.Edict.Vars == nil {
		t.Error("BotB edict is nil after signon")
	}
}

// TestMultiplayerDeathmatchClientsFightEachOther starts a dedicated server
// with two headless loopback clients, runs signon for both, positions them
// face-to-face at close range, and simulates ~3 seconds of combat where both
// bots hold the attack button. The test asserts that at least one kill was
// recorded on one of the players' frag counters, confirming that weapons
// fire, hitscan resolves, damage is applied, and the QC kill-frag path runs.
func TestMultiplayerDeathmatchClientsFightEachOther(t *testing.T) {
	s, _ := newDeathmatchServer(t)

	s.ConnectClient(0)
	s.ConnectClient(1)

	signonHeadlessClient(t, s, 0, "BotA")
	signonHeadlessClient(t, s, 1, "BotB")

	cl0 := s.Static.Clients[0]
	cl1 := s.Static.Clients[1]

	const frameTime = 1.0 / 72.0

	// Let both players fall to the ground and QC settle them (~0.5 s).
	for i := 0; i < 36; i++ {
		if err := s.Frame(frameTime); err != nil {
			t.Fatalf("settle frame %d: %v", i, err)
		}
		drainClientMessages(s, 0)
		drainClientMessages(s, 1)
	}

	if cl0.Edict == nil || cl0.Edict.Vars == nil {
		t.Fatal("BotA edict missing after settle")
	}
	if cl1.Edict == nil || cl1.Edict.Vars == nil {
		t.Fatal("BotB edict missing after settle")
	}

	// Forcibly position BotB 96 units in front of BotA so the two headless
	// bots are at close but distinct positions regardless of map spawn layout.
	origin0 := cl0.Edict.Vars.Origin
	cl1.Edict.Vars.Origin = [3]float32{
		origin0[0] + 96,
		origin0[1],
		origin0[2],
	}
	s.LinkEdict(cl1.Edict, true)

	t.Logf("combat start – BotA origin=%v health=%.0f frags=%.0f",
		cl0.Edict.Vars.Origin, cl0.Edict.Vars.Health, cl0.Edict.Vars.Frags)
	t.Logf("combat start – BotB origin=%v health=%.0f frags=%.0f",
		cl1.Edict.Vars.Origin, cl1.Edict.Vars.Health, cl1.Edict.Vars.Frags)

	// Combat loop: ~3 simulated seconds (216 frames at 72 fps).
	// Each frame both bots aim directly at each other and hold IN_ATTACK.
	const combatFrames = 216
	for i := 0; i < combatFrames; i++ {
		// Compute yaw from BotA to BotB and vice-versa each frame so aiming
		// tracks movement after respawns and physics drift.
		dx0 := cl1.Edict.Vars.Origin[0] - cl0.Edict.Vars.Origin[0]
		dy0 := cl1.Edict.Vars.Origin[1] - cl0.Edict.Vars.Origin[1]
		yaw0 := float32(math.Atan2(float64(dy0), float64(dx0)) * 180.0 / math.Pi)

		dx1 := -dx0
		dy1 := -dy0
		yaw1 := float32(math.Atan2(float64(dy1), float64(dx1)) * 180.0 / math.Pi)

		// buttons=1 → IN_ATTACK held; impulse=0 keeps whatever weapon QC gave.
		_ = s.SubmitLoopbackCmd(0, [3]float32{0, yaw0, 0}, 0, 0, 0, 1, 0, float64(s.Time))
		_ = s.SubmitLoopbackCmd(1, [3]float32{0, yaw1, 0}, 0, 0, 0, 1, 0, float64(s.Time))

		if err := s.Frame(frameTime); err != nil {
			t.Fatalf("combat frame %d: %v", i, err)
		}
		drainClientMessages(s, 0)
		drainClientMessages(s, 1)
	}

	frags0 := cl0.Edict.Vars.Frags
	frags1 := cl1.Edict.Vars.Frags
	health0 := cl0.Edict.Vars.Health
	health1 := cl1.Edict.Vars.Health

	t.Logf("combat end – BotA frags=%.0f health=%.0f  BotB frags=%.0f health=%.0f",
		frags0, health0, frags1, health1)

	if frags0 <= 0 && frags1 <= 0 {
		t.Errorf("no kills recorded after %d frames of deathmatch combat: BotA frags=%.0f BotB frags=%.0f",
			combatFrames, frags0, frags1)
	}
}
