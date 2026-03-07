//go:build !gogpu && !opengl && !cgo
// +build !gogpu,!opengl,!cgo

package renderer

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gogpu/gogpu/gmath"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
)

var (
	ErrNoBackend = errors.New("no rendering backend available - build with -tags=gogpu or -tags=opengl")
)

type stubDrawContext struct{}

func (dc *stubDrawContext) Clear(r, g, b, a float32) {}

func (dc *stubDrawContext) DrawTriangle(r, g, b, a float32) {}

func (dc *stubDrawContext) DrawPic(x, y int, pic *image.QPic) {}

func (dc *stubDrawContext) DrawFill(x, y, w, h int, color byte) {}

func (dc *stubDrawContext) DrawCharacter(x, y int, num int) {}

func (dc *stubDrawContext) SurfaceView() interface{} {
	return nil
}

func (dc *stubDrawContext) Gamma() float32 {
	return 1.0
}

// CameraState holds the player's camera position and orientation for view setup.
type CameraState struct {
	Origin gmath.Vec3
	Angles gmath.Vec3
	FOV    float32
	Time   float32
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
	DrawParticles  bool
	Draw2DOverlay  bool
	MenuActive     bool
	Particles      *ParticleSystem
	Palette        []byte
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

func (dc *DrawContext) Clear(r, g, b, a float32)          { dc.stubContext().Clear(r, g, b, a) }
func (dc *DrawContext) DrawTriangle(r, g, b, a float32)   { dc.stubContext().DrawTriangle(r, g, b, a) }
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) { dc.stubContext().DrawPic(x, y, pic) }
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.stubContext().DrawFill(x, y, w, h, color)
}
func (dc *DrawContext) DrawCharacter(x, y int, num int) { dc.stubContext().DrawCharacter(x, y, num) }
func (dc *DrawContext) SurfaceView() interface{}        { return dc.stubContext().SurfaceView() }
func (dc *DrawContext) Gamma() float32                  { return dc.stubContext().Gamma() }

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
		Origin: gmath.NewVec3(origin[0], origin[1], origin[2]),
		Angles: gmath.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
	}
}

type Renderer struct {
	mu sync.RWMutex

	config Config

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

// HasWorldData reports whether world geometry has been uploaded.
func (r *Renderer) HasWorldData() bool {
	return false
}

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
