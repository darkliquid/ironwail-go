# IRONWAIL-PLAN-003: UI Rewrite v3 Implementation Plan

- **Spec Reference:** `docs/internal/specs/003-ui-gogpu-rewrite-v3.md` (`IRONWAIL-SPEC-003`)
- **ADR Reference:** `docs/internal/adr/0010-engine-owned-overlay-rendering.md` (`ADR-0010`)
- **Branch:** `experiment/ui-rewrite-v3`
- **Estimated Milestones:** M1 through M5

---

## Phase 1 (M1): Engine-Owned Overlay Driver & Stack Refactoring

### Goal
Eliminate `desktop.Run` and `WorldTexture`. Create `OverlayRenderer` in `internal/quakui/` that renders 2D widget layers directly over the 3D world in the engine's overlay frame pass using `FlushGPUWithViewPreserveContent`.

### Tasks
1. **Task 1.1: Create `OverlayRenderer` in `internal/quakui/overlay.go`**
   - Implement `NewOverlayRenderer(host Host, mgr *legacymenu.Manager, con *console.Console, hudProv hud.Provider, drawMgr *draw.Manager, conchars []byte, palette []byte) *OverlayRenderer`.
   - Implement `DrawOverlay(targetView gpucontext.TextureView, width, height int) error`.
   - Implement `Event(e event.Event) bool`.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakui/... -count=1`
2. **Task 1.2: Remove `desktop.Run` and `WorldTexture`**
   - Delete `internal/quakui/run.go` and `internal/quakui/worldtexture.go`.
   - Update `Stack` in `internal/quakui/stack.go` to hold `[HUDRoot, ConsoleRoot, MenuRoot]`.
3. **Task 1.3: Wire `OverlayRenderer` in `internal/game`**
   - In `internal/game/game_runtime_frame.go`, instantiate `OverlayRenderer` during UI initialization.
   - In `drawRuntimeOverlayFrame`, invoke `g.overlayRenderer.DrawOverlay(targetView, w, h)`.
   - In `internal/game/game_input.go`, route `KeyMenu` and `KeyConsole` events to `g.overlayRenderer.Event(e)`.

---

## Phase 2 (M2): Menu & Visual Fidelity Verification

### Goal
Verify that `MenuRoot` renders over the 3D scene with real LMP pics, conchars font, navigation, and submenus without clearing the world.

### Tasks
1. **Task 2.1: Verify `MenuRoot` rendering and input**
   - Assert menu layout at 320x200 centered viewport.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakui/menu/... -count=1`

---

## Phase 3 (M3): Dropdown Console Verification

### Goal
Verify that `ConsoleRoot` animates and renders scrollback history, typing prompt, and autocompletion using single-image batched compositing.

### Tasks
1. **Task 3.1: Verify `ConsoleRoot` rendering and input**
   - Assert console typing, backspace, history, toggle grave/escape, and notify messages.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakui/console/... -count=1`

---

## Phase 4 (M4): Player HUD & Dynamic Resize

### Goal
Verify that `HUDRoot` draws the status bar, ammo/health numbers, face animations, and crosshair, dynamically adapting to window resize.

### Tasks
1. **Task 4.1: Verify `HUDRoot` resize and drawing**
   - Assert bottom-center status bar positioning on resize.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakui/hud/... -count=1`

---

## Phase 5 (M5): Cross-Platform & WASM Verification

### Goal
Run full repository verification and verify WebAssembly compilation.

### Tasks
1. **Task 5.1: Run Full Verification Suite**
   - `mise run verify`
2. **Task 5.2: Verify WASM Build**
   - `GOOS=js GOARCH=wasm go build ./...`

---

## Review Log

### Stage 8 — Review 3 (Plan Defense), 2026-08-20
- **Status:** APPROVED
- **Verdict:** All tasks are atomic, red-green testable, and preserve pure-Go cross-platform compatibility.
