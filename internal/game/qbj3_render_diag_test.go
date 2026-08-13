package game

import (
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj3RenderLayerCollectsWeaponAndKeycard builds the exact client state
// observed on qbj3_pixeldud through the loopback e2e (weaponStat=88 ->
// progs/v_wrench.mdl, keycard edict 16 -> progs/b_s_key.mdl) and verifies the
// render-stage collectors emit a first-person viewmodel and the keycard alias
// entity. This is the layer that decides whether the player sees them.
func TestQbj3RenderLayerCollectsWeaponAndKeycard(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	vfs := fs.NewFileSystem()
	if err := vfs.Init(quakeDir, "qbj3"); err != nil {
		t.Fatalf("Init(qbj3): %v", err)
	}
	t.Cleanup(func() { vfs.Close() })

	// prime the VFS so loadAliasModel resolves real qbj3 models
	dataWrench, err := vfs.LoadFile("progs/v_wrench.mdl")
	if err != nil {
		t.Fatalf("load v_wrench: %v", err)
	}
	dataKey, err := vfs.LoadFile("progs/b_s_key.mdl")
	if err != nil {
		t.Fatalf("load b_s_key: %v", err)
	}
	_ = dataWrench
	_ = dataKey

	g := New()
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := g.viewCalc
	originalHost := g.Host
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.viewCalc = originalViewCalc
		g.Host = originalHost
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 22
	g.Client.ViewAngles = [3]float32{0, 0, 0}
	g.Client.ViewEntAlpha = inet.ENTALPHA_DEFAULT
	g.Client.Intermission = 0

	// Real qbj3 precache layout (server FindModel indexes): v_wrench at 88,
	// b_s_key at 174. Build a ModelPrecache slice of that length with real
	// names so WeaponModelIndex()-1 / ModelIndex-1 resolve like a live session.
	precache := make([]string, 174)
	precache[87] = "progs/v_wrench.mdl"
	precache[173] = "progs/b_s_key.mdl"
	g.Client.ModelPrecache = precache

	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 88
	g.Client.Stats[inet.StatWeaponFrame] = 1
	g.Client.Stats[inet.StatActiveWeapon] = 4096
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{0, 0, 0}, ModelIndex: 85}
	g.Client.Entities[16] = inet.EntityState{Origin: [3]float32{1024, 0, 504}, ModelIndex: 174}

	g.Host = originalHost
	g.Host.CVar.Set("r_drawentities", "1")
	g.Host.CVar.Set("r_drawviewmodel", "1")
	g.Host.CVar.Set("cl_bob", "0")
	g.Host.CVar.Set("cl_bobcycle", "0")
	g.Host.CVar.Set("v_idlescale", "0")
	g.Host.CVar.Set("r_viewmodel_quake", "0")
	g.Menu = menu.NewManager(nil, nil, nil)
	g.Subs = &host.Subsystems{Files: vfs}
	g.AliasModelCache = map[string]*model.Model{}

	vm := g.collectViewModelEntity()
	if vm == nil {
		t.Fatal("collectViewModelEntity() = nil, want v_wrench viewmodel")
	}
	if vm.ModelID != "progs/v_wrench.mdl" {
		t.Fatalf("viewmodel ModelID = %q, want progs/v_wrench.mdl", vm.ModelID)
	}
	if vm.Model == nil || vm.Model.AliasHeader == nil || vm.Model.AliasHeader.NumFrames == 0 {
		t.Fatalf("viewmodel alias header empty: %+v", vm.Model)
	}
	t.Logf("viewmodel OK: %s frames=%d", vm.ModelID, vm.Model.AliasHeader.NumFrames)

	alias := g.collectAliasEntities()
	foundKeycard := false
	for _, ent := range alias {
		if ent.EntityKey == 16 && ent.ModelID == "progs/b_s_key.mdl" {
			foundKeycard = true
			t.Logf("keycard alias entity OK: model=%s origin=%v", ent.ModelID, ent.Origin)
			break
		}
	}
	if !foundKeycard {
		t.Fatal("collectAliasEntities() did not include keycard (b_s_key.mdl)")
	}

	// Exhaustive live-path check: with an active client session, the frame
	// state builder must keep the viewmodel and alias entities (this is what
	// RenderFrame consumes). A nil ViewModel here means the renderer draws
	// nothing even though collectors return valid entities.
	g.Host.SetClientState(host.ClientState(2))
	state := g.buildRuntimeRenderFrameState(nil, alias, nil, vm)
	if state.ViewModel == nil {
		t.Fatal("buildRuntimeRenderFrameState: ViewModel = nil with active session, viewmodel would not render")
	}
	if state.ViewModel.ModelID != "progs/v_wrench.mdl" {
		t.Fatalf("buildRuntimeRenderFrameState ViewModel.ModelID = %q", state.ViewModel.ModelID)
	}
	if len(state.AliasEntities) == 0 {
		t.Fatal("buildRuntimeRenderFrameState: AliasEntities empty with active session, pickups would not render")
	}
	keyInState := false
	for _, ent := range state.AliasEntities {
		if ent.EntityKey == 16 && ent.ModelID == "progs/b_s_key.mdl" {
			keyInState = true
			break
		}
	}
	if !keyInState {
		t.Fatal("buildRuntimeRenderFrameState: keycard alias entity missing from frame state")
	}
	t.Logf("RenderFrameState OK: viewmodel=%s aliasEntities=%d (keycard present)",
		state.ViewModel.ModelID, len(state.AliasEntities))
}
