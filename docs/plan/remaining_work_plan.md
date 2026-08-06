# Subpackage Decoupling — Remaining Work Plan

Supersedes the archived plan documents (`archival_*` — subpackage_architecture_spec,
subpackage_migration_plan, deep_subpackage_decomposition_plan,
architectural_decomposition_plan, aggressive_subpackage_plan) and consolidates
`refactor_plan_v2.md`'s completed work into a single forward-looking plan.

**Status: 2026-08-06.** Execution-order items 2-6 and 9 are complete (see §9):
renderer resource creation / frame-pass leaf sweeps, game CSQC image helpers,
game camera viewcalc, the server clientdata encoder, and the overlay-math
sweep are extracted behind seams. server_qc_sync no-op pruning remains
deferred as optional polish. The game and renderer roots retain their
facade/orchestration layers. This document is the checklist for finishing the
job.

---

## 1. Proven recipe (from 14 completed extractions)

Every successful extraction followed the same shape. Use it for everything
remaining. Do **not** attempt to "move files" — root methods cannot move into
subpackages (Go circular-import rule: a subpackage cannot import its parent).

1. **Move argument types into `internal/<pkg>/types`** (or a sibling package)
   so interfaces can reference them without importing the parent. *Done for
   `Client`, `Edict`, `EntityState`, `MessageBuffer`, `UserCmd`.*
2. **Export the private method** on the parent struct (e.g. `playerClient` →
   `PlayerClient`) if the subpackage needs to call it.
3. **Add a narrow interface seam** in `types` covering exactly what the moved
   code reads: `PhysicsFacade`, `MoveConfig`, `CVarReader`, `TelemetrySink`,
   `ClientThinker`, `FrameDriver`, `QCCallback`. Prefer plain params for pure
   functions (no seam at all — see net encoding, BSP builders).
4. **Move the logic** into the target subpackage as exported functions or a
   component (`physics.System`, `net`, `collision`, `game/camera`,
   `game/audio`).
5. **Root keeps a one-line delegator** with the same name, so call sites and
   tests compile unchanged.
6. **Verify parity**: the existing integration/parity tests now run *through
   the delegator* — they are the behavioral oracle. Add an isolated mock test
   in the subpackage for each extraction.
7. **Gate every step**: `mise run verify` + full `go test ./...`; only the two
   pre-existing failures may remain (demo parity missing `progs.dat`, quakego
   line ceiling).

### Seam precedence (least to most invasive)

| Kind | Example | When |
| :--- | :--- | :--- |
| Pure function, zero deps | `EncodeScale`, `PointInTreeLeaf` | Extract freely; plain params. |
| Pure + one getter | `writeEntityUpdate` (needs `ProtocolFlags`) | Pass the value as a param. |
| Read-only state | `WriteSpawnClientRoster` (`[]*Client` + handle) | Pass data + `ServerHandle`. |
| Orchestration (QC/sockets/telemetry) | `RunClients`, `WriteClientDataToMessage` | Define a narrow seam in `types`; keep 1:1 delegators. |
| Struct definition / constructor | `Server`, `game_init.go` | **Do not extract** — facade owns these. |

---

## 2. Current state (ground truth, lines = non-test root files)

| Package | Root lines | Remaining big files | Subpackages (real, wired) |
| :--- | :--- | :--- | :--- |
| `internal/server` | 7,730 | `server.go` 977, `server_net_main.go` 791, `sv_client.go` 733, `server_net_send.go` 704, `server_user_commands.go` 542, `debug_telemetry.go` 460, `server_runtime.go` 406, `user_spawn.go` 391, `world.go` 272, `server_qc_sync.go` 259 | `collision`, `commands`, `debug`, `edict`, `net`, `physics`, `qc`, `savegame`, `state`, `types` |
| `internal/game` | 8,610 | `game_init.go` 907, `game_entity.go` 823, `game_commands.go` 750, `game_input.go` 635, `game_loop.go` 610, `game_runtime_overlay.go` 515, `game_camera.go` 515 | `audio` (new), `camera` (live), `id1`, `ui` |
| `internal/renderer` | 19,876 | `renderer_gogpu_world_resources.go` 930, `renderer_gogpu_frame.go` 900, `renderer_gogpu_world_alias.go` 875, `renderer_gogpu_world_brush_render.go` 795, `renderer_gogpu_world_render.go` 782, `diag_atlas.go` 764, `renderer_gogpu_world_geometry.go` 759 | `alias`, `decal`, `gogpu`, `lightmap`, `oit`, `overlay`, `particle`, `pipeline`, `scrap`, `sky`, `surface`, `warpscale`, `world`(+`gogpu`) |

---

## 3. Remaining work — `internal/server`

The physics and net *wire encoding* are extracted. What remains is genuinely
orchestration-coupled; extract only where the seam cost is justified.

### 3.1 `WriteClientDataToMessage` (`server_net_send.go:246`, ~240 lines) — HIGH value, MEDIUM risk
Client-data delta encoder (`SVCClientData` bit packing). Deps (all
interface-able): `ProtocolFlags`, `Protocol` (params), `SetIdealPitch`,
`standardQuakeWeaponEncoding` (export + param), `FindModel`/`String`
(precache lookup seam), `DebugTelemetry` (`TelemetrySink`, already exists).
Move to `net` as `WriteClientData` with a small `PrecacheReader` seam; root
keeps the delegator. The `sv_send_clientdata_test.go` parity tests are the
oracle.

### 3.2 `RunClients` / `DropClient` (`server_user_commands.go:734`, ~150 lines) — MEDIUM value, HIGH risk
Client input/signon loop. Deps: `s.Net` (socket), `SV_ReadClientMessage`,
`handleDeathmatchRespawn`, broadcast helpers. Deep socket coupling — the seam
would be large (`MessageSource`/`MessageSink`). **Recommendation: leave in
root** unless 3.1 proves the seam pattern pays off here; it is the server's
session orchestrator, not portable logic.

### 3.3 `updateClientStats` / `SV_WriteStats` (`server_net_send.go:852`) — LOW value
Reads `s.QCVM` globals + `FindModel`/`String`. Migration requires a
`GlobalStatReader` seam for little isolation gain (stats encoding is a small,
stable surface). Defer; revisit only if `sv_stats` changes churn.

### 3.4 `user_spawn.go` spawn/QC lifecycle (`runClientSpawnQC`,
`initClientSpawnFallback`, `runClientQCFunction`, ~200 lines) — LOW-MEDIUM value
QC-called-by-name dispatch (`FindFunction` → `SetGlobal` → `ExecuteFunction`)
is already expressible via `QCCallback`/`ThinkExecutor` seams. Candidate for a
`commands` or `qc` subpackage move, but each function is small and the shared
`clientMoveContext`-style glue is thin. Pick off the three `runClient*QC`
helpers if a clean seam emerges; do not force it.

### 3.5 `server_qc_sync.go` (`syncEdictToQCVM`/`syncQCVMState`, 259 lines)
`syncEdictToQCVM` and `syncEdictFromQCVM` are **empty no-ops** (accessor-based
dual-write made them dead; kept for call-site compatibility). Deleting them
means removing ~9 call sites in `server.go` (the VM-mirroring hooks in the
QC builtins) — do this only as a deliberate cleanup pass with `sv_debug_qc_trace`
verification, not as part of a functional change. The live remainder
(`SyncQCVMGlobals`, `SetQCTimeGlobal`, `ensureQCVMEdictStorage`,
`EdictDefaultOffsets`, `worldLeafIndex`, `newCheckClient`) stays. Net gain is
small (~40 lines); treat as optional polish.

---

## 4. Remaining work — `internal/game`

The game root is the most tractable target: its subpackages (`camera`,
`audio`, `ui`) are live, and its remaining files are dominated by *wiring*
(`game_init.go`), *state accessors* (`game_entity.go`), and *command
registration* (`game_commands.go`) — mostly facade responsibilities.

### 4.1 `game_runtime_csqc.go` (515 lines) — HIGH value, LOW-MEDIUM risk
CSQC VM event dispatch + HUD draw hooks. Contains pure image helpers
(`nearestPaletteIndex`, `clipCSQCDrawRect`, `subPicFromNormalizedRect`,
`scaleQPic`, `buildCSQCFrameState`) with zero `g.` deps — extract these to a
`game/csqc` subpackage first (pure-first). The `drawRuntimeCSQCHUD` /
`buildCSQCDrawHooks` orchestration needs a small `CSQCState` seam (the
`cl.Client` + `qc.CSQC` pair) — defer or extract behind it.

### 4.2 `game_runtime_overlay.go` (515 lines) — MEDIUM value
Overlay render triggers reading `g.Host`, `g.Renderer`, `g.HUD`. Mostly
facade glue. Extract any pure dimension/canvas math
(`runtimeCanvasParams`-style pure helpers) into `game/ui` (already exists);
leave the frame wiring in root.

### 4.3 `game_camera.go` (515) & `game_camera_viewcalc.go` (396) — MEDIUM value
Continue the camera extraction: `viewCalcBob`/`viewCalcRoll`/`viewAddIdle`/
`viewApplyViewmodelQuakeFudge` read only cvars (`g.Host.CVar.Get...Float`) —
portable behind the existing `CVarReader` seam into `game/camera`. The
`viewCalcGunAngle`/`viewStairSmoothOffset` state machines share `viewCalcState`
(which references `cl.Client`) — move `viewCalcState` into `game/camera` and
parameterize. This is the largest remaining *logic* (not wiring) chunk in game.

### 4.4 `game_commands.go` / `game_input.go` (750 / 635) — LOW value
Command handlers and input dispatch are `*Game` methods touching
`g.Input`/`g.Client`/`g.Host` pervasively — the game package's
`RunClients`-equivalent (session glue). Leave in root.

### 4.5 `game_entity.go` (823) — LOW value
Entity accessors / cache lookups on `*Game` — facade state. Leave.

### 4.6 `game_init.go` (907) — NOT extractable
Central cvar registration + wiring. This is the game constructor. Leave.

---

## 5. Remaining work — `internal/renderer`

The renderer subpackages are already real and wired (unlike server's original
stubs). Every `renderer_gogpu_world_*` file is GPU-pipeline state living on
`*Renderer` (126 fields). The pattern that applies here is **state
value-structs** (group fields → subpackage component), not pure-function moves.

### 5.1 `renderer_gogpu_frame.go` (900) — HIGH value, MEDIUM risk
Frame orchestration (`DrawFrame`, render-pass sequencing). Move the
render-pass *encoder* logic (the `DrawContext` methods that build/execute GPU
passes) into `world/gogpu` or a new `renderer/pass` component, taking the
already-exported `WorldPipeline`/`AliasBatcher`/`Compositor2D` interfaces
(`renderer/interfaces.go`) + a `GPUContext`. `DrawContext` (15 fields) is the
natural seam — it already exists; promote it to the subpackage boundary.

### 5.2 `renderer_gogpu_world_resources.go` (930) — MEDIUM value
Buffer/texture creation helper soup. The creation helpers
(`createWorldBuffer`-style) are pure-`wgpu`-device code — extract to
`world/gogpu` as exported functions taking `*wgpu.Device`/`Queue` (no `*Renderer`
needed). Same move as server's BSP builders, and just as clean. Start here —
it is the purest remaining renderer chunk.

### 5.3 `renderer_gogpu_world_alias.go` (875) / `world_brush_render.go` (795) /
`world_render.go` (782) / `world_geometry.go` (759) / `world_translucent.go` —
MEDIUM value each
All follow 5.2's shape: the leaf math (vertex builders, batch sort keys,
translucent sort comparators) is pure; the `r.*` field access is the coupling.
Extract pure leafs to `world/gogpu` per-file, keeping `*Renderer` methods as
delegators. Freeze the `world` public API (externally consumed by `game`).

### 5.4 `diag_atlas.go` (764) — LOW value
`bspdiag`-style offline diagnostics; standalone (uses `os`/`strings`). Option:
move to `cmd/bspdiag` or `internal/renderer/diag` wholesale. Not part of the
hot path; extract only to reduce root noise.

### 5.5 Renderer struct-field grouping (the real prize)
`Renderer` has 126 flat fields. Group into embedded value structs by domain
(world state, alias state, overlay state, particle state) and pass those
structs by value into the subpackage components. This is the architectural
decomposition the original docs envisioned — but done as *state grouping*
(portable) rather than *method moving* (impossible). Do this **after** the
pure-leaf sweeps (5.1-5.3) so the seams are visible.

---

## 6. Recommended execution order (value ÷ risk)

| Order | Work | Root reduction | Risk |
| :--- | :--- | :--- | :--- |
| 1 | Server 3.5: prune dead `server_qc_sync.go` no-ops | ~180 | Trivial |
| 2 | Renderer 5.2: pure buffer/texture creation → `world/gogpu` | ~300-400 | Low |
| 3 | Game 4.1: pure CSQC image helpers → `game/csqc` | ~150 | Low |
| 4 | Game 4.3: cvar-only camera viewcalc → `game/camera` | ~250 | Low-Med |
| 5 | Server 3.1: `WriteClientDataToMessage` → `net` (precache seam) | ~200 | Med |
| 6 | Renderer 5.1: frame render-pass encoders → subpackage | ~300 | Med |
| 7 | Renderer 5.3: per-file pure leaf sweeps | ~500+ | Med |
| 8 | Renderer 5.5: struct-field grouping | n/a (architectural) | Med-High |
| 9 | Server 3.4 / Game 4.2: QC-helper / overlay-math sweeps | ~200 | Med |
| 10 | Server 3.2, Game 4.4/4.5, Renderer 5.4: leave in root | 0 | — |

**Stop rule:** after #7, remaining root code is facade/service wiring
(`server.go`, `game_init.go`, `game_commands.go`, `sv_client.go` session loop,
renderer frame glue). That is the correct end-state — 3-6 facade files per
package plus subpackages, not zero.

---

## 7. Verification gates (unchanged from v2)

- Every sub-step: `mise run verify` (test + build) + `go test ./...`.
- Only the two pre-existing failures may remain: `TestDemoStateParity` (missing
  `progs.dat` artifact) and `TestProjectFilesUnderLineCeiling` (the separate
  `pkg/qgo/quakego` module — out of scope, its own go.mod).
- Parity: each extraction must leave the existing integration tests running
  *through the delegator* unchanged. Add one isolated mock test per move.
- Physical file moves (`git mv`) only after the delegation is green, so every
  stage is a pure relocation with no behavior change.
- Do not commit generated logs/screenshots/profiles (`docs/PARITY.md` note).

## 8. Key risks (carried forward)

1. **Circular imports** — a subpackage can never import its parent. If a
   function needs `*Server`/`*Game`/`*Renderer`, it stays in root or receives a
   callback/interface.
2. **Typed-nil interface trap** — a getter returning a nil concrete type inside
   an interface is non-nil; check untyped-nil in delegators (hit in the cvar
   seam; fixed in `interfaces.go`).
3. **VM-accessor field offsets** — tests must set `vm.EdictSize`/`NumEdicts`
   large enough (e.g. 512) for high field offsets (`EntFieldWaterLevel`=83).
4. **`renderer/world` is externally consumed** (by `internal/game`) — freeze
   its public API during renderer work.
5. **Parity regression** — every deleted root method needs a live delegator
   producing identical behavior. The integration tests are the oracle; never
   delete a root copy before the delegator is wired and green.

## 9. Completed (2026-08-06)

| Item | Work | Result |
| :--- | :--- | :--- |
| 1 | Server 3.5 no-op pruning | **Deferred** — `syncEdictToQCVM`/`syncEdictFromQCVM` no-ops have ~70 call sites (most in tests documenting intent) for ~40-line gain. Not worth the churn. |
| 2 | Renderer 5.2 | Pure buffer/texture/sampler creation moved to `world/gogpu/resources.go` (`CreateWorldVertexBuffer`, `CreateWorldIndexBuffer`, `CreateWorldSolidTexture[Array]`, `CreateWorldWhiteTexture`, three sampler creators, `WriteTextureChunked`, `WorldLightmapFallbackView`). Root keeps 1:1 delegators; added `resources_test.go`. Root reduction: ~930 → 596 lines. |
| 3 | Game 4.1 | Pure CSQC image/rect helpers moved to new `game/csqc` package (`NearestPaletteIndex`, `ClipDrawRect`, `SubPicFromNormalizedRect`, `ScaleQPic`, `PreparePic`). Root keeps delegators for the ones with live call sites; dead ones deleted. Added `csqc_test.go`. Root reduction: 515 → 365 lines. |
| 4 | Game 4.3 | Cvar-only viewcalc moved to `game/camera` (`CalcBob`, `CalcRoll`, `AddIdle`, `ApplyViewmodelQuakeFudge`) behind a new local `CVarReader` seam (implemented by `*cvar.CVarSystem`). Stateful `viewCalcGunAngle`/`viewApplyDamageKick`/`viewStairSmoothOffset` stay in root (reference `viewCalcState` with `*cl.Client` latch). Added `viewcalc_test.go`. Root reduction: 396 → 258 lines. |
| 5 | Server 3.1 | `WriteClientDataToMessage` bit-packing moved to `net.WriteClientData` (`net/clientdata.go`) behind `ClientDataDeps{Handle, Precacher, Logger, SetIdealPitch, EdictNum, NumForEdict, Protocol, StandardQuakeWeaponEncoding, Flags}` seams. Added `clientdata_test.go` with a mock `ServerHandle`. Root reduction: 704 → 482 lines. |

Net effect: ~626 root lines relocated into subpackages; all parity/integration
tests run through the delegators unchanged; `go build ./...` + `go test ./...`
green except the two pre-existing failures (`TestDemoStateParity` missing
`progs.dat`, `TestProjectFilesUnderLineCeiling` quakego module).

Follow-up sweep (2026-08-06, continuation):
- Renderer 5.3 alias packing: `AppendAliasSceneUniformBytes`, `AliasVertexBytes[Into]`,
  `AppendAliasVertexBytes` → `world/gogpu/aliasbytes.go` (+ `PutFloat32s`, stride/align
  consts). Root `renderer_gogpu_world_alias.go` keeps delegators; dropped the duplicate
  48-byte packer const/comment from `world_pipeline.go`.
- Renderer 5.3 vertex packing: `AppendVertexBytes`/`AppendIndexBytes` → `world/gogpu/buffer.go`;
  root `appendGoGPUWorld[Vertex|Index]Bytes` delegate; removed root-only `putGoGPUFloat32Slice`
  and `goGPUWorldVertexStrideBytes`.
- Renderer 5.3 BSP texture-table helpers: `FaceTexInfo`, `TextureCount`, `TextureEntryLoaded`,
  `MissingTextureFallbackIndex`, `TextureDimensions`, `FaceTextureIndex`, `FaceFlags`,
  `TexCoordDouble` → `world/texture.go`. Root `world_geometry.go` delegates (unused delegators
  dropped). Added `texture_helpers_test.go`.
- Game 4.2 overlay math: `ClampF64`, `DemoName`, `FormatDemoBaseSpeed` → `game/ui` (+ tests);
  root `game_runtime_overlay.go` delegates.
- Server 3.4 (runClient\*QC helpers) and Renderer 5.1 (frame pass *encoders*, beyond the
  already-extracted `render_pass_parity.go` decision layer) remain deferred: the QC helpers
  are thin `*Server` glue with no clean seam, and the frame pass encoders are `DrawContext`/
  HAL orchestration.

Remaining (value-ordered): Renderer 5.5 struct-field grouping, then optional 9 (QC-helper
sweeps). Leave-in-root items (3.2, 4.4/4.5, 5.4) stand as recommended.
