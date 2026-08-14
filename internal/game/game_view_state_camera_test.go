package game

// Camera state and view model tests split from game_view_state_test.go.

import (
	"math"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestRuntimeCameraStateResetsStairSmoothingOnTeleport(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalHost := g.Host
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host = originalHost
		g.viewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.OnGround = true
	g.Client.LocalViewTeleport = true
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 0, Y: 0, Z: 300}}
	g.viewCalc.oldZ = 100
	g.viewCalc.oldZInit = true

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 322}, types.Vec3{X: 0, Y: 0, Z: 0})
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
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
	})

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
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Client.LocalViewTeleport = true
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	g.viewCalc.oldZ = 100
	g.viewCalc.oldZInit = true

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != (types.Vec3{X: 100, Y: 200, Z: 322}) {
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

	camera := g.runtimeCameraState(types.Vec3{X: 1, Y: 2, Z: 3}, types.Vec3{X: 4, Y: 5, Z: 6})
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
	g.Client.PunchAngle = types.Vec3{X: 1, Y: -2, Z: 3}

	camera := g.runtimeCameraState(types.Vec3{X: 1, Y: 2, Z: 3}, types.Vec3{X: 10, Y: 20, Z: 30})
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
	g.Client.PunchAngle = types.Vec3{X: 1, Y: -2, Z: 3}

	camera := g.runtimeCameraState(types.Vec3{X: 1, Y: 2, Z: 3}, types.Vec3{X: 10, Y: 20, Z: 30})
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
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 32, Y: 64, Z: 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = types.Vec3{X: 32, Y: 64, Z: 96}
	g.Client.ViewAngles = types.Vec3{X: 45, Y: 135, Z: 225}
	g.Client.MViewAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.MViewAngles[0] = types.Vec3{X: 10, Y: 20, Z: 30}
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
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 32, Y: 64, Z: 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = types.Vec3{X: 32, Y: 64, Z: 96}
	g.Client.ViewAngles = types.Vec3{X: 45, Y: 135, Z: 225}
	g.Client.MViewAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.MViewAngles[0] = types.Vec3{X: 10, Y: 20, Z: 30}
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
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 32, Y: 64, Z: 96}}
	g.Client.ViewHeight = 22
	g.Client.PredictedOrigin = types.Vec3{X: 32, Y: 64, Z: 96}
	g.Client.ViewAngles = types.Vec3{X: 5, Y: 10, Z: 15}
	g.Client.MViewAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.MViewAngles[0] = types.Vec3{X: 10, Y: 20, Z: 30}
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
	originalKick := g.Host.CVar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host.CVar.Set("v_gunkick", originalKick)
	})

	g.Host.CVar.Set("v_gunkick", "2")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.PunchAngles[0] = types.Vec3{X: 10, Y: 0, Z: 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 1, Y: 2, Z: 3})
	if camera.Angles.X < 5.9 || camera.Angles.X > 6.1 {
		t.Fatalf("runtimeCameraState punch interpolation = %v, want ~6", camera.Angles.X)
	}
}

func TestRuntimeCameraStateGunKickModeRaw(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalKick := g.Host.CVar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host.CVar.Set("v_gunkick", originalKick)
	})

	g.Host.CVar.Set("v_gunkick", "1")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = types.Vec3{X: 2, Y: -4, Z: 6}
	g.Client.PunchAngles[1] = types.Vec3{X: 0, Y: 0, Z: 0}
	g.Client.PunchAngles[0] = types.Vec3{X: 10, Y: 0, Z: 0}
	g.Client.PunchTime = 1.0
	g.Client.Time = 1.05

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 1, Y: 2, Z: 3})
	if camera.Angles.X != 3 || camera.Angles.Y != -2 || camera.Angles.Z != 9 {
		t.Fatalf("runtimeCameraState raw punch = %v, want {3 -2 9}", camera.Angles)
	}
}

func TestRuntimeCameraStateGunKickModeOff(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalKick := g.Host.CVar.StringValue("v_gunkick")
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host.CVar.Set("v_gunkick", originalKick)
	})

	g.Host.CVar.Set("v_gunkick", "0")
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.Intermission = 0
	g.Client.PunchAngle = types.Vec3{X: 2, Y: -4, Z: 6}

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 1, Y: 2, Z: 3})
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
	g.Client.PunchAngle = types.Vec3{X: 10, Y: 10, Z: 10}

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 1, Y: 2, Z: 3})
	// Dead players should have roll = 80 and ignore other view effects.
	if camera.Angles.Z != 80 {
		t.Fatalf("runtimeCameraState dead player roll = %v, want 80", camera.Angles.Z)
	}
}

func TestRuntimeCameraStateAppliesChaseCameraWhenActive(t *testing.T) {
	g := New()
	originalClient := g.Client
	if g.Host.CVar.Get("chase_active") == nil {
		g.Host.CVar.Register("chase_active", "0", 0, "")
	}
	if g.Host.CVar.Get("chase_back") == nil {
		g.Host.CVar.Register("chase_back", "100", 0, "")
	}
	if g.Host.CVar.Get("chase_up") == nil {
		g.Host.CVar.Register("chase_up", "16", 0, "")
	}
	if g.Host.CVar.Get("chase_right") == nil {
		g.Host.CVar.Register("chase_right", "0", 0, "")
	}
	originalActive := g.Host.CVar.StringValue("chase_active")
	originalBack := g.Host.CVar.StringValue("chase_back")
	originalUp := g.Host.CVar.StringValue("chase_up")
	originalRight := g.Host.CVar.StringValue("chase_right")
	t.Cleanup(func() {
		g.Client = originalClient
		g.Host.CVar.Set("chase_active", originalActive)
		g.Host.CVar.Set("chase_back", originalBack)
		g.Host.CVar.Set("chase_up", originalUp)
		g.Host.CVar.Set("chase_right", originalRight)
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100
	g.Host.CVar.Set("chase_active", "1")
	g.Host.CVar.Set("chase_back", "100")
	g.Host.CVar.Set("chase_up", "16")
	g.Host.CVar.Set("chase_right", "0")

	camera := g.runtimeCameraState(types.Vec3{X: 0, Y: 0, Z: 0}, types.Vec3{X: 0, Y: 0, Z: 0})
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
	g.Client.PredictedOrigin = types.Vec3{X: 32, Y: 64, Z: 96}
	g.Client.MViewAngles[1] = types.Vec3{X: 0, Y: 350, Z: 0}
	g.Client.MViewAngles[0] = types.Vec3{X: 0, Y: 10, Z: 0}
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = 1.05
	markCurrentPredictionFresh(g.Client)

	_, angles := g.runtimeViewState()
	if math.Abs(float64(angles.Y-360)) > 0.01 && math.Abs(float64(angles.Y)) > 0.01 {
		t.Fatalf("runtimeViewState wrapped yaw = %v, want 0/360 short-path interpolation", angles.Y)
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

	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	// Register view-calc cvars needed by collectViewModelEntity.
	g.Host.CVar.Set("cl_bob", "0")      // disable bob so origin is predictable
	g.Host.CVar.Set("cl_bobcycle", "0") // zero cycle → bob returns 0
	g.Host.CVar.Set("cl_bobup", "0.5")
	g.Host.CVar.Set("v_idlescale", "0") // no idle sway
	g.Host.CVar.Set("r_viewmodel_quake", "0")

	g.Client = cl.NewClient()
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: types.Vec3{X: 100, Y: 200, Z: 300}}
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 1
	g.Client.ViewAngles = types.Vec3{X: 12, Y: 34, Z: 0}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = types.Vec3{X: 100, Y: 200, Z: 300}
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil, nil)
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
	if entity.Origin != (types.Vec3{X: 100, Y: 200, Z: 328}) {
		t.Fatalf("viewmodel origin = %v, want eye origin [100 200 328]", entity.Origin)
	}
	// viewCalcGunAngle negates pitch: -(12 + 0) = -12.
	if entity.Angles.X != -12 {
		t.Fatalf("viewmodel pitch = %v, want -12", entity.Angles.X)
	}
	if entity.Angles.Y != 34 {
		t.Fatalf("viewmodel yaw = %v, want 34", entity.Angles.Y)
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

	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil, nil)
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
	ensureViewCalcCvars(g)
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.Host.CVar.Set("r_drawviewmodel", "1")
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	g.Host.CVar.Set("cl_bobcycle", "0") // disable bob for predictable test
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")

	g.Host.CVar.Set("r_drawviewmodel", "0")
	if entity := g.collectViewModelEntity(); entity != nil {
		t.Fatalf("g.collectViewModelEntity() = %#v, want nil when r_drawviewmodel=0", entity)
	}

	g.Host.CVar.Set("r_drawviewmodel", "1")
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

	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Items = cl.ItemInvisibility
	g.Menu = menu.NewManager(nil, nil, nil)
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
		g.Host.CVar.Set("chase_active", "0")
	})

	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("chase_active", "1")
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatHealth] = 100
	g.Menu = menu.NewManager(nil, nil, nil)
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
	ensureViewCalcCvars(g)
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

	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("cl_bob", "0")
	g.Host.CVar.Set("cl_bobcycle", "0")
	g.Host.CVar.Set("cl_bobup", "0.5")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")
	g.Host.CVar.Set("v_gunkick", "1")
	g.Host.CVar.Set("v_kicktime", "1")

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.ViewAngles = types.Vec3{X: 12, Y: 34, Z: 0}
	g.Client.PunchAngle = types.Vec3{X: 2, Y: 3, Z: 4}
	g.Client.ViewHeight = 28
	g.Client.PredictedOrigin = types.Vec3{X: 100, Y: 200, Z: 300}
	markCurrentPredictionFresh(g.Client)
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1},
		},
	}
	g.viewCalc.dmgTime = 0.5
	g.viewCalc.dmgPitch = 6
	g.viewCalc.dmgRoll = 8

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("g.collectViewModelEntity() = nil, want entity")
	}
	if entity.Angles.X != -12 {
		t.Fatalf("viewmodel pitch = %v, want -12", entity.Angles.X)
	}
	if entity.Angles.Y != 34 {
		t.Fatalf("viewmodel yaw = %v, want 34", entity.Angles.Y)
	}
	if entity.Angles.Z != 0 {
		t.Fatalf("viewmodel roll = %v, want 0", entity.Angles.Z)
	}
}

func TestCollectAliasEntitiesMuzzleFlashSetsLerpResetAnim2(t *testing.T) {
	g := New()
	t.Cleanup(func() { g.Client = nil; g.AliasModelCache = nil })

	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.Time = 1.1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.1, Effects: int(inet.EF_MUZZLEFLASH)},
	}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}

	entities := g.collectAliasEntities()
	if len(entities) != 1 {
		t.Fatalf("collectAliasEntities len = %d, want 1", len(entities))
	}
	want := renderer.LerpResetAnim | renderer.LerpResetAnim2
	if entities[0].LerpFlags&want != want {
		t.Fatalf("LerpFlags = %d, want LerpResetAnim|LerpResetAnim2 (%d) set", entities[0].LerpFlags, want)
	}
}

func TestCollectViewModelEntityMuzzleFlashSetsLerpResetAnim2(t *testing.T) {
	g := New()
	h := host.NewHost()
	if err := h.Init(&host.InitParams{BaseDir: t.TempDir(), UserDir: t.TempDir()}, &host.Subsystems{}); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	// Register CVars that runtimeViewModelVisible checks.
	h.CVar.Register("r_drawentities", "1", 0, "")
	h.CVar.Register("r_drawviewmodel", "1", 0, "")
	h.CVar.Register("chase_active", "0", 0, "")
	h.CVar.Register("viewsize", "100", 0, "")
	originalHost := g.Host
	t.Cleanup(func() { g.Client = nil; g.AliasModelCache = nil; g.Host = originalHost })
	g.Host = h

	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.Time = 1.1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.1, Effects: int(inet.EF_MUZZLEFLASH)},
	}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}

	// Set EF_MUZZLEFLASH on the view entity.
	g.Client.ViewEntity = 1
	state := g.Client.Entities[g.Client.ViewEntity]
	state.Effects = int(inet.EF_MUZZLEFLASH)
	g.Client.Entities[g.Client.ViewEntity] = state

	entity := g.collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity = nil, want entity")
	}
	want := renderer.LerpResetAnim | renderer.LerpResetAnim2
	if entity.LerpFlags&want != want {
		t.Fatalf("viewmodel LerpFlags = %d, want LerpResetAnim|LerpResetAnim2 (%d) set", entity.LerpFlags, want)
	}
}
