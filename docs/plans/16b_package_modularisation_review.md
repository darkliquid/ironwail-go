# Implementation Plan 16b: Package Modularisation & Sub-Package Split (Approved + Deferred Roadmap)

> **Status: APPROVED** (2026-08-05). Implement per §5 (Steps 16.0–16.6).
> Companion to the earlier proposal in
> `docs/plans/16_package_modularisation_and_subpackage_split.md`. The read of
> 16 assumed *file moves* (e.g. `world_lightmap_gogpu.go` → `lightmap/`). This
> plan is the ground-truthed revision: **every proposed sub-package is
> method-rooted on the parent type (`*Renderer`, `*Game`, `*Server`)**, so a
> plain file move is **not** possible in Go; each extraction requires an
> API/interface seam first (see §2, §4). Ground truth was gathered by reading
> the symbols and call sites in the current tree; nothing has been changed yet.
>
> The **deferred/deep stage** is sketched in §10 (rough, subject to change
> based on the prior work of this plan) with the ultimate goal of fully
> isolated modules plugged into the parents via dependency injection, and
> tests ported out of the parent packages into the new subpackages.

---

## 1. Objective

Split the three monolithic packages (`internal/renderer` ≈ 40+ files,
`internal/game` ≈ 46 files, `internal/server` ≈ 25+ files) into cohesive
sub-packages **without changing any runtime behavior**, while:
- keeping every existing test green (esp. `world_aux_test.go`,
  `world_gogpu_decal_test.go`, `render_pass_parity_test.go`,
  `runtime_ui_test.go`, `game_loop_runtime_test.go`, `sv_client_test.go`,
  `signon_test.go`, `user_test.go`, `multiplayer_e2e_test.go`);
- respecting the 1,000-line ceiling (`TestProjectFilesUnderLineCeiling`);
- defining seams that the later test-isolation plan (17) and shim-removal
  plan (18) can build on.

## 2. Core constraint discovered (why this is a refactor, not a move)

Go methods can only be declared in the package that defines their receiver
type. Every module we want to extract is expressed as methods on a type in
the parent package:

- `*Renderer` fields on `internal/renderer/renderer_gogpu.go` (≈250 lines of
  gpu wgpu fields) — decal/particle/pipeline/lightmap methods read/write
  `r.*`.
- `*DrawContext` (same package) — `renderDecalMarksHAL`,
  `renderParticlesHAL`, `renderWorld*` are `DrawContext` methods.
- `*Game` (`internal/game/game.go`) — `runtime_ui.go`, `runtime_frame.go`,
  `runtime_overlay.go`, `runtime_csqc.go` are all `(*Game)` methods.
- `*Server` / `*Client` (`internal/server/server.go`) — `sv_client.go`,
  `sv_send.go`, `user.go`, `edict.go` are all `(*Server)`/`(*Client)` methods.

Consequences:

- A new `internal/renderer/lightmap/` package cannot call `r.update...Locked`
  because `r` is `*Renderer` from the parent package; Go has no cross-package
  method attachment. The only legal patterns are: **(a) keep the methods on
  `*Renderer`/`*Game`/`*Server` and move only leaf helpers/types**;
  **(b) convert receiver state into a sub-package-owned struct/interface**
  (the "state-object inversion"); or **(c) extract pure helpers that take
  explicit parameters.** All three are used below; (b) is the deep one and is
  the only way to realize 16's "move the whole module" intent, but it is also
  the highest-risk change (it mutates the type graph) and should be staged
  behind the safe (a)/(c) moves.

Because the parent types own the used state, the plan below does the **safe
file/type consolidation first** (no behavior change), then introduces
**state-object seams only for the two modules where the win justifies the
risk** (lightmap, decal — see §5), and **defers the deep `game/ui` and
`server/state` inversions** to a follow-up (16+) that first extracts the
`types`/protocol seams. This keeps risk bounded and every step gated by the
existing test suites.

## 3. Ground truth (current tree)

All symbol/call-site data below was verified against the working tree at
write time (git clean, `go build ./...` + renderer/server/game tests green).

### 3.1 `internal/renderer` (package renderer, ~40+ files, no build tags)

Sub-packages that already exist (don't re-move):
- `world/` (types: `WorldGeometry`, `WorldVertex`, `WorldFace`,
  `WorldLightmapPage` L66, `WorldLightmapSurface` L55; `lightmap_samples.go`
  = `.lit` RGB expansion; `surface/` inside? no — see below), `surface/`
  (LightmapAllocator skyline packer, already owns "lightmap atlas
  allocation" per its doc.go), `gogpu/` (input backend + `wasm_surface.go`
  is the only `//go:build js && wasm` file in renderer), `alias/`, `sky/`,
  `oit/`, `warpscale/`, `scrap/`.

Key coupling facts:
- `world_lightmap_gogpu.go` (461 lines, `package renderer`):
  - `uploadWorldLightmapArray` (L11) builds a **stacked 2D** replacement for
    texture-array (Vulkan gogpu WriteTexture bug), 1px padding per page.
  - Helpers `buildWorldLightmapPageRGBA` (L361),
    `compositeWorldLightmapSurfaceRGBA` (L235, fast paths 1/2/3),
    `lightmapDirtyBounds` (L112), `extractLightmapRegionRGBA` (L140),
    `updateUploadedLightmapsLocked` (L159), `clearDirtyFlags` (L423),
    `markDirtyLightmapPages` (L404), `recompositeDirtySurfaces` (L438),
    `worldLightstyleScale` (L214), `defaultWorldLightStyleValues` (L208) are
    **pure functions** over `WorldLightmapPage`/`WorldLightmapSurface` +
    `[256]float32` — safe to move to `internal/renderer/lightmap`.
  - Root-only symbols these need: `gpuWorldTexture` (`world.go` L212),
    `createWorldLightmapBindGroup` (`world_resources_gogpu.go` L580),
    `writeTextureChunked` (`world_resources_gogpu.go` L924),
    `worldLightmapPageSize` (`world_geometry_gogpu.go` L17).
  - `setGoGPUWorldLightStyleValues` (L207) writes `r.worldLightStyleValues`
    (renderer_gogpu.go L255) — the only `r.*` write, trivially extractable as
    a field setter + helper.
- `world_gogpu_decal.go` (446+ lines) + `decal_shared.go` (179) +
  `mark_system.go` (64):
  - `renderDecalMarksHAL` (L240) and `prepareGoGPUDecalHALDraws` (L352) are
    `*DrawContext` methods; `ensureDecalResourcesLocked` (L12) is `*Renderer`.
  - Decal GPU fields live on `Renderer` (renderer_gogpu.go L188, L336-341);
    `destroyDecalResourcesLocked` is in `world_gogpu_shadow_resources.go`
    L154. `render_pass_parity.go` consumes `[]DecalMarkEntity`
    (entity_types.go L83) and `renderer_gogpu_frame.go` L736 calls
    `dc.renderDecalMarksHAL`.
  - `decal_shared.go`'s `generateDecalAtlasData` (L13), `prepareDecalDraws`
    (L73), `buildDecalQuad` (L112), `buildDecalBasis` (L135) are pure —
    safe leaf extraction candidates, but they return/consume
    `DecalMarkEntity`, `CameraState`, `DecalPreparedMark` etc. (root types).
  - `mark_system.go` (`DecalMarkSystem`) is fully self-contained except the
    `DecalMarkEntity` root type.
- `particle.go` (706) + `particle_gogpu.go` (481):
  - `ParticleSystem` (L244) is a self-contained leaf; `particle_gogpu.go`
    has `ensureParticleResourcesLocked` (`*Renderer`, L148),
    `renderParticlesHAL` (`*DrawContext`, L354), and WGSL consts L20-111.
  - Particle GPU fields live on `Renderer` (L187, L330-335).
- `world_shaders_gogpu.go` (742) + `world_pipelines_gogpu.go` (513) +
  `world_compute_shaders_gogpu.go` + `world_cluster_compute_gogpu.go`:
  - `world_upload_gogpu.go` calls `createWorldShaderModule`,
    `createWorldPipeline/...Pipeline`, etc. and stores the results on
    `r.world*` (renderer_gogpu.go L194-269).
  - WGSL consts are plain strings (const/var) but also `var
    worldFragmentShaderWGSL = buildWorldFragmentShaderWGSL(...)` (L234) and
    `var worldTurbulentFragmentShaderWGSL` (L490).
- `renderer_gogpu.go` L1-250: the `Renderer` struct holds *all* wgpu field
  groups (alias, sprite, particle, decal, world, lightmap, shadow, sky,
  overlay, polyblend, char/color textures). This single struct is the
  root of all coupling.

### 3.2 `internal/game` (package game, 46 files)

- No `runtime/` subpackage yet. All files `package game`.
- `runtime_ui.go` (278): `runtimeGUIDimensions` (L12),
  `runtimeConsoleDimensions` (L30), `runtimeCanvasParams` (L57),
  `runtimeOverlayCanvasParams` (L74), `runtimeConsoleCanvasParams` (L78),
  `drawRuntimeConsole` (L89), `updateRuntimeConsoleSlide` (L106),
  `runtimeConsoleAnimating` (L135), `runtimeConsoleForcedUp` (L149),
  `runtimePauseActive` (L200), `drawMenuBackdrop` (L212),
  `drawRuntimeMenu` (L227), `drawChatInput` (L239), `clippedChatInput`
  (L256), `runtimeCursorGlyph` (L274). All `(*Game)` methods.
- `runtime_frame.go` (343): `RunRuntimeRendererLoop` (L47),
  `installRuntimeRendererCallbacks` (L93), `captureRuntimeRendererScreenshot`
  (L160), `drawRuntimeRendererFrame` (L235), `drawRuntimeOverlayFrame`
  (L254), `drawRuntimeFallbackFrame` (L323).
- `game_loop.go` (611): `gameCallbacks` (L25), `ProcessClient` (L108),
  `HeadlessGameLoop` (L363), `DedicatedGameLoop` (L391), `RunRuntimeFrame`
  (L492), `CaptureScreenshot` (L553), `drawLoadingPlaque` (L478).
- `game_visual.go` (823): `updateHUDFromServer` (L237), `applyRuntime*`,
  `syncRuntime*` — all `(*Game)`.
- `runtime_overlay.go` (467): `drawRuntimeClock` (L174), `drawRuntimeFPS`
  (L190), `drawRuntimeSpeed` (L236), `drawRuntimeNet` (L467) — all `(*Game)`.
- `runtime_csqc.go` (467+): `drawRuntimeHUDLayer` (L422), `buildCSQCDrawHooks`
  (L368), `buildCSQCFrameState` (L372) — all `(*Game)`; references
  `g.Draw`, `g.Client`, `g.Menu`, `g.Host.CVar`, `g.HUD`, `g.Input`,
  `g.ConsoleSlideFraction`.
- **Conclusion**: there is no meaningful leaf in `game`; everything is a
  `(*Game)` method. The only "safe" extraction is **file-level regrouping by
  domain within `package game`** (which does split the 1,000-line files while
  preserving attachability) — that is what this plan does for 16's
  `runtime`/`ui` targets, and the deep inversion to a real `game/runtime`
  package is deferred (16b+) because it would require exporting most of the
  `Game` struct or building a facade, i.e. a much larger diff.

### 3.3 `internal/server` (package server)

- Sub-packages already exist: `types/` (Edict L152, MessageBuffer L12,
  protocol signon stages, EntityState/UserCmd/StaticSound), `physics/`,
  `collision/`, `net/`, `debug/`, `savegame/`. Root `server.go` holds
  `Server` (L43) with `Static *ServerStatic` (L83-201) and `Client` (L204).
- `edict.go` (854, `package server`): `EntityManager` (L95) —
  `NewEntityManager` (L127), `ED_Alloc` (L142), `ED_Free` (L182),
  `ED_ClearEdict` (L227), `ED_ParseEdict` (L383), `fieldDef`/`fieldType`
  (L548-556), parse helpers `parseVec3` (L644)... All `(*EntityManager)`
  — a self-contained type, but it references `*qc.VM` and writes into
  `types.Edict`.
- `sv_client.go` (784, 34 `(*Server)` methods), `sv_send.go` (946, 18
  `(*Server)` incl. `writeEntitiesToClient` L787, `buildClientDatagram`
  L921), `user.go` (916, 21 `(*Server)` incl. `RunClients` L765).
- `ServerStatic` (L195) holds `Clients []*Client` (L198); `Client` (L204)
  holds `Edict *Edict`, `Message *MessageBuffer`, `SignonIdx SignonStage`,
  `EntityStates map[int]EntityState`. Signon stage types already live in
  `types/protocol.go`.
- `multiplayer_e2e_test.go` and the `signon_test.go`/`sv_client_test.go`
  suites pin this behavior.

## 4. Seam strategy used below (three patterns)

- **Pattern A — pure-leaf extraction (safe, no receiver change):** move
  self-contained functions/types that take explicit params and touch no
  parent state into the new package; parent package imports the new package.
  Used for: lightmap compositing/dirty/stacking helpers,
  `DecalMarkSystem`, particle-effect leaf helpers, WGSL const blocks.
- **Pattern B — parent-owned state stays; new package exposes helpers,
  parent keeps thin `(*Renderer)`/`(*Game)`/`(*Server)` wrappers:** the
  receiver methods become thin adapters calling into the new package, which
  owns the impl over an injected state struct/interface. Used for: lightmap
  array upload (needs `writeTextureChunked` + `createWorldLightmapBindGroup`
  — pass them as funcs or keep the calls in parent), decal HAL drawing
  (needs `DrawContext` wgpu view/pipeline access — passed as a small
  interface).
- **Pattern C — regroup files within the parent package (no package
  boundary, still namespaced by file/comment) to hit the 1,000-line ceiling
  and create the boundary the 16+2 inversion will cross later.** Used for:
  `game/runtime`+`game/ui` (this release), `server/state` (statement of
  intent only).

## 5. Step-by-step implementation sequence

> Ordering principle: **do safe moves first, verify with the existing suites
> after each step, and never land a step that changes behavior.** Each step
> below ends with "verify": `TMPDIR=.../go test ./internal/<pkg>/...
> -count=1` plus `go build ./...` and the line-ceiling test.

### Step 16.0: Baseline & duplicate-symbol scrub (precondition)
- Files: `internal/server/physics*.go`
- The prior agent read reported 14 gopls "duplicate decl" errors in
  `internal/server` (`walkMoveNeedsUnstick`, `PhysicsWalk`,
  `SV_CheckStuck`, `PhysicsToss` declared in both `physics.go` and
  `physics_walk.go`). **Verify** whether these are real (physical duplicate
  functions) or stale-LSP artifacts; if real, delete the duplicate set in
  one file (keep the split file, which `go build ./...` currently accepts)
  before anything else.
- Verify: `go build ./...`, `go test ./internal/server -count=1`.

### Step 16.1: Extract `internal/renderer/lightmap` (Patterns A+B)
- Files moved (pure, Pattern A): `buildWorldLightmapPageRGBA`,
  `compositeWorldLightmapSurfaceRGBA`, `lightmapDirtyBounds`,
  `extractLightmapRegionRGBA`, `clearDirtyFlags`, `markDirtyLightmapPages`,
  `recompositeDirtySurfaces`, `worldLightstyleScale`,
  `defaultWorldLightStyleValues`, `lightStylesChanged`,
  `anyLightStyleChanged`, and the pure `float32ToBytes`/`uint32SliceToBytes`
  (keep those two where they are if other files use them — check first).
- New package `internal/renderer/lightmap` exports:
  `CompositePageRGBA(page *world.WorldLightmapPage, values [256]float32)`,
  `DirtyBounds`, `ExtractRegionRGBA`, `ClearDirtyFlags`,
  `MarkDirtyLightmapPages`, `LightstyleScale`, `StackPages` (the
  vertical-stack builder extracted from `uploadWorldLightmapArray`'s loop);
  package imports `internal/renderer/world` for the page types.
- Pattern B: `(*Renderer).uploadWorldLightmapArray` keeps its signature but
  delegates the pure math to `lightmap.StackPages`; the texture/bind-group
  creation (root `writeTextureChunked` + `createWorldLightmapBindGroup`)
  stays in the parent (params passed in), so no `gpuWorldTexture` move.
- `worldLightmapPageSize` (world_geometry_gogpu.go L17): if used by the new
  package it must be passed as a param or promoted to an exported const in
  `world`; prefer passing as param to avoid a new export.
- Delete the moved helpers from `world_lightmap_gogpu.go`; add a `_test.go`
  in `renderer/lightmap` copying the case logic from `world_aux_test.go`
  L210-321 (see §7) so the behavior is pinned at the new home, and keep the
  old tests green.
- Verify: `go test ./internal/renderer/... ./internal/renderer/world/...
  ./internal/renderer/surface/...`, line-ceiling.

### Step 16.2: Extract `internal/renderer/decal` (Patterns A+B)
- Files moved (Pattern A): `mark_system.go`'s `DecalMarkSystem` +
  `timedDecalMark` (takes `DecalMarkEntity` as a parameter — define an
  interface `MarkEntity` in the new package and adapt, or keep
  `DecalMarkEntity` in root and pass it; prefer the interface to avoid a
  root import).
- `decal_shared.go`'s pure helpers (`generateDecalAtlasData`,
  `prepareDecalDraws`, `buildDecalQuad`, `buildDecalBasis`,
  `decalNormalize3`) are already partially duplicated in
  `world/gogpu/decal.go` (sub-package). **Unify**: move the root copies into
  `internal/renderer/decal` and make `world/gogpu/decal.go` import it (or
  vice-versa); keep exactly one implementation.
- Pattern B for `(*Renderer).ensureDecalResourcesLocked` and
  `(*DrawContext).renderDecalMarksHAL`:
  - new `internal/renderer/decal` package exposes
    `EnsureResourcesLocked(res *Resources)` and `RenderMarksHAL(ctx
    *DrawCtx, marks []MarkEntity)` where `Resources`/`DrawCtx` are small
    interfaces (device/queue/pipeline setter, render-pass attachment access)
    implemented by the parent types. This is a **behavior-preserving but
    wider API** — the parent keeps thin wrappers.
- Keep `renderer_gogpu_frame.go:736` calling `dc.renderDecalMarksHAL`
  (wrapper unchanged).
- Tests: keep `world_gogpu_decal_test.go` + `render_pass_parity_test.go`
  green; add `renderer/decal/decal_test.go` pinning `buildDecalQuad`/
  `buildDecalBasis` (case values from the existing tests).
- Verify: `go test ./internal/renderer/... ./internal/game -count=1`
  (game uses `DecalMarkSystem` via `NewDecalMarkSystem`).

### Step 16.3: Particle + pipeline/lightmap file consolidation (Pattern C within `package renderer`)
- Split `particle.go` (706) and `particle_gogpu.go` (481) topically under
  `package renderer` (e.g. `particle_system.go` leaf + `particle_gogpu.go`
  keeps HAL) to get each under ~1,000 lines and create the seam the later
  16+2 particle package will use; **do not change signatures**.
- Likewise `world_shaders_gogpu.go` (742) + `world_pipelines_gogpu.go`
  (513): leave in `package renderer` this release (they are the deepest
  wgpu-coupled code; the `pipeline/` extraction needs the
  `Renderer.wgpu*` field group moved first — that is 16+2). Only split if a
  file is >1,000 lines.
- Verify: `go test ./internal/renderer/...`, line-ceiling.

### Step 16.4: `game` file regrouping (Pattern C, `runtime`+`ui` intent)
- Regroup under `package game` toward the plan-16 `runtime/`+`ui/`
  boundaries:
  - `game_runtime_loop.go` ← `game_loop.go`'s loop/frame half +
    `runtime_frame.go` (all `(*Game)` methods; join the frame-loop + timing +
    screenshot + overlay-frame methods; keep `gameCallbacks` + `Process*`
    together).
  - `game_runtime_ui.go` ← `runtime_ui.go` + `drawLoadingPlaque` +
    `runtimeConsole*` (currently spread across `runtime_frame.go`,
    `game_loop.go`); re-exported names unchanged.
  - `game_runtime_overlay.go` ← `runtime_overlay.go` + HUD-layer glue from
    `runtime_csqc.go` (`drawRuntimeHUDLayer`).
- These are **pure file moves within `package game`** — attachability
  preserved, no API change; only import blocks and any intra-file helper
  references churn. The real `game/runtime`/`game/ui` packages are
  explicitly **deferred to 16+2** (needs `Game` facade).
- Verify: `go test ./internal/game -count=1` + line-ceiling.

### Step 16.5: `server` state/edict seam (Pattern C now, deep split deferred)
- **Do NOT split `sv_client.go`/`sv_send.go`/`user.go` into a new package**
  this release: they are 73 `(*Server)` methods over `Server`/`Client`
  root structs; the signon `types/protocol.go` seam exists but `Client`
  (server.go L204) still links `Edict`/`MessageBuffer`/`EntityStates`. A
  real `server/state` needs those types moved + a facade — that is 16+2 scope.
- This release: (a) confirm the `EntityManager` type in `edict.go` is the
  only consumer of `fieldDef`/`fieldType`/parse helpers and keep it in
  `package server` (it already is a self-contained unit — the payoff of
  moving it to `server/edict` is low vs. the import churn of
  `types.Edict`/`qc.VM`); (b) if any `server.go`/`server` root file is
  >1,000 lines after 16.0's scrub, split topically under `package server`
  (Pattern C).
- Verify: `go test ./internal/server/... ./internal/host/...
  ./internal/game -count=1` (+ `multiplayer_e2e_test.go`).

### Step 16.6: Docs & plan-16 drift correction
- Update `internal/renderer/doc.go` + `world/doc.go` + `surface/doc.go` to
  record the *new* boundary (lightmap compositing in `renderer/lightmap`,
  decal HAL in `renderer/decal`, allocator stays in `surface/`, `.lit`
  expansion stays in `world/`).
- Update `docs/plans/16_package_modularisation_and_subpackage_split.md` to
  mark: `renderer/pipeline` deferred (16+2), `game/runtime`,`game/ui`
  deferred to 16+2 (with the reason), `server/edict`,`server/state`
  deferred (16+2), so the plan text matches reality.

## 6. Risks & mitigations

| Risk | Mitigation |
| --- | --- |
| Method-rooting makes "extraction" require API change | Pattern A/B/C ordering; only B widens APIs, and only for lightmap/decal where tests are strong |
| Behavior drift in decal/lightmap math | Move pure helpers verbatim; pin with copied tests in the new package; keep old tests green simultaneously |
| Duplicate impl in `decal_shared.go` vs `world/gogpu/decal.go` | Step 16.2 unifies to one location |
| `game/ui` needs unexported `Game` fields | Defer deep split; do file regrouping only (16.4); facade is 16+2 |
| `server/state` circular types | Defer; document types/protocol seam as the 16+2 entry point |
| 1,000-line ceiling regressions | Run `TestProjectFilesUnderLineCeiling` every step |
| gopls stale diagnostics during refactor | Use `go build ./...`/`go vet` as ground truth, not LSP |
| 16.0 duplicate-decl issue | Verify real vs stale first; scrub before refactor baseline |

## 7. Verification & testing strategy

Every step ends with the existing suites:

```bash
go build ./...
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 \
  go test ./internal/renderer/... ./internal/server/... ./internal/game -count=1
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 \
  go test ./internal/testutil -run TestProjectFilesUnderLineCeiling -count=1
```

Plus targeted new tests at the new homes (each with the case values lifted
from the existing tests so the move is behavior-preserving):

- `renderer/lightmap/lightmap_test.go`: page RGBA compositing fast paths 1/2/3
  (from `world_aux_test.go` L210-321), dirty-bounds + region extraction,
  vertical stacking padding rows.
- `renderer/decal/decal_test.go`: `buildDecalQuad`/`buildDecalBasis`
  projections (from `world_gogpu_decal_test.go`), mark lifetime
  (`mark_system.go` `AddMark`/`Run`/`ActiveMarks` — currently only tested
  indirectly via `game_audio_visual_test.go`).
- Final gate: `mise run verify` (test+build+generate) and, if available,
  `mise run smoke-map-start`.

## 8. Deferred (16+2) — this release explicitly out of scope

- `internal/renderer/pipeline` (needs the `Renderer` wgpu field group split
  into a `*Resources` object first).
- `internal/renderer/particle` as a real package (Pattern C grouping only
  this release).
- `internal/game/runtime` and `internal/game/ui` as real packages (needs
  `Game` facade + exported accessors).
- `internal/server/edict` and `internal/server/state` (needs
  `types`/protocol snap first + `Client` decoupling).

## 11. How this plan composes with plans 16/17

- Implements the *safe* half of plan 16 in this release (16.0–16.6); the
  deferred half becomes 16+2 (sketched in §10).
- Leaves plan 17 (sub-package test isolation) with concrete targets: the new
  `renderer/lightmap` and `renderer/decal` packages land already carrying
  their pinned tests (16.1/16.2), which is the first step of 17; the
  `Benchmark*` zero-alloc work later builds on the injected-constructor
  seams from 16+2.

## 9. Review checklist (approved — decisions recorded)

1. **Approved** the three-pattern seam strategy (§4). Decal's
   `Resources`/`DrawCtx` interfaces (16.2) are acceptable API widening.
2. **Approved** deferring 16.4/16.5's deep splits to 16+2 (statement of
   intent only here) vs. attempting them now.
3. **Confirmed** `world_aux_test.go` lightmap cases may be copied to the new
   package (staying in both places) to pin the move.
4. **Confirmed** the `decal_shared.go` ↔ `world/gogpu/decal.go` unification
   direction: new `renderer/decal` is the single home.

## 10. Deferred deep stage (16+2): isolated module deep-dive (rough, subject to change)

> This is a *sketch*, refined as the prior work in this plan (16.0–16.6)
> lands. Ultimate goal: **separate modules plugged into the parent packages
> via dependency injection**, with the module's tests fully isolated from the
> parent.

### 10.1 Target end-state (per module)

| Module (new package) | Plugged into parent via | Owner object |
| --- | --- | --- |
| `internal/renderer/pipeline` | `Renderer` wgpu field group → `pipeline.Resources`; `Renderer` holds `*pipeline.Resources`, created via `pipeline.NewResources()` from injected `*gogpu.Context`/`*wgpu.Device` |
| `internal/renderer/lightmap` | `Renderer` keeps `worldLightmap*` fields; `lightmap.Uploader` injected with `device/queue/sampler` + `writeFunc` closure | `uploader` |
| `internal/renderer/decal` | `Renderer`/`DrawContext` keep thin method wrappers; `decal.Manager` injected with device/queue + `DrawContext` HAL interface | `manager` |
| `internal/renderer/particle` | `Renderer`/`DrawContext` wrappers; `particle.System` owns `ParticleSystem` + wgpu resources | `system` |
| `internal/game/runtime` | `Game` facade (exported accessors or interface) → `runtime.Manager` | `manager` |
| `internal/game/ui` | `Game` facade → `ui.Manager` | `manager` |
| `internal/server/edict` | `Server` keeps `*edict.Manager`; manager injected with `vm` + `*types.Edict` access | `manager` |
| `internal/server/state` | `Server`/`Client` keep their structs; `state.Manager` injected with types + `MessageBuffer` writer | `manager` |

### 10.2 Ordering & prerequisites (per module)

1. **renderer pipeline** — first: extract the `Renderer` wgpu field group
   (~250 lines in `renderer_gogpu.go`) into `pipeline.Resources`; the parent
   keeps thin `*Renderer` methods that delegate. Tests stay in the parent
   initially, then port.
2. **renderer lightmap** — after 16.1. Introduce `lightmap.Uploader`
   (injected device/queue/sampler + writeFunc) once the pure helpers are
   stable; move the last `r.*` write (`setGoGPUWorldLightStyleValues`) behind
   the uploader.
3. **renderer decal** — after 16.2. `decal.Manager` takes ownership of the
   `Renderer.decal*` fields + `DrawContext` HAL path; parent keeps wrappers
   for `renderer_gogpu_frame.go:736`.
4. **renderer particle** — after 16.3. `particle.System` owns
   `ParticleSystem` + the `Renderer.particle*` wgpu fields.
5. **game runtime** — needs `Game` facade/interface seam first (export the
   ~dozen methods `runtime_frame.go`/`game_loop.go` cross-call); then
   `runtime.Manager` is constructed by `Game.New()` and injected.
6. **game ui** — same prerequisite; `ui.Manager` is constructed/injected by
   `Game`.
7. **server edict** — `EntityManager` is already self-contained (precondition
   of 16.5). Introduce `edict.Manager` constructing `EntityManager` with
   `types.Edict` pool + `qc.VM` injection.
8. **server state** — needs the `types`/protocol snap first (signon stages
   already in `types/protocol.go`), then `Client` decoupled from root
   `server.go` so `state.Manager` can own signon/datagram/runclients without
   importing the parent.

### 10.3 Dependency injection mechanics

- **Constructor injection**: `NewX(deps interface{}) *X` with narrow
  interfaces (e.g. `deviceProvider`, `queueProvider`, `messageWriter`),
  implemented by the parent types. This is the "plug in" seam.
- **Field-group ownership transfer**: the parent struct loses the wgpu/cvar
  field groups; the module struct owns them and is reachable via a single
  exported field/method (`Renderer.Pipeline()`, `Game.Runtime()`,
  `Server.Edicts()`).
- **Thin adapter receivers stay in the parent** only where the method is part
  of a wider parent-owned contract (e.g. `FrameCallbacks`, `RenderContext`);
  these become one-line delegations (`r.pipeline.Upload(...)` etc.).

### 10.4 Porting tests to the subpackages (full isolation)

- **Pin-and-copy**: for each module, copy the *case values* of the parent
  tests into new `_test.go` files at the module (e.g.
  `renderer/lightmap/lightmap_test.go` from `world_aux_test.go` L210-321,
  `renderer/decal/decal_test.go` from `world_gogpu_decal_test.go`, game
  runtime/ui tests from `runtime_ui_test.go`/`game_loop_runtime_test.go`
  etc.). These operate purely on the module's exported API with injected
  stub deps (`stubDevice`, `stubQueue`, `stubMessageWriter`,
  `stubFrameCallbacks`).
- **Ownership moves**: once the ported tests cover the same behavior, delete
  the parent-origin tests that only exercised the moved code (or reduce the
  parent test to a thin integration smoke that constructs the real parent and
  calls the module through the injection seam). Goal: the module package
  should run `go test ./internal/renderer/lightmap/...` etc. with **zero
  dependency on the parent package**.
- **Injection-test seams**: add `export_test.go` files in each new package
  exposing constructor parameters for tests so stubs can be supplied
  directly, keeping the public API minimal.
- **Behavior-parity harness**: keep a single *integration* test in the parent
  per module that wires the real parent to the real module and asserts the
  same cases end-to-end. This is the only parent-side test that remains
  module-specific.

### 10.5 Anti-goals / constraints

- Do not change runtime behavior in this stage either; every module is
  behavior-preserving, only ownership/API moves.
- Do not attempt `game/ui`/`game/runtime` facade before `server/edict`:
  server-side types are the deepest dependency chain, and getting those
  modules first proves the injection pattern on the most constrained domain.
- Keep the 1,000-line ceiling and full `go test ./...` green at every step;
  run `TestProjectFilesUnderLineCeiling` per module landing.
- `pipeline` module must remain buildable on all platforms (no build tags,
  only `wasm_surface.go` is tagged); the injection seam must not introduce a
  platform split.
