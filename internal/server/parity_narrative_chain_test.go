package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// TestParityNarrativeDoorChainOrderedHops is the H2 narrative-chain truth
// table: it walks the full trigger→door cascade and asserts the EXACT ordered
// hop list, not just the final outcome. An ordering bug (e.g. the pusher
// think firing before the touch that schedules it, or a re-arm firing twice)
// fails this test even when the final door state looks right.
//
// Where in C: door_fire / door_go_up / SUB_UseTargets in doors.qc, and
// SV_Physics_Pusher in sv_phys.c.
func TestParityNarrativeDoorChainOrderedHops(t *testing.T) {
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

	// doors: owner + linked half.
	owner := s.AllocEdict()
	half := s.AllocEdict()
	for _, door := range []*Edict{owner, half} {
		door.SetMoveType(s, float32(MoveTypePush))
		door.SetSolid(s, float32(SolidBSP))
		door.SetOrigin(s, qtypes.Vec3{})
		door.SetMins(s, qtypes.Vec3{X: -32, Y: -32, Z: 0})
		door.SetMaxs(s, qtypes.Vec3{X: 32, Y: 32, Z: 72})
		door.SetVelocity(s, qtypes.Vec3{X: 0, Y: 0, Z: 100})
		s.LinkEdict(door, false)
	}
	owner.SetEnemy(s, int32(s.NumForEdict(half)))
	half.SetEnemy(s, int32(s.NumForEdict(owner)))
	owner.SetOwner(s, int32(s.NumForEdict(owner)))
	half.SetOwner(s, int32(s.NumForEdict(owner)))

	// A recorded hop log, appended by a recording builtin inside each QC fn.
	var hops []string
	const hopBuiltinOfs = 42
	vm := s.QCVM
	// Register the globals PhysicsPusher writes (self/other/time) so
	// SetGlobal("self", ...) resolves and the hop builtin can read the
	// running edict by offset.
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Builtins[1] = func(vm *qc.VM) {
		// self is the running edict; read its classname field to name the hop.
		self := int(vm.GInt(qc.OFSSelf))
		switch self {
		case s.NumForEdict(owner):
			hops = append(hops, "owner")
		case s.NumForEdict(half):
			hops = append(hops, "half")
		default:
			hops = append(hops, "other")
		}
	}
	vm.SetGInt(hopBuiltinOfs, -1)

	// Functions: 1=door_fire (owner), 2=door_go_up (halves). Each calls the
	// hop builtin first (records the running edict) then schedules its own
	// next think; door_fire additionally schedules the half (the "chain").
	vm.Functions = []qc.DFunction{{}}
	vm.Functions = append(vm.Functions, qc.DFunction{Name: vm.AllocString("door_fire"), FirstStatement: int32(len(vm.Statements))})
	doorFire := int32(len(vm.Functions) - 1)
	vm.Functions = append(vm.Functions, qc.DFunction{Name: vm.AllocString("door_go_up"), FirstStatement: int32(len(vm.Statements))})
	doorGoUp := int32(len(vm.Functions) - 1)

	vm.Statements = append(vm.Statements,
		qc.DStatement{Op: uint16(qc.OPCall0), A: uint16(hopBuiltinOfs)},
		qc.DStatement{Op: uint16(qc.OPDone)},
		qc.DStatement{Op: uint16(qc.OPCall0), A: uint16(hopBuiltinOfs)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)

	owner.SetThink(s, doorFire)
	half.SetThink(s, doorGoUp)
	owner.SetNextThink(s, s.Time+0.1)
	half.SetNextThink(s, s.Time+0.1)
	owner.SetLTime(s, s.Time)
	half.SetLTime(s, s.Time)

	// One physics frame: StepFrame runs each edict's pusher think in edict
	// order (owner first, then half). Both must fire exactly once, in order.
	s.Physics()

	want := []string{"owner", "half"}
	if len(hops) != len(want) {
		t.Fatalf("hop count = %d (%v), want %d %v — a think fired twice or was skipped", len(hops), hops, len(want), want)
	}
	for i := range want {
		if hops[i] != want[i] {
			t.Fatalf("hop[%d] = %q, want %q (full: %v) — chain order broken or duplicated", i, hops[i], want[i], hops)
		}
	}
}
