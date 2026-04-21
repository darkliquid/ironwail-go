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
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
		g.ParticleRNG = originalRNG
		g.ParticleTime = originalTime
		g.SkyboxNameKey = originalSkyboxKey
		globalViewCalc = originalViewCalc
	})

	g.Renderer = &renderer.Renderer{}
	globalViewCalc = viewCalcState{
		oldGunYaw:   12,
		oldGunPitch: -7,
		dmgTime:     0.5,
		dmgRoll:     3,
		dmgPitch:    -4,
		oldZ:        128,
		oldZInit:    true,
	}

	g.resetRuntimeVisualState()

	if globalViewCalc != (viewCalcState{}) {
		t.Fatalf("globalViewCalc = %+v, want zero value", globalViewCalc)
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

	g.Host = host.NewHost()
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
	g.Host = host.NewHost()
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

	g.Host = host.NewHost()
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

	g.Host = host.NewHost()
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

func TestCollectSpriteEntitiesLoadsRuntimeSprites(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSprite(t, 1, 1),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Origin: [3]float32{7, 8, 9}, Angles: [3]float32{10, 20, 30}, Alpha: 128, Scale: 32},
	}
	g.SpriteModelCache = nil

	entities := g.collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if entities[0].Model == nil || entities[0].Model.Type != model.ModSprite {
		t.Fatalf("collectSpriteEntities model = %#v, want sprite model", entities[0].Model)
	}
	if entities[0].SpriteData == nil || entities[0].SpriteData.NumFrames != 1 {
		t.Fatalf("collectSpriteEntities sprite data = %#v, want loaded sprite data", entities[0].SpriteData)
	}
	if entities[0].Model.SpriteData == nil {
		t.Fatal("collectSpriteEntities model sprite data = nil, want preserved sprite payload")
	}
	if entities[0].Model.SpriteData != entities[0].SpriteData {
		t.Fatal("collectSpriteEntities model SpriteData should reference loaded sprite payload")
	}
	if len(entities[0].Model.SpriteData.Frames) != 1 {
		t.Fatalf("collectSpriteEntities model SpriteData frames = %d, want 1", len(entities[0].Model.SpriteData.Frames))
	}
	frame, ok := entities[0].Model.SpriteData.Frames[0].FramePtr.(*model.MSpriteFrame)
	if !ok || frame == nil {
		t.Fatalf("collectSpriteEntities model frame ptr = %T, want *model.MSpriteFrame", entities[0].Model.SpriteData.Frames[0].FramePtr)
	}
	if len(frame.Pixels) != 1 || frame.Pixels[0] != 1 {
		t.Fatalf("collectSpriteEntities model frame pixels = %v, want [1]", frame.Pixels)
	}
	if got := entities[0].Alpha; math.Abs(float64(got-inet.ENTALPHA_DECODE(128))) > 0.0001 {
		t.Fatalf("collectSpriteEntities alpha = %v, want %v", got, inet.ENTALPHA_DECODE(128))
	}
	if got := entities[0].Scale; math.Abs(float64(got-inet.ENTSCALE_DECODE(32))) > 0.0001 {
		t.Fatalf("collectSpriteEntities scale = %v, want %v", got, inet.ENTSCALE_DECODE(32))
	}
	if got := entities[0].Angles; got != [3]float32{10, 20, 30} {
		t.Fatalf("collectSpriteEntities angles = %v, want [10 20 30]", got)
	}
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after first collect = %d, want 1", got)
	}

	_ = g.collectSpriteEntities()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after cached collect = %d, want 1", got)
	}
}

func TestResolveRuntimeSpriteFrameGroupTimingWraps(t *testing.T) {
	g := New()
	viewForward, viewRight, _ := g.runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{
			{
				Type: model.SpriteFrameGroup,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 3,
					Intervals: []float32{0.1, 0.3, 0.6},
					Frames: []*model.MSpriteFrame{
						{},
						{},
						{},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		clientTime float64
		syncBase   float32
		want       int
	}{
		{name: "first interval", clientTime: 0.05, want: 0},
		{name: "second interval", clientTime: 0.20, want: 1},
		{name: "third interval", clientTime: 0.45, want: 2},
		{name: "wrap interval", clientTime: 0.65, want: 0},
		{name: "positive syncbase offset", clientTime: 0.05, syncBase: 0.20, want: 1},
		{name: "negative syncbase offset", clientTime: 0.05, syncBase: -0.10, want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := inet.EntityState{Frame: 0, SpriteSyncBase: tc.syncBase}
			if got := g.resolveRuntimeSpriteFrame(sprite, state, viewForward, viewRight, tc.clientTime); got != tc.want {
				t.Fatalf("g.resolveRuntimeSpriteFrame(time=%v) = %d, want %d", tc.clientTime, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForGroupedFrames(t *testing.T) {
	g := New()
	viewForward, viewRight, _ := g.runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 3,
		Frames: []model.MSpriteFrameDesc{
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
			{
				Type: model.SpriteFrameGroup,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 2,
					Intervals: []float32{0.2, 0.4},
					Frames: []*model.MSpriteFrame{
						{},
						{},
					},
				},
			},
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
		},
	}

	if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.05); got != 1 {
		t.Fatalf("g.resolveRuntimeSpriteFrame(group first) = %d, want 1", got)
	}
	if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.25); got != 2 {
		t.Fatalf("g.resolveRuntimeSpriteFrame(group second) = %d, want 2", got)
	}
	if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 2}, viewForward, viewRight, 0.25); got != 3 {
		t.Fatalf("g.resolveRuntimeSpriteFrame(single after group) = %d, want 3", got)
	}
	if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1, SpriteSyncBase: 0.2}, viewForward, viewRight, 0.05); got != 2 {
		t.Fatalf("g.resolveRuntimeSpriteFrame(group syncbase offset) = %d, want 2", got)
	}
}

func TestResolveRuntimeSpriteFrameAngledUsesViewDirection(t *testing.T) {
	g := New()
	sprite := &model.MSprite{
		NumFrames: 1,
		Frames: []model.MSpriteFrameDesc{
			{
				Type: model.SpriteFrameAngled,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 8,
					Intervals: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
					Frames: []*model.MSpriteFrame{
						{}, {}, {}, {}, {}, {}, {}, {},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		viewAngles [3]float32
		want       int
	}{
		{name: "front", viewAngles: [3]float32{0, 0, 0}, want: 4},
		{name: "right", viewAngles: [3]float32{0, 90, 0}, want: 6},
		{name: "back", viewAngles: [3]float32{0, 180, 0}, want: 0},
		{name: "left", viewAngles: [3]float32{0, 270, 0}, want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			viewForward, viewRight, _ := g.runtimeAngleVectors(tc.viewAngles)
			if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 0}, viewForward, viewRight, 0.35); got != tc.want {
				t.Fatalf("g.resolveRuntimeSpriteFrame(view=%v) = %d, want %d", tc.viewAngles, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForAngledFrames(t *testing.T) {
	g := New()
	viewForward, viewRight, _ := g.runtimeAngleVectors([3]float32{})
	sprite := &model.MSprite{
		NumFrames: 2,
		Frames: []model.MSpriteFrameDesc{
			{Type: model.SpriteFrameSingle, FramePtr: &model.MSpriteFrame{}},
			{
				Type: model.SpriteFrameAngled,
				FramePtr: &model.MSpriteGroup{
					NumFrames: 8,
					Intervals: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
					Frames: []*model.MSpriteFrame{
						{}, {}, {}, {}, {}, {}, {}, {},
					},
				},
			},
		},
	}

	if got := g.resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.35); got != 5 {
		t.Fatalf("g.resolveRuntimeSpriteFrame(angled offset) = %d, want 5", got)
	}
}

func TestCollectSpriteEntitiesResolvesGroupedFrameFromClientTime(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSpriteGroup(t, 2, []float32{0.2, 0.4}),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Time = 0.25
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0},
	}
	g.SpriteModelCache = nil

	entities := g.collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 1 {
		t.Fatalf("collectSpriteEntities grouped frame = %d, want 1", got)
	}
}

func TestCollectSpriteEntitiesKeepsSTSyncSpritesInLockstep(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSpriteGroupWithSyncType(t, 2, []float32{0.2, 0.4}, model.STSync),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Time = 0.25
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, SpriteSyncBase: 0.3},
	}
	g.Client.StaticEntities = []inet.EntityState{
		{ModelIndex: 1, Frame: 0, SpriteSyncBase: -0.2},
	}
	g.SpriteModelCache = nil

	entities := g.collectSpriteEntities()
	if got := len(entities); got != 2 {
		t.Fatalf("collectSpriteEntities len = %d, want 2", got)
	}
	for i, entity := range entities {
		if entity.Frame != 1 {
			t.Fatalf("collectSpriteEntities[%d] frame = %d, want 1 for lockstep STSync", i, entity.Frame)
		}
	}
	if got := g.Client.Entities[1].SpriteSyncBase; got != 0 {
		t.Fatalf("dynamic STSync SpriteSyncBase = %v, want 0", got)
	}
	if got := g.Client.StaticEntities[0].SpriteSyncBase; got != 0 {
		t.Fatalf("static STSync SpriteSyncBase = %v, want 0", got)
	}
	if got := g.SpriteModelCache["progs/flame.spr"].Model.SyncType; got != model.STSync {
		t.Fatalf("cached runtime model SyncType = %v, want %v", got, model.STSync)
	}
	if got := entities[0].SpriteData.SyncType; got != model.STSync {
		t.Fatalf("runtime sprite SyncType = %v, want %v", got, model.STSync)
	}
}

func TestCollectSpriteEntitiesAssignsAndPreservesRandomSpriteSyncBase(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeSpriteGroupWithSyncType(t, 2, []float32{0.2, 0.4}, model.STRand),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Time = 0.05
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0},
	}
	g.Client.StaticEntities = []inet.EntityState{
		{ModelIndex: 1, Frame: 0},
	}
	g.SpriteModelCache = nil

	first := g.collectSpriteEntities()
	if got := len(first); got != 2 {
		t.Fatalf("first collectSpriteEntities len = %d, want 2", got)
	}

	dynamicState := g.Client.Entities[1]
	staticState := g.Client.StaticEntities[0]
	if dynamicState.SpriteSyncBase <= 0 || dynamicState.SpriteSyncBase > 1 {
		t.Fatalf("dynamic SpriteSyncBase = %v, want (0,1]", dynamicState.SpriteSyncBase)
	}
	if staticState.SpriteSyncBase <= 0 || staticState.SpriteSyncBase > 1 {
		t.Fatalf("static SpriteSyncBase = %v, want (0,1]", staticState.SpriteSyncBase)
	}
	if dynamicState.SpriteSyncBase == staticState.SpriteSyncBase {
		t.Fatalf("dynamic/static SpriteSyncBase both = %v, want distinct randomized offsets", dynamicState.SpriteSyncBase)
	}

	entry := g.SpriteModelCache["progs/flame.spr"]
	viewForward, viewRight, _ := g.runtimeAngleVectors(g.Client.ViewAngles)
	if want := g.resolveRuntimeSpriteFrame(entry.Sprite, dynamicState, viewForward, viewRight, g.Client.Time); first[0].Frame != want {
		t.Fatalf("dynamic grouped frame = %d, want %d", first[0].Frame, want)
	}
	if want := g.resolveRuntimeSpriteFrame(entry.Sprite, staticState, viewForward, viewRight, g.Client.Time); first[1].Frame != want {
		t.Fatalf("static grouped frame = %d, want %d", first[1].Frame, want)
	}
	if got := entry.Model.SyncType; got != model.STRand {
		t.Fatalf("cached runtime model SyncType = %v, want %v", got, model.STRand)
	}

	g.Client.Time = 0.15
	second := g.collectSpriteEntities()
	if got := g.Client.Entities[1].SpriteSyncBase; got != dynamicState.SpriteSyncBase {
		t.Fatalf("dynamic SpriteSyncBase changed from %v to %v", dynamicState.SpriteSyncBase, got)
	}
	if got := g.Client.StaticEntities[0].SpriteSyncBase; got != staticState.SpriteSyncBase {
		t.Fatalf("static SpriteSyncBase changed from %v to %v", staticState.SpriteSyncBase, got)
	}
	if want := g.resolveRuntimeSpriteFrame(entry.Sprite, g.Client.Entities[1], viewForward, viewRight, g.Client.Time); second[0].Frame != want {
		t.Fatalf("second dynamic grouped frame = %d, want %d", second[0].Frame, want)
	}
	if want := g.resolveRuntimeSpriteFrame(entry.Sprite, g.Client.StaticEntities[0], viewForward, viewRight, g.Client.Time); second[1].Frame != want {
		t.Fatalf("second static grouped frame = %d, want %d", second[1].Frame, want)
	}
}

func TestCollectSpriteEntitiesResolvesAngledFrameFromViewAngles(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.SpriteModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.SpriteModelCache = originalCache
	})

	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"progs/flame.spr": testRuntimeAngledSprite(t),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}
	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.ViewAngles = [3]float32{0, 90, 0}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Angles: [3]float32{0, 0, 0}},
	}
	g.SpriteModelCache = nil

	entities := g.collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 6 {
		t.Fatalf("collectSpriteEntities angled frame = %d, want 6", got)
	}
}

func TestEntityStateScaleDecodesProtocolScale(t *testing.T) {
	g := New()
	if got := g.entityStateScale(inet.EntityState{Scale: inet.ENTSCALE_DEFAULT}); got != 1 {
		t.Fatalf("g.entityStateScale(default) = %v, want 1", got)
	}
	if got := g.entityStateScale(inet.EntityState{Scale: 32}); got != 2 {
		t.Fatalf("g.entityStateScale(32) = %v, want 2", got)
	}
	if got := g.entityStateScale(inet.EntityState{}); got != 1 {
		t.Fatalf("g.entityStateScale(zero) = %v, want 1 fallback", got)
	}
}

func TestCollectEntityEffectSourcesIncludesRocketModelFlagWithZeroEffects(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"progs/missile.mdl"}
	g.Client.ModelFlagsFunc = func(name string) int {
		if name == "progs/missile.mdl" {
			return model.EFRocket
		}
		return 0
	}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{1, 2, 3}},
	}

	sources := g.collectEntityEffectSources()
	if got := len(sources); got != 1 {
		t.Fatalf("collectEntityEffectSources len = %d, want 1 rocket source", got)
	}
	if got := sources[0].Effects; got != 0 {
		t.Fatalf("collectEntityEffectSources effects = %d, want 0", got)
	}
	if got := sources[0].ModelFlags; got&model.EFRocket == 0 {
		t.Fatalf("collectEntityEffectSources model flags = %#x, want rocket flag", got)
	}
}

func TestCollectEntityEffectSourcesKeepsAliasEffectsOnly(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{
		"progs/player.mdl",
		"*1",
		"progs/flame.spr",
	}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{1, 2, 3}, Angles: [3]float32{0, 90, 0}, Effects: inet.EF_MUZZLEFLASH},
		2: {ModelIndex: 2, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTLIGHT},
		3: {ModelIndex: 3, Origin: [3]float32{7, 8, 9}, Effects: inet.EF_DIMLIGHT},
		4: {ModelIndex: 1, Origin: [3]float32{9, 9, 9}},
	}
	g.Client.StaticEntities = []inet.EntityState{
		{ModelIndex: 1, Origin: [3]float32{10, 11, 12}, Effects: inet.EF_DIMLIGHT},
	}

	sources := g.collectEntityEffectSources()
	if got := len(sources); got != 2 {
		t.Fatalf("collectEntityEffectSources len = %d, want 2", got)
	}
	if sources[0].Origin != [3]float32{1, 2, 3} || sources[0].Effects != inet.EF_MUZZLEFLASH {
		t.Fatalf("first effect source = %#v, want alias muzzle-flash source", sources[0])
	}
	if sources[1].Origin != [3]float32{10, 11, 12} || sources[1].Effects != inet.EF_DIMLIGHT {
		t.Fatalf("second effect source = %#v, want static alias dim-light source", sources[1])
	}
}

func TestCollectAliasEntitiesSkipsStaleDynamicEntities(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.AliasModelCache = originalCache
	})

	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.Time = 1.25
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.0, LerpFlags: inet.LerpMoveStep | inet.LerpResetMove | inet.LerpResetAnim},
	}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	entities := g.collectAliasEntities()
	if got := len(entities); got != 0 {
		t.Fatalf("collectAliasEntities len = %d, want 0 for stale dynamic alias entity", got)
	}
}

func TestCollectAliasEntitiesKeepsLiveDynamicInterpolationState(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalSubs := g.Subs
	originalCache := g.AliasModelCache
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
		g.AliasModelCache = originalCache
	})

	g.Subs = &host.Subsystems{Files: &runtimeMusicTestFS{files: map[string][]byte{}}}
	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.Time = 1.25
	g.Client.ModelPrecache = []string{"progs/v_axe.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.1, LerpFlags: inet.LerpMoveStep | inet.LerpResetMove | inet.LerpResetAnim},
	}
	g.AliasModelCache = map[string]*model.Model{
		"progs/v_axe.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	entities := g.collectAliasEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectAliasEntities len = %d, want 1 for live alias entity", got)
	}
	if entities[0].EntityKey != 1 {
		t.Fatalf("collectAliasEntities entity key = %d, want 1", entities[0].EntityKey)
	}
	if entities[0].TimeSeconds != g.Client.Time {
		t.Fatalf("collectAliasEntities time = %v, want %v", entities[0].TimeSeconds, g.Client.Time)
	}
	if entities[0].LerpFlags != int(inet.LerpMoveStep|inet.LerpResetMove|inet.LerpResetAnim) {
		t.Fatalf("collectAliasEntities lerp flags = %d, want live flags preserved", entities[0].LerpFlags)
	}
}
