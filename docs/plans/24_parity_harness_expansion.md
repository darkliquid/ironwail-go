# Implementation Plan 24: Parity Harness Expansion — Six Deterministic Gates

**Priority**: #1 (Parity tooling; complements plan 23's fixes)
**Status**: IN PROGRESS (2026-08-08) — H2/H4/H5 landed (541f402); H1/H3/H6 opt-in remain
H4 StepFrame interleave fuzz, H5 save/load frame-evolution round-trip landed
(all asset-free, pass in `go test ./internal/server/...`). H1 (C dumpstate
schema), H3 (render-record hash), H6 (message-stream recorder) remain
(opt-in, need C binary/data).
**Prerequisite**: plan 23 D1-D6 fixes green; `tools/parity_screenshots` +
`tools/parity_generator` already in tree.
**Estimated effort**: 4-6 focused sessions

---

## 1. Executive Summary & Architectural Context

The current parity tooling (screenshot viewpoints with SSIM/regions; C
`-dumpstate` timedemo frame dumps replayed by `internal/game/parity_test.go`)
is real but has three gaps identified in research:

1. **The C dump schema is minimal** — per-visdict only origin/angles/model, so
   entity-slot reuse (proven in qbj2) and full-state drift are invisible to the
   Go replay.
2. **Demo dependence** — `demo1.dem` is absent from Quake Enhanced data, so
   `TestDemoStateParity` skips on the reference dataset; there is no way to
   generate reference state without a demo.
3. **No deterministic, asset-free way to prove *ordering* invariants** (the
   four intermittent symptoms are all ordering/timing races).

This plan adds six harness components (H1–H6), each landable independently,
that convert parity from a manual sweep into CI-able, deterministic gates.

## 2. Harness Component List

| # | Name | Validates | Key design | In-tree base |
| --- | --- | --- | --- | --- |
| H1 | Extended C dumpstate schema | entity full state, think cadence, slot reuse | patch C `-dumpstate` (json.c) via `tools/parity_generator/c_patch/`; emit modelindex/frame/skin/velocity/nextthink/sendinterval per visedict | `tools/parity_generator`, `internal/game/parity_test.go` |
| H2 | Narrative-chain truth table | ordered gameplay chain (door) | one test walking spawn→touch→use→fire→move→think asserting EVERY hop (edict#, accessor fields, order) on synthetic world + real QuakeGo progs | `internal/server/twindoor_targetname_regression_test.go`, `synthetic_bsp_helper.go` |
| H3 | Render-record hash | screenshot-less pixel parity | deterministic sorted `(modelindex, frame, skin, alpha, lerpfinish, quantized origin/angles)` record + viewleaf + view-matrix hash, both engines, headless | `internal/game/parity_test.go`, `tools/parity_generator` |
| H4 | StepFrame interleave fuzz | timing/interleaving races | random pusher velocity / trigger touches / force_retouch; per-frame invariants (ltime monotonic, scratch retention, FL_ONGROUND/groundentity coherence) | `internal/server/physics` mocks |
| H5 | Save/load byte round-trip | save-game parity | save→restore→120 frames→compare edict streams | `internal/server/savegame`, `testutil.CompareStructs` |
| H6 | C↔Go message-stream recorder | net codec bit-exactness | replay-DSL applied to both engines; dump `svc_*` streams; diff bytes (allowing documented packetization deltas) | `internal/server/message.go`, `internal/common.SizeBuf` |

## 3. Step-by-Step Implementation Sequence

### Step 24.1: H1 — extend C `-dumpstate` schema
- **Files**: `tools/parity_generator/c_patch/` (new dir; context-diff patch),
  `tools/parity_generator/main.go` (pass `-dumpstate-v2`), 
  `internal/game/parity_test.go` (`RefFrameState` extension).
- **Actions**: patch `ironwail/Quake/json.c`/`gl_rmain.c` dump sites to add, per
  visual edict: `modelindex, frame, skin, effects, velocity, solid, movetype,
  ltime, nextthink, sendinterval`; plus per-frame `sv.time, gravity, maxclients,
  force_retouch`. Keep the patch additive + `#ifdef COMPILED_DUMPSTATE` gated so
  upstream drift is contained. Extend `RefFrameState` and compare the new fields
  with per-field tolerances.
- **Verify**: regenerate `reference_<demo>_state.json` for demo1 on a full
  install; Go replay now catches slot-reuse and cadence drift that origin-only
  misses. Deterministic: same demo → same dump.

### Step 24.2: H2 — narrative-chain truth table (door chain)
- **Files**: `internal/server/parity_narrative_chain_test.go` (new).
- **Actions**: build a synthetic map with `trigger_multiple` → `func_door`
  chain (real QuakeGo progs compiled via `mise run build-progs`), then assert
  the exact ordered sequence via telemetry events the server already emits
  (`DebugEventTouch/Think/Physics` in `internal/server/debug/telemetry.go`):
  `spawn → player touches trigger → trigger_multiple touch → multi_trigger →
  SUB_UseTargets → door_use → door_fire → SUB_CalcMove (velocity+nextthink) →
  PushMove advances ltime → nextthink fires → door_go_up`. Every hop checks
  edict# and accessor field values.
- **Verify**: test fails if any hop is skipped/misordered/repeated (would have
  caught D1 instantly).

### Step 24.3: H3 — render-record hash
- **Files**: `tools/parity_records` (new cmd), `internal/game/parity_record_test.go`.
- **Actions**: for each viewpoint in `testdata/parity/viewpoints.json`, compute
  the render record on BOTH engines: Go via the runtime recorder hook
  (`internal/game` read-side; no screenshot needed); C via the H1 dumpstate
  extension. LZMA/XOR-hash each record; parity = record equality within
  quantization (no image I/O, no window capture, no tolerance tuning).
- **Verify**: id1 matrix hashes match C within quantization; a deliberate
  entity-slot shuffle fails the hash (proves it catches D-class drift).

### Step 24.4: H4 — StepFrame interleave fuzz
- **Files**: `internal/server/physics/stepframe_fuzz_test.go` (new), mocks
  from `clientmove_test.go`/`system_test.go`.
- **Actions**: random drives of pusher velocity/trigger touches/force_retouch
  across N frames; per-frame invariants:
  - `nextthink` never set while `think==0`;
  - pusher `ltime` monotonic unless blocked, and restores on block;
  - `PushMoveScratch` arrays empty at entry (no stale origins across calls);
  - no entity left `FL_ONGROUND` with `groundentity==0` post-restore;
  - movered entities' origins identical to C semantics (blocked restores).
- **Verify**: run with `-count=1` under the physics package; deterministic seed
  for regressions (seed printed on failure).

### Step 24.5: H5 — save/load byte round-trip
- **Files**: `internal/server/savegame/*_test.go` (new).
- **Actions**: boot synthetic world + progs, run 120 frames, `CaptureSaveGameState`,
  restore, run 120 frames, compare edict streams via `testutil.CompareStructs`
  (mismatches as hex dumps). Add a second flavor parsing a C savegame text
  fixture (H6-adjacent) to catch cross-reader drift.
- **Verify**: round-trip produces byte-identical streams; C fixture parses and
  round-trips.

### Step 24.6: H6 — C↔Go message-stream recorder
- **Files**: `tools/parity_recorder` (new cmd), `internal/server/*_test.go`.
- **Actions**: replay-DSL (scripted `+commands`: move/attack/use) applied to
  both engines with identical cvars; dump every `svc_*` message written
  (Go: `MessageBuffer` contents; C: hook the net message flush in sv_main.c via
  the H1-style patch). Diff the byte streams allowing documented
  packetization deltas (datagram boundaries) but NOT field-encoding deltas.
- **Verify**: id1 spawn → 60 frames → `svc_time`/`svc_clientdata`/`svc_sound`
  streams match modulo documented packetization.

### Step 24.7: mise wiring + CI task
- **Files**: `mise.toml` (new tasks `parity-gates`, `parity-fuzz`,
  `parity-record`, `parity-savegame`).
- **Actions**: H2/H4/H5 run asset-free in `go test ./internal/server/...`
  (no pak needed); H1/H3/H6 are opt-in (need C binary + data) behind
  `PARITY_GENERATOR=1`-style env guards like existing asset tests.

## 4. Verification & Testing Strategy
1. Every component has a failing-first probe (red/green).
2. H2/H4/H5 join `mise run test` (asset-free). H1/H3/H6 run via
   `mise run parity-gates` when QUAKE_DIR + IRONWAIL_BIN are present.
3. H3 explicitly proves the goto regression: a synthetic slot-reuse shuffle
   must fail the hash.

## 5. Risks & Mitigations
| Risk | Mitigation |
| --- | --- |
| H1 C-patch fork drift | additive `#ifdef COMPILED_DUMPSTATE` patch in `tools/parity_generator/c_patch/`; never merge upstream |
| H5 cross-reader savegame format differs deliberately | fix tolerances to documented deltas; keep byte-identical assertions only for Go↔Go round-trip |
| H6 packetization nondeterminism | diff field-encodings (svc_ sequence), not datagram boundaries |
| Fuzz flakiness | fixed seed + invariant-only asserts (no timing), print seed on failure |
