# BSPDEC Spec — ML-Augmented Quake BSP → .map Decompiler

> **Superseded (2026-09-07):** the canonical design now lives at
> `docs/superpowers/specs/2026-09-07-bspdec-design.md` (v3), with executable
> plans under `docs/superpowers/plans/`. This file is kept for its research
> narrative and citation history.

**Status:** spec (v2) · **Date:** 2026-09-07 (v2 review: dataset provisioning +
training execution hardened, milestones bound to real bead IDs)
**Source research:** `~/.local/share/crush/research/quake-bsp-decompiler-ml/report.md`
(+ `notes/*.md`) — cited below as `[report §N]`.
**Tracked in Beads:** epic `ironwail-go-xxy` + children (milestone table in §11).

---

## 1. Background and goals

Quake's BSP29 format does not store original map brushes: compile-time CSG
merges, splits, and deletes them, leaving only boundary geometry. Existing
decompilers (bsp2map, WinBSPC/MBSPC, ericw-tools `bsputil --decompile`, BSP
Forge) reconstruct geometry by walking the node tree and intersecting
root→leaf planes into convex cells, then re-texturing from the face lump; all
share documented failure modes: coplanar merged-face mis-segmentation, brush
count/order blowups, broken liquids, vanished CLIP/NULL brushes, phantom
geometry [report §3.1, §3.2].

**Goal:** a new command `bspdec` that decompiles a Quake `.bsp` to an editable
`.map`, built as a hybrid of (a) a deterministic core matching/beating
`bsputil --decompile`, and (b) ML stages that attack the two information gaps
heuristics cannot close: original brush seams hidden inside merged coplanar
faces, and original brush grouping/count [report §1, §3.4].

**Honest success criterion:** functionally-equivalent, plausibly-decomposed
`.map` that recompiles to near-identical engine behavior — **not** bit-identical
BSP round-trips (impossible in general; achievable only for BSPX-BRUSHLIST
maps) [report §1, `notes/risks-contrarian.md`].

**Parity-first:** preserve behavioral parity with C Ironwail/Quake; the repo's
parity harness (`tools/parity_screenshots`, `mise run parity-*`) is the
engine-level evaluation oracle [report §3.6].

## 2. Non-goals (v1)

- Bit-identical `.bsp` round-trip guarantees (except via BRUSHLIST shortcut).
- Editor-metadata recovery (layers, groups, func_detail): information-theoretically absent.
- Decompiling BSP2/Quake64/Hexen-II won't be a v1 priority; the design must
  not preclude them (all share the 15-lump layout).
- In-engine runtime use of ML: `bspdec` is a standalone offline tool.
- CGO anywhere in the main module (see §7.4 — GoMLX pure-Go inference).

## 3. CLI specification

```
bspdec <input.bsp> [-o output.map] [flags]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `-o, --output PATH` | `<input stem>.bspdec.map` | output `.map` path; `-` = stdout |
| `--format map` | map | only format (reserved for future formats) |
| `--no-brushlist` | off | ignore BSPX BRUSHLIST lump even if present |
| `--decompile-hull N` | none | also emit CLIP-brush geometry from collision hull N (1-3), un-expanded |
| `--merge-convex` | on | merge same-contents coplanar-adjacent convex cells (bspc `-lessbrushes` analogue) |
| `--grid-snap N` | 8 | quantize output brush vertices to integer lattice N |
| `--texture-fallback skip\|nearest\|trigger` | nearest | policy for sides with no matching face (bsputil analogue) |
| `--ml seams\|group\|all` | (unset) | enable ML stage(s); requires `--model-dir` (M2+) |
| `--model-dir DIR` | `models/bspdec/` | directory of model files + metadata (provisioned, not committed) |
| `--json` | off | machine-readable summary on stdout (schema below) |
| `-loglevel` | info | `slog` level (repo convention, as `cmd/ironwailgo` logging) |
| `-h, --help` | | usage |

Exit codes: `0` success (warnings permitted), `1` input/parse error, `2`
internal error, `3` output self-check failed (emitted `.map` failed to
re-parse / failed brush validation).

`--json` schema (v1, stable key names):

```json
{"input": "e1m1.bsp", "output": "e1m1.bspdec.map", "bspx_brushlist": false,
 "models": [{"model": 0, "brushes": 812, "leaves_solid": 430,
             "planes_used": 2411, "warnings": 3}],
 "ml": {"stage": "none", "model": null},
 "exit": 0}
```

## 4. Architecture

Package layout (new code; reuses existing):

```
cmd/bspdec                      CLI entry point (flags, slog, exit codes)
internal/bsp                    EXISTING — BSP29 loader (File/Reader/Load, all lumps)
internal/map                    NEW — .map reader (supervision) + writer (emission)
internal/bspdec/
  decompile.go                  orchestrator: pipeline state machine
  treewalk.go                   M0 deterministic core: BSP-tree walk → convex cells
  planes.go                     plane dedup, node-only-plane filtering, hull un-expansion
  texture.go                    texinfo → Valve-220 axes; face-overlap texturing;
                                texture-boundary splitting (SplitDifferentTexturedPartsOfBrush)
  brushlist.go                  BSPX BRUSHLIST detect/parse → direct brush emission
  merge.go                      same-contents convex merging (--merge-convex)
  labels.go                     M1+: label derivation (original-brush ↔ cell assignment, seam truth)
  seams.go                      Route A (M2): seam/edge classifier interface + GoMLX impl
  group.go                      Route B (M2): plane→brush grouping layer
  program.go                    Route C (M3, research): brush program generation
  search.go                     Route D (M3, research): egraph search w/ learned costs
  ml.go                         shared ML inference wrapper (GoMLX pure-Go backend), model metadata
internal/bspdec/eval/
  corpus.go                     corpus manifest loader (JSONL) + dataset shard loader
  compile.go                    pinned ericw-tools subprocess runner
  diff.go                       BSP lump diff (planes/leafs/faces) + voxelized IoU
tools/bspdec_corpus             dataset provisioning: enumerate + download + normalize pairs
tools/bspdec_synth              synthetic map generator (room grammar on 8-unit lattices)
tools/bspdec_train              training pipeline (feature extraction → train → export → package)
tools/bspdec_report             eval summarizer → JSON + markdown
models/bspdec/                  provisioned model artifacts (gitignored; see §13)
```

Pipeline (per model, i.e. world then each `*N` bmodel):

```
bsp → internal/bsp parse
   → [BRUSHLIST present & !--no-brushlist] → direct brush emission → merge → emit
   → else → treewalk (bbox+8 → split by node planes → leaf cells)
       → planes cleanup (redundant removal via winding-clip)
       → [ML seams] → seam-aware face segmentation on coplanar merged sides
       → [ML group] → learned plane→brush grouping
       → texture stage (face-overlap texturing, texture-boundary splits, Valve-220)
       → [ML all] → optional Route C repair pass
       → merge/convex cleanup → grid snap → self-check (re-parse) → .map emission
```

All ML stages sit behind flags + an ablative switch; the deterministic core
output is always reproducible with `--ml` unset [report §3.4 synthesis].

## 5. Deterministic core (M0)

Port the documented bspc/ericw algorithm family to Go [report §3.2,
`notes/overview.md`]:

1. **Per-model extraction**: take `dmodel` bbox, grow by 8 units → single box
   brush (bspc `Q1_CreateBrushesFromBSP`).
2. **Tree walk**: recurse the node tree; at each node split the current brush
   with the node's plane (winding clipping); at non-empty leaves (`contents !=
   CONTENTS_EMPTY`) emit a brush of the leaf's contents (SOLID/SKY/CLIP/
   TRANSLUCENT/WATER/SLIME/LAVA per BSP29 leaf contents + TEX_SPECIAL
   texture flags). Empty leaves discarded.
3. **Redundant plane removal**: clip a huge winding by every other plane of
   the brush; drop planes that vanish (ericw `RemoveRedundantPlanes`).
4. **Initial brush build**: per side, clip face winding by all other planes,
   keep the back side (ericw `BuildInitialBrush`).
5. **Texture-boundary splitting**: if a reconstructed side carries 2+ faces
   with different texinfo, split the brush along the face's edge planes
   (ericw `SplitDifferentTexturedPartsOfBrush`, WinBSPC "best match" analogue).
6. **Texturing**: for each side find the face on the same plane with the
   largest area overlap; copy texinfo → Valve-220 texture axes; miptex name
   from the textures lump; fallbacks per `--texture-fallback`.
7. **Origin brushes**: for bmodels, entity `origin` + origin-brush semantics
   (translate brushes so origin brush sits at 0,0,0; entity `origin` key set).
8. **Contents mapping**: leaf contents direct; `TEX_SPECIAL` textures (sky,
   `*water`/`*slime`/`*lava`) fixed per bspc `Q1_FixContentsTextures`.
9. **Bevel handling**: hull expansion introduces bevel planes and sliver
   leaves; when decompiling hulls (`--decompile-hull N`) un-expand by the
   hull's AABB offsets (bspc/bspforge technique) and drop bevel-only planes;
   hull planes offset ~±16/24 units by design [report §3.1, ericw qbsp docs].
10. **BRUSHLIST shortcut**: if BSPX `BRUSHLIST` (version 1, per-model AABB +
    contents + non-axial planes) is present and `--no-brushlist` is not set,
    emit brushes directly from it (axial planes inferred from AABB) — this is
    the near-perfect path [report §3.3, §2.4 finding 4].

Acceptance: on the eval corpus (§8), M0 must meet-or-beat `bsputil
--decompile` on recompile-diff and structural metrics, with zero panics on the
full corpus and `--json` reporting per-map brush counts.

## 6. `.map` writer specification (`internal/map`)

- Quake `.map` format (id1/vanilla-conforming): worldspawn entity block then
  brush entities; brushes as ordered face lines.
- Each face line: 3 points defining the plane + texture name + S/T axes
  (Valve-220; per-face `[u v]` offset/rotate/scale syntax; scale default 1.0 =
  1 texel per world unit [report §3.3]).
- Brush sides written as full convex hulls from the surviving windings; no
  bevel planes in output (they are compiler artifacts).
- Contents → `*water`/`*slime`/`*lava`/`sky`/`clip` texture conventions;
  triggers/NULL per fallback policy.
- Entity block preserved from the BSP entities lump verbatim except: `model`
  keys kept, bmodels unchanged; origin brushes for bmodels.
- `--grid-snap N` quantization to integer lattice (default 8, mapper
  convention [report §3.3]).
- Writer also serves as reader (for supervision & training labels): parse
  .map into brushes (plane from 3 points, contents from texture), with
  tolerance for hand-authored maps.
- Round-trip property: writer(reader(map)) must be identity for canonical
  brush definitions (unit test).

## 7. ML stages (M2/M3)

### 7.1 Route A — seam segmentation (M2, first ML scope)
- Task: classify each edge/collinear-vertex-run along merged coplanar faces as
  *brush seam* vs *interior*. Novel: no published ML T-junction/seam detector
  [report §3.4 Route A].
- Features: BRepNet-style topological features (region adjacency graph over
  planar cells; edge features: collinearity, T-vertex presence, length,
  texinfo transients, planar offset, neighborhood context) + UV-Net-style
  rasterized per-face descriptors [report §3.4 Route A].
- Model I/O contract: input = per-map graph (§9.4 record schema) → per-edge
  logit. Compact enough for GoMLX pure-Go inference.
- Supervision: labels derived by §9.4 from paired corpus + synthetic maps.

### 7.2 Route B — plane→brush grouping (M2)
- Learn grouping over the deterministic cell set (BSP-Net-style T-matrix
  formulation, used as grouping/merging prior over exact cells, NOT
  end-to-end shape generation) [report §3.4 Route B].
- Testbed for pure-Go training feasibility (GoMLX experimental GNN).
- Output: per-cell "merge with neighbor X" / "this is one brush" decisions.

### 7.3 Route C — brush program generation (M3, research)
- Autoregressive/pointer program emission per [report §3.4 Route C]: DeepCAD-
  style 8-bit closed-set tokens; plane/texinfo refs via pointer heads over the
  BSP-derived set; DETR-style slots + Hungarian matching for unordered brush
  sets; EBNF grammar masking + decode-time geometric checker (convexity,
  closure, grid snap) for validity-constrained decoding; CSGNet-style IoU RL
  where ground truth is partial.
- Feature-gated: `--ml all`; pilot on synthetic maps first.

### 7.4 Inference and the no-CGO rule
- Repo convention: `CGO_ENABLED=0`, zero build tags [AGENTS.md]. ONNX Runtime
  Go bindings are cgo-based → **not** usable in the main module.
- **Default inference path: GoMLX pure-Go backend** (no cgo, SIMD, WASM-capable)
  [report §3.5]. Models produced by (a) GoMLX training (pure Go), or (b)
  PyTorch → ONNX → GoMLX conversion (`gomlx/onnx-gomlx` / `compute-onnx`).
- Optional cgo path: a separate, documented helper `tools/bspdec_onnx` that
  links `yalue/onnxruntime_go`, built/run outside the main module CI — never
  compiled into `cmd/bspdec`.
- Model packaging: model files + JSON metadata (input schema, class labels,
  quantizer params, trained-on dataset SHA, pinned qbsp version) under
  `models/bspdec/`; `--model-dir` override. Models are **artifacts, not
  committed** (see §13) and are provisioned by `mise run bspdec-models`.

## 8. Evaluation (3 tiers)

1. **Recompile diff**: predicted `.map` + pinned qbsp → compare planes/leafs/
   faces lump stats + voxelized geometry IoU vs original (fast, coarse;
   `--json` reports).
2. **Structural fidelity**: brush-count delta, plane-set R/P, layout graph-
   edit distance (paper: SSIG-style) — tolerates different-but-valid
   decompositions [report §3.6].
3. **Behavioral parity**: run a fixed demo through C Ironwail on original
   `.bsp` vs recompiled map (via existing parity harness); collision-trace
   occupancy invariants.

Pitfalls engineered out: outside-fill masks leaks (compile-success is not a
pass criterion — tier 3 is); qbsp version pinned identically across train and
eval; classic-maps eval slice kept out of any ML training distribution (§9.5).

## 9. Dataset provisioning

Provisioning is a staged, re-runnable pipeline owned by `tools/bspdec_corpus`
+ `tools/bspdec_synth`, driven by one task: `mise run bspdec-data`. Every
stage is idempotent (re-running resumes/repairs in place) and writes a
**provenance log** so any dataset artifact can be traced to its source.

### 9.1 Stage P1 — enumerate
- Input: Quaddicted API (`api/v1/?q=*`, filter `+tags:"source=included"` or
  scan per-file manifests for `*.map`) and the `quaddicted-data` GitHub mirror
  (per-package JSON with sha256 file lists).
- Output: `dataset/bspdec/raw/manifest.jsonl` — one line per package:
  `{pkg_id, source_url, sha256, license_note, map_files[], bsp_files[],
  flags: {brushlist, toolchain_guess, era}}`.
- Also merge `fzwoch/quake_map_source` (40+ guaranteed-clean id1 pairs,
  GPL-2.0, Makefile-rebuildable).

### 9.2 Stage P2 — fetch + validate
- Download packages via `/files/by-sha256/<aa>/<sha>/<file>`; verify sha256;
  extract `.map` + `.bsp` candidates; validate each BSP with `cmd/bspdiag`
  (plane/contents/face audits) and each `.map` with `internal/map` reader.
- Output: `dataset/bspdec/raw/<pkg_id>/` (gitignored bulk) + updated manifest
  with `valid: true|false` and failure reasons.

### 9.3 Stage P3 — canonicalize (the pairing step)
For each valid package, build paired records:
- If both `.map` and `.bsp` present: keep as-is.
- If `.map` only: compile with **pinned** ericw-tools qbsp (record version) to
  synthesize the `.bsp` — twice: once plain, once with `-wrbrushes`
  (BRUSHLIST ground truth).
- If `.bsp` only: keep for the classic-eval holdout (§9.5) and for
  weak-label generation (bsputil decompile = incumbent's mistakes).
- Output: `dataset/bspdec/paired/<map_id>/{orig.map, compiled.bsp,
  compiled-wrbrushes.bsp, meta.json}`.

### 9.4 Stage P4 — label derivation (`internal/bspdec/labels.go`)
The critical supervision step; deterministic, no ML:
- **Cell assignment**: parse original `.map` brushes via `internal/map`; for
  each deterministic leaf cell, test membership against original brush
  halfspace sets (point-in-convex at sampled interior points) → per-cell label
  = original brush id (or `multi`/`none`).
- **Seam labels**: for each candidate edge/collinear vertex run on a merged
  coplanar face, mark `seam` if an original brush plane crosses the face at
  that locus (plane intersection test within tolerance), else `interior`.
- **BRUSHLIST truth**: when the `-wrbrushes` variant exists, the surviving-
  brush list is the reference decomposition.
- Output: `dataset/bspdec/labeled/<map_id>/labels.json` + graph records
  (nodes: cells w/ features, edges: adjacency + seam labels) — the Route A/B
  training tensors live here.

### 9.5 Stage P5 — splits and the classic holdout
- Split paired corpus `train/val/test` at the **package** level (never split
  maps from the same package across partitions — map-author leakage).
- **Classic holdout**: a fixed slice of vintage vanilla `.bsp`s (1996-2000
  era toolchains, no BRUSHLIST) used **only** for evaluation, never training —
  the distribution-shift test (§8 pitfall).
- Output: `dataset/bspdec/splits.json`.

### 9.6 Stage P6 — synthetic generation (`tools/bspdec_synth`)
- Room-grammar brush generator on 8-unit lattices with mapper conventions
  (blockout 64/32, detail 16/8, trim 8/4; stairs/doors multiples of 8/16/64)
  [report §3.3]; emits `.map` via `internal/map` writer; compiled with pinned
  qbsp ± `-wrbrushes`; feeds labels stage directly (perfect ground truth).
- Arbitrary scale; carries the bulk of training volume and sidesteps
  licensing entirely.
- Output: `dataset/bspdec/synth/<seed>/` + manifest entries with
  `license_note: "synthetic"`.

### 9.7 Stage P7 — toolchain augmentation
- Recompile a sample of canonical maps with alternative toolchains
  (tyrutils / vintage qbsp equivalents) to cover 30 years of compile quirks.
- Recorded in manifest `flags.toolchain_guess`; used for robustness eval.

### 9.8 Licensing policy
Per-package `license_note` is mandatory in the manifest; unlicensed packages
(id's 2006 map sources carry no license grant [report finding 9]) are kept
local-only; nothing is redistributed; synthetic data is the preferred bulk.

## 10. Training execution

Training is owned by `tools/bspdec_train`, driven by `mise run bspdec-train`
(end-to-end) or per-route subcommands. Same determinism rules as qbsp: pinned
toolchain, pinned seed, recorded dataset SHA in model metadata.

### 10.1 Per-route pipelines

| Route | Prep | Train | Export | Gate |
| --- | --- | --- | --- | --- |
| A (seams) | graph records from §9.4 | GoMLX (GNN) or PyTorch baseline; BCE on seam logits | GoMLX native or ONNX→GoMLX | seam F1 > deterministic split heuristic on val |
| B (grouping) | cell-set records + BRUSHLIST truth | GoMLX GNN testbed; group-merge BCE/contrastive | same | group recovery gain vs `--merge-convex` on val |
| C (programs) | token streams (DeepCAD-style quantization) | PyTorch (imitation) → CSGNet-style IoU RL | ONNX→GoMLX | valid .map rate + compile parity on synthetic suite |

### 10.2 Automation (the whole loop in one command)

```
mise run bspdec-data      # P1-P7 provisioning, idempotent, resumable
mise run bspdec-train     # prep → train → export → package all enabled routes
mise run bspdec-eval      # run eval tiers 1+2 on test + classic holdout
mise run bspdec-report    # JSON + markdown summary (numbers land in repo wiki/docs)
```

- Each stage is a separate, cacheable step (dataset SHA + config hash → cache
  key) so re-runs only redo what changed.
- Training emits `models/bspdec/<route>-v<semver>/` with `metadata.json`
  (input schema, class labels, quantizer params, dataset SHA, qbsp pin,
  seed, metrics). `--model-dir` selects a versioned dir.
- Eval regression guard: `bspdec-report` fails if key metrics regress vs the
  recorded best for a route.

### 10.3 Hardware/effort expectations
- Route A: small GNN (minutes-hours on one GPU or strong CPU; GoMLX pure-Go
  viable).
- Route B: comparable; doubles as the pure-Go-training feasibility proof.
- Route C: the only heavy lift (transformer, days on GPU); pilot on synthetic
  data; PyTorch primary, GoMLX stretch.

## 11. Milestones and acceptance criteria

| Ms | Beads | Scope | Exit criterion |
| --- | --- | --- | --- |
| M0 | ironwail-go-xxy.1–.4 | `internal/map`, treewalk core, `cmd/bspdec`, eval suite + corpus | meets-or-beats bsputil on corpus; `mise run verify` green |
| M1 | ironwail-go-xxy.5, .11 | BRUSHLIST path + synthetic generator + headroom study | quantified recovery ceiling → ML go/no-go gate |
| M2 | ironwail-go-xxy.6, .7, .12 | Routes A+B models + training pipeline + Go inference | seam/group accuracy > baseline heuristics on held-out corpus |
| M3 | ironwail-go-xxy.8, .9 | Routes C+D research | valid .map program output on synthetic suite; compile+parity pass |
| M4 | ironwail-go-xxy.10 | vintage shift slice, hardening, docs | full eval matrix incl. classic maps; README/manual |

## 12. Risks and gates

- **M1 is the ML go/no-go gate** (BRUSHLIST ceiling study): if oracle-perfect
  seam/grouping can't beat `bsputil` meaningfully, stop before heavy ML spend
  [report §4 "ML may add little"].
- Distribution shift (30 years of toolchains); licensing gray zone (2006 id
  sources have no license grant); eval gaming via outside-fill — mitigated in
  §8/§9.
- GoMLX API churn (v0.28 breaking) — pin versions; models versioned with
  metadata.
- Map-author leakage — splits at package level (§9.5).

## 13. Implementation conventions

- Pure Go, `CGO_ENABLED=0`, no build tags, `log/slog` logging, package-local
  test helpers, `internal/testutil` for assets, no `fmt.Println` logging —
  per AGENTS.md.
- `.gitignore` is a whitelist: `dataset/bspdec/**` and `models/bspdec/**` are
  ignored by default — that is intentional (bulk data and model artifacts are
  never committed). Only tiny hand-picked fixtures go in `testdata/bspdec/`
  (whitelisted). Provisioning is by `mise run bspdec-data` / `bspdec-models`.
- `mise.toml` tasks: `build-bspdec`, `bspdec-data`, `bspdec-train`,
  `bspdec-eval`, `bspdec-report`, `bspdec-models` (download/build model
  artifacts). None join the engine `verify` task (offline tool).
- Every algorithm port cites its C lineage inline (parity convention) e.g.
  `// Where in C: Q1_CreateBrushesFromBSP in bspc/map_q1.c`.
- Tests: unit (winding clip, plane dedup, writer round-trip), corpus-driven
  golden tests, JSON schema tests for `--json`, label-derivation tests on
  synthetic pairs.

## 14. References

- Research report: `~/.local/share/crush/research/quake-bsp-decompiler-ml/report.md`
- Beads: epic `ironwail-go-xxy` + children.
- C lineage: id Q3A `bspc/map_q1.c`; ericw-tools `common/decompile.cc`,
  `qbsp/qbsp.cc` (BRUSHLIST); id `WinQuake/bspfile.h`; Quake Specs v3.4 §4.