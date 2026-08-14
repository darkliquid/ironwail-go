// What: Entity position and angle interpolation tests.
// Why: Ensures smooth rendering of entities between server updates.
// Where in C: cl_main.c

package client

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// TestRelinkEntities_DemoViewAngleInterpolation verifies that view angles are interpolated during demo playback.
// Why: Demos are recorded at a fixed framerate; interpolation ensures smooth camera movement regardless of playback FPS.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_DemoViewAngleInterpolation(t *testing.T) {
	c := &Client{}
	// Set up time so LerpPoint returns 0.5 (midway between frames).
	// MTime[0] - MTime[1] = 0.1 (frame interval), Time at midpoint.
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95

	// Enable demo playback interpolation.
	c.DemoPlayback = true

	// Set double-buffered view angles: old=[0,0,0], new=[90,180,45]
	c.MViewAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}     // old frame
	c.MViewAngles[0] = types.Vec3{X: 90, Y: 180, Z: 45} // new frame

	c.RelinkEntities()

	// At frac=0.5, expect midpoint: [45, 90, 22.5]
	want := types.Vec3{X: 45, Y: 90, Z: 22.5}
	angles := [3]float32{c.ViewAngles.X, c.ViewAngles.Y, c.ViewAngles.Z}
	wantArr := [3]float32{want.X, want.Y, want.Z}
	for j := 0; j < 3; j++ {
		if math.Abs(float64(angles[j]-wantArr[j])) > 0.01 {
			t.Errorf("ViewAngles[%d] = %v, want %v", j, angles[j], wantArr[j])
		}
	}
}

// TestRelinkEntities_DemoViewAngleWraparound ensures that angle interpolation correctly handles the 360 to 0 degree transition.
// Why: Preventing "spinning" glitches when the camera crosses the angular wrap point.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_DemoViewAngleWraparound(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95
	c.DemoPlayback = true

	// Test wraparound: old=350°, new=10° → delta should be +20° (not -340°)
	c.MViewAngles[1] = types.Vec3{X: 0, Y: 350, Z: 0}
	c.MViewAngles[0] = types.Vec3{X: 0, Y: 10, Z: 0}

	c.RelinkEntities()

	// At frac=0.5: 350 + 0.5*20 = 360 → should be 360 (or wrapped)
	// C doesn't wrap the result, so 360 is fine.
	want := float32(360)
	if math.Abs(float64(c.ViewAngles.Y-want)) > 0.01 {
		t.Errorf("ViewAngles.Y = %v, want %v (wraparound interpolation)", c.ViewAngles.Y, want)
	}
}

// TestRelinkEntities_NoDemoNoViewAngleChange verifies that view angles are NOT interpolated during normal gameplay.
// Why: During live play, view angles are controlled by the player's mouse input, not server updates.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_NoDemoNoViewAngleChange(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95
	c.DemoPlayback = false

	// Set view angles manually.
	c.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 30}
	c.MViewAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	c.MViewAngles[0] = types.Vec3{X: 90, Y: 180, Z: 45}

	c.RelinkEntities()

	// Without demo playback, view angles should not be modified.
	want := types.Vec3{X: 10, Y: 20, Z: 30}
	if c.ViewAngles != want {
		t.Errorf("ViewAngles = %v, want %v (should be unchanged without demo)", c.ViewAngles, want)
	}
}

// TestRelinkEntities_LocalTeleportPreservesResetAndSnapsPrediction verifies that teleporting the local player resets prediction state.
// Why: Prediction must start from a fresh server-provided origin after a teleport to avoid "snapping" back to the pre-teleport position.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_LocalTeleportPreservesResetAndSnapsPrediction(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0
	c.ViewEntity = 1
	c.LastServerOrigin = types.Vec3{X: 16, Y: 0, Z: 0}
	c.PredictedOrigin = types.Vec3{X: 40, Y: 5, Z: 0}
	c.PredictionError = types.Vec3{X: 24, Y: 5, Z: 0}
	c.Velocity = types.Vec3{X: 1, Y: 2, Z: 3}
	c.CommandSequence = 2
	c.CommandCount = 2
	c.CommandBuffer[0] = UserCmd{Forward: 200, Msec: 16}
	c.CommandBuffer[1] = UserCmd{Forward: 200, Msec: 16}
	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 1,
			MsgTime:    1.0,
			MsgOrigins: [2]types.Vec3{{X: 512, Y: 256, Z: 128}, {X: 16, Y: 0, Z: 0}},
			MsgAngles:  [2]types.Vec3{{X: 0, Y: 90, Z: 0}, {X: 0, Y: 0, Z: 0}},
		},
	}

	c.RelinkEntities()

	ent := c.Entities[1]
	if ent.Origin != (types.Vec3{X: 512, Y: 256, Z: 128}) {
		t.Fatalf("Origin = %v, want teleported origin", ent.Origin)
	}
	if ent.LerpFlags&inet.LerpResetMove == 0 {
		t.Fatal("expected LerpResetMove to persist after teleport relink")
	}
	if !c.LocalViewTeleportActive() {
		t.Fatal("expected local teleport signal to stay active for frame consumers")
	}
	if c.PredictedOrigin != ent.Origin {
		t.Fatalf("PredictedOrigin = %v, want snapped origin %v", c.PredictedOrigin, ent.Origin)
	}
	if c.LastServerOrigin != ent.Origin {
		t.Fatalf("LastServerOrigin = %v, want snapped origin %v", c.LastServerOrigin, ent.Origin)
	}
	if c.PredictionError != (types.Vec3{}) {
		t.Fatalf("PredictionError = %v, want cleared", c.PredictionError)
	}
	if c.Velocity != (types.Vec3{}) {
		t.Fatalf("Velocity = %v, want cleared", c.Velocity)
	}
	if c.MVelocity != ([2]types.Vec3{}) {
		t.Fatalf("MVelocity = %v, want cleared", c.MVelocity)
	}
	if c.PredictedVelocity != (types.Vec3{}) {
		t.Fatalf("PredictedVelocity = %v, want cleared", c.PredictedVelocity)
	}
	if c.CommandCount != 0 {
		t.Fatalf("CommandCount = %d, want 0 after teleport reset", c.CommandCount)
	}

	c.MTime = [2]float64{1.1, 1.0}
	ent.MsgTime = 1.1
	ent.MsgOrigins = [2]types.Vec3{{X: 520, Y: 260, Z: 128}, {X: 512, Y: 256, Z: 128}}
	ent.MsgAngles = [2]types.Vec3{{X: 0, Y: 100, Z: 0}, {X: 0, Y: 90, Z: 0}}
	c.Entities[1] = ent
	c.RelinkEntities()

	ent = c.Entities[1]
	if ent.LerpFlags&inet.LerpResetMove != 0 {
		t.Fatal("expected LerpResetMove to clear on the next non-teleport relink")
	}
	if c.LocalViewTeleportActive() {
		t.Fatal("expected local teleport signal to clear on the next relink")
	}
}

// TestRelinkEntities_TrailEvents verifies that rocket and grenade trails are generated correctly.
// Why: Trails provide essential visual tracking for fast-moving projectiles.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_TrailEvents(t *testing.T) {
	c := &Client{}
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0

	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex:  1,
			MsgTime:     1.0,
			MsgOrigins:  [2]types.Vec3{{X: 100, Y: 200, Z: 300}, {X: 100, Y: 200, Z: 300}},
			TrailOrigin: types.Vec3{X: 90, Y: 190, Z: 290},
			ForceLink:   false,
		},
		2: {
			ModelIndex:  2,
			MsgTime:     1.0,
			MsgOrigins:  [2]types.Vec3{{X: 50, Y: 60, Z: 70}, {X: 50, Y: 60, Z: 70}},
			TrailOrigin: types.Vec3{X: 40, Y: 50, Z: 60},
			ForceLink:   false,
		},
		3: {
			ModelIndex:  3,
			MsgTime:     1.0,
			MsgOrigins:  [2]types.Vec3{{X: 10, Y: 20, Z: 30}, {X: 10, Y: 20, Z: 30}},
			TrailOrigin: types.Vec3{X: 5, Y: 15, Z: 25},
			ForceLink:   false,
		},
	}
	c.ModelPrecache = []string{"progs/missile.mdl", "progs/grenade.mdl", "progs/player.mdl"}
	c.ModelFlagsFunc = func(name string) int {
		switch name {
		case "progs/missile.mdl":
			return model.EFRocket
		case "progs/grenade.mdl":
			return model.EFGrenade
		default:
			return 0
		}
	}

	c.RelinkEntities()

	// Should have 2 trail events (rocket + grenade), none for player.
	if len(c.TrailEvents) != 2 {
		t.Fatalf("TrailEvents count = %d, want 2", len(c.TrailEvents))
	}

	// Find rocket and grenade trails (map iteration order is non-deterministic).
	var gotRocket, gotGrenade bool
	for _, te := range c.TrailEvents {
		switch te.Type {
		case 0: // rocket
			gotRocket = true
			if te.Start != (types.Vec3{X: 90, Y: 190, Z: 290}) {
				t.Errorf("rocket trail start = %v, want [90 190 290]", te.Start)
			}
		case 1: // grenade
			gotGrenade = true
			if te.Start != (types.Vec3{X: 40, Y: 50, Z: 60}) {
				t.Errorf("grenade trail start = %v, want [40 50 60]", te.Start)
			}
		default:
			t.Errorf("unexpected trail type %d", te.Type)
		}
	}
	if !gotRocket {
		t.Error("missing rocket trail event")
	}
	if !gotGrenade {
		t.Error("missing grenade trail event")
	}

	// After relink, TrailOrigin should be updated to current origin.
	ent1 := c.Entities[1]
	if ent1.TrailOrigin != ent1.Origin {
		t.Errorf("entity 1 TrailOrigin = %v, want %v", ent1.TrailOrigin, ent1.Origin)
	}
}

// TestRelinkEntities_TrailEventsUseModelIndexMinusOne ensures correct model mapping for trail effects.
// Why: Quake uses 1-based indexing for models in the precache; internal logic often uses 0-based.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_TrailEventsUseModelIndexMinusOne(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0

	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex:  1,
			MsgTime:     1.0,
			MsgOrigins:  [2]types.Vec3{{X: 100, Y: 200, Z: 300}, {X: 100, Y: 200, Z: 300}},
			TrailOrigin: types.Vec3{X: 90, Y: 190, Z: 290},
		},
		2: {
			ModelIndex:  2,
			MsgTime:     1.0,
			MsgOrigins:  [2]types.Vec3{{X: 50, Y: 60, Z: 70}, {X: 50, Y: 60, Z: 70}},
			TrailOrigin: types.Vec3{X: 40, Y: 50, Z: 60},
		},
	}
	c.ModelPrecache = []string{"progs/missile.mdl", "progs/grenade.mdl"}
	c.ModelFlagsFunc = func(name string) int {
		switch name {
		case "progs/missile.mdl":
			return model.EFRocket
		case "progs/grenade.mdl":
			return model.EFGrenade
		default:
			return 0
		}
	}

	c.RelinkEntities()

	if len(c.TrailEvents) != 2 {
		t.Fatalf("TrailEvents count = %d, want 2", len(c.TrailEvents))
	}
}

// TestRelinkEntities_RocketTrailIsRateLimited verifies that trail particles are not spawned too frequently.
// Why: Prevents particle "blooming" and performance degradation during slow-motion playback or high framerates.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_RocketTrailIsRateLimited(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.OldTime = 0.99
	c.Time = 1.0
	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 1,
			MsgTime:    1.0,
			MsgOrigins: [2]types.Vec3{{X: 10, Y: 0, Z: 0}, {X: 0, Y: 0, Z: 0}},
			MsgAngles:  [2]types.Vec3{},
			ForceLink:  true,
		},
	}
	c.ModelPrecache = []string{"progs/missile.mdl"}
	c.ModelFlagsFunc = func(name string) int { return model.EFRocket }

	c.RelinkEntities()
	if got := len(c.TrailEvents); got != 0 {
		t.Fatalf("first relink trail count = %d, want 0 after reset", got)
	}

	c.TrailEvents = nil
	c.OldTime = 1.0
	c.Time = 1.002
	ent := c.Entities[1]
	ent.MsgTime = 1.002
	ent.MsgOrigins = [2]types.Vec3{{X: 20, Y: 0, Z: 0}, {X: 10, Y: 0, Z: 0}}
	c.Entities[1] = ent
	c.MTime = [2]float64{1.002, 1.0}
	c.RelinkEntities()
	if got := len(c.TrailEvents); got != 0 {
		t.Fatalf("second relink trail count = %d, want 0 while traildelay active", got)
	}

	c.TrailEvents = nil
	c.OldTime = 1.002
	c.Time = 1.03
	ent = c.Entities[1]
	ent.MsgTime = 1.03
	ent.MsgOrigins = [2]types.Vec3{{X: 30, Y: 0, Z: 0}, {X: 20, Y: 0, Z: 0}}
	c.Entities[1] = ent
	c.MTime = [2]float64{1.03, 1.002}
	c.RelinkEntities()
	if got := len(c.TrailEvents); got != 1 {
		t.Fatalf("third relink trail count = %d, want 1 after traildelay expiry", got)
	}
}

// TestRelinkEntities_StaleEntityClearsModelAndResetsLerp verifies that entities that stop receiving updates are removed from the scene.
// Why: Prevents "ghost" entities from lingering after they should have been destroyed or moved out of range.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_StaleEntityClearsModelAndResetsLerp(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0
	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 2,
			MsgTime:    0.9,
			LerpFlags:  0,
			Origin:     types.Vec3{X: 10, Y: 20, Z: 30},
		},
	}

	c.RelinkEntities()

	ent := c.Entities[1]
	if ent.ModelIndex != 0 {
		t.Fatalf("ModelIndex = %v, want stale entity to clear its render model", ent.ModelIndex)
	}
	if ent.LerpFlags&inet.LerpResetMove == 0 {
		t.Fatal("expected stale entity to set LerpResetMove")
	}
	if ent.LerpFlags&inet.LerpResetAnim == 0 {
		t.Fatal("expected stale entity to set LerpResetAnim")
	}
}

// TestRelinkEntities_StepMoveSnapsRenderStateWithoutTeleportReset verifies that small movement steps (like stairs) snap to avoid visual jitter.
// Why: Smoothly interpolating up stairs would cause the camera to "sink" into the steps.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_StepMoveSnapsRenderStateWithoutTeleportReset(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 0.95
	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 1,
			MsgTime:    1.0,
			MsgOrigins: [2]types.Vec3{
				{X: 24, Y: 20, Z: 30},
				{X: 10, Y: 20, Z: 30},
			},
			MsgAngles: [2]types.Vec3{
				{X: 0, Y: 45, Z: 0},
				{X: 0, Y: 30, Z: 0},
			},
			Origin:    types.Vec3{X: 10, Y: 20, Z: 30},
			Angles:    types.Vec3{X: 0, Y: 30, Z: 0},
			LerpFlags: inet.LerpMoveStep,
		},
	}

	c.RelinkEntities()

	ent := c.Entities[1]
	if ent.Origin != (types.Vec3{X: 24, Y: 20, Z: 30}) {
		t.Fatalf("Origin = %v, want step-move entity snapped to latest network origin", ent.Origin)
	}
	if ent.Angles != (types.Vec3{X: 0, Y: 45, Z: 0}) {
		t.Fatalf("Angles = %v, want step-move entity snapped to latest network angles", ent.Angles)
	}
	if ent.LerpFlags&inet.LerpMoveStep == 0 {
		t.Fatal("expected LerpMoveStep to remain set for renderer-side interpolation")
	}
	if ent.LerpFlags&inet.LerpResetMove != 0 {
		t.Fatal("expected ordinary step-move update not to set teleport reset")
	}
}

// TestRelinkEntities_ExplicitRetireKeepsZeroModel verifies that entities explicitly retired by the server are deactivated.
// Why: Allows the server to precisely control the lifecycle of entities.
// Where in C: cl_main.c, CL_RelinkEntities.
func TestRelinkEntities_ExplicitRetireKeepsZeroModel(t *testing.T) {
	c := NewClient()
	c.MTime = [2]float64{1.0, 0.9}
	c.Time = 1.0
	c.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 0,
			MsgTime:    1.0,
			LerpFlags:  0,
			Origin:     types.Vec3{X: 10, Y: 20, Z: 30},
			ForceLink:  true,
		},
	}

	c.RelinkEntities()

	ent := c.Entities[1]
	if ent.ModelIndex != 0 {
		t.Fatalf("ModelIndex = %v, want explicit retire to stay zero", ent.ModelIndex)
	}
	if ent.ForceLink {
		t.Fatal("expected explicit retire to clear ForceLink")
	}
	if ent.LerpFlags&inet.LerpResetMove == 0 {
		t.Fatal("expected explicit retire to set LerpResetMove")
	}
	if ent.LerpFlags&inet.LerpResetAnim == 0 {
		t.Fatal("expected explicit retire to set LerpResetAnim")
	}
}
