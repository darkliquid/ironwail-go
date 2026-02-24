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
	"github.com/gogpu/wgpu/hal"
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
}

func DefaultCoreConfig() CoreConfig {
	return CoreConfig{
		Backend:          gogpu.BackendGo,
		GraphicsAPI:      gogpu.GraphicsAPIAuto,
		EnableValidation: true,
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

	instance hal.Instance
	adapter  hal.Adapter
	device   hal.Device
	queue    hal.Queue

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

	backend, backendName, backendVariant := native.NewHalBackend(c.cfg.GraphicsAPI)
	if backend == nil {
		return fmt.Errorf("select HAL backend: %w", ErrCoreUnsupportedBackend)
	}

	flags := gputypes.InstanceFlags(0)
	if c.cfg.EnableValidation {
		flags = gputypes.InstanceFlagsDebug | gputypes.InstanceFlagsValidation
	}

	instance, err := backend.CreateInstance(&hal.InstanceDescriptor{
		Backends: gputypes.Backends(backendVariant),
		Flags:    flags,
	})
	if err != nil {
		return fmt.Errorf("create instance: %w", err)
	}

	adapters := instance.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		instance.Destroy()
		return ErrCoreNoAdapters
	}

	selected := pickBestAdapter(adapters)
	openDevice, err := selected.Adapter.Open(0, selected.Capabilities.Limits)
	if err != nil {
		selected.Adapter.Destroy()
		instance.Destroy()
		return fmt.Errorf("open device: %w", err)
	}

	c.backendName = backendName
	c.instance = instance
	c.adapter = selected.Adapter
	c.device = openDevice.Device
	c.queue = openDevice.Queue
	c.adapterInfo = selected.Info
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
		c.device.Destroy()
		c.device = nil
	}
	if c.adapter != nil {
		c.adapter.Destroy()
		c.adapter = nil
	}
	if c.instance != nil {
		c.instance.Destroy()
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

func pickBestAdapter(adapters []hal.ExposedAdapter) hal.ExposedAdapter {
	best := adapters[0]
	bestRank := adapterRank(best.Info.DeviceType)

	for _, adapter := range adapters[1:] {
		rank := adapterRank(adapter.Info.DeviceType)
		if rank > bestRank {
			best = adapter
			bestRank = rank
		}
	}

	return best
}

func adapterRank(t gputypes.DeviceType) int {
	switch t {
	case gputypes.DeviceTypeDiscreteGPU:
		return 5
	case gputypes.DeviceTypeIntegratedGPU:
		return 4
	case gputypes.DeviceTypeVirtualGPU:
		return 3
	case gputypes.DeviceTypeCPU:
		return 2
	default:
		return 1
	}
}
