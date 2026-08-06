// clientmove_test.go verifies the client movement simulation in isolation,
// mirroring the behavior the root SV_ClientThink parity tests protect.
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// mockCVarReader implements CVarReader for the mover tests.
type mockCVarReader struct{}

func (mockCVarReader) BoolValue(name string) bool { return false }
func (mockCVarReader) Get(name string) srvtypes.CvarHandle {
	return mockCvar{}
}

type mockCvar struct{}

func (mockCvar) Bool() bool            { return false }
func (mockCvar) Float32() float32      { return 0 }

// mockFacade implements the PhysicsFacade surface used by the mover.
type mockFacade struct {
	time      float32
	frameTime float32
	gravity   float32
	friction  float32
	stopSpeed float32
}

func (m *mockFacade) GetTime() float32         { return m.time }
func (m *mockFacade) GetFrameTime() float32    { return m.frameTime }
func (m *mockFacade) GetGravity() float32      { return m.gravity }
func (m *mockFacade) GetMaxVelocity() float32  { return 2000 }
func (m *mockFacade) GetFriction() float32     { return m.friction }
func (m *mockFacade) GetStopSpeed() float32    { return m.stopSpeed }
func (m *mockFacade) BoolValue(name string) bool { return false }
func (m *mockFacade) Get(name string) srvtypes.CvarHandle { return m }
func (m *mockFacade) Bool() bool { return false }
func (m *mockFacade) Float32() float32 { return 0 }

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
