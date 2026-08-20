# ADR-0010: Engine-Owned Overlay Rendering via Direct Preserve-Content GPU Flush

- **Status:** Accepted
- **Deciders:** Antigravity Team & Project Maintainer
- **Date:** 2026-08-20
- **Supersedes:** ADR-0006 (desktop/gpuview integration)
- **Related Specs:** IRONWAIL-SPEC-003

## Context and Problem Statement

In SPEC-002 / ADR-0006, the UI architecture inverted application control by delegating the entire render and event loop to `desktop.Run(gogpuApp, uiApp)`. This introduced several severe complications:
1. The 3D game world was forced to render into an offscreen `gpuview` texture, causing continuous texture recreation/destruction cycles during window resize and widget invalidation.
2. Vulkan command buffer submissions frequently referenced retired/destroyed textures, causing swapchain presentation failures.
3. Native WebAssembly (`GOOS=js`) frame execution via `requestAnimationFrame` (`StepWasmFrame`) was incompatible with `desktop.Run`'s blocking loop.
4. Headless execution required specialized mocks.

We need a UI rendering architecture that allows the engine to maintain authoritative ownership over the frame loop while rendering 2D `gogpu/ui` widgets cleanly over the 3D world.

## Decision Drivers

- **Engine Authority**: Frame pacing, world simulation, and swapchain presentation must remain directly owned by `Game.RenderFrame()`.
- **Cross-Platform Compatibility**: Full support for Native Desktop, WebAssembly in the browser, and headless test harnesses.
- **Rendering Performance & Reliability**: Zero offscreen GPU texture churn, zero Vulkan presentation crashes, single-pass overlay compositing.
- **Codebase Isolation**: `internal/quakeui` must remain self-contained without circular dependencies on `internal/game` or `internal/renderer`.

## Considered Options

1. **Option 1: Retain `desktop.Run` + `gpuview` (SPEC-002 status quo)**:
   - *Pros*: Uses upstream `desktop` framework loop.
   - *Cons*: Incompatible with WASM browser loops; complex offscreen texture lifecycle; Vulkan presentation instability.
2. **Option 2: Direct Single-Pass Overlay via `FlushGPUWithViewPreserveContent` (Chosen)**:
   - *Pros*: Engine renders 3D world directly to swapchain texture view; `quakeui.OverlayRenderer` records 2D UI widgets and flushes them directly to the swapchain view in a single pass with `LoadOpLoad`; zero offscreen texture allocation; works identically in native, WASM, and headless.
   - *Cons*: Engine must explicitly invoke `overlayRenderer.DrawOverlay` during overlay passes.
3. **Option 3: CPU Readback / Software Blit**:
   - *Pros*: Simple software frame buffer.
   - *Cons*: High CPU-GPU readback overhead, low framerate, defeats WebGPU acceleration.

## Decision Outcome

Chosen Option: **Option 2 (Direct Single-Pass Overlay via `FlushGPUWithViewPreserveContent`)**.

### Consequences

- **Positive**:
  - `desktop.Run` is completely eliminated from the codebase.
  - Native WebAssembly builds run seamlessly via `StepWasmFrame`.
  - Vulkan command buffer retirement conflicts and swapchain presentation failures are permanently resolved.
  - Frame rendering is consolidated into a single pass: 3D scene -> 2D overlay (`LoadOpLoad`) -> present.
- **Negative**:
  - `quakeui` cannot rely on `desktop.Run`'s window event listeners, requiring explicit KeyDest event routing from `game_input.go`.
