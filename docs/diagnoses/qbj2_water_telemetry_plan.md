# Runtime Telemetry Plan for qbj2 Water Lighting & Translucency

## Context

The static analysis in `docs/diagnoses/qbj2_water_lighting_translucency.md` shows
that the renderer logic appears correct on paper: alpha resolution produces
`water=0.6`, face classification should put water faces in `PassTranslucent`, and
the lit-water shader path matches C Ironwail. Despite this, the water renders
**too bright** and **fully opaque** at runtime.

The only way to close the gap between "correct on paper" and "wrong at runtime"
is to instrument the actual decision points and observe what values the GPU
receives. This document describes the telemetry to add, where to add it, and how
to interpret the results.

## Design Principles

1. **Gated by a cvar** — all telemetry logs behind `r_debug_water` (default `0`).
   When `0`, zero overhead. When `1`, emits per-frame diagnostic lines. This
   matches the existing `sv_debug_telemetry` pattern.

2. **Prefixed lines** — all emitted lines use `[rwater ...]` prefix so they can
   be grepped from a full log stream, matching the `[svdbg ...]` convention.

3. **Per-frame, not just first frame** — the existing first-frame stats log is
   useful but may miss runtime state changes (cvar toggles, map reloads). The
   water telemetry should emit every frame when enabled, but be concise enough
   not to flood logs.

4. **No code paths changed** — telemetry is purely additive logging. No control
   flow, no render logic, no shader changes. This ensures the bug is observed,
   not masked.

---

## Telemetry Instrumentation Points

### 1. Alpha resolution at classification time

**File**: `internal/renderer/world_render_gogpu.go`
**Location**: `renderWorldInternal`, after `liquidAlpha := worldLiquidAlphaSettingsForGeometry(...)` (line ~193)

```go
if rDebugWaterEnabled() {
    slog.Debug("[rwater] world alpha settings",
        "water", liquidAlpha.water,
        "lava", liquidAlpha.lava,
        "slime", liquidAlpha.slime,
        "tele", liquidAlpha.tele,
        "has_lit_water", worldHasLitWater,
        "geom_transparent_water_safe", worldData.Geometry.TransparentWaterSafe,
        "geom_has_water_override", worldData.Geometry.LiquidAlphaOverrides.HasWater,
        "geom_water_override", worldData.Geometry.LiquidAlphaOverrides.Water,
    )
}
```

**What this tells us**: Whether the `worldLiquidAlphaSettingsForGeometry` call
actually produces `water=0.6` at runtime. If this shows `water=1`, the override
is not being applied, confirming hypothesis #1 from the diagnosis.

### 2. Face classification counts

**File**: `internal/renderer/world_render_gogpu.go`
**Location**: After the classification loop (line ~312, after `classifyFacesMS`)

```go
if rDebugWaterEnabled() {
    liquidFaceCount := 0
    translucentLiquidCount := len(translucentLiquidFaces)
    opaqueLiquidCount := len(opaqueLiquidDraws)
    for _, face := range visibleFaces {
        if face.Flags&model.SurfDrawTurb != 0 && face.Flags&model.SurfDrawSky == 0 {
            liquidFaceCount++
        }
    }
    slog.Debug("[rwater] face classification",
        "visible_liquid_faces", liquidFaceCount,
        "translucent_liquid_faces", translucentLiquidCount,
        "opaque_liquid_draws", opaqueLiquidCount,
        "cache_hit", cacheHit,
    )
}
```

**What this tells us**: Whether water faces are being classified as translucent
or opaque. If `translucent_liquid_faces=0` and `opaque_liquid_draws>0`, the
faces are going through the wrong (opaque) path — confirming the "fully opaque"
symptom.

### 3. Opaque liquid uniform values

**File**: `internal/renderer/world_render_gogpu.go`
**Location**: Inside the opaque liquid batch loop (line ~474)

```go
if rDebugWaterEnabled() && (batch.key.litWater > 0 || len(opaqueLiquidBatches) <= 3) {
    slog.Debug("[rwater] opaque liquid batch",
        "batch_idx", i,
        "num_indices", batch.numIndices,
        "lit_water", batch.key.litWater,
        "alpha_uniform", 1, // hardcoded to 1 in writeWorldUniform call
        "texture_bind_group", fmt.Sprintf("%p", batch.key.textureBindGroup),
        "lightmap_bind_group", fmt.Sprintf("%p", batch.key.lightmapBindGroup),
    )
}
```

**What this tells us**: Whether any opaque liquid batches exist (they shouldn't
if `water=0.6`) and what `litWater` value they use. If opaque liquid batches
exist with `litWater=0`, the water would render fullbright ("too bright") with
alpha=1 ("fully opaque") — matching both symptoms exactly.

### 4. Translucent liquid render values

**File**: `internal/renderer/world_gogpu_translucent.go`
**Location**: `renderGoGPUSortedTranslucentFaceRendersHAL`, inside the render loop (line ~644)

```go
if rDebugWaterEnabled() && draw.liquid {
    slog.Debug("[rwater] translucent liquid draw",
        "face_flags", draw.face.face.Flags,
        "face_lightmap_index", draw.face.face.LightmapIndex,
        "alpha", draw.face.alpha,
        "lit_water", litWater,
        "has_lit_water", draw.hasLitWater,
        "pipeline", "liquid",
    )
}
```

**What this tells us**: The actual alpha and litWater values written to the GPU
for each translucent liquid face. If `alpha=0.6` and `lit_water=1`, the GPU
should be producing correct translucent lit water. If the output is still wrong
with these values, the bug is in the shader or GPU state, not the CPU-side
logic.

### 5. Translucent liquid face collection

**File**: `internal/renderer/world_gogpu_translucent.go`
**Location**: `collectGoGPUWorldTranslucentLiquidFaceRenders`, after collecting (line ~360)

```go
if rDebugWaterEnabled() {
    slog.Debug("[rwater] collected translucent liquid renders",
        "count", len(renders),
        "cached", cachedFaces != nil,
        "world_has_lit_water", worldHasLitWater,
        "liquid_alpha_water", liquidAlpha.water,
    )
}
```

**What this tells us**: Whether the translucent liquid collection actually
finds faces, and whether it's using the cache. If `count=0` here but
`opaque_liquid_draws>0` in telemetry point #2, the faces are being classified
as opaque — confirming the root cause.

### 6. Upload-time alpha settings

**File**: `internal/renderer/world_upload_gogpu.go`
**Location**: After `liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)` (line ~36)

```go
if rDebugWaterEnabled() {
    slog.Debug("[rwater] upload-time alpha settings",
        "water", liquidAlpha.water,
        "lava", liquidAlpha.lava,
        "slime", liquidAlpha.slime,
        "tele", liquidAlpha.tele,
        "has_lit_water", geom.HasLitWater,
        "transparent_water_safe", geom.TransparentWaterSafe,
        "liquid_face_types", geom.LiquidFaceTypes,
        "face_stats_opaque_liquid", faceStats.OpaqueLiquidFaces,
        "face_stats_translucent_liquid", faceStats.TranslucentLiquidFaces,
    )
}
```

**What this tells us**: Whether the alpha settings are correct at world upload
time (the earliest point). If `face_stats_translucent_liquid=0` here, the
problem is in the geometry build or alpha resolution, not the render loop.

---

## Cvar Registration

**File**: `internal/renderer/types.go`

```go
CvarRDebugWater = "r_debug_water" // Water render debug telemetry (0=off, 1=on)
```

**File**: `internal/game/game_init.go`

```go
g.Host.CVar.Register(renderer.CvarRDebugWater, "0", cvar.FlagNone, "Log water render decision points per frame (0=off, 1=on)")
```

**File**: `internal/renderer/water_debug.go` (new file)

```go
package renderer

import (
    "log/slog"

    "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

func rDebugWaterEnabled() bool {
    // Read the cvar via the renderer's cvar registry.
    // pkgCVars is the shared cvar accessor used by worldimpl.ReadAlphaCvar.
    if pkgCVars == nil {
        return false
    }
    cv := pkgCVars.Get(CvarRDebugWater)
    if cv == nil {
        return false
    }
    return cv.Int != 0
}
```

This follows the exact pattern of `worldLitWaterCvarEnabled()` in `world.go:370`.

---

## Interpretation Guide

### Scenario A: Water classified as opaque

**Symptom in telemetry**:
- Point #1: `water=0.6` (alpha resolution correct)
- Point #2: `translucent_liquid_faces=0, opaque_liquid_draws>0`
- Point #3: Opaque liquid batches exist with `alpha_uniform=1, lit_water=0`

**Diagnosis**: The face classification is putting water faces in
`PassOpaque` despite `water=0.6`. This means `worldFaceAlpha` or `worldFacePass`
is returning the wrong result. Check:
- Is `face.Flags` actually set with `SurfDrawWater`? (could be a flag derivation bug)
- Is `liquidAlpha.water` actually 0.6 at the time `shouldDrawGoGPUOpaqueLiquidFace` is called?

### Scenario B: Alpha resolution fails

**Symptom in telemetry**:
- Point #1: `water=1` (alpha resolution WRONG)
- Point #6: `face_stats_translucent_liquid=0` (faces classified as opaque at upload time)

**Diagnosis**: The worldspawn `wateralpha=0.6` override is not being applied.
Check:
- Is `geom.LiquidAlphaOverrides.HasWater` true? (point #1 shows this)
- Is `ReadAlphaCvar(CvarRWaterAlpha, 1)` returning 1 when it should return something else?
- Is `pkgCVars` nil? (would cause `ReadAlphaCvar` to return the default, not the cvar value)

### Scenario C: Correct CPU values, wrong GPU output

**Symptom in telemetry**:
- Point #1: `water=0.6` (correct)
- Point #2: `translucent_liquid_faces>0, opaque_liquid_draws=0` (correct)
- Point #4: `alpha=0.6, lit_water=1` (correct)
- But water still renders opaque/bright

**Diagnosis**: The CPU-side logic is correct. The bug is in the GPU pipeline,
shader, or render pass state. Check:
- Is the `worldTranslucentTurbulentPipeline` actually being selected (not the opaque one)?
- Is the blend state correct in the pipeline?
- Is the depth write disabled for the translucent pass?
- Is the shader's `sampled.a` value actually 1.0 for liquid textures? (could be a texture upload bug)
- Is there a subsequent render pass overwriting the translucent water?

### Scenario D: Lit water not enabled

**Symptom in telemetry**:
- Point #4: `lit_water=0` for translucent liquid faces
- But `has_lit_water=true` and `world_has_lit_water=true`

**Diagnosis**: The `gogpuWorldLightmapArrayBindGroupForFace` function is
returning `litWater=0` when it should return `litWater=1`. Check:
- Is `worldLitWaterCvarEnabled()` returning false? (r_litwater cvar not set?)
- Is `face.Flags&SurfDrawTurb` actually set?
- Is `face.Flags&SurfDrawSky` incorrectly set?
- Is the `hasLitWater` parameter (geometry-level flag) true?

---

## 7. WebGPU Pipeline & Dynamic Uniform Offset Telemetry

**File**: `internal/renderer/world_gogpu_translucent.go`
**Location**: `renderGoGPUSortedTranslucentFaceRendersHAL`, inside draw loop (lines ~640-675)

```go
if rDebugWaterEnabled() && draw.liquid {
    slog.Debug("[rwater] late translucent gpu draw",
        "face_idx", draw.face.face.FirstIndex,
        "num_indices", draw.face.face.NumIndices,
        "alpha", draw.face.alpha,
        "lit_water", litWater,
        "dynamic_uniform_offset", offset,
        "uniform_buffer_slice_bytes", fmt.Sprintf("%x", uData[:32]),
        "pipeline_ptr", fmt.Sprintf("%p", pipeline),
        "texture_bg", fmt.Sprintf("%p", textureBindGroup),
        "lightmap_bg", fmt.Sprintf("%p", lightmapBindGroup),
    )
}
```

**What this tells us**:
- Whether `offset` (dynamic uniform offset) is validly aligned (256-byte aligned in WebGPU).
- Whether `uData` contains the expected float32 bits for `alpha` (e.g. 0.6) and `litWater` (e.g. 1.0) at offsets 96 and 100.
- Whether `pipeline` points to `res.liquidPipeline` (`worldTranslucentTurbulentPipeline`) or accidentally `worldTranslucentPipeline` / `worldTurbulentPipeline`.

---

## Tooling Extensions: Offline `bspdiag liquids` Diagnostic Command

**File**: `cmd/bspdiag/liquids.go`

Expand `bspdiag liquids <quake_dir> <map.bsp> [gamedir]` to print:
1. **Map-level Water Vis Safety**: `TransparentWaterSafe` boolean, `worldspawn` entity fields (`wateralpha`, `watervis`, `transwater`), and leaf-level visibility mask calculation details (`contentTransparent`, `contentFound`).
2. **Liquid Surface Breakdown**: Enumerates every liquid face and prints face index, texture name, `Texinfo` flags (`TEX_SPECIAL`), `SurfDrawTiled` flag, `LightOfs`, lightmap index, and sample variation.
3. **Resolved Alpha Settings**: Prints the exact outputs of `ResolveLiquidAlphaSettings` for `water`, `lava`, `slime`, `tele`.

---

## Comprehensive Execution Plan

### Step 1: Implement Telemetry Instrumentation & `bspdiag` Extensions
1. Register `r_debug_water` cvar in `internal/renderer/types.go` and `internal/game/game_init.go`.
2. Add `internal/renderer/water_debug.go` helper `rDebugWaterEnabled()`.
3. Add Telemetry Points #1 through #7 across `world_render_gogpu.go`, `world_upload_gogpu.go`, and `world_gogpu_translucent.go`.
4. Extend `cmd/bspdiag` with enhanced `liquids` inspection output.

### Step 2: Run Offline Diagnostics (`bspdiag`)
```bash
mise run build-bspdiag
./bspdiag liquids quake-data/id1 start.bsp qbj2
```
Verify whether `MapVisTransparentWaterSafe` evaluates to `true` or `false` on `qbj2 start.bsp`.

### Step 3: Run Engine with Telemetry
```bash
mise run run -- -basedir ./quake-data -game qbj2 +map start +r_debug_water 1 2>&1 | grep '\[rwater\]' > water_telemetry.log
```

### Step 4: Analyze Log Output against Candidate Root Causes
- If `[rwater] world alpha settings` shows `water=1.0` → **Vis Safety Override bug** in `MapVisTransparentWaterSafe` or entity lump parsing.
- If `[rwater] face classification` shows `opaque_liquid_draws > 0` → **Classification bug** in `shouldDrawGoGPUOpaqueLiquidFace`.
- If `[rwater] late translucent gpu draw` shows `alpha=0.6` but pipeline is opaque → **Pipeline selection bug** in `renderGoGPUSortedTranslucentFaceRendersHAL`.
- If `[rwater] late translucent gpu draw` shows `dynamic_uniform_offset` data byte mismatch → **Uniform buffer offset / alignment bug**.
- If `[rwater] late translucent gpu draw` shows correct pipeline and alpha 0.6 → **WebGPU Blend State or Shader Lightmap Overbrighting bug** (`worldTranslucentTurbulentPipeline` creation in `world_pipelines_gogpu.go`).

### Step 5: Parity Screenshot Comparison
```bash
mise run parity-ref
mise run parity-go
mise run parity-compare
```
Compare rendered outputs against C Ironwail reference screenshots to verify transparency and lighting levels.

---

## Verification Checklist

- [ ] `r_debug_water 0` produces no `[rwater]` log lines (zero overhead when disabled)
- [ ] `r_debug_water 1` produces `[rwater]` log lines on every frame
- [ ] Log lines can be filtered with `grep '\[rwater\]'`
- [ ] Offline `bspdiag liquids` inspects map vis safety and liquid face flags
- [ ] Telemetry does not change any render logic or control flow
- [ ] Build passes: `TMPDIR=.../.tmp CGO_ENABLED=0 go build ./...`
- [ ] Tests pass: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1`
- [ ] Run engine with telemetry on `qbj2 start.bsp` and analyze `water_telemetry.log`
- [ ] Document empirical findings in `docs/diagnoses/qbj2_water_lighting_translucency.md`
