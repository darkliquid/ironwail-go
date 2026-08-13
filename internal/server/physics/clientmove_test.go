// clientmove_test.go verifies the client movement simulation in isolation,
// mirroring the behavior the root SV_ClientThink parity tests protect.
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)



// mockFacade implements the PhysicsFacade surface used by the mover and the
// migrated leaf tests. Unused seams return zero values so tests can construct
// it without boilerplate.
type mockFacade struct {
	time       float32
	frameTime  float32
	gravity    float32
	friction   float32
	stopSpeed  float32
	cvars      map[string]float64
	vm         *qc.VM
	store      srvtypes.EntityStore
	maxClients int
	// facadeHandle is the ServerHandle passed to leaf algorithms; when nil,
	// newMockLeafSystem constructs a fresh handle with the facade's VM, so
	// gravity field reads and other ServerHandle reads behave like tests.
	facadeHandle srvtypes.ServerHandle
	// sounds collects StartSound sample names when non-nil.
	sounds []string
	// moved/from are the PushMoveScratch buffers.
	moved []*srvtypes.Edict
	from  [][3]float32
	// runThink overrides the RunThink gate when non-nil.
	runThink func(ent *srvtypes.Edict) bool
	// runExecute overrides ExecuteQCFunction when non-nil (pusher gate tests).
	runExecute func(funcIdx int) error
	// execCount records ExecuteQCFunction invocations when runExecute is set.
	execCount int
}

func (m *mockFacade) GetTime() float32      { return m.time }
func (m *mockFacade) GetFrameTime() float32 { return m.frameTime }
func (m *mockFacade) GetGravity() float32   { return m.gravity }
func (m *mockFacade) GetMaxVelocity() float32 { return 2000 }
func (m *mockFacade) GetFriction() float32  { return m.friction }
func (m *mockFacade) GetStopSpeed() float32 { return m.stopSpeed }
func (m *mockFacade) BoolValue(name string) bool {
	return m.cvars != nil && m.cvars[name] != 0
}
func (m *mockFacade) Get(name string) srvtypes.CvarHandle {
	if m.cvars != nil && m.cvars[name] != 0 {
		return m
	}
	return nil
}
func (m *mockFacade) Bool() bool                       { return true }
func (m *mockFacade) Float32() float32                 { return 0 }
func (m *mockFacade) FloatValue(name string) float64 {
	if m.cvars == nil {
		return 0
	}
	return m.cvars[name]
}
func (m *mockFacade) GetVM() *qc.VM                    { return m.vm }
func (m *mockFacade) GetNumEdicts() int {
	if m.store != nil {
		return m.store.GetNumEdicts()
	}
	return 0
}
func (m *mockFacade) NumForEdict(ent *srvtypes.Edict) int { return ent.Num }
func (m *mockFacade) MaxClients() int                    { return m.maxClients }
func (m *mockFacade) EdictNum(num int) *srvtypes.Edict   { return nil }
func (m *mockFacade) EventsEnabled() bool { return false }
func (m *mockFacade) BeginFrame(serverTime, frameTime float32) {}
func (m *mockFacade) EndFrame()                               {}
func (m *mockFacade) LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict, format string, args ...any) bool {
	return false
}
func (m *mockFacade) RunThink(ent *srvtypes.Edict) bool {
	if m.runThink != nil {
		return m.runThink(ent)
	}
	return true
}
func (m *mockFacade) Impact(e1, e2 *srvtypes.Edict)          {}
func (m *mockFacade) ExecuteQCFunction(funcIdx int) error {
	if m.runExecute != nil {
		m.execCount++
		return m.runExecute(funcIdx)
	}
	return nil
}
func (m *mockFacade) SyncSpawnedEdictsFromQCVM(startEntNum int) {}
func (m *mockFacade) SetQCTimeGlobal(time float32)           {}
func (m *mockFacade) StartSound(ent *srvtypes.Edict, channel int, sample string, volume int, attenuation float32) {
	if m.sounds != nil {
		m.sounds = append(m.sounds, sample)
	}
}
func (m *mockFacade) SuppressTouchQC() bool    { return false }
func (m *mockFacade) PlayerClient(ent *srvtypes.Edict) *srvtypes.Client { return nil }
func (m *mockFacade) RunClientQCThinkWithMode(client *srvtypes.Client, funcName string, fullSync bool) {}
func (m *mockFacade) DebugTriggerTouch(source string, touch, other *srvtypes.Edict) {}
func (m *mockFacade) PushMoveScratch() (moved *[]*srvtypes.Edict, from *[][3]float32) {
	return &m.moved, &m.from
}
func (m *mockFacade) GetFieldGravity() int          { return -1 }
func (m *mockFacade) CaptureExecutionContext() any { return nil }
func (m *mockFacade) RestoreExecutionContext(ctx any) {}

func TestClientMoverWalkProducesHorizontalVelocity(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	facade := &mockFacade{time: 1.0, frameTime: 0.05, gravity: 800, friction: 4, stopSpeed: 100}
	mover := NewClientMover(facade, &mockCollisionWorld{}, h)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	ent.SetHealth(h, 100)
	ent.SetFlags(h, float32(srvtypes.FlagOnGround))
	ent.SetVAngle(h, [3]float32{60, 0, 0})

	client := &srvtypes.Client{
		Edict: ent,
		LastCmd: srvtypes.UserCmd{
			ForwardMove: 200,
		},
	}

	mover.SV_ClientThink(client)

	vel := ent.Velocity(h)
	if vel[2] != 0 {
		t.Errorf("walk velocity z = %v, want 0", vel[2])
	}
	if vel[0] == 0 && vel[1] == 0 {
		t.Errorf("walk forward move did not produce horizontal velocity: %v", vel)
	}
}

func TestCalcRollFlatReturnsZero(t *testing.T) {
	// No lateral velocity -> no roll.
	if got := CalcRoll(nil, [3]float32{0, 0, 0}, [3]float32{0, 0, 0}); got != 0 {
		t.Errorf("CalcRoll flat = %v, want 0", got)
	}
}
