//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/go-gl/gl/v4.1-core/gl"
)

var (
	ErrCoreUnsupportedBackend = errors.New("OpenGL core only supports OpenGL backend")
	ErrCoreNoAdapters         = errors.New("no compatible GPU adapters found")
	ErrCoreNotInitialized     = errors.New("renderer core is not initialized")
)

type CoreConfig struct {
	EnableValidation bool
}

func DefaultCoreConfig() CoreConfig {
	return CoreConfig{
		EnableValidation: true,
	}
}

type FrameData struct {
	ViewProj   [16]float32
	ZLogScale  float32
	ZLogBias   float32
	FrameCount int
}

type AdapterInfo struct {
	Name     string
	Vendor   string
	Renderer string
	Version  string
}

type glCore struct {
	mu sync.RWMutex

	cfg         CoreConfig
	initialized bool

	adapterInfo AdapterInfo
	frameData   FrameData
}

func NewCore(cfg CoreConfig) *glCore {
	return &glCore{cfg: cfg}
}
func (c *glCore) InitHeadless() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Initialize OpenGL for headless rendering
	// Note: This is a simplified implementation. For production use,
	// you would need to use EGL or a similar headless GL context.
	// For now, we'll initialize with whatever context is available.

	if gl.Init() != nil {
		// If gl.Init() fails, it might be because we don't have an active context
		// In a real headless implementation, you would create an EGL context here
		return ErrCoreUnsupportedBackend
	}

	// Get GPU information
	c.adapterInfo.Name = gl.GoStr(gl.GetString(gl.RENDERER))
	c.adapterInfo.Vendor = gl.GoStr(gl.GetString(gl.VENDOR))
	c.adapterInfo.Version = gl.GoStr(gl.GetString(gl.VERSION))

	c.frameData = defaultFrameData()
	c.initialized = true

	return nil
}

func (c *glCore) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return
	}

	// OpenGL resources are cleaned up automatically when context is destroyed
	c.initialized = false
}

func (c *glCore) SetupFrameData(zNear, zFar float32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return ErrCoreNotInitialized
	}
	if zNear <= 0 || zFar <= zNear {
		return fmt.Errorf("invalid depth range: near=%f far=%f", zNear, zFar)
	}

	logZNear := float32(math.Log2(float64(zNear)))
	logZFar := float32(math.Log2(float64(zFar)))

	const lightTilesZ = 32.0
	c.frameData.ZLogScale = float32(lightTilesZ) / (logZFar - logZNear)
	c.frameData.ZLogBias = -c.frameData.ZLogScale * logZNear
	c.frameData.FrameCount++

	return nil
}

func (c *glCore) Initialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

func (c *glCore) AdapterInfo() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.adapterInfo
}

func (c *glCore) FrameData() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.frameData
}

func defaultFrameData() FrameData {
	fd := FrameData{}
	fd.ViewProj = [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	return fd
}
