# Deno wasm-walkthrough test suite

Tests the engine's wasm build under [Deno](https://deno.com) — which ships a
real WebGPU implementation (wgpu) via `navigator.gpu` — without a browser.
This is the "can the walkthrough actually boot, run frames, and render"
question, answered headlessly.

## Files

| File | Purpose |
| --- | --- |
| `harness.ts` | Deno driver for the pollable engine wasm (`web/bin/ironwail-harness.wasm`): loads `wasm_exec.js`, mounts a Quake pak into linear memory, reads the shared state struct, injects input, steps frames, and captures pixels. |
| `walkthrough_test.ts` | Engine-level assertions: boots the engine, loads `maps/start.bsp`, advances the host frame loop, injects input that reaches the `input.System`, and verifies renderer/pixel status degrades gracefully without a WebGPU surface. |
| `webgpu_smoke_test.ts` | Engine-independent proof that Deno's WebGPU (OffscreenCanvas + `navigator.gpu`) can clear, draw, and **read back** pixels to the CPU — the platform prerequisite for any GPU pixel assertion. |
| `deno.json` | Import map + compiler options for the suite. |

## The engine side: `cmd/ironwailgo-harness`

A separate wasm `main` package with a **pollable ABI** (no `window`, no
`document`, no `requestAnimationFrame`). The engine's browser build
(`cmd/ironwailgo`) depends on DOM + rAF for its `syscall/js` glue and cannot
run under Deno's non-DOM runtime; the harness instead exposes pure-wasm-value
exports:

| Export | Purpose |
| --- | --- |
| `mount_resize(needed) → ptr` | Grow a linear-memory pak slot; Deno writes `pak0.pak` bytes into it. |
| `mount_pak() → rc` | `InitSubsystems` + attach the pak + `CmdMap("start")`. |
| `state_poll() → ptr` | 88-byte little-endian state struct: flags (running/map/paused), framecount, camera origin/angles, map entities, pixel arena metadata, input/diagnostics. |
| `input_slot() → ptr` | 32-byte input struct the host fills (`fwd, strafe, yaw, pitch, btn0..3`) then calls `input_inject`. |
| `engine_advance(dtNS) → ptr` | Run one host frame synchronously (mirrors the renderer `OnUpdate` wiring: poll input, mouse-look, `Host.Frame`) then poll state. |
| `pixels_capture() → 0/1` | Best-effort world-texture readback into the pixel arena (reuses `Renderer.ReadbackWorldTexture`); returns 0 when no WebGPU device/surface is present. |
| `debug_state() → ptr` | Scratch diagnostics at state+88: input flags, `cl.ViewAngles[1]*100`, injected mouse dx/dy, `state.MouseDX`, `m_yaw/m_pitch`. |

Why pollable and not browser-compatible: Go's js/wasm event loop only advances
*while inside an exported call*. The browser build parks on `requestAnimationFrame`
callbacks; Deno has no rAF, so the engine must be driven by explicit
`engine_advance` calls instead. All state crosses the boundary as flat
little-endian structs so the host never touches Go heap internals.

## Data

The engine needs a Quake data pak. The shareware `pak0.pak` (~18 MB) is the
smallest complete source — it contains `maps/start.bsp`, `progs.dat`, gfx,
and sounds:

```
curl -L -o .tmp/pak0.pak \
  https://github.com/pweil-/origin-quake/raw/refs/heads/master/id1/pak0.pak
```

The harness tries `.tmp/pak0.pak`, then `$PAK0_PATH`. Without a pak the suite
still runs but skips the map/input/pixel assertions ("No pak found" warning).

## Running

```bash
# build the harness wasm + run the suite
mise run test-deno

# or manually:
mise run build-wasm-harness
deno test -A --no-check web/deno-tests/
```

## What "renders" means here

- The **smoke test** proves Deno's WebGPU can produce and read back real
  pixels (clear color → staging buffer → exact BGRA bytes). If it fails, no
  render test can pass and the host (driver/GPU/CI) is the problem.
- `pixels_capture` returns **real** pixels only when the renderer has a
  WebGPU surface. Headless harness runs (no canvas, no DOM) report
  `pixel_valid=0` gracefully — that's the deterministic-degrade path, not a
  failure. Driving the *renderer's* surface under Deno (canvas polyfill) is
  future work; the harness ABI is already shaped for it.
- Deterministic assertions that always hold: engine boots → map active →
  frames advance → dt is honored → input reaches `input.System` → renderer
  present + readback attempts don't crash the engine.
