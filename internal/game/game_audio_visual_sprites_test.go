package game

// Sprite and entity collection tests split from game_audio_visual_test.go.

import (
	"math"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

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
		1: {ModelIndex: 1, Frame: 0, Origin: types.Vec3{X: 7, Y: 8, Z: 9}, Angles: types.Vec3{X: 10, Y: 20, Z: 30}, Alpha: 128, Scale: 32},
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
	if got := entities[0].Angles; got != (types.Vec3{X: 10, Y: 20, Z: 30}) {
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
	viewForward, viewRight, _ := g.runtimeAngleVectors(types.Vec3{})
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
	viewForward, viewRight, _ := g.runtimeAngleVectors(types.Vec3{})
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
		viewAngles types.Vec3
		want       int
	}{
		{name: "front", viewAngles: types.Vec3{X: 0, Y: 0, Z: 0}, want: 4},
		{name: "right", viewAngles: types.Vec3{X: 0, Y: 90, Z: 0}, want: 6},
		{name: "back", viewAngles: types.Vec3{X: 0, Y: 180, Z: 0}, want: 0},
		{name: "left", viewAngles: types.Vec3{X: 0, Y: 270, Z: 0}, want: 2},
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
	viewForward, viewRight, _ := g.runtimeAngleVectors(types.Vec3{})
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
	g.Client.ViewAngles = types.Vec3{X: 0, Y: 90, Z: 0}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Frame: 0, Angles: types.Vec3{X: 0, Y: 0, Z: 0}},
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
		1: {ModelIndex: 1, Origin: types.Vec3{X: 1, Y: 2, Z: 3}},
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
		1: {ModelIndex: 1, Origin: types.Vec3{X: 1, Y: 2, Z: 3}, Angles: types.Vec3{X: 0, Y: 90, Z: 0}, Effects: inet.EF_MUZZLEFLASH},
		2: {ModelIndex: 2, Origin: types.Vec3{X: 4, Y: 5, Z: 6}, Effects: inet.EF_BRIGHTLIGHT},
		3: {ModelIndex: 3, Origin: types.Vec3{X: 7, Y: 8, Z: 9}, Effects: inet.EF_DIMLIGHT},
		4: {ModelIndex: 1, Origin: types.Vec3{X: 9, Y: 9, Z: 9}},
	}
	g.Client.StaticEntities = []inet.EntityState{
		{ModelIndex: 1, Origin: types.Vec3{X: 10, Y: 11, Z: 12}, Effects: inet.EF_DIMLIGHT},
	}

	sources := g.collectEntityEffectSources()
	if got := len(sources); got != 2 {
		t.Fatalf("collectEntityEffectSources len = %d, want 2", got)
	}
	if sources[0].Origin != (types.Vec3{X: 1, Y: 2, Z: 3}) || sources[0].Effects != inet.EF_MUZZLEFLASH {
		t.Fatalf("first effect source = %#v, want alias muzzle-flash source", sources[0])
	}
	if sources[1].Origin != (types.Vec3{X: 10, Y: 11, Z: 12}) || sources[1].Effects != inet.EF_DIMLIGHT {
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
	if entities[0].LerpFlags != renderer.LerpMoveStep|renderer.LerpResetMove|renderer.LerpResetAnim {
		t.Fatalf("collectAliasEntities lerp flags = %d, want renderer-translated flags (LerpMoveStep|LerpResetMove|LerpResetAnim = %d)", entities[0].LerpFlags, renderer.LerpMoveStep|renderer.LerpResetMove|renderer.LerpResetAnim)
	}
}
