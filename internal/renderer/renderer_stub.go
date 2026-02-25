//go:build !gogpu && !opengl
// +build !gogpu,!opengl

package renderer

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNoBackend = errors.New("no rendering backend available - build with -tags=gogpu or -tags=opengl")
)

type stubDrawContext struct{}

func (dc *stubDrawContext) Clear(r, g, b, a float32) {}

func (dc *stubDrawContext) DrawTriangle(r, g, b, a float32) {}

func (dc *stubDrawContext) SurfaceView() interface{} {
	return nil
}

func (dc *stubDrawContext) Gamma() float32 {
	return 1.0
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

func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

type stubCore struct {
	mu sync.RWMutex

	cfg CoreConfig

	adapterInfo interface{}
	frameData   interface{}
	initialized bool
}

func NewCore(cfg CoreConfig) *stubCore {
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
