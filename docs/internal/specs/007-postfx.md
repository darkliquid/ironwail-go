# SPEC-007: Modular Post-Processing Pipeline (Bloom, SSAO, CRT)

Bead: ironwail-go-nwo · Status: draft
Depends on: SPEC-006 §11.3 (renderer pass hooks), SPEC-006 §11.4 (post-FX shader chain)
Extends: SPEC-006 §11.5 (uxr bead — mods register custom post-FX hooks)

## Problem

The renderer's frame graph (`internal/renderer/frame_graph.go`) has a
`Pass` interface with named stages, but there is no post-processing
pipeline: the final frame is written directly to the swap chain without
any full-screen effect passes. Bloom, SSAO, and CRT retro filters require
intermediate textures, WGSL shaders, and pipeline objects that the current
architecture does not support.

## Goal

Add a modular post-processing pipeline to the renderer's frame graph that:

1. Implements bloom (HDR threshold → downsample blur → composite),
   SSAO (depth-based ambient occlusion → blur), and CRT
   (scanline/phosphor/dither) as independent, individually toggleable
   passes.
2. Registers cvars (`r_bloom`, `r_ssao`, `r_crt`) with zero overhead when
   disabled (no GPU work, no texture allocation, no pipeline creation).
3. Exposes a `PostFXRegistry` that mods use to insert custom WGSL
   post-processing passes at named points in the chain (via the `uxr`
   bead's `qcmod init` template and SPEC-006 §11.4 `PostFXShader`
   interface).

## Non-Goals

- Changing the existing world geometry, alias model, or particle rendering
  passes.
- Implementing transparency sorting (OIT already handles this).
- Supporting resolution scaling (render at lower res, upscale) — that is a
  separate feature that can reuse the same intermediate-texture pattern.

## Architecture

### PostFX chain position in the frame graph

The frame graph currently executes passes in insertion order. Post-FX
passes run after all world/entity/particle passes and before the overlay
(HUD/menu) pass. The chain is:

```
world passes (opaque, translucent, sky, particles, ...)
         |
         v
  [PostFX chain]                          <-- this spec
    1. SSAO (optional, reads depth)
    2. Bloom (optional, reads HDR color)
    3. CRT (optional, reads final color)
         |
         v
  overlay pass (HUD, menu, console, centerprint)
         |
         v
  swap chain present
```

Post-FX passes read from and write to intermediate textures managed by a
`PostFXTargets` struct. The chain is ordered: SSAO modifies the color
buffer first (it needs the depth buffer which is invalidated after the
depth attachment is resolved), then bloom reads the color buffer, then CRT
applies the final filter.

### PostFXPass interface

Extends the existing `Pass` interface with post-FX-specific metadata:

```go
// internal/renderer/postfx/pass.go
type PostFXPass interface {
    Pass // Name() string, Record(fc *FrameContext) error

    // Enabled reports whether this pass should execute this frame.
    // Checked before any GPU work; false means zero overhead.
    Enabled() bool

    // Init allocates GPU resources (textures, pipelines, bind groups).
    // Called once on first use or on resize. Returns error on failure.
    Init(device *wgpu.Device, surfaceFormat gputypes.TextureFormat, width, height int) error

    // Resize recreates intermediate textures on framebuffer resize.
    Resize(width, height int) error

    // Release frees GPU resources. Called on shutdown or when disabled.
    Release()
}
```

### PostFXTargets — intermediate texture management

```go
// internal/renderer/postfx/targets.go
type PostFXTargets struct {
    device *wgpu.Device
    width, height int
    surfaceFormat gputypes.TextureFormat

    // color targets for ping-pong rendering
    colorA, colorB *wgpu.TextureView

    // depth texture (read-only for SSAO)
    depth *wgpu.TextureView
}
```

The targets manage a ping-pong pair of RGBA16Float textures. Each post-FX
pass reads from one and writes to the other; the final pass writes to the
swap chain view. On resize, both are recreated.

### PostFXChain — ordered pass list

```go
// internal/renderer/postfx/chain.go
type Chain struct {
    passes  []PostFXPass
    targets *PostFXTargets
    device  *wgpu.Device
    enabled bool // master toggle; false = skip entire chain
}
```

The chain implements the existing `Pass` interface so it integrates with
the frame graph:

```go
frameGraph.Add(postfx.NewChain(device, targets, cvars))
```

### PostFXRegistry — mod extensibility (uxr integration)

The registry allows mods to insert custom passes at named positions in the
chain. Each position has an insertion order (before/after the built-in
passes).

```go
// internal/renderer/postfx/registry.go
type Registry struct {
    passes map[string]PostFXPass
    order  []string // execution order
}

// Register adds a custom pass at the given position.
// Position is one of: "pre-ssao", "pre-bloom", "pre-crt", "post-crt".
func (r *Registry) Register(name string, pass PostFXPass, position string)

// Unregister removes a pass by name.
func (r *Registry) Unregister(name string)
```

The `uxr` bead's `qcmod init` template exposes this through the SDK:

```go
// future: in mod's main.go (after engine.Run)
hooks.PostFX.Register("my-warp", myWarpPass{}, "post-crt")
```

## Pass implementations

### 1. CRT filter (simplest, implement first)

A single full-screen fragment shader applied to the final color buffer.

**Effect:** scanlines (horizontal darkening based on y), phosphor glow
(vertical blur between adjacent scanlines), and Bayer dithering.

**WGSL shader:** one fragment shader, reads the input color texture,
outputs the filtered color. No vertex transform (full-screen triangle).

```wgsl
// crt.frag.wgsl (simplified)
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let color = textureSample(inputTexture, inputSampler, uv);
    let scanline = 1.0 - scanlineStrength * abs(sin(uv.y * resolution.y * PI));
    let phosphor = mix(color.rgb, phosphorTint, phosphorStrength);
    let dither = bayer4x4(uv * resolution) * ditherStrength;
    return vec4(phosphor.rgb * scanline + dither, color.a);
}
```

**Pipeline:** one render pipeline (full-screen triangle, no depth test).
Reads from colorA, writes to colorB (or swap chain if last).

**Cvars:**
- `r_crt` (int): 0=off, 1=on. Default 0.
- `r_crt_scanline` (float): scanline strength 0-1. Default 0.3.
- `r_crt_phosphor` (float): phosphor glow 0-1. Default 0.2.
- `r_crt_dither` (float): dither strength 0-1. Default 0.1.

**LOC estimate:** ~200 lines (shader ~50, pipeline ~60, pass ~50, cvars ~20, tests ~20).

### 2. Bloom (medium complexity)

Multi-pass: bright-pass extraction, separable blur (horizontal + vertical
per level), and composite.

**Effect:** emissive surfaces, projectile trails, and explosion sprites
glow. Pixels above a luminance threshold are extracted, blurred at
multiple scales (a downsample chain), and additively composited back.

**WGSL shaders:**

1. **Bright pass** (`bloom_threshold.frag`): reads the color buffer,
   outputs `max(color - threshold, 0) * scale`. One per-pixel pass.

2. **Blur** (`bloom_blur.frag`): Gaussian blur, direction uniform
   (horizontal or vertical). Runs per mip level in the downsample chain.

3. **Composite** (`bloom_composite.frag`): reads the original color +
   blurred bloom texture, adds them with a strength multiplier.

**Downsample chain:**

```
colorA ──> threshold ──> bloom mip0 (half res)
                            |
                         blur H+V
                            |
                         bloom mip1 (quarter res)
                            |
                         blur H+V
                            |
                         ...
                            |
                         composite (sum all mips) ──> colorB
```

5 mip levels is typical for Quake-scale resolutions (320x200 to 1920x1080).

**Textures:**
- bloom_mip[0..4]: RGBA16Float at half, quarter, eighth, etc. resolution.
- Each mip needs a blur target (or ping-pong pair).

**Cvars:**
- `r_bloom` (int): 0=off, 1=on. Default 0.
- `r_bloom_threshold` (float): luminance threshold 0-1. Default 0.6.
- `r_bloom_strength` (float): composite strength 0-2. Default 0.8.
- `r_bloom_levels` (int): number of downsample levels 1-6. Default 5.

**LOC estimate:** ~600-800 lines (3 shaders ~200, pipelines ~150, texture management ~100, pass logic ~100, cvars ~30, tests ~50).

### 3. SSAO (highest complexity)

Screen-space ambient occlusion darkens corners and crevices where
ambient light is blocked by nearby geometry. Needs the depth buffer
(read-only) and generates an AO value per pixel.

**WGSL shaders:**

1. **SSAO compute** (`ssao.frag`): samples the depth buffer at multiple
   offsets around each pixel, reconstructs view-space position, computes
   occlusion from the depth difference. Outputs AO factor (0-1).

2. **SSAO blur** (`ssao_blur.frag`): bilateral blur that preserves edges
   (uses the depth buffer to avoid blurring across geometry boundaries).

**Implementation approach:** simplified hemisphere sampling (Kajija-style)
or a cheaper depth-difference approach. For Quake's low-poly geometry and
limited light count, a simple 4-tap depth-comparison approach is
sufficient and avoids the complexity of normal reconstruction.

**Textures:**
- ssao_raw: R8Unorm at full resolution (AO factor per pixel).
- ssao_blurred: R8Unorm at full resolution (blurred AO factor).

**Integration:** the SSAO factor multiplies the ambient contribution in
the world fragment shader. This requires either:
- A second render target that the world shader reads (clean but adds a
  pass), or
- Modifying the world fragment shader to read the SSAO texture directly
  (faster but couples SSAO to the world shader).

For v1, SSAO is applied as a post-pass (multiplies the color buffer by
the AO factor). This is less physically correct but requires no changes
to the world shader. A future version can move it into the world shader.

**Cvars:**
- `r_ssao` (int): 0=off, 1=on. Default 0.
- `r_ssao_radius` (float): sampling radius in world units. Default 16.
- `r_ssao_strength` (float): darkening strength 0-2. Default 1.0.
- `r_ssao_samples` (int): number of depth samples 4-16. Default 8.

**LOC estimate:** ~500-600 lines (2 shaders ~150, pipelines ~100, texture management ~80, pass logic ~100, cvars ~30, tests ~40).

## Implementation plan (milestones)

| M | Scope | Deliverable |
|---|---|---|
| P0 | PostFX framework: PostFXPass interface, PostFXTargets, Chain, cvar registration, `r_crt` pass | CRT working, framework proven |
| P1 | Bloom: threshold + blur chain + composite | Bloom working, HDR intermediate textures |
| P2 | SSAO: depth-based AO + bilateral blur | SSAO working, depth texture integration |
| P3 | PostFXRegistry for mod extensibility (uxr integration) | Mods can register custom passes |

Each milestone is independently testable and committable. P0 establishes
the framework that P1 and P2 build on.

## Cvar registration

All cvars are registered in `internal/renderer/cvars.go` alongside the
existing renderer cvars, with `cvar.FlagArchive` for persistence:

```go
func RegisterPostFXCvars(register func(name, defaultVal string, flags cvar.CVarFlags, desc string) *cvar.CVar) {
    register("r_bloom", "0", cvar.FlagArchive, "Bloom post-processing (0=off, 1=on)")
    register("r_bloom_threshold", "0.6", cvar.FlagArchive, "Bloom luminance threshold")
    register("r_bloom_strength", "0.8", cvar.FlagArchive, "Bloom composite strength")
    register("r_ssao", "0", cvar.FlagArchive, "SSAO post-processing (0=off, 1=on)")
    register("r_ssao_radius", "16", cvar.FlagArchive, "SSAO sampling radius")
    register("r_ssao_strength", "1.0", cvar.FlagArchive, "SSAO darkening strength")
    register("r_crt", "0", cvar.FlagArchive, "CRT retro filter (0=off, 1=on)")
    register("r_crt_scanline", "0.3", cvar.FlagArchive, "CRT scanline strength")
    register("r_crt_phosphor", "0.2", cvar.FlagArchive, "CRT phosphor glow")
    register("r_crt_dither", "0.1", cvar.FlagArchive, "CRT dither strength")
}
```

Zero overhead when disabled: the Chain's `Record` method checks
`Enabled()` on each pass and skips it entirely — no GPU commands are
encoded, no textures are bound, no pipelines are touched.

## Testing

- **P0**: unit test that the Chain skips all passes when all cvars are 0
  (no GPU commands encoded). Integration test: CRT pass produces different
  pixel values than the unfiltered frame.
- **P1**: unit test that the threshold shader outputs 0 for pixels below
  the threshold. Integration test: bloom makes emissive pixels glow.
- **P2**: integration test: SSAO darkens corners (compare AO values at
  wall-floor junctions vs. open floor).
- **P3**: unit test that a registered custom pass executes between the
  built-in passes.
- **Regression**: all existing renderer tests must pass unchanged (the
  post-FX chain is additive and disabled by default).

## Future work (not this bead)

- `sourcesContent` in source maps (DAP serves source without mod sources).
- HDR rendering pipeline (render to RGBA16Float, tone-map at the end).
- Resolution scaling (render world at lower res, upscale in post-FX).
- Motion blur (reads velocity buffer from the frame graph).
- Color grading / LUT support (reads a lookup texture).
- Screen-space reflections (reads color + depth, similar to SSAO).
