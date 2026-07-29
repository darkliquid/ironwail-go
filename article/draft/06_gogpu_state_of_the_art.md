# Chapter 6: GoGPU — Pure-Go WebGPU in Practice

The decision to use a pure-Go WebGPU stack is the defining technical gamble of
`ironwail-go`. The README states it as a first principle: *"gogpu/WebGPU as the
canonical gameplay renderer/runtime."* [#README](#readme) The project compiles
with `CGO_ENABLED=0`. There is no C in the runtime path — not in the renderer,
not in the audio, not in the windowing. This chapter is a field report on what
that actually means, using the real bugs, issues, and lessons encountered over
the course of the port.

---

## The GoGPU module family

GoGPU is not a single library. It is a family of Go modules, each with a
specific role in the WebGPU stack:

| Module | Version | Role |
| --- | --- | --- |
| `github.com/gogpu/gogpu` | v0.44.1 | High-level renderer: `App`, event loop, window, surface, input |
| `github.com/gogpu/gpucontext` | v0.21.0 | Event source abstraction: keyboard, mouse, resize, focus, IME |
| `github.com/gogpu/gputypes` | v0.5.1 | Type definitions: vertex formats, blend states, bind group layouts |
| `github.com/gogpu/naga` | v0.17.15 | WGSL → SPIR-V shader compiler (Go port of the naga project) |
| `github.com/gogpu/wgpu` | v0.30.10 | Low-level WebGPU bindings (Instance, Device, Queue, buffers, textures) |
| `github.com/go-webgpu/goffi` | v0.5.6 (indirect) | FFI layer for native library calls without CGO |
| `github.com/go-webgpu/webgpu` | v0.5.2 (indirect) | Underlying WebGPU API definitions |

The dependency graph is: `gogpu` → `gpucontext` (events) + `wgpu` (GPU
primitives) + `gputypes` (type defs). `wgpu` → `goffi` (cgo-free native FFI)
+ `webgpu` (API types). `naga` is used at shader compilation time to translate
WGSL source (Go string constants) into SPIR-V bytecode that the Vulkan backend
can consume. None of these require CGO — the native library FFI is done via
`purego` (cgo-free dynamic loading of shared libraries).

---

## The WGSL → SPIR-V pipeline via naga

WebGPU shaders are written in WGSL (WebGPU Shading Language). But the native
Vulkan backend does not consume WGSL directly — it needs SPIR-V bytecode. The
`naga` module is the compiler that bridges this gap: it parses WGSL, builds an
intermediate representation, and emits SPIR-V. This happens at pipeline creation
time, when the Go code calls `device.CreateRenderPipeline` with a shader module
compiled from a WGSL string constant.

Every shader in `ironwail-go` is a Go string constant:

```go
const worldVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    // ...
}
@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    // ...
}
`
```

When the pipeline is created, `naga` compiles this string to SPIR-V, and the
resulting shader module is bound into the pipeline. If `naga` produces invalid
SPIR-V, the pipeline creation may succeed (naga does not always validate against
the SPIR-V spec) but the GPU driver will crash or produce garbage at draw time.

This is exactly what happened.

---

## Bug: naga invalid SPIR-V for scalar `mix()` (issue #162)

### The problem

The SPIR-V spec requires all `FMix` operands to have matching types. When
WGSL's `mix()` function uses a scalar blend factor (`f32`) with `vec3<f32>`
operands, naga v0.15.2 emitted an `FMix` instruction with mismatched operand
types — a `vec3` and a scalar `float`. AMD's RADV driver tolerated this on the
integrated GPU, but running with `DRI_PRIME=1` to enforce the discrete NVIDIA
GPU crashed with `SIGSEGV at addr=0x10`. [#GogpuIssues](#gogpuissues)

### The workaround

Commit `d5ff084` splatted the scalar to a `vec3` explicitly:

```go
// Before (crashed on NVIDIA):
result = vec4<f32>(mix(result.rgb, uniforms.fogColor, uniforms.fogDensity), 1.0);

// After (workaround):
result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), 1.0);
```

This appears in `world_shaders_gogpu.go:637` and `:723`. The explicit
`vec3<f32>(...)` splat forces naga to emit a correct
`OpCompositeConstruct` + `FMix` sequence.

### The fix

naga v0.17.0+ fixed the scalar-to-vector splat automatically. The fix
produces:

```spirv
%22 = OpLoad %float %fog_ptr
%23 = OpCompositeConstruct %v3float %22 %22 %22    ; scalar → vec3 splat
%24 = OpExtInst %v3float FMix %a %b %23            ; all operands vec3 ✅
```

After upgrading to naga v0.17.15 (the current `go.mod` version), the
workaround is no longer necessary, though the explicit splat remains in the
shader as defensive coding. [#GogpuIssues](#gogpuissues)

---

## Bug: naga swizzle gap (issue #157 comments)

### The problem

naga's WGSL parser could not handle swizzle expressions in certain
contexts. The particle vertex shader used a writable swizzle compound
assignment that triggered `ExprSwizzle is not a pointer expression` in naga.
This prevented the GoGPU renderer from compiling its shaders at all — no
visuals, just a crash. [#GogpuIssues](#gogpuissues)

### The workaround

Commits `de40302` and `ef8f1c0` replaced writable swizzle compound
assignments with explicit vector reconstruction. The particle shader was
rewritten to avoid swizzles entirely. This was the breakthrough that
produced the first actual visuals on the GoGPU renderer — the screenshot in
issue #157 shows a rendered Quake scene after the swizzle fixes.

### The response

gogpu maintainer kolkov confirmed both the swizzle gap and the
`dpdx`/`dpdy`/`textureDimensions` SPIR-V issue (which affected the scene
composite fragment shader) as naga bugs, filed them as naga #45 and #46, and
said: *"These are exactly the real-world 3D patterns we were missing in our
test coverage."* [#GogpuIssues](#gogpuissues)

---

## Bug: Linux X11 input stub (issue #129)

### The problem

The X11 key handling code in gogpu's Linux platform layer was just a stub —
no keyboard events were delivered. Mouse input was also absent. This was the
first input blocker that forced the cgo-GLFW detour (Chapter 2).

### The fix

Fixed in gogpu v0.22.8. The `InputBackend` in
`internal/renderer/gogpu/input_backend.go` bridges gogpu's `gpucontext` event
source to the engine's `internal/input.Backend` interface. It uses callback-based
input (`OnKeyPress`, `OnMouseMove`, etc.) when available, and falls back to
polling `b.app.Input().Keyboard()` / `.Mouse()` state otherwise. The polling path
has a heartbeat log to detect silent input failures. [#GogpuIssues](#gogpuissues)

### The input architecture

The Go port's input is layered:

1. **Platform polling**: gogpu's `gpucontext` polls X11/Wayland events.
2. **Backend adaptation**: `InputBackend` (`input_backend.go`) translates
   `gpucontext` events into Quake key codes via `input_map.go`
   (`MapGPUContextMouseButton`, key code mappings).
3. **Input system**: `internal/input.System` normalizes events and dispatches
   based on `KeyDest` (console, menu, game).
4. **Game handler**: `internal/game/game_input.go` routes keys to the
   appropriate subsystem.

This keeps the higher-level game code unaware of gogpu/X11/Wayland details,
matching the C engine's separation of `in_sdl.c` from `keys.c`.

---

## Bug: no pointer lock / mouse grab (issues #173, #175)

### The problem

An FPS requires pointer lock (mouse grab) to look around without the cursor
leaving the window. gogpu had no API for this. On Wayland, the pointer
constraints protocol implementation did not exist at all.

### The resolution

Issue #173 asked for the feature; issue #175 pointed to
[`libwldevices-go`](https://github.com/bnema/libwldevices-go) as a potential
Wayland implementation dependency. Both were closed after gogpu added pointer
lock support. [#GogpuIssues](#gogpuissues)

---

## Bug: adapter power preference ignored (issue #176)

### The problem

On hybrid-GPU Linux systems (integrated + discrete, e.g., Intel + NVIDIA
laptops), the windowed renderer did not forward a power preference to
`RequestAdapter`. This meant the runtime might select the discrete NVIDIA
adapter even when the application explicitly requested low-power/integrated.
`DRI_PRIME` environment variables are not a reliable substitute.

### The fix

The issue proposed adding a `PowerPreference` field to `gogpu.Config` and
forwarding it through `RequestAdapter`. The `Core` struct in
`core_gogpu.go:46` now has `GPUPreference` in `CoreConfig`, with
`DefaultCoreConfig()` returning `GPUPreferHighPerformance`. The `CoreConfig`
is the Go-side mechanism for this — the engine can expose a user-facing GPU
preference cvar and pass it through. [#GogpuIssues](#gogpuissues)

---

## The Wayland two-connection bug (BUG-GOGPU-002)

This is the defining architectural bug of the gogpu stack, and the one that
caused the most frustration during the port. It is documented in the issue
#157 comment thread by gogpu maintainer kolkov. [#GogpuIssues](#gogpuissues)

### The problem

gogpu used two separate `wl_display_connect()` calls from the same process:

- **Pure Go connection** — owned `wl_seat`, `wl_pointer`, `wl_keyboard`
  (where gogpu listened for input events).
- **C libwayland connection** (via goffi) — owned the visible `wl_surface`
  + `xdg_toplevel` (where Vulkan rendered).

Wayland delivers input events to the connection that owns the focused
surface. The window was on the C connection. The input listeners were on
the Pure Go connection. **They never met.** No mouse, no keyboard, no input
of any kind registered.

### Why it worked on X11

On X11, window IDs are server-side — they are shared across connections.
Two X11 connections to the same display server can both see the same
window. Wayland surfaces are client-side — they are scoped to the
connection that created them. The dual-connection design that worked on X11
was fundamentally broken on Wayland.

### The verification

kolkov verified that no toolkit does this: *"GLFW, Gio, winit,
neurlang-wayland — all use a single connection."* The gogpu stack had gotten
away with it because X11's server-side window IDs masked the architectural
error.

### The fix

Bind `wl_seat` + `wl_pointer` + `wl_keyboard` on the C connection and
forward events to Go. gogpu's CSD (client-side decoration) code already did
exactly this for pointer events on decoration subsurfaces — it needed to be
generalized to the main surface. Tracked as BUG-GOGPU-002 (P0).
[#GogpuIssues](#gogpuissues)

### The lesson for engine authors

This bug is not just a gogpu bug. It is a lesson in what happens when a
cross-platform abstraction assumes platform semantics that do not hold. The
X11/Wayland split in Linux desktop graphics is a minefield, and the
"pure-Go, no-CGO" constraint makes it harder, not easier, because the FFI
boundary between Go and C libraries is where these connection-scope issues
live.

---

## The cgo-GLFW detour and return

Chapter 2 covered this at a high level. Here is the gogpu-specific arc:

1. **2026-02-24**: Commit `064c027` — "renderer: port WebGPU core
   initialization." The first GoGPU commit. One day later...
2. **2026-02-25**: Commit `15b888e` — "alternate cgo gl renderer." The
   detour begins. gogpu hits naga swizzle bugs and Wayland input failures.
   A cgo-based OpenGL renderer using GLFW for windowing becomes the
   working path.
3. **2026-03-28**: Commits `de40302` and `ef8f1c0` — swizzle workarounds
   land. GoGPU shaders compile. First visuals appear.
4. **2026-04-01**: Commit `d5ff084` — scalar `mix()` splat workaround for
   NVIDIA SPIR-V crash. Issue #162 filed.
5. **2026-04-05**: Commit `b2fb6e9` — "Retire gl+sdl (#11)." The OpenGL
   renderer, SDL input, and SDL audio are removed. GoGPU becomes the sole
   renderer. Oto becomes the canonical audio backend.
6. **2026-04-21**: Issue #157 updated with screenshot showing actual
   gameplay visuals on GoGPU.
7. **2026-04-24**: Commit `889f797` — "drop renderer shims." Game loop
   cleanup, final shim removal.

The gogpu issue #157 opening body captures the state at the detour's peak:

> I first attempted to tackle things using GoGPU as the rendering backend,
> but eventually hit enough issues that I sadly switched to cgo GLFW code.
> [#GogpuIssues](#gogpuissues)

The return was driven by naga fixes (swizzle, scalar `mix()`), the X11 input
fix (v0.22.8), and the decision that pure-Go was worth the remaining pain.

---

## General state of pure-Go graphics in 2026

### Where it is strong

- **No CGO**: The entire stack compiles with `CGO_ENABLED=0`. No C toolchain
  required. Cross-compilation is trivial. Static binaries. This is the
  primary value proposition — `ironwail-go` is a real 3D game engine with
  no C in its runtime.
- **WebGPU portability**: WebGPU is designed as a cross-platform API. The
  same WGSL shaders run on Vulkan (Linux), Metal (macOS), and D3D12
  (Windows). The browser target (WASM + WebGPU) is a future possibility.
- **Real-world validation**: `ironwail-go` is the largest real-world 3D
  engine running on the pure-Go GPU stack. gogpu issue #163 ("Ironwail-go
  demo") is the showcase thread. The gogpu maintainers use it as evidence
  that the stack can handle a real engine, not just toy examples.
  [#GogpuIssues](#gogpuissues)
- **Active development**: The gogpu maintainer (kolkov) is responsive and
  uses `ironwail-go` bug reports to prioritize naga and platform fixes.

### Where it is weak

- **naga maturity**: The WGSL → SPIR-V compiler has had real gaps in
  coverage. Swizzle expressions, scalar-vector `mix()`, derivatives
  (`dpdx`/`dpdy`), and `textureDimensions` all produced invalid SPIR-V at
  various points. Each was fixed, but each required a workaround until the
  fix landed. Engine authors must be prepared to read SPIR-V disassembly and
  file compiler bugs.
- **Wayland/windowing**: The two-connection bug (BUG-GOGPU-002) is the
  most severe example, but the broader issue is that Linux desktop
  windowing (X11 vs Wayland, pointer lock, IME, multi-layout) is a large
  surface area with many compositors and edge cases. gogpu issue #227
  (multiple keyboard layouts) is another example — 75 comments of
  discussion about X11 keyboard group handling.
- **Driver conformance variance**: AMD/RADV silently tolerates invalid
  SPIR-V that crashes NVIDIA. This means bugs can be hardware-specific and
  invisible until tested on multiple GPUs. The `DRI_PRIME=1` workflow is
  essential for hybrid-GPU testing.
- **Tooling**: GoGPU debugging is harder than OpenGL debugging. There is no
  equivalent to `apitrace` or `RenderDoc` that works smoothly with the
  pure-Go stack. The project built its own diagnostic tooling: `bspdiag`
  for offline BSP inspection, `r_debug_water` for per-frame liquid face
  telemetry, `r_debug_passes` for render pass tracing, and `host_speeds`
  for per-phase timing.
- **Documentation**: WebGPU is newer than OpenGL, and the pure-Go stack is
  newer than WebGPU. The `docs/RENDERER_LEARNING_PLAN.md` was written
  precisely because there was no existing curriculum for learning WebGPU
  via a real Go codebase.

### Lessons for engine authors

1. **Avoid swizzles in WGSL shaders.** Write explicit vector
   reconstruction. It is more verbose but survives naga parser gaps.
2. **Prefer explicit splats.** `vec3<f32>(scalar)` is safer than relying on
   implicit scalar-to-vector promotion in `mix()`, `clamp()`, etc.
3. **Validate SPIR-V on multiple drivers.** What RADV tolerates, NVIDIA
   crashes on. Test on both if possible.
4. **Build your own diagnostic tooling.** `r_debug_water`, `bspdiag`,
   `host_speeds` — these exist because standard graphics debuggers do not
   integrate smoothly with the pure-Go stack.
5. **Expect platform bugs to be architectural, not superficial.** The
   Wayland two-connection bug was not a missing feature; it was a
   fundamentally broken design that happened to work on X11.
6. **Contribute upstream.** Every bug filed against gogpu/naga was fixed.
   The pure-Go graphics stack improves because real engines stress-test it.
   `ironwail-go` is that stress test.

---

## References

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript
of gogpu/gogpu issues, fetched 2026-07-27).
