# BSPDEC M2: Training Pipeline + Model Registry (ironwail-go-xxy.12)

> Status: in progress (started 2026-09-09). Bead `ironwail-go-xxy.12`;
> unblocks Routes A/B (`xxy.6`, `xxy.7`). **Spec:** §7 (Routes A/B), §9.4
> (labels/graph records), §10 (training execution), §13 (gates), §3/§7.4
> (`--ml`, `--model-dir`, metadata.json).

**Goal (acceptance):** `mise run bspdec-train` produces a packaged Route A
model end-to-end from a fresh corpus; model metadata recorded under
`models/bspdec/`; the regression guard in `bspdec-report` fires on a
synthetic regression.

**M1 gate context:** the headroom study recorded no-go (mean ceiling gain
0.0028 < 0.02 on the synth slice) because the treewalk baseline already sits
at the BRUSHLIST ceiling on intact lattice geometry; real maps are blocked by
CSG-fidelity beads `xxy.6xi`/`xxy.aeh`. Follow-up bead `ironwail-go-gw8`
regrounds M2 expected gains. The xxy.12 acceptance is about the *pipeline*
(nothing to beat), so it proceeds; Route-gate F1 evaluations belong to
xxy.6/.7 and stay behind their own gates.

**Model backend decision (v0):** pure-stdlib gradient descent (logistic
regression on hand-built edge features) proves the full train→register→guard
loop with zero new dependencies, matching the repo's stdlib-only convention
until GoMLX is actually needed. The registry schema and loader interface
(`internal/bspdec/ml.go`) are versioned so a GoMLX-backed model can slot in
under the same `metadata.json` contract (spec §7.4 fields). GoMLX is
revisited when Route B GNN work starts.

---

## File structure

| File | Responsibility |
| --- | --- |
| `internal/bspdec/labels.go` (mod) | `SeamTruth` — original-brush seam supervision (replaces the texture proxy in `LabelCells`) |
| `internal/bspdec/labels_test.go` (mod) | seam-truth unit tests, updated texture-proxy test |
| `tools/bspdec_train/main.go` | CLI: `-data`, `-route`, `-seed`, `-out`; corpus scan; train/val split; cache keys (dataset SHA + config hash) |
| `tools/bspdec_train/features.go` | Route A per-edge feature extraction from label records |
| `tools/bspdec_train/train.go` | seed-pinned logistic regression loop, F1 on val |
| `tools/bspdec_train/export.go` | model.json + metadata.json packaging |
| `tools/bspdec_train/*_test.go` | determinism, feature extraction, packaging, guard |
| `internal/bspdec/ml.go` | model metadata + weight loader + deterministic fallback for `--ml seams` |
| `cmd/bspdec/main.go` (mod) | wire `--ml seams` to the loader + fallback |
| `tools/bspdec_report/main.go` (mod) | `--regress <models/bspdec>` guard comparing evals vs recorded best |
| `mise.toml` (mod) | real `bspdec-train`/`bspdec-models` tasks |
| `docs/superpowers/plans/...` | this plan |

---

### Task 1: Original-brush seam supervision (`SeamTruth`)

**Why first:** every `labels.json` today carries `"seams": null` — the
texture-transient proxy (`SeamEdges`) finds nothing because the CSG erases
coincident coplanar faces. Route A has zero supervision; a training pipeline
cannot train an edge classifier from an empty label set.

**Algorithm** (deterministic, on the pre-split cells + original brush plane
sets `brushPlaneSets`):

- For each cell side on plane P: collect original brush faces coplanar with
  P (base-winding stand-ins `BaseWinding(plane)`, clipped to the cell side
  via the existing `clipToBrush`+`segInside` machinery).
- Every pair of coplanar original faces whose abutting/overlap produces an
  edge of one face strictly inside the other's interior → that segment is a
  hidden brush seam (the merged face hides the join). `SeamLabel{Seam:true}`
  per segment.
- Single-face sides produce no seams.

Steps:

- [x] Write the failing unit tests: two coplanar-adjacent slabs → one seam at
      the junction; one slab → no seams; the two-slab fixture from the old
      texture test now yields a seam through `SeamTruth`.
- [x] Implement `SeamTruth` in `labels.go`; swap `LabelCells` to use it.
- [x] Run `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1`.
- [x] Regenerate corpus labels (synth stage for the 50 synthetic pairs; the
      labels stage for dm1/e1m8): all 52 files carry non-null seams.
- [x] Verify: 46,586 seam labels across 52 maps; every record `Seam: true`
      with a 2-point Edge; zero bad records.

### Task 2: `tools/bspdec_train` skeleton + cache keys

- [ ] CLI flags `-data`, `-route a|all`, `-seed`, `-out` (default `models/bspdec`).
- [ ] Corpus scan: read `raw/manifest.jsonl` + `labeled/*/*.labels.json` +
      `splits.json`; package-level train/val/test like `stageSplits`.
- [ ] Dataset SHA (sha256 of sorted labeled record bytes) + config hash
      (seed, route, feature schema version) — the cache keys for every step.
- [ ] Unit test: corpus scan + split consistency; cache keys stable.

### Task 3: Route A feature extraction

Per edge sample (from `SeamLabel` records + the side polygon they came
from — the label record must gain the source polygon/plane context in T1):
edge length, collinearity with the merged side, T-vertex presence, number of
coplanar original faces, per-face plane offsets, and the texture pair
(hashed). Normalized with recorded quantizer params. Failing unit tests on
deterministic fixtures first.

### Task 4: Route A v0 train loop

Seed-pinned logistic regression (BGD, fixed epochs/lr), train/val split from
Task 2, F1 metric on val; deterministic across runs and platforms. Tests: two
seeds/two runs identical; learned F1 > random on a separable fixture.

### Task 5: Registry + packaging + `--ml seams`

- [ ] `export.go`: writes `models/bspdec/route-a-vX/model.json` (schema
      version, weights, feature means/stds, class labels) +
      `metadata.json` (input schema, class labels, quantizer params, dataset
      SHA, pinned qbsp version, seed, metrics, timestamp).
- [ ] `internal/bspdec/ml.go`: `Model` metadata type + `LoadModel(dir)` +
      `Predict(edge)`; deterministic fallback (split heuristic) when the
      model file is absent.
- [ ] `cmd/bspdec`: `-ml seams` loads the model dir (default
      `models/bspdec`), else exits 1 with the spec §11 message; when loaded,
      marks seam edges in the emitted map (Route A first wiring).
- [ ] Tests: metadata round-trip; loader rejection of corrupt model.

### Task 6: Regression guard + mise tasks + acceptance

- [ ] `bspdec_report -regress models/bspdec`: compares fresh eval metrics to
      recorded best; exits non-zero on a synthetic regression (right now: a
      planted weight corruption).
- [ ] `mise.toml`: `bspdec-train` = `go run ./tools/bspdec_train`; 
      `bspdec-models` = `go run ./tools/bspdec_train -out models/bspdec` (or
      a provision subcommand).
- [ ] Acceptance run: `mise run bspdec-train` on a fresh `-data` corpus →
      packaged route-a model + metadata; `bspdec-report -regress` fires on
      the planted regression; `mise run verify` green.

---

## Wrap-up

- [ ] `mise run verify` green.
- [ ] Beads: close `xxy.12` with the packaged-model evidence; update `xxy.6`
      /`.7` notes that the pipeline + truth supervision exist.
- [ ] Report: files changed, model metadata, regression-guard evidence,
      and the M2 expected-gains note (`gw8`).

## Self-review

- **Spec coverage** — §10 table row A (prep: graph records → T1/T3; train:
  BCE loop → T4; export: registry → T5; gate: seam F1 vs split heuristic →
  delegated to xxy.6): yes. §9.4 graph records: T1 adds the missing piece.
  §7.4 metadata fields: T5. §11 `--ml` error: T5. §14 gitignore: `models/`
  already ignored by the whitelist default.
- **Constraint check** — stdlib only for v0 (GoMLX deferred, noted); no build
  tags; `log/slog` diagnostics; seeded `math/rand` for train; tests match the
  mise env.
- **Known risks** — (1) seam-truth definition is a judgment call (base-winding
  stand-ins for original faces); beats the null baseline and is the same
  geometric object Route A must classify, but if overlap-abutment semantics
  prove noisy on real maps, the corpus re-run is cheap. (2) v0 logistic
  regression is a feasibility proof, not a Route-A model that beats the split
  heuristic — that gate lives in xxy.6 and is unchanged. (3) `bspdec_train`
  re-compiles nothing; corpus regeneration is `bspdec-data`'s job
  (cacheable via dataset SHA).