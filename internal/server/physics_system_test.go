// physics_system_test.go tests the PhysicsSystem component in isolation using mocks.

// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// Mock collision world for isolated testing.
type mockCollisionWorld struct {
	moveTrace TraceResult
	contents  int
}

func (m *mockCollisionWorld) SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult {
	if m.moveTrace.Fraction != 0 {
		tr := m.moveTrace
		if tr.EndPos == [3]float32{} {
			tr.EndPos = end
		}
		return tr
	}
	return TraceResult{Fraction: 1, EndPos: end}
}


func (m *mockCollisionWorld) SV_TestEntityPosition(ent *Edict) *Edict { return nil }

func (m *mockCollisionWorld) SV_HullForEntity(ent *Edict, mins, maxs [3]float32) (*model.Hull, [3]float32) {
	return nil, [3]float32{}
}

func (m *mockCollisionWorld) LinkEdict(ent *Edict, touchTriggers bool) {}

func (m *mockCollisionWorld) PointContents(p [3]float32) int {
	return m.contents
}

// Mock entity store for isolated testing.
type mockEntityStore struct {
	edicts []*Edict
}

func (m *mockEntityStore) EdictNum(num int) *Edict {
	if num < 0 || num >= len(m.edicts) {
		return nil
	}
	return m.edicts[num]
}

func (m *mockEntityStore) AllocEdict() *Edict {
	e := &Edict{Num: len(m.edicts)}
	m.edicts = append(m.edicts, e)
	return e
}

func (m *mockEntityStore) FreeEdict(ed *Edict) {
	if ed != nil {
		ed.Free = true
	}
}

// Mock physics config.
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

// Mock frame timing.
type mockFrameTiming struct {
	time      float32
	frameTime float32
}

func (m *mockFrameTiming) GetTime() float32      { return m.time }
func (m *mockFrameTiming) GetFrameTime() float32 { return m.frameTime }

// Mock think executor.
type mockThinkExecutor struct{}

func (m *mockThinkExecutor) RunThink(ent *Edict) bool         { return true }
func (m *mockThinkExecutor) ExecuteQCFunction(funcIdx int) error { return nil }

func TestPhysicsSystemCheckBottomSolidGround(t *testing.T) {
	col := &mockCollisionWorld{contents: bsp.ContentsSolid}
	store := &mockEntityStore{}
	cfg := &mockPhysicsConfig{gravity: 800, maxVelocity: 2000}
	timing := &mockFrameTiming{time: 1.0, frameTime: 0.05}
	exec := &mockThinkExecutor{}

	s := NewServer()
	sys := NewPhysicsSystem(col, store, cfg, timing, exec, s)

	ent := store.AllocEdict()

	// When beneath entity is solid, CheckBottom returns true
	if !sys.CheckBottom(ent) {
		t.Errorf("CheckBottom returned false for solid ground, expected true")
	}
}

func TestPhysicsSystemMoveStepInIsolation(t *testing.T) {
	col := &mockCollisionWorld{
		contents:  bsp.ContentsSolid,
		moveTrace: TraceResult{Fraction: 0.5, EndPos: [3]float32{10, 0, 0}},
	}
	store := &mockEntityStore{}
	cfg := &mockPhysicsConfig{gravity: 800, maxVelocity: 2000}
	timing := &mockFrameTiming{time: 1.0, frameTime: 0.05}
	exec := &mockThinkExecutor{}

	s := NewServer()
	sys := NewPhysicsSystem(col, store, cfg, timing, exec, s)

	ent := store.AllocEdict()

	moved := sys.MoveStep(ent, [3]float32{10, 0, 0}, false)
	if !moved {
		t.Errorf("MoveStep returned false for clear path, expected true")
	}
}

func TestPhysicsSystemStepDirectionWithMocks(t *testing.T) {
	col := &mockCollisionWorld{
		contents:  bsp.ContentsSolid,
		moveTrace: TraceResult{Fraction: 0.5, EndPos: [3]float32{16, 0, 0}},
	}
	store := &mockEntityStore{}
	cfg := &mockPhysicsConfig{gravity: 800, maxVelocity: 2000}
	timing := &mockFrameTiming{time: 1.0, frameTime: 0.05}
	exec := &mockThinkExecutor{}

	s := NewServer()
	sys := NewPhysicsSystem(col, store, cfg, timing, exec, s)

	ent := store.AllocEdict()

	stepped := sys.StepDirection(ent, 0, 16)
	if !stepped {
		t.Errorf("StepDirection returned false for clear path, expected true")
	}
}

