// leafs_test.go verifies the migrated per-entity physics leaf algorithms in
// isolation against mocks, documenting the parity behaviors they must retain
// (mirroring sv_phys.c).
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestClipVelocitySlidesOffSurface(t *testing.T) {
	// Head-on into a wall: velocity along the normal is removed.
	out := ClipVelocity(qtypes.Vec3{X: 100, Y: 0, Z: 0}, qtypes.Vec3{X: 1, Y: 0, Z: 0}, 1)
	if out.X != 0 || out.Y != 0 || out.Z != 0 {
		t.Errorf("ClipVelocity head-on = %v, want [0 0 0]", out)
	}

	// 45-degree slide: X is clipped, Y preserved.
	out = ClipVelocity(qtypes.Vec3{X: 100, Y: 100, Z: 0}, qtypes.Vec3{X: 1, Y: 0, Z: 0}, 1)
	if out.X != 0 || out.Y != 100 {
		t.Errorf("ClipVelocity slide = %v, want [0 100 0]", out)
	}
}

func TestSVCheckWaterDetectsSubmersion(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)

	col := &mockCollisionWorld{contents: bsp.ContentsWater}
	sys := NewSystem(col, &mockEntityStore{}, &handle{vm: vm})

	ent := &srvtypes.Edict{Num: 1}
	ent.SetMins(&handle{vm: vm}, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(&handle{vm: vm}, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetOrigin(&handle{vm: vm}, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	ent.SetViewOfs(&handle{vm: vm}, qtypes.Vec3{X: 0, Y: 0, Z: 22})

	if !sys.SV_CheckWater(ent) {
		t.Error("SV_CheckWater = false, want true (entity in water)")
	}
	if ent.WaterLevel(&handle{vm: vm}) != 3 {
		t.Errorf("WaterLevel = %v, want 3 (fully submerged)", ent.WaterLevel(&handle{vm: vm}))
	}
}

func TestPushEntityMovesOriginAndRelinks(t *testing.T) {
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	col := &mockCollisionWorld{}
	sys := NewSystem(col, &mockEntityStore{}, h)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetOrigin(h, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	ent.SetMins(h, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(h, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetMoveType(h, float32(srvtypes.MoveTypeWalk))

	trace := sys.PushEntity(ent, qtypes.Vec3{X: 10, Y: 0, Z: 0})

	if trace.Fraction != 0.5 {
		t.Errorf("PushEntity trace.Fraction = %v, want 0.5", trace.Fraction)
	}
	got := ent.Origin(h)
	if got.X != 10 || got.Y != 0 || got.Z != 0 {
		t.Errorf("PushEntity origin = %v, want [10 0 0]", got)
	}
}
