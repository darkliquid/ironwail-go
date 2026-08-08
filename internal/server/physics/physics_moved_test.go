// physics_moved_test.go holds tests migrated from the root package's
// physics_test.go. They exercise the physics System directly through mocks
// instead of bouncing through the *Server delegators, so each leaf algorithm
// is verified in isolation. Mirror the original test names/documented parity
// behavior (Where in C annotations preserved) to keep the migration audit
// simple. The BSP/QCVM-integration tests stay in package server.
package physics

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// newMockLeafSystem builds a System wired to the mock collision world, edict
// store, VM-backed handle, and the mock facade. The VM edict table is sized
// so field accessors behave like production (EdictSize 512).
func newMockLeafSystem(t *testing.T, col srvtypes.CollisionWorld, facade *mockFacade) (*System, *mockEntityStore, *handle) {
	t.Helper()
	store := &mockEntityStore{}
	if facade.store == nil {
		facade.store = store
	}
	sh := facade.facadeHandle
	if sh == nil {
		sh = &handle{vm: facade.vm}
	}
	return NewSystemWithFacade(col, store, sh, facade), store, sh.(*handle)
}

// TestPhysicsNoClipMovesOriginAndAngles verifies noclip integrates velocity
// and angular velocity into origin/angles each frame.
// Where in C: SV_Physics_Noclip in sv_phys.c
func TestPhysicsNoClipMovesOriginAndAngles(t *testing.T) {
	vm := newTestVM(t)
	facade := &mockFacade{frameTime: 0.1, vm: vm}
	sys, _, h := newMockLeafSystem(t, &mockCollisionWorld{}, facade)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetVelocity(h, [3]float32{10, -5, 2})
	ent.SetAVelocity(h, [3]float32{0, 90, 0})

	sys.PhysicsNoClip(ent)

	if got := ent.Origin(h); got != [3]float32{1, -0.5, 0.2} {
		t.Fatalf("origin = %v", got)
	}
	if got := ent.Angles(h); got != [3]float32{0, 9, 0} {
		t.Fatalf("angles = %v", got)
	}
}

// TestPhysicsPusherAdvancesLocalTimeWhenIdle verifies pushers advance their
// local time even when idle (no velocity), driving platform animations.
// Where in C: SV_Physics_Pusher in sv_phys.c
func TestPhysicsPusherAdvancesLocalTimeWhenIdle(t *testing.T) {
	vm := newTestVM(t)
	facade := &mockFacade{frameTime: 0.1, vm: vm}
	sys, _, h := newMockLeafSystem(t, &mockCollisionWorld{}, facade)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetMoveType(h, float32(srvtypes.MoveTypePush))
	ent.SetLTime(h, 3)
	ent.SetNextThink(h, 10)
	sys.PhysicsPusher(ent)

	if diff := ent.LTime(h) - 3.1; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("ltime = %v, want 3.1", ent.LTime(h))
	}
}

// TestPhysicsTossOnGroundDoesNotMove verifies toss items stay put once they
// land, matching C's early return when FlagOnGround is set.
// Where in C: SV_Physics_Toss in sv_phys.c
func TestPhysicsTossOnGroundDoesNotMove(t *testing.T) {
	vm := newTestVM(t)
	facade := &mockFacade{frameTime: 0.1, vm: vm}
	sys, _, h := newMockLeafSystem(t, &mockCollisionWorld{}, facade)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetFlags(h, float32(srvtypes.FlagOnGround))
	ent.SetOrigin(h, [3]float32{1, 2, 3})
	ent.SetVelocity(h, [3]float32{50, 60, 70})

	sys.PhysicsToss(ent)

	if ent.Origin(h) != [3]float32{1, 2, 3} {
		t.Fatalf("origin changed on ground toss: %v", ent.Origin(h))
	}
}

// TestPhysicsStepOnGroundSkipsFreefall verifies a grounded step entity does
// not receive gravity or vertical movement.
// Where in C: SV_Physics_Step in sv_phys.c
func TestPhysicsStepOnGroundSkipsFreefall(t *testing.T) {
	vm := newTestVM(t)
	facade := &mockFacade{frameTime: 0.1, vm: vm}
	sys, _, h := newMockLeafSystem(t, &mockCollisionWorld{}, facade)

	ent := &srvtypes.Edict{Num: 1}
	ent.SetFlags(h, float32(srvtypes.FlagOnGround))
	ent.SetVelocity(h, [3]float32{0, 0, 42})

	sys.PhysicsStep(ent)

	if ent.Velocity(h)[2] != 42 {
		t.Fatalf("z velocity changed: %v", ent.Velocity(h)[2])
	}
}

// newTestVM constructs a VM-backed ServerHandle with an edict table large
// enough for the field accessors used by the moved tests.
func newTestVM(t *testing.T) *qc.VM {
	t.Helper()
	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	return vm
}

// TestWalkMoveNeedsUnstickUsesDistEpsilonThreshold pins the strict-'<'
// DIST_EPSILON threshold parity with C: a move that is exactly on the
// threshold does not request unstick, only strictly inside does.
// Where in C: SV_WalkMove in sv_phys.c
func TestWalkMoveNeedsUnstickUsesDistEpsilonThreshold(t *testing.T) {
	oldOrg := [3]float32{100, 200, 0}
	distEps := srvtypes.DistEpsilon

	// Strictly inside threshold on both axes should request unstick.
	inside := [3]float32{100 + distEps - 0.0001, 200 - distEps + 0.0001, 0}
	if !srvtypes.WalkMoveNeedsUnstick(oldOrg, inside) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = false, want true", oldOrg, inside)
	}

	// Exact threshold should not request unstick (strict '<' parity with C code).
	xEdge := [3]float32{100 + distEps, 200, 0}
	if srvtypes.WalkMoveNeedsUnstick(oldOrg, xEdge) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, xEdge)
	}

	yEdge := [3]float32{100, 200 - distEps, 0}
	if srvtypes.WalkMoveNeedsUnstick(oldOrg, yEdge) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, yEdge)
	}

	// Outside threshold on either axis should not request unstick.
	outside := [3]float32{100 + distEps + 0.0001, 200, 0}
	if srvtypes.WalkMoveNeedsUnstick(oldOrg, outside) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, outside)
	}
}
