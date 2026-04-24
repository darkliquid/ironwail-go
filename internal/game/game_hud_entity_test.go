package game

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestCollectAliasEntitiesThreadsPlayerColorMap(t *testing.T) {
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
	g.Client.MTime = [2]float64{2, 1}
	g.Client.Time = 2
	g.Client.ModelPrecache = []string{"progs/player.mdl"}
	g.Client.PlayerColors[1] = 0x4f
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 2, Colormap: 1},
	}
	g.AliasModelCache = map[string]*model.Model{
		"progs/player.mdl": {
			Type:        model.ModAlias,
			AliasHeader: &model.AliasHeader{NumFrames: 1, Poses: [][]model.TriVertX{{}}},
		},
	}

	entities := g.collectAliasEntities()
	if len(entities) != 1 {
		t.Fatalf("collectAliasEntities len = %d, want 1", len(entities))
	}
	if !entities[0].IsPlayer {
		t.Fatal("collectAliasEntities IsPlayer = false, want true")
	}
	if entities[0].ColorMap != 0x4f {
		t.Fatalf("collectAliasEntities ColorMap = %#x, want 0x4f", entities[0].ColorMap)
	}
}

func TestCollectSpriteEntitiesSkipsStaleDynamicEntities(t *testing.T) {
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
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.ModelPrecache = []string{"progs/flame.spr"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.0},
	}

	entities := g.collectSpriteEntities()
	if got := len(entities); got != 0 {
		t.Fatalf("collectSpriteEntities len = %d, want 0 for stale sprite entity", got)
	}
}

func TestCollectEntityEffectSourcesSkipsStaleDynamicEntities(t *testing.T) {
	g := New()
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.ModelPrecache = []string{"progs/player.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, MsgTime: 1.0, Origin: [3]float32{1, 2, 3}, Effects: inet.EF_MUZZLEFLASH},
	}

	sources := g.collectEntityEffectSources()
	if got := len(sources); got != 0 {
		t.Fatalf("collectEntityEffectSources len = %d, want 0 for stale dynamic effect source", got)
	}
}

func TestCollectBrushEntitiesSkipsStaleBrushSubmodels(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
	})

	g.Client = cl.NewClient()
	g.Client.MTime = [2]float64{1.1, 1.0}
	g.Client.ModelPrecache = []string{"maps/start.bsp", "*1"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 2, MsgTime: 1.0, Origin: [3]float32{1, 2, 3}},
	}
	g.Server = &server.Server{WorldTree: &bsp.Tree{Models: []bsp.DModel{{}, {}}}}

	brushEntities := g.collectBrushEntities()
	if got := len(brushEntities); got != 0 {
		t.Fatalf("collectBrushEntities len = %d, want 0 for stale brush submodel", got)
	}
}

func TestCollectBrushEntitiesDecodesProtocolAlphaAndScale(t *testing.T) {
	g := New()
	originalClient := g.Client
	originalServer := g.Server
	t.Cleanup(func() {
		g.Client = originalClient
		g.Server = originalServer
	})

	g.Client = cl.NewClient()
	g.Client.ModelPrecache = []string{"maps/start.bsp", "*1"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {
			ModelIndex: 2,
			Frame:      3,
			Origin:     [3]float32{1, 2, 3},
			Angles:     [3]float32{10, 20, 30},
			Alpha:      128,
			Scale:      32,
		},
	}
	g.Server = &server.Server{WorldTree: &bsp.Tree{Models: []bsp.DModel{{}, {}}}}

	brushEntities := g.collectBrushEntities()
	if got := len(brushEntities); got != 1 {
		t.Fatalf("collectBrushEntities len = %d, want 1", got)
	}
	if brushEntities[0].SubmodelIndex != 1 || brushEntities[0].Origin != [3]float32{1, 2, 3} {
		t.Fatalf("brush entity = %#v, want submodel 1 at origin [1 2 3]", brushEntities[0])
	}
	if brushEntities[0].Frame != 3 {
		t.Fatalf("brush frame = %d, want 3", brushEntities[0].Frame)
	}
	if got := brushEntities[0].Alpha; math.Abs(float64(got-inet.ENTALPHA_DECODE(128))) > 0.0001 {
		t.Fatalf("brush alpha = %v, want %v", got, inet.ENTALPHA_DECODE(128))
	}
	if got := brushEntities[0].Scale; math.Abs(float64(got-inet.ENTSCALE_DECODE(32))) > 0.0001 {
		t.Fatalf("brush scale = %v, want %v", got, inet.ENTSCALE_DECODE(32))
	}
}

func TestUpdateHUDFromServerUsesClientState(t *testing.T) {
	g := New()
	originalHUD := g.HUD
	originalClient := g.Client
	originalServer := g.Server
	originalShowScores := g.ShowScores
	t.Cleanup(func() {
		g.HUD = originalHUD
		g.Client = originalClient
		g.Server = originalServer
		g.ShowScores = originalShowScores
	})

	g.HUD = hud.NewHUD(nil, g.Host.CVar)
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 111
	g.Client.Stats[inet.StatArmor] = 55
	g.Client.Stats[inet.StatAmmo] = 22
	g.Client.Stats[inet.StatWeapon] = 7
	g.Client.Stats[inet.StatActiveWeapon] = cl.ItemRocketLauncher
	g.Client.Stats[inet.StatShells] = 10
	g.Client.Stats[inet.StatNails] = 20
	g.Client.Stats[inet.StatRockets] = 30
	g.Client.Stats[inet.StatCells] = 40
	g.Client.Stats[inet.StatTotalSecrets] = 9
	g.Client.Stats[inet.StatTotalMonsters] = 66
	g.Client.Stats[inet.StatSecrets] = 3
	g.Client.Stats[inet.StatMonsters] = 12
	g.Client.MaxClients = 4
	g.Client.GameType = 1
	g.Client.ViewEntity = 2
	g.Client.PlayerNames[0] = "alpha"
	g.Client.PlayerNames[1] = "bravo"
	g.Client.PlayerNames[2] = "charlie"
	g.Client.PlayerColors[0] = 0x1f
	g.Client.PlayerColors[1] = 0x2e
	g.Client.PlayerColors[2] = 0x3d
	g.Client.Frags[0] = 4
	g.Client.Frags[1] = 10
	g.Client.Frags[2] = 6
	g.Client.Items = cl.ItemRocketLauncher | cl.ItemRockets | cl.ItemArmor2 | cl.ItemQuad
	g.Client.Intermission = 2
	g.Client.CompletedTime = 123
	g.Client.Time = 124
	g.Client.CenterPrint = "The End"
	g.Client.CenterPrintAt = 120
	g.Client.Paused = true
	g.Client.LevelName = "Unit Test Map"
	g.ShowScores = true

	g.updateHUDFromServer()

	got := g.HUD.State()
	if got.Health != 111 || got.Armor != 55 || got.Ammo != 22 {
		t.Fatalf("hud core stats = %#v, want health=111 armor=55 ammo=22", got)
	}
	if got.WeaponModel != 7 || got.ActiveWeapon != cl.ItemRocketLauncher {
		t.Fatalf("hud weapon state = %#v, want model=7 active=%d", got, cl.ItemRocketLauncher)
	}
	if got.Shells != 10 || got.Nails != 20 || got.Rockets != 30 || got.Cells != 40 {
		t.Fatalf("hud ammo strip = %#v, want [10 20 30 40]", got)
	}
	if got.Items != g.Client.Items {
		t.Fatalf("hud items = %#x, want %#x", got.Items, g.Client.Items)
	}
	if got.Intermission != 2 || got.CompletedTime != 123 || got.Time != 124 {
		t.Fatalf("hud intermission state = %#v", got)
	}
	if got.CenterPrint != "The End" || got.CenterPrintAt != 120 || got.LevelName != "Unit Test Map" {
		t.Fatalf("hud center/intermission text state = %#v", got)
	}
	if got.FaceAnimUntil != g.Client.FaceAnimUntil {
		t.Fatalf("hud face anim state = %#v, want FaceAnimUntil=%v", got, g.Client.FaceAnimUntil)
	}
	if !got.Paused {
		t.Fatalf("hud paused state = %#v, want Paused=true", got)
	}
	if got.Secrets != 3 || got.TotalSecrets != 9 || got.Monsters != 12 || got.TotalMonsters != 66 {
		t.Fatalf("hud intermission stats = %#v", got)
	}
	if !got.ShowScores || got.GameType != 1 || got.MaxClients != 4 {
		t.Fatalf("hud multiplayer state = %#v", got)
	}
	if len(got.Scoreboard) != 3 {
		t.Fatalf("hud scoreboard len = %d, want 3", len(got.Scoreboard))
	}
	if got.Scoreboard[0].Name != "bravo" || got.Scoreboard[0].Frags != 10 || !got.Scoreboard[0].IsCurrent {
		t.Fatalf("hud scoreboard top row = %#v, want bravo/10/current", got.Scoreboard[0])
	}
}

func TestApplyDefaultGameplayBindings(t *testing.T) {
	g := New()
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	g.Input = input.NewSystem(nil)
	g.applyDefaultGameplayBindings()

	cases := []struct {
		key  int
		want string
	}{
		{key: int('`'), want: "toggleconsole"},
		{key: int('w'), want: "+forward"},
		{key: input.KUpArrow, want: "+forward"},
		{key: input.KMouse1, want: "+attack"},
		{key: input.KMouse2, want: "+jump"},
		{key: input.KTab, want: "+showscores"},
		{key: input.KMWheelUp, want: "impulse 10"},
		{key: input.KMWheelDown, want: "impulse 12"},
	}

	for _, tc := range cases {
		if got := g.Input.Binding(tc.key); got != tc.want {
			t.Fatalf("binding for key %d = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestUpdateHUDFromServerKeepsIntermissionOverlayVisibleOutsideGameplayInput(t *testing.T) {
	g := New()
	originalHUD := g.HUD
	originalClient := g.Client
	originalInput := g.Input
	t.Cleanup(func() {
		g.HUD = originalHUD
		g.Client = originalClient
		g.Input = originalInput
	})

	g.HUD = hud.NewHUD(nil, g.Host.CVar)
	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyConsole)

	g.updateHUDFromServer()

	if got := g.HUD.State(); got.HideIntermissionOverlay {
		t.Fatalf("HideIntermissionOverlay = %v, want false to match C intermission flow", got.HideIntermissionOverlay)
	}
}

func TestDrawRuntimeHUDLayerFallsBackToNativeHUDWhenCSQCDrawFails(t *testing.T) {
	g := New()
	originalHUD := g.HUD
	originalClient := g.Client
	originalCSQC := g.CSQC
	t.Cleanup(func() {
		g.HUD = originalHUD
		g.Client = originalClient
		g.CSQC = originalCSQC
	})

	g.HUD = hud.NewHUD(nil, g.Host.CVar)
	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 123
	g.CSQC = nil

	rc := &consoleOverlayDrawContext{}
	telemetry := TelemetryState{}
	g.drawRuntimeHUDLayer(rc, 320, 200, &telemetry)
	if got := g.HUD.State().Health; got != 123 {
		t.Fatalf("HUD fallback did not refresh native HUD state, health=%d want 123", got)
	}
}
