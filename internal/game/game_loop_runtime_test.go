package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestRunRuntimeFrameRunsClientPrediction(t *testing.T) {
	g := New()
	g.Host = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.Time = g.Client.MTime[0]
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
}

func TestRunRuntimeFrameSyncsAudioViewEntity(t *testing.T) {
	g := New()
	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	g.Host = nil
	g.Audio = audio.NewAudioAdapter(sys)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 3
	g.Client.ViewHeight = 22
	g.Client.Entities[3] = inet.EntityState{Origin: [3]float32{64, 32, 16}}

	g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if got := sys.ViewEntity(); got != 3 {
		t.Fatalf("audio view entity after active client frame = %d, want 3", got)
	}

	g.Client = nil
	g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if got := sys.ViewEntity(); got != 0 {
		t.Fatalf("audio view entity after clearing client = %d, want 0", got)
	}
}

func TestRunRuntimeFrameUpdatesLeafAmbientAndUnderwaterAudio(t *testing.T) {
	g := New()
	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	g.Audio = audio.NewAudioAdapter(sys)
	g.Audio.SetAmbientSound(0, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})
	g.Audio.SetAmbientSound(1, &audio.SFX{Cache: &audio.SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}})

	g.Host = nil
	g.Subs = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 0
	g.Client.Entities[1] = inet.EntityState{
		Origin:     [3]float32{64, 0, 0},
		MsgOrigins: [2][3]float32{{64, 0, 0}, {64, 0, 0}},
		MsgTime:    g.Client.MTime[0],
	}
	g.Server = &server.Server{
		WorldTree: &bsp.Tree{
			Planes: []bsp.DPlane{
				{Normal: [3]float32{1, 0, 0}, Dist: 0},
			},
			Nodes: []bsp.TreeNode{
				{
					PlaneNum: 0,
					Children: [2]bsp.TreeChild{
						{IsLeaf: true, Index: 1},
						{IsLeaf: true, Index: 2},
					},
				},
			},
			Leafs: []bsp.TreeLeaf{
				{Contents: bsp.ContentsSolid},
				{Contents: bsp.ContentsWater, AmbientLevel: [bsp.NumAmbients]uint8{80, 80, 0, 0}},
				{Contents: bsp.ContentsEmpty, AmbientLevel: [bsp.NumAmbients]uint8{0, 0, 0, 0}},
			},
		},
	}

	g.RunRuntimeFrame(0.1, gameCallbacks{g: g})
	if got := sys.UnderwaterIntensity(); got <= 0 {
		t.Fatalf("underwater intensity in water leaf = %v, want > 0", got)
	}
	if got := sys.ViewEntity(); got != 1 {
		t.Fatalf("audio view entity after leaf update = %d, want 1", got)
	}
	if got := sys.AmbientVolume(0); got != 10 {
		t.Fatalf("ambient channel 0 volume = %d, want 10", got)
	}
	if got := sys.AmbientVolume(1); got != 10 {
		t.Fatalf("ambient channel 1 volume = %d, want 10", got)
	}

	g.Client.Entities[1] = inet.EntityState{
		Origin:     [3]float32{-64, 0, 0},
		MsgOrigins: [2][3]float32{{-64, 0, 0}, {-64, 0, 0}},
		MsgTime:    g.Client.MTime[0],
	}
	g.RunRuntimeFrame(0.1, gameCallbacks{g: g})
	if got := sys.UnderwaterIntensity(); got != 0 {
		t.Fatalf("underwater intensity in dry leaf = %v, want 0", got)
	}
	if got := sys.AmbientVolume(0); got != 0 {
		t.Fatalf("ambient channel 0 volume in dry leaf = %d, want 0", got)
	}
	if got := sys.AmbientVolume(1); got != 0 {
		t.Fatalf("ambient channel 1 volume in dry leaf = %d, want 0", got)
	}

	g.Server = nil
	g.RunRuntimeFrame(0.1, gameCallbacks{g: g})
	if sys.AmbientSound(0) != nil || sys.AmbientSound(1) != nil {
		t.Fatalf("ambient channels should clear when no world tree is available")
	}
}

func TestRunRuntimeFrameConsumesTransientEventsOnce(t *testing.T) {
	g := New()
	g.Host = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.SoundEvents = []cl.SoundEvent{{Entity: 1, Channel: 2, SoundIndex: 3}}
	g.Client.StopSoundEvents = []cl.StopSoundEvent{{Entity: 4, Channel: 5}}
	g.Client.ParticleEvents = []cl.ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	g.Client.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}}}

	events := g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if len(events.SoundEvents) != 1 || len(events.StopSoundEvents) != 1 || len(events.ParticleEvents) != 1 || len(events.TempEntities) != 1 {
		t.Fatalf("RunRuntimeFrame consumed = %d sounds, %d stops, %d particles, %d temps; want 1,1,1,1", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
	if len(g.Client.SoundEvents) != 0 || len(g.Client.StopSoundEvents) != 0 || len(g.Client.ParticleEvents) != 0 || len(g.Client.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d sounds %d stops %d particles %d temps", len(g.Client.SoundEvents), len(g.Client.StopSoundEvents), len(g.Client.ParticleEvents), len(g.Client.TempEntities))
	}

	events = g.RunRuntimeFrame(0.016, gameCallbacks{g: g})
	if len(events.SoundEvents) != 0 || len(events.StopSoundEvents) != 0 || len(events.ParticleEvents) != 0 || len(events.TempEntities) != 0 {
		t.Fatalf("second frame consumed = %d sounds, %d stops, %d particles, %d temps; want 0,0,0,0", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
}

func TestRunRuntimeFrameRelinksBeforeViewAndViewModelConsumers(t *testing.T) {
	g := New()
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		globalViewCalc = originalViewCalc
	})

	ensureViewCalcCvars()
	cvar.Set("r_drawentities", "1")
	cvar.Set("r_drawviewmodel", "1")
	cvar.Set("chase_active", "0")
	cvar.Set("cl_bob", "0")
	cvar.Set("cl_bobcycle", "0")
	cvar.Set("v_idlescale", "0")
	cvar.Set("r_viewmodel_quake", "0")

	g.Host = nil
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
	globalViewCalc.oldZInit = false

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.ViewHeight = 20
	g.Client.Time = 1.05
	g.Client.MTime[1] = 1.0
	g.Client.MTime[0] = 1.1
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Stats[inet.StatHealth] = 100
	g.Client.Stats[inet.StatWeapon] = 1
	g.Client.Stats[inet.StatWeaponFrame] = 0
	g.Client.Entities[1] = inet.EntityState{
		ModelIndex:  1,
		Origin:      [3]float32{0, 0, 0},
		MsgOrigins:  [2][3]float32{{100, 0, 0}, {0, 0, 0}},
		MsgAngles:   [2][3]float32{{0, 0, 0}, {0, 0, 0}},
		MsgTime:     1.1,
		TrailOrigin: [3]float32{0, 0, 0},
	}

	g.RunRuntimeFrame(0.016, gameCallbacks{g: g})

	viewOrigin, _ := g.runtimeViewState()
	if want := [3]float32{50, 0, 20}; viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want relinked origin %v", viewOrigin, want)
	}

	viewModel := g.collectViewModelEntity()
	if viewModel == nil {
		t.Fatal("collectViewModelEntity() = nil, want viewmodel")
	}
	if viewModel.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want same relinked eye origin %v", viewModel.Origin, viewOrigin)
	}
}
