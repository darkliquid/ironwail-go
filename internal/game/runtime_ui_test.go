package game

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/host"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func TestDrawMenuBackdropUsesAlphaFill(t *testing.T) {
	g := New()
	g.Host.CVar.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "")
	dc := &consoleOverlayDrawContext{}

	g.drawMenuBackdrop(dc, 8, 10)

	if len(dc.fills) != 1 {
		t.Fatalf("fill count = %d, want 1", len(dc.fills))
	}
	if got := dc.fills[0]; got.x != 0 || got.y != 0 || got.w != 8 || got.h != 10 || got.color != 0 || math.Abs(float64(got.alpha)-0.7) > 0.0001 {
		t.Fatalf("fill = %+v, want x=0 y=0 w=8 h=10 color=0 alpha=0.7", got)
	}
	if dc.canvas.Type != renderer.CanvasDefault {
		t.Fatalf("backdrop canvas = %v, want %v", dc.canvas.Type, renderer.CanvasDefault)
	}
}

func TestDrawMenuBackdropClampsAlphaCVar(t *testing.T) {
	g := New()
	g.Host.CVar.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "")
	dc := &consoleOverlayDrawContext{}

	g.Host.CVar.Set("scr_menubgalpha", "2")
	g.drawMenuBackdrop(dc, 8, 10)
	g.Host.CVar.Set("scr_menubgalpha", "-1")
	g.drawMenuBackdrop(dc, 8, 10)
	t.Cleanup(func() {
		g.Host.CVar.Set("scr_menubgalpha", "0.7")
	})

	if len(dc.fills) != 2 {
		t.Fatalf("fill count = %d, want 2", len(dc.fills))
	}
	if got := dc.fills[0].alpha; math.Abs(float64(got)-1) > 0.0001 {
		t.Fatalf("clamped high alpha = %f, want 1", got)
	}
	if got := dc.fills[1].alpha; math.Abs(float64(got)-0) > 0.0001 {
		t.Fatalf("clamped low alpha = %f, want 0", got)
	}
}

func TestDrawRuntimeMenuDrawsBackdropBeforeMenu(t *testing.T) {
	g := New()
	registerConsoleCanvasTestCvars(g)
	g.Host.CVar.Set("vid_width", "1280")
	g.Host.CVar.Set("vid_height", "720")
	g.Host.CVar.Set("scr_pixelaspect", "1")
	g.Host.CVar.Set("scr_menuscale", "2.25")

	dc := &consoleOverlayDrawContext{}
	menuDrawCalled := false

	g.drawRuntimeMenu(dc, 16, 12, func(rc renderer.RenderContext) {
		menuDrawCalled = true
		if len(dc.fills) == 0 {
			t.Fatal("expected backdrop fills before menu draw")
		}
		if rc.Canvas().Type != renderer.CanvasMenu {
			t.Fatalf("menu canvas = %v, want %v", rc.Canvas().Type, renderer.CanvasMenu)
		}
		rc.DrawCharacter(24, 32, 'M')
	})

	if !menuDrawCalled {
		t.Fatal("menu draw callback was not invoked")
	}
	if len(dc.canvasParams) != 1 {
		t.Fatalf("canvas params count = %d, want 1", len(dc.canvasParams))
	}
	params := dc.canvasParams[0]
	if params.GUIWidth != 16 || params.GUIHeight != 12 {
		t.Fatalf("menu GUI params = %.0fx%.0f, want 16x12", params.GUIWidth, params.GUIHeight)
	}
	if math.Abs(float64(params.MenuScale-2.25)) > 0.0001 {
		t.Fatalf("menu scale = %.2f, want 2.25", params.MenuScale)
	}
	if len(dc.chars) != 1 || dc.chars[0].num != 'M' {
		t.Fatalf("menu draw chars = %+v, want one 'M'", dc.chars)
	}
}

func TestRuntimeConsoleDimensionsMatchCReferenceSizing(t *testing.T) {
	g := New()
	registerConsoleCanvasTestCvars(g)
	g.Host.CVar.Set("scr_pixelaspect", "1")
	g.Host.CVar.Set("scr_conwidth", "0")
	g.Host.CVar.Set("scr_conscale", "2")

	if gotW, gotH := g.runtimeConsoleDimensions(1280, 720); gotW != 640 || gotH != 360 {
		t.Fatalf("runtimeConsoleDimensions = %dx%d, want 640x360", gotW, gotH)
	}

	g.Host.CVar.Set("scr_conwidth", "200")
	g.Host.CVar.Set("scr_conscale", "1")
	if gotW, gotH := g.runtimeConsoleDimensions(1280, 720); gotW != 320 || gotH != 180 {
		t.Fatalf("runtimeConsoleDimensions clamp = %dx%d, want 320x180", gotW, gotH)
	}
}

func TestRuntimeGUIDimensionsApplyPixelAspect(t *testing.T) {
	g := New()
	registerConsoleCanvasTestCvars(g)

	g.Host.CVar.Set("vid_width", "1280")
	g.Host.CVar.Set("vid_height", "720")
	g.Host.CVar.Set("scr_pixelaspect", "5:6")
	if gotW, gotH := g.runtimeGUIDimensions(1280, 720); gotW != 1280 || gotH != 600 {
		t.Fatalf("runtimeGUIDimensions tall pixels = %dx%d, want 1280x600", gotW, gotH)
	}

	g.Host.CVar.Set("scr_pixelaspect", "1.5")
	if gotW, gotH := g.runtimeGUIDimensions(1280, 720); gotW != 853 || gotH != 720 {
		t.Fatalf("runtimeGUIDimensions wide pixels = %dx%d, want 853x720", gotW, gotH)
	}
}

func TestDrawRuntimeConsoleUsesConsoleCanvasAndBackgroundPic(t *testing.T) {
	g := New()
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	registerConsoleCanvasTestCvars(g)
	g.Host.CVar.Set("vid_width", "1280")
	g.Host.CVar.Set("vid_height", "720")
	g.Host.CVar.Set("scr_pixelaspect", "1")
	g.Host.CVar.Set("scr_conwidth", "0")
	g.Host.CVar.Set("scr_conscale", "2")

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	console.Printf("console line")

	palette := make([]byte, 768)
	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"conback": {
			Width:  320,
			Height: 200,
			Pixels: make([]byte, 320*200),
		},
	}, palette)

	dc := &consoleOverlayDrawContext{}
	g.ConsoleSlideFraction = 0.5
	g.drawRuntimeConsole(dc, 1864, 1428, true, false)

	if got := dc.Canvas().Type; got != renderer.CanvasConsole {
		t.Fatalf("console canvas = %v, want %v", got, renderer.CanvasConsole)
	}
	if len(dc.canvasParams) != 1 {
		t.Fatalf("canvas params count = %d, want 1", len(dc.canvasParams))
	}
	params := dc.canvasParams[0]
	if params.GUIWidth != 1864 || params.GUIHeight != 1428 {
		t.Fatalf("GUI params = %.0fx%.0f, want 1864x1428", params.GUIWidth, params.GUIHeight)
	}
	if params.GLWidth != 1864 || params.GLHeight != 1428 {
		t.Fatalf("GL params = %.0fx%.0f, want 1864x1428", params.GLWidth, params.GLHeight)
	}
	if params.ConWidth != 928 || params.ConHeight != 710 {
		t.Fatalf("console params = %.0fx%.0f, want 928x710", params.ConWidth, params.ConHeight)
	}
	if math.Abs(float64(params.ConSlideFraction-0.5)) > 0.0001 {
		t.Fatalf("console slide fraction = %.2f, want 0.50", params.ConSlideFraction)
	}
	if len(dc.pics) != 1 {
		t.Fatalf("background pic draws = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0].pic.Width; got != 928 {
		t.Fatalf("background width = %d, want 928", got)
	}
	if got := dc.pics[0].pic.Height; got != 355 {
		t.Fatalf("background height = %d, want 355", got)
	}
	if got := len(dc.pics[0].pic.Pixels); got != 928*355 {
		t.Fatalf("background pixel count = %d, want %d", got, 928*355)
	}
	if len(dc.fills) != 0 {
		t.Fatalf("unexpected solid fills when conback is present: %d", len(dc.fills))
	}
	if len(dc.chars) == 0 {
		t.Fatal("expected console text to be drawn")
	}
}

func TestDrawRuntimeConsoleUsesPixelAspectAdjustedGUI(t *testing.T) {
	g := New()
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	registerConsoleCanvasTestCvars(g)
	g.Host.CVar.Set("vid_width", "1280")
	g.Host.CVar.Set("vid_height", "720")
	g.Host.CVar.Set("scr_pixelaspect", "5:6")
	g.Host.CVar.Set("scr_conwidth", "0")
	g.Host.CVar.Set("scr_conscale", "2")

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"conback": {
			Width:  320,
			Height: 200,
			Pixels: make([]byte, 320*200),
		},
	}, make([]byte, 768))

	dc := &consoleOverlayDrawContext{}
	g.drawRuntimeConsole(dc, 1280, 720, true, false)

	if len(dc.canvasParams) != 1 {
		t.Fatalf("canvas params count = %d, want 1", len(dc.canvasParams))
	}
	params := dc.canvasParams[0]
	if params.GUIWidth != 1280 || params.GUIHeight != 600 {
		t.Fatalf("GUI params = %.0fx%.0f, want 1280x600", params.GUIWidth, params.GUIHeight)
	}
	if params.ConWidth != 640 || params.ConHeight != 300 {
		t.Fatalf("console params = %.0fx%.0f, want 640x300", params.ConWidth, params.ConHeight)
	}
}

func TestScreenToMenuCoordsUsesCanvasMenuTransform(t *testing.T) {
	g := New()
	registerConsoleCanvasTestCvars(g)
	g.Host.CVar.Set("vid_width", "320")
	g.Host.CVar.Set("vid_height", "200")
	g.Host.CVar.Set("scr_pixelaspect", "1")
	g.Host.CVar.Set("scr_menuscale", "1")

	params := g.runtimeOverlayCanvasParams(320, 200)
	transform := renderer.GetCanvasTransform(renderer.CanvasMenu, params)
	menuX, menuY := 160.75, 72.75
	ndcX := transform.Scale[0]*float32(menuX) + transform.Offset[0]
	ndcY := transform.Scale[1]*float32(menuY) + transform.Offset[1]
	screenX := int(math.Floor(float64((ndcX+1)*params.GLWidth*0.5 - 0.5)))
	screenY := int(math.Floor(float64((1-ndcY)*params.GLHeight*0.5 - 0.5)))

	gotX, gotY, ok := g.screenToMenuCoords(screenX, screenY)
	if !ok {
		t.Fatalf("screenToMenuCoords(%d,%d) reported outside menu", screenX, screenY)
	}
	if gotX != 160 || gotY != 72 {
		t.Fatalf("screenToMenuCoords(%d,%d) = (%d,%d), want (160,72)", screenX, screenY, gotX, gotY)
	}
}

func TestUpdateRuntimeConsoleSlide(t *testing.T) {
	g := New()
	registerConsoleCanvasTestCvars(g)
	originalFraction := g.ConsoleSlideFraction
	t.Cleanup(func() {
		g.ConsoleSlideFraction = originalFraction
	})

	g.Host.CVar.Set("scr_conspeed", "300")

	g.ConsoleSlideFraction = 0
	g.updateRuntimeConsoleSlide(0.25, true, false)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-0.25)) > 0.0001 {
		t.Fatalf("open slide fraction = %.2f, want 0.25", got)
	}

	g.updateRuntimeConsoleSlide(0.25, false, false)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-0.0)) > 0.0001 {
		t.Fatalf("close slide fraction = %.2f, want 0.00", got)
	}

	g.updateRuntimeConsoleSlide(0.25, false, true)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-1.0)) > 0.0001 {
		t.Fatalf("forced slide fraction = %.2f, want 1.00", got)
	}
}

func TestDrawChatInputClipsAndDrawsBlinkCursor(t *testing.T) {
	g := New()
	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	g.chatBuffer = "abcdef"
	g.chatTeam = false

	dc := &consoleOverlayDrawContext{}
	g.drawChatInput(dc, 80, 200)

	if len(dc.chars) != 8 {
		t.Fatalf("chat draw count = %d, want 8", len(dc.chars))
	}
	var text strings.Builder
	for i := 0; i < len(dc.chars)-1; i++ {
		text.WriteRune(rune(dc.chars[i].num))
	}
	if got := text.String(); got != "say: ef" {
		t.Fatalf("chat visible text = %q, want %q", got, "say: ef")
	}
	last := dc.chars[len(dc.chars)-1]
	if last.x != 64 || last.y != 0 || (last.num != 10 && last.num != 11) {
		t.Fatalf("chat cursor = (%d,%d,%d), want (64,0,10 or 11)", last.x, last.y, last.num)
	}
}

func TestDrawChatInputTracksNotifyRows(t *testing.T) {
	g := New()
	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	g.Host.CVar.Set("con_notifytime", "3")

	g.chatBuffer = "hi"
	console.Printf("notify")

	dc := &consoleOverlayDrawContext{}
	g.drawChatInput(dc, 80, 200)

	last := dc.chars[len(dc.chars)-1]
	if last.y != 8 {
		t.Fatalf("chat cursor y with one notify row = %d, want 8", last.y)
	}
}

func TestShouldUploadRuntimeWorld(t *testing.T) {
	g := New()
	tests := []struct {
		name         string
		uploadedKey  string
		targetKey    string
		hasWorldData bool
		want         bool
	}{
		{
			name:         "missing target map skips upload",
			uploadedKey:  "maps/start.bsp",
			targetKey:    "",
			hasWorldData: true,
			want:         false,
		},
		{
			name:         "initial upload without world data",
			targetKey:    "maps/start.bsp",
			hasWorldData: false,
			want:         true,
		},
		{
			name:         "same uploaded map reuses world data",
			uploadedKey:  "maps/start.bsp",
			targetKey:    "maps/start.bsp",
			hasWorldData: true,
			want:         false,
		},
		{
			name:         "map change forces reupload",
			uploadedKey:  "maps/start.bsp",
			targetKey:    "maps/e1m1.bsp",
			hasWorldData: true,
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.shouldUploadRuntimeWorld(tc.uploadedKey, tc.targetKey, tc.hasWorldData); got != tc.want {
				t.Fatalf("shouldUploadRuntimeWorld(%q, %q, %v) = %v, want %v", tc.uploadedKey, tc.targetKey, tc.hasWorldData, got, tc.want)
			}
		})
	}
}

func TestRegisterConsoleCompletionProvidersIncludesAliases(t *testing.T) {
	g := New()
	console.ResetCompletion()
	t.Cleanup(console.ResetCompletion)

	g.Host.Cmd.AddAlias("zz_alias_test", "echo hi\n")
	g.registerConsoleCompletionProviders()

	got, matches := console.CompleteInput("zz_al", true)
	if got != "zz_alias_test" {
		t.Fatalf("CompleteInput = %q, want %q", got, "zz_alias_test")
	}
	found := false
	for _, match := range matches {
		if match == "zz_alias_test (alias)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("matches = %v, want zz_alias_test (alias)", matches)
	}
}

func TestRegisterConsoleCompletionProvidersIncludesMapFiles(t *testing.T) {
	g := New()
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Subs = originalSubs
		console.ResetCompletion()
	})

	console.ResetCompletion()

	baseDir := t.TempDir()
	mapsDir := filepath.Join(baseDir, "id1", "maps")
	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mapsDir, "e1m1.bsp"), []byte("bsp"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, ""); err != nil {
		t.Fatalf("filesystem init failed: %v", err)
	}
	t.Cleanup(fileSys.Close)

	g.Subs = &host.Subsystems{Files: fileSys}

	g.registerConsoleCompletionProviders()

	got, matches := console.CompleteInput("map e1", true)
	if got != "map e1m1" {
		t.Fatalf("CompleteInput = %q, want %q", got, "map e1m1")
	}
	found := false
	for _, match := range matches {
		if match == "e1m1 (map)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("matches = %v, want e1m1 (map)", matches)
	}
}

func TestDrawLoadingPlaqueDrawsPlaqueAndCenteredLoadingPic(t *testing.T) {
	g := New()
	plaque := &qimage.QPic{Width: 320, Height: 20}
	loading := &qimage.QPic{Width: 160, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/qplaque.lmp": plaque,
			"gfx/loading.lmp": loading,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	g.drawLoadingPlaque(dc, pics)

	if len(dc.pics) != 0 {
		t.Fatalf("screen-space draw call count = %d, want 0", len(dc.pics))
	}
	if len(dc.menuPics) != 2 {
		t.Fatalf("menu draw call count = %d, want 2", len(dc.menuPics))
	}
	if dc.menuPics[0].x != 16 || dc.menuPics[0].y != 4 || dc.menuPics[0].pic != plaque {
		t.Fatalf("plaque draw = %+v, want x=16 y=4 plaque", dc.menuPics[0])
	}
	if dc.menuPics[1].x != 80 || dc.menuPics[1].y != 84 || dc.menuPics[1].pic != loading {
		t.Fatalf("loading draw = %+v, want centered loading pic", dc.menuPics[1])
	}
}

func TestDrawLoadingPlaqueNoopWithoutPics(t *testing.T) {
	g := New()
	dc := &loadingPlaqueDrawContext{}
	g.drawLoadingPlaque(dc, nil)
	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0", len(dc.pics), len(dc.menuPics))
	}
}

func TestDrawLoadingPlaqueNoopWithoutRenderContext(t *testing.T) {
	g := New()
	loading := &qimage.QPic{Width: 160, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/loading.lmp": loading,
		},
	}

	g.drawLoadingPlaque(nil, pics)
}

func TestDrawPauseOverlayDrawsCenteredPausePic(t *testing.T) {
	g := New()
	pause := &qimage.QPic{Width: 128, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/pause.lmp": pause,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	g.drawPauseOverlay(dc, pics)

	if len(dc.pics) != 0 {
		t.Fatalf("screen-space draw call count = %d, want 0", len(dc.pics))
	}
	if len(dc.menuPics) != 1 {
		t.Fatalf("menu draw call count = %d, want 1", len(dc.menuPics))
	}
	if dc.menuPics[0].x != 96 || dc.menuPics[0].y != 84 || dc.menuPics[0].pic != pause {
		t.Fatalf("pause draw = %+v, want x=96 y=84 pause", dc.menuPics[0])
	}
}

func TestDrawPauseOverlayNoopWithoutPics(t *testing.T) {
	g := New()
	dc := &loadingPlaqueDrawContext{}
	g.drawPauseOverlay(dc, nil)
	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0", len(dc.pics), len(dc.menuPics))
	}
}

func TestDrawPauseOverlayHonorsShowPause(t *testing.T) {
	g := New()
	g.Host.CVar.Register("showpause", "1", cvar.FlagArchive, "")
	g.Host.CVar.Set("showpause", "0")
	t.Cleanup(func() {
		g.Host.CVar.Set("showpause", "1")
	})

	pause := &qimage.QPic{Width: 128, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/pause.lmp": pause,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	g.drawPauseOverlay(dc, pics)

	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0 when showpause=0", len(dc.pics), len(dc.menuPics))
	}
}

func TestRuntimePauseActiveTracksServerClientAndDemoPause(t *testing.T) {
	g := New()
	g.Client = cl.NewClient()
	if g.runtimePauseActive() {
		t.Fatal("runtimePauseActive() = true, want false")
	}

	g.Host.SetServerPaused(true)
	if !g.runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused server, want true")
	}

	g.Host.SetServerPaused(false)
	g.Client.Paused = true
	if !g.runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused client, want true")
	}

	g.Client.Paused = false
	g.Host.SetDemoState(&cl.DemoState{Playback: true, Paused: true})
	if !g.runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused demo playback, want true")
	}
}

func TestRuntimeConsoleForcedUpUsesClientStateWhenActive(t *testing.T) {
	g := New()
	g.Client = nil
	if !g.runtimeConsoleForcedUp() {
		t.Fatal("runtimeConsoleForcedUp() = false, want true when client is nil")
	}

	g.Client = cl.NewClient()
	g.Client.State = cl.StateConnected
	g.Client.Signon = 0
	if !g.runtimeConsoleForcedUp() {
		t.Fatal("runtimeConsoleForcedUp() = false, want true while client not active")
	}

	g.Client.State = cl.StateActive
	g.Client.Signon = 0
	if g.runtimeConsoleForcedUp() {
		t.Fatal("runtimeConsoleForcedUp() = true, want false when client state is active")
	}
}
