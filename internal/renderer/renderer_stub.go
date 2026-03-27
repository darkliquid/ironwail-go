//go:build !gogpu && !opengl && !cgo
// +build !gogpu,!opengl,!cgo

package renderer

import (
	"errors"
	"fmt"
	stdimage "image"
	"image/color"
	"image/png"
	"os"
	"sync"

	"github.com/ironwail/ironwail-go/internal/bsp"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
	"github.com/ironwail/ironwail-go/pkg/types"
)

var (
	ErrNoBackend = errors.New("no rendering backend available - build with -tags=gogpu or -tags=opengl")
)

type stubDrawContext struct {
	canvas CanvasState
}

func (dc *stubDrawContext) Clear(r, g, b, a float32) {}

func (dc *stubDrawContext) DrawTriangle(r, g, b, a float32) {}

func (dc *stubDrawContext) DrawPic(x, y int, pic *qimage.QPic) {}

func (dc *stubDrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {}

func (dc *stubDrawContext) DrawFill(x, y, w, h int, color byte) {}

func (dc *stubDrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {}

func (dc *stubDrawContext) DrawCharacter(x, y int, num int) {}

func (dc *stubDrawContext) DrawMenuCharacter(x, y int, num int) {}

func (dc *stubDrawContext) SurfaceView() interface{} {
	return nil
}

func (dc *stubDrawContext) Gamma() float32 {
	return 1.0
}

func (dc *stubDrawContext) SetCanvas(ct CanvasType) {
	dc.canvas.Type = ct
}

func (dc *stubDrawContext) Canvas() CanvasState {
	return dc.canvas
}

// CameraState holds the player's camera position and orientation for view setup.
type CameraState struct {
	Origin       types.Vec3
	Angles       types.Vec3
	FOV          float32
	Time         float32
	WaterwarpFOV bool
}

// RenderFrameState carries per-frame render configuration passed to RenderFrame.
type RenderFrameState struct {
	ClearColor     [4]float32
	DrawWorld      bool
	DrawEntities   bool
	BrushEntities  []BrushEntity
	AliasEntities  []AliasModelEntity
	SpriteEntities []SpriteEntity
	DecalMarks     []DecalMarkEntity
	ViewModel      *AliasModelEntity
	LightStyles    [64]float32
	FogColor       [3]float32
	FogDensity     float32
	DrawParticles  bool
	Draw2DOverlay  bool
	MenuActive     bool
	CSQCDrawHud    bool
	Particles      *ParticleSystem
	Palette        []byte

	// WaterWarp, WaterWarpTime, ForceUnderwater: see stubs_opengl.go for semantics.
	// These fields are parsed by the authoritative OpenGL path only; stub ignores them.
	WaterWarp       bool
	WaterWarpTime   float32
	ForceUnderwater bool

	// VBlend: see stubs_opengl.go for semantics.
	// Parsed by the authoritative OpenGL path only; stub ignores this field.
	VBlend [4]float32
}

// DrawContext is the no-backend draw context used by untagged builds.
type DrawContext struct {
	stub *stubDrawContext
}

// RenderFrame executes the minimal no-backend frame pipeline.
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil || !state.Draw2DOverlay || draw2DOverlay == nil {
		return
	}
	if dc.stub == nil {
		dc.stub = &stubDrawContext{}
	}
	draw2DOverlay(dc)
}

func (dc *DrawContext) Clear(r, g, b, a float32)           { dc.stubContext().Clear(r, g, b, a) }
func (dc *DrawContext) DrawTriangle(r, g, b, a float32)    { dc.stubContext().DrawTriangle(r, g, b, a) }
func (dc *DrawContext) DrawPic(x, y int, pic *qimage.QPic) { dc.stubContext().DrawPic(x, y, pic) }
func (dc *DrawContext) DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32) {
	if alpha <= 0 {
		return
	}
	dc.stubContext().DrawPic(x, y, pic)
}
func (dc *DrawContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
	dc.stubContext().DrawMenuPic(x, y, pic)
}
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.stubContext().DrawFill(x, y, w, h, color)
}
func (dc *DrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	dc.stubContext().DrawFillAlpha(x, y, w, h, color, alpha)
}
func (dc *DrawContext) DrawCharacter(x, y int, num int) { dc.stubContext().DrawCharacter(x, y, num) }
func (dc *DrawContext) DrawMenuCharacter(x, y int, num int) {
	dc.stubContext().DrawMenuCharacter(x, y, num)
}
func (dc *DrawContext) SurfaceView() interface{} { return dc.stubContext().SurfaceView() }
func (dc *DrawContext) Gamma() float32           { return dc.stubContext().Gamma() }
func (dc *DrawContext) SetCanvas(ct CanvasType)  { dc.stubContext().SetCanvas(ct) }
func (dc *DrawContext) Canvas() CanvasState      { return dc.stubContext().Canvas() }

func (dc *DrawContext) stubContext() *stubDrawContext {
	if dc.stub == nil {
		dc.stub = &stubDrawContext{}
	}
	return dc.stub
}

// DefaultRenderFrameState returns a sensible default RenderFrameState.
func DefaultRenderFrameState() *RenderFrameState {
	return &RenderFrameState{
		ClearColor:    [4]float32{0, 0, 0, 1},
		DrawWorld:     false,
		DrawEntities:  false,
		DrawParticles: false,
		Draw2DOverlay: true,
		MenuActive:    true,
	}
}

// ConvertClientStateToCamera converts predicted client state to camera state.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{
		Origin: types.NewVec3(origin[0], origin[1], origin[2]),
		Angles: types.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
	}
}

type Renderer struct {
	mu sync.RWMutex

	config Config

	aliasEntityStates   map[int]*AliasEntity
	viewModelAliasState *AliasEntity

	drawCallback   func(RenderContext)
	updateCallback func(dt float64)
	closeCallback  func()

	running bool
}

func New() (*Renderer, error) {
	return NewWithConfig(ConfigFromCvars())
}

func NewWithConfig(cfg Config) (*Renderer, error) {
	return nil, ErrNoBackend
}

func (r *Renderer) OnDraw(callback func(RenderContext)) {
	r.mu.Lock()
	r.drawCallback = callback
	r.mu.Unlock()
}

func (r *Renderer) OnUpdate(callback func(dt float64)) {
	r.mu.Lock()
	r.updateCallback = callback
	r.mu.Unlock()
}

func (r *Renderer) OnClose(callback func()) {
	r.mu.Lock()
	r.closeCallback = callback
	r.mu.Unlock()
}

func (r *Renderer) Input() interface{} {
	return nil
}

func (r *Renderer) Size() (width, height int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Width, r.config.Height
}

func (r *Renderer) ScaleFactor() float64 {
	return 1.0
}

func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

func (r *Renderer) SetConfig(cfg Config) {
	r.mu.Lock()
	r.config = cfg
	r.mu.Unlock()
}

func (r *Renderer) Run() error {
	return ErrNoBackend
}

func (r *Renderer) Stop() {}

func (r *Renderer) Shutdown() {}

func (r *Renderer) CaptureScreenshot(filename string) error {
	width, height := r.Size()
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
	fill := color.NRGBA{R: 20, G: 20, B: 46, A: 255}
	for y := 0; y < height; y++ {
		rowStart := y * img.Stride
		row := img.Pix[rowStart : rowStart+width*4]
		for x := 0; x < width; x++ {
			idx := x * 4
			row[idx+0] = fill.R
			row[idx+1] = fill.G
			row[idx+2] = fill.B
			row[idx+3] = fill.A
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("capture screenshot: create file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("capture screenshot: encode png: %w", err)
	}
	return nil
}

func (r *Renderer) SetPalette(_ []byte) {}

func (r *Renderer) SetConchars(_ []byte) {}

func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// UpdateCamera updates the cached camera state for compatibility with tagged backends.
func (r *Renderer) UpdateCamera(camera CameraState, nearPlane, farPlane float32) {}

// UploadWorld reports that no rendering backend is available.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	return ErrNoBackend
}

// ClearWorld is a no-op in the no-backend build.
func (r *Renderer) ClearWorld() {}

// HasWorldData reports whether world geometry has been uploaded.
func (r *Renderer) HasWorldData() bool {
	return false
}

func (r *Renderer) SpawnDynamicLight(light DynamicLight) bool {
	return false
}

// SpawnKeyedDynamicLight adds or replaces a keyed dynamic light (no-op in stub).
func (r *Renderer) SpawnKeyedDynamicLight(light DynamicLight) bool {
	return false
}

func (r *Renderer) UpdateLights(deltaTime float32) {}

func (r *Renderer) ClearDynamicLights() {}

func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {}

type WorldFace = worldimpl.WorldFace

// GetWorldBounds returns no bounds in the no-backend build.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	return min, max, false
}

type stubCore struct {
	mu sync.RWMutex

	cfg Config

	adapterInfo interface{}
	frameData   interface{}
	initialized bool
}

func NewCore(cfg Config) *stubCore {
	return &stubCore{cfg: cfg}
}
func (c *stubCore) InitHeadless() error {
	return ErrNoBackend
}

func (c *stubCore) Shutdown() {}

func (c *stubCore) SetupFrameData(zNear, zFar float32) error {
	return fmt.Errorf("%w: cannot setup frame data without a backend", ErrNoBackend)
}

func (c *stubCore) Initialized() bool {
	return false
}

func (c *stubCore) AdapterInfo() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.adapterInfo
}

func (c *stubCore) FrameData() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.frameData
}
