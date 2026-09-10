# bspdec Operations Manual: The BSP-to-Map Decompiler & ML Pipeline

A comprehensive guide to decompiling Quake `.bsp` files back into editable `.map` files using the `bspdec` toolchain in `ironwail-go`.

---

## Table of Contents

1. [Executive Summary & Layman's Primer](#1-executive-summary--laymans-primer)
   - [What is a Quake .map File? (Brushes)](#what-is-a-quake-map-file-brushes)
   - [What is a Quake .bsp File? (The Binary Space Partition)](#what-is-a-quake-bsp-file-the-binary-space-partition)
   - [The Decompilation Challenge (Why is it hard?)](#the-decompilation-challenge-why-is-it-hard)
2. [Architecture: The Hybrid Decompilation Engine](#2-architecture-the-hybrid-decompilation-engine)
   - [Deterministic Treewalk](#deterministic-treewalk)
   - [The BRUSHLIST Oracle (BSPX)](#the-brushlist-oracle-bspx)
   - [The ML Routes (Routes A, B, C, D)](#the-ml-routes-routes-a-b-c-d)
3. [Complete Tool Inventory](#3-complete-tool-inventory)
   - [cmd/bspdec (The Main Decompiler CLI)](#cmdbspdec-the-main-decompiler-cli)
   - [tools/bspdec_corpus (Corpus Orchestrator)](#toolsbspdec_corpus-corpus-orchestrator)
   - [tools/bspdec_synth (Deterministic Synthetic Map Generator)](#toolsbspdec_synth-deterministic-synthetic-map-generator)
   - [tools/bspdec_train (ML Training & Packaging Pipeline)](#toolsbspdec_train-ml-training--packaging-pipeline)
   - [tools/bspdec_prog (Program Pilots: Routes C & D)](#toolsbspdec_prog-program-pilots-routes-c--d)
   - [tools/bspdec_report (Evaluation Matrix & Headroom Study)](#toolsbspdec_report-evaluation-matrix--headroom-study)
4. [The Machine Learning Pipeline Explained](#4-the-machine-learning-pipeline-explained)
   - [Step 1: Data Acquisition & Canonicalization](#step-1-data-acquisition--canonicalization)
   - [Step 2: Ground-Truth Label Derivation](#step-2-ground-truth-label-derivation)
   - [Step 3: Dataset Splitting](#step-3-dataset-splitting)
   - [Step 4: Feature Extraction (The 7 Route-A Features)](#step-4-feature-extraction-the-7-route-a-features)
   - [Step 5: Training & Quantization](#step-5-training--quantization)
   - [Step 6: Model Registry & Versioned Packaging](#step-6-model-registry--versioned-packaging)
   - [Step 7: Regression Guards & Gating](#step-7-regression-guards--gating)
   - [Step 8: Inference (Scoring Seams During Decompilation)](#step-8-inference-scoring-seams-during-decompilation)
5. [Operator's Cookbook: Step-by-Step Workflows](#5-operators-cookbook-step-by-step-workflows)
   - [Quickstart: Decompiling a Map](#quickstart-decompiling-a-map)
   - [Workflow: Generating the Synthetic Dataset](#workflow-generating-the-synthetic-dataset)
   - [Workflow: Training & Packaging a New Model](#workflow-training--packaging-a-new-model)
   - [Workflow: Running Validation & Gating Checks](#workflow-running-validation--gating-checks)
   - [Workflow: Evaluating the Entire Corpus Matrix](#workflow-evaluating-the-entire-corpus-matrix)
6. [Key Metrics & Terminology Reference](#6-key-metrics--terminology-reference)

---

## 1. Executive Summary & Layman's Primer

### What is a Quake .map File? (Brushes)

In Quake engine level design, maps are authored using **Constructive Solid Geometry (CSG)**. Level designers do not build models vertex-by-vertex; they carve and assemble rooms out of 3D convex polyhedra known as **brushes**.

A brush is defined mathematically by an intersection of infinite half-spaces (planes). For example, a standard cube brush is defined by 6 planes (top, bottom, north, south, east, west). Each face specifies:
1. Three 3D points defining its plane: `( 128 0 0 ) ( 128 64 0 ) ( 128 0 64 )`
2. The applied texture name, texture alignment offsets, rotation, and scaling factors.

Brushes are clean, intuitive, and easy to edit in mapping software like TrenchBroom.

```
       +--------------------+  <-- Original Brush 1 (Floor Slab)
       |                    |
+------+------+      +------+------+
|             |      |             | <-- Original Brush 2 & 3 (Pillars)
|   Pillar    |      |   Pillar    |
|             |      |             |
+-------------+      +-------------+
```

### What is a Quake .bsp File? (The Binary Space Partition)

When a mapper clicks "Compile", a tool named `qbsp` transforms the human-friendly `.map` into an optimized runtime structure called a **BSP (Binary Space Partitioning) tree**:

1. **Chopping & Slicing**: Every plane of every brush cuts across other brushes. The solid volume of the world is diced into dozens of tiny, non-overlapping convex fragments called **leaves** (or cells).
2. **Discarding Hidden Geometry**: Any internal face where two brushes touched each other is deleted (because players can never see inside solid walls).
3. **Discarding Brush Identity**: The concept of individual brushes completely disappears! The output `.bsp` file only contains the BSP node tree, leaves, planar polygon faces with lightmaps, and collision clipnodes.

```
Original Map Brushes                   Compiled BSP Tree Leaves
+-------------+-------------+          +-------+-----+-------+-----+
|   Room A    |   Room B    |  qbsp    | Leaf1 |Leaf2| Leaf3 |Leaf4| (Chop!)
|             |             | ------>  +-------+-----+-------+-----+
|             |             |          | Leaf5 |Leaf6| Leaf7 |Leaf8|
+-------------+-------------+          +-------+-----+-------+-----+
(2 neat author brushes)                (Dozens of chopped BSP leaf cells;
                                        internal seam plane deleted!)
```

### The Decompilation Challenge (Why is it hard?)

**Decompilation (`bspdec`) is the inverse problem:** Given only the chopped-up BSP tree leaves and external render faces, can we reconstruct the original, clean, editable brushes?

Decompilation faces three major obstacles:
1. **The Over-Segmentation Problem**: Without merging, a simple wall turns into 50 tiny, fragmented sliver brushes that are impossible for a human to edit.
2. **The Lost Seams Problem**: When coplanar adjacent leaf facets are merged into larger polygons, where did the original author's brush boundaries actually exist? Were they vertical, horizontal, or diagonal?
3. **Hidden / Void Faces**: BSP render geometry has no back-faces facing the void or facing other solid brushes. A decompiler must synthesize closed, watertight convex polyhedra from partial boundary faces.

---

## 2. Architecture: The Hybrid Decompilation Engine

`bspdec` solves this through a layered, hybrid approach:

```
                          Compiled .bsp File
                                   │
                 ┌─────────────────┴─────────────────┐
                 ▼                                   ▼
       Has BSPX BRUSHLIST lump?            Standard Legacy BSP?
                 │                                   │
                 ▼ (YES)                             ▼ (NO)
       ┌──────────────────┐               ┌──────────────────────┐
       │ BRUSHLIST Direct │               │ Deterministic        │
       │ Path (Ceiling)   │               │ Treewalk             │
       └─────────┬────────┘               └──────────┬───────────┘
                 │                                   │ (Convex leaf cells)
                 │                                   ▼
                 │                        ┌──────────────────────┐
                 │                        │ Redundant Plane      │
                 │                        │ Removal & Texturing  │
                 │                        └──────────┬───────────┘
                 │                                   │
                 │                                   ▼
                 │                        ┌──────────────────────┐
                 │                        │ Convex Cell Merge    │
                 │                        │ (-merge-convex)      │
                 │                        └──────────┬───────────┘
                 │                                   │
                 │                                   ▼
                 │                        ┌──────────────────────┐
                 │                        │ Optional ML Route A: │
                 │                        │ Seam Classification  │
                 │                        └──────────┬───────────┘
                 │                                   │
                 │                                   ▼
                 │                        ┌──────────────────────┐
                 │                        │ Grid Snap (8-unit) & │
                 │                        │ Self-Check           │
                 │                        └──────────┬───────────┘
                 │                                   │
                 └─────────────────┬─────────────────┘
                                   ▼
                         Valid Quake .map File
```

### Deterministic Treewalk
- Walks the BSP tree nodes and extracts convex polyhedra for all leaves marked `CONTENTS_SOLID`.
- **Redundant Plane Removal**: Removes interior division planes that don't form real boundaries.
- **Texture Assignment**: Maps BSP visual face textures onto decompiled brush sides. If a side was internal or faces the void, uses fallback policies (`nearest`, `skip`, or `trigger`).
- **Convex Merging (`-merge-convex`)**: Iteratively tests if adjacent cells sharing the same texture and contents can be unified into a single convex brush without introducing concave angles or re-splitting existing faces.
- **Grid Snapping (`-grid-snap 8`)**: Snaps brush plane coordinates to standard Quake grid increments (default 8 units).

### The BRUSHLIST Oracle (BSPX)
Modern compilers (including our in-repo `internal/qbsp`) can write an optional extra data block into the compiled BSP file: the `BSPX` format's `BRUSHLIST` lump.
- This lump contains the exact original authoring brushes.
- When `bspdec` detects this lump, it can bypass the treewalk and emit the exact original brushes directly (yielding a 100% perfect reconstruction).
- **Crucial Role**: Even when evaluating the treewalk or training ML models, the `BRUSHLIST` lump serves as the absolute ground truth ("the oracle").

### The ML Routes (Routes A, B, C, D)
To recover lost designer intent on legacy maps without a `BRUSHLIST` lump, `bspdec` implements research routes:
- **Route A (Seam Classifier)**: Predicts which collinear edges along merged coplanar faces are true brush seams. (Trained and shipped in production with `test F1 = 0.945`).
- **Route B (Cell Grouping)**: Clusters adjacent decompiled convex cells to decide if they belong to the same parent brush.
- **Route C (Plane-Program Synthesis)**: Tokenizes brushes as sequences of signed plane-table indices.
- **Route D (Search-Based Plane Decoder)**: Cost-guided beam search over canonical BSP planes to recover brushes.

---

## 3. Complete Tool Inventory

The repository provides a modular suite of tools located in `cmd/` and `tools/`:

```
ironwail-go/
├── cmd/
│   └── bspdec/            # Production CLI decompiler tool
├── tools/
│   ├── bspdec_corpus/     # Dataset provisioning, canonicalization & eval
│   ├── bspdec_synth/      # Deterministic procedural map generator
│   ├── bspdec_train/      # ML feature extraction, training & packaging
│   ├── bspdec_prog/       # Plane-program tokenization & beam-search pilots
│   └── bspdec_report/     # Evaluation matrix generator & headroom study
```

### `cmd/bspdec` (The Main Decompiler CLI)
The standalone binary used to decompile individual maps.

**Common Usage:**
```bash
# Basic decompilation
bspdec -o maps/e1m1.map maps/e1m1.bsp

# Decompile without using the embedded BRUSHLIST lump (forces treewalk)
bspdec -no-brushlist -o maps/e1m1.treewalk.map maps/e1m1.bsp

# Decompile with Route A seam prediction enabled
bspdec -no-brushlist -ml seams -model-dir models/bspdec -o maps/e1m1.map maps/e1m1.bsp

# Decompile collision hull 1 (player bounding box collision geometry)
bspdec -decompile-hull 1 -o maps/e1m1_hull1.map maps/e1m1.bsp

# Output machine-readable JSON telemetry
bspdec -json maps/e1m1.bsp
```

**Key Flags:**
- `-o <path>`: Destination `.map` file (`-` for stdout).
- `-no-brushlist`: Ignores the `BRUSHLIST` lump even if present (forces the geometric treewalk).
- `-merge-convex`: Enables convex cell merging (default: `true`).
- `-grid-snap <N>`: Snaps brush vertices to an $N$-unit grid (default: `8`, `0` disables).
- `-texture-fallback <policy>`: Policy for hidden faces: `nearest` (default), `skip`, or `trigger`.
- `-ml seams`: Runs the Route A ML seam classifier.
- `-model-dir <dir>`: Path to packaged ML models (default: `models/bspdec`).
- `-json`: Emits structured JSON summary to stdout.

**Exit Codes:**
- `0`: Success.
- `1`: Input error (bad flags, missing file).
- `2`: Internal decompilation error.
- `3`: Self-check failed (decompiled map failed to re-parse or contained invalid brushes).

---

### `tools/bspdec_corpus` (Corpus Orchestrator)
Manages the dataset lifecycle across raw source maps, paired `.map`/`.bsp` files, ground-truth labels, and train/val/test splits.

**Subcommands:**
```bash
# 1. Enumerate: Discover raw map sources
go run ./tools/bspdec_corpus enumerate -data dataset/bspdec

# 2. Canonicalize: Compile all raw maps with our pinned qbsp
go run ./tools/bspdec_corpus canonicalize -data dataset/bspdec

# 2b. Canonicalize a single package (for fast debugging):
go run ./tools/bspdec_corpus canonicalize-onemap -data dataset/bspdec -pkg e1m1

# 3. Labels: Derive cell and seam ground-truth labels
go run ./tools/bspdec_corpus labels -data dataset/bspdec

# 4. Splits: Compute train/val/test split manifests
go run ./tools/bspdec_corpus splits -data dataset/bspdec

# 5. Eval: Run full evaluation matrix over the corpus
go run ./tools/bspdec_corpus eval -data dataset/bspdec
```

---

### `tools/bspdec_synth` (Deterministic Synthetic Map Generator)
Generates procedural Quake maps with known, mathematically sound ground-truth brushes.

**Why is this needed?**
Classic human-authored Quake maps often contain messy, non-axial, or open brushes with stray faces. To train and test ML models under clean, controlled conditions, `bspdec_synth` uses a room grammar on an 8-unit lattice to generate hundreds of guaranteed-sealed maps.

**Usage:**
```bash
# Generate 1,000 synthetic maps using seed 0x5EED
go run ./tools/bspdec_synth/cmd/bspdec_synth -data dataset/bspdec -count 1000 -seed 0x5EED
```

---

### `tools/bspdec_train` (ML Training & Packaging Pipeline)
Extracts feature vectors, trains logistic classifiers, runs regression guards, and packages versioned models.

**Usage:**
```bash
# Train and package Route A (seams) model into models/bspdec/
go run ./tools/bspdec_train -data dataset/bspdec -route a -out models/bspdec

# Run regression guard (fails if validation AUC drops or dataset drifts)
go run ./tools/bspdec_train -data dataset/bspdec -validate -out models/bspdec

# Run the Route A held-out acceptance gate (checks if ML beats heuristic)
go run ./tools/bspdec_train -data dataset/bspdec -gate -route a
```

---

### `tools/bspdec_prog` (Program Pilots: Routes C & D)
Researches the representation of brushes as sequences of signed plane-table index tokens ("brush programs").

**Usage:**
```bash
# Run Route C pilot (direct BRUSHLIST plane-index tokenization)
go run ./tools/bspdec_prog -data dataset/bspdec -count 20

# Run Route D pilot (cost-guided beam-search plane recovery)
go run ./tools/bspdec_prog -data dataset/bspdec -count 20 -search
```

---

### `tools/bspdec_report` (Evaluation Matrix & Headroom Study)
Generates markdown reports from evaluation results and runs the headroom comparison.

**Usage:**
```bash
# Generate markdown eval matrix from dataset/bspdec/eval.json
go run ./tools/bspdec_report -data dataset/bspdec

# Run the M1 Headroom Study (treewalk baseline vs BRUSHLIST ceiling)
go run ./tools/bspdec_report headroom -data dataset/bspdec
```

---

## 4. The Machine Learning Pipeline Explained

A major innovation of `bspdec` is its supervised learning pipeline for predicting lost brush seams. Here is how raw geometry becomes a trained, packaged model:

```
[Raw .map Files]
       │
       ▼ (1. Compile with pinned qbsp)
[Paired .bsp Files with BRUSHLIST]
       │
       ▼ (2. Extract Labels & SeamTruth)
[labels.json (Cell & Seam Ground Truth)]
       │
       ▼ (3. Train / Val / Test Splits)
[splits.json]
       │
       ▼ (4. Geometric Feature Extraction)
[7-Dimensional Feature Vectors]
       │
       ▼ (5. Train Pure-Go Logistic Classifier)
[Quantized Weights + Intercepts]
       │
       ▼ (6. Package & Version)
[models/bspdec/route-a-v0.1.0/ (model.json + metadata.json)]
       │
       ▼ (7. Inference in cmd/bspdec)
[Decompiled Map + .seams.json Predictions]
```

### Step 1: Data Acquisition & Canonicalization
Training requires paired pairs: an original `.map` file and its compiled `.bsp` counterpart.
- Source `.map` files are gathered from the ID1 corpus (`quake_map_source`) and procedural generation (`bspdec_synth`).
- `bspdec_corpus canonicalize` compiles each `.map` into `dataset/bspdec/paired/<pkg>/<name>.bsp` using the pinned `qbsp` compiler.
- The compiled `.bsp` includes the `BRUSHLIST` lump, preserving the ground-truth authoring brushes.

### Step 2: Ground-Truth Label Derivation
For each paired map, `tools/bspdec_corpus labels` derives two forms of truth:
1. **Cell Labels**: For every convex cell extracted by the treewalk, tests which original brush's convex volume enclosed its centroid (`internal/bspdec/labels.go`).
2. **Seam Truth (`SeamTruth`)**: When coplanar leaf faces are merged into larger composite faces, the interior dividing lines are tested against the original brush faces. If a line segment was a boundary of an original brush, it is labeled `Seam = true`; if it was just an artificial cut made by BSP node slicing, it is labeled `Seam = false`.

### Step 3: Dataset Splitting
`tools/bspdec_corpus splits` partitions all packages into:
- **`train` (70%)**: Used to compute features and train classifier weights.
- **`val` (15%)**: Used to tune thresholds and evaluate the regression guard.
- **`test` (15%)**: Held-out set used strictly for the final acceptance gate.
- **`classic-holdout`**: Classic vanilla Quake maps kept out of training to measure real-world distribution shift.

### Step 4: Feature Extraction (The 7 Route-A Features)
For each candidate edge along a merged face, `internal/bspdec/ml.go` computes a 7-dimensional normalized feature vector:

| Feature Index | Feature Formula | Geometric Rationale |
|---|---|---|
| **$f_0$** | `length / 8.0` | Edge length normalized to the standard 8-unit Quake grid lattice. |
| **$f_1$** | `|plane.Normal.X|` | X-axis component of the host face normal. |
| **$f_2$** | `|plane.Normal.Y|` | Y-axis component of the host face normal. |
| **$f_3$** | `|plane.Normal.Z|` | Z-axis component of the host face normal (distinguishes floors/ceilings from vertical walls). |
| **$f_4$** | `dist(midpoint, centroid) / sqrt(area)` | Distance from edge midpoint to face center, normalized by face scale. |
| **$f_5$** | `log(1 + num_seams)` | Log-count of other candidate seams on this face (topological complexity). |
| **$f_6$** | `shared_endpoints` | Number of neighboring seam segments sharing vertices with this segment. |

> **Critical Parity Rule**: The exact same function (`seamFeatures` in `internal/bspdec/ml.go`) is called during both offline training and online CLI inference, ensuring zero train-serving skew.

### Step 5: Training & Quantization
`tools/bspdec_train` trains a logistic regression model:
- **Pure-Go Implementation**: No external CGO, Python, or PyTorch dependencies.
- **Z-Score Normalization**: Features are normalized using mean and standard deviation:
  $$z_i = \frac{f_i - \mu_i}{\sigma_i}$$
- **Logistic Probability**:
  $$P(\text{is\_seam}) = \frac{1}{1 + e^{-(\mathbf{w} \cdot \mathbf{z} + b)}}$$
- **Seed-Pinned Determinism**: Seed `0x5EED` guarantees identical weights across all machines and architectures.

### Step 6: Model Registry & Versioned Packaging
Trained models are packaged in `models/bspdec/<route>-v<semver>/`:
- **`model.json`**:
  - `schema`: e.g. `"route-a-model-v1"`
  - `weights`: The learned 7 float64 weights
  - `intercept`: The learned float64 bias
  - `quantizer`: The 7 means ($\mu$) and standard deviations ($\sigma$)
- **`metadata.json`**:
  - Training metrics (val AUC, test F1, precision, recall)
  - Provenance: `dataset_sha`, `qbsp_pin`, `timestamp`, `seed`

### Step 7: Regression Guards & Gating
`bspdec_train` includes automated sanity gates:
- **The Regression Guard (`-validate`)**: Computes the Mann-Whitney AUC over the validation set. If code changes degrade model AUC below the recorded benchmark, the build fails.
- **The Held-Out Gate (`-gate`)**: Evaluates the model against a deterministic split heuristic baseline. The ML model must significantly beat the baseline ($F_1 \ge 0.85$ vs $0.55$) on unseen test maps.

### Step 8: Inference (Scoring Seams During Decompilation)
When a user runs `bspdec -ml seams input.bsp`:
1. `bspdec` decompiles the geometry.
2. It loads `models/bspdec/route-a-v0.1.0/model.json`.
3. It extracts candidates and scores every edge with $P(\text{is\_seam})$.
4. It outputs `<input>.bspdec.map` and writes `<input>.bspdec.map.seams.json` containing the classified probabilities for editor visualization.

---

## 5. Operator's Cookbook: Step-by-Step Workflows

All standard operations are wrapped as canonical `mise` tasks defined in `mise.toml`.

### Quickstart: Decompiling a Map
```bash
# Build the decompiler binary
mise run build-bspdec

# Decompile any Quake map
./bin/bspdec -o my_map.map path/to/level.bsp
```

### Workflow: Generating the Synthetic Dataset
To regenerate the 1,000 synthetic test levels:
```bash
# Generates 1,000 maps under dataset/bspdec/synth/24301/
go run ./tools/bspdec_synth/cmd/bspdec_synth -data dataset/bspdec -count 1000 -seed 0x5EED
```

### Workflow: Training & Packaging a New Model
To train Route A after updating features or the dataset:
```bash
# Step 1: Extract features and train Route A
mise run bspdec-train

# Step 2: Re-provision and verify model artifacts
mise run bspdec-models
```

### Workflow: Running Validation & Gating Checks
To verify that the model has not suffered a regression:
```bash
# Run the regression guard (validates AUC against recorded metadata)
mise run bspdec-check

# Run the held-out performance gate (verifies ML beats heuristic baseline)
mise run bspdec-gate
```

### Workflow: Evaluating the Entire Corpus Matrix
To score all real and synthetic pairs across the dataset:
```bash
# Run batch evaluation across all pairs
mise run bspdec-eval

# Generate the Markdown evaluation summary
mise run bspdec-report
```

---

## 6. Key Metrics & Terminology Reference

- **Voxel IoU (Intersection over Union)**:
  Measures 3D geometric fidelity. The bounding box of the map is discretized into a 3D grid of 8-unit voxels. Solid voxels in the original BSP are compared against solid voxels in the recompiled decompiled map:
  $$\text{IoU} = \frac{|V_{\text{orig}} \cap V_{\text{recomp}}|}{|V_{\text{orig}} \cup V_{\text{recomp}}|}$$
  - $\text{IoU} = 1.000$: Perfect 1:1 solid volume reproduction.
  - $\text{IoU} \ge 0.900$: High-fidelity passing compile.
- **BrushDelta ($\Delta \text{brush}$)**:
  $$\Delta \text{brush} = N_{\text{decompiled brushes}} - N_{\text{original brushes}}$$
  Indicates over-segmentation. A lower delta means the decompiler did a better job merging chopped cells back into clean designer brushes.
- **Seam**:
  The shared boundary edge where two original brushes touched each other.
- **Face / Winding**:
  A planar convex polygon representing one side of a 3D brush.
- **Lattice / Grid Snap**:
  Quantizing plane distances and vertices to integer coordinates (standard Quake mapping uses multiples of 8 or 16 units) to avoid floating-point micro-cracks.
- **Self-Check**:
  A safety pass in `bspdec`: before writing `.map` output, the decompiler parses its own emitted text back into memory and verifies that all brushes are closed, valid, and non-degenerate.
