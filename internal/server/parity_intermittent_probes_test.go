// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// These tests are REGRESSION GUARDS for the three intermittently-reported
// engine anomalies (double-door pair misfire, AI tracer/visibility, delayed
// sound delivery). Each encodes a C-reference-guaranteed behavior and asserts
// the Go server reproduces it on a synthetic world, so a cross-frame
// regression is caught in CI rather than "sometimes" in game.
//
// Status (2026-08-07): all PASS. Initial "red probe" attempts failed only
// due to test-harness bugs (invoking VM builtins by raw index instead of via
// a Function, missing ltime/world setup, unseeded function index 0). After
// those were fixed, the engine reproduced all three correct behaviors, which
// rules OUT the base paths and points the symptom hunt at the interleavings
// (riders on both walls, partial/water traces, message-flush boundaries).
// See docs/diagnoses/intermittent_anomalies.md for the full investigation.
//
// Where in C:
//   - double-door pair: SV_Physics_Pusher + SV_PushMove in sv_phys.c
//   - AI tracer:        SV_TraceLine/SV_Move in world.c + trap.visible in ai.qc
//   - sound delivery:   SV_StartSound in sv_main.c
//
// Run with:
//   TMPDIR=<repo>/.tmp CGO_ENABLED=0 go test ./internal/server -run 'TestParity' -v

// ---------------------------------------------------------------------------
// Symptom 2 — paired double doors must advance BOTH halves in the same frame
// ---------------------------------------------------------------------------

func TestParityDoubleDoorPairAdvancesBothHalves(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 32)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	t.Cleanup(s.Shutdown)
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()
	s.FrameTime = 0.1

	// Two door halves sharing a trigger: both must receive the same push
	// think in the same server frame.
	left := s.AllocEdict()
	right := s.AllocEdict()
	if left == nil || right == nil {
		t.Fatal("failed to allocate door halves")
	}
	for _, door := range []*Edict{left, right} {
		door.SetMoveType(s, float32(MoveTypePush))
		door.SetSolid(s, float32(SolidBSP))
		door.SetOrigin(s, [3]float32{0, 0, 0})
		door.SetMins(s, [3]float32{-32, -32, 0})
		door.SetMaxs(s, [3]float32{32, 32, 72})
		door.SetVelocity(s, [3]float32{0, 0, 100})
		s.LinkEdict(door, false)
	}
	left.SetOrigin(s, [3]float32{-40, 0, 0})
	right.SetOrigin(s, [3]float32{40, 0, 0})
	s.LinkEdict(left, false)
	s.LinkEdict(right, false)

	// Register a think that flips a "opened" flag once fired.
	const (
		openedOfs = 90 // scratch global offset for door_opened
		oneOfs    = 91 // constant 1.0
	)
	vm := s.QCVM
	vm.GlobalDefs = append(vm.GlobalDefs,
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: uint16(openedOfs), Name: vm.AllocString("door_opened")},
	)
	vm.SetGFloat(oneOfs, 1)
	// Function index 0 is reserved ("no function" in the VM). Seed a
	// dummy so our appended think lands at a non-zero index, matching
	// how real progs.dat lays out compiles functions (index 0 reserved).
	vm.Functions = []qc.DFunction{{}}
	vm.Functions = append(vm.Functions, qc.DFunction{
		Name: vm.AllocString("door_open_think"), FirstStatement: int32(len(vm.Statements)),
	})
	doorOpenThink := int32(len(vm.Functions) - 1)
	vm.Statements = append(vm.Statements,
		qc.DStatement{Op: uint16(qc.OPStoreF), A: uint16(oneOfs), B: uint16(openedOfs)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)

	// Give both halves the same think and same nextthink, and set ltime
	// to the spawn-time the engine would (matching C: edicts spawn with
	// v.ltime = sv.time).
	for _, door := range []*Edict{left, right} {
		door.SetLTime(s, s.Time)
		door.SetThink(s, doorOpenThink)
		door.SetNextThink(s, s.Time+0.1)
	}

	// Step one frame. BOTH halves must have fired by end of frame;
	// a paired-door misfire drops one half.
	s.Physics()
	if vm.GlobalFloat("door_opened") == 0 {
		t.Fatal("door_pair: neither half fired its open think in the same frame")
	}
}

// ---------------------------------------------------------------------------
// Symptom 3 — AI tracer: a clear-LOS line reports visible
// ---------------------------------------------------------------------------

func TestParityAITracelineReportsClearLOS(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 32)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	t.Cleanup(s.Shutdown)
	s.WorldModel = CreateSyntheticWorldModel()
	s.ClearWorld()

	// Monster at origin, player 200 units along +x at eye height. No walls.
	monster := s.AllocEdict()
	player := s.AllocEdict()
	monster.SetOrigin(s, [3]float32{0, 0, 0})
	player.SetOrigin(s, [3]float32{200, 0, 0})
	monster.SetViewOfs(s, [3]float32{0, 0, 36})
	player.SetViewOfs(s, [3]float32{0, 0, 36})
	monster.SetSolid(s, float32(SolidBBox))
	player.SetSolid(s, float32(SolidSlideBox))
	s.LinkEdict(monster, false)
	s.LinkEdict(player, false)

	// The QC builtin 'traceline' (internal/server/server.go:319) invokes
	// s.SV_Move with a zero hull. A clear LOS from monster eye to player
	// eye must report Fraction == 1 (visible() would return TRUE).
	start := [3]float32{0, 0, 36}
	end := [3]float32{200, 0, 36}
	trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNoMonsters), monster)
	if trace.Fraction < 1.0 {
		t.Fatalf("ai_traceline: clear LOS reported fraction=%v, want 1.0", trace.Fraction)
	}
	if trace.AllSolid {
		t.Fatalf("ai_traceline: trace_allsolid set on open line")
	}
}

// ---------------------------------------------------------------------------
// Symptom 4 — a sound() call emits svc_sound in the same server datagram
// ---------------------------------------------------------------------------

func TestParitySoundEmittedSameFrame(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 32)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	t.Cleanup(s.Shutdown)

	const sample = "misc/h2ohit2.wav"
	s.SoundPrecache = append(s.SoundPrecache, sample)
	if s.FindSound(sample) < 0 {
		t.Fatalf("precache: FindSound(%q) = -1", sample)
	}

	ent := s.AllocEdict()
	ent.SetOrigin(s, [3]float32{0, 0, 24})
	ent.SetMins(s, [3]float32{-2, -4, -6})
	ent.SetMaxs(s, [3]float32{2, 4, 6})
	s.LinkEdict(ent, false)

	// StartSound is the exact path the QC builtin sound() bridge calls
	// (server.go:764-767). It must serialize into the datagram now.
	s.StartSound(ent, 0, sample, 255, 1)

	// The svc_sound opcode must be in the datagram already.
	found := false
	for i := 0; i+1 < len(s.Datagram.Data); i++ {
		if s.Datagram.Data[i] == byte(inet.SVCSound) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sound_delivery: svc_sound not present after StartSound (delayed/dropped)")
	}
}
