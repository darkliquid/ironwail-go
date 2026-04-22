package game

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/audio"
	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
)

type runtimeSceneStateTestRenderer struct {
	reloadTestRenderer
	hasWorldData bool
}

func (r runtimeSceneStateTestRenderer) HasWorldData() bool { return r.hasWorldData }

func TestRefreshRuntimeSoundCacheResetsOnPrecacheChange(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalMap := g.SoundSFXByIndex
	originalKey := g.SoundPrecacheKey
	t.Cleanup(func() {
		g.Client = originalClient
		g.SoundSFXByIndex = originalMap
		g.SoundPrecacheKey = originalKey
	})

	g.Client = cl.NewClient()
	g.Client.SoundPrecache = []string{"weapons/rocket1.wav"}
	g.SoundPrecacheKey = "weapons/rocket1.wav"
	g.SoundSFXByIndex = map[int]*audio.SFX{1: nil}

	g.refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 1 {
		t.Fatalf("same precache unexpectedly reset cache; len = %d, want 1", got)
	}

	g.Client.SoundPrecache = []string{"weapons/shotgn2.wav"}
	g.refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("changed precache should reset cache; len = %d, want 0", got)
	}
}

func TestSyncRuntimeStaticSoundsTracksClientStateAndSnapshotChanges(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalAudio := g.Audio
	originalSubs := g.Subs
	originalMap := g.SoundSFXByIndex
	originalPrecacheKey := g.SoundPrecacheKey
	originalStaticKey := g.StaticSoundKey
	t.Cleanup(func() {
		g.Client = originalClient
		g.Audio = originalAudio
		g.Subs = originalSubs
		g.SoundSFXByIndex = originalMap
		g.SoundPrecacheKey = originalPrecacheKey
		g.StaticSoundKey = originalStaticKey
	})

	g.Subs = nil
	g.Audio = audio.NewAudioAdapter(nil)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.SoundPrecache = []string{"ambience/drip.wav"}
	g.Client.StaticSounds = []cl.StaticSound{
		{Origin: [3]float32{10, 20, 30}, SoundIndex: 1, Volume: 255, Attenuation: 1},
	}

	g.syncRuntimeStaticSounds()
	firstKey := g.StaticSoundKey
	if firstKey == "" {
		t.Fatalf("expected static sound snapshot key to be populated")
	}

	g.syncRuntimeStaticSounds()
	if g.StaticSoundKey != firstKey {
		t.Fatalf("unchanged snapshot should not churn static key; got %q, want %q", g.StaticSoundKey, firstKey)
	}

	g.Client.StaticSounds = append(g.Client.StaticSounds, cl.StaticSound{
		Origin: [3]float32{40, 50, 60}, SoundIndex: 2, Volume: 200, Attenuation: 0.5,
	})
	g.syncRuntimeStaticSounds()
	secondKey := g.StaticSoundKey
	if secondKey == firstKey {
		t.Fatalf("static sound list change should rebuild snapshot key")
	}

	g.SoundSFXByIndex = map[int]*audio.SFX{1: nil}
	g.Client.SoundPrecache = []string{"ambience/wind2.wav"}
	g.syncRuntimeStaticSounds()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("precache change should reset runtime SFX cache before static sync; len = %d, want 0", got)
	}
	if g.StaticSoundKey == secondKey {
		t.Fatalf("precache change should rebuild static snapshot key")
	}

	g.Client.State = cl.StateConnected
	g.syncRuntimeStaticSounds()
	if g.StaticSoundKey != "" {
		t.Fatalf("non-active client state should clear static snapshot key, got %q", g.StaticSoundKey)
	}
}

func TestSyncRuntimeVisualEffectsEmitsParticlesAndDecals(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	g.resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ParticleEvents = []cl.ParticleEvent{
		{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 99},
	}
	g.Client.TempEntities = []cl.TempEntityEvent{
		{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}},
	}

	transientEvents := g.Client.ConsumeTransientEvents()
	g.syncRuntimeVisualEffects(0.1, transientEvents)

	if g.Particles == nil || g.Particles.ActiveCount() == 0 {
		t.Fatalf("expected runtime visual sync to emit particles")
	}
	gotMarks := 0
	if g.DecalMarks != nil {
		gotMarks = g.DecalMarks.ActiveCount()
	}
	if gotMarks != 1 {
		t.Fatalf("expected runtime visual sync to emit one decal mark, got %d", gotMarks)
	}
	if got := g.ParticleTime; got <= 0 {
		t.Fatalf("g.ParticleTime = %v, want > 0", got)
	}
	if len(g.Client.ParticleEvents) != 0 || len(g.Client.TempEntities) != 0 {
		t.Fatalf("runtime visual sync should consume client effect buffers")
	}
}

func TestSyncRuntimeVisualEffectsEmitsBrightFieldParticles(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	g.resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ModelPrecache = []string{"progs/player.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTFIELD},
	}

	g.syncRuntimeVisualEffects(0.1, cl.TransientEvents{})

	if g.Particles == nil {
		t.Fatalf("expected runtime visual sync to keep particle system initialized")
	}
	if got := g.Particles.ActiveCount(); got != 162 {
		t.Fatalf("brightfield particle count = %d, want 162", got)
	}
}

func TestSyncRuntimeVisualEffectsResetsEffectsWhenClientInactive(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	t.Cleanup(func() {
		g.Client = originalClient
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
	})

	g.Renderer = &renderer.Renderer{}
	g.resetRuntimeVisualState()
	g.DecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{0, 0, 0},
		Normal: [3]float32{0, 0, 1},
		Size:   8,
		Alpha:  1,
	}, 5, 0)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateConnected
	g.Client.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_EXPLOSION, Origin: [3]float32{1, 1, 1}}}

	transientEvents := g.Client.ConsumeTransientEvents()
	g.syncRuntimeVisualEffects(0.1, transientEvents)

	gotMarks := 0
	if g.DecalMarks != nil {
		gotMarks = g.DecalMarks.ActiveCount()
	}
	if gotMarks != 0 {
		t.Fatalf("inactive client should clear runtime decal marks")
	}
	if g.Particles == nil {
		t.Fatalf("inactive client reset should leave runtime particle system initialized")
	}
	if len(g.Client.TempEntities) != 0 {
		t.Fatalf("inactive client should consume queued temp entities")
	}
}

func TestCollectAliasEntitiesIncludesBeamSegments(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalRuntimeBeams := g.RuntimeBeams
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		g.RuntimeBeams = originalRuntimeBeams
	})

	g.Client = cl.NewClient()
	g.Client.Time = 1
	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.AliasModelCache = map[string]*model.Model{
		"progs/bolt.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}
	g.RuntimeBeams = []cl.BeamSegment{{
		Model:  "progs/bolt.mdl",
		Origin: [3]float32{1, 2, 3},
		Angles: [3]float32{4, 5, 6},
	}}

	entities := g.collectAliasEntities()
	if len(entities) != 1 {
		t.Fatalf("g.collectAliasEntities() len = %d, want 1", len(entities))
	}
	if got := entities[0].ModelID; got != "progs/bolt.mdl" {
		t.Fatalf("beam model = %q, want progs/bolt.mdl", got)
	}
	if got := entities[0].Origin; got != [3]float32{1, 2, 3} {
		t.Fatalf("beam origin = %v, want [1 2 3]", got)
	}
}

func TestResetRuntimeVisualStateResetsPersistentViewCalcState(t *testing.T) {
	g := New()
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	originalRNG := g.ParticleRNG
	originalTime := g.ParticleTime
	originalSkyboxKey := g.SkyboxNameKey
	originalViewCalc := g.viewCalc
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
		g.SkyboxNameKey = originalSkyboxKey
		g.viewCalc = originalViewCalc
	})

	g.Renderer = &renderer.Renderer{}
	g.viewCalc = viewCalcState{
		oldGunYaw:   12,
		oldGunPitch: -7,
		dmgTime:     0.5,
		dmgRoll:     3,
		dmgPitch:    -4,
		oldZ:        128,
		oldZInit:    true,
	}

	g.resetRuntimeVisualState()

	if g.viewCalc != (viewCalcState{}) {
		t.Fatalf("g.viewCalc = %+v, want zero value", g.viewCalc)
	}
}

func TestBuildRuntimeRenderFrameStateIncludesDecalMarks(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalClient := g.Client
	originalServer := g.Server
	originalMenu := g.Menu
	originalDraw := g.Draw
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Server = originalServer
		g.Menu = originalMenu
		g.Draw = originalDraw
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
	})

	g.Host.SetDemoState(&cl.DemoState{Playback: true})
	g.Renderer = &renderer.Renderer{}
	g.Client = cl.NewClient()
	g.Server = nil
	g.Client.FogDensity = 128
	g.Client.FogColor = [3]byte{64, 128, 255}
	g.Menu = nil
	g.Draw = nil
	g.Particles = renderer.NewParticleSystem(renderer.MaxParticles)
	g.DecalMarks = renderer.NewDecalMarkSystem()
	g.DecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{1, 2, 3},
		Normal: [3]float32{0, 0, 1},
		Size:   12,
		Alpha:  1,
	}, 5, 0)

	state := g.buildRuntimeRenderFrameState(nil, nil, []renderer.SpriteEntity{{
		ModelID: "progs/flame.spr",
		Model:   &model.Model{Type: model.ModSprite},
		Scale:   1,
	}}, nil)
	if got := len(state.DecalMarks); got != 1 {
		t.Fatalf("DecalMarks len = %d, want 1", got)
	}
	if got := len(state.SpriteEntities); got != 1 {
		t.Fatalf("SpriteEntities len = %d, want 1", got)
	}
	if !state.DrawEntities {
		t.Fatalf("DrawEntities = false, want true when sprite entities are present")
	}
	if !state.Draw2DOverlay {
		t.Fatalf("Draw2DOverlay = false, want true")
	}
	if math.Abs(float64(state.FogDensity-float32(128)/255.0)) > 0.0001 {
		t.Fatalf("FogDensity = %v, want %v", state.FogDensity, float32(128)/255.0)
	}
	if state.FogColor != [3]float32{64.0 / 255.0, 128.0 / 255.0, 1} {
		t.Fatalf("FogColor = %v, want [64/255 128/255 1]", state.FogColor)
	}
}

func TestBuildRuntimeRenderFrameStateAppliesWorldspawnFogDefaults(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	originalMenu := g.Menu
	originalDraw := g.Draw
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
		g.Menu = originalMenu
		g.Draw = originalDraw
		g.Renderer = originalRenderer
	})

	g.Renderer = &renderer.Renderer{}
	g.Client = cl.NewClient()
	g.Server = &server.Server{
		WorldTree: &bsp.Tree{
			Entities: []byte(`{"classname" "worldspawn" "fog" "0.5 0.25 0.5 0.75"}`),
		},
	}
	g.Menu = nil
	g.Draw = nil

	state := g.buildRuntimeRenderFrameState(nil, nil, nil, nil)

	if math.Abs(float64(state.FogDensity-float32(128)/255.0)) > 0.0001 {
		t.Fatalf("FogDensity = %v, want %v", state.FogDensity, float32(128)/255.0)
	}
	wantColor := [3]float32{64.0 / 255.0, 128.0 / 255.0, 191.0 / 255.0}
	if state.FogColor != wantColor {
		t.Fatalf("FogColor = %v, want %v", state.FogColor, wantColor)
	}
}

func TestBuildRuntimeRenderFrameStatePreservesModelSpriteDataFallback(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalRenderer := g.Renderer
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Renderer = originalRenderer
		g.Client = originalClient
	})
	g.Host.SetDemoState(&cl.DemoState{Playback: true})
	g.Renderer = &renderer.Renderer{}
	g.Client = cl.NewClient()

	spritePayload := &model.MSprite{
		Type:      0,
		MaxWidth:  4,
		MaxHeight: 4,
		NumFrames: 1,
	}
	state := g.buildRuntimeRenderFrameState(nil, nil, []renderer.SpriteEntity{{
		ModelID: "progs/flame.spr",
		Model: &model.Model{
			Type:       model.ModSprite,
			SpriteData: spritePayload,
		},
		SpriteData: nil,
	}}, nil)

	if got := len(state.SpriteEntities); got != 1 {
		t.Fatalf("SpriteEntities len = %d, want 1", got)
	}
	if state.SpriteEntities[0].SpriteData != nil {
		t.Fatalf("SpriteEntities[0].SpriteData = %#v, want nil explicit payload", state.SpriteEntities[0].SpriteData)
	}
	if state.SpriteEntities[0].Model == nil || state.SpriteEntities[0].Model.SpriteData != spritePayload {
		t.Fatal("SpriteEntities[0].Model.SpriteData should preserve fallback sprite payload")
	}
}

func TestBuildRuntimeRenderFrameStateSuppressesStaleSceneWhenDisconnected(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalRenderer := g.Renderer
	originalClient := g.Client
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	t.Cleanup(func() {
		g.Host = originalHost
		g.Renderer = originalRenderer
		g.Client = originalClient
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
	})

	g.Renderer = runtimeSceneStateTestRenderer{hasWorldData: true}
	g.Client = cl.NewClient()
	g.Client.State = cl.StateDisconnected
	g.Particles = renderer.NewParticleSystem(renderer.MaxParticles)
	g.DecalMarks = renderer.NewDecalMarkSystem()
	g.DecalMarks.AddMark(renderer.DecalMarkEntity{
		Origin: [3]float32{1, 2, 3},
		Normal: [3]float32{0, 0, 1},
		Size:   12,
		Alpha:  1,
	}, 5, 0)

	state := g.buildRuntimeRenderFrameState(
		[]renderer.BrushEntity{{}},
		[]renderer.AliasModelEntity{{}},
		[]renderer.SpriteEntity{{ModelID: "progs/flame.spr"}},
		&renderer.AliasModelEntity{},
	)

	if state.DrawWorld {
		t.Fatal("DrawWorld = true, want false when disconnected and not playing a demo")
	}
	if state.DrawEntities {
		t.Fatal("DrawEntities = true, want false when disconnected and not playing a demo")
	}
	if state.DrawParticles {
		t.Fatal("DrawParticles = true, want false when disconnected and not playing a demo")
	}
	if len(state.DecalMarks) != 0 {
		t.Fatalf("DecalMarks len = %d, want 0 when disconnected and not playing a demo", len(state.DecalMarks))
	}
}

func TestBuildRuntimeRenderFrameStateKeepsDemoSceneWhileDisconnected(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalRenderer := g.Renderer
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Renderer = originalRenderer
		g.Client = originalClient
	})

	g.Host.SetDemoState(&cl.DemoState{Playback: true})
	g.Renderer = runtimeSceneStateTestRenderer{hasWorldData: true}
	g.Client = cl.NewClient()
	g.Client.State = cl.StateDisconnected

	state := g.buildRuntimeRenderFrameState(nil, nil, []renderer.SpriteEntity{{
		ModelID: "progs/flame.spr",
		Model:   &model.Model{Type: model.ModSprite},
	}}, nil)

	if !state.DrawWorld {
		t.Fatal("DrawWorld = false, want true during demo playback")
	}
	if !state.DrawEntities {
		t.Fatal("DrawEntities = false, want true during demo playback")
	}
}
