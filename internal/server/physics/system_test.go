// system_test.go tests the Physics System component in isolation using mocks.
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type mockCollisionWorld struct {
	moveTrace srvtypes.TraceResult
	contents  int
}

func (m *mockCollisionWorld) SV_Move(start, mins, maxs, end [3]float32, moveType srvtypes.MoveType, passedict *srvtypes.Edict) srvtypes.TraceResult {
	if m.moveTrace.Fraction != 0 {
		tr := m.moveTrace
		if tr.EndPos == [3]float32{} {
			tr.EndPos = end
		}
		return tr
	}
	return srvtypes.TraceResult{Fraction: 0.5, EndPos: end}
}

func (m *mockCollisionWorld) SV_TestEntityPosition(ent *srvtypes.Edict) *srvtypes.Edict { return nil }

func (m *mockCollisionWorld) SV_HullForEntity(ent *srvtypes.Edict, mins, maxs [3]float32) (*model.Hull, [3]float32) {
	return nil, [3]float32{}
}

func (m *mockCollisionWorld) LinkEdict(ent *srvtypes.Edict, touchTriggers bool) {}

func (m *mockCollisionWorld) PointContents(p [3]float32) int {
	return m.contents
}

type mockEntityStore struct {
	edicts []*srvtypes.Edict
}

func (m *mockEntityStore) EdictNum(num int) *srvtypes.Edict {
	if num < 0 || num >= len(m.edicts) {
		return nil
	}
	return m.edicts[num]
}

func (m *mockEntityStore) AllocEdict() *srvtypes.Edict {
	e := &srvtypes.Edict{Num: len(m.edicts)}
	m.edicts = append(m.edicts, e)
	return e
}

func (m *mockEntityStore) FreeEdict(ed *srvtypes.Edict) {
	if ed != nil {
		ed.Free = true
	}
}

func (m *mockEntityStore) GetNumEdicts() int { return len(m.edicts) }
func (m *mockEntityStore) GetMaxEdicts() int { return len(m.edicts) }

type mockPhysicsConfig struct {
	gravity     float32
	maxVelocity float32
	friction    float32
	stopSpeed   float32
}

func (m *mockPhysicsConfig) GetGravity() float32     { return m.gravity }
func (m *mockPhysicsConfig) GetMaxVelocity() float32 { return m.maxVelocity }
func (m *mockPhysicsConfig) GetFriction() float32    { return m.friction }
func (m *mockPhysicsConfig) GetStopSpeed() float32   { return m.stopSpeed }

type mockFrameTiming struct {
	time      float32
	frameTime float32
}

func (m *mockFrameTiming) GetTime() float32      { return m.time }
func (m *mockFrameTiming) GetFrameTime() float32 { return m.frameTime }

type mockThinkExecutor struct{}

func (m *mockThinkExecutor) RunThink(ent *srvtypes.Edict) bool   { return true }
func (m *mockThinkExecutor) ExecuteQCFunction(funcIdx int) error { return nil }

func TestPhysicsSystemCheckBottomSolidGround(t *testing.T) {
	col := &mockCollisionWorld{contents: bsp.ContentsSolid}
	store := &mockEntityStore{}

	sys := NewSystem(col, store, nil)

	ent := &srvtypes.Edict{Num: 1}

	if !sys.CheckBottom(ent) {
		t.Errorf("CheckBottom() = false, want true for solid ground")
	}
}

func TestPhysicsSystemMoveStepInIsolation(t *testing.T) {
	col := &mockCollisionWorld{contents: bsp.ContentsSolid}
	store := &mockEntityStore{}

	sys := NewSystem(col, store, nil)

	ent := &srvtypes.Edict{Num: 1}

	moved := sys.MoveStep(ent, [3]float32{10, 0, 0}, false)
	if !moved {
		t.Errorf("MoveStep() = false, want true for clear move")
	}
}

func TestPhysicsSystemStepDirectionWithMocks(t *testing.T) {
	col := &mockCollisionWorld{contents: bsp.ContentsSolid}
	store := &mockEntityStore{}

	sys := NewSystem(col, store, nil)

	ent := &srvtypes.Edict{Num: 1}

	stepped := sys.StepDirection(ent, 90.0, 16.0)
	if !stepped {
		t.Errorf("StepDirection() = false, want true for open space step")
	}
}
