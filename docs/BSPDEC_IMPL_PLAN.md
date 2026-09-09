# BSPDEC Implementation Plan

> **Superseded further (2026-09-09):** by
> `docs/superpowers/plans/2026-09-09-bspdec-m1-brushlist-path-and-synth.md`
> and the M0/M1 plan series; M1 (BRUSHLIST direct path + synthetic corpus +
> headroom gate) is complete. Kept for history.
>
> **Superseded (2026-09-07):** executable plans now live under
> `docs/superpowers/plans/` (plan 1 = M0 deterministic decompiler,
> `2026-09-07-bspdec-m0-deterministic-decompiler.md`), arguing from the
> canonical spec `docs/superpowers/specs/2026-09-07-bspdec-design.md`. This
> file is kept for its phase overview and repo-asset inventory.

**Status:** plan (v1) · **Date:** 2026-09-07
**Spec:** [docs/BSPDEC_SPEC.md](BSPDEC_SPEC.md) (v2) · **Beads:** epic
`ironwail-go-xxy` + 12 children · **Research:**
`~/.local/share/crush/research/quake-bsp-decompiler-ml/report.md`

---

## 0. How to work this plan

- Claim beads before starting (`bd update <id> --claim`), one bead at a time;
  close with `bd close <id> --reason="..."` when its acceptance is met.
- TDD per AGENTS.md: write the failing test first for every stage; keep
  `mise run verify` green at every merge point.
- Stable baseline first: `mise run verify` before touching anything.
- Single-package test runs must match the mise env:
  `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestX -count=1`.
- Conservative git policy: no commits/pushes without explicit user approval.
- Cite C lineage inline on every ported algorithm (parity convention).

## 1. Repo-asset inventory (verified 2026-09-07 — changes the plan)

These existing assets mean M0/M1 are mostly *wiring*, not greenfield:

| Asset | What it gives us |
| --- | --- |
| `internal/bsp` | BSP29/BSP2 reader: all lumps, tree, lit |
| `internal/qbsp` | **Pure-Go compiler** (QuakeEd+Valve 220 parse → BSP29/BSP2 + clipnode hulls); **BRUSHLIST BSPX is appended unconditionally** (`compiler.go:241`, `bspx.go`); `ReadBSPXBrushList` reader exists; `mapfile.go` `ParseMap` = existing .map parser |
| `cmd/qbsp`, `cmd/vis`, `cmd/light` | In-repo compile toolchain → **pinned toolchain for corpus/eval with no external subprocess**; ericw-tools becomes cross-validation only |
| `cmd/bspgen` | Pattern reference for `tools/bspdec_synth` (currently one-off transparency map) |
| `tools/parity_screenshots`, `testdata/parity/` | Tier-3 behavioral eval oracle |
| `cmd/bspdiag` | Corpus validation CLI |

**Plan deltas vs spec:** (a) `internal/map` reuses `internal/qbsp/mapfile.go`
instead of a from-scratch parser; (b) BRUSHLIST reading should move to
`internal/bsp` (reader-side) so `internal/bspdec` doesn't import the compiler
package; (c) eval uses in-repo `cmd/qbsp` as the pinned compiler.

## 2. Phase 0 — scaffolding (no bead; half a day)

1. `mise run verify` (baseline).
2. Create `internal/bspdec/` skeleton (`doc.go` with lineage section) +
   `internal/bspdec/eval/`; create `cmd/bspdec/` skeleton.
3. `mise.toml` task stubs (echo "not implemented"): `build-bspdec`,
   `bspdec-data`, `bspdec-train`, `bspdec-eval`, `bspdec-report`,
   `bspdec-models`.
4. Add whitelist exception if any committed fixtures land outside existing
   allow rules (spec §13 keeps bulk out of git; verify `.gitignore` allows
   `testdata/bspdec/**` if fixtures are added).
5. Verify: `mise run verify`.

## 3. Phase 1 — M0 deterministic decompiler (beads xxy.1, .2, .3, .4)

Order: **.1 ∥ .2** (parallel-safe), then **.3**, then **.4**.

### 3.1 `ironwail-go-xxy.1` — `internal/map` reader+writer

1. Inventory `internal/qbsp/mapfile.go` API surface (`ParseMap`, `Map`,
   entity/brush/face types) — decide extract-vs-wrap; preferred: move shared
   types to `internal/map` and have `internal/qbsp` import them (single source
   of truth; keep qbsp's parity tests green throughout).
2. Implement writer: worldspawn + brush entities; face lines (3-point planes,
   texture, Valve-220 axes, scale default 1.0); contents→texture conventions
   (`*water`/`*slime`/`*lava`/`sky`/`clip`); origin-brush semantics;
   grid-snap quantizer.
3. Tests-first: writer(reader(x)) identity on canonical brushes (fixtures from
   `internal/qbsp/mapfile_test.go` cases); tolerance for hand-authored maps.
4. Verify: `go test ./internal/map ./internal/qbsp -count=1` + `mise run verify`.

### 3.2 `ironwail-go-xxy.2` — treewalk core (`internal/bspdec`)

Files per spec §4. Steps (each with a failing test first):

1. `types.go`: `Cell`, `CellSide`, `Brush` (halfspace set), contents enum.
2. `treewalk.go`: per-model bbox+8 init → node recursion with winding-clip
   splits → leaf emit by contents (`// Where in C: Q1_CreateBrushes_r in
   bspc/map_q1.c`).
3. `planes.go`: redundant-plane removal via winding clip (`RemoveRedundantPlanes`),
   node-only-plane filtering.
4. `brush.go`: initial-brush build (clip each side's face winding by other
   planes, keep back side — `BuildInitialBrush`).
5. `texture.go`: face-overlap texturing + Valve-220 axis recovery;
   `SplitDifferentTexturedPartsOfBrush`; `--texture-fallback` policies.
6. `entities.go`: origin brushes; entity block preservation.
7. `merge.go`: same-contents convex merging (`--merge-convex`).
8. `hull.go`: `--decompile-hull N` un-expansion + bevel-plane dropping.
9. Golden tests: compile known .maps with in-repo `cmd/qbsp` (BRUSHLIST
   present → also gives oracle), decompile, compare against BRUSHLIST truth;
   no panics on `quake-data` maps when available (skip via
   `testutil.SkipIfNoPak0`-style helper).
10. Verify: `go test ./internal/bspdec -count=1` + `mise run verify`.

### 3.3 `ironwail-go-xxy.3` — `cmd/bspdec` CLI

1. Flags table from spec §3 (exact names/defaults), exit codes 0/1/2/3,
   slog wiring (`-loglevel`), `--json` schema (spec §3) + schema test.
2. Self-check (exit 3): re-parse emitted `.map` via `internal/map`; validate
   brushes (non-empty, convex).
3. Integration test: decompile a compiled synthetic map end-to-end; assert
   JSON fields + output parseability.
4. Verify: `go test ./cmd/bspdec -count=1`.

### 3.4 `ironwail-go-xxy.4` — eval suite + corpus bootstrap

1. `internal/bspdec/eval/corpus.go`: JSONL manifest (spec §9.1 schema) +
   loader; `compile.go`: pinned in-repo `cmd/qbsp` runner (+ optional
   external-ericw cross-check flag); `diff.go`: lump diff + voxelized IoU.
2. `tools/bspdec_corpus`: stages P1–P7 per spec §9 (enumerate Quaddicted +
   `quake_map_source`; fetch+`bspdiag` validate; canonicalize (in-repo qbsp,
   BRUSHLIST always on); label derivation via `internal/bspdec/labels.go`;
   package-level splits + classic holdout; toolchain augmentation via vintage
   external compilers as optional later step).
3. `internal/bspdec/labels.go`: cell assignment (point-in-convex vs original
   brush halfspaces), seam labels (plane-crossing test), BRUSHLIST truth —
   spec §9.4.
4. `mise run bspdec-data` = full idempotent pipeline; `bspdec-eval` runs tiers
   1+2; `bspdec-report` summarizes.
5. Acceptance gate: corpus reproduces from scratch; eval matrix on ≥50 paired
   maps; numbers recorded in the bead.
6. Verify: full pipeline dry-run on `quake_map_source` subset first.

## 4. Phase 2 — M1 oracle + gate (beads xxy.11, .5)

1. **xxy.11 synth generator** (`tools/bspdec_synth`): room-grammar generator
   following the `cmd/bspgen` pattern but emitting via `internal/map` writer;
   8-unit lattice + mapper conventions; seeded determinism; ≥1000 pairs; feeds
   labels stage directly.
2. **xxy.5 BRUSHLIST path**: move BSPX reading to `internal/bsp`
   (`ReadBSPXBrushList` relocates from `internal/qbsp` — update its callers);
   `internal/bspdec/brushlist.go` direct emission path; `--no-brushlist`
   honored.
3. **Headroom study** (the gate): on corpus+synth, compare (a) treewalk
   baseline, (b) BRUSHLIST-oracle decomposition, on recompile-diff + structural
   metrics; the gap = ML's maximum added value.
4. **Gate decision**: if oracle can't beat baseline meaningfully, record
   no-go in xxy.5, close it, and stop before M2 (spec §12).
5. Verify: `mise run bspdec-eval` reproduces study table.

## 5. Phase 3 — M2 ML stages (beads xxy.12, .6, .7)

Order: **.12** (pipeline) then **.6 ∥ .7**.

### 5.1 `ironwail-go-xxy.12` — training pipeline + registry

1. `tools/bspdec_train`: feature extraction from labeled records (spec §9.4
   graph schema); GoMLX training loops (Routes A/B); export (GoMLX native;
   ONNX→GoMLX conversion path documented); packaging to
   `models/bspdec/<route>-v<N>/` + `metadata.json` (spec §10.1 schema).
2. Cacheable steps (dataset SHA + config hash → cache key); `mise run
   bspdec-train` end-to-end; `bspdec-models` provisioning task.
3. Regression guard in `tools/bspdec_report`: fail on metric regression vs
   recorded best.
4. Acceptance: fresh corpus → packaged Route A model in one command.

### 5.2 `ironwail-go-xxy.6` — Route A seam classifier

1. Graph builder: cells + adjacency + candidate-seam edges from treewalk
   output; features per spec §7.1 (collinearity, T-vertex, texinfo transients,
   neighborhood context; UV-Net-style rasterized face descriptors).
2. GoMLX GNN edge classifier; `--ml seams` wiring in `internal/bspdec/seams.go`
   (interface + deterministic fallback).
3. Gate: seam F1 > deterministic split heuristic on held-out corpus.

### 5.3 `ironwail-go-xxy.7` — Route B grouping layer

1. BSP-Net-style T-matrix formulation over deterministic cells (learned
   merge prior, not generation); supervision from §9.4 cell-assignment labels +
   BRUSHLIST truth.
2. `--ml group` wiring; ablation vs `--merge-convex` baseline.
3. Gate: group-recovery gain on held-out corpus; GoMLX pure-Go training
   feasibility verdict recorded.

## 6. Phase 4 — M3 research (beads xxy.8, .9)

1. **xxy.8 Route C pilot**: tokenizer (DeepCAD-style 8-bit closed-set tokens);
   pointer heads over BSP-derived plane/texinfo sets; DETR-style slots +
   Hungarian for brush sets; EBNF masking + decode-time geometric checker;
   CSGNet-style IoU RL; PyTorch primary; synthetic-only pilot; `--ml all`.
   Gate: valid .map program output on synthetic suite; compile+parity pass.
2. **xxy.9 Route D endgame**: egraph (egg Rust binding vs pure-Go rewrite
   search — spike first, decision recorded); learned costs from A/B scores;
   semantics-preserving compression. Gate: parity-tier eval passes.

## 7. Phase 5 — M4 hardening (bead xxy.10)

1. Vintage eval slice: 1996-2000-era `.bsp`s from Quaddicted bsp-only packages
   (external vintage compilers for augmentation, optional).
2. Classic-maps slice kept out of all training splits (assert in pipeline).
3. Full eval matrix incl. classic maps; `tools/bspdec_report` markdown;
   README/manual for bspdec; parity-harness integration for tier 3.
4. Gate: matrix published; docs complete.

## 8. Verification matrix (per phase)

| Phase | Commands |
| --- | --- |
| 0 | `mise run verify` |
| M0 | `go test ./internal/map ./internal/bspdec ./cmd/bspdec -count=1`; `mise run verify`; `mise run bspdec-eval` |
| M1 | headroom table reproduces via `mise run bspdec-eval` |
| M2 | `mise run bspdec-train` end-to-end; F1/gain gates in beads .6/.7 |
| M3 | synthetic-suite compile+parity pass |
| M4 | full matrix incl. classic holdout |

## 9. Gates and stop conditions

- **After M0**: if treewalk can't reach `bsputil` parity on corpus, fix core
  before any ML work (baseline is the contract).
- **After M1**: BRUSHLIST headroom go/no-go (spec §12) — the single most
  important decision point; record verdict in bead xxy.5.
- **During M2**: if Route A doesn't beat the deterministic split heuristic on
  held-out data, Route C is descoped (flagship without seam signal is a bad
  bet); Route D becomes primary research track.
