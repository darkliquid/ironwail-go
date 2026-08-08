# Implementation Plan 27: Hot-Path Optimization After Parity — Quantified, Gated

**Priority**: #3 (Performance; strictly AFTER plan 23 parity fixes)
**Status**: IN PROGRESS (2026-08-08) — O1 landed (QCVM edict access fast path,
15-18% on benchmark, `13c318a`); O3-O6 remain (profile-dependent + D5-gated).
**Prerequisite**: plan 23 (D1-D6) green, `internal/qc/setglobal_bench_test.go`
precedent (zero-alloc globals), profile evidence from qbj3 sweep
(docs/PARITY.md §qbj3; edict-sync + SetEFloat dominant).
**Estimated effort**: 3-5 focused sessions

---

## 1. Executive Summary & Architectural Context

Research profile data (qbj3 class maps: 750 models, thousands of edicts) puts
the hot paths in (a) QCVM edict field access (171 typed accessors each doing
2-3 bound checks + manual LE unpack — `internal/server/types/entity_accessors*.go`,
`internal/qc/vm_edict.go`), (b) per-frame bookkeeping (StepFrame phase
closures, `host_speeds` measurement), (c) client entity send (candidate sort),
and (d) QC execution context capture/restore (`internal/server/qc_trace.go`).
The rule from research: **optimize only after the C-mapped logic is proven
parity-correct** — a micro-opt on a wrong accessor preserves the bug twice.
Plan 23 fixes parity first; this plan only then benchmarks and optimizes, with
every change gated on (a) the parity suite and (b) ameaningful profile gain.

## 2. Target List (with measurement, not vibes)

| # | Hot spot | Where | Current cost evidence | Plan action |
| --- | --- | --- | --- | --- |
| O1 | QCVM field access (EFloat/EInt/EVector/SetE*) | `internal/qc/vm_edict.go`, accessors | per-field 2-3 bound checks + LE unpack; profile dominated (qbj3) | hoist invariant checks (EdictSize>28 + field-range) out of per-call hot loops via a precomputed per-entity offset table; keep bounds for safety; benchmark |
| O2 | StepFrame measurement closures | `internal/server/physics/stepframe.go` | phaseBegin/phaseEnd alloc when host_speeds off? (verify) | guard `phaseBegin/End` on `measureEnabled()` once (already done) — benchmark confirmation; avoid per-edict closure allocs |
| O3 | Client entity send sort | `internal/server/server_net_send.go:363+` | full-candidate sort per client per frame | if profile confirms, replace `sort.SliceStable` with a ring/priority queue or partial-sort honoring the message-limit cutoff; keep D6 documented deviation |
| O4 | QC execution context capture/restore | `internal/server/qc_trace.go:71` | GInt reads + Depth bookkeeping per QC call | hoist `GetVM()`/`GetTime()` reads out of loops; keep capture semantics bit-identical |
| O5 | `ensureQCVMEdictStorage` growth (`make`+`copy`) | `internal/server/server_qc_sync.go:183` | buffer churn on many-edict maps; `SightEntity` staleness risk (intermittent-anomalies.doc §3) | preallocate to MaxEdicts once at progs load; never realloc mid-frame (also removes the SightEntity-dangle class) |
| O6 | Renderer world upload/passes (if parity D5 clean) | `internal/renderer/renderer_gogpu_world_upload.go`, `render_pass_parity.go` | qbj3 uploads: 86k raw faces → 168k triangles etc. | batch/atlas consolidation (align with plan 02 texture-atlas precedent); measure with `host_speeds 1` + renderer stats |

## 3. Step-by-Step Implementation Sequence

### Step 27.1: Benchmark harness baseline
- **Files**: `internal/qc/edict_access_bench_test.go` (new),
  `internal/server/stepframe_bench_test.go` (new), `internal/server/net_bench_test.go`.
- **Actions**: benchmarks that mirror real qbj3-shaped load (edict count ~2k,
  fields/edict ~40 accessor calls per frame, 750 models). Record baseline
  ns/op + allocs. These benchmarks are the gate: no change below noise.

### Step 27.2: O1 — accessor fast path
- **Files**: `internal/qc/vm_edict.go`, `internal/server/types/entity_accessors*.go`.
- **Actions**: build a per-VM precomputed `offsetTable` (fieldOfs → validated
  byte range) so the hot accessors skip redundant re-validation; keep a slow
  path for tests/edge. Re-run benchmark; verify parity suite + `mise run test`.
- **Verify**: ≥10% on O1 bench, 0 behavior delta (parity suite green).

### Step 27.3: O5 — edict storage preallocation
- **Files**: `internal/server/server_qc_sync.go` (ensureQCVMEdictStorage),
  `cmd/ironwailgo/main.go` (or game init) wiring MaxEdicts early.
- **Actions**: allocate `Edicts `once at MaxEdicts on progs load; growth path
  becomes unreachable in production (keep for tests). Removes pointer churn +
  SightEntity dangle class.
- **Verify**: benchmark + the AI/checkclient probe tests
  (`TestParityAITracelineReportsClearLOS` etc.) green.

### Step 27.4: O3/O4 — send sort + context hoist (profile-dependent)
- **Actions**: only if the benchmark/profiling shows real cost: implement O4
  hoist (low risk) and O3 partial-sort (documented deviation stays). Each gated
  on tests + doc update (plan 26 PARITY.md D6 row).

### Step 27.5: O6 — renderer upload consolidation (D5-dependent)
- **Actions**: only after the D5 photometric audit settles (plan 23.4): batch
  static face uploads, reuse atlas pages (plan 02 precedent); measure via
  `host_speeds` RENDER phase bars before/after on qbj3_stickflip + id1 matrix.

### Step 27.6: parity gate every change
- **Files**: `mise.toml` task `parity-perf-gate` (runs parity suite + benches).
- **Actions**: CI-lite task asserting: all parity tests green AND no
  benchmark regression >5% vs baseline (stored numbers).

## 4. Verification & Testing Strategy
1. Baseline benchmarks committed first (27.1) so every optimization compares
   against recorded numbers.
2. Per-change: `go test ./internal/server/... ./internal/qc/...` + benchmark +
   parity probes (asset-free ones) + `mise run doc-check` (26.4) if docs
   touched.
3. Optional real-map gate: `mise run parity-all` when QUAKE_DIR available.

## 5. Risks & Mitigations
| Risk | Mitigation |
| --- | --- |
| Optimizing before parity (vanity perf on wrong math) | plan order: 23 → 27; step 27.6 enforces |
| `offsetTable` breaks test-only VMs (EdictSize caveats) | slow path retained; unit tests on tiny VMs |
| Sort change alters wire order → client visible ordering | D6 documented; assertions on same-frame order preserved (H2/H4 catch) |
| Upload batching changes lightmap behavior | D5 matrix gate; render_pass_parity.go pass-order oracle |
