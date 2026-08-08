# Implementation Plan 23: Parity Hardening — Behavior Divergences C vs Go

**Priority**: #1 (Parity is the project's primary oracle)
**Status**: IN PROGRESS (2026-08-08) — D1 fixed earlier; D4 fixed (inactive-slots gate
+ C-cited test); D3 NaN warnings landed; D2 verified-equal; D6 documented in
PARITY.md; D8 verified by existing tests + doc note. D5 (renderer audit) and
D7 (docs, folded into plan 26) remain.
**Prerequisite**: stable baseline (all tests pass); includes a **landed fix** from
research (`internal/server/physics/leafs.go` pusher think-gate) already in tree.
**Estimated effort**: 4-7 focused sessions

---

## 1. Executive Summary & Architectural Context

Research (`docs/plans/22_*` sibling report + `$HOME/.local/share/crush/research/
ironwail-go-parity/report.md`) cataloged the remaining behavioral divergences
between C Ironwail (`ironwail/Quake/*.c`) and Go. The strongest one — the pusher
think-gate race — is **fixed in-tree** (see §4, D1). This plan hardens the rest,
each with a C-cited red/green test, and expands the parity tooling to turn the
current manual sweep into deterministic, CI-able gates.

## 2. Catalog of Divergences to Fix (severity order)

| # | Divergence | C anchor | Go anchor | Severity | Status |
| --- | --- | --- | --- | --- | --- |
| D1 | Pusher think-gate re-read → double/skip think | `SV_Physics_Pusher` sv_phys.c:618-652 | `leafs.go` PhysicsPusher | **high** | FIXED in-tree (+test) |
| D2 | flymove clip-plane budget: C `MAX_CLIP_PLANES 5`, Go `maxClipPlanes=5` + `bumpCount<4` (matches C `numbumps=4`) — **verified equal, no change needed** | sv_phys.c:230,254 | leafs.go:21,193 | n/a (verified) | closed |
| D3 | NaN velocity/origin: C warns, Go silent | sv_phys.c:87-110 | leafs.go CheckVelocity | low | open (cosmetic) |
| D4 | Inactive client slots still dispatch movetype | sv_phys.c:946-956 | stepframe.go:106-186 | med | open |
| D5 | Renderer brightness/contrast (qbj3 ~7 mean delta) | gl_rmain.c/gamma path | renderer | high (visual) | open (needs photometric audit) |
| D6 | Entity send distance-sorted (Go addition; C edict order) | sv_main.c | server_net_send.go:363+ | intentional | DOCUMENT, don't fix |
| D7 | Stale docs claim QCVM sync-all; code is no-op accessors | QCVM_ENTITY_SYNC.md | server_qc_sync.go:11-17 | low (teachability) | plan 24 doc consolidation |
| D8 | LERP_FINISH byte: verify equality at encode time | sv_main.c:952 | net/encode.go:32-43 | low | empirical probe needed |

## 3. Step-by-Step Implementation Sequence

### Step 23.1: D2 — FlyMove MAX_CLIP_PLANES 4 → 5 (+ C-cited probe)
- **Files**: `internal/server/physics/leafs.go` (`maxClipPlanes`),
  `internal/server/physics/*_test.go`.
- **Actions**: align `maxClipPlanes` to C's 5; add a parity probe that runs an
  entity into a synthetic 5-plane crease (mock collision world emitting 5
  co-planar terms) and asserts the Go path does not dead-stop (returns blocked
  per C, not 7-with-zero-velocity).
- **Where in C**: `#define MAX_CLIP_PLANES 5` sv_phys.c:216 + SV_FlyMove
  plane/crease block sv_phys.c:320-345.

### Step 23.2: D4 — inactive client slots skip movetype dispatch
- **Files**: `internal/server/physics/stepframe.go`.
- **Actions**: mirror C's `if (!svs.clients[num-1].active) return;` gate at the
  top of the client-slot iteration (i in 1..maxclients): skip the movetype
  switch AND pre/post think for inactive slots. Add a test with an inactive
  slot whose movetype is WALK asserting no `PlayerPreThink`/`PhysicsWalk` runs.
- **Where in C**: `SV_Physics_Client` sv_phys.c:946-956.

### Step 23.3: D3 — NaN warns (cosmetic parity)
- **Files**: `internal/server/physics/leafs.go` CheckVelocity.
- **Actions**: emit `slog.Warn` (like C `Con_Printf`) when NaN velocity/origin
  zeroed; keep behavior identical.
- **Where in C**: sv_phys.c:87-110.

### Step 23.4: D5 — Renderer brightness/contrast audit (photometric)
- **Files**: `internal/renderer` (gamma/tonemap/lightmap sampling candidates),
  `tools/parity_screenshots/compare.go` (already computes mean/max channel
  delta + SSIM — use it as the measurement tool).
- **Actions**:
  1. Classify the delta (whole-frame gain vs local lightmap drift): run the
     id1 scene matrix (all 32 viewpoints; `mise run parity-*`) and split
     diffs into global (gain/gamma) vs spatial (lightmap/texinfo) buckets.
  2. Compare Go's gamma application path against C (`r_gamma` handling in
     gl_rmain.c) and the lightmap sample→final pipeline against
     `gl_lightmap`/`r_lightmap.c`-equivalent; fix the culprit.
  3. Re-run the matrix; target qbj3 mean channel delta < 2 with id1/expansion
     green.
- **Verify**: `mise run parity-ref && parity-go && parity-compare`; tighten
  `PARITY_COMPARE_TOLERANCE`/`PARITY_MAX_MISMATCH_PERCENT` as deltas shrink.

### Step 23.5: D6 — Document the entity-send sort as intentional
- **Files**: `docs/PARITY.md` §Known parity gaps (extend the table with a
  "deliberate deviation" row citing `server_net_send.go`), plus a code comment
  at the sort (`entitySendSortKey`) naming C's edict-order baseline.
- **Action**: no behavior change; make the deviation auditable.

### Step 23.6: D8 — empirical LERP_FINISH probe
- **Files**: `internal/server/*_test.go` (parity probe), reuse
  `TestParitySoundEmittedSameFrame` conventions.
- **Actions**: build a client/server pair (loopback), drive a WALK entity with
  a known `nextthink`; assert `EncodeLerpFinish(nextThink, s.Time)` byte equals
  `(byte)Q_rint((nextthink - qcvm->time)*255)` from the C reference for the
  same inputs, including the gate on `sendinterval` (j in 0..255, j≠25,26).

### Step 23.7: QCVM/EntVars parity re-check (pigsback on zero-sync completion)
- **Files**: `docs/plans/10/12` (`EntVars` removal), `internal/server/edict`.
- **Actions**: after zero-sync completes, re-run the qbj2 door + lift chain
  probes (existing `TestQbj2TwinDoorsBothFireViaChain` etc.) and assert the
  ORIGINAL frame-parity tests (`internal/game/parity_test.go` demo parity) stay
  green — the accessor layer is the new source of truth.

## 4. Verification & Testing Strategy

1. **Red/green discipline per fix**: each step lands with its C-cited test
   first (fails before, passes after).
2. **Regression surface**: `mise run test` (no assets required), then
   `mise run smoke-map-start` (needs display) and `mise run parity-all` for
   visual regressions after D5.
3. **Frame-state parity** (`internal/game/parity_test.go`) re-run for demo1 on
   a full install with `demo1.dem` present (skips on Quake Enhanced data —
   extend with H1 dump schema from plan 24 so any demo becomes usable).

## 5. Risks & Mitigations

| Risk | Mitigation |
| --- | --- |
| D2 crease path hard to hit deterministically | mock collision world emits plane terms; also fuzz via H4 (plan 24) |
| D5 audit churns lightmap pipeline | gate every change on the 32-viewpoint matrix; keep deltas as fences |
| D4 changes feel-behavior for intermission/non-walk clients | C-cited gate + explicit tests incl. intermission MOVETYPE_NONE path |
| D8 is encode-time-clock-drift sensitive | drive clocks exactly (srvTime known); assert range not exact where C/Go frame timing differs |
