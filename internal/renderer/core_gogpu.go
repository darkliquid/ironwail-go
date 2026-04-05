package renderer

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gpu/backend/native"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

var (
	ErrCoreUnsupportedBackend = errors.New("renderer core only supports pure-Go backend")
	ErrCoreNoAdapters         = errors.New("no compatible GPU adapters found")
	ErrCoreNotInitialized     = errors.New("renderer core is not initialized")
)

type CoreConfig struct {
	Backend          types.BackendType
	GraphicsAPI      types.GraphicsAPI
	EnableValidation bool
	GPUPreference    GPUPreference
}

func DefaultCoreConfig() CoreConfig {
	return CoreConfig{
		Backend:          gogpu.BackendGo,
		GraphicsAPI:      gogpu.GraphicsAPIAuto,
		EnableValidation: true,
		GPUPreference:    GPUPreferHighPerformance,
	}
}

type FrameData struct {
	ViewProj  [16]float32
	ZLogScale float32
	ZLogBias  float32

	FrameCount int
}

type Core struct {
	mu sync.RWMutex

	cfg CoreConfig

	backendName string

	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue

	adapterInfo gputypes.AdapterInfo
	frameData   FrameData

	initialized bool
}

func NewCore(cfg CoreConfig) *Core {
	return &Core{cfg: cfg}
}

func (c *Core) InitHeadless() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	if c.cfg.Backend == gogpu.BackendRust {
		return ErrCoreUnsupportedBackend
	}

	backendName, backendVariant := native.BackendInfo(c.cfg.GraphicsAPI)
	if backendVariant == 0 {
		return fmt.Errorf("select HAL backend: %w", ErrCoreUnsupportedBackend)
	}

	instance, err := wgpu.CreateInstance(&wgpu.InstanceDescriptor{
		Backends: gputypes.Backends(backendVariant),
	})
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}

	var powerPref gputypes.PowerPreference
	switch c.cfg.GPUPreference {
	case GPUPreferHighPerformance:
		powerPref = gputypes.PowerPreferenceHighPerformance
	case GPUPreferLowPower:
		powerPref = gputypes.PowerPreferenceLowPower
	default:
		powerPref = gputypes.PowerPreferenceNone
	}
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference: powerPref,
	})
	if err != nil {
		instance.Release()
		return ErrCoreNoAdapters
	}

	openDevice, err := adapter.RequestDevice(&wgpu.DeviceDescriptor{
		Label:            "Ironwail-Go WGPU Device",
		RequiredFeatures: 0,
		RequiredLimits:   adapter.Limits(),
	})
	if err != nil {
		adapter.Release()
		instance.Release()
		return fmt.Errorf("open device: %w", err)
	}

	c.backendName = backendName
	c.instance = instance
	c.adapter = adapter
	c.device = openDevice
	c.queue = openDevice.Queue()
	c.adapterInfo = adapter.Info()
	c.frameData = defaultFrameData()
	c.initialized = true

	return nil
}

func (c *Core) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return
	}

	if c.device != nil {
		_ = c.device.WaitIdle()
		c.device.Release()
		c.device = nil
	}
	if c.adapter != nil {
		c.adapter.Release()
		c.adapter = nil
	}
	if c.instance != nil {
		c.instance.Release()
		c.instance = nil
	}

	c.queue = nil
	c.initialized = false
}

func (c *Core) SetupFrameData(zNear, zFar float32) error {
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

func (c *Core) Initialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

func (c *Core) AdapterInfo() gputypes.AdapterInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.adapterInfo
}

func (c *Core) FrameData() FrameData {
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
