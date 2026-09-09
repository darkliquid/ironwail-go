# BSPDEC M0: Eval Suite + Corpus Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `bspdec` an objective definition of done: an evaluation suite (BSP lump stats + voxelized IoU) and an idempotent corpus provisioning pipeline that produces real `.map`/`.bsp` pairs, so the M0 acceptance ("meets-or-beats bsputil on the corpus") can be measured.

**Architecture:** New package `internal/bspdec/eval` provides production voxel/lump metrics and the corpus manifest (JSONL, spec §9.1 schema). `internal/bspdec/labels.go` derives supervision labels (cell→original-brush assignment, seam edges) used by P4 and by M2. `tools/bspdec_corpus` runs the stages: enumerate (clone `fzwoch/quake_map_source` + optional Quaddicted API), validate, canonicalize (in-repo qbsp compiles map-only entries; our compiler always appends the BRUSHLIST oracle), labels, package-level splits, and the tier 1+2 eval matrix. `mise run bspdec-data/eval/report` become real.

**Tech Stack:** Go 1.26, stdlib only; existing `internal/bsp`, `internal/bspdec`, `pkg/map`, `internal/qbsp`; network via `net/http` + `git clone` in the tool only.

**Spec:** [docs/superpowers/specs/2026-09-07-bspdec-design.md](../specs/2026-09-07-bspdec-design.md) §8 (evaluation tiers), §9 (dataset provisioning). Plan 1 (M0 decompiler) is committed and closed.

**Bead:** `ironwail-go-xxy.4` (M0 eval suite + corpus bootstrap). Claim it before Task 1; close when the acceptance gate (task 7) records numbers.

## Global Constraints

- Pure Go, `CGO_ENABLED=0`, no build tags, `log/slog`, no `fmt.Println` logging (CLI output excepted), per AGENTS.md.
- Single-package test runs: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -run TestX -count=1`.
- `mise run verify` green at every task boundary; `mise run lint` clean at the end.
- The corpus is an artifact, never committed: `dataset/bspdec/**` stays gitignored by the whitelist. Only `testdata/bspdec/room.map` (tiny fixture) is committed — add the whitelist rule in Task 1.
- Every stage must be **idempotent and re-runnable**: re-running resumes/repairs in place and appends provenance (`provenance.log` lines with stage, time, counts).
- Network stages degrade gracefully: on failure they log and skip, never abort the pipeline (offline-first).
- Package-level splits only (no map-author leakage); vintage bsp-only entries land in `classic_holdout`.
- Cites lineage inline where ports exist (none of this phase is a C port; label derivation is new logic, cite bspc's cell concept per `map_q1.c` where relevant).

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/bspdec/eval/doc.go` | package doc |
| `internal/bspdec/eval/stats.go` | `LumpStats`, `BSPStats`, `PointInSolid`, `OccupancyMap`, `VoxelIoU` (production ports of the M0 test helpers) |
| `internal/bspdec/eval/corpus.go` | `ManifestEntry`, `Flags`, `LoadManifest`, `AppendToManifest`, `ValidateEntry` |
| `internal/bspdec/eval/compile.go` | `CompileMapPair` (pinned in-repo qbsp), `WriteBSPXBrushlist` oracle check |
| `internal/bspdec/labels.go` | `CellLabel`, `LabelResult`, `LabelCells` (cell assignment + seam edges) |
| `internal/bspdec/labels_test.go` | label tests on room + two-texture synthetic |
| `tools/bspdec_corpus/main.go` | stage CLI: `enumerate`, `canonicalize`, `labels`, `splits`, `eval`, `report` |
| `tools/bspdec_corpus/stages_test.go` | stage funcs on a temp dataset |
| `testdata/bspdec/room.map` | committed offline fixture (sealed 6-brush room) |
| `mise.toml`, `.gitignore` | real data/eval/report tasks; `testdata/bspdec/**` allow rule |

---

## Task 1: eval scaffold + lump stats + voxel IoU

Bead: `ironwail-go-xxy.4`. The metrics are the "done" definition; make them production code with unit tests first (they are ports of the M0 golden-test helpers, adapted to work on raw BSP bytes).

**Files:**
- Create: `internal/bspdec/eval/doc.go`, `internal/bspdec/eval/stats.go`, `internal/bspdec/eval/stats_test.go`
- Modify: `.gitignore` (allow `testdata/bspdec/**`), `testdata/bspdec/room.map` (fixture)

**Interfaces:**
- Consumes: `internal/bsp` (`Load`, `LoadTree`, `File.Version`, tree fields), `internal/qbsp` (`Compile`, `ReadBSPXBrushList`) in tests only.
- Produces:
  - `type LumpStats struct { Planes, Nodes, Leafs, Faces, Edges, Clipnodes int }`
  - `func BSPStats(data []byte) (LumpStats, error)`
  - `func PointInSolid(tree *bsp.Tree, p mapfile.Vec3) bool`
  - `func OccupancyMap(data []byte, cell float64, cmins, cmaxs mapfile.Vec3) (map[[3]int]struct{}, error)` — solid lattice cells over the clamped region
  - `func VoxelIoU(a, b map[[3]int]struct{}) float64`

- [ ] **Step 1: Add the gitignore rule and fixture**

Append to `.gitignore` (it is a whitelist; add an exception before the catch-all re-checks matter — place with the other `!` exceptions):

```
!testdata/bspdec/**
```

Create `testdata/bspdec/room.map` (QuakeEd format; winding pattern compile-proven by qbsp's suite):

```
// bspdec offline fixture: sealed room, interior x,y in [0,256], z in [0,192]
{
"classname" "worldspawn"
{
( 0 -64 -64 ) ( 0 -64 256 ) ( 0 320 -64 ) wwall 0 0 0 1 1
( -64 320 -64 ) ( -64 320 256 ) ( -64 -64 256 ) wwall 0 0 0 1 1
( -64 320 -64 ) ( 0 320 -64 ) ( -64 320 256 ) wwall 0 0 0 1 1
( -64 -64 -64 ) ( -64 -64 256 ) ( 0 -64 -64 ) wwall 0 0 0 1 1
( -64 -64 256 ) ( -64 320 256 ) ( 0 -64 256 ) wwall 0 0 0 1 1
( -64 -64 -64 ) ( 0 -64 -64 ) ( -64 320 -64 ) wwall 0 0 0 1 1
}
{
( 320 -64 -64 ) ( 320 -64 256 ) ( 320 320 -64 ) wwall 0 0 0 1 1
( 256 320 -64 ) ( 256 320 256 ) ( 256 -64 256 ) wwall 0 0 0 1 1
( 256 320 -64 ) ( 320 320 -64 ) ( 256 320 256 ) wwall 0 0 0 1 1
( 256 -64 -64 ) ( 256 -64 256 ) ( 320 -64 -64 ) wwall 0 0 0 1 1
( 256 -64 256 ) ( 256 320 256 ) ( 320 -64 256 ) wwall 0 0 0 1 1
( 256 -64 -64 ) ( 320 -64 -64 ) ( 256 320 -64 ) wwall 0 0 0 1 1
}
{
( 256 -64 -64 ) ( 256 -64 256 ) ( 256 0 -64 ) wwall 0 0 0 1 1
( 0 0 -64 ) ( 0 0 256 ) ( 0 -64 256 ) wwall 0 0 0 1 1
( 0 0 -64 ) ( 256 0 -64 ) ( 0 0 256 ) wwall 0 0 0 1 1
( 0 -64 -64 ) ( 0 -64 256 ) ( 256 -64 -64 ) wwall 0 0 0 1 1
( 0 -64 256 ) ( 0 0 256 ) ( 256 -64 256 ) wwall 0 0 0 1 1
( 0 -64 -64 ) ( 256 -64 -64 ) ( 0 0 -64 ) wwall 0 0 0 1 1
}
{
( 256 256 -64 ) ( 256 256 256 ) ( 256 320 -64 ) wwall 0 0 0 1 1
( 0 320 -64 ) ( 0 320 256 ) ( 0 256 256 ) wwall 0 0 0 1 1
( 0 320 -64 ) ( 256 320 -64 ) ( 0 320 256 ) wwall 0 0 0 1 1
( 0 256 -64 ) ( 0 256 256 ) ( 256 256 -64 ) wwall 0 0 0 1 1
( 0 256 256 ) ( 0 320 256 ) ( 256 256 256 ) wwall 0 0 0 1 1
( 0 256 -64 ) ( 256 256 -64 ) ( 0 320 -64 ) wwall 0 0 0 1 1
}
{
( 256 0 0 ) ( 256 0 -64 ) ( 256 256 0 ) ffloor 0 0 0 1 1
( 0 256 0 ) ( 0 256 -64 ) ( 0 0 0 ) ffloor 0 0 0 1 1
( 0 0 0 ) ( 256 0 0 ) ( 0 256 0 ) ffloor 0 0 0 1 1
( 256 0 -64 ) ( 0 0 -64 ) ( 256 256 -64 ) ffloor 0 0 0 1 1
( 0 0 -64 ) ( 0 256 -64 ) ( 256 0 -64 ) ffloor 0 0 0 1 1
( 256 256 -64 ) ( 0 256 -64 ) ( 256 0 0 ) ffloor 0 0 0 1 1
}
{
( 0 256 192 ) ( 256 256 192 ) ( 0 0 192 ) cceil 0 0 0 1 1
( 256 256 192 ) ( 256 0 192 ) ( 0 256 192 ) cceil 0 0 0 1 1
( 256 0 192 ) ( 0 0 192 ) ( 256 256 192 ) cceil 0 0 0 1 1
( 0 256 256 ) ( 0 0 256 ) ( 256 256 256 ) cceil 0 0 0 1 1
( 0 0 256 ) ( 256 0 256 ) ( 0 256 256 ) cceil 0 0 0 1 1
( 256 0 256 ) ( 256 256 256 ) ( 0 0 256 ) cceil 0 0 0 1 1
}
}
{
"classname" "info_player_start"
"origin" "128 128 64"
}
```

> The implementer must verify this fixture compiles without a leak before
> moving on (qbsp leak detection needs the player start inside the cavity).

- [ ] **Step 2: Write the failing tests**

Create `internal/bspdec/eval/stats_test.go`:

```go
package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// compileRoomFixture compiles the committed fixture and returns its BSP bytes.
func compileRoomFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bspdec", "room.map"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, err := mapfile.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res, err := qbspCompile(m)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatal("fixture room leaks")
	}
	return res.Data
}

func TestBSPStats(t *testing.T) {
	data := compileRoomFixture(t)
	st, err := BSPStats(data)
	if err != nil {
		t.Fatalf("BSPStats: %v", err)
	}
	if st.Faces == 0 || st.Planes == 0 || st.Leafs == 0 {
		t.Fatalf("stats empty: %+v", st)
	}
}

func TestOccupancySelfIoU(t *testing.T) {
	data := compileRoomFixture(t)
	a, err := OccupancyMap(data, 8, mapfile.Vec3{X: -64, Y: -64, Z: -64}, mapfile.Vec3{X: 320, Y: 320, Z: 256})
	if err != nil {
		t.Fatalf("OccupancyMap: %v", err)
	}
	if len(a) == 0 {
		t.Fatal("empty occupancy")
	}
	if v := VoxelIoU(a, a); v < 0.999 {
		t.Fatalf("self IoU = %v", v)
	}
}

func TestOccupancyDiffersForRearrangedGeometry(t *testing.T) {
	data := compileRoomFixture(t)
	bounds := mapfile.Vec3{X: -64, Y: -64, Z: -64}
	bounds2 := mapfile.Vec3{X: 320, Y: 320, Z: 256}
	_, err := OccupancyMap(data, 8, bounds, bounds2)
	if err != nil {
		t.Fatalf("OccupancyMap: %v", err)
	}
	// a shifted scan region (z only floor half) must differ substantially
	floorOnly, err := OccupancyMap(data, 8, bounds, mapfile.Vec3{X: 320, Y: 320, Z: 0})
	if err != nil {
		t.Fatalf("OccupancyMap: %v", err)
	}
	full := baselineOccupancy(t, data)
	if v := VoxelIoU(full, floorOnly); v > 0.8 {
		t.Fatalf("floor-only IoU too high: %v — scan region clamp broken", v)
	}
}

// qbspCompile is the pinned in-repo compiler wrapper (moved to its own file
// in Task 3; defined here until then).
func qbspCompile(m *mapfile.Map) (*qbspRes, error) { return nil, errNotYet }

type qbspRes struct{ Data []byte; Leaked bool }

var errNotYet = errors.New("qqq")
```

Note the seeded compile-error: `qbspCompile`, `qbspRes`, `errNotYet`, `baselineOccupancy` are deliberately undefined until steps 3–4; the test package will fail to compile until then. Define `baselineOccupancy(t, data)` as a tiny wrapper over `OccupancyMap` with the full bounds (the two-line inline helper belongs in the test file).

- [ ] **Step 3: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -count=1`
Expected: FAIL — package does not compile (no Go files in eval).

- [ ] **Step 4: Implement `doc.go` + `stats.go`**

`internal/bspdec/eval/doc.go`:

```go
// Package eval measures decompiler fidelity: BSP lump statistics and
// voxelized occupancy IoU between an original BSP and a recompiled map.
// It also owns the corpus manifest format (spec section 9.1).
package eval
```

`internal/bspdec/eval/stats.go` (production ports of the M0 test helpers):

```go
package eval

import (
	"bytes"
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// LumpStats counts the raw lump records of a BSP (tier-1 coarse diff).
type LumpStats struct {
	Planes    int
	Nodes     int
	Leafs     int
	Faces     int
	Edges     int
	Clipnodes int
}

// BSPStats loads data and returns lump record counts.
func BSPStats(data []byte) (LumpStats, error) {
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		return LumpStats{}, err
	}
	var st LumpStats
	st.Planes = len(f.Planes)
	st.Edges = edgeCount(f)
	st.Clipnodes = clipnodeCount(f)
	// faces/leafs/nodes are version-typed; count via the shared accessors.
	st.Faces, st.Leafs, st.Nodes = faceCount(f), leafCount(f), nodeCount(f)
	return st, nil
}
```

Implement the four small accessors with type switches on `f.Nodes`/`f.Leafs`/`f.Faces`/`f.Clipnodes`/`f.Edges` (`any` fields: `[]bsp.DSNode`/`[]bsp.DL1Node`/`[]bsp.DL2Node`, `[]bsp.DSLeaf`/`[]bsp.DL1Leaf`/`[]bsp.DL2Leaf`, `[]bsp.DSFace`/`[]bsp.DLFace`, `[]bsp.DSClipNode`/`[]bsp.DLClipNode`, `[]bsp.DSEdge`/`[]bsp.DLEdge`; lengths only — the pattern is a small `len` per case).

```go
// PointInSolid classifies a point against the render tree: solid iff the
// reached leaf is non-empty.
//
// Where in C: SV_PointContents-style BSP descent in sv_main.c.
func PointInSolid(tree *bsp.Tree, p mapfile.Vec3) bool {
	idx := 0
	for {
		if idx < 0 || idx >= len(tree.Nodes) {
			return true // fell off the tree: treat as solid (Quake convention)
		}
		node := &tree.Nodes[idx]
		pl := tree.Planes[node.PlaneNum]
		d := float64(p.X)*float64(pl.Normal.X) + float64(p.Y)*float64(pl.Normal.Y) + float64(p.Z)*float64(pl.Normal.Z) - float64(pl.Dist)
		child := &node.Children[0]
		if d < 0 {
			child = &node.Children[1]
		}
		if child.IsLeaf {
			return tree.Leafs[child.Index].Contents != bsp.ContentsEmpty
		}
		idx = child.Index
	}
}

// OccupancyMap rasterizes the solid region of a BSP: lattice cell centers
// inside solid leaves, over the clamped region.
func OccupancyMap(data []byte, cell float64, cmins, cmaxs mapfile.Vec3) (map[[3]int]struct{}, error) {
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("no models in BSP")
	}
	m := tree.Models[0]
	x0 := math.Floor(math.Max(float64(m.BoundsMin.X), cmins.X)/cell) * cell
	y0 := math.Floor(math.Max(float64(m.BoundsMin.Y), cmins.Y)/cell) * cell
	z0 := math.Floor(math.Max(float64(m.BoundsMin.Z), cmins.Z)/cell) * cell
	x1 := math.Min(float64(m.BoundsMax.X), cmaxs.X)
	y1 := math.Min(float64(m.BoundsMax.Y), cmaxs.Y)
	z1 := math.Min(float64(m.BoundsMax.Z), cmaxs.Z)
	occ := map[[3]int]struct{}{}
	for x := x0; x < x1; x += cell {
		for y := y0; y < y1; y += cell {
			for z := z0; z < z1; z += cell {
				if PointInSolid(tree, mapfile.Vec3{X: x + cell/2, Y: y + cell/2, Z: z + cell/2}) {
					occ[[3]int{int(x / cell), int(y / cell), int(z / cell)}] = struct{}{}
				}
			}
		}
	}
	return occ, nil
}

// VoxelIoU returns |A∩B| / |A∪B|.
func VoxelIoU(a, b map[[3]int]struct{}) float64 {
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
```

The test step text says `math` is needed — import it. Replace the seeded stubs in the test:
- delete `qbspCompile`/`qbspRes`/`errNotYet` stubs; use `qbsp.Compile(m, qbsp.Options{})` directly via a local helper (the compile helper moves to `compile.go` in Task 3; for Task 1 the test calls `internal/qbsp` directly).
- add `baselineOccupancy(t, data)` helper + `"errors"` import removed.

- [ ] **Step 5: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -count=1 && mise run verify`
Expected: PASS (fixture compiles leak-free; stats non-zero; self-IoU ≈ 1; clamp respected).

- [ ] **Step 6: Commit**

```bash
git add internal/bspdec/eval/.gitignore testdata/bspdec/room.map
git commit -m "feat(bspdec): add eval metrics (lump stats, voxel IoU) and offline fixture"
```

---

## Task 2: Corpus manifest (JSONL)

Bead: `ironwail-go-xxy.4`. Spec §9.1 schema, idempotent append/update.

**Files:**
- Create: `internal/bspdec/eval/corpus.go`, `internal/bspdec/eval/corpus_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `type Flags struct { Brushlist bool; ToolchainGuess, Era string; ClassicHoldout bool }` (json tags per spec)
  - `type ManifestEntry struct { PkgID, SourceURL, SHA256, LicenseNote string; MapFiles, BSPFiles []string; Flags Flags }`
  - `func LoadManifest(path string) ([]ManifestEntry, error)` (nil, nil when missing)
  - `func AppendToManifest(path string, entries []ManifestEntry) error` — upsert by `PkgID` (idempotent)
  - `func ValidateEntry(e *ManifestEntry) error` — non-empty `PkgID`, `LicenseNote`, at least one of MapFiles/BSPFiles, no path escapes (`..`)

- [ ] **Step 1: Write the failing test** — round-trip: append 2 entries → load → upsert 1 → load again (count/key stability); validation rejects missing license, `../outside` paths, and empty entries.

- [ ] **Step 2: Run to see it fail** (compile error: undefined symbols).

- [ ] **Step 3: Implement** `corpus.go` per the interfaces. JSONL encode each entry on one line (`json.NewEncoder`); upsert replaces the record for the same PkgID preserving file order; rewrite atomically via temp file + rename. Manifest dir created with `os.MkdirAll(filepath.Dir(path), 0o755)`.

- [ ] **Step 4: Run tests to verify they pass** + `mise run verify`.

- [ ] **Step 5: Commit** — `feat(bspdec): add corpus manifest (JSONL) with idempotent upsert`.

---

## Task 3: Pinned compile helper

Bead: `ironwail-go-xxy.4`. In-repo qbsp is the pinned toolchain (plan-1 delta vs the ericw-subprocess spec).

**Files:**
- Create: `internal/bspdec/eval/compile.go`, `internal/bspdec/eval/compile_test.go`

**Interfaces:**
- Consumes: `pkg/map.Parse`, `internal/qbsp.Compile`, `qbsp.ReadBSPXBrushList`.
- Produces:
  - `type Pair struct { MapPath, BSPPath string }`
  - `func CompileMapPair(mapPath, outDir string) (Pair, error)` — reads the .map, compiles with `qbsp.Options{Log: nil}`, writes `<stem>.bsp` + record `<stem>.qbsp.log` (compile log lines); errors with the leak/parse message.
  - `func BrushCounts(bspPath string) ([]int, error)` — `ReadBSPXBrushList` on file bytes (oracle; empty when no lump).

- [ ] **Step 1: Failing test** — compile `testdata/bspdec/room.map` into a temp dir; assert pair exists, `BrushCounts` = `[6]`, log file written.

- [ ] **Step 2: Run to fail** (compile error).

- [ ] **Step 3: Implement** per interfaces. Note: `res.Leaked` still returns a result — write it and return an error with the leak trail length so the stage can distinguish leaks from hard errors.

- [ ] **Step 4: Pass + verify.**

- [ ] **Step 5: Commit** — `feat(bspdec): add pinned qbsp compile pair helper for corpus canonicalization`.

---

## Task 4: Label derivation (`internal/bspdec/labels.go`)

Bead: `ironwail-go-xxy.4`. Spec §9.4's deterministic supervision step: assign each decompiled cell to the original brush it came from (or `multi`/`none`), and mark seam edges on merged coplanar sides.

**Files:**
- Create: `internal/bspdec/labels.go`, `internal/bspdec/labels_test.go`

**Interfaces:**
- Consumes: bspdec internals (`decompiler`, `Brush`, `collectFaces`, `Winding` math), `pkg/map`.
- Produces:
  - `type CellLabel struct { Cell int; OriginalBrush int; Confidence string }` — `Confidence` ∈ `{"assigned","multi","none"}`
  - `type SeamLabel struct { SideBrush int; Edge [2]mapfile.Vec3; Seam bool }`
  - `func LabelCells(tree *bsp.Tree, orig *mapfile.Map, opts Options) ([]CellLabel, []SeamLabel, error)` — decompiles model 0 (world) via the existing pipeline, then:
    1. **Cell assignment**: for each cell, sample its centroid; `pointInConvex(centroid, originalBrush)` over worldspawn original brushes (planes from face points, ericw orientation — inside = behind every plane); exactly one hit → `assigned`; >1 → `multi`; 0 → `none`.
    2. **Seam labels**: for each side of each cell whose matched face region overlaps >1 original brush side, test each edge (collinear run between consecutive winding points) for an intersecting original-brush plane within the segment (segment↔plane intersection + on-segment test); `Seam=true` when found. (M2 Route A consumes these as the classification targets.)
  - `func pointInConvex(p mapfile.Vec3, mb *mapfile.MapBrush) bool`

- [ ] **Step 1: Failing tests** — (a) room pair: `LabelCells` on `compileFixture(t, roomMap())` returns candidates; every worldspawn cell is `assigned` or `multi`, and `none` count is 0; (b) the two-texture synthetic (`twoTextureTopTree` from split_test.go): the top side's internal edge (x=32) is labeled `Seam=true` when an original two-brush map has its boundary plane at x=32.

- [ ] **Step 2: Run to fail.**

- [ ] **Step 3: Implement** `labels.go`. Reuse `pick3Points`/`PlaneFromPoints`; the original-brush plane list is built once. For seam edges, classify each edge of a side winding by intersecting the edge segment with every original-brush face plane (skip planes the side is coplanar with). Use small tolerances (`planeMatchEpsilon`, `onEpsilon`).

- [ ] **Step 4: Pass + verify.**

- [ ] **Step 5: Commit** — `feat(bspdec): derive cell and seam supervision labels from paired maps`.

---

## Task 5: Corpus pipeline stages

Bead: `ironwail-go-xxy.4`. `tools/bspdec_corpus` runs idempotent stages over `dataset/bspdec/`.

**Files:**
- Create: `tools/bspdec_corpus/main.go`, `tools/bspdec_corpus/stages.go`, `tools/bspdec_corpus/stages_test.go`

**Interfaces:**
- Consumes: `eval` package (manifest, compile, stats), `internal/bspdec` (`LabelCells`, `Decompile`), `pkg/map`.
- Produces (stage funcs, all `func(ctx) error`-style with `*stageCtx{dataDir string; manifestPath string}`):
  - `stageEnumerate(ctx)` — clone `fzwoch/quake_map_source` (git, pinned rev `27abebaa`) into `raw/quake_map_source` when missing; walk for `*.map`; group by directory into entries (map_files + sibling `*.bsp`); read `LICENSE*`/README license note naive detection ("GPL" if found elsewhere per package); upsert manifest. `-quaddicted` flag degrades to a log notice when the API is unreachable (M0 keeps local sources canonical).
  - `stageValidate(ctx)` — for every listed file: `.map` → `mapfile.Parse`; `.bsp` → `bsp.Load`; failures recorded as `flags.Failed`? — spec wants failures recorded; add `Valid bool` to the entry via a side file `raw/validation.json` keyed by path (keep manifest lean: store `valid_file[]` list without mutating schema — simplest: drop invalid files from MapFiles/BSPFiles and log; record counts in provenance).
  - `stageCanonicalize(ctx)` — map-only entries → `CompileMapPair` into `paired/<pkg_id>/<stem>.bsp` (+ always-BRUSHLIST oracle); both-present entries copied to `paired/`; bsp-only entries → `classic-holdout/` listing.
  - `stageLabels(ctx)` — for each pair with a map: `LabelCells`; write `labeled/<pkg_id>/<stem>.labels.json` (cells + seams arrays) + summary counts in provenance.
  - `stageSplits(ctx)` — package-level train/val/test (80/10/10 by pkg_id hash) + classic holdout set → `splits.json`.
  - CLI: `tools/bspdec_corpus <stage> [-data dir] [-quaddicted]`; unknown stage prints usage.

- [ ] **Step 1: Failing test** — on a temp dataset dir seeded by copying `testdata/bspdec/room.map` into `raw/fixture/room.map`: run `stageEnumerate` (with `QuakeMapSourceDir` pointing at the seeded dir instead of cloning — factor the scan into `scanLocalSource(srcDir)` so tests skip git), then canonicalize/labels/splits; assert manifest has 1 entry, `paired/fixture/room.bsp` exists with BRUSHLIST `[6]`, labels file exists, splits.json valid.

- [ ] **Step 2: Run to fail** (undefined stages).

- [ ] **Step 3: Implement** `stages.go` (+ `main.go` wiring). Idempotency: every stage re-reads state and skips completed work (manifest upsert + `os.Stat` guards); appends to `provenance.log`.

- [ ] **Step 4: Pass + verify + `mise run lint`.**

- [ ] **Step 5: Commit** — `feat(bspdec): add idempotent corpus provisioning pipeline`.

---

## Task 6: Eval matrix + report

Bead: `ironwail-go-xxy.4`. Tier-1 + tier-2 over the corpus.

**Files:**
- Create: `internal/bspdec/eval/matrix.go`, `internal/bspdec/eval/matrix_test.go`

**Interfaces:**
- Consumes: `BSPStats`, `OccupancyMap`, `VoxelIoU`, `bspdec.Decompile`, `CompileMapPair`.
- Produces:
  - `type PairResult struct { PkgID, MapID string; OrigLump, RecompLump LumpStats; VoxelIoU float64; BrushDelta int; Warnings int }`
  - `func EvaluatePair(pairPath, outDir string) (PairResult, error)` — decompile the original bsp → write .map → recompile with the pinned qbsp → compare lump stats + VoxelIoU(orig, recompiled over orig bounds) + brush-count delta vs BRUSHLIST oracle.
  - `func EvaluateCorpus(corpusPath string, outDir string) ([]PairResult, error)` — iterate `paired/*/` directories.

- [ ] **Step 1: Failing test** — on the room pair: `EvaluatePair` returns IoU ≥ 0.9, BrushDelta ≥ 0, Warnings recorded; `EvaluateCorpus` aggregates ≥ 1 result.

- [ ] **Step 2: Run to fail.**

- [ ] **Step 3: Implement.**

- [ ] **Step 4: Pass + verify.**

- [ ] **Step 5: Commit** — `feat(bspdec): add tier-1/2 eval matrix over corpus pairs`.

---

## Task 7: Wiring, gates, bead close

Bead: `ironwail-go-xxy.4`.

**Files:** `mise.toml` (real tasks), `tools/bspdec_report/main.go` (markdown/JSON summary from eval output).

- [ ] **Step 1:** Real mise tasks:

```toml
[tasks.bspdec-data]
description = "Provision the bspdec corpus (enumerate..labels..splits)"
run = "go run ./tools/bspdec_corpus enumerate && go run ./tools/bspdec_corpus canonicalize && go run ./tools/bspdec_corpus labels && go run ./tools/bspdec_corpus splits"

[tasks.bspdec-eval]
description = "Run bspdec eval tiers 1+2 over the corpus"
run = "go run ./tools/bspdec_corpus eval"

[tasks.bspdec-report]
description = "Summarize bspdec eval results"
run = "go run ./tools/bspdec_report -data dataset/bspdec"
```

(replace the existing stubs for these three; keep `bspdec-train`/`bspdec-models` stubs.)

- [ ] **Step 2:** `tools/bspdec_report`: reads the eval JSON, prints a markdown table (map, IoU, lump deltas) + JSON summary; exit 1 when fewer than `-min-pairs` pairs (default 1) so CI-style runs are honest.

- [ ] **Step 3:** End-to-end: run `mise run bspdec-data` then `mise run bspdec-eval` then `mise run bspdec-report` with the corpus seeded from a scratch data dir (run the tool with `-data` pointing at a temp dataset populated from `quake_map_source` if network worked in Task 5, else the committed fixture). Record actual pair counts and the IoU/lump-delta numbers in the bead:

```bash
bd update ironwail-go-xxy.4 --design "... provisioned N pairs; eval: mean VoI <number>, median brush delta <number>; corpus sources: ..."
```

- [ ] **Step 4:** `mise run verify && mise run lint`; close the bead with the recorded numbers:

```bash
bd close ironwail-go-xxy.4 --reason="eval suite (lump stats + voxel IoU) and idempotent corpus pipeline landed; N pairs provisioned; M0 parity numbers recorded in the bead"
```

- [ ] **Step 5:** Commit everything remaining:

```bash
git add mise.toml tools/bspdec_report internal/bspdec/labels.go internal/bspdec/labels_test.go
git commit -m "feat(bspdec): wire bspdec-data/eval/report tasks and close M0 corpus milestone"
```

---

## Self-review (plan author)

- **Spec coverage** — §8 tier 1 (recompile lump diff + voxel IoU) → Tasks 1+6; tier 2 (structural: brush-count delta) → Task 6 `BrushDelta`; tier 3 (behavioral parity) stays with the existing parity harness (plan-1 scope, out). §9 P1 (enumerate; Quaddicted optional) → Task 5; P2 (validate) → Task 5; P3 (canonicalize with in-repo qbsp, BRUSHLIST always on) → Tasks 3+5; P4 (labels) → Task 4; P5 (splits + classic holdout) → Task 5; P6 (synth) → plan 3 (xxy.11); P7 (toolchain aug) → deferred, noted in `ToolchainGuess`. §10 training → plan 4 (M2). §12 corpus-driven golden tests → Task 6.
- **Placeholder scan** — no TBDs; the only intentional stub is M0's `-quaddicted` network fallback (degradation, logged).
- **Type consistency** — `eval` names (`BSPStats`, `OccupancyMap`, `VoxelIoU`, `CompileMapPair`, `ManifestEntry`) match across Tasks 1–3 and 5–7; `bspdec.LabelCells` (Task 4) consumed by Task 5; `PairResult` (Task 6) consumed by `bspdec_report` (Task 7).
---

## Execution notes (2026-09-08 — deviations found while executing)

1. **qbsp submodel OOB bug fixed (root cause)** — `splitByTree`/`siblingAtPlane`
   index model-local slices but childRefs carry global node/leaf offsets for
   submodels; every multi-model map with interior submodel splits crashed
   (visible the moment real maps were compiled). Fix in `internal/qbsp`:
   `buildModelSurfaces` works on a locally-rebased copy; regression test
   `TestSubmodelInteriorSplits` added. This is a toolchain bug the corpus
   existed to find.
2. **qbsp leak false-positive on sealed id1 maps (open)** — 36/38 id1 maps
   report "leaks to the void" though ericw compiles them. Follow-up bead
   filed; blocks the wider corpus.
3. **Decompile→recompile leaks on real maps (open)** — dm1/e1m8 decompile
   to SelfCheck-clean maps but recompile leaky: 8-unit grid snap collapses
   sub-grid geometry (dm1's 4-unit trim plates) into cracks. The sliver
   filter became snap-aware (`sideSurvivesGrid`) so output is at least
   self-consistent; the rebuild-ability gap is a follow-up bead.
4. **Subprocess-per-map isolation** — canonicalize compiles each map in a
   child process so qbsp panics cannot abort the pipeline. The first
   implementation re-executed the *test binary* (when `go test` was the
   executable), which recursed into itself and OOM-killed the machine.
   Fixed: `stageCtx.compile` injection, `subprocessCompile` never runs
   under a `.test` binary, tests compile in-process. `GOMEMLIMIT=4GiB` was
   also added to the mise env as a GC soft limit.
5. **Eval tier-1 result shape** — recompile of decompiled output is
   currently the binding constraint on real maps (leak), so eval rows carry
   the failure in `error` rather than scores; the ladder to full acceptance
   is the two follow-up beads.
6. Commits withheld on earlier steps per policy; final commit covers plan 2
   in one go (submitted after explicit approval).
