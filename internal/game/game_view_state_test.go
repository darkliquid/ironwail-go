package game

import (
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestRuntimeViewStatePrefersAuthoritativeViewEntityOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 128, Y: 64, Z: 32}}
	g.Client.PredictedOrigin = types.Vec3{X: 64, Y: 32, Z: 16}
	g.Client.ViewHeight = 30
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}

	origin, angles := g.runtimeViewState()
	if want := (types.Vec3{X: 128, Y: 64, Z: 62}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateDoesNotFallBackToPredictedOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.PredictedOrigin = types.Vec3{X: 128, Y: 64, Z: 32}
	g.Client.ViewHeight = 18
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := (types.Vec3{X: 0, Y: 0, Z: 128}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != (types.Vec3{}) {
		t.Fatalf("runtimeViewState angles = %v, want zero fallback angles", angles)
	}
}

func TestRuntimeViewStateUsesStaleAuthoritativeEntityInsteadOfPredictedFallback(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1.0, 0.9}
	g.Client.Entities[1] = inet.EntityState{
		ModelIndex: 1,
		MsgTime:    0.9,
		Origin:     types.Vec3{X: 10, Y: 20, Z: 30},
	}
	g.Client.PredictedOrigin = types.Vec3{X: 128, Y: 64, Z: 32}
	g.Client.ViewHeight = 18
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.Time = 1.0
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := (types.Vec3{X: 10, Y: 20, Z: 48}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want stale authoritative origin %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesPredictedXYDuringActiveMovementWhenSafe(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.Entities[1] = inet.EntityState{
		Origin:     types.Vec3{X: 100, Y: 200, Z: 300},
		MsgOrigins: [2]types.Vec3{{X: 100, Y: 200, Z: 300}, {X: 100, Y: 200, Z: 300}},
		MsgTime:    g.Client.MTime[0],
	}
	g.Client.PendingCmd = cl.UserCmd{
		ViewAngles: types.Vec3{X: 0, Y: 0, Z: 0},
		Forward:    100,
	}

	g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if got := g.Client.PredictedOrigin; got.X <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
	if got := g.Client.PredictedOrigin; got.Z >= 300 {
		t.Fatalf("expected collisionless prediction to drift below authoritative Z, got %#v", got)
	}

	origin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 100, Y: 200, Z: 300 + g.Client.ViewHeight}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeInterpolatedVelocityUsesLerpHistory(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{0.1, 0}
	g.Client.Time = 0.05
	g.Client.MVelocity[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.MVelocity[0] = types.Vec3{X: 320, Y: 0, Z: 0}
	g.Client.Velocity = types.Vec3{X: 320, Y: 0, Z: 0}

	if got := g.runtimeInterpolatedVelocity(); got != (types.Vec3{X: 160, Y: 0, Z: 0}) {
		t.Fatalf("g.runtimeInterpolatedVelocity() = %v, want [160 0 0]", got)
	}
}

func TestRuntimeViewStateUsesAuthoritativeOriginWhenPredictionIsSafe(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.PredictedOrigin = types.Vec3{X: 102, Y: 198, Z: 280}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	markCurrentPredictionFresh(g.Client)

	origin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 100, Y: 200, Z: 322}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeEvaluatePredictedFirstPersonXYOriginRejectsStalePrediction(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.PredictedOrigin = types.Vec3{X: 102, Y: 198, Z: 280}

	decision := g.runtimeEvaluatePredictedFirstPersonXYOrigin(types.Vec3{X: 100, Y: 200, Z: 300})
	if decision.OK {
		t.Fatalf("g.runtimeEvaluatePredictedFirstPersonXYOrigin() = %+v, want rejection for stale prediction", decision)
	}
	if decision.RejectReason != runtimeOriginRejectInvalidPrediction {
		t.Fatalf("reject reason = %s, want %s", decision.RejectReason, runtimeOriginRejectInvalidPrediction)
	}
}

func TestRuntimeViewStateUsesRelinkedAuthoritativeOriginWhenPredictionIsSafe(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.Entities[1] = inet.EntityState{
		Origin:     types.Vec3{X: 95, Y: 200, Z: 300},
		MsgOrigins: [2]types.Vec3{{X: 100, Y: 200, Z: 300}, {X: 90, Y: 200, Z: 300}},
	}
	g.Client.LastServerOrigin = types.Vec3{X: 100, Y: 200, Z: 300}
	g.Client.PredictedOrigin = types.Vec3{X: 97, Y: 200, Z: 280}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	markCurrentPredictionFresh(g.Client)

	origin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 95, Y: 200, Z: 322}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want relinked authoritative origin %v", origin, want)
	}
}

func TestRuntimeViewStateUsesAuthoritativeOriginWhenPredictionUnsafe(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.PredictedOrigin = types.Vec3{X: 110, Y: 200, Z: 280}
	g.Client.PredictionError = types.Vec3{X: RuntimeMaxPredictedXYOffset + 1, Y: 0, Z: 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}

	origin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 100, Y: 200, Z: 300 + g.Client.ViewHeight}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeViewStateUsesLastServerOriginWhenViewEntityMissing(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Renderer = originalRenderer
	})

	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.LastServerOrigin = types.Vec3{X: 430, Y: 690, Z: 2}
	g.Client.PredictedOrigin = types.Vec3{X: 100, Y: 200, Z: 300}
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := (types.Vec3{X: 430, Y: 690, Z: 24}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want last server origin %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesTeleportSnappedOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.viewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 512, Y: 256, Z: 128}}
	g.Client.PredictedOrigin = types.Vec3{X: 540, Y: 280, Z: 128}
	g.Client.PredictionError = types.Vec3{X: 28, Y: 24, Z: 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	g.Client.LocalViewTeleport = true
	g.Client.Time = 1.1
	g.Client.OldTime = 1.0
	g.viewCalc.oldZ = 64
	g.viewCalc.oldZInit = true

	origin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 512, Y: 256, Z: 150}); origin != want {
		t.Fatalf("runtimeViewState origin = %v, want hard-snapped origin %v", origin, want)
	}
}

func TestRuntimeViewStateKeepsViewModelAlignedWithAuthoritativeOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
	})

	ensureViewCalcCvars(g)
	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("cl_bob", "0")
	g.Host.CVar.Set("cl_bobcycle", "0")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.PredictedOrigin = types.Vec3{X: 102, Y: 198, Z: 280}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	viewOrigin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 100, Y: 200, Z: 322}); viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative eye origin %v", viewOrigin, want)
	}

	viewModel := g.collectViewModelEntity()
	if viewModel == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want viewmodel")
	}
	if viewModel.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want aligned eye origin %v", viewModel.Origin, viewOrigin)
	}
}

func TestRuntimeViewStateAppliesCanonicalBobInFirstPersonPath(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.viewCalc = originalViewCalc
	})

	ensureViewCalcCvars(g)

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.Time = 0.1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.Velocity = types.Vec3{X: 300, Y: 0, Z: 0}

	if bob := g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity()); bob == 0 {
		t.Fatal("test setup produced zero bob, want non-zero bob input")
	} else {
		origin, _ := g.runtimeViewState()
		want := types.Vec3{X: 100, Y: 200, Z: 322 + bob}
		if origin != want {
			t.Fatalf("runtimeViewState origin = %v, want bobbed eye origin %v", origin, want)
		}
	}
}

func TestRuntimeViewStateSmoothsUpwardStepAndKeepsViewModelAligned(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
	})

	ensureViewCalcCvars(g)
	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("cl_bob", "0")
	g.Host.CVar.Set("cl_bobcycle", "0")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")
	g.Host.SetFrameTime(0.1)

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.Time = 1.1
	g.Client.OldTime = 1.0
	g.Client.OnGround = true
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 110}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type: model.ModAlias,
			AliasHeader: &model.AliasHeader{
				NumFrames: 1,
				Poses:     [][]model.TriVertX{{}},
			},
		},
	}
	g.viewCalc.oldZ = 100
	g.viewCalc.oldZInit = true

	viewOrigin, _ := g.runtimeViewState()
	if want := (types.Vec3{X: 100, Y: 200, Z: 130}); viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want smoothed eye origin %v", viewOrigin, want)
	}
	if got := g.runtimeWeaponBaseOrigin(); got != viewOrigin {
		t.Fatalf("g.runtimeWeaponBaseOrigin() = %v, want same smoothed eye origin %v", got, viewOrigin)
	}

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	// With no bob, the weapon origin equals the eye origin (C V_CalcRefdef:
	// view->origin = ent->origin + viewheight, bob contribution zero).
	if entity.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want aligned smoothed eye origin %v", entity.Origin, viewOrigin)
	}
}

func TestCollectViewModelEntityAppliesCanonicalBobWhenPresent(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
	})

	ensureViewCalcCvars(g)
	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.Time = 0.1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.Velocity = types.Vec3{X: 300, Y: 0, Z: 0}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	if bob := g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity()); bob == 0 {
		t.Fatal("test setup produced zero bob, want non-zero bob input")
	} else {
		viewOrigin, _ := g.runtimeViewState()
		if want := (types.Vec3{X: 100, Y: 200, Z: 322 + bob}); viewOrigin != want {
			t.Fatalf("runtimeViewState origin = %v, want bobbed eye origin %v", viewOrigin, want)
		}
		if got := g.runtimeWeaponBaseOrigin(); got != (types.Vec3{X: 100, Y: 200, Z: 322}) {
			t.Fatalf("g.runtimeWeaponBaseOrigin() = %v, want bob-free weapon base origin [100 200 322]", got)
		}

		entity := g.collectViewModelEntity()
		if entity == nil {
			t.Fatal("g.collectViewModelEntity() = nil, want entity")
		}
		// C V_CalcRefdef applies the view bob exactly once to the weapon
		// origin. The doubled-bob behavior (weapon base origin pre-bobbed and
		// then bobbed again in collect) was a Go regression.
		wantOrigin := types.Vec3{X: 100 + bob*0.4, Y: 200, Z: 322 + bob}
		if entity.Origin != wantOrigin {
			t.Fatalf("viewmodel origin = %v, want bobbed weapon origin %v", entity.Origin, wantOrigin)
		}
	}
}

func TestCollectViewModelEntityIgnoresCameraPunchAngles(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
	})

	ensureViewCalcCvars(g)
	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("cl_bob", "0")
	g.Host.CVar.Set("cl_bobcycle", "0")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")
	g.Host.CVar.Set("v_gunkick", "1")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = types.Vec3{X: 10, Y: 20, Z: 0}
	g.Client.PunchAngle = types.Vec3{X: 5, Y: 7, Z: 0}
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if got := entity.Angles.X; got != -10 {
		t.Fatalf("viewmodel pitch = %v, want -10 without camera punch", got)
	}
	if got := entity.Angles.Y; got != 20 {
		t.Fatalf("viewmodel yaw = %v, want 20 without camera punch", got)
	}
}
