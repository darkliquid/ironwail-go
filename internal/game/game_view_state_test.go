package game

import (
	"math"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
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
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{128, 64, 32}}
	g.Client.PredictedOrigin = [3]float32{64, 32, 16}
	g.Client.ViewHeight = 30
	g.Client.ViewAngles = [3]float32{10, 20, 0}

	origin, angles := g.runtimeViewState()
	if want := [3]float32{128, 64, 62}; origin != want {
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
	g.Client.PredictedOrigin = [3]float32{128, 64, 32}
	g.Client.ViewHeight = 18
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := [3]float32{0, 0, 128}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != [3]float32{} {
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
		Origin:     [3]float32{10, 20, 30},
	}
	g.Client.PredictedOrigin = [3]float32{128, 64, 32}
	g.Client.ViewHeight = 18
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Time = 1.0
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := [3]float32{10, 20, 48}; origin != want {
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

	g.Host = nil
	g.Server = nil
	g.Renderer = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{
		Origin:     [3]float32{100, 200, 300},
		MsgOrigins: [2][3]float32{{100, 200, 300}, {100, 200, 300}},
		MsgTime:    g.Client.MTime[0],
	}
	g.Client.PendingCmd = cl.UserCmd{
		ViewAngles: [3]float32{0, 0, 0},
		Forward:    100,
	}

	g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if got := g.Client.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
	if got := g.Client.PredictedOrigin; got[2] >= 300 {
		t.Fatalf("expected collisionless prediction to drift below authoritative Z, got %#v", got)
	}

	origin, _ := g.runtimeViewState()
	if want := [3]float32{100, 200, 300 + g.Client.ViewHeight}; origin != want {
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
	g.Client.MVelocity[1] = [3]float32{0, 0, 0}
	g.Client.MVelocity[0] = [3]float32{320, 0, 0}
	g.Client.Velocity = [3]float32{320, 0, 0}

	if got := g.runtimeInterpolatedVelocity(); got != [3]float32{160, 0, 0} {
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
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	markCurrentPredictionFresh(g.Client)

	origin, _ := g.runtimeViewState()
	if want := [3]float32{100, 200, 322}; origin != want {
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
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}

	decision := g.runtimeEvaluatePredictedFirstPersonXYOrigin([3]float32{100, 200, 300})
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
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{
		Origin:     [3]float32{95, 200, 300},
		MsgOrigins: [2][3]float32{{100, 200, 300}, {90, 200, 300}},
	}
	g.Client.LastServerOrigin = [3]float32{100, 200, 300}
	g.Client.PredictedOrigin = [3]float32{97, 200, 280}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	markCurrentPredictionFresh(g.Client)

	origin, _ := g.runtimeViewState()
	if want := [3]float32{95, 200, 322}; origin != want {
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
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{110, 200, 280}
	g.Client.PredictionError = [3]float32{RuntimeMaxPredictedXYOffset + 1, 0, 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}

	origin, _ := g.runtimeViewState()
	if want := [3]float32{100, 200, 300 + g.Client.ViewHeight}; origin != want {
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
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.LastServerOrigin = [3]float32{430, 690, 2}
	g.Client.PredictedOrigin = [3]float32{100, 200, 300}
	markCurrentPredictionFresh(g.Client)

	origin, angles := g.runtimeViewState()
	if want := [3]float32{430, 690, 24}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want last server origin %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesTeleportSnappedOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{512, 256, 128}}
	g.Client.PredictedOrigin = [3]float32{540, 280, 128}
	g.Client.PredictionError = [3]float32{28, 24, 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}
	g.Client.LocalViewTeleport = true
	g.Client.Time = 1.1
	g.Client.OldTime = 1.0
	globalViewCalc.oldZ = 64
	globalViewCalc.oldZInit = true

	origin, _ := g.runtimeViewState()
	if want := [3]float32{512, 256, 150}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want hard-snapped origin %v", origin, want)
	}
}

func TestRuntimeViewStateKeepsViewModelAlignedWithAuthoritativeOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()
	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{0, 0, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	viewOrigin, _ := g.runtimeViewState()
	if want := [3]float32{100, 200, 322}; viewOrigin != want {
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
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.Time = 0.1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.Velocity = [3]float32{300, 0, 0}

	if bob := g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity()); bob == 0 {
		t.Fatal("test setup produced zero bob, want non-zero bob input")
	} else {
		origin, _ := g.runtimeViewState()
		want := [3]float32{100, 200, 322 + bob}
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
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()
	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{0, 0, 0}
	g.Client.Time = 1.1
	g.Client.OldTime = 1.0
	g.Client.OnGround = true
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 110}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil)
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
	globalViewCalc.oldZ = 100
	globalViewCalc.oldZInit = true

	viewOrigin, _ := g.runtimeViewState()
	if want := [3]float32{100, 200, 130}; viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want smoothed eye origin %v", viewOrigin, want)
	}
	if got := g.runtimeWeaponBaseOrigin(); got != viewOrigin {
		t.Fatalf("g.runtimeWeaponBaseOrigin() = %v, want same smoothed eye origin %v", got, viewOrigin)
	}

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
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
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()
	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{0, 0, 0}
	g.Client.Time = 0.1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.Velocity = [3]float32{300, 0, 0}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil)
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
		if want := [3]float32{100, 200, 322 + bob}; viewOrigin != want {
			t.Fatalf("runtimeViewState origin = %v, want bobbed eye origin %v", viewOrigin, want)
		}
		if got := g.runtimeWeaponBaseOrigin(); got != [3]float32{100, 200, 322} {
			t.Fatalf("g.runtimeWeaponBaseOrigin() = %v, want bob-free weapon base origin [100 200 322]", got)
		}

		entity := g.collectViewModelEntity()
		if entity == nil {
			t.Fatal("g.collectViewModelEntity() = nil, want entity")
		}
		wantOrigin := [3]float32{100 + bob*0.4, 200, 322 + bob}
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
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()
	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")
	cvar.Set("v_gunkick", "1")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{10, 20, 0}
	g.Client.PunchAngle = [3]float32{5, 7, 0}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Menu = menu.NewManager(nil, nil)
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
	if got := entity.Angles[0]; got != -10 {
		t.Fatalf("viewmodel pitch = %v, want -10 without camera punch", got)
	}
	if got := entity.Angles[1]; got != 20 {
		t.Fatalf("viewmodel yaw = %v, want 20 without camera punch", got)
	}
}

func TestRuntimeCameraStateResetsStairSmoothingOnTeleport(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalHost := g.Host
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host = originalHost
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.OnGround = true
	g.Client.LocalViewTeleport = true
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{0, 0, 300}}
	globalViewCalc.oldZ = 100
	globalViewCalc.oldZInit = true

	camera := g.runtimeCameraState([3]float32{0, 0, 322}, [3]float32{0, 0, 0})
	if math.Abs(float64(camera.Origin.Z-(322+1.0/32.0))) > 0.001 {
		t.Fatalf("camera origin z = %v, want snapped z %v", camera.Origin.Z, 322+1.0/32.0)
	}
}

func TestCollectViewModelEntityResetsWeaponOffsetOnTeleport(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{0, 0, 0}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Client.LocalViewTeleport = true
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	globalViewCalc.oldZ = 100
	globalViewCalc.oldZInit = true

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != [3]float32{100, 200, 322} {
		t.Fatalf("viewmodel origin = %v, want hard-snapped eye origin", entity.Origin)
	}
}

func TestRuntimeCameraStateCarriesClientTime(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Time = 12.5

	camera := g.runtimeCameraState([3]float32{1, 2, 3}, [3]float32{4, 5, 6})
	if camera.Time != 12.5 {
		t.Fatalf("runtimeCameraState time = %v, want 12.5", camera.Time)
	}
}

func TestRuntimeCameraStateAppliesPunchAnglesOutsideIntermission(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := g.runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 11 || camera.Angles.Y != 18 || camera.Angles.Z != 33 {
		t.Fatalf("runtimeCameraState angles = %v, want {11 18 33}", camera.Angles)
	}
}

func TestRuntimeCameraStateSkipsPunchAnglesDuringIntermission(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := g.runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 10 || camera.Angles.Y != 20 || camera.Angles.Z != 30 {
		t.Fatalf("runtimeCameraState angles = %v, want {10 20 30}", camera.Angles)
	}
}

func TestRuntimeViewStateUsesLiveViewAnglesWithoutInterpolation(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{32, 64, 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.ViewAngles = [3]float32{45, 135, 225}
	g.Client.MViewAngles[1] = [3]float32{0, 0, 0}
	g.Client.MViewAngles[0] = [3]float32{10, 20, 30}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	markCurrentPredictionFresh(g.Client)

	_, angles := g.runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want live angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesForcedAnglesWithoutInterpolation(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{32, 64, 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.ViewAngles = [3]float32{45, 135, 225}
	g.Client.MViewAngles[1] = [3]float32{0, 0, 0}
	g.Client.MViewAngles[0] = [3]float32{10, 20, 30}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	g.Client.FixAngle = true
	markCurrentPredictionFresh(g.Client)

	_, angles := g.runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want forced angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesDemoViewAnglesWithoutDoubleInterpolation(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{32, 64, 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.ViewAngles = [3]float32{5, 10, 15}
	g.Client.MViewAngles[1] = [3]float32{0, 0, 0}
	g.Client.MViewAngles[0] = [3]float32{10, 20, 30}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	g.Client.DemoPlayback = true
	markCurrentPredictionFresh(g.Client)

	_, angles := g.runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState demo angles = %v, want current angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeCameraStateInterpolatesPunchAngles(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "2")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngles[1] = [3]float32{0, 0, 0}
	g.Client.PunchAngles[0] = [3]float32{10, 0, 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := g.runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X < 5.9 || camera.Angles.X > 6.1 {
		t.Fatalf("runtimeCameraState punch interpolation = %v, want ~6", camera.Angles.X)
	}
}

func TestRuntimeCameraStateGunKickModeRaw(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "1")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{2, -4, 6}
	g.Client.PunchAngles[1] = [3]float32{0, 0, 0}
	g.Client.PunchAngles[0] = [3]float32{10, 0, 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := g.runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 3 || camera.Angles.Y != -2 || camera.Angles.Z != 9 {
		t.Fatalf("runtimeCameraState raw punch = %v, want {3 -2 9}", camera.Angles)
	}
}

func TestRuntimeCameraStateGunKickModeOff(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalKick := cvar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("v_gunkick", originalKick)
	})

	cvar.Set("v_gunkick", "0")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{2, -4, 6}

	camera := g.runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 1 || camera.Angles.Y != 2 || camera.Angles.Z != 3 {
		t.Fatalf("runtimeCameraState with gunkick off = %v, want {1 2 3}", camera.Angles)
	}
}

func TestRuntimeCameraStateDeadPlayerRoll(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 0 // Dead player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{10, 10, 10}

	camera := g.runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	// Dead players should have roll = 80 and ignore other view effects.
	if camera.Angles.Z != 80 {
		t.Fatalf("runtimeCameraState dead player roll = %v, want 80", camera.Angles.Z)
	}
}

func TestRuntimeCameraStateAppliesChaseCameraWhenActive(t *testing.T) {
	g := New()
	originalClient := g.Client
	if cvar.Get("chase_active") == nil {
		cvar.Register("chase_active", "0", 0, "")
	}
	if cvar.Get("chase_back") == nil {
		cvar.Register("chase_back", "100", 0, "")
	}
	if cvar.Get("chase_up") == nil {
		cvar.Register("chase_up", "16", 0, "")
	}
	if cvar.Get("chase_right") == nil {
		cvar.Register("chase_right", "0", 0, "")
	}
	originalActive := cvar.StringValue("chase_active")
	originalBack := cvar.StringValue("chase_back")
	originalUp := cvar.StringValue("chase_up")
	originalRight := cvar.StringValue("chase_right")
	t.Cleanup(func() {
		g.Client = originalClient
		cvar.Set("chase_active", originalActive)
		cvar.Set("chase_back", originalBack)
		cvar.Set("chase_up", originalUp)
		cvar.Set("chase_right", originalRight)
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100
	cvar.Set("chase_active", "1")
	cvar.Set("chase_back", "100")
	cvar.Set("chase_up", "16")
	cvar.Set("chase_right", "0")

	camera := g.runtimeCameraState([3]float32{0, 0, 0}, [3]float32{0, 0, 0})
	if math.Abs(float64(camera.Origin.X+100)) > 0.001 || math.Abs(float64(camera.Origin.Y)) > 0.001 || math.Abs(float64(camera.Origin.Z-16)) > 0.001 {
		t.Fatalf("runtimeCameraState chase origin = %v, want {-100 0 16}", camera.Origin)
	}
	if math.Abs(float64(camera.Angles.Y)) > 0.001 {
		t.Fatalf("runtimeCameraState chase yaw = %v, want 0", camera.Angles.Y)
	}
	if camera.Angles.X <= 0 {
		t.Fatalf("runtimeCameraState chase pitch = %v, want positive down-look pitch", camera.Angles.X)
	}
}

func TestRuntimeViewStateInterpolatesYawAcrossWrap(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = [3]float32{32, 64, 96}
	g.Client.MViewAngles[1] = [3]float32{0, 350, 0}
	g.Client.MViewAngles[0] = [3]float32{0, 10, 0}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	markCurrentPredictionFresh(g.Client)

	_, angles := g.runtimeViewState()
	if math.Abs(float64(angles[1]-360)) > 0.01 && math.Abs(float64(angles[1])) > 0.01 {
		t.Fatalf("runtimeViewState wrapped yaw = %v, want 0/360 short-path interpolation", angles[1])
	}
}

func TestCollectViewModelEntityAnchorsToEyeOrigin(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	// Register view-calc cvars needed by collectViewModelEntity.
	cvar.Set("cl_bob", "0")      // disable bob so origin is predictable
	cvar.Set("cl_bobcycle", "0") // zero cycle → bob returns 0
	cvar.Set("cl_bobup", "0.5")
	cvar.Set("v_idlescale", "0") // no idle sway
	cvar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 1
	g.Client.ViewAngles = [3]float32{12, 34, 0}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = [3]float32{100, 200, 300}
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 2},
		},
	}

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != [3]float32{100, 200, 328} {
		t.Fatalf("viewmodel origin = %v, want eye origin [100 200 328]", entity.Origin)
	}
	// viewCalcGunAngle negates pitch: -(12 + 0) = -12.
	if entity.Angles[0] != -12 {
		t.Fatalf("viewmodel pitch = %v, want -12", entity.Angles[0])
	}
	if entity.Angles[1] != 34 {
		t.Fatalf("viewmodel yaw = %v, want 34", entity.Angles[1])
	}
	if entity.Frame != 1 {
		t.Fatalf("viewmodel frame = %d, want 1", entity.Frame)
	}
	if entity.EntityKey != renderer.AliasViewModelEntityKey {
		t.Fatalf("viewmodel entity key = %d, want %d", entity.EntityKey, renderer.AliasViewModelEntityKey)
	}
	if entity.TimeSeconds != g.Client.Time {
		t.Fatalf("viewmodel time = %v, want %v", entity.TimeSeconds, g.Client.Time)
	}
}

func TestCollectViewModelEntitySuppressesIntermission(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := g.collectViewModelEntity(); entity != nil {
		t.Fatalf("g.collectViewModelEntity() = %#v, want nil during intermission", entity)
	}
}

func TestCollectViewModelEntityHonorsDrawViewModelCvar(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		cvar.Set("r_drawviewmodel", "1")
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	cvar.Set("cl_bobcycle", "0") // disable bob for predictable test
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	cvar.Set("r_drawviewmodel", "0")
	if entity := g.collectViewModelEntity(); entity != nil {
		t.Fatalf("g.collectViewModelEntity() = %#v, want nil when r_drawviewmodel=0", entity)
	}

	cvar.Set("r_drawviewmodel", "1")
	if entity := g.collectViewModelEntity(); entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity when r_drawviewmodel=1")
	}
}

func TestCollectViewModelEntitySuppressesWhenInvisible(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
	})

	cvar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Items = cl.ItemInvisibility
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := g.collectViewModelEntity(); entity != nil {
		t.Fatalf("g.collectViewModelEntity() = %#v, want nil when invisibility is active", entity)
	}
}

func TestCollectViewModelEntitySuppressesDuringChaseCamera(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		cvar.Set("chase_active", "0")
	})

	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("chase_active", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}

	if entity := g.collectViewModelEntity(); entity != nil {
		t.Fatalf("g.collectViewModelEntity() = %#v, want nil when chase_active=1", entity)
	}
}

func TestCollectViewModelEntityAppliesPunchAndDamageKickAngles(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
	})

	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("cl_bobup", "0.5")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")
	cvar.Set("v_gunkick", "1")
	cvar.Set("v_kicktime", "1")

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.ViewAngles = [3]float32{12, 34, 0}
	g.Client.PunchAngle = [3]float32{2, 3, 4}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = [3]float32{100, 200, 300}
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	globalViewCalc.dmgTime = 0.5
	globalViewCalc.dmgPitch = 6
	globalViewCalc.dmgRoll = 8

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if entity.Angles[0] != -12 {
		t.Fatalf("viewmodel pitch = %v, want -12", entity.Angles[0])
	}
	if entity.Angles[1] != 34 {
		t.Fatalf("viewmodel yaw = %v, want 34", entity.Angles[1])
	}
	if entity.Angles[2] != 0 {
		t.Fatalf("viewmodel roll = %v, want 0", entity.Angles[2])
	}
}
