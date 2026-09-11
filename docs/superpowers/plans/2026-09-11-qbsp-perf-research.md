# qbsp Compile Performance Research Plan (ironwail-go-ros)

> **For agentic workers:** This is a research + recommendation plan. Phases P0-P4
> collect evidence; P5 synthesizes recommendations; P6 sequences implementation.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the >24x qbsp compile-time gap vs ericw-tools on large community
maps (jjj22_dfl et al.) by replacing the superlinear split-selection path and
cutting allocation churn, with a measured decision on Go 1.27 SIMD.

**Architecture:** The Go qbsp re-scores every candidate brush side at every BSP
node by classifying every brush against it (O(C×B×S×V) per node) — ericw-tools
uses AABB-volume metrics (O(1) per candidate) plus a mid-split FAST mode for
large regions. We replace the scoring model, then attack winding-allocation
churn, then evaluate SIMD for the residual batch math.

**Tech Stack:** Go 1.26.6 (go.mod `go 1.26`), pprof + benchstat, ericw-tools C++
source as algorithmic reference, Go 1.27 `simd` package (GOEXPERIMENT-gated,
no compat promise) as an optional accelerator.

**Spec:** beads `ironwail-go-ros` (blocks `ironwail-go-n8g`, `ironwail-go-551`).

## Global Constraints

- CGO_ENABLED=0 always (mise env); TMPDIR must be the repo's `.tmp/` scratch dir.
- Output validity is the gate: sealed maps stay sealed, leak maps keep identical
  leak trails, all `internal/qbsp` tests pass. Byte-exact output with ericw is
  NOT required; Go-qbsp-to-Go-qbsp recompile determinism IS.
- Worktree: `/home/darkliquid/Projects/ironwail-go.worktrees/research-ironwail-go-ros`
  (branch `research/ironwail-go-ros`).
- Corpus maps live in `dataset/bspdec/paired/…` in the main checkout (60 GB,
  gitignored); reference them by absolute path.

---

## Collected Evidence (2026-09-11, 90s partial profile of jjj22_dfl)

Compile did not finish in 90s (production timeout is 5 min; a foreground run
exceeded 16 min without completing). Confirmed CPU-bound, allocation-churny:

CPU (90.49s samples):

| flat | cum | function |
|---|---|---|
| 44.3% | 94.6% | `classifyBrush` (brush.go:180) |
| 42.4% | 42.4% | `pkg/types.Vec3T[float64].Dot` (inlined into classifyBrush) |
| 6.8% | 6.8% | `planeEqualOriented` (called per side inside classifyBrush) |
| — | 95.9% | `selectSplitPlane` (solidbsp.go:223) via `(*treeBuild).build` |

Allocations (4.95 GiB in 90s, ~55 MiB/s, 81 GC cycles, 7 ms total pause):

| share | function |
|---|---|
| 45.1% | `clipWinding` (poly.go:29 — `side []int` + new winding per call) |
| 28.0% | `splitBrush` (brush.go:198) |
| 17.0% | `chopBrushes` |
| 4.9% | `windingFromBoxPlane` |

Root causes identified by reading the C reference
(`~/Projects/ericw-tools/qbsp/brushbsp.cc`):

1. Go `selectSplitPlane` evaluates metric `5*facing - 5*splits - |fronts-backs|`
   for EVERY candidate side by classifying EVERY brush (full winding-vertex
   dot-product scan + `planeEqualOriented` 3-dot compare per side).
   ericw's `SplitPlaneMetric` is `|vol(front)-vol(back)|` from `DivideBounds`
   — pure AABB math, no brush classification at all.
2. ericw marks sides `onnode` once used as a splitter and never reconsiders
   them; Go only filters planes equal to the current region's planes.
3. ericw runs a FAST/midsplit mode for large open regions
   (`ChooseMidPlaneFromList`, bounds-midpoint, O(B) per node, one
   classification pass for the chosen plane only). Go has a `splitFast`
   policy but it still scans candidates the same way.
4. ericw compares planes by integer `planenum` from a dedup table; Go compares
   float normals/dists in the hot loop.
5. Winding splits allocate a fresh `side []int` + output winding per
   `clipWinding` call in the hottest loops.

## Phases

### P0 — Benchmark & profile harness (repeatable baselines)

**Files:** `tools/qbspprof/main.go` (exists, uncommitted), new
`internal/qbsp/perf_bench_test.go`, `docs/superpowers/plans/perf-matrix.md`.

- [ ] Commit `tools/qbspprof` (deadline-based partial profile dumping).
- [ ] Build a fixed 6-map benchmark matrix: 3 that compile fast today (regression
      canaries) + 3 large timeouts (jjj22_dfl, sm190_nait, jam6_necros_v2).
- [ ] Add `BenchmarkCompile<Map>` tests that parse once and compile, reporting
      `b.ReportMetric` for allocs/op; record `benchstat` baselines into
      `.tmp/bench/` (gitignored output, doc lists commands).
- [ ] Capture baseline CPU/heap profiles for all 6 maps via qbspprof
      (`-deadline 90s` for the big three).

**Exit criteria:** `benchstat old.txt` files exist; commands reproducible from
the plan doc alone.

### P1 — Algorithmic split-selection (the 95% fix)

**Files:** `internal/qbsp/solidbsp.go` (selectSplitPlane, treeBuild.build),
`internal/qbsp/brush.go` (classifyBrush, splitBrush), `internal/qbsp/planes.go`
(plane table), tests in `internal/qbsp/solidbsp_test.go`.

Ordered sub-experiments, each measured separately on the matrix:

- [ ] **P1a Plane table + integer identity.** Dedup planes at brush-build time
      into a global table (hash on quantized normal+dist, both orientations);
      carry `planenum` on `bspSide`. Replace `planeEqualOriented` in hot loops
      with integer comparison. Expected: kills the 6.8% + shrinks constants.
- [ ] **P1b `onnode` marking.** Once a side's plane becomes a node splitter,
      flag the side; skip flagged sides in candidate scans (mirror ericw).
- [ ] **P1c Bounds-based metric.** Replace per-brush classification in the
      scoring loop with ericw's `SplitPlaneMetric` (DivideBounds volume
      difference, axial preference via the 4-pass visible/detail ordering).
      Classification then runs once per node for the *chosen* plane (as ericw
      does in the FAST path and for front/back partitioning).
- [ ] **P1d FAST/midsplit for large regions.** Port `midsplitbrushfraction`
      behavior: when `brushes/totalBrushes` exceeds the fraction, use
      `ChooseMidPlaneFromList` (bounds midpoint, O(B)).

**Validation per sub-experiment:** full `internal/qbsp` test suite; sealed/leak
outcomes unchanged on the matrix; qbspprof wall time + new profile; benchstat
vs baseline. Tree-shape changes are acceptable if validity gates hold — record
leaf/node deltas in the perf matrix doc.

**Exit criteria:** jjj22_dfl compiles < 60s (target < 15s to match ericw's
12.6s); no test regressions.

### P2 — Allocation churn (the 4.9 GiB/s → near-zero pass)

**Files:** `internal/qbsp/poly.go` (clipWinding), `internal/qbsp/brush.go`
(splitBrush/chopBrushes), possibly a small `internal/qbsp/arena.go`.

- [ ] Re-profile after P1 (the churn profile will change shape; old data is
      pre-P1 and only partially predictive).
- [ ] Hoist the `side []int` scratch out of `clipWinding` (sync.Pool or
      compiler-scoped scratch buffer sized to max winding).
- [ ] Evaluate winding slicing reuse: split outputs write into caller-provided
      capacity (front/back buffers) instead of `make` per call.
- [ ] If bspBrush/bspSide garbage dominates post-P1: region-scoped arena
      (offset-based slice allocation, freed per subtree).
- [ ] Gate: `go test -bench` allocs/op drop >= 5x on the matrix; wall time
      improvement retained; no correctness drift.

### P3 — SIMD evaluation (Go 1.27 `simd` package)

**Files:** spike branch or `internal/qbsp/simd_test.go` benchmarks only.

- [ ] Post-P1, identify the residual vectorizable loops (expected: batch
      vertex-vs-plane dot classification inside classifyBrush; 8-corner
      bounds test in planeSplitsBounds).
- [ ] Prototype with Go 1.27 `simd` (portable, GOEXPERIMENT=simd):
      `LoadFloat64s`/`BroadcastFloat64s` + `MulAdd` + `Min`/`Max` +
      comparison masks map 1:1 to the vertex-dot scan and corner reduction.
- [ ] Compare against two non-SIMD baselines on the same loops: (a) plain
      scalar, (b) manual SoA + 4-wide unrolled float64 (compiler does not
      auto-vectorize today, but known-bits/LICM in 1.27 help marginally).
- [ ] Adopt only if: >= 2x on the loop microbenchmark AND >= 5% end-to-end on
      the matrix AND we accept the constraints — experimental, no Go 1
      compat promise, amd64+arm64 only with pure-Go fallback elsewhere.

### P4 — Go 1.27 bump mechanics (only if P3 adopts, or for the free runtime wins)

**Files:** `go.mod`, `mise.toml` (`go = "1.27"`), CI/lint config.

- [ ] The version bump alone buys Go 1.27 runtime improvements independent of
      SIMD: size-specialized small-allocation routines (up to 30% cheaper
      sub-80-byte allocs — directly attacks P2 churn), loop-invariant code
      motion, known-bits pass. Measure on the matrix before adopting SIMD.
- [ ] Rollout: go.mod `go 1.27` + mise pin + `GOEXPERIMENT=simd` in
      build/lint/test tasks if P3 adopted; document the experiment flag in
      AGENTS.md gotchas (breaks if unset; export-data `@` symbol caveat for
      debuggers).

### P5 — Recommendation synthesis

**Files:** `docs/superpowers/plans/qbsp-perf-recommendations.md`, beads tasks.

- [ ] Consolidate the perf-matrix evidence into a scored table: change,
      wall-time delta per map, allocs/op delta, complexity class before/after,
      parity risk, maintenance cost, SIMD dependency.
- [ ] Rank into: adopt now / adopt with Go 1.27 / reject (with data). Each
      "adopt" row becomes a beads task with its validation commands.
- [ ] Write the one-page ADR for the split-selection model change (parity
      tradeoff: Go no longer mirrors the old id-metric scoring; cite ericw
      line numbers and this plan's evidence).

### P6 — Implementation sequencing (output of P5, not started here)

Bite-sized tasks, each: failing bench/test → change → matrix benchstat →
full test suite → commit. Order strictly: P1a → P1b → P1c → P1d → P2 →
(optional P4) → (optional P3), re-profiling between each.

---

## Measurement Commands

```
# single-map partial profile (deadline dumps even if compile never finishes)
./.tmp/qbspprof -cpuprofile .tmp/m.prof -memprofile .tmp/m.mem -deadline 90s <map>.map
go tool pprof -top .tmp/qbspprof .tmp/m.prof
go tool pprof -top -sample_index=alloc_space .tmp/qbspprof .tmp/m.mem

# benchmarks
TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/qbsp -bench=BenchmarkCompile -benchmem -count=10 | tee .tmp/bench/new.txt
benchstat .tmp/bench/old.txt .tmp/bench/new.txt
```
