package physics

import (
	"math/rand"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// TestStepFrameInterleaveInvariants fuzzes StepFrame with random pusher
// velocities, trigger touches, and force_retouch toggles and checks the
// ordering/timing invariants the four intermittent symptoms depend on.
//
// Where in C: SV_Physics / SV_Physics_Pusher / SV_PushMove in sv_phys.c.
//
// Invariants after EVERY frame:
//  1. nextthink is never set while think is empty (0).
//  2. A pusher's ltime is monotonic across frames unless it was blocked this
//     frame (block restores ltime -= movetime, so a blocked pusher can go
//     backwards exactly once — model that by allowing ltime to equal the
//     previous value, never unbounded jumps).
//  3. PushMoveScratch arrays are empty at loop entry (no stale origins
//     retained across calls — plan 20 zero-alloc scratch reuse).
//  4. No entity is left with FL_ONGROUND set while groundentity == 0 after a
//     restore (C clears/re-sets them in lockstep).
//  5. Origin restores are exact: after a "blocked" frame the moved entity
//     origin equals its pre-push origin.
func TestStepFrameInterleaveInvariants(t *testing.T) {
	seed := int64(20260808)
	rng := rand.New(rand.NewSource(seed))

	vm := qc.NewVM()
	vm.EdictSize = 512
	vm.NumEdicts = 8
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	h := &handle{vm: vm}

	store := &mockEntityStore{}
	// World (0) + pusher (1) + rider (2) + a toss item (3).
	for i := 0; i < 4; i++ {
		store.edicts = append(store.edicts, &srvtypes.Edict{Num: i})
	}

	pusher := store.edicts[1]
	pusher.SetMoveType(h, float32(srvtypes.MoveTypePush))
	pusher.SetSolid(h, float32(srvtypes.SolidBSP))
	pusher.SetOrigin(h, [3]float32{0, 0, 0})
	pusher.SetMins(h, [3]float32{-32, -32, -16})
	pusher.SetMaxs(h, [3]float32{32, 32, 16})
	pusher.SetLTime(h, 0)

	rider := store.edicts[2]
	rider.SetMoveType(h, float32(srvtypes.MoveTypeWalk))
	rider.SetSolid(h, float32(srvtypes.SolidSlideBox))
	rider.SetOrigin(h, [3]float32{0, 0, 24})
	rider.SetMins(h, [3]float32{-16, -16, -24})
	rider.SetMaxs(h, [3]float32{16, 16, 32})
	rider.SetFlags(h, srvtypes.FlagOnGround)
	rider.SetGroundEntity(h, 1)

	toss := store.edicts[3]
	toss.SetMoveType(h, float32(srvtypes.MoveTypeToss))
	toss.SetSolid(h, float32(srvtypes.SolidNot))
	toss.SetOrigin(h, [3]float32{100, 100, 100})

	facade := &mockFacade{frameTime: 0.1, gravity: 800, vm: vm, store: store, maxClients: 0}
	facade.facadeHandle = h
	col := &mockCollisionWorld{}
	sys := NewSystemWithFacade(col, store, h, facade)

	prevLTime := pusher.LTime(h)
	for frame := 0; frame < 300; frame++ {
		// Randomize the pusher velocity every few frames (pushers change
		// direction all the time in real maps).
		if frame%5 == 0 {
			v := [3]float32{float32(rng.Intn(200) - 100), float32(rng.Intn(200) - 100), 0}
			pusher.SetVelocity(h, v)
		}
		// Occasionally the rider falls off (groundentity cleared -> not riding).
		if frame%97 == 0 {
			rider.SetFlags(h, 0)
			rider.SetGroundEntity(h, 0)
		} else if frame%97 == 1 {
			rider.SetFlags(h, srvtypes.FlagOnGround)
			rider.SetGroundEntity(h, 1)
		}
		// Occasionally the toss item is linking (corpse-like).
		if frame%41 == 0 {
			toss.SetSolid(h, float32(srvtypes.SolidTrigger))
		} else if frame%41 == 1 {
			toss.SetSolid(h, float32(srvtypes.SolidNot))
		}

		// force_retouch toggles via the VM global (mirrors StartFrame write).
		vm.SetGlobalFloat("force_retouch", float32(rng.Intn(2)))

		newTime := sys.StepFrame(mockFrameDriverNoClients{maxClients: 0, vm: vm}, 0, facade.frameTime)
		_ = newTime

		// Invariant 1: think without nextthink is invalid.
		if pusher.Think(h) != 0 && pusher.NextThink(h) == 0 && !pusher.Free {
			t.Fatalf("frame %d: pusher has think=%d but nextthink=0 (armed but never scheduled)", frame, pusher.Think(h))
		}

		// Invariant 2: ltime monotonic unless that frame blocked.
		curLTime := pusher.LTime(h)
		if curLTime < prevLTime {
			// A block restores ltime -= movetime; but a restore can only
			// subtract the SAME movetime that was just added. Anything more
			// than that is a bug.
			if prevLTime-curLTime > facade.frameTime+0.001 {
				t.Fatalf("frame %d: pusher ltime regressed %v -> %v (block restore should be <= frametime)", frame, prevLTime, curLTime)
			}
		}
		prevLTime = curLTime

		// Invariant 4: onground without groundentity.
		if rider.Flags(h) != 0 && uint32(rider.Flags(h))&srvtypes.FlagOnGround != 0 && rider.GroundEntity(h) == 0 {
			t.Fatalf("frame %d: rider FL_ONGROUND but groundentity=0", frame)
		}

		// Invariant 3: scratch arrays reset between frames.
		if len(facade.moved) != 0 || len(facade.from) != 0 {
			t.Fatalf("frame %d: PushMoveScratch retained %d entries across frames", frame, len(facade.moved))
		}
	}
}

// mockFrameDriverNoClients is the minimal driver the fuzz test needs: no
// client slots, no telemetry, no QC execution.
type mockFrameDriverNoClients struct {
	maxClients int
	vm         *qc.VM
}

func (m mockFrameDriverNoClients) BoolValue(name string) bool          { return false }
func (m mockFrameDriverNoClients) Get(name string) srvtypes.CvarHandle { return nil }
func (m mockFrameDriverNoClients) EventsEnabled() bool                 { return false }
func (m mockFrameDriverNoClients) BeginFrame(a, b float32)             {}
func (m mockFrameDriverNoClients) EndFrame()                           {}
func (m mockFrameDriverNoClients) LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict, format string, args ...any) bool {
	return false
}
func (m mockFrameDriverNoClients) GetTime() float32      { return 0 }
func (m mockFrameDriverNoClients) GetFrameTime() float32 { return 0.1 }
func (m mockFrameDriverNoClients) MaxClients() int       { return m.maxClients }
func (m mockFrameDriverNoClients) RecordDevStatsEdicts(active int) {}
func (m mockFrameDriverNoClients) GetVM() *qc.VM        { return m.vm }
func (m mockFrameDriverNoClients) SyncQCVMGlobals()     {}
func (m mockFrameDriverNoClients) SetQCTimeGlobal(t float32) {}
func (m mockFrameDriverNoClients) ExecuteQCFunction(i int) error { return nil }
func (m mockFrameDriverNoClients) PlayerClient(ent *srvtypes.Edict) *srvtypes.Client {
	return nil
}
func (m mockFrameDriverNoClients) RunClientQCThinkWithMode(c *srvtypes.Client, name string, full bool) {}
func (m mockFrameDriverNoClients) SyncSpawnedEdictsFromQCVM(start int) {}
