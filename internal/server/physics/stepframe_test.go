// stepframe_test.go verifies the frame physics loop drives movetype dispatch
// and time advance in isolation, using mocks for every injected dependency.
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// handle is a minimal ServerHandle wired to a real VM so Edict accessors
// (MoveType, etc.) resolve through the VM edict table like production.
type handle struct {
	vm *qc.VM
}

func (h *handle) GetVM() *qc.VM              { return h.vm }
func (h *handle) String(idx int32) string    { return "" }
func (h *handle) GetFieldAlpha() int         { return -1 }
func (h *handle) GetFieldScale() int         { return -1 }
func (h *handle) GetFieldGravity() int       { return -1 }
func (h *handle) GetFieldItems2() int        { return -1 }
func (h *handle) GetFieldState() int         { return -1 }
func (h *handle) GetFieldWait() int          { return -1 }
func (h *handle) GetFieldSpeed() int         { return -1 }
func (h *handle) GetFieldCustomFlags() int   { return -1 }
func (h *handle) GetFieldThCheckAttack() int { return -1 }
func (h *handle) GetFieldMap() int           { return -1 }

// mockFrameDriver implements the FrameDriver surface for loop tests.
type mockFrameDriver struct {
	cvars      map[string]bool
	maxClients int
	vm         *qc.VM
	// pclients maps edict Num -> Client for slots that are active+spawned.
	pclients map[int]*srvtypes.Client
}

func (m *mockFrameDriver) BoolValue(name string) bool {
	if m.cvars == nil {
		return false
	}
	return m.cvars[name]
}
func (m *mockFrameDriver) Get(name string) srvtypes.CvarHandle {
	if m.cvars != nil && m.cvars[name] {
		return cvarTrue{}
	}
	return nil
}
func (m *mockFrameDriver) EventsEnabled() bool     { return false }
func (m *mockFrameDriver) BeginFrame(a, b float32) {}
func (m *mockFrameDriver) EndFrame()               {}
func (m *mockFrameDriver) LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict, format string, args ...any) bool {
	return false
}
func (m *mockFrameDriver) GetTime() float32                { return 0 }
func (m *mockFrameDriver) GetFrameTime() float32           { return 0 }
func (m *mockFrameDriver) MaxClients() int                 { return m.maxClients }
func (m *mockFrameDriver) RecordDevStatsEdicts(active int) {}
func (m *mockFrameDriver) GetVM() *qc.VM                   { return m.vm }
func (m *mockFrameDriver) SyncQCVMGlobals()                {}
func (m *mockFrameDriver) SetQCTimeGlobal(t float32)       {}
func (m *mockFrameDriver) ExecuteQCFunction(i int) error   { return nil }
func (m *mockFrameDriver) PlayerClient(ent *srvtypes.Edict) *srvtypes.Client {
	if m.pclients == nil {
		return nil
	}
	return m.pclients[ent.Num]
}
func (m *mockFrameDriver) RunClientQCThinkWithMode(c *srvtypes.Client, name string, full bool) {}
func (m *mockFrameDriver) SyncSpawnedEdictsFromQCVM(start int)                                 {}

type cvarTrue struct{}

func (cvarTrue) Bool() bool       { return true }
func (cvarTrue) Float32() float32 { return 0 }

func TestStepFrameDispatchesMovetypes(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	walkEnt := &srvtypes.Edict{Num: 1}
	tossEnt := &srvtypes.Edict{Num: 2}
	noneEnt := &srvtypes.Edict{Num: 3}
	walkEnt.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	walkEnt.SetOrigin(h, [3]float32{0, 0, 0})
	walkEnt.SetVelocity(h, [3]float32{0, 0, -100})
	walkEnt.SetMins(h, [3]float32{-16, -16, -24})
	walkEnt.SetMaxs(h, [3]float32{16, 16, 32})
	tossEnt.SetMoveType(h, float32(srvtypes.MoveTypeToss))
	tossEnt.SetOrigin(h, [3]float32{0, 0, 32})
	tossEnt.SetVelocity(h, [3]float32{0, 0, 0})
	tossEnt.SetMins(h, [3]float32{-16, -16, -24})
	tossEnt.SetMaxs(h, [3]float32{16, 16, 32})
	noneEnt.SetMoveType(h, float32(srvtypes.MoveTypeNone))
	noneEnt.SetOrigin(h, [3]float32{5, 5, 5})

	col := &mockCollisionWorld{}
	store := &mockEntityStore{edicts: []*srvtypes.Edict{walkEnt, tossEnt, noneEnt}}
	// maxClients 0: no client slots, so edicts 1-3 are plain entities and all
	// dispatch through their movetypes (a dedicated server with no clients).
	facade := &mockFacade{frameTime: 0.1, gravity: 800, vm: vm, store: store, maxClients: 0}

	driver := &mockFrameDriver{
		cvars:      map[string]bool{"sv_freezenonclients": false},
		maxClients: 0,
		vm:         vm,
	}

	sys := NewSystemWithFacade(col, store, h, facade)

	newTime := sys.StepFrame(driver, 1.0, 0.1)

	if newTime != 1.1 {
		t.Errorf("StepFrame time = %v, want 1.1", newTime)
	}

	// MoveTypeWalk: velocity integrated -> descending origin.
	if got := walkEnt.Origin(h)[2]; got >= 0 {
		t.Errorf("walk origin z = %v, want < 0 after gravity frame", got)
	}
	// MoveTypeToss: gravity applied -> velocity z decreased below 0.
	if got := tossEnt.Velocity(h)[2]; got >= 0 {
		t.Errorf("toss velocity z = %v, want < 0 after gravity frame", got)
	}
	// MoveTypeNone: no movement.
	if got := noneEnt.Origin(h); got != [3]float32{5, 5, 5} {
		t.Errorf("none origin = %v, want [5 5 5]", got)
	}
}

func TestStepFrameFreezeNonClientsSkipsEdictsAndTime(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	walkEnt := &srvtypes.Edict{Num: 1}
	walkEnt.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	walkEnt.SetOrigin(h, [3]float32{0, 0, 0})
	walkEnt.SetVelocity(h, [3]float32{0, 0, -100})
	walkEnt.SetMins(h, [3]float32{-16, -16, -24})
	walkEnt.SetMaxs(h, [3]float32{16, 16, 32})

	// Index 0 is the world edict (nil here); walkEnt is a non-client edict
	// beyond the freeze cap, so it must be skipped.
	col := &mockCollisionWorld{}
	store := &mockEntityStore{edicts: []*srvtypes.Edict{nil, walkEnt}}
	facade := &mockFacade{frameTime: 0.1, gravity: 800, vm: vm, store: store, maxClients: 0}

	driver := &mockFrameDriver{
		cvars:      map[string]bool{"sv_freezenonclients": true},
		maxClients: 0, // cap = 0+1 = 1: the walk edict at index 1 is skipped
		vm:         vm,
	}

	sys := NewSystemWithFacade(col, store, h, facade)

	newTime := sys.StepFrame(driver, 1.0, 0.1)

	if newTime != 1.0 {
		t.Errorf("StepFrame freeze time = %v, want 1.0 (time frozen)", newTime)
	}
	if got := walkEnt.Origin(h); got != [3]float32{0, 0, 0} {
		t.Errorf("walk origin = %v, want [0 0 0] (edict beyond client cap skipped)", got)
	}
}

// TestStepFrameSkipsInactiveClientSlots pings C parity for SV_Physics_Client's
// inactive-slot early return.
//
// Where in C: SV_Physics_Client in sv_phys.c:946-956:
//
//	if (!svs.clients[num-1].active)
//		return; // unconnected slot
//
// C returns BEFORE PlayerPreThink and BEFORE the movetype switch for any
// client slot that is not active. Go's StepFrame now does the same: an edict
// in 1..maxclients whose PlayerClient is nil (inactive/unspawned) is skipped
// entirely — it must NOT run PhysicsWalk/Noclip/etc. (which previously
// applied gravity and moved the standing edict).
func TestStepFrameSkipsInactiveClientSlots(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	// Edict 1 = inactive client slot (PlayerClient returns nil -> must skip).
	inactiveEnt := &srvtypes.Edict{Num: 1}
	inactiveEnt.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	inactiveEnt.SetOrigin(h, [3]float32{0, 0, 0})
	inactiveEnt.SetVelocity(h, [3]float32{0, 0, -100}) // would fall if PhysicsWalk ran
	inactiveEnt.SetMins(h, [3]float32{-16, -16, -24})
	inactiveEnt.SetMaxs(h, [3]float32{16, 16, 32})

	col := &mockCollisionWorld{}
	store := &mockEntityStore{edicts: []*srvtypes.Edict{nil, inactiveEnt}}
	facade := &mockFacade{frameTime: 0.1, gravity: 800, vm: vm, store: store, maxClients: 1}

	driver := &mockFrameDriver{
		cvars:      map[string]bool{"sv_freezenonclients": false},
		maxClients: 1,
		vm:         vm,
		// NOTE: pclients is empty -> slot 1 is inactive.
	}

	sys := NewSystemWithFacade(col, store, h, facade)

	newTime := sys.StepFrame(driver, 1.0, 0.1)

	// Time still advances (frame ran, world/loop bookkeeping).
	if newTime != 1.1 {
		t.Errorf("StepFrame time = %v, want 1.1", newTime)
	}
	// The inactive slot's edict must be untouched: no gravity, no walkmove.
	if got := inactiveEnt.Origin(h); got != [3]float32{0, 0, 0} {
		t.Errorf("inactive client origin = %v, want [0 0 0] (slot skipped like C)", got)
	}
	if got := inactiveEnt.Velocity(h); got != [3]float32{0, 0, -100} {
		t.Errorf("inactive client velocity = %v, want unchanged [0 0 -100]", got)
	}
}
