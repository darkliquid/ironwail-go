package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
)

// TestInitializeUIPathFreezesDecision pins G11/ADR-0012 A1: ui_backend is
// read exactly once at startup and the decision is frozen — a mid-session
// cvar toggle after initializeUIPath must not flip the active path.
func TestInitializeUIPathFreezesDecision(t *testing.T) {
	// ui_backend 0 at startup -> legacy frozen; toggling to 1 later must not
	// activate the gogpu/ui path.
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "0")

	g.initializeUIPath()
	if !g.uiBackendFrozen {
		t.Fatal("initializeUIPath did not freeze the decision")
	}
	if g.uiPathActive() {
		t.Fatal("uiPathActive() = true with ui_backend 0 at startup (must be frozen legacy)")
	}

	// Mid-session toggle: no effect on the frozen path.
	g.Host.CVar.Set("ui_backend", "1")
	if g.uiPathActive() {
		t.Fatal("uiPathActive() = true after mid-session ui_backend toggle (G11 frozen path violated)")
	}
}

// TestInitializeUIPathForcesLegacyWithoutProvider pins the AC3c/AC4 fail-open:
// when a provider is unavailable (software/headless/pre-init), selecting
// ui_backend 1 must force the legacy path instead of rendering a broken
// overlay.
func TestInitializeUIPathForcesLegacyWithoutProvider(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "1")
	g.Renderer = &reloadTestRenderer{} // no GPUContextProvider / nil GogpuApp

	g.initializeUIPath()
	if !g.uiBackendForceLegacy {
		t.Fatal("expected forced legacy without a gogpu provider (software/headless fail-open)")
	}
	if g.uiPathActive() {
		t.Fatal("uiPathActive() = true despite forced legacy")
	}
}

// TestInitializeUIPathCreatesOverlayOnceAndWiresTeardown covers the G11
// lifecycle: on the gogpu/ui path the overlay (and uiApp widget tree) is
// created once at startup, teardown is wired to the renderer close hook, and
// teardownUIPath() is idempotent and releases the overlay.
func TestInitializeUIPathCreatesOverlayOnceAndWiresTeardown(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "0") // legacy: overlay not created
	g.Renderer = &providerTestRenderer{}

	g.initializeUIPath()
	if g.quakeuiOverlay != nil {
		t.Fatal("overlay created on the legacy path — must stay nil")
	}
	if g.uiTeardownRegistered {
		t.Fatal("teardown wired on the legacy path — must stay false")
	}

	// Force the gogpu/ui path by providing a stub provider through the
	// renderer.
	g2 := New()
	g2.Host = host.NewHost()
	g2.Host.CVar.Set("ui_backend", "1")
	g2.Renderer = &providerTestRenderer{}
	g2.Input = input.NewSystem(nil)
	g2.initializeUIPath()

	if !g2.uiPathActive() {
		t.Fatal("uiPathActive() = false with provider + ui_backend 1")
	}
	if g2.quakeuiOverlay == nil {
		t.Fatal("overlay (uiApp tree) not created at startup on gogpu/ui path")
	}
	if !g2.uiTeardownRegistered {
		t.Fatal("teardown not wired to the renderer close hook")
	}

	// Teardown idempotency: calling twice must not panic or double-release.
	g2.teardownUIPath()
	if g2.quakeuiOverlay != nil {
		t.Fatal("teardownUIPath did not release the overlay")
	}
	g2.teardownUIPath()
}

// providerTestRenderer is a minimal renderer exposing a gogpu device provider
// so initializeUIPath takes the gogpu/ui path. The provider is opaque and
// never used for actual drawing in this test.
type providerTestRenderer struct {
	reloadTestRenderer
}

func (r *providerTestRenderer) GPUContextProvider() gpucontext.DeviceProvider {
	return &lifecycleStubProvider{}
}

// lifecycleStubProvider implements gpucontext.DeviceProvider with opaque
// handles; only SurfaceFormat is exercised by life cycle selection.
type lifecycleStubProvider struct{}

func (l *lifecycleStubProvider) Device() gpucontext.Device             { return gpucontext.Device{} }
func (l *lifecycleStubProvider) Queue() gpucontext.Queue               { return gpucontext.Queue{} }
func (l *lifecycleStubProvider) Adapter() gpucontext.Adapter           { return gpucontext.Adapter{} }
func (l *lifecycleStubProvider) SurfaceFormat() gputypes.TextureFormat { return gputypes.TextureFormatUndefined }
func (l *lifecycleStubProvider) AdapterInfo() gpucontext.AdapterInfo {
	return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
}

// TestInitializeUIPathHeadlessFailsOpenLegacy is the M5.2 RED (AC3c): a
// headless boot (no renderer, hence no gogpu device provider) at ui_backend 1
// must fail open to legacy cleanly — the frozen path is legacy, uiPathActive
// is false, and no UI render is attempted (no overlay built).
func TestInitializeUIPathHeadlessFailsOpenLegacy(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "1")
	// nil Renderer == headless (no window, no gogpu provider).
	g.Renderer = nil

	g.initializeUIPath()
	if !g.uiBackendFrozen {
		t.Fatal("initializeUIPath did not freeze the decision")
	}
	if !g.uiBackendForceLegacy {
		t.Fatal("headless ui_backend 1 did not force legacy (AC3c fail-open violated)")
	}
	if g.uiPathActive() {
		t.Fatal("uiPathActive() = true on headless — UI render would be attempted without a surface")
	}
	if g.quakeuiOverlay != nil {
		t.Fatal("overlay built on headless fail-open path — must not attempt UI render")
	}
}
