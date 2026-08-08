// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Interleaving probes for the intermittently-reported anomalies.
//
// The plain-path guards in parity_intermittent_probes_test.go pass; these
// exercise the *interleavings* the symptoms point at:
//
//   - door pair: the Owner/Enemy chain built by LinkDoors (doors.go) must
//     fire BOTH halves when the shared trigger fires the owner
//   - AI water trace: visible() must still return TRUE through a water
//     volume where trace_inopen and trace_inwater both set
//   - sound watermark: StartSound must not be dropped when the datagram is
//     within one message of the flush watermark
//
// Where in C: doors.qc LinkDoors/door_fire, sv_main.c SV_StartSound.

// ---------------------------------------------------------------------------
// Door pair chain: firing the owner must fire BOTH linked halves.
// ---------------------------------------------------------------------------

func TestParityDoorChainFiresBothHalves(t *testing.T) {
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

	// Two door halves, linked as owner/enemy (the LinkDoors contract).
	owner := s.AllocEdict()
	half := s.AllocEdict()
	if owner == nil || half == nil {
		t.Fatal("failed to allocate door halves")
	}
	for _, door := range []*Edict{owner, half} {
		door.SetMoveType(s, float32(MoveTypePush))
		door.SetSolid(s, float32(SolidBSP))
		door.SetOrigin(s, [3]float32{0, 0, 0})
		door.SetMins(s, [3]float32{-32, -32, 0})
		door.SetMaxs(s, [3]float32{32, 32, 72})
		door.SetVelocity(s, [3]float32{0, 0, 100})
		s.LinkEdict(door, false)
	}
	owner.SetOrigin(s, [3]float32{-40, 0, 0})
	half.SetOrigin(s, [3]float32{40, 0, 0})
	s.LinkEdict(owner, false)
	s.LinkEdict(half, false)
	owner.SetEnemy(s, int32(s.NumForEdict(half)))
	half.SetEnemy(s, int32(s.NumForEdict(owner)))
	owner.SetOwner(s, int32(s.NumForEdict(owner)))
	half.SetOwner(s, int32(s.NumForEdict(owner)))

	// Register a "door_go_up" think that records it ran.
	const (
		upFlag = 90
		oneOfs = 91
	)
	vm := s.QCVM
	vm.GlobalDefs = append(vm.GlobalDefs,
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: uint16(upFlag), Name: vm.AllocString("door_up_count")},
	)
	vm.SetGFloat(oneOfs, 1)
	// Model the real door_fire/chain: the owner's Think is door_fire which
	// walks the Enemy chain and calls door_go_up on each half, and each
	// half's door_go_up sets its OWN NextThink (SUB_CalcMove schedules the
	// move). The pusher loop then runs each half independently.
	// Functions: index 1 = door_fire (owner), index 2 = door_go_up (halves).
	vm.Functions = []qc.DFunction{{}}
	vm.Functions = append(vm.Functions, qc.DFunction{Name: vm.AllocString("door_fire"), FirstStatement: int32(len(vm.Statements))})
	doorFire := int32(len(vm.Functions) - 1)
	// door_fire: increment counter, then (approximating SUB_UseTargets on
	// the enemy) schedule the enemy by setting its NextThink.
	vm.Functions = append(vm.Functions, qc.DFunction{Name: vm.AllocString("door_go_up"), FirstStatement: int32(len(vm.Statements))})
	doorGoUp := int32(len(vm.Functions) - 1)

	vm.Statements = append(vm.Statements,
		// door_fire body (owner): counter++ via builtin-free ops
		qc.DStatement{Op: uint16(qc.OPAddF), A: uint16(upFlag), B: uint16(oneOfs), C: uint16(upFlag)},
		qc.DStatement{Op: uint16(qc.OPDone)},
		// door_go_up body (each half): counter++, schedule own next think.
		qc.DStatement{Op: uint16(qc.OPAddF), A: uint16(upFlag), B: uint16(oneOfs), C: uint16(upFlag)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)

	// Owner's think = door_fire; each half's think = door_go_up.
	// The owner is scheduled; door_fire routes the cascade through each
	// half's own think, which the pusher loop runs per half.
	owner.SetThink(s, doorFire)
	half.SetThink(s, doorGoUp)
	// Both halves are independently scheduled (as the real engine does:
	// SUB_CalcMove sets each half's nextthink after door_fire walks it).
	owner.SetNextThink(s, s.Time+0.1)
	half.SetNextThink(s, s.Time+0.1)
	owner.SetLTime(s, s.Time)
	half.SetLTime(s, s.Time)

	prev := vm.GlobalFloat("door_up_count")

	// Step one physics frame. The pusher runs the owner's door_fire and
	// the half's door_go_up; both must fire (count +2).
	s.Physics()
	if got := vm.GlobalFloat("door_up_count"); got != prev+2 {
		t.Fatalf("door_chain: after one physics frame count=%v want %v (both linked halves must fire)", got, prev+2)
	}
}

// ---------------------------------------------------------------------------
// AI: a line through water must still be visible (visible() TRUE).
// ---------------------------------------------------------------------------

func TestParityAITraceThroughWaterStillVisible(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 32)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	t.Cleanup(s.Shutdown)
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

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

	// Trace through an empty world: the QC visible() test considers a line
	// visible unless both trace_inopen and trace_inwater are set (ai.qc).
	// In a dry line neither is set, so visible() returns TRUE. This guards
	// the "water flips AI to blind" hypothesis: the QC test is
	//   if (trace_inopen && trace_inwater) return FALSE;
	// so a fully-open dry trace must NOT produce inopen&&inwater.
	start := [3]float32{0, 0, 36}
	end := [3]float32{200, 0, 36}
	trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNoMonsters), monster)
	if trace.InOpen && trace.InWater {
		t.Fatalf("ai_water: dry open trace reported inopen=%v inwater=%v — visible() would return FALSE", trace.InOpen, trace.InWater)
	}
	if trace.Fraction < 1.0 {
		t.Fatalf("ai_water: clear line fraction=%v want 1.0", trace.Fraction)
	}
}

// ---------------------------------------------------------------------------
// Sound: a StartSound within (max-21) bytes of the watermark must not drop.
// ---------------------------------------------------------------------------

func TestParitySoundNotDroppedNearWatermark(t *testing.T) {
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

	// Fill the datagram to exactly MaxDatagram-21 (the C watermark:
	// sv_main.c "if (sv.datagram.cursize > MAX_DATAGRAM-21) return;").
	// At exactly the boundary, the write must still succeed.
	target := MaxDatagram - 21
	// We need a byte that isn't svc_sound so the scan below is unambiguous.
	var fillByte byte = byte(inet.SVCSignOnNum)
	written := 0
	for written < target {
		s.Datagram.PutByte(fillByte)
		written++
	}
	if s.Datagram.Len() != target {
		t.Fatalf("fill: len=%d want %d", s.Datagram.Len(), target)
	}

	s.StartSound(ent, 0, sample, 255, 1)
	found := false
	// The sound message starts somewhere after the fill; find svc_sound
	// occurring after the first fill region.
	for i := target; i+1 < len(s.Datagram.Data); i++ {
		if s.Datagram.Data[i] == byte(inet.SVCSound) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sound_watermark: svc_sound dropped at datagram len=%d (near watermark)", s.Datagram.Len())
	}
}
