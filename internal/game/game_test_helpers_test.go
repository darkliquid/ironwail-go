package game

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/audio"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/host"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
)

// ---------------------------------------------------------------------------
// markCurrentPredictionFresh marks the current prediction as valid.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// demoMessageClient – minimal client that returns a canned server message.
// ---------------------------------------------------------------------------

type demoMessageClient struct {
	message []byte
}

func (c *demoMessageClient) Init() error                { return nil }
func (c *demoMessageClient) Frame(float64) error        { return nil }
func (c *demoMessageClient) Shutdown()                  {}
func (c *demoMessageClient) State() host.ClientState    { return 0 }
func (c *demoMessageClient) ReadFromServer() error      { return nil }
func (c *demoMessageClient) SendCommand() error         { return nil }
func (c *demoMessageClient) SendStringCmd(string) error { return nil }
func (c *demoMessageClient) LastServerMessage() []byte  { return append([]byte(nil), c.message...) }


// ---------------------------------------------------------------------------
// processClientPhaseTestClient – counts Read/Send calls.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// activatingProcessClientTestClient – client that transitions to active.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// demoPlaybackNoopServer – no-op server for demo playback tests.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// demoPlaybackConsole – console that collects printed messages.
// ---------------------------------------------------------------------------

type demoPlaybackConsole struct{ messages []string }

func (c *demoPlaybackConsole) Init() error       { return nil }
func (c *demoPlaybackConsole) Print(msg string)  { c.messages = append(c.messages, msg) }
func (c *demoPlaybackConsole) Clear()            {}
func (c *demoPlaybackConsole) Dump(string) error { return nil }
func (c *demoPlaybackConsole) Shutdown()         {}

// ---------------------------------------------------------------------------
// demoPlaybackCommandBuffer – command buffer that records added texts.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// loadingPlaqueTestPics – pic provider backed by a map.
// ---------------------------------------------------------------------------

type loadingPlaqueTestPics struct {
	pics map[string]*qimage.QPic
}

func (p *loadingPlaqueTestPics) GetPic(name string) *qimage.QPic {
	return p.pics[name]
}

// ---------------------------------------------------------------------------
// loadingPlaqueDrawContext – draw context for loading/pause overlay tests.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// csqcDrawTestContext – draw context for CSQC draw hook tests.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// WAD/draw manager test helpers.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// consoleOverlayDrawContext – draw context for console/menu overlay tests.
// ---------------------------------------------------------------------------

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
	chars []overlayChar
}

type overlayChar struct {
	x, y, num int
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
	dc.chars = append(dc.chars, overlayChar{x: x, y: y, num: num})
}
func (dc *consoleOverlayDrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.DrawCharacter(x, y, num)
}
func (dc *consoleOverlayDrawContext) SetCanvas(ct renderer.CanvasType) { dc.canvas.Type = ct }
func (dc *consoleOverlayDrawContext) Canvas() renderer.CanvasState     { return dc.canvas }
func (dc *consoleOverlayDrawContext) SetCanvasParams(p renderer.CanvasTransformParams) {
	dc.canvasParams = append(dc.canvasParams, p)
}

// ---------------------------------------------------------------------------
// telemetryOverlayDrawContext – draw context that tracks canvas transitions.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// charsToRunes converts a slice of telemetry char calls to runes.
// ---------------------------------------------------------------------------

func charsToRunes(chars []telemetryOverlayCharCall) []rune {
	out := make([]rune, 0, len(chars))
	for _, ch := range chars {
		out = append(out, rune(ch.num))
	}
	return out
}

// ---------------------------------------------------------------------------
// overlayTestPics – simple pic provider backed by a map.
// ---------------------------------------------------------------------------

type overlayTestPics struct {
	pics map[string]*qimage.QPic
}

func (p overlayTestPics) GetPic(name string) *qimage.QPic {
	return p.pics[name]
}

// ---------------------------------------------------------------------------
// registerConsoleCanvasTestCvars registers all cvars needed by canvas tests.
// ---------------------------------------------------------------------------

func registerConsoleCanvasTestCvars(g *Game) {
	cv := g.Host.CVar
	cv.Register("vid_width", "1280", cvar.FlagArchive, "test vid width")
	cv.Register("vid_height", "720", cvar.FlagArchive, "test vid height")
	cv.Register("scr_conwidth", "0", cvar.FlagArchive, "test console width")
	cv.Register("scr_conscale", "1", cvar.FlagArchive, "test console scale")
	cv.Register("scr_menuscale", "1", cvar.FlagArchive, "test menu scale")
	cv.Register("scr_sbarscale", "1", cvar.FlagArchive, "test sbar scale")
	cv.Register("scr_crosshairscale", "1", cvar.FlagArchive, "test crosshair scale")
	cv.Register("scr_pixelaspect", "1", cvar.FlagArchive, "test pixel aspect")
	cv.Register("scr_conspeed", "300", cvar.FlagArchive, "test console slide speed")
}

// ---------------------------------------------------------------------------
// ensureViewCalcCvars registers cvars needed by viewcalc functions.
// ---------------------------------------------------------------------------

func ensureViewCalcCvars(g *Game) {
	cv := g.Host.CVar
	defaults := map[string]string{
		"cl_bob":            "0.02",
		"cl_bobcycle":       "0.6",
		"cl_bobup":          "0.5",
		"cl_rollangle":      "2.0",
		"cl_rollspeed":      "200",
		"v_idlescale":       "0",
		"v_iyaw_cycle":      "2",
		"v_iroll_cycle":     "0.5",
		"v_ipitch_cycle":    "1",
		"v_iyaw_level":      "0.3",
		"v_iroll_level":     "0.1",
		"v_ipitch_level":    "0.3",
		"r_viewmodel_quake": "0",
		"r_drawentities":    "1",
		"r_drawviewmodel":   "1",
		"chase_active":      "0",
		"scr_viewsize":      "100",
	}
	for name, def := range defaults {
		if cv.Get(name) == nil {
			cv.Register(name, def, 0, "")
		} else {
			cv.Set(name, def)
		}
	}
}

// ---------------------------------------------------------------------------
// runtimeMusicTestFS – in-memory filesystem for music/audio tests.
// ---------------------------------------------------------------------------

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
	return nil, os.ErrNotExist
}

func (fsys *runtimeMusicTestFS) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	fsys.loads++
	for _, filename := range filenames {
		if data, ok := fsys.files[filename]; ok {
			return filename, data, nil
		}
	}
	return "", nil, os.ErrNotExist
}

func (fsys *runtimeMusicTestFS) FileExists(filename string) bool {
	_, ok := fsys.files[filename]
	return ok
}

// ---------------------------------------------------------------------------
// testRuntimeMusicWAV builds a minimal WAV file for audio tests.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Sprite test helpers.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// demoBootstrapTestFS – filesystem for demo bootstrap tests.
// ---------------------------------------------------------------------------

type demoBootstrapTestFS struct{}

func (demoBootstrapTestFS) Init(baseDir, gameDir string) error { return nil }
func (demoBootstrapTestFS) Close()                             {}
func (demoBootstrapTestFS) LoadFile(filename string) ([]byte, error) {
	return nil, os.ErrNotExist
}
func (demoBootstrapTestFS) LoadFirstAvailable([]string) (string, []byte, error) {
	return "", nil, os.ErrNotExist
}
func (demoBootstrapTestFS) FileExists(string) bool { return false }

type demoBootstrapLitFS struct {
	demoBootstrapTestFS
	worldData []byte
	litData   []byte
}

func (f demoBootstrapLitFS) LoadMapBSPAndLit(worldModel string) ([]byte, []byte, error) {
	if worldModel != "maps/start.bsp" {
		return nil, nil, os.ErrNotExist
	}
	return f.worldData, f.litData, nil
}

// ---------------------------------------------------------------------------
// Unused import prevention – ensure audio is referenced.
// ---------------------------------------------------------------------------
var _ = audio.NewNullBackend
var _ = fs.NewFileSystem
