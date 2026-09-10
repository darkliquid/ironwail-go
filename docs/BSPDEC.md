# bspdec — BSP-to-Map Decompiler

`bspdec` reconstructs Quake `.map` sources from compiled `.bsp` files,
with a deterministic core, a BRUSHLIST direct path, optional ML stages,
and a full corpus/training/tooling suite. Spec:
`docs/superpowers/specs/2026-09-07-bspdec-design.md`.

## CLI (`cmd/bspdec`)

```
bspdec [flags] input.bsp
  -o out.map              output path (default <input>.bspdec.map; - = stdout)
  -format map             only map output
  -no-brushlist           ignore the BRUSHLIST lump even when present
  -decompile-hull 1-3     collision-hull decompile instead of the render hull
  -merge-convex           merge same-contents coplanar-adjacent convex cells (default on)
  -grid-snap 8            quantize output to an integer lattice
  -texture-fallback       skip | nearest | trigger
  -ml seams               run the Route A seam classifier (needs -model-dir)
  -model-dir models/bspdec  provisioned model registry
  -json                   machine-readable summary on stdout
```

Pipeline per model: treewalk (or hull walk) → redundant-plane removal →
texturing → texture-boundary splits → convex merge → entity attach. When the
input BSP carries the BRUSHLIST BSPX lump (every compile by our qbsp does),
`Decompile` emits the original brushes directly instead of the treewalk
unless `-no-brushlist`; the treewalk remains the fallback and `-ml seams`
needs the lump for original-plane ground truth.

`--ml seams` scores every collinear segment along merged coplanar faces with
the packaged model and writes `<output>.seams.json` (per-candidate
probability + true seam flag from the lump). Exit codes: 0 ok, 1 input,
2 internal, 3 self-check failed.

## Corpus pipeline (`tools/bspdec_corpus`, `tools/bspdec_train`, `...`)

The dataset root is `dataset/bspdec/` (gitignored). Tasks (mise):

| Task | Action |
| --- | --- |
| `mise run bspdec-data` | enumerate sources → canonicalize (compile with pinned qbsp) → labels → splits |
| `mise run bspdec-train` | scan → extract Route A features → train → package `models/bspdec/route-a-v0.1.0/` |
| `mise run bspdec-eval` | tier-1/2 matrix over paired + synthetic pairs → `eval.json` |
| `mise run bspdec-report` | markdown matrix from `eval.json` |
| `mise run bspdec-models` | provision models (re-run train/package) |
| `mise run bspdec-check` | regression guard: val AUC vs the recorded best |
| `mise run bspdec-gate` | Route A held-out gate: ML F1 vs the deterministic split heuristic |

`tools/bspdec_synth` generates the deterministic synthetic corpus (room
grammar on an 8-unit lattice, sealed compiles, 1000 maps at seed 0x5EED);
`tools/bspdec_report headroom` runs the M1 baseline-vs-ceiling study.

### Labels and ML supervision

`internal/bspdec/labels.go` derives, per map: cell labels (each final cell
assigned to its original brush by point-in-convex) and seam labels
(`SeamTruth`): on each merged coplanar face, the edges of one original brush
face lying inside another's are the hidden brush joins Route A classifies.
`SeamCandidates` (`internal/bspdec/ml.go`) turns label records + face
geometry into the 7-feature route-a vectors shared by training and
inference, so the model sees identical inputs at both ends.

### Model registry

`models/bspdec/<route>-v<semver>/` holds `model.json` (schema, weights,
z-score quantizer, class labels) and `metadata.json` (input schema, classes,
quantizer, dataset SHA, qbsp pin, seed, metrics, timestamp). `registrar`
(`internal/bspdec/ml.go` `LoadSeamModel`) validates schema and weight count
and refuses corrupt artifacts. Regression guard: threshold-free val AUC
(pure-Go Mann-Whitney); a degenerate constant predictor scores 0.5 and can
never pass silently.

## Current results (2026-09-10, seed 0x5EED corpus)

- Headroom study (M1): mean BRUSHLIST-ceiling gain on the synthetic slice
  is 0.0028 voxel-IoU (threshold 0.02) — the treewalk already sits at the
  ceiling on intact lattice geometry → no-go for geometry-recovery framing.
- Route A (seam classification; bead xxy.6 ✓): held-out test F1 **0.945 vs
  the deterministic split heuristic 0.549**, test AUC 0.905; classic-map
  distribution-shift slice (dm1/e1m8, 30,870 samples, held out of
  training): AUC **0.940** — the synthetic-trained model transfers to real
  maps.
- Route B (cell grouping; bead xxy.7): pipeline + packaged
  `route-b-v0.1.0`; the held-out gate is inconclusive on the synthetic
  slice because the deterministic `--merge-convex` merger already recovers
  the grouping (no residual truth signal). Re-gate after the CSG-fidelity
  beads bring real-map labels.
- Eval matrix (1002 pairs: 1000 synthetic + dm1 + e1m8): synthetic rows at
  voxel IoU 1.000 with the BRUSHLIST direct path; classic rows carry the
  known CSG-fidelity caveats. See `docs/BSPDEC_EVAL_MATRIX.md`.

## Status / roadmap

M0 (deterministic core+eval) ✓, M1 (BRUSHLIST path + headroom gate) ✓, M2
(Routes A/B + training pipeline; A gated ✓, B inconclusive) ✓. Open: M3
research tracks (Routes C/D), M4 hardening (vintage slice, parity vs C
Ironwail), CSG-fidelity beads `ironwail-go-6xi`/`ironwail-go-aeh`
(real-map canonicalization, the unlock for richer labels + the headroom
re-run). Parity with C Ironwail remains the behavioral oracle
(`docs/PARITY.md`).
