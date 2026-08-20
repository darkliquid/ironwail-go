# IRONWAIL-SPEC-003: gogpu/ui UI Rewrite v3 — Engine-Owned Render Loop + Visual Fidelity + WASM/Desktop Isolation

**Component Identifier:** IRONWAIL-SPEC-003  
**Status:** DRAFT (Stage 2) — pending Review 1  
**Date:** 2026-08-20  
**Branch:** `experiment/ui-rewrite-v3`  
**Supersedes:** IRONWAIL-SPEC-001 (abandoned: TTF text, engine leaks), IRONWAIL-SPEC-002 (desktop.Run lifecycle inversion)  
**Language/Runtime:** Go 1.26, `CGO_ENABLED=0`, gogpu/ui v0.1.54 / gogpu v0.53.0 / gg v0.52.3  
**Primary dependencies:** `github.com/gogpu/ui`, `github.com/gogpu/gg`, `github.com/gogpu/gogpu`  
**Target location:** `internal/quakui/` (self-contained) + engine overlay pass in `internal/game`

---

## 1. Metadata & Overview

### 1.1 Purpose

Synthesize the lessons of SPEC-001 and SPEC-002 into a permanent, production-ready UI architecture:
1. **Engine-Owned Render Loop (v1 strength)**: The engine retains full authority over the window event loop, frame timing (`Game.RenderFrame`), and frame stepping. No `desktop.Run` hijacking the main thread. This enables full compatibility with Native Desktop, WASM (`GOOS=js` / `requestAnimationFrame`), and headless environments.
2. **Visual Fidelity & Clean Isolation (v2 strength)**: Render real Quake `gfx/*.lmp` artwork and `conchars` bitmap typography at 320x200 legacy layout coordinates with zero TTF substitution. Keep `internal/quakui/` self-contained and isolated from `internal/game` internals.
3. **Direct Single-Pass GPU Overlay Rendering**: Render the 2D UI overlay directly on top of the active swapchain texture view using `gg.Context.FlushGPUWithViewPreserveContent` (`LoadOpLoad`), eliminating multi-pass offscreen `gpuview` texture churn, destroyed texture command buffer references, and Vulkan swapchain presentation failures.

### 1.2 Scope & Milestones

| Milestone | Scope | Deliverables |
|---|---|---|
| **M1: Engine-Owned Host & Menu** | Engine-owned frame loop + Main/Submenus | Overlay renderer driver, `quakui.OverlayRenderer`, `MenuRoot` rendering with real LMP pics + conchars |
| **M2: Dropdown Console** | Dropdown console + notification overlays | `ConsoleRoot` with batched single-image CPU compositing, scrollback history, command execution, tab completion |
| **M3: Player HUD** | Player heads-up display | `HUDRoot` with status bar, ammo/health/armor counters, face animations, crosshair, dynamic resize anchoring |
| **M4: WASM & Verification** | Cross-platform validation | Native + WebAssembly compilation and runtime verification |

### 1.3 Branch & Gating Strategy

- Active branch: `experiment/ui-rewrite-v3` (branched directly from `experiment/ui-rewrite-v2`).
- Retain the `ui_backend` cvar gate (0 = legacy path, 1 = gogpu/ui v3 path, default 0).
- Pure Go (`CGO_ENABLED=0`), zero build tags (`//go:build`).

---

## 2. High-Level AI-DLC / Agent Prompt

Implement the v3 UI rewrite behind `ui_backend 0|1` following IRONWAIL-SPEC-003:
1. Remove `desktop.Run` and `quakui.Run`. The engine owns the main window and calls `overlayRenderer.DrawOverlay(targetView, width, height)` during `drawRuntimeOverlayFrame`.
2. Use `gg.NewContext(width, height)` + `ui/render.NewCanvas(dc, width, height)` + `dc.FlushGPUWithViewPreserveContent(targetView, width, height)` to render the 2D widget overlay over the 3D world in a single render pass without clearing.
3. Preserve all v2 widgets (`MenuRoot`, `ConsoleRoot`, `HUDRoot`, `ConcharsAtlas`) and visual bridges (`QPicToImage`, `DrawGlyph`).
4. Route keyboard and character events from `game_input.go` directly into `overlayRenderer.Event(e)`.

---

## 3. Information Architecture & Topology

```
internal/quakui/
  overlay.go            OverlayRenderer — engine-owned frame driver (NewOverlayRenderer, DrawOverlay, Event)
  adapter.go            Host interface (CVar, PlaySound, ExecuteCommandText, Quit)
  stack.go              Stack container (MenuRoot, ConsoleRoot, HUDRoot)
  gfx/
    gfx.go              QPicToImage, ConcharsAtlas (DrawGlyph, GlyphImage), skin translation
  menu/
    menu.go             MenuRoot (real LMP pics, submenu layouts, navigation)
  console/
    console.go          ConsoleRoot (single-image batched compositing, dropdown, input)
  hud/
    hud.go              HUDRoot (status bar, crosshair, centerprint, dynamic resize)
internal/game/
  game_runtime_frame.go Wires OverlayRenderer into drawRuntimeOverlayFrame
  game_input.go         Routes KeyDest KeyMenu/KeyConsole events to OverlayRenderer
```

### 3.1 Frame Sequence

```
Engine Frame (game_runtime_frame.go):
  1. Engine updates physics, client state, and camera
  2. Engine renders 3D world into Swapchain View (Renderer.RenderWorldIntoView)
  3. If ui_backend == 1:
       OverlayRenderer.DrawOverlay(SwapchainView, WindowWidth, WindowHeight):
         a. Layout Stack to (WindowWidth, WindowHeight)
         b. Record widget draw commands into ui/render.NewCanvas(dc)
         c. dc.FlushGPUWithViewPreserveContent(SwapchainView) -> GPU single pass with LoadOpLoad
     Else (ui_backend == 0):
       drawRuntimeOverlayFrame(legacy)
  4. Swapchain View presented to display
```

---

## 4. Data Models & Interface Contracts

### 4.1 OverlayRenderer Contract

```go
type OverlayRenderer struct {
    host     Host
    stack    *Stack
    menuRoot *quakuimenu.MenuRoot
    conRoot  *quakuiconsole.ConsoleRoot
    hudRoot  *quakuihud.HUDRoot
    dc       *gg.Context
}

func NewOverlayRenderer(host Host, mgr *legacymenu.Manager, con *console.Console, hudProv hud.Provider, drawMgr *draw.Manager, conchars []byte, palette []byte) *OverlayRenderer
func (r *OverlayRenderer) DrawOverlay(targetView gpucontext.TextureView, width, height int) error
func (r *OverlayRenderer) Event(e event.Event) bool
```

---

## 5. Security & Failure Model

- Pure Go, no CGO, no unsanitized memory operations.
- Headless Mode: If `ui_backend=1` is active in headless, `OverlayRenderer` renders to a software/dummy canvas or falls back safely to legacy.
- Failure resilience: If WebGPU context acquisition fails, falls back gracefully without corrupting engine state.

---

## 6. Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| **AC1** | Engine owns the render loop; `desktop.Run` is completely eliminated | `grep -rn "desktop.Run" internal/` returns empty |
| **AC2** | Main menu, dropdown console, and player HUD render with full visual fidelity at `ui_backend=1` | Functional in-game verification |
| **AC3** | 2D UI composites over 3D world with `LoadOpLoad` without clearing world or offscreen texture leaks | GPU frame captures & clean logs |
| **AC4** | Keyboard and character typing in console and menu works with immediate reactivity | In-game console typing & menu navigation |
| **AC5** | HUD status bar dynamically repositions on window resize | Window resize tests |
| **AC6** | `mise run verify` passes on pure Go (`CGO_ENABLED=0`) | Automated test suite |

---

## Review Log

### Stage 3 — Review 1 (Spec Defense), 2026-08-20
- **Status:** APPROVED
- **Findings & Resolutions:**
  1. *Canvas & Context Lifecycle*: `OverlayRenderer` manages a reusable `*gg.Context` resized on demand, avoiding per-frame context allocations.
  2. *Single Pass Integration*: Direct GPU render pass with `FlushGPUWithViewPreserveContent` completely bypasses offscreen `gpuview` texture allocation, fixing all prior texture lifecycle issues.
  3. *Input Dispatch*: `OverlayRenderer.Event(e)` maps directly to `stack.Event`, providing clean KeyDest routing with zero double-dispatch.
