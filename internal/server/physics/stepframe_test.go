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

func (h *handle) GetVM() *qc.VM           { return h.vm }
func (h *handle) String(idx int32) string { return "" }
func (h *handle) GetFieldAlpha() int      { return -1 }
func (h *handle) GetFieldScale() int      { return -1 }
func (h *handle) GetFieldGravity() int    { return -1 }
func (h *handle) GetFieldItems2() int     { return -1 }
func (h *handle) GetFieldState() int      { return -1 }
func (h *handle) GetFieldWait() int       { return -1 }
func (h *handle) GetFieldSpeed() int      { return -1 }
func (h *handle) GetFieldCustomFlags() int { return -1 }
func (h *handle) GetFieldThCheckAttack() int { return -1 }
func (h *handle) GetFieldMap() int        { return -1 }

// mockFrameDriver implements the FrameDriver surface for loop tests.
type mockFrameDriver struct {
	cvars      map[string]bool
	maxClients int
	vm         *qc.VM
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
func (m *mockFrameDriver) EventsEnabled() bool   { return false }
func (m *mockFrameDriver) BeginFrame(a, b float32) {}
func (m *mockFrameDriver) EndFrame()               {}
func (m *mockFrameDriver) LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict, format string, args ...any) bool {
	return false
}
func (m *mockFrameDriver) GetTime() float32       { return 0 }
func (m *mockFrameDriver) GetFrameTime() float32  { return 0 }
func (m *mockFrameDriver) MaxClients() int        { return m.maxClients }
func (m *mockFrameDriver) RecordDevStatsEdicts(active int) {}
func (m *mockFrameDriver) GetVM() *qc.VM          { return m.vm }
func (m *mockFrameDriver) SyncQCVMGlobals()       {}
func (m *mockFrameDriver) SetQCTimeGlobal(t float32) {}
func (m *mockFrameDriver) ExecuteQCFunction(i int) error { return nil }
func (m *mockFrameDriver) PlayerClient(ent *srvtypes.Edict) *srvtypes.Client { return nil }
func (m *mockFrameDriver) RunClientQCThinkWithMode(c *srvtypes.Client, name string, full bool) {}
func (m *mockFrameDriver) SyncSpawnedEdictsFromQCVM(start int) {}

type cvarTrue struct{}

func (cvarTrue) Bool() bool { return true }

// mockDispatch counts movetype dispatch calls.
type mockDispatch struct {
	walk, toss, none, step, push, noclip int
}

func (d *mockDispatch) PhysicsPusher(ent *srvtypes.Edict) { d.push++ }
func (d *mockDispatch) PhysicsNone(ent *srvtypes.Edict)   { d.none++ }
func (d *mockDispatch) PhysicsNoClip(ent *srvtypes.Edict) { d.noclip++ }
func (d *mockDispatch) PhysicsStep(ent *srvtypes.Edict)   { d.step++ }
func (d *mockDispatch) PhysicsToss(ent *srvtypes.Edict)   { d.toss++ }
func (d *mockDispatch) PhysicsWalk(ent *srvtypes.Edict)   { d.walk++ }

func TestStepFrameDispatchesMovetypes(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 128
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	walkEnt := &srvtypes.Edict{Num: 1}
	tossEnt := &srvtypes.Edict{Num: 2}
	walkEnt.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	tossEnt.SetMoveType(h, float32(srvtypes.MoveTypeToss))

	col := &mockCollisionWorld{}
	store := &mockEntityStore{edicts: []*srvtypes.Edict{walkEnt, tossEnt}}

	driver := &mockFrameDriver{
		cvars:      map[string]bool{"sv_freezenonclients": false},
		maxClients: 2,
		vm:         vm,
	}

	sys := NewSystem(col, store, h)
	dispatch := &mockDispatch{}

	newTime := sys.StepFrame(driver, dispatch, 1.0, 0.1)

	if newTime != 1.1 {
		t.Errorf("StepFrame time = %v, want 1.1", newTime)
	}
	if dispatch.walk != 1 {
		t.Errorf("walk dispatch = %d, want 1", dispatch.walk)
	}
	if dispatch.toss != 1 {
		t.Errorf("toss dispatch = %d, want 1", dispatch.toss)
	}
	if dispatch.none != 0 || dispatch.push != 0 || dispatch.step != 0 || dispatch.noclip != 0 {
		t.Errorf("unexpected dispatches: none=%d push=%d step=%d noclip=%d",
			dispatch.none, dispatch.push, dispatch.step, dispatch.noclip)
	}
}

func TestStepFrameFreezeNonClientsSkipsEdictsAndTime(t *testing.T) {
	vm := qc.NewVM()
	h := &handle{vm: vm}

	walkEnt := &srvtypes.Edict{Num: 1}
	walkEnt.SetMoveType(h, float32(srvtypes.MoveTypeWalk))

	col := &mockCollisionWorld{}
	store := &mockEntityStore{edicts: []*srvtypes.Edict{walkEnt}}

	driver := &mockFrameDriver{
		cvars:      map[string]bool{"sv_freezenonclients": true},
		maxClients: 0, // cap = 0+1 = 1: the walk edict at index 1 is skipped
		vm:         vm,
	}

	sys := NewSystem(col, store, h)
	dispatch := &mockDispatch{}

	newTime := sys.StepFrame(driver, dispatch, 1.0, 0.1)

	if newTime != 1.0 {
		t.Errorf("StepFrame freeze time = %v, want 1.0 (time frozen)", newTime)
	}
	if dispatch.walk != 0 {
		t.Errorf("walk dispatch = %d, want 0 (edict beyond client cap skipped)", dispatch.walk)
	}
}
