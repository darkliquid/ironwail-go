# BSPDEC Design — ML-Augmented Quake BSP → .map Decompiler

**Date:** 2026-09-07 · **Status:** approved design (v3; supersedes
`docs/BSPDEC_SPEC.md` v2, which remains the research narrative)
**Research:** `~/.local/share/crush/research/quake-bsp-decompiler-ml/report.md`
(+ `notes/*.md`), cited as `[report §N]`
**Tracked in Beads:** epic `ironwail-go-xxy` + 12 children
**Implementation plans:** `docs/superpowers/plans/` (one per milestone cluster;
plan 1 = M0)

---

## 1. Purpose

Quake's BSP29 format does not store original map brushes: compile-time CSG
merges, splits, and deletes them, leaving only boundary geometry. Existing
decompilers (bsp2map, WinBSPC/MBSPC, ericw-tools `bsputil --decompile`,
BSP Forge) walk the node tree and intersect root→leaf planes into convex
cells, then re-texture from the face lump. They share documented failure
modes: coplanar merged-face mis-segmentation, brush count/order blowups,
broken liquids, vanished CLIP/NULL brushes, phantom geometry
[report §3.1, §3.2].

**Goal:** a new command `bspdec` that decompiles a Quake `.bsp` to an
editable `.map`: a deterministic core matching or beating
`bsputil --decompile`, plus ML stages attacking the two information gaps
heuristics cannot close — original brush seams hidden inside merged coplanar
faces, and original brush grouping/count [report §1, §3.4].

**Honest success criterion:** a functionally-equivalent, plausibly-decomposed
`.map` that recompiles to near-identical engine behavior — not bit-identical
BSP round-trips (impossible in general; achievable only for BSPX-BRUSHLIST
maps) [report §1, `notes/risks-contrarian.md`].

## 2. Non-goals (v1)

- Bit-identical `.bsp` round-trip guarantees (except via the BRUSHLIST shortcut).
- Editor-metadata recovery (layers, groups, func_detail): information-theoretically absent.
- BSP2/Quake64/Hexen-II input is not a v1 priority; the design must not
  preclude it (all share the 15-lump layout).
- In-engine runtime use of ML: `bspdec` is a standalone offline tool.
- CGO anywhere in the main module (§7.4 — GoMLX pure-Go inference).

## 3. CLI contract

```
bspdec <input.bsp> [-o output.map] [flags]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `-o, --output PATH` | `<input stem>.bspdec.map` | output `.map` path; `-` = stdout |
| `--format map` | map | only format (reserved) |
| `--no-brushlist` | off | ignore BSPX BRUSHLIST lump even if present |
| `--decompile-hull N` | 0 | emit CLIP-brush geometry from collision hull N (1–3), un-expanded |
| `--merge-convex` | on | merge same-contents coplanar-adjacent convex cells (bspc `-lessbrushes` analogue) |
| `--grid-snap N` | 8 | quantize emitted plane points to integer lattice N |
| `--texture-fallback skip\|nearest\|trigger` | nearest | policy for sides with no matching face |
| `--ml seams\|group\|all` | (unset) | enable ML stage(s); requires `--model-dir` (M2+) |
| `--model-dir DIR` | `models/bspdec/` | provisioned model files + metadata |
| `--json` | off | machine-readable summary on stdout (schema below) |
| `-loglevel` | info | `slog` level (repo convention) |

Exit codes: `0` success (warnings permitted), `1` input/parse error, `2`
internal error, `3` output self-check failed (emitted `.map` failed to
re-parse or failed brush validation).

`--json` schema (v1, stable key names):

```json
{"input": "e1m1.bsp", "output": "e1m1.bspdec.map", "bspx_brushlist": false,
 "models": [{"model": 0, "brushes": 812, "leaves_solid": 430,
             "planes_used": 2411, "warnings": 3}],
 "ml": {"stage": "none", "model": null},
 "exit": 0}
```

## 4. Architecture

New and reused packages:

```
cmd/bspdec                      CLI entry point (flags, slog, exit codes)
internal/bsp                    EXISTING — BSP29 loader (LoadTree/Load, all lumps)
internal/map                    NEW (extracted from internal/qbsp/mapfile.go) —
                                package mapfile: .map reader + writer
internal/bspdec/
  decompile.go                  orchestrator: pipeline state machine, Options, Stats
  types.go                      Brush, Side, contents handling
  winding.go                    Winding type, BaseWinding, Clip, Area
  treewalk.go                   M0 deterministic core: BSP-tree walk → convex cells
  planes.go                     redundant-plane removal, winding rebuild, hull un-expansion
  texture.go                    face-overlap texturing, Valve-220 axes,
                                texture-boundary splitting
  entities.go                   entity-lump passthrough, origin brushes
  merge.go                      same-contents convex merging (--merge-convex)
  hull.go                       clipnode-tree walk for --decompile-hull
  brushlist.go                  BSPX BRUSHLIST detect/parse → direct emission (M1)
  labels.go                     M1+: label derivation (original-brush ↔ cell, seam truth)
  seams.go / group.go           M2: Route A/B interfaces + GoMLX implementations
  program.go / search.go        M3: Route C/D research
  ml.go                         shared GoMLX inference wrapper + model metadata
internal/bspdec/eval/
  corpus.go                     corpus manifest loader (JSONL) + shard loader
  compile.go                    pinned in-repo qbsp runner (+ external cross-check)
  diff.go                       BSP lump diff + voxelized IoU
tools/bspdec_corpus             dataset provisioning (P1–P7)
tools/bspdec_synth              synthetic map generator (room grammar, 8-unit lattices)
tools/bspdec_train              training pipeline (features → train → export → package)
tools/bspdec_report             eval summarizer → JSON + markdown
models/bspdec/                  provisioned model artifacts (gitignored)
```

Repo accelerators that shape the design (verified 2026-09-07):

- `internal/qbsp` is a full pure-Go compiler (`ParseMap`, `Compile`,
  `Options`); it **always appends a BRUSHLIST BSPX lump**
  (`compiler.go`, `bspx.go`) and ships `ReadBSPXBrushList`. This makes the
  in-repo qbsp the pinned corpus/eval toolchain — no external ericw-tools
  subprocess needed (ericw becomes cross-validation only).
- `internal/qbsp/mapfile.go` is an ericw-faithful QuakeEd + Valve-220 .map
  parser; `internal/map` reuses it by extraction, with type aliases left in
  `internal/qbsp` so the compiler and its parity tests are untouched.
- `cmd/qbsp`, `cmd/vis`, `cmd/light`, `cmd/bspdiag`, `cmd/bspgen`,
  `tools/parity_screenshots` + `testdata/parity/` are existing patterns/oracles.

Pipeline (per model: world, then each `*N` bmodel):

```
bsp → internal/bsp parse
   → [BRUSHLIST present & !--no-brushlist] → direct brush emission → merge → emit
   → else → treewalk (bbox+8 → split by node planes → leaf cells)
       → redundant-plane removal + winding rebuild
       → [ML seams] → seam-aware face segmentation on merged coplanar sides
       → [ML group] → learned plane→brush grouping
       → texture stage (face-overlap, texture-boundary splits, Valve-220)
       → [ML all] → optional Route C repair pass
       → merge/convex cleanup → grid snap → self-check (re-parse) → .map emission
```

All ML stages sit behind flags + ablation switches; deterministic output is
always reproducible with `--ml` unset [report §3.4 synthesis].

## 5. Deterministic core (M0)

Port the documented bspc/ericw algorithm family to Go [report §3.2],
citing C lineage inline per repo convention:

1. **Per-model extraction**: take `dmodel` bbox, grow by 8 units → single box
   brush (bspc `Q1_CreateBrushesFromBSP`).
2. **Tree walk**: recurse the node tree; at each node split the current brush
   by the node plane (winding clipping); at non-empty leaves
   (`contents != CONTENTS_EMPTY`) emit a brush with the leaf's contents
   (bspc `Q1_CreateBrushes_r`).
3. **Redundant plane removal + winding rebuild**: per side, clip the plane's
   base winding by every other plane of the brush; sides that vanish are
   dropped (ericw `RemoveRedundantPlanes` / `BuildInitialBrush`).
4. **Texture-boundary splitting**: a reconstructed side carrying 2+ faces
   with different texinfo splits the brush along the face boundary planes
   (ericw `SplitDifferentTexturedPartsOfBrush`).
5. **Texturing**: per side, find the face on the same plane with the largest
   area overlap; copy texinfo → Valve-220 axes; miptex name from the textures
   lump; fallbacks per `--texture-fallback`.
6. **Origin brushes**: bmodels translate so the origin sits at 0,0,0 and the
   entity carries the `origin` key (bspc semantics).
7. **Contents mapping**: leaf contents direct; `TEX_SPECIAL` textures fixed
   per bspc `Q1_FixContentsTextures`.
8. **Bevel handling**: `--decompile-hull N` walks the clipnode tree, reverses
   hull expansion by the hull AABB (Minkowski offset subtraction), and drops
   bevel-only planes (no hull-0 plane match) [report §3.1].
9. **BRUSHLIST shortcut** (M1): BSPX `BRUSHLIST` v1 → direct emission, axial
   planes from the AABB [report §3.3, §2.4 finding 4].

**M0 acceptance:** on the eval corpus, meet-or-beat `bsputil --decompile` on
recompile-diff and structural metrics; zero panics corpus-wide; `--json`
reports per-map brush counts.

## 6. `.map` package (`internal/map`, package name `mapfile`)

- Quake `.map` (id1/vanilla-conforming): worldspawn block, then brush
  entities; brushes as ordered face lines.
- Face line: 3 plane points + texture name + Valve-220 axes
  (`[ ux uy uz uoff ] [ vx vy vz voff ] rot xscale yscale`); scale default
  1.0 [report §3.3].
- Brush sides written from surviving windings; no bevel planes (compiler
  artifacts).
- Contents ↔ texture conventions: `*water`/`*slime`/`*lava`/`sky`/`clip`;
  unmatched sides per fallback policy (`skip` = SKIP texture, `nearest` =
  largest matched side's texture, `trigger` = TRIGGER placeholder).
- Entity lump preserved verbatim (keys/order), bmodels get origin-brush
  semantics.
- `--grid-snap N` quantizes emitted plane points to the integer lattice
  (default 8, mapper convention).
- Reader = the extracted ericw-faithful parser (QuakeEd + Valve-220).
- Round-trip property: `parse(write(parse(m))) == parse(m)` for canonical
  (Valve-220, integral-point) brushes — unit-tested.

## 7. ML stages (M2/M3)

### 7.1 Route A — seam segmentation (M2, first ML scope)
Classify each edge/collinear-vertex-run along merged coplanar faces as
*brush seam* vs *interior*. No published ML T-junction/seam detector exists
[report §3.4 Route A]. Features: BRepNet-style region-adjacency graph over
planar cells (collinearity, T-vertex presence, length, texinfo transients,
planar offset, neighborhood context) + UV-Net-style rasterized per-face
descriptors. Model I/O: per-map graph (§9.4 schema) → per-edge logit;
compact enough for GoMLX pure-Go inference. Supervision from §9.4 labels.

### 7.2 Route B — plane→brush grouping (M2)
BSP-Net-style T-matrix formulation as a learned grouping/merging prior over
exact deterministic cells (not end-to-end shape generation) [report §3.4
Route B]. Doubles as the pure-Go training feasibility testbed (GoMLX
experimental GNN). Output: per-cell merge decisions.

### 7.3 Route C — brush program generation (M3, research)
DeepCAD-style 8-bit closed-set tokens; pointer heads over the BSP-derived
plane/texinfo set; DETR-style slots + Hungarian matching for unordered brush
sets; EBNF grammar masking + decode-time geometric checker (convexity,
closure, grid snap) for validity-constrained decoding; CSGNet-style IoU RL
where ground truth is partial. Feature-gated `--ml all`; synthetic-first
pilot.

### 7.4 Inference and the no-CGO rule
Repo convention: `CGO_ENABLED=0`, zero build tags [AGENTS.md]. ONNX Runtime
Go bindings are cgo-based → unusable in the main module. **Default inference:
GoMLX pure-Go backend** [report §3.5], via GoMLX-native training or
PyTorch → ONNX → GoMLX conversion. Optional cgo helper `tools/bspdec_onnx`
(yalue/onnxruntime_go) lives outside main-module CI and is never compiled
into `cmd/bspdec`. Models are versioned artifacts with `metadata.json`
(input schema, class labels, quantizer params, dataset SHA, qbsp pin, seed,
metrics) under `models/bspdec/<route>-v<semver>/`; provisioned by
`mise run bspdec-models`, never committed.

## 8. Evaluation (3 tiers)

1. **Recompile diff**: predicted `.map` + pinned in-repo qbsp → compare
   planes/leafs/faces lump stats + voxelized geometry IoU vs original (fast,
   coarse; reported by `--json`).
2. **Structural fidelity**: brush-count delta, plane-set precision/recall,
   layout graph-edit distance — tolerates different-but-valid decompositions
   [report §3.6].
3. **Behavioral parity**: fixed demo through C Ironwail on original `.bsp`
   vs recompiled map (existing parity harness); collision-trace occupancy
   invariants.

Engineered-out pitfalls: outside-fill masks leaks (compile success is not a
pass criterion — tier 3 is); qbsp pinned identically for train and eval;
classic-maps eval slice excluded from all training data (§9.5).

## 9. Dataset provisioning

Staged, idempotent, re-runnable pipeline owned by `tools/bspdec_corpus` +
`tools/bspdec_synth`, driven by `mise run bspdec-data`; every stage writes a
provenance log.

- **P1 enumerate**: Quaddicted API + `quaddicted-data` mirror +
  `fzwoch/quake_map_source` → `dataset/bspdec/raw/manifest.jsonl`
  (`{pkg_id, source_url, sha256, license_note, map_files[], bsp_files[],
  flags{brushlist, toolchain_guess, era}}`).
- **P2 fetch + validate**: download by sha256, verify, extract `.map`/`.bsp`;
  validate BSPs with `cmd/bspdiag`, maps with the `internal/map` reader.
- **P3 canonicalize**: pair `.map`↔`.bsp`; map-only entries compiled with the
  pinned in-repo qbsp twice (plain + BRUSHLIST, which our compiler always
  emits) → `dataset/bspdec/paired/<map_id>/`; bsp-only entries feed the
  classic holdout and weak-label generation.
- **P4 label derivation** (`internal/bspdec/labels.go`, deterministic):
  per-cell original-brush assignment (point-in-convex vs original halfspace
  sets), seam labels (original brush plane crosses merged face at that
  locus), BRUSHLIST truth → `dataset/bspdec/labeled/<map_id>/labels.json` +
  graph records (Route A/B training tensors).
- **P5 splits**: train/val/test at the **package** level (no map-author
  leakage); fixed vintage classic holdout used only for evaluation →
  `dataset/bspdec/splits.json`.
- **P6 synthetic** (`tools/bspdec_synth`): room-grammar generator on 8-unit
  lattices (blockout 64/32, detail 16/8, trim 8/4; stairs/doors multiples of
  8/16/64) [report §3.3]; perfect ground truth, arbitrary scale, no licensing
  issues; carries training volume.
- **P7 toolchain augmentation**: recompile samples with alternative
  toolchains for 30 years of compile quirks; recorded in
  `flags.toolchain_guess`.
- **Licensing**: per-package `license_note` mandatory; unlicensed packages
  stay local-only; nothing redistributed; synthetic preferred for bulk.

## 10. Training execution

`tools/bspdec_train` driven by `mise run bspdec-train`; pinned toolchain,
pinned seed, dataset SHA recorded in model metadata.

| Route | Prep | Train | Export | Gate |
| --- | --- | --- | --- | --- |
| A (seams) | §9.4 graph records | GoMLX GNN or PyTorch baseline; BCE on seam logits | GoMLX native or ONNX→GoMLX | seam F1 > deterministic split heuristic on val |
| B (grouping) | cell records + BRUSHLIST truth | GoMLX GNN; group-merge BCE/contrastive | same | group-recovery gain vs `--merge-convex` on val |
| C (programs) | DeepCAD-style tokens | PyTorch imitation → CSGNet-style IoU RL | ONNX→GoMLX | valid .map rate + compile parity on synthetic suite |

Automation: `bspdec-data` → `bspdec-train` → `bspdec-eval` →
`bspdec-report`; cacheable steps keyed by dataset SHA + config hash;
regression guard fails on metric regression vs recorded best. Hardware:
Routes A/B are minutes–hours on one GPU or strong CPU (GoMLX viable); Route
C is the only heavy lift (transformer, days on GPU, PyTorch primary).

## 11. Error handling

- CLI exit codes 0/1/2/3 per §3; the self-check (re-parse + brush validation:
  ≥4 sides, convex, non-empty windings) is mandatory before exit 0.
- Malformed BSP lumps, missing textures, degenerate windings, and sliver
  brushes degrade to per-model warnings (counted in `--json`), never panics.
- `--ml` set without provisioned models → exit 1 with a clear message (M2+).
- Corpus pipeline stages are idempotent and record failure reasons in the
  manifest instead of aborting the run.

## 12. Testing strategy

- Unit: winding clip, base winding, plane dedup, brush split, merge,
  un-expansion, writer round-trip identity, JSON schema.
- Golden: compile known `.map` fixtures with in-repo qbsp (BRUSHLIST always
  present → oracle), decompile, compare against BRUSHLIST truth and by
  recompile voxel-IoU; deterministic without external assets.
- Corpus: `quake-data` maps when present (skip via `internal/testutil`
  helpers otherwise); corpus-driven golden tests once provisioning lands.
- Eval tiers 1–2 automated in `bspdec-eval`; tier 3 via the parity harness.
- All tests: package-local helpers, `internal/testutil` for assets,
  `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ...` for single-package runs.

## 13. Milestones, gates, and bead map

| Ms | Beads | Scope | Exit criterion |
| --- | --- | --- | --- |
| M0 | xxy.1–.4 | `internal/map`, treewalk core, `cmd/bspdec`, eval suite + corpus | meets-or-beats bsputil on corpus; `mise run verify` green |
| M1 | xxy.5, .11 | BRUSHLIST path + synthetic generator + headroom study | quantified recovery ceiling → ML go/no-go gate |
| M2 | xxy.6, .7, .12 | Routes A+B + training pipeline + Go inference | seam/group accuracy > baselines on held-out corpus |
| M3 | xxy.8, .9 | Routes C+D research | valid .map program output on synthetic suite; compile+parity pass |
| M4 | xxy.10 | vintage shift slice, hardening, docs | full eval matrix incl. classic maps; README/manual |

Gates / stop conditions:

- **After M0**: if the treewalk core can't reach bsputil parity on the
  corpus, fix the core before any ML work — the baseline is the contract.
- **After M1 (the ML go/no-go gate)**: if oracle-perfect seam/grouping can't
  beat `bsputil` meaningfully, record no-go in xxy.5 and stop before M2
  [report §4 "ML may add little"].
- **During M2**: if Route A doesn't beat the deterministic split heuristic
  on held-out data, Route C is descoped; Route D becomes the primary
  research track.

Risks: distribution shift across 30 years of toolchains; licensing gray zone
(2006 id map sources carry no license grant — local only); eval gaming via
outside-fill (mitigated by tier 3); GoMLX API churn (pin versions); map-author
leakage (package-level splits).

## 14. Implementation conventions

- Pure Go, `CGO_ENABLED=0`, no build tags, `log/slog`, package-local test
  helpers, `internal/testutil` for assets, no `fmt.Println` logging
  [AGENTS.md].
- `.gitignore` is a whitelist: `dataset/bspdec/**` and `models/bspdec/**` are
  intentionally ignored; only tiny fixtures in `testdata/bspdec/` (add a
  whitelist rule when first needed). Provisioning via mise tasks.
- `mise.toml` tasks: `build-bspdec`, `bspdec-data`, `bspdec-train`,
  `bspdec-eval`, `bspdec-report`, `bspdec-models`; none join `verify`.
- Every algorithm port cites C lineage inline
  (`// Where in C: Q1_CreateBrushesFromBSP in bspc/map_q1.c`).

## 15. References

- Research: `~/.local/share/crush/research/quake-bsp-decompiler-ml/report.md`
- Beads: epic `ironwail-go-xxy` + children
- C lineage: id Q3A `bspc/map_q1.c`; ericw-tools `common/decompile.cc`,
  `common/polylib.cc`, `qbsp/qbsp.cc` (BRUSHLIST); id `WinQuake/bspfile.h`;
  Quake Specs v3.4 §4
