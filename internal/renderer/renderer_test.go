package renderer

import (
	"errors"
	"testing"

	"github.com/gogpu/gogpu"
)

func TestCoreDefaultConfig(t *testing.T) {
	cfg := DefaultCoreConfig()

	if cfg.Backend != gogpu.BackendGo {
		t.Fatalf("backend = %v, want %v", cfg.Backend, gogpu.BackendGo)
	}
	if cfg.GraphicsAPI != gogpu.GraphicsAPIAuto {
		t.Fatalf("graphics API = %v, want %v", cfg.GraphicsAPI, gogpu.GraphicsAPIAuto)
	}
	if !cfg.EnableValidation {
		t.Fatalf("EnableValidation = false, want true")
	}
}

func TestCoreSetupFrameDataRequiresInit(t *testing.T) {
	core := NewCore(DefaultCoreConfig())
	err := core.SetupFrameData(0.5, 65536)
	if !errors.Is(err, ErrCoreNotInitialized) {
		t.Fatalf("SetupFrameData error = %v, want %v", err, ErrCoreNotInitialized)
	}
}

func TestCoreInitHeadlessUnsupportedBackend(t *testing.T) {
	cfg := DefaultCoreConfig()
	cfg.Backend = gogpu.BackendRust

	core := NewCore(cfg)
	err := core.InitHeadless()
	if !errors.Is(err, ErrCoreUnsupportedBackend) {
		t.Fatalf("InitHeadless error = %v, want %v", err, ErrCoreUnsupportedBackend)
	}
}

func TestCoreInitHeadlessAndFrameData(t *testing.T) {
	core := NewCore(DefaultCoreConfig())
	err := core.InitHeadless()
	if err != nil {
		t.Skipf("headless renderer core unavailable in this environment: %v", err)
	}
	t.Cleanup(core.Shutdown)

	if !core.Initialized() {
		t.Fatalf("core not initialized after InitHeadless")
	}

	if err := core.SetupFrameData(0.5, 65536); err != nil {
		t.Fatalf("SetupFrameData failed: %v", err)
	}

	fd := core.FrameData()
	if fd.ZLogScale <= 0 {
		t.Fatalf("ZLogScale = %f, want > 0", fd.ZLogScale)
	}
	if fd.FrameCount != 1 {
		t.Fatalf("FrameCount = %d, want 1", fd.FrameCount)
	}
}
