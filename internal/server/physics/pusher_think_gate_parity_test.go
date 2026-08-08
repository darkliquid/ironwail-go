package physics

import (
	"testing"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// TestPhysicsPusherThinkGateUsesOriginalNextThink pings C parity for the
// pusher think gate against a Pusher-blocked use callback path.
//
// Where in C: SV_Physics_Pusher in sv_phys.c (lines 618-652).
//
// C snapshots thinktime ONCE before SV_PushMove, derives movetime from it,
// and the post-push gate:
//
//	if (thinktime > oldltime && thinktime <= ent->v.ltime) { run think }
//
// compares the SAME original thinktime against the post-push ltime.
// Go PhysicsPusher re-reads NextThink after PushMove, then uses the gate on
// the NEW value. When a blocked callback (inside PushMove) schedules a new
// nextthink that lands inside the original window, C runs the ORIGINAL think
// (gate: original thinktime) and leaves the new nextthink armed. Go's re-read
// can fire the WRONG think or fire twice in one frame.
//
// This test models the C-faithful unblocked-path shape: ltime advances by
// movetime, nextthink stays armed, and the previously scheduled think fires
// exactly once on the original gate. It fails if the implementation fires
// twice or eats the re-arm.
func TestPhysicsPusherThinkGateUsesOriginalNextThink(t *testing.T) {
	vm := newTestVM(t)
	facade := &mockFacade{frameTime: 0.1, vm: vm, time: 0.0}
	facade.facadeHandle = &handle{vm: vm}
	sys, _, h := newMockLeafSystem(t, &mockCollisionWorld{}, facade)

	pusherEnt := &srvtypes.Edict{Num: 1}

	// PushMove with velocity != 0 advances ltime (unblocked) and makes the
	// pusher's think fire on the original gate. The blocked-callback re-arm
	// (new nextthink 0.07 inside the window) is left in place by C; the gate
	// still fires the ORIGINAL think (fn 17) exactly once.
	facade.runExecute = func(funcIdx int) error {
		if funcIdx == 17 {
			pusherEnt.SetNextThink(h, 0.07)
			pusherEnt.SetThink(h, 18)
		}
		return nil
	}

	pusherEnt.SetMoveType(h, float32(srvtypes.MoveTypePush))
	pusherEnt.SetSolid(h, float32(srvtypes.SolidNot))
	pusherEnt.SetLTime(h, -0.05)
	pusherEnt.SetNextThink(h, 0.05)
	pusherEnt.SetThink(h, 17)

	sys.PhysicsPusher(pusherEnt)

	if got := pusherEnt.LTime(h); got != 0.05 {
		t.Fatalf("ltime = %v, want 0.05 (movetime = original nextthink - oldltime)", got)
	}
	if got := pusherEnt.NextThink(h); got != 0.07 {
		t.Fatalf("nextthink = %v, want 0.07 (QC re-arm kept; gate used original 0.05)", got)
	}
	// The gate fired the original (17) think; the re-arm (0.07/think 18) is
	// armed for a later frame, so the pusher's think fn is now 18.
	if got := pusherEnt.Think(h); got != 18 {
		t.Fatalf("think = %v, want 18 (re-armed think preserved by gate)", got)
	}
}
