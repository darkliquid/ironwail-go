package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/bsp"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
)

type demoMessageClient struct {
	message []byte
}

func markCurrentPredictionFresh(c *cl.Client) {
	if c == nil {
		return
	}
	entNum := c.ViewEntity
	if entNum == 0 {
		if _, ok := c.Entities[1]; ok {
			entNum = 1
		}
	}
	c.PredictionValid = true
	c.PredictionEntityNum = entNum
	c.PredictionFrameTime = c.Time
}

func (c *demoMessageClient) Init() error                { return nil }
func (c *demoMessageClient) Frame(float64) error        { return nil }
func (c *demoMessageClient) Shutdown()                  {}
func (c *demoMessageClient) State() host.ClientState    { return 0 }
func (c *demoMessageClient) ReadFromServer() error      { return nil }
func (c *demoMessageClient) SendCommand() error         { return nil }
func (c *demoMessageClient) SendStringCmd(string) error { return nil }
func (c *demoMessageClient) LastServerMessage() []byte  { return append([]byte(nil), c.message...) }

type activeStateTestClient struct {
	state       host.ClientState
	clientState *cl.Client
}

func (c *activeStateTestClient) Init() error                { return nil }
func (c *activeStateTestClient) Frame(float64) error        { return nil }
func (c *activeStateTestClient) Shutdown()                  {}
func (c *activeStateTestClient) State() host.ClientState    { return c.state }
func (c *activeStateTestClient) ReadFromServer() error      { return nil }
func (c *activeStateTestClient) SendCommand() error         { return nil }
func (c *activeStateTestClient) SendStringCmd(string) error { return nil }
func (c *activeStateTestClient) ClientState() *cl.Client    { return c.clientState }

type staticTestFilesystem struct {
	files map[string]string
}

func (f *staticTestFilesystem) Init(baseDir, gameDir string) error { return nil }
func (f *staticTestFilesystem) Close()                             {}
func (f *staticTestFilesystem) LoadFile(filename string) ([]byte, error) {
	if data, ok := f.files[filename]; ok {
		return []byte(data), nil
	}
	return nil, os.ErrNotExist
}
func (f *staticTestFilesystem) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	for _, filename := range filenames {
		if data, ok := f.files[filename]; ok {
			return filename, []byte(data), nil
		}
	}
	return "", nil, os.ErrNotExist
}
func (f *staticTestFilesystem) FileExists(filename string) bool {
	_, ok := f.files[filename]
	return ok
}

type processClientPhaseTestClient struct {
	readCalls int
	sendCalls int
	state     host.ClientState
}

func (c *processClientPhaseTestClient) Init() error                { return nil }
func (c *processClientPhaseTestClient) Frame(float64) error        { return nil }
func (c *processClientPhaseTestClient) Shutdown()                  {}
func (c *processClientPhaseTestClient) State() host.ClientState    { return c.state }
func (c *processClientPhaseTestClient) ReadFromServer() error      { c.readCalls++; return nil }
func (c *processClientPhaseTestClient) SendCommand() error         { c.sendCalls++; return nil }
func (c *processClientPhaseTestClient) SendStringCmd(string) error { return nil }

type activatingProcessClientTestClient struct {
	state       host.ClientState
	clientState *cl.Client
	readCalls   int
	sendCalls   int
}

func (c *activatingProcessClientTestClient) Init() error         { return nil }
func (c *activatingProcessClientTestClient) Frame(float64) error { return nil }
func (c *activatingProcessClientTestClient) Shutdown()           {}
func (c *activatingProcessClientTestClient) State() host.ClientState {
	return c.state
}
func (c *activatingProcessClientTestClient) ReadFromServer() error {
	c.readCalls++
	if c.clientState != nil {
		c.clientState.State = cl.StateActive
		c.clientState.Signon = cl.Signons
	}
	c.state = host.ClientState(3)
	return nil
}
func (c *activatingProcessClientTestClient) SendCommand() error         { c.sendCalls++; return nil }
func (c *activatingProcessClientTestClient) SendStringCmd(string) error { return nil }
func (c *activatingProcessClientTestClient) ClientState() *cl.Client    { return c.clientState }

type demoPlaybackNoopServer struct{}

func (s *demoPlaybackNoopServer) Init(int) error                           { return nil }
func (s *demoPlaybackNoopServer) SpawnServer(string, *fs.FileSystem) error { return nil }
func (s *demoPlaybackNoopServer) ConnectClient(int)                        {}
func (s *demoPlaybackNoopServer) KillClient(int) bool                      { return false }
func (s *demoPlaybackNoopServer) KickClient(int, string, string) bool      { return false }
func (s *demoPlaybackNoopServer) Frame(float64) error                      { return nil }
func (s *demoPlaybackNoopServer) Shutdown()                                {}
func (s *demoPlaybackNoopServer) SaveSpawnParms()                          {}
func (s *demoPlaybackNoopServer) GetMaxClients() int                       { return 1 }
func (s *demoPlaybackNoopServer) IsClientActive(int) bool                  { return false }
func (s *demoPlaybackNoopServer) GetClientName(int) string                 { return "" }
func (s *demoPlaybackNoopServer) SetClientName(int, string)                {}
func (s *demoPlaybackNoopServer) GetClientColor(int) int                   { return 0 }
func (s *demoPlaybackNoopServer) SetClientColor(int, int)                  {}
func (s *demoPlaybackNoopServer) GetClientPing(int) float32                { return 0 }
func (s *demoPlaybackNoopServer) EdictNum(int) *server.Edict               { return nil }
func (s *demoPlaybackNoopServer) GetMapName() string                       { return "" }
func (s *demoPlaybackNoopServer) IsActive() bool                           { return false }
func (s *demoPlaybackNoopServer) IsPaused() bool                           { return false }
func (s *demoPlaybackNoopServer) RestoreTextSaveGameState(*server.TextSaveGameState) error {
	return nil
}
func (s *demoPlaybackNoopServer) SetLoadGame(bool)           {}
func (s *demoPlaybackNoopServer) SetPreserveSpawnParms(bool) {}

type demoPlaybackConsole struct{}

func (c *demoPlaybackConsole) Init() error       { return nil }
func (c *demoPlaybackConsole) Print(string)      {}
func (c *demoPlaybackConsole) Clear()            {}
func (c *demoPlaybackConsole) Dump(string) error { return nil }
func (c *demoPlaybackConsole) Shutdown()         {}

type demoPlaybackCommandBuffer struct {
	added    []string
	executes int
}

func (c *demoPlaybackCommandBuffer) Init()                                         {}
func (c *demoPlaybackCommandBuffer) Execute()                                      { c.executes++ }
func (c *demoPlaybackCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) { c.executes++ }
func (c *demoPlaybackCommandBuffer) AddText(text string)                           { c.added = append(c.added, text) }
func (c *demoPlaybackCommandBuffer) InsertText(string)                             {}
func (c *demoPlaybackCommandBuffer) Shutdown()                                     {}

type loadingPlaqueTestPics struct {
	pics map[string]*qimage.QPic
}

func (p *loadingPlaqueTestPics) GetPic(name string) *qimage.QPic {
	return p.pics[name]
}

type loadingPlaqueDrawCall struct {
	x   int
	y   int
	pic *qimage.QPic
}

type loadingPlaqueDrawContext struct {
	pics     []loadingPlaqueDrawCall
	menuPics []loadingPlaqueDrawCall
	canvas   renderer.CanvasState
}

func (dc *loadingPlaqueDrawContext) Clear(r, g, b, a float32)            {}
func (dc *loadingPlaqueDrawContext) DrawTriangle(r, g, b, a float32)     {}
func (dc *loadingPlaqueDrawContext) SurfaceView() interface{}            { return nil }
func (dc *loadingPlaqueDrawContext) Gamma() float32                      { return 1 }
func (dc *loadingPlaqueDrawContext) DrawFill(x, y, w, h int, color byte) {}
func (dc *loadingPlaqueDrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
}
func (dc *loadingPlaqueDrawContext) DrawCharacter(x, y int, num int)     {}
func (dc *loadingPlaqueDrawContext) DrawMenuCharacter(x, y int, num int) {}
func (dc *loadingPlaqueDrawContext) DrawPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *loadingPlaqueDrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
	dc.menuPics = append(dc.menuPics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *loadingPlaqueDrawContext) SetCanvas(ct renderer.CanvasType) {
	dc.canvas.Type = ct
}
func (dc *loadingPlaqueDrawContext) Canvas() renderer.CanvasState { return dc.canvas }

type csqcDrawFillCall struct {
	x     int
	y     int
	w     int
	h     int
	color byte
	alpha float32
}

type csqcDrawTestContext struct {
	pics   []loadingPlaqueDrawCall
	fills  []csqcDrawFillCall
	canvas renderer.CanvasState
}

func (dc *csqcDrawTestContext) Clear(r, g, b, a float32)            {}
func (dc *csqcDrawTestContext) DrawTriangle(r, g, b, a float32)     {}
func (dc *csqcDrawTestContext) SurfaceView() interface{}            { return nil }
func (dc *csqcDrawTestContext) Gamma() float32                      { return 1 }
func (dc *csqcDrawTestContext) DrawCharacter(x, y int, num int)     {}
func (dc *csqcDrawTestContext) DrawMenuCharacter(x, y int, num int) {}
func (dc *csqcDrawTestContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *csqcDrawTestContext) DrawPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, loadingPlaqueDrawCall{x: x, y: y, pic: pic})
}
func (dc *csqcDrawTestContext) DrawFill(x, y, w, h int, color byte) {
	dc.fills = append(dc.fills, csqcDrawFillCall{x: x, y: y, w: w, h: h, color: color, alpha: 1})
}
func (dc *csqcDrawTestContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	dc.fills = append(dc.fills, csqcDrawFillCall{x: x, y: y, w: w, h: h, color: color, alpha: alpha})
}
func (dc *csqcDrawTestContext) SetCanvas(ct renderer.CanvasType) {
	dc.canvas.Type = ct
}
func (dc *csqcDrawTestContext) Canvas() renderer.CanvasState { return dc.canvas }

type testWadLump struct {
	name string
	typ  qimage.LumpType
	data []byte
}

func encodeTestQPic(width, height int, pixels []byte) []byte {
	data := make([]byte, 8+len(pixels))
	binary.LittleEndian.PutUint32(data[0:4], uint32(width))
	binary.LittleEndian.PutUint32(data[4:8], uint32(height))
	copy(data[8:], pixels)
	return data
}

func writeTestGfxWad(t *testing.T, dir string, lumps []testWadLump) {
	t.Helper()

	var data bytes.Buffer
	infos := make([]qimage.LumpInfo, 0, len(lumps))
	for _, lump := range lumps {
		var name [16]byte
		copy(name[:], lump.name)
		info := qimage.LumpInfo{
			FilePos:  int32(12 + data.Len()),
			DiskSize: int32(len(lump.data)),
			Size:     int32(len(lump.data)),
			Type:     lump.typ,
			Name:     name,
		}
		if _, err := data.Write(lump.data); err != nil {
			t.Fatalf("write lump data: %v", err)
		}
		infos = append(infos, info)
	}

	header := qimage.WadHeader{
		Identification: [4]byte{'W', 'A', 'D', '2'},
		NumLumps:       int32(len(infos)),
		InfoTableOfs:   int32(12 + data.Len()),
	}

	var wad bytes.Buffer
	if err := binary.Write(&wad, binary.LittleEndian, header); err != nil {
		t.Fatalf("write wad header: %v", err)
	}
	if _, err := wad.Write(data.Bytes()); err != nil {
		t.Fatalf("write wad body: %v", err)
	}
	for _, info := range infos {
		if err := binary.Write(&wad, binary.LittleEndian, info); err != nil {
			t.Fatalf("write wad dir: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "gfx.wad"), wad.Bytes(), 0o644); err != nil {
		t.Fatalf("write gfx.wad: %v", err)
	}
}

func newTestDrawManager(t *testing.T, pics map[string]*qimage.QPic, palette []byte) *draw.Manager {
	t.Helper()

	dir := t.TempDir()
	lumps := []testWadLump{
		{name: "palette.lmp", typ: qimage.TypPalette, data: append([]byte(nil), palette...)},
	}
	for name, pic := range pics {
		lumps = append(lumps, testWadLump{
			name: name,
			typ:  qimage.TypQPic,
			data: encodeTestQPic(int(pic.Width), int(pic.Height), pic.Pixels),
		})
	}
	writeTestGfxWad(t, dir, lumps)

	mgr := draw.NewManager()
	if err := mgr.InitFromDir(dir); err != nil {
		t.Fatalf("InitFromDir failed: %v", err)
	}
	return mgr
}

type consoleOverlayDrawContext struct {
	canvas       renderer.CanvasState
	canvasParams []renderer.CanvasTransformParams
	pics         []struct {
		x, y int
		pic  *qimage.QPic
	}
	fills []struct {
		x, y, w, h int
		color      byte
		alpha      float32
	}
	chars []struct {
		x, y, num int
	}
}

func (dc *consoleOverlayDrawContext) Clear(r, g, b, a float32)        {}
func (dc *consoleOverlayDrawContext) DrawTriangle(r, g, b, a float32) {}
func (dc *consoleOverlayDrawContext) SurfaceView() interface{}        { return nil }
func (dc *consoleOverlayDrawContext) Gamma() float32                  { return 1 }
func (dc *consoleOverlayDrawContext) DrawPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, struct {
		x, y int
		pic  *qimage.QPic
	}{x, y, pic})
}
func (dc *consoleOverlayDrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {}
func (dc *consoleOverlayDrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.fills = append(dc.fills, struct {
		x, y, w, h int
		color      byte
		alpha      float32
	}{x, y, w, h, color, 1})
}
func (dc *consoleOverlayDrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	dc.fills = append(dc.fills, struct {
		x, y, w, h int
		color      byte
		alpha      float32
	}{x, y, w, h, color, alpha})
}
func (dc *consoleOverlayDrawContext) DrawCharacter(x, y int, num int) {
	dc.chars = append(dc.chars, struct {
		x, y, num int
	}{x, y, num})
}
func (dc *consoleOverlayDrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.DrawCharacter(x, y, num)
}
func (dc *consoleOverlayDrawContext) SetCanvas(ct renderer.CanvasType) { dc.canvas.Type = ct }
func (dc *consoleOverlayDrawContext) Canvas() renderer.CanvasState     { return dc.canvas }
func (dc *consoleOverlayDrawContext) SetCanvasParams(p renderer.CanvasTransformParams) {
	dc.canvasParams = append(dc.canvasParams, p)
}

type telemetryOverlayCharCall struct {
	canvas renderer.CanvasType
	x      int
	y      int
	num    int
}

type telemetryOverlayDrawContext struct {
	canvas renderer.CanvasState
	chars  []telemetryOverlayCharCall
	pics   []struct {
		canvas renderer.CanvasType
		x      int
		y      int
		pic    *qimage.QPic
	}
}

func (dc *telemetryOverlayDrawContext) Clear(r, g, b, a float32)        {}
func (dc *telemetryOverlayDrawContext) DrawTriangle(r, g, b, a float32) {}
func (dc *telemetryOverlayDrawContext) SurfaceView() interface{}        { return nil }
func (dc *telemetryOverlayDrawContext) Gamma() float32                  { return 1 }
func (dc *telemetryOverlayDrawContext) DrawPic(x, y int, pic *qimage.QPic) {
	dc.pics = append(dc.pics, struct {
		canvas renderer.CanvasType
		x      int
		y      int
		pic    *qimage.QPic
	}{canvas: dc.canvas.Type, x: x, y: y, pic: pic})
}
func (dc *telemetryOverlayDrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
}
func (dc *telemetryOverlayDrawContext) DrawFill(x, y, w, h int, color byte) {}
func (dc *telemetryOverlayDrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
}
func (dc *telemetryOverlayDrawContext) DrawCharacter(x, y int, num int) {
	dc.chars = append(dc.chars, telemetryOverlayCharCall{canvas: dc.canvas.Type, x: x, y: y, num: num})
}
func (dc *telemetryOverlayDrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.DrawCharacter(x, y, num)
}
func (dc *telemetryOverlayDrawContext) SetCanvas(ct renderer.CanvasType) {
	dc.canvas.Type = ct
	if ct == renderer.CanvasCrosshair {
		dc.canvas.Top = -100
		dc.canvas.Bottom = 100
	} else {
		dc.canvas.Top = 0
		dc.canvas.Bottom = 0
	}
}
func (dc *telemetryOverlayDrawContext) Canvas() renderer.CanvasState { return dc.canvas }

func TestDrawMenuBackdropUsesAlphaFill(t *testing.T) {
	cvar.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "")
	dc := &consoleOverlayDrawContext{}

	drawMenuBackdrop(dc, 8, 10)

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
	cvar.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "")
	dc := &consoleOverlayDrawContext{}

	cvar.Set("scr_menubgalpha", "2")
	drawMenuBackdrop(dc, 8, 10)
	cvar.Set("scr_menubgalpha", "-1")
	drawMenuBackdrop(dc, 8, 10)
	t.Cleanup(func() {
		cvar.Set("scr_menubgalpha", "0.7")
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
	registerConsoleCanvasTestCvars()
	cvar.Set("vid_width", "1280")
	cvar.Set("vid_height", "720")
	cvar.Set("scr_pixelaspect", "1")
	cvar.Set("scr_menuscale", "2.25")

	dc := &consoleOverlayDrawContext{}
	menuDrawCalled := false

	drawRuntimeMenu(dc, 16, 12, func(rc renderer.RenderContext) {
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

func registerConsoleCanvasTestCvars() {
	cvar.Register("vid_width", "1280", cvar.FlagArchive, "test vid width")
	cvar.Register("vid_height", "720", cvar.FlagArchive, "test vid height")
	cvar.Register("scr_conwidth", "0", cvar.FlagArchive, "test console width")
	cvar.Register("scr_conscale", "1", cvar.FlagArchive, "test console scale")
	cvar.Register("scr_menuscale", "1", cvar.FlagArchive, "test menu scale")
	cvar.Register("scr_sbarscale", "1", cvar.FlagArchive, "test sbar scale")
	cvar.Register("scr_crosshairscale", "1", cvar.FlagArchive, "test crosshair scale")
	cvar.Register("scr_pixelaspect", "1", cvar.FlagArchive, "test pixel aspect")
	cvar.Register("scr_conspeed", "300", cvar.FlagArchive, "test console slide speed")
}

type mouseDeltaBackend struct {
	dx         int32
	dy         int32
	x          int32
	y          int32
	mouseValid bool
}

func (b *mouseDeltaBackend) Init() error                   { return nil }
func (b *mouseDeltaBackend) Shutdown()                     {}
func (b *mouseDeltaBackend) PollEvents() bool              { return true }
func (b *mouseDeltaBackend) GetMouseDelta() (dx, dy int32) { return b.dx, b.dy }
func (b *mouseDeltaBackend) GetMousePosition() (x, y int32, valid bool) {
	return b.x, b.y, b.mouseValid
}
func (b *mouseDeltaBackend) GetModifierState() input.ModifierState  { return input.ModifierState{} }
func (b *mouseDeltaBackend) SetTextMode(input.TextMode)             {}
func (b *mouseDeltaBackend) SetCursorMode(input.CursorMode)         {}
func (b *mouseDeltaBackend) ShowKeyboard(bool)                      {}
func (b *mouseDeltaBackend) GetGamepadState(int) input.GamepadState { return input.GamepadState{} }
func (b *mouseDeltaBackend) IsGamepadConnected(int) bool            { return false }
func (b *mouseDeltaBackend) SetMouseGrab(bool)                      {}
func (b *mouseDeltaBackend) SetWindow(interface{})                  {}

func TestStartupMapArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "plus map", args: []string{"+map", "start"}, want: "start"},
		{name: "positional map", args: []string{"start"}, want: "start"},
		{name: "plus map wins", args: []string{"start", "+map", "e1m1"}, want: "e1m1"},
		{name: "no map", args: []string{"+skill", "2"}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := startupMapArg(tc.args); got != tc.want {
				t.Fatalf("startupMapArg(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestParseStartupOptions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		wantBaseDir string
		wantGameDir string
		wantMax     int
		wantPort    int
		wantDed     bool
		wantListen  bool
		wantArgs    []string
		wantErr     string
	}{
		{name: "defaults", args: []string{"+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 1, wantPort: 26000, wantArgs: []string{"+map", "start"}},
		{name: "dedicated default count", args: []string{"-dedicated", "+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 8, wantPort: 26000, wantDed: true, wantArgs: []string{"+map", "start"}},
		{name: "listen explicit count and port", args: []string{"+map", "start", "-listen", "4", "-port", "27001"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 4, wantPort: 27001, wantListen: true, wantArgs: []string{"+map", "start"}},
		{name: "basedir and game anywhere", args: []string{"+map", "e1m1", "-game", "hipnotic", "-basedir", "/tmp/quake"}, wantBaseDir: "/tmp/quake", wantGameDir: "hipnotic", wantMax: 1, wantPort: 26000, wantArgs: []string{"+map", "e1m1"}},
		{name: "dedicated and listen conflict", args: []string{"-dedicated", "-listen"}, wantErr: "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStartupOptions(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseStartupOptions(%v) error = %v, want substring %q", tc.args, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStartupOptions(%v) failed: %v", tc.args, err)
			}
			if got.BaseDir != tc.wantBaseDir || got.GameDir != tc.wantGameDir || got.MaxClients != tc.wantMax || got.Port != tc.wantPort || got.Dedicated != tc.wantDed || got.Listen != tc.wantListen || !reflect.DeepEqual(got.Args, tc.wantArgs) {
				t.Fatalf("parseStartupOptions(%v) = %+v, want base=%q game=%q max=%d port=%d dedicated=%v listen=%v args=%v", tc.args, got, tc.wantBaseDir, tc.wantGameDir, tc.wantMax, tc.wantPort, tc.wantDed, tc.wantListen, tc.wantArgs)
			}
		})
	}
}

func TestRuntimeConsoleDimensionsMatchCReferenceSizing(t *testing.T) {
	registerConsoleCanvasTestCvars()
	cvar.Set("scr_pixelaspect", "1")
	cvar.Set("scr_conwidth", "0")
	cvar.Set("scr_conscale", "2")

	if gotW, gotH := runtimeConsoleDimensions(1280, 720); gotW != 640 || gotH != 360 {
		t.Fatalf("runtimeConsoleDimensions = %dx%d, want 640x360", gotW, gotH)
	}

	cvar.Set("scr_conwidth", "200")
	cvar.Set("scr_conscale", "1")
	if gotW, gotH := runtimeConsoleDimensions(1280, 720); gotW != 320 || gotH != 180 {
		t.Fatalf("runtimeConsoleDimensions clamp = %dx%d, want 320x180", gotW, gotH)
	}
}

func TestRuntimeGUIDimensionsApplyPixelAspect(t *testing.T) {
	registerConsoleCanvasTestCvars()

	cvar.Set("vid_width", "1280")
	cvar.Set("vid_height", "720")
	cvar.Set("scr_pixelaspect", "5:6")
	if gotW, gotH := runtimeGUIDimensions(1280, 720); gotW != 1280 || gotH != 600 {
		t.Fatalf("runtimeGUIDimensions tall pixels = %dx%d, want 1280x600", gotW, gotH)
	}

	cvar.Set("scr_pixelaspect", "1.5")
	if gotW, gotH := runtimeGUIDimensions(1280, 720); gotW != 853 || gotH != 720 {
		t.Fatalf("runtimeGUIDimensions wide pixels = %dx%d, want 853x720", gotW, gotH)
	}
}

func TestDrawRuntimeConsoleUsesConsoleCanvasAndBackgroundPic(t *testing.T) {
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	registerConsoleCanvasTestCvars()
	cvar.Set("vid_width", "1280")
	cvar.Set("vid_height", "720")
	cvar.Set("scr_pixelaspect", "1")
	cvar.Set("scr_conwidth", "0")
	cvar.Set("scr_conscale", "2")

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
	drawRuntimeConsole(dc, 1864, 1428, true, false)

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
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	registerConsoleCanvasTestCvars()
	cvar.Set("vid_width", "1280")
	cvar.Set("vid_height", "720")
	cvar.Set("scr_pixelaspect", "5:6")
	cvar.Set("scr_conwidth", "0")
	cvar.Set("scr_conscale", "2")

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
	drawRuntimeConsole(dc, 1280, 720, true, false)

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
	registerConsoleCanvasTestCvars()
	cvar.Set("vid_width", "320")
	cvar.Set("vid_height", "200")
	cvar.Set("scr_pixelaspect", "1")
	cvar.Set("scr_menuscale", "1")

	params := runtimeOverlayCanvasParams(320, 200)
	transform := renderer.GetCanvasTransform(renderer.CanvasMenu, params)
	menuX, menuY := 160.75, 72.75
	ndcX := transform.Scale[0]*float32(menuX) + transform.Offset[0]
	ndcY := transform.Scale[1]*float32(menuY) + transform.Offset[1]
	screenX := int(math.Floor(float64((ndcX+1)*params.GLWidth*0.5 - 0.5)))
	screenY := int(math.Floor(float64((1-ndcY)*params.GLHeight*0.5 - 0.5)))

	gotX, gotY, ok := screenToMenuCoords(screenX, screenY)
	if !ok {
		t.Fatalf("screenToMenuCoords(%d,%d) reported outside menu", screenX, screenY)
	}
	if gotX != 160 || gotY != 72 {
		t.Fatalf("screenToMenuCoords(%d,%d) = (%d,%d), want (160,72)", screenX, screenY, gotX, gotY)
	}
}

func TestUpdateRuntimeConsoleSlide(t *testing.T) {
	registerConsoleCanvasTestCvars()
	originalFraction := g.ConsoleSlideFraction
	t.Cleanup(func() {
		g.ConsoleSlideFraction = originalFraction
	})

	cvar.Set("scr_conspeed", "300")

	g.ConsoleSlideFraction = 0
	updateRuntimeConsoleSlide(0.25, true, false)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-0.25)) > 0.0001 {
		t.Fatalf("open slide fraction = %.2f, want 0.25", got)
	}

	updateRuntimeConsoleSlide(0.25, false, false)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-0.0)) > 0.0001 {
		t.Fatalf("close slide fraction = %.2f, want 0.00", got)
	}

	updateRuntimeConsoleSlide(0.25, false, true)
	if got := g.ConsoleSlideFraction; math.Abs(float64(got-1.0)) > 0.0001 {
		t.Fatalf("forced slide fraction = %.2f, want 1.00", got)
	}
}

func TestDrawChatInputClipsAndDrawsBlinkCursor(t *testing.T) {
	originalNow := runtimeNow
	originalChatBuffer := chatBuffer
	originalChatTeam := chatTeam
	t.Cleanup(func() {
		runtimeNow = originalNow
		chatBuffer = originalChatBuffer
		chatTeam = originalChatTeam
	})
	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	runtimeNow = func() time.Time { return time.Unix(0, int64(time.Second/4)) }
	chatBuffer = "abcdef"
	chatTeam = false

	dc := &consoleOverlayDrawContext{}
	drawChatInput(dc, 80, 200)

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
	if last.x != 64 || last.y != 0 || last.num != 11 {
		t.Fatalf("chat cursor = (%d,%d,%d), want (64,0,11)", last.x, last.y, last.num)
	}
}

func TestDrawChatInputTracksNotifyRows(t *testing.T) {
	originalNow := runtimeNow
	originalChatBuffer := chatBuffer
	originalChatTeam := chatTeam
	t.Cleanup(func() {
		runtimeNow = originalNow
		chatBuffer = originalChatBuffer
		chatTeam = originalChatTeam
	})
	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	cvar.Set("con_notifytime", "3")

	runtimeNow = func() time.Time { return time.Unix(0, int64(time.Second/4)) }
	chatBuffer = "hi"
	console.Printf("notify")

	dc := &consoleOverlayDrawContext{}
	drawChatInput(dc, 80, 200)

	last := dc.chars[len(dc.chars)-1]
	if last.y != 8 {
		t.Fatalf("chat cursor y with one notify row = %d, want 8", last.y)
	}
}

func TestShouldUploadRuntimeWorld(t *testing.T) {
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
			if got := shouldUploadRuntimeWorld(tc.uploadedKey, tc.targetKey, tc.hasWorldData); got != tc.want {
				t.Fatalf("shouldUploadRuntimeWorld(%q, %q, %v) = %v, want %v", tc.uploadedKey, tc.targetKey, tc.hasWorldData, got, tc.want)
			}
		})
	}
}

func TestRegisterConsoleCompletionProvidersIncludesAliases(t *testing.T) {
	cmdsys.UnaliasAll()
	t.Cleanup(cmdsys.UnaliasAll)
	console.ResetCompletion()
	t.Cleanup(console.ResetCompletion)

	cmdsys.AddAlias("zz_alias_test", "echo hi\n")
	registerConsoleCompletionProviders()

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

func TestDrawLoadingPlaqueDrawsPlaqueAndCenteredLoadingPic(t *testing.T) {
	plaque := &qimage.QPic{Width: 320, Height: 20}
	loading := &qimage.QPic{Width: 160, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/qplaque.lmp": plaque,
			"gfx/loading.lmp": loading,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	drawLoadingPlaque(dc, pics)

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
	dc := &loadingPlaqueDrawContext{}
	drawLoadingPlaque(dc, nil)
	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0", len(dc.pics), len(dc.menuPics))
	}
}

func TestDrawPauseOverlayDrawsCenteredPausePic(t *testing.T) {
	pause := &qimage.QPic{Width: 128, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/pause.lmp": pause,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	drawPauseOverlay(dc, pics)

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
	dc := &loadingPlaqueDrawContext{}
	drawPauseOverlay(dc, nil)
	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0", len(dc.pics), len(dc.menuPics))
	}
}

func TestDrawPauseOverlayHonorsShowPause(t *testing.T) {
	cvar.Register("showpause", "1", cvar.FlagArchive, "")
	cvar.Set("showpause", "0")
	t.Cleanup(func() {
		cvar.Set("showpause", "1")
	})

	pause := &qimage.QPic{Width: 128, Height: 24}
	pics := &loadingPlaqueTestPics{
		pics: map[string]*qimage.QPic{
			"gfx/pause.lmp": pause,
		},
	}
	dc := &loadingPlaqueDrawContext{}

	drawPauseOverlay(dc, pics)

	if len(dc.pics) != 0 || len(dc.menuPics) != 0 {
		t.Fatalf("draw call counts = (%d screen, %d menu), want 0 when showpause=0", len(dc.pics), len(dc.menuPics))
	}
}

func TestRuntimePauseActiveTracksServerClientAndDemoPause(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = host.NewHost()
	g.Client = cl.NewClient()
	if runtimePauseActive() {
		t.Fatal("runtimePauseActive() = true, want false")
	}

	g.Host.SetServerPaused(true)
	if !runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused server, want true")
	}

	g.Host.SetServerPaused(false)
	g.Client.Paused = true
	if !runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused client, want true")
	}

	g.Client.Paused = false
	g.Host.SetDemoState(&cl.DemoState{Playback: true, Paused: true})
	if !runtimePauseActive() {
		t.Fatal("runtimePauseActive() = false with paused demo playback, want true")
	}
}

func TestDrawRuntimeClockAndFPSUseBottomRightCanvasForClassicHUD(t *testing.T) {
	dc := &telemetryOverlayDrawContext{}
	state := runtimeTelemetryState{
		RealTime:   1,
		FrameCount: 100,
		ViewSize:   100,
		HUDStyle:   renderer.HUDClassic,
		ShowFPS:    1,
		ShowClock:  1,
		ClientTime: 125,
	}
	fps := &runtimeFPSOverlay{}

	drawRuntimeClock(dc, state)
	drawRuntimeFPS(dc, state, fps)

	if len(dc.chars) != len("2:05")+len(" 100 fps") {
		t.Fatalf("char count = %d, want %d", len(dc.chars), len("2:05")+len(" 100 fps"))
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasBottomRight || got.x != 288 || got.y != 192 || got.num != '2' {
		t.Fatalf("clock first char = %+v, want bottom-right at 288,192 with '2'", got)
	}
	if got := dc.chars[len("2:05")]; got.canvas != renderer.CanvasBottomRight || got.x != 256 || got.y != 184 {
		t.Fatalf("fps first char = %+v, want bottom-right at 256,184", got)
	}
}

func TestDrawRuntimeSpeedUsesCrosshairCanvas(t *testing.T) {
	dc := &telemetryOverlayDrawContext{}
	state := runtimeTelemetryState{
		RealTime:     0.05,
		ViewSize:     100,
		ShowSpeed:    true,
		ShowSpeedOfs: 10,
		Velocity:     [3]float32{300, 400, 200},
	}
	speed := &runtimeSpeedOverlay{}

	drawRuntimeSpeed(dc, state, speed)
	state.RealTime = 0.10
	state.Velocity = [3]float32{}
	drawRuntimeSpeed(dc, state, speed)

	if len(dc.chars) != len("500") {
		t.Fatalf("char count = %d, want %d", len(dc.chars), len("500"))
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasCrosshair || got.x != -12 || got.y != 14 || got.num != '5' {
		t.Fatalf("speed first char = %+v, want crosshair at -12,14 with '5'", got)
	}
}

type overlayTestPics struct {
	pics map[string]*qimage.QPic
}

func (p overlayTestPics) GetPic(name string) *qimage.QPic {
	return p.pics[name]
}

func TestDrawRuntimeTurtleUsesViewportOriginAfterThreeSlowFrames(t *testing.T) {
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"turtle": {Width: 16, Height: 16}}}
	state := runtimeTelemetryState{
		ShowTurtle: true,
		FrameTime:  0.1,
		ViewRect:   renderer.ViewRect{X: 12, Y: 34, Width: 200, Height: 100},
	}
	count := 0

	drawRuntimeTurtle(dc, pics, state, &count)
	drawRuntimeTurtle(dc, pics, state, &count)
	drawRuntimeTurtle(dc, pics, state, &count)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasDefault || got.x != 12 || got.y != 34 {
		t.Fatalf("turtle draw = %+v, want default canvas at 12,34", got)
	}
}

func TestDrawRuntimeNetUsesViewportOffsetAfterLag(t *testing.T) {
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"net": {Width: 16, Height: 16}}}
	state := runtimeTelemetryState{
		RealTime:        10,
		LastServerMsgAt: 9.6,
		ClientActive:    true,
		ViewRect:        renderer.ViewRect{X: 20, Y: 40, Width: 200, Height: 100},
	}

	drawRuntimeNet(dc, pics, state)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasDefault || got.x != 84 || got.y != 40 {
		t.Fatalf("net draw = %+v, want default canvas at 84,40", got)
	}
}

func TestDrawRuntimeSavingIndicatorUsesTopRightOffset(t *testing.T) {
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"disc": {Width: 24, Height: 24}}}
	state := runtimeTelemetryState{
		SavingActive: true,
		HUDStyle:     renderer.HUDCompact,
		ViewSize:     100,
		ShowClock:    1,
		ShowFPS:      1,
	}

	drawRuntimeSavingIndicator(dc, pics, state)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasTopRight || got.x != 280 || got.y != 32 {
		t.Fatalf("saving draw = %+v, want top-right canvas at 280,32", got)
	}
}

func TestDrawRuntimeDemoControlsUsesSbarCanvas(t *testing.T) {
	cvar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	dc := &telemetryOverlayDrawContext{}
	state := runtimeTelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      1,
		DemoBaseSpeed:  1,
		DemoProgress:   0.5,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     125,
		FrameTime:      0.1,
	}
	overlay := &runtimeDemoOverlay{}

	drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) == 0 {
		t.Fatal("expected demo control characters")
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasSbar || got.x != 8 || got.y != -20 || got.num != '>' {
		t.Fatalf("first demo control char = %+v, want sbar canvas at 8,-20 with '>'", got)
	}
	foundCursor := false
	for _, ch := range dc.chars {
		if ch.canvas == renderer.CanvasSbar && ch.num == 131 {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Fatal("expected demo seek cursor character")
	}
}

func TestDrawRuntimeDemoControlsTimeoutHidesOverlay(t *testing.T) {
	cvar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	state := runtimeTelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      1,
		DemoBaseSpeed:  1,
		DemoProgress:   0.25,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     10,
		FrameTime:      0.1,
	}
	overlay := &runtimeDemoOverlay{}
	drawRuntimeDemoControls(&telemetryOverlayDrawContext{}, nil, state, overlay)

	dc := &telemetryOverlayDrawContext{}
	state.FrameTime = 1.1
	drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) != 0 || len(dc.pics) != 0 {
		t.Fatalf("expected timed-out demo overlay to draw nothing, got chars=%d pics=%d", len(dc.chars), len(dc.pics))
	}
}

func TestDrawRuntimeDemoControlsUsesMenuCanvasDuringIntermission(t *testing.T) {
	cvar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	dc := &telemetryOverlayDrawContext{}
	state := runtimeTelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      0,
		DemoBaseSpeed:  1,
		DemoProgress:   0.5,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     5,
		FrameTime:      0.1,
		Intermission:   1,
	}
	overlay := &runtimeDemoOverlay{}

	drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) == 0 {
		t.Fatal("expected intermission demo control characters")
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasMenu || got.y != 25 || got.num != 'I' {
		t.Fatalf("first intermission demo control char = %+v, want menu canvas at y=25 with 'I'", got)
	}
}

func TestBuildCSQCDrawHooksUsesNamedPicsAndScales(t *testing.T) {
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	palette := make([]byte, 768)
	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"test": &qimage.QPic{Width: 2, Height: 1, Pixels: []byte{1, 2}},
	}, palette)

	dc := &csqcDrawTestContext{}
	hooks := buildCSQCDrawHooks(dc)

	if !hooks.IsCachedPic("gfx/test.lmp") {
		t.Fatal("IsCachedPic(gfx/test.lmp) = false, want true")
	}
	if hooks.IsCachedPic("gfx/missing.lmp") {
		t.Fatal("IsCachedPic(gfx/missing.lmp) = true, want false")
	}
	if width, height := hooks.GetImageSize("gfx/test.lmp"); width != 2 || height != 1 {
		t.Fatalf("GetImageSize = (%v, %v), want (2, 1)", width, height)
	}

	hooks.DrawPic(10, 20, "gfx/test.lmp", 4, 2, 1, 1, 1, 1, 0)
	if len(dc.pics) != 1 {
		t.Fatalf("DrawPic calls = %d, want 1", len(dc.pics))
	}
	got := dc.pics[0]
	if got.x != 10 || got.y != 20 {
		t.Fatalf("DrawPic coords = (%d, %d), want (10, 20)", got.x, got.y)
	}
	if got.pic == nil {
		t.Fatal("DrawPic pic = nil, want scaled pic")
	}
	if got.pic.Width != 4 || got.pic.Height != 2 {
		t.Fatalf("DrawPic size = %dx%d, want 4x2", got.pic.Width, got.pic.Height)
	}
	wantPixels := []byte{1, 1, 2, 2, 1, 1, 2, 2}
	if !bytes.Equal(got.pic.Pixels, wantPixels) {
		t.Fatalf("DrawPic pixels = %v, want %v", got.pic.Pixels, wantPixels)
	}
}

func TestBuildCSQCDrawHooksSubPicClipAndFill(t *testing.T) {
	originalDraw := g.Draw
	t.Cleanup(func() {
		g.Draw = originalDraw
	})

	palette := make([]byte, 768)
	palette[0], palette[1], palette[2] = 0, 0, 0
	palette[3], palette[4], palette[5] = 255, 0, 0
	palette[6], palette[7], palette[8] = 0, 255, 0
	palette[9], palette[10], palette[11] = 0, 0, 255

	g.Draw = newTestDrawManager(t, map[string]*qimage.QPic{
		"clip": &qimage.QPic{
			Width:  4,
			Height: 4,
			Pixels: []byte{
				0, 1, 2, 3,
				4, 5, 6, 7,
				8, 9, 10, 11,
				12, 13, 14, 15,
			},
		},
	}, palette)

	dc := &csqcDrawTestContext{}
	hooks := buildCSQCDrawHooks(dc)

	hooks.DrawSubPic(0, 0, 2, 2, "gfx/clip.lmp", 0.25, 0.25, 0.5, 0.5, 1, 1, 1, 1, 0)
	if len(dc.pics) != 1 {
		t.Fatalf("DrawSubPic calls = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0].pic; got == nil || got.Width != 2 || got.Height != 2 || !bytes.Equal(got.Pixels, []byte{5, 6, 9, 10}) {
		t.Fatalf("DrawSubPic pic = %+v, want cropped 2x2 center", dc.pics[0].pic)
	}

	hooks.SetClipArea(1, 1, 2, 2)
	hooks.DrawPic(0, 0, "gfx/clip.lmp", 4, 4, 1, 1, 1, 1, 0)
	if len(dc.pics) != 2 {
		t.Fatalf("DrawPic with clip calls = %d, want 2 total", len(dc.pics))
	}
	clipped := dc.pics[1]
	if clipped.x != 1 || clipped.y != 1 {
		t.Fatalf("clipped DrawPic coords = (%d, %d), want (1, 1)", clipped.x, clipped.y)
	}
	if clipped.pic == nil {
		t.Fatal("clipped DrawPic pic = nil, want clipped pic")
	}
	if clipped.pic.Width != 2 || clipped.pic.Height != 2 {
		t.Fatalf("clipped DrawPic size = %dx%d, want 2x2", clipped.pic.Width, clipped.pic.Height)
	}
	if want := []byte{5, 6, 9, 10}; !bytes.Equal(clipped.pic.Pixels, want) {
		t.Fatalf("clipped DrawPic pixels = %v, want %v", clipped.pic.Pixels, want)
	}

	hooks.DrawFill(0, 0, 4, 4, 0.1, 0.9, 0.1, 1, 0)
	if len(dc.fills) != 1 {
		t.Fatalf("DrawFill calls = %d, want 1", len(dc.fills))
	}
	if got := dc.fills[0]; got.x != 1 || got.y != 1 || got.w != 2 || got.h != 2 || got.color != 2 {
		t.Fatalf("clipped DrawFill = %+v, want x=1 y=1 w=2 h=2 color=2", got)
	}

	hooks.ResetClipArea()
	hooks.DrawFill(0, 0, 4, 4, 0.1, 0.9, 0.1, 1, 0)
	if len(dc.fills) != 2 {
		t.Fatalf("DrawFill after reset calls = %d, want 2", len(dc.fills))
	}
	if got := dc.fills[1]; got.x != 0 || got.y != 0 || got.w != 4 || got.h != 4 || got.color != 2 {
		t.Fatalf("reset DrawFill = %+v, want x=0 y=0 w=4 h=4 color=2", got)
	}
}

func TestRunRuntimeFrameRunsClientPrediction(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

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

	runRuntimeFrame(0.016, gameCallbacks{})

	if got := g.Client.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
}

func TestRunRuntimeFrameSyncsAudioViewEntity(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalAudio := g.Audio
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Audio = originalAudio
	})

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

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 3 {
		t.Fatalf("audio view entity after active client frame = %d, want 3", got)
	}

	g.Client = nil
	runRuntimeFrame(0.016, gameCallbacks{})
	if got := sys.ViewEntity(); got != 0 {
		t.Fatalf("audio view entity after clearing client = %d, want 0", got)
	}
}

func TestRunRuntimeFrameUpdatesLeafAmbientAndUnderwaterAudio(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalAudio := g.Audio
	originalServer := g.Server
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Audio = originalAudio
		g.Server = originalServer
		g.Subs = originalSubs
	})

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

	runRuntimeFrame(0.1, gameCallbacks{})
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
	runRuntimeFrame(0.1, gameCallbacks{})
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
	runRuntimeFrame(0.1, gameCallbacks{})
	if sys.AmbientSound(0) != nil || sys.AmbientSound(1) != nil {
		t.Fatalf("ambient channels should clear when no world tree is available")
	}
}

func TestRunRuntimeFrameConsumesTransientEventsOnce(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = nil
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.SoundEvents = []cl.SoundEvent{{Entity: 1, Channel: 2, SoundIndex: 3}}
	g.Client.StopSoundEvents = []cl.StopSoundEvent{{Entity: 4, Channel: 5}}
	g.Client.ParticleEvents = []cl.ParticleEvent{{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 4}}
	g.Client.TempEntities = []cl.TempEntityEvent{{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}}}

	events := runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 1 || len(events.StopSoundEvents) != 1 || len(events.ParticleEvents) != 1 || len(events.TempEntities) != 1 {
		t.Fatalf("runRuntimeFrame consumed = %d sounds, %d stops, %d particles, %d temps; want 1,1,1,1", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
	if len(g.Client.SoundEvents) != 0 || len(g.Client.StopSoundEvents) != 0 || len(g.Client.ParticleEvents) != 0 || len(g.Client.TempEntities) != 0 {
		t.Fatalf("client buffers not cleared: %d sounds %d stops %d particles %d temps", len(g.Client.SoundEvents), len(g.Client.StopSoundEvents), len(g.Client.ParticleEvents), len(g.Client.TempEntities))
	}

	events = runRuntimeFrame(0.016, gameCallbacks{})
	if len(events.SoundEvents) != 0 || len(events.StopSoundEvents) != 0 || len(events.ParticleEvents) != 0 || len(events.TempEntities) != 0 {
		t.Fatalf("second frame consumed = %d sounds, %d stops, %d particles, %d temps; want 0,0,0,0", len(events.SoundEvents), len(events.StopSoundEvents), len(events.ParticleEvents), len(events.TempEntities))
	}
}

func TestRunRuntimeFrameRelinksBeforeViewAndViewModelConsumers(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalMenu := g.Menu
	originalSubs := g.Subs
	originalAliasCache := g.AliasModelCache
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Menu = originalMenu
		g.Subs = originalSubs
		g.AliasModelCache = originalAliasCache
		globalViewCalc = originalViewCalc
		cvar.Set("r_drawentities", "1")
		cvar.Set("r_drawviewmodel", "1")
		cvar.Set("chase_active", "0")
	})

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
		Origin:      [3]float32{0, 0, 0}, // stale rendered position from previous frame
		MsgOrigins:  [2][3]float32{{100, 0, 0}, {0, 0, 0}},
		MsgAngles:   [2][3]float32{{0, 0, 0}, {0, 0, 0}},
		MsgTime:     1.1,
		TrailOrigin: [3]float32{0, 0, 0},
	}

	runRuntimeFrame(0.016, gameCallbacks{})

	viewOrigin, _ := runtimeViewState()
	if want := [3]float32{50, 0, 20}; viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want relinked origin %v", viewOrigin, want)
	}

	viewModel := collectViewModelEntity()
	if viewModel == nil {
		t.Fatal("collectViewModelEntity() = nil, want viewmodel")
	}
	if viewModel.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want same relinked eye origin %v", viewModel.Origin, viewOrigin)
	}
}

func TestRuntimeViewStatePrefersAuthoritativeViewEntityOrigin(t *testing.T) {
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

	origin, angles := runtimeViewState()
	if want := [3]float32{128, 64, 62}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateDoesNotFallBackToPredictedOrigin(t *testing.T) {
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

	origin, angles := runtimeViewState()
	if want := [3]float32{0, 0, 128}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want %v", origin, want)
	}
	if angles != [3]float32{} {
		t.Fatalf("runtimeViewState angles = %v, want zero fallback angles", angles)
	}
}

func TestRuntimeViewStateUsesStaleAuthoritativeEntityInsteadOfPredictedFallback(t *testing.T) {
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

	origin, angles := runtimeViewState()
	if want := [3]float32{10, 20, 48}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want stale authoritative origin %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesPredictedXYDuringActiveMovementWhenSafe(t *testing.T) {
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

	runRuntimeFrame(0.016, gameCallbacks{})
	if got := g.Client.PredictedOrigin; got[0] <= 100 {
		t.Fatalf("expected PredictPlayers to advance predicted origin, got %#v", got)
	}
	if got := g.Client.PredictedOrigin; got[2] >= 300 {
		t.Fatalf("expected collisionless prediction to drift below authoritative Z, got %#v", got)
	}

	origin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 300 + g.Client.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeInterpolatedVelocityUsesLerpHistory(t *testing.T) {
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

	if got := runtimeInterpolatedVelocity(); got != [3]float32{160, 0, 0} {
		t.Fatalf("runtimeInterpolatedVelocity() = %v, want [160 0 0]", got)
	}
}

func TestRuntimeViewStateUsesAuthoritativeOriginWhenPredictionIsSafe(t *testing.T) {
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

	origin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 322}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeEvaluatePredictedFirstPersonXYOriginRejectsStalePrediction(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}

	decision := runtimeEvaluatePredictedFirstPersonXYOrigin([3]float32{100, 200, 300})
	if decision.OK {
		t.Fatalf("runtimeEvaluatePredictedFirstPersonXYOrigin() = %+v, want rejection for stale prediction", decision)
	}
	if decision.RejectReason != runtimeOriginRejectInvalidPrediction {
		t.Fatalf("reject reason = %s, want %s", decision.RejectReason, runtimeOriginRejectInvalidPrediction)
	}
}

func TestRuntimeViewStateUsesRelinkedAuthoritativeOriginWhenPredictionIsSafe(t *testing.T) {
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

	origin, _ := runtimeViewState()
	if want := [3]float32{95, 200, 322}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want relinked authoritative origin %v", origin, want)
	}
}

func TestRuntimeViewStateUsesAuthoritativeOriginWhenPredictionUnsafe(t *testing.T) {
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
	g.Client.PredictionError = [3]float32{runtimeMaxPredictedXYOffset + 1, 0, 0}
	g.Client.PendingCmd = cl.UserCmd{Forward: 100}

	origin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 300 + g.Client.ViewHeight}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative origin %v", origin, want)
	}
}

func TestRuntimeViewStateUsesLastServerOriginWhenViewEntityMissing(t *testing.T) {
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

	origin, angles := runtimeViewState()
	if want := [3]float32{430, 690, 24}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want last server origin %v", origin, want)
	}
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesTeleportSnappedOrigin(t *testing.T) {
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

	origin, _ := runtimeViewState()
	if want := [3]float32{512, 256, 150}; origin != want {
		t.Fatalf("runtimeViewState origin = %v, want hard-snapped origin %v", origin, want)
	}
}

func TestRuntimeViewStateKeepsViewModelAlignedWithAuthoritativeOrigin(t *testing.T) {
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

	viewOrigin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 322}; viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want authoritative eye origin %v", viewOrigin, want)
	}

	viewModel := collectViewModelEntity()
	if viewModel == nil {
		t.Fatal("collectViewModelEntity() = nil, want viewmodel")
	}
	if viewModel.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want aligned eye origin %v", viewModel.Origin, viewOrigin)
	}
}

func TestRuntimeViewStateAppliesCanonicalBobInFirstPersonPath(t *testing.T) {
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

	if bob := viewCalcBob(g.Client.Time, runtimeInterpolatedVelocity()); bob == 0 {
		t.Fatal("test setup produced zero bob, want non-zero bob input")
	} else {
		origin, _ := runtimeViewState()
		want := [3]float32{100, 200, 322 + bob}
		if origin != want {
			t.Fatalf("runtimeViewState origin = %v, want bobbed eye origin %v", origin, want)
		}
	}
}

func TestRuntimeViewStateSmoothsUpwardStepAndKeepsViewModelAligned(t *testing.T) {
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

	viewOrigin, _ := runtimeViewState()
	if want := [3]float32{100, 200, 130}; viewOrigin != want {
		t.Fatalf("runtimeViewState origin = %v, want smoothed eye origin %v", viewOrigin, want)
	}
	if got := runtimeWeaponBaseOrigin(); got != viewOrigin {
		t.Fatalf("runtimeWeaponBaseOrigin() = %v, want same smoothed eye origin %v", got, viewOrigin)
	}

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != viewOrigin {
		t.Fatalf("viewmodel origin = %v, want aligned smoothed eye origin %v", entity.Origin, viewOrigin)
	}
}

func TestCollectViewModelEntityAppliesCanonicalBobWhenPresent(t *testing.T) {
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

	if bob := viewCalcBob(g.Client.Time, runtimeInterpolatedVelocity()); bob == 0 {
		t.Fatal("test setup produced zero bob, want non-zero bob input")
	} else {
		viewOrigin, _ := runtimeViewState()
		if want := [3]float32{100, 200, 322 + bob}; viewOrigin != want {
			t.Fatalf("runtimeViewState origin = %v, want bobbed eye origin %v", viewOrigin, want)
		}
		if got := runtimeWeaponBaseOrigin(); got != [3]float32{100, 200, 322} {
			t.Fatalf("runtimeWeaponBaseOrigin() = %v, want bob-free weapon base origin [100 200 322]", got)
		}

		entity := collectViewModelEntity()
		if entity == nil {
			t.Fatal("collectViewModelEntity() = nil, want entity")
		}
		wantOrigin := [3]float32{100 + bob*0.4, 200, 322 + bob}
		if entity.Origin != wantOrigin {
			t.Fatalf("viewmodel origin = %v, want bobbed weapon origin %v", entity.Origin, wantOrigin)
		}
	}
}

func TestCollectViewModelEntityIgnoresCameraPunchAngles(t *testing.T) {
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

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
	}
	if got := entity.Angles[0]; got != -10 {
		t.Fatalf("viewmodel pitch = %v, want -10 without camera punch", got)
	}
	if got := entity.Angles[1]; got != 20 {
		t.Fatalf("viewmodel yaw = %v, want 20 without camera punch", got)
	}
}

func TestRuntimeCameraStateResetsStairSmoothingOnTeleport(t *testing.T) {
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

	camera := runtimeCameraState([3]float32{0, 0, 322}, [3]float32{0, 0, 0})
	if math.Abs(float64(camera.Origin.Z-(322+1.0/32.0))) > 0.001 {
		t.Fatalf("camera origin z = %v, want snapped z %v", camera.Origin.Z, 322+1.0/32.0)
	}
}

func TestCollectViewModelEntityResetsWeaponOffsetOnTeleport(t *testing.T) {
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

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
	}
	if entity.Origin != [3]float32{100, 200, 322} {
		t.Fatalf("viewmodel origin = %v, want hard-snapped eye origin", entity.Origin)
	}
}

func TestRuntimeCameraStateCarriesClientTime(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Time = 12.5

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{4, 5, 6})
	if camera.Time != 12.5 {
		t.Fatalf("runtimeCameraState time = %v, want 12.5", camera.Time)
	}
}

func TestRuntimeCameraStateAppliesPunchAnglesOutsideIntermission(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 100 // Alive player
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 11 || camera.Angles.Y != 18 || camera.Angles.Z != 33 {
		t.Fatalf("runtimeCameraState angles = %v, want {11 18 33}", camera.Angles)
	}
}

func TestRuntimeCameraStateSkipsPunchAnglesDuringIntermission(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Client.PunchAngle = [3]float32{1, -2, 3}

	camera := runtimeCameraState([3]float32{1, 2, 3}, [3]float32{10, 20, 30})
	if camera.Angles.X != 10 || camera.Angles.Y != 20 || camera.Angles.Z != 30 {
		t.Fatalf("runtimeCameraState angles = %v, want {10 20 30}", camera.Angles)
	}
}

func TestRuntimeViewStateUsesLiveViewAnglesWithoutInterpolation(t *testing.T) {
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

	_, angles := runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want live angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesForcedAnglesWithoutInterpolation(t *testing.T) {
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

	_, angles := runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState angles = %v, want forced angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeViewStateUsesDemoViewAnglesWithoutDoubleInterpolation(t *testing.T) {
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

	_, angles := runtimeViewState()
	if angles != g.Client.ViewAngles {
		t.Fatalf("runtimeViewState demo angles = %v, want current angles %v", angles, g.Client.ViewAngles)
	}
}

func TestRuntimeCameraStateInterpolatesPunchAngles(t *testing.T) {
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

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X < 5.9 || camera.Angles.X > 6.1 {
		t.Fatalf("runtimeCameraState punch interpolation = %v, want ~6", camera.Angles.X)
	}
}

func TestRuntimeCameraStateGunKickModeRaw(t *testing.T) {
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

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 3 || camera.Angles.Y != -2 || camera.Angles.Z != 9 {
		t.Fatalf("runtimeCameraState raw punch = %v, want {3 -2 9}", camera.Angles)
	}
}

func TestRuntimeCameraStateGunKickModeOff(t *testing.T) {
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

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	if camera.Angles.X != 1 || camera.Angles.Y != 2 || camera.Angles.Z != 3 {
		t.Fatalf("runtimeCameraState with gunkick off = %v, want {1 2 3}", camera.Angles)
	}
}

func TestRuntimeCameraStateDeadPlayerRoll(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[inet.StatHealth] = 0 // Dead player
	g.Client.Intermission = 0
	g.Client.PunchAngle = [3]float32{10, 10, 10}

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{1, 2, 3})
	// Dead players should have roll = 80 and ignore other view effects.
	if camera.Angles.Z != 80 {
		t.Fatalf("runtimeCameraState dead player roll = %v, want 80", camera.Angles.Z)
	}
}

func TestRuntimeCameraStateAppliesChaseCameraWhenActive(t *testing.T) {
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

	camera := runtimeCameraState([3]float32{0, 0, 0}, [3]float32{0, 0, 0})
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

	_, angles := runtimeViewState()
	if math.Abs(float64(angles[1]-360)) > 0.01 && math.Abs(float64(angles[1])) > 0.01 {
		t.Fatalf("runtimeViewState wrapped yaw = %v, want 0/360 short-path interpolation", angles[1])
	}
}

func TestCollectViewModelEntityAnchorsToEyeOrigin(t *testing.T) {
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

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
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

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil during intermission", entity)
	}
}

func TestCollectViewModelEntityHonorsDrawViewModelCvar(t *testing.T) {
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
	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when r_drawviewmodel=0", entity)
	}

	cvar.Set("r_drawviewmodel", "1")
	if entity := collectViewModelEntity(); entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity when r_drawviewmodel=1")
	}
}

func TestCollectViewModelEntitySuppressesWhenInvisible(t *testing.T) {
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

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when invisibility is active", entity)
	}
}

func TestCollectViewModelEntitySuppressesDuringChaseCamera(t *testing.T) {
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

	if entity := collectViewModelEntity(); entity != nil {
		t.Fatalf("collectViewModelEntity() = %#v, want nil when chase_active=1", entity)
	}
}

func TestCollectViewModelEntityAppliesPunchAndDamageKickAngles(t *testing.T) {
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

	entity := collectViewModelEntity()
	if entity == nil {
		t.Fatal("collectViewModelEntity() = nil, want entity")
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

func TestApplyDemoPlaybackViewAnglesUpdatesCurrentAndPreviousAngles(t *testing.T) {
	clientState := cl.NewClient()
	clientState.MViewAngles[0] = [3]float32{1, 2, 3}
	clientState.ViewAngles = [3]float32{4, 5, 6}

	applyDemoPlaybackViewAngles(clientState, [3]float32{10, 20, 30})

	if clientState.MViewAngles[1] != [3]float32{1, 2, 3} {
		t.Fatalf("previous demo angles = %v, want [1 2 3]", clientState.MViewAngles[1])
	}
	if clientState.MViewAngles[0] != [3]float32{10, 20, 30} {
		t.Fatalf("current demo angles = %v, want [10 20 30]", clientState.MViewAngles[0])
	}
	if clientState.ViewAngles != [3]float32{10, 20, 30} {
		t.Fatalf("view angles = %v, want [10 20 30]", clientState.ViewAngles)
	}
}

func TestDemoPlaybackReadsOneFramePerHostFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	originalClient := g.Client
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Client = originalClient
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("single_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("single_step", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil || !clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("demo flags at start = %#v, want demo playback true and timedemo false", clientState)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after second host frame = %d, want 2", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}
	if demo.Playback {
		t.Fatal("expected demo playback to stop at EOF")
	}
	if clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("demo flags after EOF = demo:%v timedemo:%v, want both false", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
}

func TestDemoPlaybackEOFQueuesNextPlaylistDemo(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("playlist_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	cmdBuf := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmdBuf,
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.SetDemoList([]string{"demo2"})
	g.Host.SetDemoNum(0)
	g.Host.CmdPlaydemo("playlist_step", g.Subs)

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}

	demo := g.Host.DemoState()
	if demo == nil {
		t.Fatal("expected demo state")
	}
	if demo.Playback {
		t.Fatal("expected playback to stop before queued playlist advance")
	}
	if len(cmdBuf.added) == 0 || cmdBuf.added[len(cmdBuf.added)-1] != "demos\n" {
		t.Fatalf("queued commands = %q, want trailing demos command", cmdBuf.added)
	}
}

func TestPausedDemoPlaybackDoesNotReadFrames(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("paused", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("paused", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.Paused = true

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}
	if demo.FrameIndex != 0 {
		t.Fatalf("frame index while paused = %d, want 0", demo.FrameIndex)
	}
}

func TestDemoPlaybackNegativeSpeedRewindsOneFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("rewind_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d: %v", i, err)
		}
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("rewind_step", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.EnableTimeDemo()

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if got := demo.FrameIndex; got != 2 {
		t.Fatalf("frame index before rewind = %d, want 2", got)
	}

	demo.SetSpeed(-1)
	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame rewind: %v", err)
	}
	if got := demo.FrameIndex; got != 1 {
		t.Fatalf("frame index after rewind = %d, want 1", got)
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame backstop: %v", err)
	}
	if got := demo.FrameIndex; got != 1 {
		t.Fatalf("frame index at rewind backstop = %d, want 1", got)
	}
	if !demo.RewindBackstop() {
		t.Fatal("expected rewind backstop after rewinding to the first frame")
	}
}

func TestDemoPlaybackWaitsForRecordedServerTime(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var frame bytes.Buffer
		frame.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&frame, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("binary.Write(time): %v", err)
		}
		frame.WriteByte(0xff)
		return frame.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timed", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.2), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("timed", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index before recorded time elapses = %d, want 1", demo.FrameIndex)
	}

	for i := 0; i < 6; i++ {
		if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
			t.Fatalf("Host.Frame catch-up %d: %v", i, err)
		}
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after recorded time elapses = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackTimeDemoIgnoresRecordedServerTime(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	originalClient := g.Client
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Client = originalClient
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var msg bytes.Buffer
		msg.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&msg, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("Write(time): %v", err)
		}
		msg.WriteByte(0xff)
		return msg.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timedemo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(2.0), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdTimedemo("timedemo", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if !clientState.DemoPlayback || !clientState.TimeDemoActive {
		t.Fatalf("timedemo flags at start = demo:%v timedemo:%v, want both true", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback || !demo.TimeDemo {
		t.Fatal("expected active timedemo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after timedemo frames = %d, want 2", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}
	if demo.Playback {
		t.Fatal("expected timedemo playback to stop at EOF")
	}
	if clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("timedemo flags after EOF = demo:%v timedemo:%v, want both false", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
}

type demoBootstrapTestFS struct{}

func (demoBootstrapTestFS) Init(baseDir, gameDir string) error { return nil }
func (demoBootstrapTestFS) Close()                             {}
func (demoBootstrapTestFS) LoadFile(filename string) ([]byte, error) {
	return nil, fmt.Errorf("unexpected LoadFile(%q)", filename)
}
func (demoBootstrapTestFS) LoadFirstAvailable([]string) (string, []byte, error) {
	return "", nil, fmt.Errorf("not implemented")
}
func (demoBootstrapTestFS) FileExists(string) bool { return false }

type demoBootstrapLitFS struct {
	demoBootstrapTestFS
	worldData []byte
	litData   []byte
}

func (f demoBootstrapLitFS) LoadMapBSPAndLit(worldModel string) ([]byte, []byte, error) {
	if worldModel != "maps/start.bsp" {
		return nil, nil, fmt.Errorf("unexpected world model %q", worldModel)
	}
	return f.worldData, f.litData, nil
}

func TestLoadWorldModelAndLitUsesOptionalLoader(t *testing.T) {
	world, lit, err := loadWorldModelAndLit(demoBootstrapLitFS{
		worldData: []byte("bsp"),
		litData:   []byte("lit"),
	}, "maps/start.bsp")
	if err != nil {
		t.Fatalf("loadWorldModelAndLit error: %v", err)
	}
	if got := string(world); got != "bsp" {
		t.Fatalf("world data = %q, want %q", got, "bsp")
	}
	if got := string(lit); got != "lit" {
		t.Fatalf("lit data = %q, want %q", got, "lit")
	}
}

func TestDemoPlaybackBootstrapsWorldAfterServerInfo(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	originalServer := g.Server
	originalClient := g.Client
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	originalLoader := loadDemoWorldTree
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Server = originalServer
		g.Client = originalClient
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
		loadDemoWorldTree = originalLoader
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	serverInfoMsg := bytes.NewBuffer(nil)
	serverInfoMsg.WriteByte(byte(inet.SVCServerInfo))
	if err := binary.Write(serverInfoMsg, binary.LittleEndian, int32(inet.PROTOCOL_FITZQUAKE)); err != nil {
		t.Fatalf("binary.Write(protocol): %v", err)
	}
	serverInfoMsg.WriteByte(1)
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteString("Demo Test")
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteString("maps/start.bsp")
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteByte(0)

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("demo_bootstrap", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(serverInfoMsg.Bytes(), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCSignOnNum), 0x02}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(signon2): %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCSignOnNum), 0x03}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(signon3): %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCTime), 0, 0, 0, 0}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(time): %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	wantTree := &bsp.Tree{Models: []bsp.DModel{{}}}
	var loadedModel string
	loadDemoWorldTree = func(files host.Filesystem, worldModel string) (*bsp.Tree, error) {
		loadedModel = worldModel
		return wantTree, nil
	}

	g.Host = host.NewHost()
	g.Server = &server.Server{}
	g.Subs = &host.Subsystems{
		Server:  &demoPlaybackNoopServer{},
		Console: &demoPlaybackConsole{},
		Files:   demoBootstrapTestFS{},
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.MouseGrabbed = false
	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()
	g.Menu.ShowMenu()
	syncGameplayInputMode()
	g.Host.CmdPlaydemo("demo_bootstrap", g.Subs)

	for i := 0; i < 4; i++ {
		if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
			t.Fatalf("Host.Frame(%d): %v", i, err)
		}
	}
	if loadedModel != "maps/start.bsp" {
		t.Fatalf("loaded model = %q, want maps/start.bsp", loadedModel)
	}
	if g.Server.ModelName != "maps/start.bsp" {
		t.Fatalf("server model name = %q, want maps/start.bsp", g.Server.ModelName)
	}
	if g.Server.WorldTree != wantTree {
		t.Fatalf("server world tree = %p, want %p", g.Server.WorldTree, wantTree)
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil || clientState.State != cl.StateActive || clientState.Signon != cl.Signons {
		t.Fatalf("client state/signon = %#v, want active/%d", clientState, cl.Signons)
	}
	if g.Menu.IsActive() {
		t.Fatal("expected startup menu to hide once demo playback became active")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after demo startup = %v, want game", got)
	}
}

func TestDemoPlaybackFlushesStuffTextSameFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	message := bytes.NewBuffer(nil)
	message.WriteByte(byte(inet.SVCStuffText))
	message.WriteString("bf\n")
	message.WriteByte(0)
	message.WriteByte(0xff)

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("stuffcmd", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(message.Bytes(), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("stuffcmd", g.Subs)

	if err := g.Host.Frame(0.016, gameCallbacks{}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if cmd.executes < 2 {
		t.Fatalf("executes = %d, want at least 2", cmd.executes)
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after same-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientFlushesLiveStuffTextSameFrame(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
	})

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	tmpDir := t.TempDir()
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.StuffCmdBuf = "bf\n"

	gameCallbacks{}.ProcessClient()

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after live-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientSendPhaseOnlySendsCommand(t *testing.T) {
	originalSubs := g.Subs
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Subs = originalSubs
		runtimeProcessClientPhase = originalPhase
	})

	client := &processClientPhaseTestClient{state: host.ClientState(3)}
	g.Subs = &host.Subsystems{Client: client}
	runtimeProcessClientPhase = "send"

	gameCallbacks{}.ProcessClient()

	if client.sendCalls != 1 || client.readCalls != 0 {
		t.Fatalf("send/read calls = %d/%d, want 1/0", client.sendCalls, client.readCalls)
	}
}

func TestProcessClientReadPhaseOnlyReadsServer(t *testing.T) {
	originalSubs := g.Subs
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Subs = originalSubs
		runtimeProcessClientPhase = originalPhase
	})

	client := &processClientPhaseTestClient{state: host.ClientState(3)}
	g.Subs = &host.Subsystems{Client: client}
	runtimeProcessClientPhase = "read"

	gameCallbacks{}.ProcessClient()

	if client.sendCalls != 0 || client.readCalls != 1 {
		t.Fatalf("send/read calls = %d/%d, want 0/1", client.sendCalls, client.readCalls)
	}
}

func TestProcessClientAppliesGameplayInputWhenClientBecomesActive(t *testing.T) {
	originalHost := g.Host
	originalSubs := g.Subs
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
		runtimeProcessClientPhase = originalPhase
	})

	clientState := cl.NewClient()
	clientState.State = cl.StateConnected
	clientState.Signon = cl.Signons - 1
	client := &activatingProcessClientTestClient{
		state:       host.ClientState(2),
		clientState: clientState,
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Client: client}
	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()
	g.Menu.ShowMenu()
	syncGameplayInputMode()
	runtimeProcessClientPhase = "read"

	gameCallbacks{}.ProcessClient()

	if client.readCalls != 1 || client.sendCalls != 0 {
		t.Fatalf("send/read calls = %d/%d, want 0/1", client.sendCalls, client.readCalls)
	}
	if g.Menu.IsActive() {
		t.Fatal("menu should hide when client becomes active during ProcessClient")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after activation = %v, want game", got)
	}
	if !g.MouseGrabbed {
		t.Fatal("mouse should be grabbed when client becomes active during ProcessClient")
	}
}

func TestRecordRuntimeDemoFrameWritesLatestServerMessage(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	g.Host = host.NewHost()
	demo := cl.NewDemoState()
	if err := demo.StartDemoRecording("runtime_demo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	t.Cleanup(func() {
		_ = demo.StopRecording()
	})
	g.Host.SetDemoState(demo)

	g.Client = cl.NewClient()
	g.Client.ViewAngles = [3]float32{10, 20, 30}
	g.Subs = &host.Subsystems{Client: &demoMessageClient{message: []byte{1, 2, 3}}}

	recordRuntimeDemoFrame()
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "demos", "runtime_demo.dem"))
	if err != nil {
		t.Fatalf("ReadFile(demo): %v", err)
	}
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 || string(data[:newline+1]) != "0\n" {
		t.Fatalf("demo header = %q, want %q", string(data), "0\\n")
	}

	reader := bytes.NewReader(data[newline+1:])
	var msgSize int32
	if err := binary.Read(reader, binary.LittleEndian, &msgSize); err != nil {
		t.Fatalf("Read(msgSize): %v", err)
	}
	if msgSize != 3 {
		t.Fatalf("msgSize = %d, want 3", msgSize)
	}
	for i, want := range [3]float32{10, 20, 30} {
		var got float32
		if err := binary.Read(reader, binary.LittleEndian, &got); err != nil {
			t.Fatalf("Read(viewAngle %d): %v", i, err)
		}
		if got != want {
			t.Fatalf("view angle %d = %v, want %v", i, got, want)
		}
	}
	frame := make([]byte, msgSize)
	if _, err := reader.Read(frame); err != nil {
		t.Fatalf("Read(frame): %v", err)
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Fatalf("frame = %v, want [1 2 3]", frame)
	}
}

func TestRuntimeAngleVectorsYawNinety(t *testing.T) {
	forward, right, up := runtimeAngleVectors([3]float32{0, 90, 0})
	if math.Abs(float64(forward[0])) > 0.0001 || math.Abs(float64(forward[1]-1)) > 0.0001 || math.Abs(float64(forward[2])) > 0.0001 {
		t.Fatalf("forward = %v, want [0 1 0]", forward)
	}
	if math.Abs(float64(right[0]-1)) > 0.0001 || math.Abs(float64(right[1])) > 0.0001 || math.Abs(float64(right[2])) > 0.0001 {
		t.Fatalf("right = %v, want [1 0 0]", right)
	}
	if math.Abs(float64(up[0])) > 0.0001 || math.Abs(float64(up[1])) > 0.0001 || math.Abs(float64(up[2]-1)) > 0.0001 {
		t.Fatalf("up = %v, want [0 0 1]", up)
	}
}

func TestRefreshRuntimeSoundCacheResetsOnPrecacheChange(t *testing.T) {
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

	refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 1 {
		t.Fatalf("same precache unexpectedly reset cache; len = %d, want 1", got)
	}

	g.Client.SoundPrecache = []string{"weapons/shotgn2.wav"}
	refreshRuntimeSoundCache()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("changed precache should reset cache; len = %d, want 0", got)
	}
}

func TestSyncRuntimeStaticSoundsTracksClientStateAndSnapshotChanges(t *testing.T) {
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

	syncRuntimeStaticSounds()
	firstKey := g.StaticSoundKey
	if firstKey == "" {
		t.Fatalf("expected static sound snapshot key to be populated")
	}

	syncRuntimeStaticSounds()
	if g.StaticSoundKey != firstKey {
		t.Fatalf("unchanged snapshot should not churn static key; got %q, want %q", g.StaticSoundKey, firstKey)
	}

	g.Client.StaticSounds = append(g.Client.StaticSounds, cl.StaticSound{
		Origin: [3]float32{40, 50, 60}, SoundIndex: 2, Volume: 200, Attenuation: 0.5,
	})
	syncRuntimeStaticSounds()
	secondKey := g.StaticSoundKey
	if secondKey == firstKey {
		t.Fatalf("static sound list change should rebuild snapshot key")
	}

	g.SoundSFXByIndex = map[int]*audio.SFX{1: nil}
	g.Client.SoundPrecache = []string{"ambience/wind2.wav"}
	syncRuntimeStaticSounds()
	if got := len(g.SoundSFXByIndex); got != 0 {
		t.Fatalf("precache change should reset runtime SFX cache before static sync; len = %d, want 0", got)
	}
	if g.StaticSoundKey == secondKey {
		t.Fatalf("precache change should rebuild static snapshot key")
	}

	g.Client.State = cl.StateConnected
	syncRuntimeStaticSounds()
	if g.StaticSoundKey != "" {
		t.Fatalf("non-active client state should clear static snapshot key, got %q", g.StaticSoundKey)
	}
}

func TestSyncRuntimeVisualEffectsEmitsParticlesAndDecals(t *testing.T) {
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
	resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ParticleEvents = []cl.ParticleEvent{
		{Origin: [3]float32{1, 2, 3}, Count: 12, Color: 99},
	}
	g.Client.TempEntities = []cl.TempEntityEvent{
		{Type: inet.TE_GUNSHOT, Origin: [3]float32{4, 5, 6}},
	}

	transientEvents := g.Client.ConsumeTransientEvents()
	syncRuntimeVisualEffects(0.1, transientEvents)

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
	resetRuntimeVisualState()
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ModelPrecache = []string{"progs/player.mdl"}
	g.Client.Entities = map[int]inet.EntityState{
		1: {ModelIndex: 1, Origin: [3]float32{4, 5, 6}, Effects: inet.EF_BRIGHTFIELD},
	}

	syncRuntimeVisualEffects(0.1, cl.TransientEvents{})

	if g.Particles == nil {
		t.Fatalf("expected runtime visual sync to keep particle system initialized")
	}
	if got := g.Particles.ActiveCount(); got != 162 {
		t.Fatalf("brightfield particle count = %d, want 162", got)
	}
}

func TestSyncRuntimeVisualEffectsResetsEffectsWhenClientInactive(t *testing.T) {
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
	resetRuntimeVisualState()
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
	syncRuntimeVisualEffects(0.1, transientEvents)

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

	entities := collectAliasEntities()
	if len(entities) != 1 {
		t.Fatalf("collectAliasEntities() len = %d, want 1", len(entities))
	}
	if got := entities[0].ModelID; got != "progs/bolt.mdl" {
		t.Fatalf("beam model = %q, want progs/bolt.mdl", got)
	}
	if got := entities[0].Origin; got != [3]float32{1, 2, 3} {
		t.Fatalf("beam origin = %v, want [1 2 3]", got)
	}
}

func TestResetRuntimeVisualStateResetsPersistentViewCalcState(t *testing.T) {
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

	resetRuntimeVisualState()

	if globalViewCalc != (viewCalcState{}) {
		t.Fatalf("globalViewCalc = %+v, want zero value", globalViewCalc)
	}
}

func TestBuildRuntimeRenderFrameStateIncludesDecalMarks(t *testing.T) {
	originalClient := g.Client
	originalMenu := g.Menu
	originalDraw := g.Draw
	originalRenderer := g.Renderer
	originalParticles := g.Particles
	originalMarks := g.DecalMarks
	t.Cleanup(func() {
		g.Client = originalClient
		g.Menu = originalMenu
		g.Draw = originalDraw
		g.Renderer = originalRenderer
		g.Particles = originalParticles
		g.DecalMarks = originalMarks
	})

	g.Renderer = &renderer.Renderer{}
	g.Client = cl.NewClient()
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

	state := buildRuntimeRenderFrameState(nil, nil, []renderer.SpriteEntity{{
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

func TestCollectSpriteEntitiesLoadsRuntimeSprites(t *testing.T) {
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

	entities := collectSpriteEntities()
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

	_ = collectSpriteEntities()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads after cached collect = %d, want 1", got)
	}
}

func TestResolveRuntimeSpriteFrameGroupTimingWraps(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
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
			if got := resolveRuntimeSpriteFrame(sprite, state, viewForward, viewRight, tc.clientTime); got != tc.want {
				t.Fatalf("resolveRuntimeSpriteFrame(time=%v) = %d, want %d", tc.clientTime, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForGroupedFrames(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
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

	if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.05); got != 1 {
		t.Fatalf("resolveRuntimeSpriteFrame(group first) = %d, want 1", got)
	}
	if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.25); got != 2 {
		t.Fatalf("resolveRuntimeSpriteFrame(group second) = %d, want 2", got)
	}
	if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 2}, viewForward, viewRight, 0.25); got != 3 {
		t.Fatalf("resolveRuntimeSpriteFrame(single after group) = %d, want 3", got)
	}
	if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1, SpriteSyncBase: 0.2}, viewForward, viewRight, 0.05); got != 2 {
		t.Fatalf("resolveRuntimeSpriteFrame(group syncbase offset) = %d, want 2", got)
	}
}

func TestResolveRuntimeSpriteFrameAngledUsesViewDirection(t *testing.T) {
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
			viewForward, viewRight, _ := runtimeAngleVectors(tc.viewAngles)
			if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 0}, viewForward, viewRight, 0.35); got != tc.want {
				t.Fatalf("resolveRuntimeSpriteFrame(view=%v) = %d, want %d", tc.viewAngles, got, tc.want)
			}
		})
	}
}

func TestResolveRuntimeSpriteFrameUsesFlatOffsetForAngledFrames(t *testing.T) {
	viewForward, viewRight, _ := runtimeAngleVectors([3]float32{})
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

	if got := resolveRuntimeSpriteFrame(sprite, inet.EntityState{Frame: 1}, viewForward, viewRight, 0.35); got != 5 {
		t.Fatalf("resolveRuntimeSpriteFrame(angled offset) = %d, want 5", got)
	}
}

func TestCollectSpriteEntitiesResolvesGroupedFrameFromClientTime(t *testing.T) {
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

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 1 {
		t.Fatalf("collectSpriteEntities grouped frame = %d, want 1", got)
	}
}

func TestCollectSpriteEntitiesKeepsSTSyncSpritesInLockstep(t *testing.T) {
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

	entities := collectSpriteEntities()
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
	if got := g.SpriteModelCache["progs/flame.spr"].model.SyncType; got != model.STSync {
		t.Fatalf("cached runtime model SyncType = %v, want %v", got, model.STSync)
	}
	if got := entities[0].SpriteData.SyncType; got != model.STSync {
		t.Fatalf("runtime sprite SyncType = %v, want %v", got, model.STSync)
	}
}

func TestCollectSpriteEntitiesAssignsAndPreservesRandomSpriteSyncBase(t *testing.T) {
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

	first := collectSpriteEntities()
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
	viewForward, viewRight, _ := runtimeAngleVectors(g.Client.ViewAngles)
	if want := resolveRuntimeSpriteFrame(entry.sprite, dynamicState, viewForward, viewRight, g.Client.Time); first[0].Frame != want {
		t.Fatalf("dynamic grouped frame = %d, want %d", first[0].Frame, want)
	}
	if want := resolveRuntimeSpriteFrame(entry.sprite, staticState, viewForward, viewRight, g.Client.Time); first[1].Frame != want {
		t.Fatalf("static grouped frame = %d, want %d", first[1].Frame, want)
	}
	if got := entry.model.SyncType; got != model.STRand {
		t.Fatalf("cached runtime model SyncType = %v, want %v", got, model.STRand)
	}

	g.Client.Time = 0.15
	second := collectSpriteEntities()
	if got := g.Client.Entities[1].SpriteSyncBase; got != dynamicState.SpriteSyncBase {
		t.Fatalf("dynamic SpriteSyncBase changed from %v to %v", dynamicState.SpriteSyncBase, got)
	}
	if got := g.Client.StaticEntities[0].SpriteSyncBase; got != staticState.SpriteSyncBase {
		t.Fatalf("static SpriteSyncBase changed from %v to %v", staticState.SpriteSyncBase, got)
	}
	if want := resolveRuntimeSpriteFrame(entry.sprite, g.Client.Entities[1], viewForward, viewRight, g.Client.Time); second[0].Frame != want {
		t.Fatalf("second dynamic grouped frame = %d, want %d", second[0].Frame, want)
	}
	if want := resolveRuntimeSpriteFrame(entry.sprite, g.Client.StaticEntities[0], viewForward, viewRight, g.Client.Time); second[1].Frame != want {
		t.Fatalf("second static grouped frame = %d, want %d", second[1].Frame, want)
	}
}

func TestCollectSpriteEntitiesResolvesAngledFrameFromViewAngles(t *testing.T) {
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

	entities := collectSpriteEntities()
	if got := len(entities); got != 1 {
		t.Fatalf("collectSpriteEntities len = %d, want 1", got)
	}
	if got := entities[0].Frame; got != 6 {
		t.Fatalf("collectSpriteEntities angled frame = %d, want 6", got)
	}
}

func TestEntityStateScaleDecodesProtocolScale(t *testing.T) {
	if got := entityStateScale(inet.EntityState{Scale: inet.ENTSCALE_DEFAULT}); got != 1 {
		t.Fatalf("entityStateScale(default) = %v, want 1", got)
	}
	if got := entityStateScale(inet.EntityState{Scale: 32}); got != 2 {
		t.Fatalf("entityStateScale(32) = %v, want 2", got)
	}
	if got := entityStateScale(inet.EntityState{}); got != 1 {
		t.Fatalf("entityStateScale(zero) = %v, want 1 fallback", got)
	}
}

func TestCollectEntityEffectSourcesKeepsAliasEffectsOnly(t *testing.T) {
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

	sources := collectEntityEffectSources()
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

	entities := collectAliasEntities()
	if got := len(entities); got != 0 {
		t.Fatalf("collectAliasEntities len = %d, want 0 for stale dynamic alias entity", got)
	}
}

func TestCollectAliasEntitiesKeepsLiveDynamicInterpolationState(t *testing.T) {
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

	entities := collectAliasEntities()
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

func TestCollectAliasEntitiesThreadsPlayerColorMap(t *testing.T) {
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

	entities := collectAliasEntities()
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

	entities := collectSpriteEntities()
	if got := len(entities); got != 0 {
		t.Fatalf("collectSpriteEntities len = %d, want 0 for stale sprite entity", got)
	}
}

func TestCollectEntityEffectSourcesSkipsStaleDynamicEntities(t *testing.T) {
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

	sources := collectEntityEffectSources()
	if got := len(sources); got != 0 {
		t.Fatalf("collectEntityEffectSources len = %d, want 0 for stale dynamic effect source", got)
	}
}

func TestCollectBrushEntitiesSkipsStaleBrushSubmodels(t *testing.T) {
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

	brushEntities := collectBrushEntities()
	if got := len(brushEntities); got != 0 {
		t.Fatalf("collectBrushEntities len = %d, want 0 for stale brush submodel", got)
	}
}

func TestCollectBrushEntitiesDecodesProtocolAlphaAndScale(t *testing.T) {
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

	brushEntities := collectBrushEntities()
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

	g.HUD = hud.NewHUD(nil)
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

	updateHUDFromServer()

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
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	g.Input = input.NewSystem(nil)
	applyDefaultGameplayBindings()

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
		if got := g.Input.GetBinding(tc.key); got != tc.want {
			t.Fatalf("binding for key %d = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestUpdateHUDFromServerKeepsIntermissionOverlayVisibleOutsideGameplayInput(t *testing.T) {
	originalHUD := g.HUD
	originalClient := g.Client
	originalInput := g.Input
	t.Cleanup(func() {
		g.HUD = originalHUD
		g.Client = originalClient
		g.Input = originalInput
	})

	g.HUD = hud.NewHUD(nil)
	g.Client = cl.NewClient()
	g.Client.Intermission = 1
	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyConsole)

	updateHUDFromServer()

	if got := g.HUD.State(); got.HideIntermissionOverlay {
		t.Fatalf("HideIntermissionOverlay = %v, want false to match C intermission flow", got.HideIntermissionOverlay)
	}
}

func TestGameplayBindCommandsAndDispatch(t *testing.T) {
	originalInput := g.Input
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Client = originalClient
	})

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client = cl.NewClient()
	registerGameplayBindCommands()

	cmdsys.ExecuteText("unbindall")
	cmdsys.ExecuteText("bind w +forward")
	cmdsys.ExecuteText("bind MWHEELUP \"impulse 12\"")

	if got := g.Input.GetBinding(int('w')); got != "+forward" {
		t.Fatalf("bind command did not set w binding, got %q", got)
	}
	if got := g.Input.GetBinding(input.KMWheelUp); got != "impulse 12" {
		t.Fatalf("bind command did not set MWHEELUP binding, got %q", got)
	}

	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("expected +forward to press InputForward")
	}
	handleGameKeyEvent(input.KeyEvent{Key: int('w'), Down: false})
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("expected -forward to release InputForward")
	}

	handleGameKeyEvent(input.KeyEvent{Key: input.KMWheelUp, Down: true})
	if g.Client.InImpulse != 12 {
		t.Fatalf("expected wheel bind to set impulse 12, got %d", g.Client.InImpulse)
	}

	cmdsys.ExecuteText("unbind w")
	if got := g.Input.GetBinding(int('w')); got != "" {
		t.Fatalf("unbind did not clear w binding, got %q", got)
	}

	cmdsys.ExecuteText("unbindall")
	if got := g.Input.GetBinding(input.KMWheelUp); got != "" {
		t.Fatalf("unbindall did not clear MWHEELUP binding, got %q", got)
	}
}

func TestScrAutoScaleUsesRendererSize(t *testing.T) {
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Renderer = originalRenderer
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	cvar.SetInt("vid_width", 640)
	cvar.SetInt("vid_height", 480)
	registerGameplayBindCommands()

	cmdsys.ExecuteText("scr_autoscale")

	for _, name := range []string{"scr_conscale", "scr_menuscale", "scr_sbarscale", "scr_crosshairscale"} {
		if got := cvar.FloatValue(name); math.Abs(got-2.25) > 0.0001 {
			t.Fatalf("%s = %v, want 2.25", name, got)
		}
	}
}

func TestScrAutoScaleFallsBackToVideoCvars(t *testing.T) {
	originalRenderer := g.Renderer
	t.Cleanup(func() {
		g.Renderer = originalRenderer
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = nil
	cvar.SetInt("vid_width", 800)
	cvar.SetInt("vid_height", 600)
	registerGameplayBindCommands()

	cmdsys.ExecuteText("scr_autoscale")

	for _, name := range []string{"scr_conscale", "scr_menuscale", "scr_sbarscale", "scr_crosshairscale"} {
		if got := cvar.FloatValue(name); math.Abs(got-1.25) > 0.0001 {
			t.Fatalf("%s = %v, want 1.25", name, got)
		}
	}
}

func TestEnsureStartupUIScaleAppliesAutoscaleWhenUnconfigured(t *testing.T) {
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	g.Host = host.NewHost()
	g.Host.SetUserDir(t.TempDir())

	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1)
	}

	ensureStartupUIScale()

	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-2.25) > 0.0001 {
			t.Fatalf("%s = %v, want 2.25", name, got)
		}
	}
}

func TestEnsureStartupUIScaleSkipsExplicitUserConfig(t *testing.T) {
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "autoexec.cfg"), []byte("scr_menuscale 1.5\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(autoexec.cfg): %v", err)
	}
	g.Host = host.NewHost()
	g.Host.SetUserDir(userDir)

	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1)
	}

	ensureStartupUIScale()

	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-1) > 0.0001 {
			t.Fatalf("%s = %v, want 1 when user config mentions UI scale", name, got)
		}
	}
}

func TestEnsureStartupUIScaleAutoscaleWhenConfigOnlyHasDefaultScaleValues(t *testing.T) {
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "ironwail.cfg"), []byte("scr_menuscale 1\nscr_conscale 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ironwail.cfg): %v", err)
	}
	g.Host = host.NewHost()
	g.Host.SetUserDir(userDir)

	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1)
	}

	ensureStartupUIScale()

	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-2.25) > 0.0001 {
			t.Fatalf("%s = %v, want 2.25", name, got)
		}
	}
}

func TestEnsureStartupUIScaleRefreshesStaleArchivedAutoScaleForLargerFramebuffer(t *testing.T) {
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "ironwail.cfg"), []byte("scr_menuscale 1.5\nscr_conscale 1.5\nscr_sbarscale 1.5\nscr_crosshairscale 1.5\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ironwail.cfg): %v", err)
	}
	g.Host = host.NewHost()
	g.Host.SetUserDir(userDir)
	cvar.SetInt("vid_width", 1280)
	cvar.SetInt("vid_height", 720)
	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1.5)
	}

	ensureStartupUIScale()

	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-2.25) > 0.0001 {
			t.Fatalf("%s = %v, want 2.25 after refreshing stale archived autoscale", name, got)
		}
	}
}

func TestEnsureStartupUIScalePreservesExplicitPinnedNonLegacyScale(t *testing.T) {
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "ironwail.cfg"), []byte("scr_menuscale 1.75\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ironwail.cfg): %v", err)
	}
	g.Host = host.NewHost()
	g.Host.SetUserDir(userDir)
	cvar.SetInt("vid_width", 1280)
	cvar.SetInt("vid_height", 720)
	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1.75)
	}

	ensureStartupUIScale()

	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-1.75) > 0.0001 {
			t.Fatalf("%s = %v, want explicit pinned scale preserved", name, got)
		}
	}
}

func TestUpdateRuntimeTextEditRepeatRepeatsConsoleBackspace(t *testing.T) {
	originalInput := g.Input
	originalRepeat := g.TextEditRepeat
	t.Cleanup(func() {
		g.Input = originalInput
		g.TextEditRepeat = originalRepeat
	})

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyConsole)
	console.SetInputLine("test")
	g.Input.OnKey = handleGameKeyEvent

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KBackspace, Down: true})
	if got := console.InputLine(); got != "tes" {
		t.Fatalf("console input after initial backspace = %q, want %q", got, "tes")
	}

	updateRuntimeTextEditRepeat(0.30)
	if got := console.InputLine(); got != "tes" {
		t.Fatalf("console input before repeat delay = %q, want unchanged", got)
	}

	updateRuntimeTextEditRepeat(0.20)
	if got := console.InputLine(); got != "te" {
		t.Fatalf("console input after repeat delay = %q, want %q", got, "te")
	}
}

func TestUpdateRuntimeTextEditRepeatRepeatsChatBackspace(t *testing.T) {
	originalInput := g.Input
	originalRepeat := g.TextEditRepeat
	originalChat := chatBuffer
	t.Cleanup(func() {
		g.Input = originalInput
		g.TextEditRepeat = originalRepeat
		chatBuffer = originalChat
	})

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyMessage)
	g.Input.OnKey = handleGameKeyEvent
	chatBuffer = "test"

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KBackspace, Down: true})
	if got := chatBuffer; got != "tes" {
		t.Fatalf("chat buffer after initial backspace = %q, want %q", got, "tes")
	}

	updateRuntimeTextEditRepeat(0.50)
	if got := chatBuffer; got != "te" {
		t.Fatalf("chat buffer after repeat delay = %q, want %q", got, "te")
	}
}

func TestGameplayDemoControlsInterceptBindingsAndUpdateSpeed(t *testing.T) {
	originalInput := g.Input
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Input = input.NewSystem(nil)
	g.Input.OnKey = handleGameKeyEvent
	g.Input.SetKeyDest(input.KeyGame)
	g.Host = host.NewHost()
	g.Host.SetDemoState(&cl.DemoState{Playback: true, Speed: 1, BaseSpeed: 1})
	g.Client = cl.NewClient()
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	demo := g.Host.DemoState()
	if demo == nil {
		t.Fatal("expected demo state")
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: false})
	if !demo.Paused || demo.Speed != 0 {
		t.Fatalf("space should pause demo, paused=%v speed=%f", demo.Paused, demo.Speed)
	}
	if g.Client.InputJump.State != 0 {
		t.Fatalf("space during demo playback should not trigger +jump, got state %d", g.Client.InputJump.State)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KUpArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KUpArrow, Down: false})
	if demo.Paused {
		t.Fatal("up arrow should resume demo playback")
	}
	if demo.BaseSpeed != 1 || demo.Speed != 1 {
		t.Fatalf("resume speed = base:%f speed:%f, want 1/1", demo.BaseSpeed, demo.Speed)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KUpArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KUpArrow, Down: false})
	if demo.BaseSpeed != 2 || demo.Speed != 2 {
		t.Fatalf("accelerated speed = base:%f speed:%f, want 2/2", demo.BaseSpeed, demo.Speed)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KShift, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KLeftArrow, Down: true})
	if demo.Speed != -2.5 {
		t.Fatalf("held left+shift speed = %f, want -2.5", demo.Speed)
	}
	if g.Client.InputLeft.State != 0 {
		t.Fatalf("left arrow during demo playback should not trigger +left, got state %d", g.Client.InputLeft.State)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KLeftArrow, Down: false})
	if demo.Speed != 0.5 {
		t.Fatalf("speed after releasing left with shift held = %f, want 0.5", demo.Speed)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KShift, Down: false})
	if demo.Speed != 2 {
		t.Fatalf("speed after releasing shift = %f, want 2", demo.Speed)
	}
}

func TestSyncGameplayInputModeClearsHeldScoreboardOutsideGameInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalShowScores := g.ShowScores
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.ShowScores = originalShowScores
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	g.Input.SetKeyDest(input.KeyGame)
	g.MouseGrabbed = false
	syncGameplayInputMode()

	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})
	if !g.ShowScores {
		t.Fatalf("+showscores should set held scoreboard state")
	}

	g.Input.SetKeyDest(input.KeyConsole)
	syncGameplayInputMode()
	if g.ShowScores {
		t.Fatalf("scoreboard hold should clear when leaving gameplay input")
	}
}

func TestStartupMenuStateSuppressesGameplayMovementInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	// initSubsystems shows the menu at startup; +map start does not close it.
	g.Menu.ShowMenu()
	syncGameplayInputMode()
	if got := g.Input.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination with startup menu active = %v, want menu", got)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("+forward should not activate while key destination is menu")
	}
	if g.Client.InputJump.State&1 != 0 {
		t.Fatalf("+jump should not activate while key destination is menu")
	}

	g.Menu.HideMenu()
	syncGameplayInputMode()
	g.Input.ClearKeyStates()

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after menu closes")
	}
	if g.Client.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after menu closes")
	}
}

func TestApplyStartupGameplayInputModeHidesMenuAndEnablesMovementInput(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	g.Menu.ShowMenu()
	syncGameplayInputMode()
	if got := g.Input.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination before startup transition = %v, want menu", got)
	}

	applyStartupGameplayInputMode()
	if g.Menu.IsActive() {
		t.Fatalf("startup transition should hide menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after startup transition = %v, want game", got)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KSpace, Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("+forward should activate after startup transition")
	}
	if g.Client.InputJump.State&1 == 0 {
		t.Fatalf("+jump should activate after startup transition")
	}
}

func TestHandleMenuKeyEventSyncsGameplayInputWhenMenuCloses(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.MouseGrabbed = false

	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent
	g.Input.OnKey = handleGameKeyEvent
	g.Input.OnChar = handleGameCharEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	g.Menu.ShowMenu()
	syncGameplayInputMode()
	if got := g.Input.GetKeyDest(); got != input.KeyMenu {
		t.Fatalf("key destination before single-player confirm = %v, want menu", got)
	}

	handleMenuKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	if !g.Menu.IsActive() || g.Menu.GetState() != menu.MenuSinglePlayer {
		t.Fatalf("main menu confirm should open the single-player menu, got active=%v state=%v", g.Menu.IsActive(), g.Menu.GetState())
	}

	handleMenuKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if g.Menu.IsActive() {
		t.Fatalf("single-player confirm should hide the menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after single-player confirm = %v, want game", got)
	}
	if !g.MouseGrabbed {
		t.Fatalf("mouse should be grabbed immediately after closing the menu")
	}
}

func TestHostInitLoadsBindingOverridesFromConfig(t *testing.T) {
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	userDir := t.TempDir()
	configPath := filepath.Join(userDir, "ironwail.cfg")
	if err := os.WriteFile(configPath, []byte("bind w +back\nbind F10 +attack\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configPath, err)
	}

	h := host.NewHost()
	subs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := h.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if got := g.Input.GetBinding(int('w')); got != "+back" {
		t.Fatalf("binding for w after config load = %q, want %q", got, "+back")
	}
	if got := g.Input.GetBinding(input.KF10); got != "+attack" {
		t.Fatalf("binding for F10 after config load = %q, want %q", got, "+attack")
	}
}

func TestEnsureGameplayBindingsReappliesDefaultsAfterStartupConfigClearsAllBinds(t *testing.T) {
	originalInput := g.Input
	originalHost := g.Host
	t.Cleanup(func() {
		g.Input = originalInput
		g.Host = originalHost
	})

	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "ironwail.cfg"), []byte("unbindall\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ironwail.cfg): %v", err)
	}

	h := host.NewHost()
	subs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := h.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	ensureGameplayBindings()

	if got := g.Input.GetBinding(int('w')); got != "+forward" {
		t.Fatalf("binding for w after startup fallback = %q, want %q", got, "+forward")
	}
	if got := g.Input.GetBinding(int('`')); got != "toggleconsole" {
		t.Fatalf("binding for ` after startup fallback = %q, want %q", got, "toggleconsole")
	}
}

func TestBuiltinDefaultCfgExecBindsConsoleAndAutoscaleUI(t *testing.T) {
	originalInput := g.Input
	originalRenderer := g.Renderer
	originalHost := g.Host
	t.Cleanup(func() {
		g.Input = originalInput
		g.Renderer = originalRenderer
		g.Host = originalHost
	})

	registerConsoleCanvasTestCvars()
	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	g.Renderer = &renderer.Renderer{}
	g.Renderer.SetConfig(renderer.Config{Width: 1920, Height: 1080})
	g.Host = host.NewHost()

	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, 1)
	}

	subs := &host.Subsystems{
		Files:    &staticTestFilesystem{files: map[string]string{}},
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	g.Host.SetUserDir(t.TempDir())
	g.Host.CmdExec([]string{"default.cfg"}, subs)

	if got := g.Input.GetBinding(int('`')); got != "toggleconsole" {
		t.Fatalf("binding for ` after builtin default.cfg exec = %q, want %q", got, "toggleconsole")
	}
	if got := g.Input.GetBinding(input.KEscape); got != "togglemenu" {
		t.Fatalf("binding for ESCAPE after builtin default.cfg exec = %q, want %q", got, "togglemenu")
	}
	for _, name := range uiScaleCVarNames {
		if got := cvar.FloatValue(name); math.Abs(got-2.25) > 0.0001 {
			t.Fatalf("%s = %v, want 2.25 after builtin default.cfg scr_autoscale", name, got)
		}
	}
}

func TestQuotedBindingsRoundTripThroughConfig(t *testing.T) {
	originalInput := g.Input
	t.Cleanup(func() {
		g.Input = originalInput
	})

	userDir := t.TempDir()
	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()

	writerHost := host.NewHost()
	writerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := writerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, writerSubs); err != nil {
		t.Fatalf("writer Init failed: %v", err)
	}

	want := "say He said \"hello\" \\world\nnext\tline"
	cmdsys.ExecuteText(`bind t "say He said \"hello\" \\world\nnext\tline"`)
	if got := g.Input.GetBinding(int('t')); got != want {
		t.Fatalf("binding before save = %q, want %q", got, want)
	}
	if err := writerHost.WriteConfig(writerSubs); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	g.Input = input.NewSystem(nil)
	registerGameplayBindCommands()
	readerHost := host.NewHost()
	readerSubs := &host.Subsystems{
		Commands: globalCommandBuffer{},
		Input:    g.Input,
	}
	if err := readerHost.Init(&host.InitParams{BaseDir: ".", UserDir: userDir}, readerSubs); err != nil {
		t.Fatalf("reader Init failed: %v", err)
	}

	if got := g.Input.GetBinding(int('t')); got != want {
		t.Fatalf("binding after reload = %q, want %q", got, want)
	}
}

func TestVidRestartCommandInvokesRestartHook(t *testing.T) {
	registerGameplayBindCommands()

	originalRestart := vidRestartFunc
	t.Cleanup(func() {
		vidRestartFunc = originalRestart
	})

	calls := 0
	vidRestartFunc = func() error {
		calls++
		return nil
	}

	cmdsys.ExecuteText("vid_restart")

	if calls != 1 {
		t.Fatalf("vid_restart call count = %d, want 1", calls)
	}
}

func TestSyncControlCvarsToClient(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	g.Client = cl.NewClient()
	syncControlCvarsToClient()

	if g.Client.AlwaysRun {
		t.Fatalf("AlwaysRun should follow cl_alwaysrun")
	}
	if g.Client.FreeLook {
		t.Fatalf("FreeLook should follow freelook")
	}
	if !g.Client.LookSpring {
		t.Fatalf("LookSpring should follow lookspring")
	}
}

func TestSyncHostClientStateReappliesControlCvarsOnClientReplacement(t *testing.T) {
	originalClient := g.Client
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Client = originalClient
		g.Subs = originalSubs
	})

	registerControlCvars()
	cvar.Set("cl_alwaysrun", "0")
	cvar.Set("freelook", "0")
	cvar.Set("lookspring", "1")

	firstClient := cl.NewClient()
	g.Subs = &host.Subsystems{
		Client: &activeStateTestClient{
			state:       host.ClientState(1),
			clientState: firstClient,
		},
	}
	syncHostClientState()

	if firstClient.AlwaysRun || firstClient.FreeLook || !firstClient.LookSpring {
		t.Fatalf("first client controls = %+v, want alwaysrun=false freelook=false lookspring=true", firstClient)
	}

	replacedClient := cl.NewClient()
	replacedClient.AlwaysRun = true
	replacedClient.FreeLook = true
	replacedClient.LookSpring = false
	g.Subs.Client = &activeStateTestClient{
		state:       host.ClientState(1),
		clientState: replacedClient,
	}
	syncHostClientState()

	if replacedClient.AlwaysRun || replacedClient.FreeLook || !replacedClient.LookSpring {
		t.Fatalf("replaced client controls = %+v, want alwaysrun=false freelook=false lookspring=true", replacedClient)
	}
}

func TestApplyGameplayMouseLookUsesControlCvars(t *testing.T) {
	originalInput := g.Input
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Client = originalClient
	})

	registerControlCvars()
	backend := &mouseDeltaBackend{}
	g.Input = input.NewSystem(backend)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client = cl.NewClient()

	cvar.Set("sensitivity", "10")
	cvar.Set("m_yaw", "0.01")
	cvar.Set("m_pitch", "0.02")
	cvar.Set("freelook", "1")

	backend.dx = 2
	backend.dy = 3
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[1]; math.Abs(float64(got-(-0.2))) > 0.0001 {
		t.Fatalf("yaw after mouse look = %.2f, want -0.20", got)
	}
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-0.6)) > 0.0001 {
		t.Fatalf("pitch after mouse look = %.2f, want 0.60", got)
	}

	g.Client.ViewAngles = [3]float32{}
	cvar.Set("freelook", "0")
	backend.dx = 0
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; got != 0 {
		t.Fatalf("pitch should stay unchanged when freelook is off and +mlook inactive, got %.2f", got)
	}
	if got := g.Client.MouseForwardMove; math.Abs(float64(got-(-50))) > 0.0001 {
		t.Fatalf("forward move with freelook off = %.2f, want -50.00", got)
	}

	g.Client.InputMLook.State = 1
	g.Client.MouseForwardMove = 0
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-1.0)) > 0.0001 {
		t.Fatalf("pitch with +mlook held = %.2f, want 1.00", got)
	}
	if got := g.Client.MouseForwardMove; got != 0 {
		t.Fatalf("forward move with +mlook held = %.2f, want 0", got)
	}

	g.Client.ViewAngles = [3]float32{}
	g.Client.InputMLook.State = 0
	cvar.Set("freelook", "1")
	cvar.Set("m_pitch", "-0.02")
	backend.dy = 5
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[0]; math.Abs(float64(got-(-1.0))) > 0.0001 {
		t.Fatalf("pitch with inverted mouse = %.2f, want -1.00", got)
	}

	g.Client.ViewAngles = [3]float32{}
	cvar.Set("m_pitch", "0.02")
	cvar.Set("freelook", "0")
	cvar.Set("lookstrafe", "1")
	g.Client.InputMLook.State = 1
	backend.dx = 2
	backend.dy = 0
	applyGameplayMouseLook()
	if got := g.Client.ViewAngles[1]; got != 0 {
		t.Fatalf("yaw with lookstrafe active = %.2f, want 0", got)
	}
	if got := g.Client.MouseSideMove; math.Abs(float64(got-16)) > 0.0001 {
		t.Fatalf("side move with lookstrafe active = %.2f, want 16.00", got)
	}
}

func TestApplyGameplayMouseLookSkipsIntermissionCutscene(t *testing.T) {
	originalInput := g.Input
	originalClient := g.Client
	t.Cleanup(func() {
		g.Input = originalInput
		g.Client = originalClient
	})

	registerControlCvars()
	backend := &mouseDeltaBackend{}
	g.Input = input.NewSystem(backend)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client = cl.NewClient()
	g.Client.FixAngle = true
	g.Client.Intermission = 1
	g.Client.ViewEntity = 1
	g.Client.ViewAngles = [3]float32{15, 25, 0}

	backend.dx = 4
	backend.dy = 5
	applyGameplayMouseLook()

	if got := g.Client.ViewAngles; got != [3]float32{15, 25, 0} {
		t.Fatalf("ViewAngles during intermission = %v, want unchanged", got)
	}
	if got := g.Client.MouseSideMove; got != 0 {
		t.Fatalf("MouseSideMove during intermission = %.2f, want 0", got)
	}
	if got := g.Client.MouseForwardMove; got != 0 {
		t.Fatalf("MouseForwardMove during intermission = %.2f, want 0", got)
	}
}

func TestApplyMenuMouseMoveUsesAbsolutePosition(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	registerConsoleCanvasTestCvars()
	cvar.Set("vid_width", "320")
	cvar.Set("vid_height", "200")
	cvar.Set("scr_pixelaspect", "1")
	cvar.Set("scr_menuscale", "1")

	backend := &mouseDeltaBackend{x: 160, y: 72, mouseValid: true}
	g.Input = input.NewSystem(backend)
	g.Input.SetKeyDest(input.KeyMenu)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()

	applyMenuMouseMove()
	if got := g.Menu.MainCursor(); got != 0 {
		t.Fatalf("first absolute menu sample should be ignored, got %d", got)
	}

	applyMenuMouseMove()
	if got := g.Menu.MainCursor(); got != 2 {
		t.Fatalf("main cursor after absolute menu move = %d, want 2", got)
	}
}

func TestApplyMenuMouseMoveFallsBackToDeltasWhenAbsoluteInvalid(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	backend := &mouseDeltaBackend{dy: 8}
	g.Input = input.NewSystem(backend)
	g.Input.SetKeyDest(input.KeyMenu)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()

	applyMenuMouseMove()
	if got := g.Menu.MainCursor(); got != 1 {
		t.Fatalf("main cursor after delta fallback = %d, want 1", got)
	}
}

func TestToggleConsoleClosesMenuAndSwitchesKeyDest(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.MouseGrabbed = true

	cmdToggleConsole(nil)

	if g.Menu.IsActive() {
		t.Fatalf("toggleconsole should hide the menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after toggleconsole = %v, want console", got)
	}
	if g.MouseGrabbed {
		t.Fatalf("console mode should release mouse grab")
	}

	cmdToggleConsole(nil)
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestMenuConsoleKeyOpensConsole(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnKey = handleGameKeyEvent
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.MouseGrabbed = true

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('`'), Down: true})

	if g.Menu.IsActive() {
		t.Fatalf("menu should be inactive after console key")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after menu console key = %v, want console", got)
	}
	if g.MouseGrabbed {
		t.Fatalf("console mode should release mouse grab")
	}
}

func TestMenuTapDownMovesCursorOnce(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after down+enter tap = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestMenuTapEscapeFromSubmenuReturnsToMain(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent

	// Enter multiplayer menu.
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: false})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: false})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after entering submenu = %v, want %v", got, menu.MenuMultiPlayer)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: false})

	if !g.Menu.IsActive() {
		t.Fatalf("menu should remain active after escape tap from submenu")
	}
	if got := g.Menu.GetState(); got != menu.MenuMain {
		t.Fatalf("menu state after escape tap = %v, want %v", got, menu.MenuMain)
	}
}

func TestMenuTapEscapeFromMainReturnsToGame(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnKey = handleGameKeyEvent

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEscape, Down: false})

	if g.Menu.IsActive() {
		t.Fatalf("menu should be inactive after escape tap from main menu")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after escape tap = %v, want %v", got, input.KeyGame)
	}
}

func TestMenuCharRoutingUpdatesSetupName(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Menu.ShowMenu()
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.OnMenuKey = handleMenuKeyEvent
	g.Input.OnMenuChar = handleMenuCharEvent

	// Enter multiplayer -> setup.
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := g.Menu.GetState(); got != menu.MenuSetup {
		t.Fatalf("menu state = %v, want %v", got, menu.MenuSetup)
	}

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // name
	g.Input.HandleCharEvent('x')
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // shirt
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // pants
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true}) // accept
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if got := g.Menu.GetState(); got != menu.MenuMultiPlayer {
		t.Fatalf("menu state after accept = %v, want %v", got, menu.MenuMultiPlayer)
	}
}

func TestConsoleKeyRoutingExecutesCommands(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.Input.SetKeyDest(input.KeyGame)
	registerGameplayBindCommands()
	applyDefaultGameplayBindings()

	var gotArgs []string
	cmdsys.AddCommand("testconsolecmd", func(args []string) {
		gotArgs = append([]string(nil), args...)
	}, "test console command")

	handleGameKeyEvent(input.KeyEvent{Key: int('`'), Down: true})
	if got := g.Input.GetKeyDest(); got != input.KeyConsole {
		t.Fatalf("key destination after console bind = %v, want console", got)
	}

	for _, ch := range "testconsolecmd 42" {
		handleGameCharEvent(ch)
	}
	if got := console.InputLine(); got != "testconsolecmd 42" {
		t.Fatalf("console input line = %q, want %q", got, "testconsolecmd 42")
	}

	handleGameKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	if len(gotArgs) != 1 || gotArgs[0] != "42" {
		t.Fatalf("console command args = %v, want [42]", gotArgs)
	}
	if got := console.InputLine(); got != "" {
		t.Fatalf("console input line after enter = %q, want empty", got)
	}

	handleGameKeyEvent(input.KeyEvent{Key: int('`'), Down: true})
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after closing console = %v, want game", got)
	}
}

func TestProcessConsoleCommandsExecutesBufferedLocalCommands(t *testing.T) {
	cmdsys.Execute()
	const commandName = "testruntimeprocessconsolecommands"
	cmdsys.RemoveCommand(commandName)
	executed := 0
	cmdsys.AddCommand(commandName, func(args []string) {
		executed++
	}, "test runtime process console commands")
	t.Cleanup(func() {
		cmdsys.RemoveCommand(commandName)
		cmdsys.Execute()
	})

	cmdsys.AddText(commandName)
	gameCallbacks{}.ProcessConsoleCommands()

	if executed != 1 {
		t.Fatalf("executed local buffered commands = %d, want 1", executed)
	}
}

func TestConsoleTabCompletionCompletesCommand(t *testing.T) {
	originalInput := g.Input
	originalMenu := g.Menu
	t.Cleanup(func() {
		g.Input = originalInput
		g.Menu = originalMenu
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()
	console.ResetCompletion()

	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	g.Input.SetKeyDest(input.KeyConsole)

	for _, ch := range "tog" {
		handleGameCharEvent(ch)
	}
	handleGameKeyEvent(input.KeyEvent{Key: input.KTab, Down: true})

	if got := console.InputLine(); got != "toggleconsole" {
		t.Fatalf("console input line after tab completion = %q, want %q", got, "toggleconsole")
	}
}

func TestHandleGameKeyEventUnboundSpecialKeyFeedback(t *testing.T) {
	originalInput := g.Input
	originalHost := g.Host
	t.Cleanup(func() {
		g.Input = originalInput
		g.Host = originalHost
		console.SetPrintCallback(nil)
	})

	if err := console.InitGlobal(0); err != nil {
		t.Fatalf("InitGlobal failed: %v", err)
	}
	console.Clear()

	g.Input = input.NewSystem(nil)
	g.Input.SetKeyDest(input.KeyGame)

	var printed strings.Builder
	console.SetPrintCallback(func(msg string) {
		printed.WriteString(msg)
	})

	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	output := printed.String()
	if !strings.Contains(output, "MOUSE4 is unbound, use Options menu to set.") {
		t.Fatalf("unbound special-key feedback = %q, missing expected hint", output)
	}

	printed.Reset()
	g.Input.SetKeyDest(input.KeyMenu)
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("menu destination should not print unbound game hint, got %q", got)
	}

	printed.Reset()
	g.Input.SetKeyDest(input.KeyGame)
	g.Host = host.NewHost()
	g.Host.SetDemoState(&cl.DemoState{Playback: true})
	handleGameKeyEvent(input.KeyEvent{Key: input.KMouse4, Down: true})
	if got := printed.String(); got != "" {
		t.Fatalf("demo playback should suppress unbound hint, got %q", got)
	}
}

func TestRuntimeMusicSelectionUsesDemoHeaderFallback(t *testing.T) {
	originalHost := g.Host
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
	})

	g.Host = host.NewHost()
	demo := cl.NewDemoState()
	demo.Playback = true
	demo.CDTrack = 5
	g.Host.SetDemoState(demo)
	g.Client = cl.NewClient()

	track, loopTrack := runtimeMusicSelection()
	if track != 5 || loopTrack != 5 {
		t.Fatalf("runtimeMusicSelection() = %d/%d, want 5/5", track, loopTrack)
	}

	g.Client.CDTrack = 2
	g.Client.LoopTrack = 3
	track, loopTrack = runtimeMusicSelection()
	if track != 2 || loopTrack != 3 {
		t.Fatalf("runtimeMusicSelection() with live client track = %d/%d, want 2/3", track, loopTrack)
	}
}

func TestRuntimeWaterwarpStateUsesRealtimeForForcedPreview(t *testing.T) {
	originalHost := g.Host
	originalMenu := g.Menu
	originalClient := g.Client
	t.Cleanup(func() {
		g.Host = originalHost
		g.Menu = originalMenu
		g.Client = originalClient
	})

	if cvar.Get(renderer.CvarRWaterwarp) == nil {
		cvar.Register(renderer.CvarRWaterwarp, "0", 0, "Underwater warp test")
	}
	cvar.Set(renderer.CvarRWaterwarp, "2")
	t.Cleanup(func() {
		cvar.Set(renderer.CvarRWaterwarp, "0")
	})

	g.Host = host.NewHost()
	g.Client = cl.NewClient()
	g.Client.Time = 12.5
	g.Menu = menu.NewManager(nil, input.NewSystem(nil))
	g.Menu.ShowMenu()
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KEnter)
	g.Menu.M_Key(input.KDownArrow)
	g.Menu.M_Key(input.KEnter)
	for i := 0; i < 6; i++ {
		g.Menu.M_Key(input.KDownArrow)
	}
	if !g.Menu.ForcedUnderwater() {
		t.Fatal("expected WATERWARP menu preview to force underwater mode")
	}

	waterWarp, waterwarpFOV, warpTime := runtimeWaterwarpState()
	if waterWarp {
		t.Fatalf("waterWarp = true, want false for r_waterwarp=2")
	}
	if !waterwarpFOV {
		t.Fatalf("waterwarpFOV = false, want true for r_waterwarp=2")
	}
	if warpTime != 0 {
		t.Fatalf("warpTime = %v, want host realtime 0 instead of client time %v", warpTime, g.Client.Time)
	}
}

func TestSyncRuntimeMusicLoadsTrackOnceAndStops(t *testing.T) {
	originalAudio := g.Audio
	originalClient := g.Client
	originalHost := g.Host
	originalSubs := g.Subs
	originalKey := g.MusicTrackKey
	t.Cleanup(func() {
		g.Audio = originalAudio
		g.Client = originalClient
		g.Host = originalHost
		g.Subs = originalSubs
		g.MusicTrackKey = originalKey
	})

	sys := &audio.System{}
	sys = audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}

	g.Audio = audio.NewAudioAdapter(sys)
	g.Client = cl.NewClient()
	g.Client.CDTrack = 2
	g.Client.LoopTrack = 2
	testFS := &runtimeMusicTestFS{
		files: map[string][]byte{
			"music/track02.wav": testRuntimeMusicWAV(t, 44100, 2, 2, 64),
		},
	}
	g.Subs = &host.Subsystems{Files: testFS}

	syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 2 {
		t.Fatalf("CurrentMusicTrack = %d, want 2", got)
	}
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want 1 after first sync", got)
	}

	syncRuntimeMusic()
	if got := testFS.loads; got != 1 {
		t.Fatalf("filesystem loads = %d, want no reload for unchanged request", got)
	}

	g.Client.CDTrack = 0
	g.Client.LoopTrack = 0
	syncRuntimeMusic()
	if got := sys.CurrentMusicTrack(); got != 0 {
		t.Fatalf("CurrentMusicTrack = %d, want 0 after stopping music", got)
	}
}

func TestApplySVolumeUsesCVarAndClamps(t *testing.T) {
	originalAudio := g.Audio
	t.Cleanup(func() {
		g.Audio = originalAudio
	})

	sys := audio.NewSystem()
	if err := sys.Init(audio.NewNullBackend(), 44100, false); err != nil {
		t.Fatalf("audio.Init failed: %v", err)
	}
	if err := sys.Startup(); err != nil {
		t.Fatalf("audio.Startup failed: %v", err)
	}
	g.Audio = audio.NewAudioAdapter(sys)

	cv := cvar.Get("s_volume")
	if cv == nil {
		cv = cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	}
	originalValue := cv.String
	originalCallback := cv.Callback
	t.Cleanup(func() {
		cv.Callback = originalCallback
		cvar.Set("s_volume", originalValue)
	})

	cvar.Set("s_volume", "0.25")
	applySVolume()
	if got := sys.Volume(); math.Abs(got-0.25) > 0.0001 {
		t.Fatalf("volume after s_volume=0.25 = %v, want 0.25", got)
	}

	cvar.Set("s_volume", "2.5")
	applySVolume()
	if got := sys.Volume(); math.Abs(got-1.0) > 0.0001 {
		t.Fatalf("volume after s_volume=2.5 = %v, want clamped 1.0", got)
	}
}

type runtimeMusicTestFS struct {
	files map[string][]byte
	loads int
}

func (fsys *runtimeMusicTestFS) Init(baseDir, gameDir string) error { return nil }
func (fsys *runtimeMusicTestFS) Close()                             {}

func (fsys *runtimeMusicTestFS) LoadFile(filename string) ([]byte, error) {
	fsys.loads++
	if data, ok := fsys.files[filename]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("missing %s", filename)
}

func (fsys *runtimeMusicTestFS) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	fsys.loads++
	for _, filename := range filenames {
		if data, ok := fsys.files[filename]; ok {
			return filename, data, nil
		}
	}
	return "", nil, fmt.Errorf("missing files: %v", filenames)
}

func (fsys *runtimeMusicTestFS) FileExists(filename string) bool {
	_, ok := fsys.files[filename]
	return ok
}

func testRuntimeMusicWAV(t *testing.T, sampleRate, channels, width, frames int) []byte {
	t.Helper()

	blockAlign := channels * width
	dataSize := frames * blockAlign
	var data bytes.Buffer
	for frame := 0; frame < frames; frame++ {
		for channel := 0; channel < channels; channel++ {
			sample := int16((frame + 1) * 128)
			if channel%2 == 1 {
				sample = -sample
			}
			if err := binary.Write(&data, binary.LittleEndian, sample); err != nil {
				t.Fatalf("binary.Write sample: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	writeString := func(value string) {
		if _, err := wav.WriteString(value); err != nil {
			t.Fatalf("WriteString(%q): %v", value, err)
		}
	}

	writeString("RIFF")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatalf("binary.Write RIFF size: %v", err)
	}
	writeString("WAVE")
	writeString("fmt ")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatalf("binary.Write fmt size: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatalf("binary.Write format: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(channels)); err != nil {
		t.Fatalf("binary.Write channels: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatalf("binary.Write sample rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*blockAlign)); err != nil {
		t.Fatalf("binary.Write byte rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(blockAlign)); err != nil {
		t.Fatalf("binary.Write block align: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(width*8)); err != nil {
		t.Fatalf("binary.Write bits: %v", err)
	}
	writeString("data")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatalf("binary.Write data size: %v", err)
	}
	if _, err := wav.Write(data.Bytes()); err != nil {
		t.Fatalf("Write data: %v", err)
	}
	return wav.Bytes()
}

func testRuntimeSprite(t *testing.T, width, height int32) []byte {
	return testRuntimeSpriteWithSyncType(t, width, height, model.STSync)
}

func testRuntimeSpriteWithSyncType(t *testing.T, width, height int32, syncType model.SyncType) []byte {
	t.Helper()

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(width))
	write(width)
	write(height)
	write(int32(1))
	write(float32(0))
	write(int32(syncType))
	write(int32(model.SpriteFrameSingle))
	write([2]int32{0, 0})
	write(width)
	write(height)
	if _, err := spr.Write([]byte{1}); err != nil {
		t.Fatalf("Write pixel data: %v", err)
	}

	return spr.Bytes()
}

func testRuntimeSpriteGroup(t *testing.T, frames int32, intervals []float32) []byte {
	return testRuntimeSpriteGroupWithSyncType(t, frames, intervals, model.STSync)
}

func testRuntimeSpriteGroupWithSyncType(t *testing.T, frames int32, intervals []float32, syncType model.SyncType) []byte {
	t.Helper()
	if frames <= 0 {
		t.Fatalf("invalid frame count: %d", frames)
	}
	if len(intervals) != int(frames) {
		t.Fatalf("interval count = %d, want %d", len(intervals), frames)
	}

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(1))
	write(int32(1))
	write(int32(1))
	write(int32(1))
	write(float32(0))
	write(int32(syncType))

	write(int32(model.SpriteFrameGroup))
	write(frames)
	for _, interval := range intervals {
		write(interval)
	}
	for i := int32(0); i < frames; i++ {
		write([2]int32{0, 0})
		write(int32(1))
		write(int32(1))
		if err := spr.WriteByte(byte(i + 1)); err != nil {
			t.Fatalf("Write pixel data: %v", err)
		}
	}

	return spr.Bytes()
}

func testRuntimeAngledSprite(t *testing.T) []byte {
	return testRuntimeAngledSpriteWithSyncType(t, model.STSync)
}

func testRuntimeAngledSpriteWithSyncType(t *testing.T, syncType model.SyncType) []byte {
	t.Helper()

	var spr bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&spr, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(model.IDSpriteHeader))
	write(int32(model.SpriteVersion))
	write(int32(0))
	write(float32(1))
	write(int32(1))
	write(int32(1))
	write(int32(1))
	write(float32(0))
	write(int32(syncType))

	write(int32(model.SpriteFrameAngled))
	write(int32(8))
	for i := 0; i < 8; i++ {
		write(float32(i+1) * 0.1)
	}
	for i := 0; i < 8; i++ {
		write([2]int32{0, 0})
		write(int32(1))
		write(int32(1))
		if err := spr.WriteByte(byte(i + 1)); err != nil {
			t.Fatalf("Write pixel data: %v", err)
		}
	}

	return spr.Bytes()
}
