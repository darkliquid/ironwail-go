# BSPDEC M1: BRUSHLIST Direct Path + Synthetic Generator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the M1 milestone: the BRUSHLIST direct-emission path (near-perfect decompile route), the deterministic synthetic map generator that feeds unlimited labeled training pairs, and the quantified headroom study that produces the ML go/no-go gate for M2.

**Architecture:** Parse the BSPX `BRUSHLIST` lump our in-repo qbsp always appends (`internal/qbsp/bspx.go` stores per-model original brushes: AABB + contents + full face planes), add a direct emission path to `internal/bspdec` gated by the already-reserved `Options.NoBrushlist`, and expose it via the existing `--no-brushlist` CLI flag. `tools/bspdec_synth` generates sealed room-grammar maps on an 8-unit lattice with mapper conventions (blockout 64/32, detail 16/8, trim 8/4; stairs/doors multiples of 8/16/64) via the `pkg/map` writer, compiles each with the pinned in-repo qbsp (BRUSHLIST oracle), and records pairs + labels under `dataset/bspdec/synth/`. The headroom study (`tools/bspdec_report` extended) compares the treewalk baseline against the BRUSHLIST-direct ceiling on recompile-diff and voxel-IoU, and records the go/no-go decision in bead `ironwail-go-xxy.5`.

**Tech Stack:** Go 1.26, `CGO_ENABLED=0`, stdlib only (`flag`, `log/slog`, `encoding/binary`, `math/rand` for seeded generation), existing packages `internal/bsp`, `internal/qbsp`, `internal/bspdec`, `internal/bspdec/eval`, `pkg/map` (package `mapfile`).

**Spec:** [docs/superpowers/specs/2026-09-07-bspdec-design.md](../specs/2026-09-07-bspdec-design.md) — read §4 (pipeline), §5 step 9 (BRUSHLIST shortcut), §8 (eval tiers), §9.3 (P3 -wrbrushes), §9.4 (P4 labels), §9.6 (P6 synthetic), §10, §12, §13 (M1 gate). Research narrative: [docs/BSPDEC_SPEC.md](../BSPDEC_SPEC.md) §9.3–9.6, §3.3.

**Beads:** Task 1 → `ironwail-go-xxy.5` (BRUSHLIST path half); Tasks 2–4 → `ironwail-go-xxy.11`; Task 5 → `ironwail-go-xxy.5` (headroom study, completes the bead). Claim each bead (`bd update <id> --claim`) before its first task; close with `bd close <id> --reason="..."` when its acceptance is met. Beads xxy.6/.7/.12 (M2) are gated behind this plan per spec §13.

## Global Constraints

Every task implicitly includes these (from spec §14 and AGENTS.md):

- Pure Go, `CGO_ENABLED=0`, **no build tags** in any `.go` file, stdlib + existing repo packages only.
- `log/slog` for all diagnostics; never `fmt.Println`/`log.Printf` for logging. Corpus/report CLI tools may use plain stdout for their tabular output (existing report tool prints tables with `fmt.Println` — follow that local pattern).
- Every ported algorithm cites C lineage inline where applicable (BRUSHLIST format per `ericw-tools qbsp/brush.cc` layout comment in `internal/qbsp/bspx.go`).
- Single-package test runs must match the mise env: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestName -count=1`.
- `mise run verify` green at every task boundary (`go generate ./...` + full tests + build).
- `internal/qbsp` tests are the oracle for anything touching the compiler: they must stay green (the M0 leak-fix batch is already in; do not regress it).
- The package in `pkg/map` is named `mapfile`. `internal/bspdec/eval` holds corpus/eval helpers; do not move them.
- Dataset paths are relative to `dataset/bspdec/`: `paired/`, `labeled/`, `synth/`, `raw/manifest.jsonl`. `.gitignore` is a whitelist — dataset content is already ignored except manifest/`*.json` handled by corpus tooling; adding new files to `dataset/` is fine and local-only.
- Seeded generation: use `math/rand.New(rand.NewSource(seed))` — deterministic across runs and platforms; never `time.Now()` seeds.
- Commits: one per task where marked. Per repo policy, commit only with operator approval; if executing without commit approval, stage changes and report instead. Never push.

---

## File Structure

New files:

| File | Responsibility |
| --- | --- |
| `internal/bspdec/brushlist.go` | `ParseBrushList` (BSPX BRUSHLIST v1 reader), `BSPXBrush`/`BSPXModel` types, `brushListFromAppendedBSP` (locate the appended lump), direct-to-`mapfile.MapBrush` conversion |
| `internal/bspdec/brushlist_test.go` | round-trip vs `internal/qbsp.AppendBSPX`, golden emission reparse, edge cases (empty list, missing lump) |
| `tools/bspdec_synth/main.go` | CLI (`-data`, `-count`, `-seed`, `-out`) |
| `tools/bspdec_synth/generator.go` | `Generator` (seeded `*rand.Rand`), room grammar: `placeRooms`, `connectRooms`, `placeStairs`, `placeTrims`, `placeDetails`, `placeLiquids`, worldspawn-brush assembly |
| `tools/bspdec_synth/generator_test.go` | determinism, lattice membership, sealed-compile, label coverage |
| `internal/bspdec/eval/heads.go` | headroom metrics: baseline-vs-brushlist `HeadroomRow`, `ComposeHeadroomReport` |
| `internal/bspdec/eval/heads_test.go` | metric math unit tests |

Modified files:

| File | Change |
| --- | --- |
| `internal/bspdec/types.go` | (none required by this plan; `NoBrushlist` already reserved) |
| `internal/bspdec/decompile.go` | pipeline branch: when `!opts.NoBrushlist` and a BRUSHLIST lump is present, run direct emission instead of treewalk (per model); keep the treewalk as the fallback |
| `cmd/bspdec/main.go` | wire `--brushlist` output path only if needed (flag already exists); pass through unchanged otherwise |
| `tools/bspdec_corpus/stages.go` | new `synth` stage entry (`stageSynth`) + manifest registration of synthetic pairs |
| `tools/bspdec_report/main.go` | `headroom` subcommand printing the baseline-vs-ceiling table and go/no-go verdict |
| `docs/BSPDEC_IMPL_PLAN.md` | superseded note pointing at this plan (kept for history; add a one-line pointer only) |

---

### Task 1: BRUSHLIST v1 parser + direct emission path

The in-repo qbsp always appends a BSPX `BRUSHLIST` lump (`internal/qbsp/bspx.go`), so every compiled corpus BSP carries the original brushes. This task reads it and emits them directly — the near-perfect decompile route (spec §5 step 9). Format (already documented in `serializeBSPXBrushes`):

```
per model record: int32 ver(=1); int32 modelnum; int32 numbrushes; int32 numfaces
per brush:        aabb3f mins (3 f32), aabb3f maxs (3 f32), int16 contents, uint16 numfaces,
                  per face: qplane3f (normal 3 f32 + dist f32)
```

The lump sits inside an appended "BSPX" header: 8-byte `"BSPX"+uint32 numlumps`, then per lump a 32-byte entry (`[24]byte` name, `uint32 ofs`, `uint32 len`) after the official 15-lump BSP region (see `AppendBSPX`). `ReadBSPXBrushList` in `internal/qbsp/bspx.go` already finds and validates the lump — reuse its locating logic; this task converts the payload into decompiler brushes.

**Files:**
- Create: `internal/bspdec/brushlist.go`
- Create: `internal/bspdec/brushlist_test.go`
- Modify: `internal/bspdec/decompile.go` (pipeline branch ~line 57, `Decompile`)

**Interfaces:**
- Consumes: `qbsp.ReadBSPXBrushList(bspData []byte) ([]int, error)` and the payload layout constants in `internal/qbsp/bspx.go`; `mapfile.MapBrush`, `mapfile.MapFace`, `mapfile.Plane` from `pkg/map`; `bspdec.Options.NoBrushlist`.
- Produces:
  - `type BSPXBrush struct { Model int; Mins, Maxs mapfile.Vec3; Contents int32; Faces []mapfile.Plane }`
  - `func ParseBrushList(recordBuf []byte) ([]BSPXBrush, error)` — parses the concatenated per-model records (the `payload` written by `serializeBSPXBrushes`).
  - `func BrushListFromBSP(bspData []byte) ([]BSPXBrush, error)` — locates the appended BRUSHLIST lump (reuse the scan from `ReadBSPXBrushList`), then `ParseBrushList`; returns `(nil, nil)` when no lump exists.
  - `func BrushFromBSPX(b BSPXBrush) (mapfile.MapBrush, error)` — builds a `mapfile.MapBrush` with one `MapFace` per stored plane: face plane = `mapfile.PlaneFromPoints`-compatible `{Normal, Dist}` (see `mapfile.Plane` in `pkg/map/mapfile.go`), texture name `"mt_wall"` (M1 direct path has no texture info; the texture stage is a later concern), and `VectorS`/`VectorT` zeroed (Valve-220 axes are filled by a later pass — see Task 3 note).

- [ ] **Step 1: Write the failing parser test**

`internal/bspdec/brushlist_test.go`:

```go
func TestParseBrushListRoundTrip(t *testing.T) {
	// Build a tiny map with known world brushes, compile with the pinned
	// qbsp (always appends BRUSHLIST), then parse it back and compare.
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoomSrc(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	m, err := mapfile.Parse(strings.NewReader(src))
	if err != nil { t.Fatal(err) }
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil { t.Fatal(err) }
	brushes, err := BrushListFromBSP(res.Data)
	if err != nil { t.Fatal(err) }
	if len(brushes) == 0 { t.Fatal("no BRUSHLIST brushes parsed") }
	// The world brush count must match the source worldspawn brush count.
	want := len(m.Entities[0].Brushes)
	var world []BSPXBrush
	for _, b := range brushes {
		if b.Model == 0 { world = append(world, b) }
	}
	if len(world) != want {
		t.Fatalf("world brushes = %d, want %d", len(world), want)
	}
	// Contents must be preserved and bounds must match the originals.
	if world[0].Contents != int32(bsp.ContentsSolid) {
		t.Errorf("contents = %d, want solid", world[0].Contents)
	}
}
```

`prettyRoomSrc` is the test-local helper already used by `internal/qbsp/solidbsp_test.go` (`prettyRoom(...)`); replicate it locally as `prettyRoomSrc` (package-local helpers are the convention).

- [ ] **Step 2: Run it to make sure it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestParseBrushListRoundTrip -count=1`
Expected: FAIL, `undefined: BrushListFromBSP`.

- [ ] **Step 3: Implement `ParseBrushList` + `BrushListFromBSP`**

```go
var brushListErr = fmt.Errorf("bspdec: BRUSHLIST parse")

// BSPXBrush is one original brush recovered from the BRUSHLIST lump.
type BSPXBrush struct {
	Model    int
	Mins     mapfile.Vec3
	Maxs     mapfile.Vec3
	Contents int32
	Faces    []mapfile.Plane
}

// ParseBrushList parses the concatenated per-model BRUSHLIST records
// written by internal/qbsp.serializeBSPXBrushes (ericw
// bspxbrushes_permodel layout).
func ParseBrushList(buf []byte) ([]BSPXBrush, error) {
	var out []BSPXBrush
	off := 0
	for off+16 <= len(buf) {
		ver := int32(binary.LittleEndian.Uint32(buf[off:]))
		model := int(binary.LittleEndian.Uint32(buf[off+4:]))
		nbrushes := int(binary.LittleEndian.Uint32(buf[off+8:]))
		off += 16
		if ver != 1 {
			return nil, fmt.Errorf("%w: unsupported version %d", brushListErr, ver)
		}
		for i := 0; i < nbrushes; i++ {
			if off+28 > len(buf) {
				return nil, fmt.Errorf("%w: truncated brush", brushListErr)
			}
			b := BSPXBrush{Model: model}
			b.Mins.X = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off:])))
			b.Mins.Y = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4:])))
			b.Mins.Z = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+8:])))
			b.Maxs.X = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+12:])))
			b.Maxs.Y = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+16:])))
			b.Maxs.Z = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+20:])))
			b.Contents = int32(int16(binary.LittleEndian.Uint16(buf[off+24:])))
			nfaces := int(binary.LittleEndian.Uint16(buf[off+26:]))
			off += 28
			for j := 0; j < nfaces; j++ {
				if off+16 > len(buf) {
					return nil, fmt.Errorf("%w: truncated face", brushListErr)
				}
				pl := mapfile.Plane{
					Normal: mapfile.Vec3{
						X: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off:]))),
						Y: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4:]))),
						Z: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+8:]))),
					},
					Dist: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+12:]))),
				}
				b.Faces = append(b.Faces, pl)
				off += 16
			}
			out = append(out, b)
		}
	}
	return out, nil
}
```

`BrushListFromBSP` reuses the appended-lump scan from `internal/qbsp/bspx.go:ReadBSPXBrushList` — read that function and replicate its header walking (verify "BSPX" magic, iterate `numlumps` 32-byte entries, match name `"BRUSHLIST"`, bounds-check `ofs/len`, slice the payload) rather than importing it (the qbsp helper returns counts only).

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestParseBrushListRoundTrip -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing direct-emission test**

```go
func TestBrushFromBSPXRewritesPlanes(t *testing.T) {
	// A single original brush with 6 axial faces; BrushFromBSPX must
	// reproduce 6 MapFaces whose Planes reconstruct the same bounds.
	src := "{\n\"classname\" \"worldspawn\"\n" +
		"{\n" + slabFacesSrc(0, 0, 0, 16, 32, 8, "mt_wall") + "}\n" +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"8 16 4\"\n}\n"
	m, err := mapfile.Parse(strings.NewReader(src))
	if err != nil { t.Fatal(err) }
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil { t.Fatal(err) }
	brushes, err := BrushListFromBSP(res.Data)
	if err != nil { t.Fatal(err) }
	if len(brushes) == 0 { t.Fatal("no brushes") }
	mb, err := BrushFromBSPX(brushes[0])
	if err != nil { t.Fatal(err) }
	if len(mb.Faces) != 6 { t.Fatalf("faces = %d, want 6", len(mb.Faces)) }
	// Reparse the emitted brush through the map parser's PlaneFromPoints
	// path: the brush must be convex-closed (the map parser rejects open
	// brushes).
	round := writeAndReparse(t, mb)
	if len(round.Faces) != 6 { t.Fatalf("reparsed faces = %d, want 6", len(round.Faces)) }
}
```

`slabFacesSrc` is a 6-face brush block builder and `writeAndReparse` writes a map holding one brush and re-parses it with `mapfile.Parse` — implement them as package-local helpers in `brushlist_test.go`.

- [ ] **Step 6: Run the direct-emission test to verify it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestBrushFromBSPXRewritesPlanes -count=1`
Expected: FAIL, `undefined: BrushFromBSPX`.

- [ ] **Step 7: Implement `BrushFromBSPX`**

For each stored plane build a `mapfile.MapFace` with `Plane: pl` (stored planes are already outward-facing with positive-axial normalization, matching `mapfile.Plane`), `TexName: "mt_wall"`, zeroed `VectorS/VectorT`, and `Line: 0`. All six planes are stored explicitly, so no axial inference is needed (the AABB in the synth/labels path is used for validation, not emission).

- [ ] **Step 8: Run test to verify it passes**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestBrushFromBSPXRewritesPlanes -count=1`
Expected: PASS.

- [ ] **Step 9: Write the failing pipeline-branch test**

```go
// TestDecompileBrushListPath: a compiled BSP (BRUSHLIST present) must
// decompile through the direct path, producing one brush per original
// world brush instead of the treewalk's leaf-derived cells.
func TestDecompileBrushListPath(t *testing.T) {
	m, _ := mapfile.Parse(strings.NewReader(boxMapSrc()))
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil { t.Fatal(err) }
	with, errs := Decompile(res.Data, Options{GridSnap: 8})
	if errs != nil { t.Fatal(errs) }
	without, errs := Decompile(res.Data, Options{NoBrushlist: true, GridSnap: 8})
	if errs != nil { t.Fatal(errs) }
	want := len(m.Entities[0].Brushes)
	if len(with[0].Brushes) != want {
		t.Errorf("brushlist path brushes = %d, want %d (original world brushes)", len(with[0].Brushes), want)
	}
	_ = without // treewalk fallback must still produce output and is compared by the headroom study (Task 5)
}
```

`boxMapSrc` returns the sealed-box map fixture (reuse the `boxMap()`-style helper text from `internal/qbsp/qbsp_test.go`), and `ModelStats` must gain a `Brushes int` field if it does not already expose the emitted brush count for model 0 (check `internal/bspdec/types.go`; add it if missing, updating `cmd/bspdec --json` output consumers).

- [ ] **Step 10: Run it to verify it fails**

Expected: FAIL, brushlist path returns treewalk counts, not `want`.

- [ ] **Step 11: Branch the pipeline in `Decompile`**

At the top of the per-model loop in `Decompile` (after `opts.TextureFallback` defaulting, before the treewalk), when `!opts.NoBrushlist`:

```go
if ls, err := BrushListFromBSP(data); err == nil && len(ls) > 0 {
	return decompileFromBrushList(data, ls, opts)
}
```

`decompileFromBrushList` groups `BSPXBrush` by `Model`, converts each with `BrushFromBSPX`, runs the existing merge/cleanup stages (call the merge and grid-snap helpers already used by the treewalk path — reuse `merge.go`'s exported entry point; if none is exported, export one: `MergeBrushesConvex(brushes []*Brush, opts Options) []*Brush`), and produces the `*mapfile.Map` with a worldspawn entity carrying the converted brushes plus the entity-lump passthrough already implemented in `internal/bspdec/entities.go` (call its existing helper; check its signature in `entities.go` and reuse it). Model stats: one `ModelStats{Model: i, Brushes: n}` per model.

- [ ] **Step 12: Run the pipeline test to verify it passes, then the full package**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1`
Expected: PASS (all previous M0 tests stay green; the brushlist path only activates when the lump exists and `NoBrushlist == false` — which the M0 golden tests already exercise, so keep their expectations: if any M0 test asserts the exact brush count of the treewalk path on a compiled BSP, it must now opt into `NoBrushlist: true`; update such tests deliberately and note it in the commit message).

- [ ] **Step 13: Commit**

```bash
git add internal/bspdec/brushlist.go internal/bspdec/brushlist_test.go internal/bspdec/decompile.go internal/bspdec/types.go
git commit -m "feat(bspdec): add BRUSHLIST direct decompile path"
```

---

### Task 2: Synthetic map generator — lattice room grammar

Implements spec §9.6 P6 and bead `ironwail-go-xxy.11`: deterministic room-grammar brush layouts on 8-unit lattices with mapper conventions. Maps are worldspawn-only solids (no func_detail/bmodels) so they compile clean with the pinned qbsp every time — perfect for scalable labeled training volume.

**Files:**
- Create: `tools/bspdec_synth/generator.go`
- Create: `tools/bspdec_synth/generator_test.go`
- Create: `tools/bspdec_synth/main.go`

**Interfaces:**
- Consumes: `mapfile.Map`, `mapfile.Write`, `mapfile.WriteOptions{GridSnap}` from `pkg/map`; `eval.CompileMapPair(mapPath, outDir string) (eval.Pair, error)` from `internal/bspdec/eval`; `eval.AppendToManifest(path string, entries []eval.ManifestEntry)` (check exact signature in `internal/bspdec/eval/corpus.go`).
- Produces:
  - `type Generator struct { r *rand.Rand; seed int64; count int; grid int }`
  - `func NewGenerator(seed int64, count int) *Generator`
  - `func (g *Generator) GenMap(n int) *mapfile.Map` — `n`-th map in the seed's deterministic sequence (map id `synth-<seed>-<n>`).
  - `func (g *Generator) Emit(m *mapfile.Map, w io.Writer) error` — `mapfile.Write` with `GridSnap: 8` (or `g.grid`).
  - Lattice helpers (package-level, unit-testable): `func snapGrid(v float64, g int) float64`, `func (g *Generator) room(x0, y0, z0, x1, y1, z1 float64, tex string) mapfile.MapBrush`, `func (g *Generator) connectRooms(a, b mapfile.MapBrush, level float64) []mapfile.MapBrush`.

- [ ] **Step 1: Write the failing determinism + lattice test**

```go
func TestGeneratorDeterministic(t *testing.T) {
	a := NewGenerator(42, 3)
	b := NewGenerator(42, 3)
	m1 := a.GenMap(1)
	m2 := b.GenMap(1)
	if len(m1.Entities) == 0 { t.Fatal("no entities") }
	if !brushSetsEqual(m1.Entities[0].Brushes, m2.Entities[0].Brushes) {
		t.Fatal("same seed produced different maps")
	}
	// All coordinates must lie on the 8-unit lattice.
	for _, br := range m1.Entities[0].Brushes {
		for _, f := range br.Faces {
			if !onLattice(f.Plane.Normal, 0.6) || !onLattice(f.Plane.Dist, 0.6) {
				t.Fatalf("face (%v %v) off the 8-unit lattice", f.Plane.Normal, f.Plane.Dist)
			}
		}
	}
	// Different seeds differ.
	m3 := NewGenerator(7, 3).GenMap(1)
	if brushSetsEqual(m1.Entities[0].Brushes, m3.Entities[0].Brushes) {
		t.Fatal("different seeds produced identical maps")
	}
}
```

`brushSetsEqual` compares face-plane sequences via `mapfile.Vec3` equality; `onLattice(v float64, tol float64)` rounds to the nearest 8-multiple and compares within `tol`. The plane constant of an axis-aligned face is exactly its offset, so `snapGrid(planeDist, 8) == planeDist` is the check for the axial planes the grammar generates.

- [ ] **Step 2: Run it to verify it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./tools/bspdec_synth -run TestGeneratorDeterministic -count=1`
Expected: FAIL, package not found.

- [ ] **Step 3: Implement the room grammar primitives**

```go
const (
	grid        = 8   // master lattice
	blockoutMin = 64  // walls/floors
	blockoutMax = 32  // thick blockout slabs
	detailK     = 16  // detail walls
	detailS     = 8   // thin detail
	trimK       = 8
	trimS       = 4
)

// snapGrid rounds to the nearest lattice multiple.
func snapGrid(v float64, g int) float64 {
	s := float64(g)
	return math.Round(v/s) * s
}

// randBetween returns a lattice multiple in [lo, hi] (both multiples of grid).
func (g *Generator) randBetween(lo, hi float64) float64 {
	steps := int((hi - lo) / grid)
	if steps < 1 { return lo }
	return lo + grid*float64(g.r.Intn(steps+1))
}
```

`NewGenerator` seeds `rand.New(rand.NewSource(seed))`. `room(...)` builds a 6-face `mapfile.MapBrush` with outward axial planes via `mapfile.PlaneFromPoints`-style construction: locals `[3][3]float64` corners per face following the existing `internal/qbsp` slab format (the `prettySlab` helper in `internal/qbsp/solidbsp_test.go` shows the exact 6-face layout; mirror it for `mapfile`).

- [ ] **Step 4: Run test to verify it passes**

Expected: PASS.

- [ ] **Step 5: Write the failing grammar test (sealed room layout)**

```go
func TestGenMapRoomsSealed(t *testing.T) {
	g := NewGenerator(99, 1)
	m := g.GenMap(0)
	// Every brush must be a closed convex 6-face box on the lattice.
	for _, br := range m.Entities[0].Brushes {
		if len(br.Faces) != 6 { t.Fatalf("brush faces = %d, want 6", len(br.Faces)) }
	}
	// The layout must contain at least one room envelope: the union AABB
	// has all axis-aligned slab faces (checked by compile in Step 7).
}
```

Plumb `GenMap` to place a random room (blockout 64 floor/ceiling, 32-thick walls) via `randBetween` on the lattice, then a corridor connecting two rooms (see `connectRooms` in Step 7's test) — start with two rooms + one corridor; expand grammar in later steps.

- [ ] **Step 6: Run to verify it fails**, then implement `GenMap` / `Emit`

`GenMap` builds the worldspawn entity: `{"classname","worldspawn"}, {"wad",""}, {"_name","synth-<seed>-<n>"}` plus brushes; adds `info_player_start` at the first room's interior center. `Emit` writes the map via `mapfile.Write` + `WriteOptions{GridSnap: 8}`.

- [ ] **Step 7: Write the failing end-to-end compile test**

```go
func TestSynthPairCompilesClean(t *testing.T) {
	dir := t.TempDir()
	g := NewGenerator(1234, 5)
	for n := 0; n < 5; n++ {
		m := g.GenMap(n)
		var buf bytes.Buffer
		if err := g.Emit(m, &buf); err != nil { t.Fatal(err) }
		src := filepath.Join(dir, fmt.Sprintf("s%d.map", n))
		if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil { t.Fatal(err) }
		p, err := eval.CompileMapPair(src, dir)
		if err != nil {
			t.Fatalf("map %d fails to compile: %v", n, err)
		}
		data, err := os.ReadFile(p.BSPPath)
		if err != nil { t.Fatal(err) }
		brushes, err := bspdec.BrushListFromBSP(data)
		if err != nil { t.Fatal(err) }
		if len(brushes) < 6 { t.Fatalf("map %d: BRUSHLIST brushes = %d, want >= 6", n, len(brushes)) }
	}
}
```

- [ ] **Step 8: Run to verify it fails**
Expected: FAIL (compile error: map leaks or is degenerate).

- [ ] **Step 9: Expand the grammar until sealed maps compile**

Add, in this order, each with its own small unit test asserting only lattice-valid brush outputs: `connectRooms` (corridor + door, widths multiples of 16/64), `placeStairs` (stair flight where floor height differs by ≥ 16; tread depth multiple of 8), `placeTrims` (baseboard/floor-edge trims: thickness 8, height 4), `placeDetails` (pillars of 16, alcoves of 8), `placeLiquids` (`*water1` flat pool at a room floor). All brushes are worldspawn solids except water (content from texture). Keep every brush closed and axial — do not introduce bevels or open brushes (the corpus follow-up bead `ironwail-go-aeh` owns that fidelity class; synth deliberately avoids it).

- [ ] **Step 10: Run the full synth test suite; then `mise run verify`**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./tools/bspdec_synth ./internal/bspdec/... -count=1`
Expected: PASS everywhere.

- [ ] **Step 11: Implement the CLI**

`tools/bspdec_synth/main.go`: flags `-data` (default `dataset/bspdec`), `-count` (default 1000; follows bead acceptance `>=1000`), `-seed` (default 0x5EED). Flow: for n in [0, count): `GenMap(n)`, write `dataset/bspdec/synth/<seed>/<mapid>.map`, compile via `eval.CompileMapPair`, append a `ManifestEntry{PkgID: fmt.Sprintf("synth-%d", seed), LicenseNote: "synthetic", MapFiles: [...], BSPFiles: [...]}` using `eval.AppendToManifest` (idempotent per-PkgID upsert, `corpus.go:62`). Log progress with `slog`; exit non-zero on any compile failure.

- [ ] **Step 12: Generate 1000 pairs locally and verify**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go run ./tools/bspdec_synth -data dataset/bspdec -count 1000`
Expected: completes; `dataset/bspdec/synth/` holds 1000 `.map`/`.bsp` pairs; no compile failures in the output; manifest gains 1000 entries with `license_note: synthetic`.

- [ ] **Step 13: Commit**

```bash
git add tools/bspdec_synth/
git commit -m "feat(bspdec): add seeded synthetic map generator"
```

---

### Task 3: Label derivation coverage for synthetic pairs

Bead `ironwail-go-xxy.11` acceptance requires labels derived for 100% of cells/edges on synthetic maps. Since BRUSHLIST is the oracle and each synth brush remains intact after compile (sealed maps, no chops except the compiler's own), every decompiled cell maps to exactly one original brush: derive labels by point-in-brush over the BRUSHLIST geometry, deterministic and total.

**Files:**
- Modify: `tools/bspdec_corpus/stages.go` (new `stageSynth` wiring the generator → compile → labels path; register `"synth"` in `stageFuncs` in `main.go`)
- Modify: `tools/bspdec_corpus/main.go` (stage registration)
- Test inside `tools/bspdec_corpus/stages_test.go` (in-process, per the existing `compile`-injection convention)

**Interfaces:**
- Consumes: `bspdec.LabelCells(tree *bsp.Tree, orig *mapfile.Map, opts bspdec.Options) ([]bspdec.CellLabel, []bspdec.SeamLabel, error)` (already implemented in `internal/bspdec/labels.go`), `eval.CompileMapPair`, `tools/bspdec_synth.Generator`.
- Produces: `files dataset/bspdec/labeled/synth-<seed>/<mapid>.labels.json` in the existing `deriveLabels` JSON shape (see `labelRecord` in `stages.go`).

- [ ] **Step 1: Write the failing coverage test**

```go
// TestSynthLabelsFullCoverage: every world-model cell of a generated map
// receives exactly one original-brush label (BRUSHLIST ground truth).
func TestSynthLabelsFullCoverage(t *testing.T) {
	dir := t.TempDir()
	g := synth.NewGenerator(7, 1)
	m := g.GenMap(0)
	var buf bytes.Buffer
	if err := g.Emit(m, &buf); err != nil { t.Fatal(err) }
	src := filepath.Join(dir, "m.map")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil { t.Fatal(err) }
	p, err := eval.CompileMapPair(src, dir)
	if err != nil { t.Fatal(err) }
	data, err := os.ReadFile(p.BSPPath)
	if err != nil { t.Fatal(err) }
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil { t.Fatal(err) }
	orig, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil { t.Fatal(err) }
	cells, seams, err := bspdec.LabelCells(tree, orig, bspdec.Options{GridSnap: 8})
	if err != nil { t.Fatal(err) }
	if len(cells) == 0 { t.Fatal("no labeled cells") }
	for i, c := range cells {
		if c.OriginalBrush < 0 || c.OriginalBrush >= len(orig.Entities[0].Brushes) {
			t.Fatalf("cell %d labeled with out-of-range brush %d", i, c.OriginalBrush)
		}
	}
	for i, s := range seams {
		if !s.Seam { t.Fatalf("edge %d unlabeled (expected exact seam truth on synthetic)", i) }
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./tools/bspdec_corpus -run TestSynthLabelsFullCoverage -count=1`
Expected: FAIL (labels missing or partial — expose the current `LabelCells` total/partial behavior; note that the M0 label-derivation stage covers non-synthetic pairs where cracks leave `none` labels — synthetic maps must reach 100% because geometry is intact).

- [ ] **Step 3: Close the coverage gap in `LabelCells`**

In `internal/bspdec/labels.go`, the cell membership test (`pointInPlanes`) must sample an interior point per cell and require exactly one matching original brush; when none matches (should not happen on intact synthetic geometry), fall back to nearest-brush within 8 units and mark `Multi: false, Seam: true` deterministically. Do not weaken the non-synthetic path: keep `multi`/`none` outcomes for cracked maps. Add a `LabelCoverage(cells []CellLabel) (labeled, total int)` helper returning the percentage for the coverage assertion.

- [ ] **Step 4: Run to verify it passes (100%)**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./tools/bspdec_corpus -run TestSynthLabelsFullCoverage ./internal/bspdec -run TestLabels -count=1`
Expected: PASS; coverage == 100%.

- [ ] **Step 5: Add the `synth` corpus stage**

`stageSynth(ctx *stageCtx)` runs the generator for `-count` maps into `dataset/bspdec/synth/`, compiles each with the injected `ctx.compile` (in tests the in-process `CompileMapPair`, in production the subprocess isolation — reuse the existing `subprocessCompile` pattern from `stages.go`), writes labels via `deriveLabels` (the existing function in `stages.go`), and appends manifest entries. Wire `"synth"` into `stageFuncs` in `tools/bspdec_corpus/main.go`.

- [ ] **Step 6: Run the corpus tests + a 50-map smoke**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go run ./tools/bspdec_corpus synth -data dataset/bspdec -count 50`
Expected: 50 pairs in `synth/`, 50 `labels.json` with 100% coverage, manifest updated, exit 0.

- [ ] **Step 7: Commit**

```bash
git add internal/bspdec/labels.go internal/bspdec/labels_test.go tools/bspdec_corpus/
git commit -m "feat(bspdec): full-coverage labels for synthetic pairs"
```

---

### Task 4: Generative corpus wiring + docs

Make the synthetic corpus a first-class dataset slice end-to-end: splits and report plumbing.

**Files:**
- Modify: `tools/bspdec_corpus/stages.go` (`stageSplits` must include `synth-*` packages in the split — package-level split already defined; just include the new packages in the split pool; read the existing `stageSplits` implementation and add the synth dir scan)
- Modify: `tools/bspdec_report/main.go` (accept `synth-*` pairs in the report; they flow through the same `eval.EvaluatePair` path)
- Modify: `docs/BSPDEC_IMPL_PLAN.md` (one-line superseded pointer)

- [ ] **Step 1: Write the failing split-inclusion test**

```go
// TestSplitsIncludeSynth: the split file must contain synth packages when
// the synth corpus exists.
func TestSplitsIncludeSynth(t *testing.T) {
	dataDir := t.TempDir()
	// fabricate one synth pair + manifest entry
	writeSynthFixture(t, dataDir) // helper: one synth-0 package with one pair
	if err := runSplits(dataDir); err != nil { t.Fatal(err) }
	b, err := os.ReadFile(filepath.Join(dataDir, "splits.json"))
	if err != nil { t.Fatal(err) }
	s := string(b)
	if !strings.Contains(s, "synth-0") { t.Fatalf("splits.json missing synth packages:\n%s", s) }
}
```

`runSplits` is the extracted core of `stageSplits` (refactor it out so the test can call it without a `stageCtx`).

- [ ] **Step 2: Run to verify it fails; implement**

Extend `stageSplits` to scan `dataset/bspdec/synth/*/` for package dirs and fold them into the package-level train/val/test split pool.

- [ ] **Step 3: Run to verify it passes**

Expected: PASS.

- [ ] **Step 4: Report tool smoke on synth pairs**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go run ./tools/bspdec_report -data dataset/bspdec | tail -5`
Expected: synthetic rows appear with IoU near 1.0 (BRUSHLIST direct path) and no rows error out.

- [ ] **Step 5: Add the superseded pointer to `docs/BSPDEC_IMPL_PLAN.md`**

One line at the top: "Superseded by `docs/superpowers/plans/2026-09-09-bspdec-m1-brushlist-path-and-synth.md` + the M0/M1 plan series; kept for history."

- [ ] **Step 6: Commit**

```bash
git add tools/bspdec_corpus/ tools/bspdec_report/ docs/BSPDEC_IMPL_PLAN.md
git commit -m "feat(bspdec): fold synthetic corpus into splits and eval reporting"
```

---

### Task 5: Headroom study — BRUSHLIST ceiling vs treewalk baseline

Completes bead `ironwail-go-xxy.5`'s acceptance: "quantified ceiling in report; go/no-go decision for M2 recorded in bead." The study compares, per corpus pair, the treewalk baseline (current `Decompile`) against the BRUSHLIST-direct ceiling (Task 1) on tier-1 metrics (voxel IoU, lump deltas) — plus a seam/grouping proxy: brush-delta versus the original BRUSHLIST count. The verdict rule (spec §13 "After M1"): if the BRUSHLIST ceiling's IoU gain over the treewalk baseline is below the recorded threshold (`ceilingGain < 0.02` on the synth slice), record no-go for Routes A/B.

**Files:**
- Create: `internal/bspdec/eval/heads.go`
- Create: `internal/bspdec/eval/heads_test.go`
- Modify: `tools/bspdec_report/main.go` (`headroom` subcommand)

**Interfaces:**
- Consumes: `eval.EvaluatePair(pairDir, pkgID, mapID string) (eval.PairResult, error)`, `eval.BrushCounts(bspPath string) ([]int, error)`, `bspdec.Decompile` with `Options{NoBrushlist: bool}`.
- Produces:
  - `type HeadroomRow struct { PkgID, MapID string; BaselineIoU, CeilingIoU float64; BaselineBrushes, CeilingBrushes, OracleBrushes int; Gain float64 }`
  - `func ComposeHeadroomReport(pairs []eval.PairResult, brushlistRun bool) ([]HeadroomRow, float64)` — mean gain, min/max rows; `gain = ceilIoU - baseIoU` per row, overall mean.
  - `func GoVerdict(gain float64, threshold float64) string` — `"go"` / `"no-go"`.

- [ ] **Step 1: Write the failing metric test**

```go
func TestHeadroomGain(t *testing.T) {
	rows := []HeadroomRow{
		{PkgID: "a", BaselineIoU: 0.90, CeilingIoU: 0.99},
		{PkgID: "b", BaselineIoU: 0.92, CeilingIoU: 0.93},
	}
	_, mean := ComposeHeadroomReport(rows, true)
	if math.Abs(mean-0.05) > 1e-9 { t.Fatalf("mean gain = %v, want 0.05", mean) }
	if v := GoVerdict(mean, 0.02); v != "go" { t.Fatalf("verdict = %q, want go", v) }
	if v := GoVerdict(0.01, 0.02); v != "no-go" { t.Fatalf("verdict = %q, want no-go", v) }
}
```

- [ ] **Step 2: Run to verify it fails; implement `heads.go`**

`ComposeHeadroomReport` walks `pairs`; for each row it needs the baseline and ceiling IoUs — the caller (Task 5 Step 5's runner) computes them by running `EvaluatePair` twice (`Options{}` vs `Options{NoBrushlist: true}`) on the same pair and mapping `PairResult.VoxelIoU` via `eval.VoxelIoU`/`OccupancyMap` — `EvaluatePair` already computes these; add a helper `func EvaluateHeadroom(pairDir, pkgID, mapID string, useBrushlist bool) (float64, int, error)` that mirrors `EvaluatePair` but forces the direct path via `Options{NoBrushlist: !useBrushlist}`; keep the existing `EvaluatePair` unchanged.

- [ ] **Step 3: Run to verify it passes**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -run TestHeadroom -count=1`
Expected: PASS.

- [ ] **Step 4: Implement the `headroom` subcommand in `tools/bspdec_report`**

`bspdec_report headroom -data dataset/bspdec` iterates the manifest's paired entries (both `paired/` and `synth-*`), computes baseline + ceiling per pair (two `EvaluateHeadroom` calls each; the brushed `partial` maps use the same `OccupancyMap` sampling as `EvaluatePair` for comparability), emits a markdown table `| pkg | map | base IoU | ceil IoU | Δ | baseBr | ceilBr | oracleBr | err |`, then a verdict line `M1 gate: <go|no-go> (mean Δ IoU = x.xx, threshold 0.02)` plus per-route note text (Routes A/B see ceiling headroom only if the mean gain clears the threshold).

- [ ] **Step 5: Run the study on the corpus and record the decision in the bead**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go build -o /tmp/bspdec_report ./tools/bspdec_report && /tmp/bspdec_report headroom -data dataset/bspdec 2>&1 | tee /tmp/headroom.txt`
Expected: complete rows for every paired map (leaked-pair rows carry `err` but still count), verdict prints.
Then: `bd update ironwail-go-xxy.5 --notes="M1 headroom results: <paste verdict + mean/median gain, flag if baseline already near ceiling>"` and, per the recorded verdict: if `no-go`, `bd close ironwail-go-xxy.5 --reason="recovery ceiling measured: ML adds < 0.02 IoU over BRUSHLIST direct path; Routes A/B gated off per spec §13"` and file a follow-up bead adjusting M2's expected gains; if `go`, `bd close ironwail-go-xxy.5 --reason="ceiling measured: <mean gain>; M2 Routes A/B proceed"`.

- [ ] **Step 6: Commit**

```bash
git add internal/bspdec/eval/heads.go internal/bspdec/eval/heads_test.go tools/bspdec_report/main.go
git commit -m "feat(bspdec): headroom study and ML go-no-go verdict"
```

---

## Wrap-up

- [ ] Run `mise run verify` from the repo root; fix anything it flags.
- [ ] Run `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./... -count=1` and confirm the engine/renderer suites are unaffected (they skip without assets).
- [ ] Confirm both M1 beads closed (`bd list --status=closed | grep xxy.1[15]` shows `ironwail-go-xxy.5` and `ironwail-go-xxy.11`), and M2 beads `xxy.6/.7/.12` are unblocked (check `ironwail-go-xxy.6` in `bd ready`).
- [ ] Report handoff: files changed, validation evidence (synth pair count, coverage %, verdict), and the recorded go/no-go decision.

## Self-review

- **Spec coverage** — §5 step 9 (BRUSHLIST shortcut) → Task 1; §9.6 P6 (synth, 8-unit lattices, mapper conventions, `-wrbrushes` variant ≡ our always-on BRUSHLIST, `license_note: synthetic`) → Tasks 2–4; §9.4 P4 labels (BRUSHLIST truth, 100% on synth) → Task 3; §8 tier 1 + §13 M1 gate (quantified ceiling, go/no-go recorded in bead) → Task 5; §3 `--no-brushlist` flag plumbed in Task 1 (already present in `cmd/bspdec`). No spec section left uncovered.
- **Placeholder scan** — every step carries code or an exact command; no TBDs. The study's threshold value (0.02) is explicit and matches the spec's "meaningfully" by being stated in the verdict rule.
- **Type consistency** — `BrushListFromBSP([]byte) ([]BSPXBrush, error)`, `BrushFromBSPX(BSPXBrush) (mapfile.MapBrush, error)`, `Generator.New/GenMap/Emit`, `HeadroomRow{...}`, `GoVerdict(float64,float64) string`, `EvaluateHeadroom(pairDir,pkgID,mapID string,useBrushlist bool) (float64,int,error)` are used identically across Tasks. `eval.Pair`, `eval.PairResult`, `mapfile.*`, `bspdec.Options.NoBrushlist` match the existing codebase signatures verified 2026-09-09.
- **Known risks** — (1) Task 1's pipeline branch changes M0 golden expectations on compiled-BSP decompiles; the plan makes opting tests into `NoBrushlist: true` explicit. (2) The leak-fix batch (`ironwail-go-6xi` / `ironwail-go-aeh`) still blocks 36/38 real-map canonicalization; the headroom study intentionally measures the synth + clean-pair slice first and records partial-coverage caveats in the verdict, and the corpus-pair re-run of the study is re-executable once the CSG-fidelity bead lands.