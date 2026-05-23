# Ironwail-Go Parity Guide

This is the canonical parity document for Ironwail-Go. It consolidates the old
scene matrix and broad parity audit into one live guide for comparing the Go
port against reference C Ironwail.

## Scope and reference baseline

The C Ironwail/Quake implementation is the canonical behavior reference. Use it
to resolve ambiguity before changing Go behavior, especially for renderer pass
ordering, collision, trigger callbacks, QuakeC state synchronization, and wire
formats.

Near-term parity is scoped to the canonical NetQuake/FitzQuake engine path
through the GoGPU renderer. CSQC runtime integration remains deferred: the repo
has CSQC wrapper infrastructure, but host/client runtime wiring for a full CSQC
gameplay path is outside the current parity milestone.

The current parity focus is `qbj3_stickflip`: record the evidence workflow and
known investigation targets without committing map assets, generated screenshots,
or harness data.

## Current qbj3_stickflip status

`qbj3_stickflip` is the current priority community-map stress case. The latest
local sweep collected Go-side startup, trigger, renderer, C reference, window
capture, and CPU profile evidence. Automated harness comparison remains blocked
by missing viewpoint data and by the current GoGPU `-screenshot` path producing a
placeholder image instead of reading back the swapchain.

| Area | Status | Primary Go surfaces | C reference surfaces | Evidence needed |
| --- | --- | --- | --- | --- |
| Performance | Reported poor | `internal/renderer/world_render_gogpu.go`, `internal/renderer/render_pass_parity.go`, `internal/game/runtime_frame.go` | `Quake/gl_rmain.c`, `Quake/r_world.c`, `Quake/host.c` | C vs Go frame timings, visible face counts, batch counts, entity counts, GPU upload counters |
| Texture/decal z-fighting | Reported | `internal/renderer/world_gogpu_decal.go`, `internal/renderer/world_gogpu_brush_render.go`, `internal/renderer/world_depth_gogpu.go` | `Quake/gl_rmain.c`, `Quake/r_world.c` | Matched screenshots, onion overlays, depth/stencil state notes, decal count/location notes |
| Trigger reliability | Reported | `internal/server/world.go`, `internal/server/physics.go`, `internal/server/debug_telemetry.go` | `Quake/world.c`, `Quake/sv_phys.c`, `Quake/pr_cmds.c` | `sv_debug_telemetry` logs, trigger classname/targetname list, C-vs-Go touch/use sequence comparison |
| External skybox and map data | Suspected relevant | `internal/renderer/sky/external.go`, `internal/server/sv_main.go`, `internal/bsp` | `Quake/gl_sky.c`, `Quake/gl_model.c` | Entity lump notes, skybox config lookup, BSP variant/texture flag summary |

### Latest local qbj3_stickflip sweep findings

Local artifacts live outside the repository in the session artifact folder. Do
not commit generated logs, screenshots, profiles, or qbj3 assets unless a
maintainer explicitly asks for a reviewed reproduction artifact.

Verified findings from the latest local sweep:

- `qbj3_stickflip` is present under the local `qbj3` game directory and loads
  through `-game qbj3`.
- C Ironwail v0.8.2-dev launches the map with the supplied reference binary and
  `-basedir`, enters the game, and can be captured through the visible X11
  window.
- Go headless startup spawns `qbj3_stickflip`, mounts the filesystem, loads QC,
  and reaches `map spawn finished`.
- Trigger telemetry is very active on map start and early frames. The sweep saw
  trigger spawn records for `trigger_multiple`, `trigger_once`,
  `trigger_monsterjump`, `trigger_push`, `trigger_once_box`, `trigger_counter`,
  `trigger_relay`, `trigger_ladder`, `trigger_instateleport_destination`,
  `trigger_instateleport`, `trigger_teleport`, `trigger_secret`, and
  `trigger_changelevel`.
- Touch callback evidence was observed for `trigger_multiple`, `trigger_push`,
  `trigger_once`, `trigger_monsterjump`, `trigger_ladder`,
  `trigger_instateleport`, `trigger_secret`, and `trigger_changelevel`.
- GoGPU world upload stats for the local qbj3 capture show a large map:
  85,936 raw faces, 77,001 built faces, 168,142 built triangles, 322,144
  vertices, 22,195 leafs, 750 models, 106 textures, four lightmap pages, 1,295
  lit-water/turbulent faces, and 228 sky faces.
- The first rendered qbj3 frame at the captured spawn view reported 1,002 visible
  faces, 2,029 visible triangles, eight opaque world batches, seven opaque brush
  entities, eleven opaque alias entities, no decal marks, and no late
  translucency.
- The map requests external skybox `stick_sunset2_`. PNG candidates are missing,
  but TGA faces load successfully; `stick_sunset2_wind.cfg` is missing, so no
  external sky wind config is loaded.
- A short headless CPU profile is dominated by QC/server edict synchronization
  and reflection-heavy paths such as `syncEntVarsFromQC`,
  `syncEntVarsToQC`, `captureNonPusherQCVMEdictSnapshots`,
  `syncMutatedNonPushersFromQCVM`, and `qc.(*VM).SetEFloat`.
- Clean external window captures for C and Go at the qbj3 spawn view both
  rendered the same room at 1892x1072. The Go image is visibly darker and lower
  contrast, with the largest differences in the upper-center ceiling/light
  regions. Pixel comparison reported RGB mean absolute delta 7.893/7.062/6.898,
  95.030% of pixels differing by at least one luma step, 35.632% differing by
  more than four luma steps, and 8.951% differing by more than sixteen luma
  steps. Treat this as local manual evidence, not harness-grade sign-off.

Blocked or incomplete evidence:

- The committed `testdata/parity/viewpoints.json` is intentionally a tiny id1
  smoke seed. Add local qbj3-specific viewpoints from C `viewpos` output before
  treating harness results as qbj3 sign-off.
- The GoGPU runtime `-screenshot` path is still not the canonical visual capture
  path. The parity harness defaults to `PARITY_GO_CAPTURE=window`, which captures
  the rendered X11 window via `xdotool` and ImageMagick `import`; use
  `PARITY_GO_CAPTURE=engine` only when testing the in-engine screenshot fallback.
- The matched qbj3 spawn captures came from external window capture, and they do
  not cover reported decal/z-fighting or trigger failure locations yet.
- The debug GoGPU screenshot attempt timed out before writing an image, although
  it did emit useful renderer upload and first-frame stats.

## qbj3_stickflip evidence workflow

Keep all qbj3 map packages, screenshots, overlays, logs, and profiler outputs as
local working artifacts unless a maintainer explicitly approves committing a
small synthetic reproduction. Do not commit proprietary or third-party map
assets.

Use environment variables instead of hard-coded local paths:

```bash
export QUAKE_BASEDIR=/path/to/quake
export IRONWAIL_BIN=/path/to/c/ironwail
export PARITY_GO_CAPTURE=window
```

Recommended C reference launch:

```bash
"${IRONWAIL_BIN}" -basedir "${QUAKE_BASEDIR}" -window +map qbj3_stickflip
```

Recommended Go launch after building:

```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp mise run build
./ironwailgo -basedir "${QUAKE_BASEDIR}" +map qbj3_stickflip
```

For automated visual captures, use the mise parity tasks from the repository
root. The Go capture path currently uses the visible X11 window, so the local
machine needs `DISPLAY`, `xdotool`, and ImageMagick `import` available:

```bash
mise run parity-ref
mise run parity-go
mise run parity-compare
```

For each issue, capture:

1. `viewpos` from C Ironwail and an equivalent Go `setpos <x> <y> <z> <pitch> <yaw> <roll>`.
2. Console cvars used for the capture.
3. C screenshot and Go screenshot stored locally.
4. Whether the difference is visible without pixel comparison.
5. The suspected subsystem and a short "why" based on C reference behavior.

Use clean visual captures:

```text
scr_viewsize 130
r_drawviewmodel 0
crosshair 0
fov 90
host_framerate 0.0001
```

## qbj3_stickflip sweep checklist

Use this checklist to make the qbj3 sweep exhaustive enough to support follow-up
fix work.

| Sweep | What to inspect | Capture notes |
| --- | --- | --- |
| Dense outdoor view | Sky, world PVS, face batching, frustum culling, GPU uploads | Include `host_speeds 1`, `r_debug_passes 1`, `devstats` |
| Decal/z-fighting site | Decal depth compare, depth bias, stencil behavior, brush/world coplanar surfaces | Capture before and after shooting/mark creation if relevant |
| Texture alignment site | BSP texinfo, animated textures, fullbright, lightmap page selection | Include still screenshots and texture names when identifiable |
| Liquid/alpha site | Opaque vs translucent liquid pass ordering, brush-entity liquid handling | Record alpha cvars and whether water/lava/slime/tele surfaces are present |
| Moving brush area | Pusher state, brush model render order, collision hull selection | Record edict numbers, model indices, `ltime`, `nextthink`, and blocked callbacks |
| Trigger path | `touchLinks`, `Impact`, `UseTargets`, QuakeC mutation side effects | Use trigger filters and compare C/Go callback order |
| Entity stress view | Visedicts, temp entities, sprites, particles, dynamic lights | Include `devstats`, `profile`, and visible effect notes |

## Trigger telemetry workflow

The server already exposes focused telemetry for trigger and QC behavior:

```text
sv_debug_telemetry 1
sv_debug_telemetry_events trigger,touch,physics,qc
sv_debug_telemetry_classname trigger_*
sv_debug_telemetry_summary 2
```

Escalate to QC trace only for a narrow repro:

```text
sv_debug_qc_trace 1
sv_debug_qc_trace_verbosity 2
```

For qbj3 trigger failures, record:

- The trigger edict number, classname, targetname, target, model, origin, absmin,
  and absmax.
- The moving/player edict origin, velocity, flags, ground entity, fixangle, and
  teleport time before and after the callback.
- Whether `touchLinks` found zero candidates, rejected overlap by axis, executed
  a callback, or saw the callback mutate linked pusher state.
- Whether target activation happens inside QuakeC (`SUB_UseTargets`) and needs
  QC trace evidence rather than a server-side `use` telemetry line.
- The matching C behavior from `SV_AreaTriggerEdicts`, `SV_TouchLinks`, and
  `SV_LinkEdict` in `Quake/world.c`.

## Performance workflow

Use both engine-level and subsystem-level evidence. A qbj3 performance report is
actionable only when it says which phase is expensive.

Recommended console commands:

```text
host_speeds 1
r_debug_passes 1
devstats
profile
```

For longer captures, use the runtime profiling commands:

```text
profile_cpu_start
map qbj3_stickflip
profile_cpu_stop
profile_dump_heap
profile_dump_allocs
```

Classify evidence into:

| Class | Symptoms | Likely owner |
| --- | --- | --- |
| World visibility/batching | High visible face count, repeated batch rebuilds, cache misses | `internal/renderer/world_render_gogpu.go` |
| Brush/entity rendering | Many brush models, translucent pass inflation, repeated scratch uploads | `internal/renderer/world_gogpu_brush_render.go`, `internal/renderer/render_pass_parity.go` |
| Decals/particles | High late-translucency cost, many per-mark uploads, overdraw | `internal/renderer/world_gogpu_decal.go`, `internal/renderer/particle_gogpu.go` |
| Server/QC | High `Physics`, `StartFrame`, touch, think, or QC profile counts | `internal/server/physics_loop.go`, `internal/server/world.go`, `internal/qc` |
| Asset/loading | Spikes on map start, texture upload, skybox probes, BSP data conversion | `internal/fs`, `internal/bsp`, `internal/renderer/world_upload_gogpu.go` |

## Visual parity workflow

Human side-by-side review is authoritative. Pixel comparison is supporting
evidence and should not hide localized rendering defects.

Current screenshot harness commands:

```bash
mise run smoke-all
mise run parity-ref
mise run parity-go
PARITY_COMPARE_TOLERANCE=0 PARITY_MAX_MISMATCH_PERCENT=0 mise run parity-compare
```

The harness writes local working artifacts under:

```text
testdata/parity/reference/
testdata/parity/go/
testdata/parity/diff/
testdata/parity/overlay/
```

Do not treat these generated images as golden assets to commit casually.

### Current harness gap

`tools/parity_screenshots/main.go` requires
`testdata/parity/viewpoints.json`, but that file is not present in this
checkout. Restoring the generic viewpoint file and adding local qbj3 viewpoints
is a separate harness/data follow-up, not part of the current documentation-only
scope.

The Go capture side of the same harness currently invokes `ironwailgo
-screenshot`, but the GoGPU runtime implementation writes a deterministic
clear-color placeholder PNG instead of capturing the rendered swapchain. Until
that is fixed, use the harness for C reference coverage only and collect GoGPU
visual evidence with an explicitly documented external window capture method.

## Scene matrix

The matrix mixes canonical id1 scenes with qbj3-specific stress scenes. Exact
camera coordinates belong in local viewpoint artifacts or future reviewed test
data.

| ID | Category | Map | Setup | What to validate |
| --- | --- | --- | --- | --- |
| id1-lightmapped | Indoor lightmapped | `e1m1` | Start hallway | Lightmap quality, brush surfaces, texture alignment |
| id1-combat | Indoor combat | `e1m3` | First ogre room | Dynamic lights, alias models, particles |
| id1-sky | Outdoor sky | `e1m5` | Sky-visible outdoor area | Sky seams, sky scroll, world boundary behavior |
| id1-liquid | Liquid-heavy | `e1m4` | Bridge over slime | Liquid warp, transparency, edge sorting |
| id1-underwater | Underwater | `e2m2` | Submerged view | Waterwarp, content tint, underwater effects |
| id1-visibility | Occlusion/PVS | `e1m2` | Doorway transition | PVS correctness, no portal pop-in |
| id1-viewmodel | Viewmodel | `e1m1` | Fire shotgun | View bob, kick, muzzle flash, model placement |
| qbj3-overview | qbj3 dense overview | `qbj3_stickflip` | Highest-cost reported view | World batching, skybox, face count, frame time |
| qbj3-decal-depth | qbj3 decal/depth | `qbj3_stickflip` | Reported z-fighting surface | Decal depth bias, stencil, coplanar surface handling |
| qbj3-texture-depth | qbj3 texture/depth | `qbj3_stickflip` | Reported texture flicker site | Texture alignment, brush/world depth interaction |
| qbj3-trigger-path | qbj3 trigger path | `qbj3_stickflip` | Trigger that fails or flakes | Touch candidate discovery, callback order, QC mutation sync |
| qbj3-moving-brush | qbj3 moving brush | `qbj3_stickflip` | Moving platform/door/brush area | Pusher physics, brush entity draw order, collision hull parity |

## Known parity gaps to track

These are carried forward as live tracking categories, not as accepted
deviations.

| Area | Current concern | Next evidence |
| --- | --- | --- |
| Extended network protocol | Coordinate/angle precision and advanced parse paths must stay checked against modern Ironwail behavior | Protocol tests and demo/map repros that exercise extended flags |
| Temporary entities and effects | Explosions, beams, trails, particles, and sounds must match parse and render timing | Net debug logs plus screenshot/effect captures |
| Renderer pass ordering | GoGPU deliberately differs architecturally from OpenGL C Ironwail, so pass order must be verified by behavior | Render pass telemetry, scene matrix captures, z-fighting repros |
| Save/load and demo lifecycle | Some host/UI lifecycle features may lag C Ironwail | Command coverage and user-visible workflow tests |
| Audio spatialization/filtering | Oto mixer path is not a 1:1 C DMA mixer port | Focused underwater/spatial audio captures |
| Mod/addon management | Downloader and server-browser paths are separate from core renderer/server parity | Feature-specific tests rather than visual parity gates |

## Sign-off rules

For a scene or issue to move from "reported" to "verified":

1. Record C reference behavior and Go behavior with the same map, cvars, camera,
   and route.
2. Save commands and observations in the parity issue, PR, or this document.
3. Attach generated screenshots/logs only where project policy allows them.
4. Label root cause confidence as verified, suspected, or unknown.
5. Add or identify a regression test whenever the final fix changes engine
   behavior.

Accept deviations only when they are intentional, documented, and reviewed.
Document the affected scene, visible difference, metric evidence when available,
and the reason the difference is acceptable.
