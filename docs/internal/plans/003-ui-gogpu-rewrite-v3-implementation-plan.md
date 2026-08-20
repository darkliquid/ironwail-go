# IRONWAIL-PLAN-003: UI Rewrite v3 Implementation Plan

- **Spec Reference:** `docs/internal/specs/003-ui-gogpu-rewrite-v3.md` (`IRONWAIL-SPEC-003`)
- **ADR Reference:** `docs/internal/adr/0010-engine-owned-overlay-rendering.md` (`ADR-0010`)
- **Branch:** `experiment/ui-rewrite-v3`
- **Estimated Milestones:** M1 through M5

---

## Phase 1 (M1): Engine-Owned Overlay Driver & Stack Refactoring

### Goal
Eliminate `desktop.Run` and `WorldTexture`. Create `OverlayRenderer` in `internal/quakeui/` that renders 2D widget layers directly over the 3D world in the engine's overlay frame pass using `FlushGPUWithViewPreserveContent`.

### Tasks
1. **Task 1.1: Create `OverlayRenderer` in `internal/quakeui/overlay.go`**
   - Implement `NewOverlayRenderer(host Host, mgr *legacymenu.Manager, con *console.Console, hudProv hud.Provider, drawMgr *draw.Manager, conchars []byte, palette []byte) *OverlayRenderer`.
   - Implement `DrawOverlay(targetView gpucontext.TextureView, width, height int) error`.
   - Implement `Event(e event.Event) bool`.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakeui/... -count=1`
2. **Task 1.2: Remove `desktop.Run` and `WorldTexture`**
   - Delete `internal/quakeui/run.go` and `internal/quakeui/worldtexture.go`.
   - Update `Stack` in `internal/quakeui/stack.go` to hold `[HUDRoot, ConsoleRoot, MenuRoot]`.
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
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakeui/menu/... -count=1`

---

## Phase 3 (M3): Dropdown Console Verification

### Goal
Verify that `ConsoleRoot` slides down smoothly, receives keyboard text input, executes engine commands, and handles notification message fading.

### Tasks
1. **Task 3.1: Verify `ConsoleRoot` rendering and interaction**
   - Assert toggle key opens/closes console.
   - Assert command execution through engine command buffer.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakeui/console/... -count=1`

---

## Phase 4 (M4): HUD & Non-Interactive Fallthrough Verification

### Goal
Verify that `HUDRoot` draws the status bar, crosshair, and centerprints while preserving full gameplay input fallthrough.

### Tasks
1. **Task 4.1: Verify `HUDRoot` rendering and fallthrough**
   - Assert key events fall through to `KeyGame` when only HUD is visible.
   - Test command: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakeui/hud/... -count=1`

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
