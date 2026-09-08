# BSPDEC M0: Deterministic Decompiler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `bspdec`, a CLI that decompiles Quake BSP29 files into editable `.map` files with a deterministic treewalk core that meets or beats `bsputil --decompile` on geometry fidelity.

**Architecture:** Extract the ericw-faithful `.map` parser from `internal/qbsp` into `internal/map` (package `mapfile`) and add a writer. New package `internal/bspdec` ports the bspc/ericw decompiler family to Go: per-model bbox+8 box brush, node-tree recursion with winding-clip splits, leaf-cell emission by contents, redundant-plane removal, face-overlap texturing (Valve-220), texture-boundary splitting, origin brushes, convex merging, and hull un-expansion. `cmd/bspdec` wires flags, exit codes, slog, and a `--json` summary. Golden tests compile fixture maps with the in-repo `internal/qbsp` (which always appends a BRUSHLIST BSPX oracle) and compare decompiled occupancy against the original brushes by voxelized IoU.

**Tech Stack:** Go 1.26, `CGO_ENABLED=0`, stdlib only (`flag`, `log/slog`, `encoding/binary`), existing packages `internal/bsp`, `internal/qbsp`, `pkg/types`.

**Spec:** [docs/superpowers/specs/2026-09-07-bspdec-design.md](../specs/2026-09-07-bspdec-design.md) (design, §3 CLI, §5 core algorithms, §6 map package). Read it first; this plan argues from it.

**Beads:** Tasks 1–2 → `ironwail-go-xxy.1`; Tasks 3–11 → `ironwail-go-xxy.2`; Task 12 → `ironwail-go-xxy.3`. Claim each bead (`bd update <id> --claim`) before its first task; close with `bd close <id> --reason="..."` when its acceptance is met. Beads xxy.4+ are covered by later plans (see "Subsequent plans" at the end).

## Global Constraints

Every task implicitly includes these (from spec §14 and AGENTS.md):

- Pure Go, `CGO_ENABLED=0`, **no build tags** in any `.go` file, stdlib + existing repo packages only.
- `log/slog` for all diagnostics; never `fmt.Println`/`log.Printf` for logging.
- Every ported algorithm cites C lineage inline, e.g. `// Where in C: Q1_CreateBrushesFromBSP in bspc/map_q1.c`.
- Single-package test runs must match the mise env: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run TestName -count=1`.
- `mise run verify` must be green at every task boundary (it runs `go generate ./...` + full tests + build).
- `internal/qbsp`'s existing tests are the refactor oracle for Task 1: they must stay green with zero behavioral change.
- The package in `internal/map` is named `mapfile` (`map` is a Go keyword).
- Package-local test helpers prefixed `new`/`with`; no shared testutil scaffolding for subsystem fixtures.
- Commits: one per task where marked. Per repo policy, commit only with operator approval; if executing without commit approval, stage changes and report instead. Never push.
- Windings are treated as immutable: `Clip` may return the receiver; no method mutates a `Winding` in place.

---

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/map/doc.go` | package doc + C lineage |
| `internal/map/mapfile.go` | `Map`, `Entity`, `Epair`, `MapBrush`, `MapFace`, `TexDef`, `Vec3`, `Plane`, `Parse`, `PlaneFromPoints` (moved from `internal/qbsp/mapfile.go`) |
| `internal/map/vector.go` | unexported vec3 helpers used by the parser (copied from `internal/qbsp/vector.go`) |
| `internal/map/write.go` | `WriteOptions`, `Write` — Valve-220 `.map` emission, grid snap |
| `internal/map/mapfile_test.go`, `write_test.go` | parser smoke test, round-trip identity, grid snap |
| `internal/qbsp/mapfile.go` | thin alias file (`type Map = mapfile.Map` …) preserving the qbsp API |
| `internal/qbsp/vector.go` | `type vec3 = mapfile.Vec3`, `type plane = mapfile.Plane`, `planeFromPoints` wrapper |
| `internal/bspdec/doc.go` | package doc + C lineage |
| `internal/bspdec/winding.go` | `Winding`, vec helpers, `BaseWinding`, `Clip`, `Area`, `Centroid`, `negatePlane` |
| `internal/bspdec/types.go` | `Side`, `Brush`, `Options`, `ModelStats` |
| `internal/bspdec/treewalk.go` | decompiler state, `boxBrush`, `clipBrush`, `splitBrush`, model walk |
| `internal/bspdec/planes.go` | `rebuildWindings`, `removeRedundantPlanes`, hull un-expansion helpers |
| `internal/bspdec/texture.go` | face windings, plane matching, overlap texturing, fallbacks, contents textures |
| `internal/bspdec/split.go` | texture-boundary splitting (`SplitDifferentTexturedPartsOfBrush` port, simplified) |
| `internal/bspdec/entities.go` | entity-lump passthrough, bmodel attach, origin keys, `toMapBrush` |
| `internal/bspdec/merge.go` | `--merge-convex` merging |
| `internal/bspdec/hull.go` | clipnode walk, hull AABB un-expansion, bevel dropping |
| `internal/bspdec/decompile.go` | `Decompile` orchestrator, `SelfCheck`, `ValidateBrush` |
| `cmd/bspdec/main.go` | CLI: flags, exit codes, slog, `--json` |
| `mise.toml` | `build-bspdec` + stubs for `bspdec-data/train/eval/report/models` |

---

## Task 1: Extract `internal/map` from `internal/qbsp` + scaffolding

Bead: `ironwail-go-xxy.1`. The parser already exists and is ericw-faithful; we move it so both the compiler and the decompiler share one source of truth, and leave aliases so `internal/qbsp` compiles unchanged.

**Files:**
- Create: `internal/map/doc.go`, `internal/map/mapfile.go`, `internal/map/vector.go`, `internal/map/mapfile_test.go`
- Modify: `internal/qbsp/mapfile.go` (replace contents with aliases), `internal/qbsp/vector.go:9,40-43` (alias types, wrap `planeFromPoints`)
- Modify: `mise.toml` (append task stubs)

**Interfaces:**
- Consumes: nothing new (pure move).
- Produces: `mapfile.Vec3` = `[3]float64`; `mapfile.Plane{Normal Vec3; Dist float64}`; `mapfile.Map{Entities []Entity}`; `mapfile.Entity{Epairs []Epair; Brushes []MapBrush}` with `Value(key string) (string, bool)`; `mapfile.Epair{Key, Value string}`; `mapfile.MapBrush{Faces []MapFace; Line int}`; `mapfile.MapFace{Points [3]Vec3; Normal Vec3; Dist float64; TexName string; Tex TexDef; Vecs [2][4]float64; Line int}`; `mapfile.TexDef{QuakeEd bool; ShiftX, ShiftY, Rotate, ScaleX, ScaleY float64; Axis [2]Vec3}`; `mapfile.Parse(r io.Reader) (*Map, error)`; `mapfile.PlaneFromPoints(p0, p1, p2 Vec3) (Plane, float64)`.

- [ ] **Step 1: Add mise task stubs**

Append to `mise.toml` (after the `build-bspdiag` block):

```toml
[tasks.build-bspdec]
description = "Build the bspdec BSP-to-map decompiler CLI"
run = "go build -o bin/bspdec ./cmd/bspdec"

[tasks.bspdec-data]
description = "Provision the bspdec corpus (stub until the M0-eval plan lands)"
run = "echo 'bspdec-data: not implemented yet'"

[tasks.bspdec-train]
description = "Train bspdec ML routes (stub until M2)"
run = "echo 'bspdec-train: not implemented yet'"

[tasks.bspdec-eval]
description = "Run bspdec eval tiers 1+2 (stub until the M0-eval plan lands)"
run = "echo 'bspdec-eval: not implemented yet'"

[tasks.bspdec-report]
description = "Summarize bspdec eval results (stub until the M0-eval plan lands)"
run = "echo 'bspdec-report: not implemented yet'"

[tasks.bspdec-models]
description = "Provision bspdec model artifacts (stub until M2)"
run = "echo 'bspdec-models: not implemented yet'"
```

- [ ] **Step 2: Write the failing test**

Create `internal/map/mapfile_test.go`:

```go
package mapfile

import (
	"strings"
	"testing"
)

const valve220BoxFixture = `// entity 0
{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 64 0 ) ( 0 64 64 ) brick [ 0 1 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 64 0 0 ) ( 64 0 64 ) ( 64 64 64 ) brick [ 0 1 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 0 0 ) ( 64 0 0 ) ( 64 0 64 ) brick [ 1 0 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 64 0 ) ( 0 64 64 ) ( 64 64 64 ) brick [ 1 0 0 0 ] [ 0 0 -1 0 ] 0 1 1
( 0 0 0 ) ( 0 64 0 ) ( 64 64 0 ) brick [ 1 0 0 0 ] [ 0 -1 0 0 ] 0 1 1
( 0 0 64 ) ( 64 0 64 ) ( 64 64 64 ) brick [ 1 0 0 0 ] [ 0 -1 0 0 ] 0 1 1
}
}
`

func TestParseValve220Box(t *testing.T) {
	m, err := Parse(strings.NewReader(valve220BoxFixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(m.Entities))
	}
	e := m.Entities[0]
	if v, ok := e.Value("classname"); !ok || v != "worldspawn" {
		t.Fatalf("classname = %q, %v", v, ok)
	}
	if len(e.Brushes) != 1 {
		t.Fatalf("brushes = %d, want 1", len(e.Brushes))
	}
	if got := len(e.Brushes[0].Faces); got != 6 {
		t.Fatalf("faces = %d, want 6", got)
	}
	f := e.Brushes[0].Faces[0]
	if f.TexName != "brick" {
		t.Fatalf("texname = %q, want brick", f.TexName)
	}
	if f.Tex.QuakeEd {
		t.Fatal("expected Valve 220 texdef")
	}
	// ericw convention: normal = normalize(cross(p0-p1, p2-p1)) = (-1,0,0)
	if f.Normal != (Vec3{-1, 0, 0}) {
		t.Fatalf("normal = %v, want [-1 0 0]", f.Normal)
	}
	if f.Dist != 0 {
		t.Fatalf("dist = %v, want 0", f.Dist)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/map -run TestParseValve220Box -count=1`
Expected: FAIL — `package github.com/darkliquid/ironwail-go/internal/map: no Go files` (or undefined `Parse`).

- [ ] **Step 4: Move the parser into `internal/map`**

1. `git mv internal/qbsp/mapfile.go internal/map/mapfile.go` (or move with an editor; preserve history if convenient, content is what matters).
2. In the moved file: `package qbsp` → `package mapfile`; rename unexported `vec3` → exported `Vec3` and `plane` → `Plane` **throughout the moved file only**; `planeFromPoints` → exported `PlaneFromPoints`; add the package doc below as `internal/map/doc.go`.
3. Create `internal/map/vector.go`: copy from `internal/qbsp/vector.go` every helper the moved file references (e.g. `v3Sub`, `v3Cross`, `v3Dot`, `v3Scale`, `v3Normalize` — let the compiler list undefined symbols; copy each verbatim, operating on `Vec3`). Do not share these with qbsp; duplication is deliberate.
4. Replace `internal/qbsp/mapfile.go` contents with:

```go
package qbsp

import (
	"io"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// The .map parser and its types live in internal/map (package mapfile) so the
// decompiler can share them; these aliases keep that single source of truth
// without changing this package's API.
type (
	Map      = mapfile.Map
	Entity   = mapfile.Entity
	Epair    = mapfile.Epair
	MapBrush = mapfile.MapBrush
	MapFace  = mapfile.MapFace
	TexDef   = mapfile.TexDef
)

// ParseMap parses a Quake .map file (QuakeEd and Valve 220 texture formats).
func ParseMap(r io.Reader) (*Map, error) { return mapfile.Parse(r) }
```

5. In `internal/qbsp/vector.go`: replace `type vec3 [3]float64` with `type vec3 = mapfile.Vec3` and the `plane` struct definition with `type plane = mapfile.Plane`; replace the body of `planeFromPoints` with `return mapfile.PlaneFromPoints(p0, p1, p2)` (keep the signature `(p0, p1, p2 vec3) (plane, float64)`). Add the mapfile import. Everything else in qbsp keeps compiling because aliases are identical types.

`internal/map/doc.go`:

```go
// Package mapfile reads and writes Quake .map files (QuakeEd and Valve 220
// texture formats), shared by the qbsp compiler and the bspdec decompiler.
//
// # Original C lineage
//
// The parser is a Go port of ericw-tools common/mapfile.cc (tokenizer,
// entity/brush grammar, both texture-definition forms). The writer emits
// Valve 220 face lines.
package mapfile
```

- [ ] **Step 5: Run the tests**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/map ./internal/qbsp -count=1 && mise run verify`
Expected: PASS; qbsp suite green (unchanged behavior); full build green.

- [ ] **Step 6: Commit**

```bash
git add internal/map internal/qbsp/mapfile.go internal/qbsp/vector.go mise.toml
git commit -m "refactor: extract .map parser into internal/map for shared compiler/decompiler use"
```

---

## Task 2: `internal/map` writer with grid snap

Bead: `ironwail-go-xxy.1`. Spec §6: Valve-220 face lines, entity blocks verbatim (raw bytes, no Go quoting escapes — Quake text may carry high-bit glyph bytes), grid-snap quantization of emitted plane points, round-trip identity.

**Files:**
- Create: `internal/map/write.go`, `internal/map/write_test.go`

**Interfaces:**
- Consumes: `mapfile.Map`, `MapFace` from Task 1.
- Produces: `WriteOptions{GridSnap int}`; `Write(w io.Writer, m *Map, opts WriteOptions) error`. Writer emits face axes **from `MapFace.Vecs`** (`[ sx sy sz soff ]` per axis) with `rot 0, scale 1 1` — this is the canonical form the decompiler produces and what the round-trip property covers. Callers must set `Points`, `TexName`, and `Vecs` on emitted faces.

- [ ] **Step 1: Write the failing test**

Create `internal/map/write_test.go`:

```go
package mapfile

import (
	"bytes"
	"strings"
	"testing"
)

// equalCanonical compares the canonical brush definition: entity/epair order
// and per-face (Points, TexName, Vecs). TexDef surface form may differ after a
// rewrite (rotation/scale folded into axes), Vecs may not.
func equalCanonical(a, b *Map) bool {
	if len(a.Entities) != len(b.Entities) {
		return false
	}
	for i := range a.Entities {
		ea, eb := &a.Entities[i], &b.Entities[i]
		if len(ea.Epairs) != len(eb.Epairs) || len(ea.Brushes) != len(eb.Brushes) {
			return false
		}
		for j := range ea.Epairs {
			if ea.Epairs[j] != eb.Epairs[j] {
				return false
			}
		}
		for j := range ea.Brushes {
			ba, bb := &ea.Brushes[j], &eb.Brushes[j]
			if len(ba.Faces) != len(bb.Faces) {
				return false
			}
			for k := range ba.Faces {
				fa, fb := &ba.Faces[k], &bb.Faces[k]
				if fa.Points != fb.Points || fa.TexName != fb.TexName || fa.Vecs != fb.Vecs {
					return false
				}
			}
		}
	}
	return true
}

func TestWriteParseRoundTrip(t *testing.T) {
	m1, err := Parse(strings.NewReader(valve220BoxFixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, m1, WriteOptions{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	m2, err := Parse(&buf)
	if err != nil {
		t.Fatalf("re-parse of written output: %v\n%s", err, buf.String())
	}
	if !equalCanonical(m1, m2) {
		t.Fatalf("round trip mismatch, wrote:\n%s", buf.String())
	}
}

func TestWriteGridSnap(t *testing.T) {
	src := strings.Replace(valve220BoxFixture, "( 0 64 0 ) ( 0 64 64 ) brick", "( 0 65 0 ) ( 0 65 66 ) brick", 1)
	// note: use a distinct 2nd point to stay non-collinear after snapping
	m, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, m, WriteOptions{GridSnap: 8}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "65") || strings.Contains(out, "66") {
		t.Fatalf("unsnapped points in output:\n%s", out)
	}
	if !strings.Contains(out, "( 0 64 0 ) ( 0 64 64 )") {
		t.Fatalf("expected snapped points in output:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/map -run 'TestWriteParseRoundTrip|TestWriteGridSnap' -count=1`
Expected: FAIL — `undefined: Write` / `undefined: WriteOptions`.

- [ ] **Step 3: Implement the writer**

Create `internal/map/write.go`:

```go
package mapfile

import (
	"fmt"
	"io"
	"math"
	"strconv"
)

// WriteOptions controls .map emission.
type WriteOptions struct {
	// GridSnap quantizes emitted plane points to multiples of N (0 = off).
	// Mappers work on integer lattices; 8 is the decompiler default.
	GridSnap int
}

// Write emits m as a Quake .map: entity blocks in order, epairs first (file
// order preserved), then brushes as Valve 220 face lines.
//
// Where in C: ericw-tools common/mapfile.cc token conventions; Valve 220
// face form per ericw qbsp docs.
func Write(w io.Writer, m *Map, opts WriteOptions) error {
	for i := range m.Entities {
		if err := writeEntity(w, &m.Entities[i], opts); err != nil {
			return err
		}
	}
	return nil
}

func writeEntity(w io.Writer, e *Entity, opts WriteOptions) error {
	if _, err := fmt.Fprintln(w, "{"); err != nil {
		return err
	}
	for _, p := range e.Epairs {
		// Raw quoting, not %q: Quake text may carry high-bit glyph bytes
		// which Go escapes would corrupt (see AGENTS.md console-text note).
		if _, err := fmt.Fprintf(w, "\"%s\" \"%s\"\n", p.Key, p.Value); err != nil {
			return err
		}
	}
	for i := range e.Brushes {
		if err := writeBrush(w, &e.Brushes[i], opts); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}

func writeBrush(w io.Writer, b *MapBrush, opts WriteOptions) error {
	if _, err := fmt.Fprintln(w, "{"); err != nil {
		return err
	}
	for i := range b.Faces {
		if err := writeFace(w, &b.Faces[i], opts); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}

// writeFace emits one Valve 220 face line:
//
//	( p0 ) ( p1 ) ( p2 ) tex [ ux uy uz uoff ] [ vx vy vz voff ] rot sx sy
//
// Axes come from the computed Vecs so both QuakeEd- and Valve-parsed faces
// emit a single canonical form (rotation/scale already folded into Vecs).
func writeFace(w io.Writer, f *MapFace, opts WriteOptions) error {
	pts := f.Points
	if opts.GridSnap > 0 {
		pts = snapPoints(pts, opts.GridSnap)
	}
	u := Vec3{f.Vecs[0][0], f.Vecs[0][1], f.Vecs[0][2]}
	v := Vec3{f.Vecs[1][0], f.Vecs[1][1], f.Vecs[1][2]}
	_, err := fmt.Fprintf(w, "( %s ) ( %s ) ( %s ) %s [ %s %s ] [ %s %s ] 0 1 1\n",
		fmtPoint(pts[0]), fmtPoint(pts[1]), fmtPoint(pts[2]),
		f.TexName,
		fmtPoint(u), fmtNum(f.Vecs[0][3]),
		fmtPoint(v), fmtNum(f.Vecs[1][3]))
	return err
}

func snapPoints(pts [3]Vec3, n int) [3]Vec3 {
	var out [3]Vec3
	f := float64(n)
	for i, p := range pts {
		for a := 0; a < 3; a++ {
			out[i][a] = math.Round(p[a]/f) * f
		}
	}
	return out
}

func fmtPoint(p Vec3) string {
	return fmtNum(p[0]) + " " + fmtNum(p[1]) + " " + fmtNum(p[2])
}

// fmtNum renders integers without a decimal point (mapper convention) and
// everything else in shortest round-trip form.
func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/map -count=1 && mise run verify`
Expected: PASS. If the round trip fails on `Vecs` mismatch, inspect `computeVecs` for Valve-220 scale handling — the writer emits whatever `Vecs` holds, so the failure is in the test fixture, not the writer; the fixture above is scale-1/rot-0 which round-trips.

- [ ] **Step 5: Commit**

```bash
git add internal/map/write.go internal/map/write_test.go
git commit -m "feat(map): add Valve 220 .map writer with grid snap and round-trip identity"
```

Bead `ironwail-go-xxy.1` acceptance is now met (round-trip identity test green). Close it: `bd close ironwail-go-xxy.1 --reason="internal/map extracted from qbsp with aliases; writer with grid snap + round-trip identity tests green"`.

---

## Task 3: `internal/bspdec` skeleton + winding math

Bead: `ironwail-go-xxy.2`. The winding is the workhorse primitive for every later task: split, clip, area. Get it exactly right here with unit tests before building on it.

**Files:**
- Create: `internal/bspdec/doc.go`, `internal/bspdec/winding.go`, `internal/bspdec/winding_test.go`

**Interfaces:**
- Consumes: `mapfile.Vec3`, `mapfile.Plane` (Task 1).
- Produces (everything later relies on these exact signatures):
  - `type Winding struct { Points []mapfile.Vec3 }` — a convex polygon; point order is consistent but orientation (CW/CCW) is not relied on anywhere.
  - `func BaseWinding(p mapfile.Plane) *Winding` — huge quad on the plane.
  - `func (w *Winding) Clip(p mapfile.Plane) *Winding` — keeps the **front** (`dot(n,x) >= dist`, on-plane kept); returns nil when nothing survives; may return the receiver.
  - `func (w *Winding) Area() float64` — always non-negative.
  - `func (w *Winding) Centroid() mapfile.Vec3`
  - `func negatePlane(p mapfile.Plane) mapfile.Plane`
  - vec helpers `v3Add/v3Sub/v3Dot/v3Cross/v3Scale/v3Len/v3Normalize` on `mapfile.Vec3`.

- [ ] **Step 1: Write the failing tests**

Create `internal/bspdec/winding_test.go`:

```go
package bspdec

import (
	"math"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

func newSquareWinding() *Winding {
	return &Winding{Points: []mapfile.Vec3{{0, 0, 0}, {64, 0, 0}, {64, 64, 0}, {0, 64, 0}}}
}

func TestClipKeepsFrontHalf(t *testing.T) {
	got := newSquareWinding().Clip(mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 32})
	if got == nil {
		t.Fatal("winding vanished")
	}
	if len(got.Points) != 4 {
		t.Fatalf("points = %d, want 4 (%v)", len(got.Points), got.Points)
	}
	if a := got.Area(); math.Abs(a-32*64) > 0.5 {
		t.Fatalf("area = %v, want %v", a, 32*64)
	}
	for _, p := range got.Points {
		if p[0] < 32-0.01 {
			t.Fatalf("point %v behind clip plane", p)
		}
	}
}

func TestClipFullyBehindReturnsNil(t *testing.T) {
	if got := newSquareWinding().Clip(mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 100}); got != nil {
		t.Fatalf("expected nil, got %v", got.Points)
	}
}

func TestClipFullyInFrontReturnsReceiver(t *testing.T) {
	w := newSquareWinding()
	if got := w.Clip(mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: -5}); got != w {
		t.Fatal("expected receiver back for unclipped winding")
	}
}

func TestBaseWindingLiesOnPlane(t *testing.T) {
	w := BaseWinding(mapfile.Plane{Normal: mapfile.Vec3{0, 0, 1}, Dist: 64})
	if w == nil || len(w.Points) != 4 {
		t.Fatalf("base winding = %v", w)
	}
	for _, p := range w.Points {
		if math.Abs(p[2]-64) > 0.01 {
			t.Fatalf("point %v off plane z=64", p)
		}
	}
	if a := w.Area(); a < 1e9 {
		t.Fatalf("base winding suspiciously small: %v", a)
	}
}

func TestAreaIsOrientationIndependent(t *testing.T) {
	a := newSquareWinding()
	rev := &Winding{Points: []mapfile.Vec3{a.Points[3], a.Points[2], a.Points[1], a.Points[0]}}
	if math.Abs(a.Area()-rev.Area()) > 1e-9 || a.Area() != 64*64 {
		t.Fatalf("areas %v vs %v", a.Area(), rev.Area())
	}
}

func TestCentroid(t *testing.T) {
	c := newSquareWinding().Centroid()
	if c != (mapfile.Vec3{32, 32, 0}) {
		t.Fatalf("centroid = %v, want [32 32 0]", c)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1`
Expected: FAIL — no Go files / undefined symbols.

- [ ] **Step 3: Implement winding math**

Create `internal/bspdec/doc.go`:

```go
// Package bspdec decompiles Quake BSP29 files into editable .map files.
//
// # Original C lineage
//
// The deterministic core ports the bspc/ericw-tools decompiler family:
// id Q3A bspc/map_q1.c (Q1_CreateBrushesFromBSP, Q1_CreateBrushes_r,
// Q1_FixContentsTextures), ericw-tools common/decompile.cc
// (RemoveRedundantPlanes, BuildInitialBrush,
// SplitDifferentTexturedPartsOfBrush) and common/polylib.cc
// (BaseWindingForPlane, ClipWindingEpsilon).
package bspdec
```

Create `internal/bspdec/winding.go`:

```go
package bspdec

import (
	"math"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// onEpsilon matches ericw's decompile-side epsilon; generous because BSP
// float32 plane dists drift relative to the float64 brush math.
const onEpsilon = 0.1

// baseWindingExtent is the half-size of the initial plane quad. Quake maps
// fit in +/-32768; double it for headroom (ericw uses a similar overshoot).
const baseWindingExtent = 65536

// Winding is a convex polygon. Windings are immutable: Clip returns new
// storage (or the receiver when nothing was clipped).
type Winding struct {
	Points []mapfile.Vec3
}

func v3Add(a, b mapfile.Vec3) mapfile.Vec3 { return mapfile.Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func v3Sub(a, b mapfile.Vec3) mapfile.Vec3 { return mapfile.Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func v3Dot(a, b mapfile.Vec3) float64      { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func v3Scale(a mapfile.Vec3, s float64) mapfile.Vec3 {
	return mapfile.Vec3{a[0] * s, a[1] * s, a[2] * s}
}
func v3Cross(a, b mapfile.Vec3) mapfile.Vec3 {
	return mapfile.Vec3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
func v3Len(a mapfile.Vec3) float64 { return math.Sqrt(v3Dot(a, a)) }
func v3Normalize(a mapfile.Vec3) mapfile.Vec3 {
	l := v3Len(a)
	if l == 0 {
		return mapfile.Vec3{}
	}
	return v3Scale(a, 1/l)
}

func negatePlane(p mapfile.Plane) mapfile.Plane {
	return mapfile.Plane{Normal: v3Scale(p.Normal, -1), Dist: -p.Dist}
}

// BaseWinding returns a huge quad on p.
//
// Where in C: BaseWindingForPlane in ericw-tools common/polylib.cc (id
// winding.cc).
func BaseWinding(p mapfile.Plane) *Winding {
	// dominant axis of the normal chooses the "up" reference vector
	x := 0
	max := math.Abs(p.Normal[0])
	for i := 1; i < 3; i++ {
		if v := math.Abs(p.Normal[i]); v > max {
			max = v
			x = i
		}
	}
	up := mapfile.Vec3{}
	if x == 0 || x == 1 {
		up[2] = 1
	} else {
		up[0] = 1
	}
	// project up onto the plane and normalize
	v := v3Dot(up, p.Normal)
	up = v3Sub(up, v3Scale(p.Normal, v))
	up = v3Normalize(up)
	org := v3Scale(p.Normal, p.Dist)
	right := v3Cross(up, p.Normal)
	up = v3Scale(up, baseWindingExtent)
	right = v3Scale(right, baseWindingExtent)
	return &Winding{Points: []mapfile.Vec3{
		v3Add(v3Sub(org, right), up),
		v3Add(v3Add(org, right), up),
		v3Sub(v3Add(org, right), up),
		v3Sub(v3Sub(org, right), up),
	}}
}

const (
	sideFront = iota
	sideBack
	sideOn
)

// Clip returns the part of w in front of p (dot(n,x) >= dist), or nil when
// nothing survives. On-plane points are kept.
//
// Where in C: ClipWindingEpsilon in ericw-tools common/polylib.cc.
func (w *Winding) Clip(p mapfile.Plane) *Winding {
	n := len(w.Points)
	dists := make([]float64, n+1)
	sides := make([]int, n+1)
	counts := [3]int{}
	for i, pt := range w.Points {
		d := v3Dot(pt, p.Normal) - p.Dist
		dists[i] = d
		switch {
		case d > onEpsilon:
			sides[i] = sideFront
			counts[sideFront]++
		case d < -onEpsilon:
			sides[i] = sideBack
			counts[sideBack]++
		default:
			sides[i] = sideOn
			counts[sideOn]++
		}
	}
	sides[n] = sides[0]
	dists[n] = dists[0]
	if counts[sideFront] == 0 {
		return nil
	}
	if counts[sideBack] == 0 {
		return w
	}
	out := &Winding{Points: make([]mapfile.Vec3, 0, n+4)}
	for i, pt := range w.Points {
		if sides[i] == sideOn {
			out.Points = append(out.Points, pt)
			continue
		}
		if sides[i] == sideFront {
			out.Points = append(out.Points, pt)
		}
		if sides[i+1] == sideOn || sides[i+1] == sides[i] {
			continue
		}
		// edge crosses the plane: interpolate the split point
		j := (i + 1) % n
		d := dists[i] / (dists[i] - dists[j])
		mid := v3Add(pt, v3Scale(v3Sub(w.Points[j], pt), d))
		out.Points = append(out.Points, mid)
	}
	if len(out.Points) < 3 {
		return nil
	}
	return out
}

// Area returns the polygon area (always non-negative).
//
// Where in C: WindingArea in id winding.cc.
func (w *Winding) Area() float64 {
	var total mapfile.Vec3
	for i := 2; i < len(w.Points); i++ {
		total = v3Add(total, v3Cross(v3Sub(w.Points[i-1], w.Points[0]), v3Sub(w.Points[i], w.Points[0])))
	}
	return v3Len(total) / 2
}

// Centroid returns the average of the winding's points.
func (w *Winding) Centroid() mapfile.Vec3 {
	var c mapfile.Vec3
	for _, p := range w.Points {
		c = v3Add(c, p)
	}
	if len(w.Points) > 0 {
		c = v3Scale(c, 1/float64(len(w.Points)))
	}
	return c
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS, all 6 winding tests.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/doc.go internal/bspdec/winding.go internal/bspdec/winding_test.go
git commit -m "feat(bspdec): add winding polygon math ported from ericw polylib"
```

---

## Task 4: Brush type + treewalk core

Bead: `ironwail-go-xxy.2`. Spec §5 steps 1–2: bbox+8 box brush per model, node recursion splitting by node planes, emit a brush per non-empty leaf. Every solid-child termination emits a cell even though all solid areas share leaf 0 — the tree path, not the leaf index, defines the cell.

**Files:**
- Create: `internal/bspdec/types.go`, `internal/bspdec/treewalk.go`, `internal/bspdec/treewalk_test.go`

**Interfaces:**
- Consumes: `Winding` machinery (Task 3); `bsp.Tree` fields `Planes []bsp.DPlane`, `Nodes []bsp.TreeNode` (`PlaneNum int32`, `Children [2]bsp.TreeChild{IsLeaf bool; Index int}` — children[0] is the plane front per WinQuake `bspfile.h` `dnode_t`), `Leafs []bsp.TreeLeaf` (`Contents int32`), `Models []bsp.DModel` (`BoundsMin/Max types.Vec3`, `HeadNode [4]int32`).
- Produces:
  - `type Side struct { Plane mapfile.Plane; Winding *Winding; TexName string; Vecs [2][4]float64 }`
  - `type Brush struct { Sides []*Side; Contents int32 }`
  - `type Options struct { NoBrushlist bool; DecompileHull int; MergeConvex bool; GridSnap int; TextureFallback string }` (ML flags are CLI-only in M0, rejected there)
  - `type ModelStats struct { Model, Brushes, LeavesSolid, PlanesUsed, Warnings int }`
  - `boxBrush(mins, maxs mapfile.Vec3) *Brush`
  - `clipBrush(b *Brush, p mapfile.Plane) *Brush` — keeps the back of p (`dot(n,x) <= dist`), adds p as a new side only when it bounds the result; nil when the brush is entirely in front.
  - `splitBrush(b *Brush, p mapfile.Plane) (front, back *Brush)`
  - `rebuildWindings(b *Brush)`
  - `newDecompiler(tree *bsp.Tree, opts Options) *decompiler`
  - `(d *decompiler) decompileModel(modelIdx int) ([]*Brush, error)`

- [ ] **Step 1: Write the failing tests**

Create `internal/bspdec/treewalk_test.go`:

```go
package bspdec

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// slabBox returns one QuakeEd-format box brush string. The face winding
// pattern is copied from internal/qbsp/mapfile_test.go's slabBrush, which the
// qbsp suite compiles successfully, so orientation is compile-proven.
func slabBox(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	format := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mins := [3]float64{x0, y0, z0}
	maxs := [3]float64{x1, y1, z1}
	return "{\n" +
		format([3]float64{maxs[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], maxs[2]}, [3]float64{maxs[0], maxs[1], mins[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{mins[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{maxs[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{mins[0], mins[1], maxs[2]}, [3]float64{maxs[0], mins[1], mins[2]}) +
		format([3]float64{mins[0], mins[1], maxs[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{maxs[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], mins[2]}, [3]float64{mins[0], maxs[1], mins[2]}) +
		"}\n"
}

// roomMap is a sealed room: interior x,y in [0,256], z in [0,192], 64-unit
// walls, with floor/wall/ceiling textured differently for the texturing
// tasks. info_player_start is required: qbsp leak detection needs an entity
// inside the empty region.
func roomMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(-64, -64, -64, 0, 320, 256, "wwall") +
		slabBox(256, -64, -64, 320, 320, 256, "wwall") +
		slabBox(0, -64, -64, 256, 0, 256, "wwall") +
		slabBox(0, 256, -64, 256, 320, 256, "wwall") +
		slabBox(0, 0, -64, 256, 256, 0, "ffloor") +
		slabBox(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
}

// compileFixture compiles mapSrc with the in-repo qbsp and loads the BSP
// tree; the compile always appends a BRUSHLIST BSPX oracle.
func compileFixture(t *testing.T, mapSrc string) (*bsp.Tree, []byte) {
	t.Helper()
	m, err := qbsp.ParseMap(strings.NewReader(mapSrc))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("fixture leaks; fix the fixture")
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	return tree, res.Data
}

func TestBoxBrushHasSixClosedSides(t *testing.T) {
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	if len(b.Sides) != 6 {
		t.Fatalf("sides = %d, want 6", len(b.Sides))
	}
	for i, s := range b.Sides {
		if s.Winding == nil || len(s.Winding.Points) != 4 {
			t.Fatalf("side %d winding not a closed quad", i)
		}
		if a := s.Winding.Area(); a < 64*64-1 {
			t.Fatalf("side %d area = %v, want 4096", i, a)
		}
	}
}

func TestSplitBrushProducesTwoHalves(t *testing.T) {
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	front, back := splitBrush(b, mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 32})
	if front == nil || back == nil {
		t.Fatalf("front=%v back=%v, want both non-nil", front != nil, back != nil)
	}
	for name, half := range map[string]*Brush{"front": front, "back": back} {
		if len(half.Sides) != 6 {
			t.Fatalf("%s sides = %d, want 6", name, len(half.Sides))
		}
		vol := 0.0
		for _, s := range half.Sides {
			if s.Winding == nil {
				t.Fatalf("%s has nil winding", name)
			}
			vol += s.Winding.Area()
		}
		if vol < 6*64*32-64 {
			t.Fatalf("%s total area %v implausibly small", name, vol)
		}
	}
}

func TestDecompileModelEmitsSolidCells(t *testing.T) {
	tree, data := compileFixture(t, roomMap())
	counts, err := qbsp.ReadBSPXBrushList(data)
	if err != nil {
		t.Fatalf("ReadBSPXBrushList: %v", err)
	}
	if len(counts) == 0 || counts[0] != 6 {
		t.Fatalf("BRUSHLIST oracle = %v, want [6]", counts)
	}
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	// CSG splits can only increase cell count vs the original 6 brushes.
	if len(brushes) < counts[0] {
		t.Fatalf("brushes = %d, want >= %d (oracle)", len(brushes), counts[0])
	}
	if d.leavesSolid == 0 {
		t.Fatal("no solid leaves visited")
	}
	for i, b := range brushes {
		if b.Contents == bsp.ContentsEmpty {
			t.Fatalf("brush %d emitted with empty contents", i)
		}
		if len(b.Sides) < 4 {
			t.Fatalf("brush %d has %d sides", i, len(b.Sides))
		}
		for _, s := range b.Sides {
			if s.Winding == nil || len(s.Winding.Points) < 3 {
				t.Fatalf("brush %d has degenerate side", i)
			}
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestBoxBrush|TestSplitBrush|TestDecompileModel' -count=1`
Expected: FAIL — undefined `boxBrush`, `splitBrush`, `newDecompiler`.

- [ ] **Step 3: Implement the treewalk core**

Create `internal/bspdec/types.go`:

```go
package bspdec

import (
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// Side is one convex brush face: its plane, surviving polygon, and texture.
type Side struct {
	Plane   mapfile.Plane
	Winding *Winding
	TexName string
	// Vecs is the s/t texture mapping copied from BSP texinfo, emitted as
	// Valve 220 axes by the map writer.
	Vecs [2][4]float64
}

// Brush is a convex cell: a set of halfspace sides with windings.
type Brush struct {
	Sides    []*Side
	Contents int32 // bsp.Contents* of the leaf the cell came from
}

// Options controls a Decompile run. ML flags are CLI-rejected in M0.
type Options struct {
	NoBrushlist     bool   // reserved for M1; M0 has no BRUSHLIST path
	DecompileHull   int    // 0 = render hull; 1-3 = collision hull (replaces hull 0 output)
	MergeConvex     bool   // merge same-contents coplanar-adjacent convex cells
	GridSnap        int    // quantize emitted plane points to this lattice (0 = off)
	TextureFallback string // "skip" | "nearest" | "trigger"
}

// ModelStats is the per-model summary reported by --json.
type ModelStats struct {
	Model       int
	Brushes     int
	LeavesSolid int
	PlanesUsed  int
	Warnings    int
}
```

Create `internal/bspdec/treewalk.go`:

```go
package bspdec

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// bboxGrow is how far the per-model seed box extends past the dmodel bounds.
//
// Where in C: Q1_CreateBrushesFromBSP in bspc map_q1.c (bbox + 8).
const bboxGrow = 8

type decompiler struct {
	tree        *bsp.Tree
	opts        Options
	warnings    int
	leavesSolid int
}

func newDecompiler(tree *bsp.Tree, opts Options) *decompiler {
	return &decompiler{tree: tree, opts: opts}
}

func (d *decompiler) warnf(format string, args ...any) {
	d.warnings++
	slog.Warn("bspdec: " + fmt.Sprintf(format, args...))
}

// plane converts a lump plane to float64 math.
func (d *decompiler) plane(i int32) mapfile.Plane {
	p := d.tree.Planes[i]
	return mapfile.Plane{
		Normal: mapfile.Vec3{float64(p.Normal.X), float64(p.Normal.Y), float64(p.Normal.Z)},
		Dist:   float64(p.Dist),
	}
}

// boxBrush builds an axial box brush from mins/maxs.
func boxBrush(mins, maxs mapfile.Vec3) *Brush {
	b := &Brush{Contents: bsp.ContentsSolid}
	planes := []mapfile.Plane{
		{Normal: mapfile.Vec3{1, 0, 0}, Dist: maxs[0]},
		{Normal: mapfile.Vec3{-1, 0, 0}, Dist: -mins[0]},
		{Normal: mapfile.Vec3{0, 1, 0}, Dist: maxs[1]},
		{Normal: mapfile.Vec3{0, -1, 0}, Dist: -mins[1]},
		{Normal: mapfile.Vec3{0, 0, 1}, Dist: maxs[2]},
		{Normal: mapfile.Vec3{0, 0, -1}, Dist: -mins[2]},
	}
	for _, p := range planes {
		b.Sides = append(b.Sides, &Side{Plane: p})
	}
	rebuildWindings(b)
	return b
}

// rebuildWindings recomputes every side's winding from the halfspace set:
// the side's base winding clipped by all other planes.
//
// Where in C: BuildInitialBrush in ericw-tools common/decompile.cc.
func rebuildWindings(b *Brush) {
	for i, s := range b.Sides {
		w := BaseWinding(s.Plane)
		for j, o := range b.Sides {
			if i == j {
				continue
			}
			w = w.Clip(negatePlane(o.Plane))
			if w == nil {
				break
			}
		}
		s.Winding = w
	}
}

// clipBrush keeps the part of b behind p (dot(n,x) <= dist) and adds p as a
// new side when it actually bounds the result. Returns nil when b is entirely
// in front of p. When b does not touch p the geometry is returned unchanged
// (no new side).
func clipBrush(b *Brush, p mapfile.Plane) *Brush {
	out := &Brush{Contents: b.Contents}
	for _, s := range b.Sides {
		w := s.Winding.Clip(negatePlane(p))
		if w == nil {
			continue // this side does not bound the kept part
		}
		out.Sides = append(out.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs, Winding: w})
	}
	if len(out.Sides) == 0 {
		return nil // entirely in front of p
	}
	nw := BaseWinding(p)
	for _, s := range out.Sides {
		nw = nw.Clip(negatePlane(s.Plane))
		if nw == nil {
			break
		}
	}
	if nw != nil {
		out.Sides = append(out.Sides, &Side{Plane: p, Winding: nw})
	}
	if len(out.Sides) < 4 {
		return nil // degenerate fragment, not a polyhedron
	}
	return out
}

// splitBrush splits b by p into the front and back parts (either nil).
//
// Where in C: brush splitting under Q1_CreateBrushes_r in bspc map_q1.c.
func splitBrush(b *Brush, p mapfile.Plane) (front, back *Brush) {
	return clipBrush(b, negatePlane(p)), clipBrush(b, p)
}

// decompileModel walks one model's render tree and returns one brush per
// non-empty leaf cell.
//
// Where in C: Q1_CreateBrushesFromBSP + Q1_CreateBrushes_r in bspc map_q1.c.
func (d *decompiler) decompileModel(modelIdx int) ([]*Brush, error) {
	m := d.tree.Models[modelIdx]
	mins := mapfile.Vec3{
		float64(m.BoundsMin.X) - bboxGrow,
		float64(m.BoundsMin.Y) - bboxGrow,
		float64(m.BoundsMin.Z) - bboxGrow,
	}
	maxs := mapfile.Vec3{
		float64(m.BoundsMax.X) + bboxGrow,
		float64(m.BoundsMax.Y) + bboxGrow,
		float64(m.BoundsMax.Z) + bboxGrow,
	}
	head := m.HeadNode[0]
	if head < 0 || int(head) >= len(d.tree.Nodes) {
		return nil, fmt.Errorf("model %d: bad headnode %d", modelIdx, head)
	}
	var out []*Brush
	d.walk(int(head), boxBrush(mins, maxs), &out)
	return out, nil
}

// walk recurses the node tree. children[0] is the plane front, children[1]
// the back (WinQuake bspfile.h dnode_t). Every non-empty leaf termination
// emits a brush: all solid areas share leaf 0, so the path defines the cell,
// not the leaf index.
func (d *decompiler) walk(nodeIdx int, b *Brush, out *[]*Brush) {
	node := d.tree.Nodes[nodeIdx]
	front, back := splitBrush(b, d.plane(node.PlaneNum))
	parts := [2]*Brush{front, back}
	for side := 0; side < 2; side++ {
		part := parts[side]
		if part == nil {
			continue
		}
		child := node.Children[side]
		if child.IsLeaf {
			leaf := d.tree.Leafs[child.Index]
			if leaf.Contents == bsp.ContentsEmpty {
				continue
			}
			if leaf.Contents == bsp.ContentsSolid {
				d.leavesSolid++
			}
			part.Contents = leaf.Contents
			*out = append(*out, part)
			continue
		}
		d.walk(child.Index, part, out)
	}
}
```

`rebuildWindings` lives here (used by `boxBrush`); Task 5 adds the removal pass in `planes.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS. If `TestDecompileModelEmitsSolidCells` sees `brushes < 6`, check child front/back order and leaf index decoding against `internal/bsp/tree.go` `loadNodes`/`loadLeafs` before changing logic.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/types.go internal/bspdec/treewalk.go internal/bspdec/treewalk_test.go
git commit -m "feat(bspdec): add treewalk decompiler core (bbox+8 seed, node splits, leaf cells)"
```

---

## Task 5: Redundant plane removal

Bead: `ironwail-go-xxy.2`. Spec §5 step 3. The treewalk leaves hundreds of node-only planes on each cell; dropping planes whose halfspace contributes no area is what makes output brush counts plausible.

**Files:**
- Create: `internal/bspdec/planes.go`, `internal/bspdec/planes_test.go`

**Interfaces:**
- Consumes: `Brush`, `rebuildWindings` (Task 4).
- Produces: `removeRedundantPlanes(b *Brush)`; `planesMatch(a, b mapfile.Plane) bool`; `planesOpposite(a, b mapfile.Plane) bool`; `planeSetCount(brushes []*Brush) int`.

- [ ] **Step 1: Write the failing test**

Create `internal/bspdec/planes_test.go`:

```go
package bspdec

import (
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

func TestRemoveRedundantPlanes(t *testing.T) {
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	// a distant plane that cuts nothing is redundant
	b.Sides = append(b.Sides, &Side{Plane: mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 1000}})
	removeRedundantPlanes(b)
	if len(b.Sides) != 6 {
		t.Fatalf("sides = %d, want 6", len(b.Sides))
	}
	for i, s := range b.Sides {
		if s.Winding == nil || s.Winding.Area() < 64*64-1 {
			t.Fatalf("side %d lost area after pruning", i)
		}
	}
	// and a real plane is never redundant
	removeRedundantPlanes(b)
	if len(b.Sides) != 6 {
		t.Fatalf("second pass changed sides = %d", len(b.Sides))
	}
}

func TestPlanesMatch(t *testing.T) {
	a := mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 64}
	if !planesMatch(a, mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 64.01}) {
		t.Fatal("near-identical planes should match")
	}
	if planesMatch(a, mapfile.Plane{Normal: mapfile.Vec3{1, 0, 0}, Dist: 65}) {
		t.Fatal("distinct dists should not match")
	}
	if planesMatch(a, negatePlane(a)) {
		t.Fatal("opposite planes should not match")
	}
	if !planesOpposite(a, negatePlane(a)) {
		t.Fatal("negated plane should be opposite")
	}
}

func TestDecompiledCellsShrinkAfterPruning(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	totalBefore := 0
	for _, b := range brushes {
		totalBefore += len(b.Sides)
		removeRedundantPlanes(b)
	}
	totalAfter := 0
	for _, b := range brushes {
		totalAfter += len(b.Sides)
		if len(b.Sides) < 4 {
			t.Fatalf("brush pruned below 4 sides: %d", len(b.Sides))
		}
	}
	if totalAfter >= totalBefore {
		t.Fatalf("pruning removed nothing: before=%d after=%d", totalBefore, totalAfter)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestRemoveRedundant|TestPlanesMatch|TestDecompiledCells' -count=1`
Expected: FAIL — undefined `removeRedundantPlanes`, `planesMatch`, `planesOpposite`.

- [ ] **Step 3: Implement `planes.go`**

Create `internal/bspdec/planes.go`:

```go
package bspdec

import (
	"math"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// planeMatchEpsilon covers float32 BSP plane dists vs float64 brush math.
const planeMatchEpsilon = 0.05

// planesMatch reports whether a and b are the same oriented plane.
func planesMatch(a, b mapfile.Plane) bool {
	return v3Dot(a.Normal, b.Normal) > 1-1e-4 && math.Abs(a.Dist-b.Dist) < planeMatchEpsilon
}

// planesOpposite reports whether a and b are the same plane, opposite sides.
func planesOpposite(a, b mapfile.Plane) bool {
	return v3Dot(a.Normal, b.Normal) < -(1 - 1e-4) && math.Abs(a.Dist+b.Dist) < planeMatchEpsilon
}

// removeRedundantPlanes drops sides that contribute no area to the brush.
//
// Where in C: RemoveRedundantPlanes in ericw-tools common/decompile.cc — a
// plane whose base winding vanishes when clipped by all other planes is not
// part of the brush boundary.
func removeRedundantPlanes(b *Brush) {
	rebuildWindings(b)
	kept := b.Sides[:0]
	for _, s := range b.Sides {
		if s.Winding != nil && len(s.Winding.Points) >= 3 {
			kept = append(kept, s)
		}
	}
	b.Sides = kept
	// recompute survivors' windings without the dropped planes so areas are exact
	rebuildWindings(b)
}

// planeSetCount returns the number of distinct oriented planes across brushes
// (the PlanesUsed stat).
func planeSetCount(brushes []*Brush) int {
	var planes []mapfile.Plane
	for _, b := range brushes {
		for _, s := range b.Sides {
			seen := false
			for _, p := range planes {
				if planesMatch(s.Plane, p) {
					seen = true
					break
				}
			}
			if !seen {
				planes = append(planes, s.Plane)
			}
		}
	}
	return len(planes)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/planes.go internal/bspdec/planes_test.go
git commit -m "feat(bspdec): add redundant-plane removal (ericw RemoveRedundantPlanes port)"
```

---

## Task 6: Texturing — face windings, overlap matching, fallbacks

Bead: `ironwail-go-xxy.2`. Spec §5 steps 6+8: each brush side gets the texinfo of the tree face on the same plane with the largest overlap; unmatched sides get contents textures or the fallback policy.

**Files:**
- Create: `internal/bspdec/texture.go`, `internal/bspdec/texture_test.go`
- Modify: `internal/bspdec/treewalk.go` (add `texNames []string` + `faces []texturedFace` fields to the `decompiler` struct and initialize `texNames: textureNames(tree)` in `newDecompiler`)

**Interfaces:**
- Consumes: `bsp.TreeFace{PlaneNum, Side, FirstEdge, NumEdges, Texinfo int32}`, `bsp.TreeEdge{V [2]uint32}`, `Tree.Surfedges []int32`, `Tree.Vertexes []bsp.DVertex`, `Tree.Texinfo []bsp.Texinfo{Vecs [2][4]float32; Miptex, Flags int32}`, `Tree.TextureData []byte`; Brush/Side from Task 4.
- Produces:
  - `type texturedFace struct { plane mapfile.Plane; winding *Winding; texName string; vecs [2][4]float64 }`
  - `func textureNames(tree *bsp.Tree) []string` (miptex lump decode)
  - `(d *decompiler) facePlane(f *bsp.TreeFace) mapfile.Plane` — negated when `f.Side != 0`
  - `(d *decompiler) faceWinding(fi int) *Winding`
  - `(d *decompiler) collectFaces() []texturedFace` — lazily cached on `d.faces`
  - `func clipToBrush(w *Winding, b *Brush) *Winding` — w clipped inside every brush plane
  - `func overlapArea(fw *Winding, b *Brush) float64`
  - `(d *decompiler) textureBrush(b *Brush)` / `textureBrushes(brushes []*Brush)`
  - `func contentsTexture(c int32) string` — `"*water"/"*slime"/"*lava"/"sky"/"clip"` or `""`
  - `(d *decompiler) fallbackTexture(b *Brush, s *Side)`
  - `func vecsEqual(a, b [2][4]float64) bool`

- [ ] **Step 1: Write the failing tests**

Create `internal/bspdec/texture_test.go`:

```go
package bspdec

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

func TestTextureNamesFromFixture(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	names := textureNames(tree)
	want := map[string]bool{"wwall": true, "ffloor": true, "cceil": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("missing texture names: %v (got %v)", want, names)
	}
}

func TestTextureBrushesAssignsFaces(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	for _, b := range brushes {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(brushes)
	seen := map[string]bool{}
	for _, b := range brushes {
		for _, s := range b.Sides {
			if s.TexName == "" {
				t.Fatal("side with empty texture after texturing+fallback")
			}
			seen[s.TexName] = true
		}
	}
	// floor and wall faces must be recovered from the face lump
	if !seen["ffloor"] || !seen["wwall"] {
		t.Fatalf("expected ffloor and wwall among side textures, got %v", seen)
	}
}

func TestContentsTexture(t *testing.T) {
	cases := map[int32]string{
		bsp.ContentsWater: "*water",
		bsp.ContentsSlime: "*slime",
		bsp.ContentsLava:  "*lava",
		bsp.ContentsSky:   "sky",
		bsp.ContentsClip:  "clip",
		bsp.ContentsSolid: "",
		bsp.ContentsEmpty: "",
	}
	for c, want := range cases {
		if got := contentsTexture(c); got != want {
			t.Fatalf("contentsTexture(%d) = %q, want %q", c, got, want)
		}
	}
}

func TestFallbackNearestUsesLargestMatchedSide(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	// no tree faces near this brush: every side falls back, and with no
	// matched side in the brush the policy warns and uses "clip"
	d.textureBrush(b)
	for _, s := range b.Sides {
		if s.TexName != "clip" {
			t.Fatalf("fallback texture = %q, want clip", s.TexName)
		}
	}
	if d.warnings == 0 {
		t.Fatal("expected a fallback warning")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestTexture|TestContents|TestFallback' -count=1`
Expected: FAIL — undefined `textureNames`, `textureBrush(es)`, `contentsTexture`.

- [ ] **Step 3: Implement `texture.go`**

In `treewalk.go`, add fields `texNames []string` and `faces []texturedFace` to the `decompiler` struct and change `newDecompiler` to `return &decompiler{tree: tree, opts: opts, texNames: textureNames(tree)}`; then create `internal/bspdec/texture.go`:

```go
package bspdec

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// texturedFace is a tree face with its plane and winding precomputed.
type texturedFace struct {
	plane   mapfile.Plane
	winding *Winding
	texName string
	vecs    [2][4]float64
}

// textureNames decodes the miptex lump into a name table.
//
// Where in C: dmiptexlump_t/miptex_t in WinQuake bspfile.h.
func textureNames(tree *bsp.Tree) []string {
	data := tree.TextureData
	if len(data) < 4 {
		return nil
	}
	n := int(int32(binary.LittleEndian.Uint32(data)))
	if n < 0 || 4+4*n > len(data) {
		return nil
	}
	names := make([]string, n)
	for i := 0; i < n; i++ {
		ofs := int(int32(binary.LittleEndian.Uint32(data[4+i*4:])))
		if ofs <= 0 || ofs+16 > len(data) {
			continue // unassigned texture slot
		}
		names[i] = string(bytes.TrimRight(data[ofs:ofs+16], "\x00"))
	}
	return names
}

func (d *decompiler) texName(mi int32) string {
	if mi < 0 || int(mi) >= len(d.texNames) {
		return ""
	}
	return d.texNames[mi]
}

// texinfoVecs copies the float32 s/t mapping to float64 for map emission.
func texinfoVecs(ti *bsp.Texinfo) [2][4]float64 {
	var v [2][4]float64
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			v[i][j] = float64(ti.Vecs[i][j])
		}
	}
	return v
}

func vecsEqual(a, b [2][4]float64) bool {
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			if math.Abs(a[i][j]-b[i][j]) > 1e-4 {
				return false
			}
		}
	}
	return true
}

// facePlane returns the face's oriented plane (negated for side 1).
func (d *decompiler) facePlane(f *bsp.TreeFace) mapfile.Plane {
	p := d.plane(f.PlaneNum)
	if f.Side != 0 {
		p = negatePlane(p)
	}
	return p
}

// faceWinding rebuilds the polygon of tree face fi from the surfedge walk.
//
// Where in C: face winding reconstruction in ericw-tools common/decompile.cc
// (surfedges -> edges -> vertexes; negative surfedge = reversed edge).
func (d *decompiler) faceWinding(fi int) *Winding {
	f := d.tree.Faces[fi]
	w := &Winding{}
	for se := f.FirstEdge; se < f.FirstEdge+f.NumEdges; se++ {
		s := d.tree.Surfedges[se]
		var vi uint32
		if s >= 0 {
			vi = d.tree.Edges[s].V[0]
		} else {
			vi = d.tree.Edges[-s].V[1]
		}
		p := d.tree.Vertexes[vi].Point
		w.Points = append(w.Points, mapfile.Vec3{float64(p.X), float64(p.Y), float64(p.Z)})
	}
	return w
}

// collectFaces precomputes every tree face's plane and winding once.
func (d *decompiler) collectFaces() []texturedFace {
	if d.faces != nil {
		return d.faces
	}
	d.faces = make([]texturedFace, 0, len(d.tree.Faces))
	for fi := range d.tree.Faces {
		f := &d.tree.Faces[fi]
		w := d.faceWinding(fi)
		if len(w.Points) < 3 {
			continue
		}
		if f.Texinfo < 0 || int(f.Texinfo) >= len(d.tree.Texinfo) {
			continue
		}
		ti := &d.tree.Texinfo[f.Texinfo]
		d.faces = append(d.faces, texturedFace{
			plane:   d.facePlane(f),
			winding: w,
			texName: d.texName(ti.Miptex),
			vecs:    texinfoVecs(ti),
		})
	}
	return d.faces
}

// clipToBrush returns the part of w lying inside brush b, or nil.
func clipToBrush(w *Winding, b *Brush) *Winding {
	for _, s := range b.Sides {
		w = w.Clip(negatePlane(s.Plane))
		if w == nil {
			return nil
		}
	}
	return w
}

// overlapArea returns the area of fw lying inside brush b. Both caller and
// face are known near-coplanar with one of b's sides, so the clipped polygon
// is the overlap region.
func overlapArea(fw *Winding, b *Brush) float64 {
	w := clipToBrush(fw, b)
	if w == nil {
		return 0
	}
	return w.Area()
}

// textureBrushes assigns every side of every brush a texture.
//
// Where in C: face-overlap texturing in ericw-tools common/decompile.cc.
func (d *decompiler) textureBrushes(brushes []*Brush) {
	for _, b := range brushes {
		d.textureBrush(b)
	}
}

// textureBrush assigns each side the texinfo of the tree face on the same
// plane with the largest overlap area; unmatched sides fall back.
func (d *decompiler) textureBrush(b *Brush) {
	faces := d.collectFaces()
	for _, s := range b.Sides {
		best := -1
		bestArea := 0.0
		for i := range faces {
			if !planesMatch(s.Plane, faces[i].plane) {
				continue
			}
			if a := overlapArea(faces[i].winding, b); a > bestArea {
				bestArea = a
				best = i
			}
		}
		if best >= 0 {
			s.TexName = faces[best].texName
			s.Vecs = faces[best].vecs
			continue
		}
		d.fallbackTexture(b, s)
	}
}

// contentsTexture maps leaf contents to its convention texture ("" if none).
//
// Where in C: Q1_FixContentsTextures in bspc map_q1.c.
func contentsTexture(c int32) string {
	switch c {
	case bsp.ContentsWater:
		return "*water"
	case bsp.ContentsSlime:
		return "*slime"
	case bsp.ContentsLava:
		return "*lava"
	case bsp.ContentsSky:
		return "sky"
	case bsp.ContentsClip:
		return "clip"
	default:
		return ""
	}
}

// fallbackTexture textures a side that matched no tree face: the brush's
// contents texture first (liquid/sky interiors), then the policy.
func (d *decompiler) fallbackTexture(b *Brush, s *Side) {
	if t := contentsTexture(b.Contents); t != "" {
		s.TexName = t
		return
	}
	switch d.opts.TextureFallback {
	case "skip":
		s.TexName = "skip" // ericw qbsp drops SKIP surfaces at compile
	case "trigger":
		s.TexName = "trigger"
	default: // "nearest"
		best := ""
		bestArea := -1.0
		for _, o := range b.Sides {
			if o.TexName == "" || o.Winding == nil {
				continue
			}
			if a := o.Winding.Area(); a > bestArea {
				bestArea = a
				best = o.TexName
			}
		}
		if best == "" {
			d.warnf("brush has no matched side to copy a texture from; using clip")
			best = "clip"
		}
		s.TexName = best
	}
}
```

Note for the implementer: `overlapArea` needs the brush, but the match loop only knows the side — that is correct: the face is clipped by the *whole brush*, which bounds the overlap to the cell. `collectFaces` on every `textureBrush` call is cached via `d.faces`, so the per-brush cost is one plane-match scan over faces.

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS. If `TestTextureBrushesAssignsFaces` finds no `ffloor`, check surfedge sign handling in `faceWinding` first (reversed edges pick `V[1]`), then `facePlane` side negation.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/texture.go internal/bspdec/texture_test.go internal/bspdec/treewalk.go
git commit -m "feat(bspdec): add face-overlap texturing with contents and fallback policies"
```

---

## Task 7: Texture-boundary splitting

Bead: `ironwail-go-xxy.2`. Spec §5 step 5: when one reconstructed side overlaps faces with different texinfo, split the brush along the boundary. **Documented simplification vs ericw**: only cleanly-separable planar splits are performed (a single edge plane of one texinfo's region that no other texinfo's region straddles); unseparable mixes keep the dominant texture and count a warning, matching bsputil's best-match behavior. Recorded as a known M0 limitation that ML Route A (M2) is designed to close.

**Files:**
- Create: `internal/bspdec/split.go`, `internal/bspdec/split_test.go`

**Interfaces:**
- Consumes: `collectFaces`, `clipToBrush`, `textureBrush`, `clipBrush` (Tasks 4/6), `planesMatch`, `vecsEqual`.
- Produces: `(d *decompiler) splitDifferentTextures(b *Brush) []*Brush`; `func inwardEdgePlane(a, b mapfile.Vec3, ref mapfile.Plane, refPt mapfile.Vec3) mapfile.Plane`.

- [ ] **Step 1: Write the failing test**

Create `internal/bspdec/split_test.go`:

```go
package bspdec

import (
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// buildMiptexLump builds a minimal miptex lump: count, offset table, then
// name records (name[16] + width + height; bspdec reads only the name).
func buildMiptexLump(names ...string) []byte {
	out := make([]byte, 4+4*len(names))
	binary.LittleEndian.PutUint32(out[0:], uint32(len(names)))
	for i, n := range names {
		binary.LittleEndian.PutUint32(out[4+i*4:], uint32(len(out)))
		rec := make([]byte, 24)
		copy(rec[:16], n)
		out = append(out, rec...)
	}
	return out
}

// twoTextureTopTree builds a minimal tree whose only faces are two coplanar
// quads on z=64: "left" covering x in [0,32], "right" covering x in [32,64].
func twoTextureTopTree() *bsp.Tree {
	mkQuad := func(x0, x1 float32) (verts []bsp.DVertex) {
		for _, p := range [4][3]float32{{x0, 0, 64}, {x1, 0, 64}, {x1, 64, 64}, {x0, 64, 64}} {
			verts = append(verts, bsp.DVertex{Point: types.Vec3{X: p[0], Y: p[1], Z: p[2]}})
		}
		return verts
	}
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 64, Type: bsp.PlaneZ}},
		TextureData: buildMiptexLump("left", "right"),
		Texinfo: []bsp.Texinfo{
			{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, -1, 0, 0}}, Miptex: 0},
			{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, -1, 0, 0}}, Miptex: 1},
		},
	}
	for _, q := range [2][]bsp.DVertex{mkQuad(0, 32), mkQuad(32, 64)} {
		base := uint32(len(tree.Vertexes))
		tree.Vertexes = append(tree.Vertexes, q...)
		edgeBase := len(tree.Edges)
		for e := 0; e < 4; e++ {
			tree.Edges = append(tree.Edges, bsp.TreeEdge{V: [2]uint32{base + uint32(e), base + uint32((e+1)%4)}})
			tree.Surfedges = append(tree.Surfedges, int32(edgeBase+e))
		}
		tree.Faces = append(tree.Faces, bsp.TreeFace{
			PlaneNum: 0, Side: 0,
			FirstEdge: int32(edgeBase), NumEdges: 4,
			Texinfo: int32(len(tree.Faces)),
		})
	}
	return tree
}

func TestSplitDifferentTextures(t *testing.T) {
	tree := twoTextureTopTree()
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	d.textureBrush(b)
	parts := d.splitDifferentTextures(b)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	top := mapfile.Plane{Normal: mapfile.Vec3{0, 0, 1}, Dist: 64}
	tops := map[string]bool{}
	for _, p := range parts {
		for _, s := range p.Sides {
			if planesMatch(s.Plane, top) {
				tops[s.TexName] = true
			}
		}
	}
	if !tops["left"] || !tops["right"] {
		t.Fatalf("top textures after split = %v, want left+right", tops)
	}
}

func TestSplitKeepsUniformSideAlone(t *testing.T) {
	tree := twoTextureTopTree()
	// drop the "right" face: only one texinfo remains
	tree.Faces = tree.Faces[:1]
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{64, 64, 64})
	d.textureBrush(b)
	if parts := d.splitDifferentTextures(b); len(parts) != 1 {
		t.Fatalf("parts = %d, want 1 (no split)", len(parts))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestSplit' -count=1`
Expected: FAIL — undefined `splitDifferentTextures`.

- [ ] **Step 3: Implement `split.go`**

Create `internal/bspdec/split.go`:

```go
package bspdec

import (
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// inwardEdgePlane returns the plane through edge a->b, perpendicular to ref,
// oriented so refPt (the region) lies on its back (inside) side.
func inwardEdgePlane(a, b mapfile.Vec3, ref mapfile.Plane, refPt mapfile.Vec3) mapfile.Plane {
	n := v3Normalize(v3Cross(ref.Normal, v3Sub(b, a)))
	p := mapfile.Plane{Normal: n, Dist: v3Dot(n, a)}
	if v3Dot(p.Normal, refPt)-p.Dist > 0 {
		p = negatePlane(p)
	}
	return p
}

// splitDifferentTextures splits brushes whose side overlaps tree faces with
// more than one texinfo, so each output brush carries one texture per side.
//
// Where in C: SplitDifferentTexturedPartsOfBrush in ericw-tools
// common/decompile.cc. Simplified: only cleanly-separable planar splits are
// performed; unseparable mixes keep the dominant (largest-overlap) texture
// and warn, matching bsputil's best-match behavior.
func (d *decompiler) splitDifferentTextures(b *Brush) []*Brush {
	out := []*Brush{b}
	for i := 0; i < len(out); i++ {
		if parts := d.splitOneSide(out[i]); parts != nil {
			out = append(out[:i], append(parts, out[i+1:]...)...)
			i-- // re-check the inserted pieces for further splits
		}
	}
	return out
}

// splitOneSide performs one texture-boundary split, or returns nil when no
// side of b needs (or can cleanly make) one.
func (d *decompiler) splitOneSide(b *Brush) []*Brush {
	faces := d.collectFaces()
	for _, s := range b.Sides {
		var matches []texturedFace
		for _, f := range faces {
			if !planesMatch(s.Plane, f.plane) {
				continue
			}
			w := clipToBrush(f.winding, b)
			if w == nil {
				continue
			}
			matches = append(matches, texturedFace{plane: f.plane, winding: w, texName: f.texName, vecs: f.vecs})
		}
		distinct := map[string]bool{}
		for _, m := range matches {
			distinct[m.texName] = true
		}
		if len(distinct) < 2 {
			continue
		}
		// s.TexName is the dominant texinfo (assigned by textureBrush).
		// Try every edge of every minority region as a separating plane.
		for _, f := range matches {
			if f.texName == s.TexName {
				continue
			}
			for ei := range f.winding.Points {
				a := f.winding.Points[ei]
				bb := f.winding.Points[(ei+1)%len(f.winding.Points)]
				ep := inwardEdgePlane(a, bb, s.Plane, f.winding.Centroid())
				clean := true
				for _, o := range matches {
					if o.texName == f.texName {
						continue
					}
					// any other-texinfo region with area inside ep straddles
					if o.winding.Clip(negatePlane(ep)) != nil {
						clean = false
						break
					}
				}
				if !clean {
					continue
				}
				region := clipBrush(b, ep)
				rest := clipBrush(b, negatePlane(ep))
				if region == nil || rest == nil {
					continue // split plane coincides with a brush boundary
				}
				d.textureBrush(region)
				d.textureBrush(rest)
				return []*Brush{region, rest}
			}
		}
		d.warnf("side has %d mixed texinfos with no clean split plane; keeping dominant %q", len(distinct), s.TexName)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS. If `TestSplitDifferentTextures` returns 1 part, the separator search is failing — debug `inwardEdgePlane` orientation first (the region centroid must be on the back of the returned plane).

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/split.go internal/bspdec/split_test.go
git commit -m "feat(bspdec): split brushes at coplanar texture boundaries"
```

---

## Task 8: Entity passthrough, bmodel attach, origin keys

Bead: `ironwail-go-xxy.2`. Spec §6: entity lump preserved verbatim, bmodels get their brushes and an `origin` key recovered from `dmodel.Origin`. BSP bmodel geometry is stored at its authored position, so no translation is needed — only the key.

**Files:**
- Create: `internal/bspdec/entities.go`, `internal/bspdec/entities_test.go`

**Interfaces:**
- Consumes: `Tree.Entities []byte`, `Tree.Models []bsp.DModel` (`Origin types.Vec3`), `Brush` (Task 4).
- Produces:
  - `func parseEntities(tree *bsp.Tree) (*mapfile.Map, error)`
  - `func setEpair(ent *mapfile.Entity, key, value string)`
  - `func setBmodelOrigin(ent *mapfile.Entity, m *bsp.DModel)`
  - `func toMapBrush(b *Brush) mapfile.MapBrush`
  - `func pick3Points(w *Winding) [3]mapfile.Vec3` — max-area triple
  - `func attachBrushes(ents *mapfile.Map, perModel [][]*Brush, tree *bsp.Tree) *mapfile.Map`

- [ ] **Step 1: Write the failing test**

Create `internal/bspdec/entities_test.go`:

```go
package bspdec

import (
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// roomWithDoorMap adds a func_door bmodel inside the room.
func roomWithDoorMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(-64, -64, -64, 0, 320, 256, "wwall") +
		slabBox(256, -64, -64, 320, 320, 256, "wwall") +
		slabBox(0, -64, -64, 256, 0, 256, "wwall") +
		slabBox(0, 256, -64, 256, 320, 256, "wwall") +
		slabBox(0, 0, -64, 256, 256, 0, "ffloor") +
		slabBox(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n" +
		"{\n\"classname\" \"func_door\"\n\"targetname\" \"d1\"\n\"sounds\" \"2\"\n" +
		slabBox(96, 96, 16, 160, 160, 112, "ddoor") +
		"}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 64\"\n}\n"
}

func TestAttachBrushesPreservesEntities(t *testing.T) {
	tree, data := compileFixture(t, roomWithDoorMap())
	counts, err := qbsp.ReadBSPXBrushList(data)
	if err != nil {
		t.Fatalf("ReadBSPXBrushList: %v", err)
	}
	if len(counts) != 2 || counts[0] != 6 || counts[1] != 1 {
		t.Fatalf("BRUSHLIST oracle = %v, want [6 1]", counts)
	}
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	perModel := make([][]*Brush, len(tree.Models))
	for mi := range tree.Models {
		brushes, err := d.decompileModel(mi)
		if err != nil {
			t.Fatalf("decompileModel(%d): %v", mi, err)
		}
		for _, b := range brushes {
			removeRedundantPlanes(b)
		}
		d.textureBrushes(brushes)
		perModel[mi] = brushes
	}
	ents, err := parseEntities(tree)
	if err != nil {
		t.Fatalf("parseEntities: %v", err)
	}
	out := attachBrushes(ents, perModel, tree)

	// worldspawn got model-0 brushes
	ws := out.Entities[0]
	if v, _ := ws.Value("classname"); v != "worldspawn" {
		t.Fatalf("entity 0 classname = %q", v)
	}
	if len(ws.Brushes) < counts[0] {
		t.Fatalf("worldspawn brushes = %d, want >= %d", len(ws.Brushes), counts[0])
	}

	// the door entity keeps its epairs verbatim and gains model-1 brushes
	var door *mapfile.Entity
	for i := range out.Entities {
		if v, _ := out.Entities[i].Value("classname"); v == "func_door" {
			door = &out.Entities[i]
		}
	}
	if door == nil {
		t.Fatal("func_door entity lost")
	}
	if v, ok := door.Value("targetname"); !ok || v != "d1" {
		t.Fatalf("door targetname = %q, %v", v, ok)
	}
	if v, ok := door.Value("sounds"); !ok || v != "2" {
		t.Fatalf("door sounds = %q, %v (epairs must be verbatim)", v, ok)
	}
	if v, ok := door.Value("model"); !ok || v != "*1" {
		t.Fatalf("door model = %q, %v, want *1 (kept from the lump)", v, ok)
	}
	if len(door.Brushes) < counts[1] {
		t.Fatalf("door brushes = %d, want >= %d", len(door.Brushes), counts[1])
	}
	// door geometry stays at its authored position (96..160)
	if len(perModel[1]) == 0 {
		t.Fatal("model 1 produced no brushes")
	}
	found := false
	for _, s := range perModel[1][0].Sides {
		if s.Plane.Normal == (mapfile.Vec3{0, 0, 1}) {
			found = true
		}
	}
	if !found {
		t.Fatal("door brush has no +z side")
	}
	for _, f := range door.Brushes[0].Faces {
		for _, p := range f.Points {
			if p[0] < 96-72 || p[0] > 160+72 {
				t.Fatalf("door point %v far from authored bounds", p)
			}
		}
	}
}

func TestPick3PointsNonCollinear(t *testing.T) {
	w := &Winding{Points: []mapfile.Vec3{{0, 0, 0}, {32, 0, 0}, {64, 0, 0}, {64, 64, 0}, {0, 64, 0}}}
	pts := pick3Points(w)
	p, length := mapfile.PlaneFromPoints(pts[0], pts[1], pts[2])
	if length < 0.01 {
		t.Fatalf("degenerate triple %v", pts)
	}
	if p.Normal[2] < 0 {
		_ = p // orientation is the writer's concern, not the triple's
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestAttachBrushes|TestPick3' -count=1`
Expected: FAIL — undefined `parseEntities`, `attachBrushes`, `pick3Points`, `toMapBrush`.

- [ ] **Step 3: Implement `entities.go`**

Create `internal/bspdec/entities.go`:

```go
package bspdec

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// parseEntities decodes the BSP entity lump (the same "{ }" block grammar as
// .map files, minus brushes).
func parseEntities(tree *bsp.Tree) (*mapfile.Map, error) {
	m, err := mapfile.Parse(bytes.NewReader(tree.Entities))
	if err != nil {
		return nil, fmt.Errorf("parsing entity lump: %w", err)
	}
	return m, nil
}

// setEpair replaces the last value stored under key, or appends it.
func setEpair(ent *mapfile.Entity, key, value string) {
	for i := range ent.Epairs {
		if ent.Epairs[i].Key == key {
			ent.Epairs[i].Value = value
			return
		}
	}
	ent.Epairs = append(ent.Epairs, mapfile.Epair{Key: key, Value: value})
}

// setBmodelOrigin copies dmodel.Origin into the entity's origin key. BSP
// bmodel geometry is stored at its authored position, so the key alone
// reproduces the compile-time placement.
//
// Where in C: model origin recovery in bspc map_q1.c / ericw decompile.cc.
func setBmodelOrigin(ent *mapfile.Entity, m *bsp.DModel) {
	if m.Origin.X == 0 && m.Origin.Y == 0 && m.Origin.Z == 0 {
		return
	}
	setEpair(ent, "origin", fmt.Sprintf("%g %g %g", m.Origin.X, m.Origin.Y, m.Origin.Z))
}

// attachBrushes wires per-model decompiled brushes into the entity map:
// model 0 brushes go to worldspawn (entity 0), model N brushes to the entity
// whose "model" key is "*N". All other epairs pass through verbatim.
//
// Where in C: entity handling in bspc map_q1.c (Q1_LoadBSPFile entity pass).
func attachBrushes(ents *mapfile.Map, perModel [][]*Brush, tree *bsp.Tree) *mapfile.Map {
	out := &mapfile.Map{}
	for ei := range ents.Entities {
		ent := ents.Entities[ei] // shallow copy; Brushes replaced below
		ent.Brushes = nil
		model := -1
		if ei == 0 {
			model = 0
		} else if v, ok := ent.Value("model"); ok && strings.HasPrefix(v, "*") {
			if n, err := strconv.Atoi(v[1:]); err == nil {
				model = n
			}
		}
		if model >= 0 && model < len(perModel) {
			for _, b := range perModel[model] {
				ent.Brushes = append(ent.Brushes, toMapBrush(b))
			}
			if model > 0 && model < len(tree.Models) {
				setBmodelOrigin(&ent, &tree.Models[model])
			}
		}
		out.Entities = append(out.Entities, ent)
	}
	return out
}

// toMapBrush converts a decompiled brush to a .map brush: each side becomes a
// face defined by three non-collinear winding points plus Valve 220 vecs.
func toMapBrush(b *Brush) mapfile.MapBrush {
	var mb mapfile.MapBrush
	for _, s := range b.Sides {
		if s.Winding == nil || len(s.Winding.Points) < 3 {
			continue
		}
		mb.Faces = append(mb.Faces, mapfile.MapFace{
			Points:  pick3Points(s.Winding),
			TexName: s.TexName,
			Vecs:    s.Vecs,
		})
	}
	return mb
}

// pick3Points returns the maximum-area triple of winding points, which keeps
// the re-derived plane as numerically accurate as possible.
func pick3Points(w *Winding) [3]mapfile.Vec3 {
	p0 := w.Points[0]
	p1, p2 := w.Points[1], w.Points[1]
	best := -1.0
	for _, p := range w.Points[1:] {
		if d := v3Len(v3Sub(p, p0)); d > best {
			best = d
			p1 = p
		}
	}
	best = -1.0
	for _, p := range w.Points {
		if a := v3Len(v3Cross(v3Sub(p1, p0), v3Sub(p, p0))); a > best {
			best = a
			p2 = p
		}
	}
	return [3]mapfile.Vec3{p0, p1, p2}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS. If the door entity has no `model` key, check what qbsp writes into the entity lump for brush entities — the decompiler trusts the lump, so a missing `*1` there is a fixture/qbsp question, not an `attachBrushes` bug.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/entities.go internal/bspdec/entities_test.go
git commit -m "feat(bspdec): attach decompiled brushes to entities with verbatim epairs and origin keys"
```

---

## Task 9: Convex merging (`--merge-convex`)

Bead: `ironwail-go-xxy.2`. bspc `-lessbrushes` analogue: merge same-contents cell pairs whose union is convex. The texture guard matters: merging must never rejoin what Task 7 split apart.

**Files:**
- Create: `internal/bspdec/merge.go`, `internal/bspdec/merge_test.go`

**Interfaces:**
- Consumes: `Brush`, `planesOpposite`, `rebuildWindings`, `negatePlane`, `vecsEqual`.
- Produces: `mergeConvex(brushes []*Brush) []*Brush`; `tryMerge(a, b *Brush) *Brush`; `brushCuts(a, b *Brush) bool`.

- [ ] **Step 1: Write the failing test**

Create `internal/bspdec/merge_test.go`:

```go
package bspdec

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

func newTexturedBox(mins, maxs mapfile.Vec3, tex string, contents int32) *Brush {
	b := boxBrush(mins, maxs)
	b.Contents = contents
	for _, s := range b.Sides {
		s.TexName = tex
	}
	return b
}

func TestMergeConvexJoinsAdjacentBoxes(t *testing.T) {
	a := newTexturedBox(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{32, 64, 64}, "brick", bsp.ContentsSolid)
	b := newTexturedBox(mapfile.Vec3{32, 0, 0}, mapfile.Vec3{64, 64, 64}, "brick", bsp.ContentsSolid)
	out := mergeConvex([]*Brush{a, b})
	if len(out) != 1 {
		t.Fatalf("brushes = %d, want 1", len(out))
	}
	if len(out[0].Sides) != 6 {
		t.Fatalf("merged sides = %d, want 6", len(out[0].Sides))
	}
	for _, s := range out[0].Sides {
		if s.Winding == nil {
			t.Fatal("merged brush has nil winding")
		}
	}
}

func TestMergeConvexRejectsDifferentContents(t *testing.T) {
	a := newTexturedBox(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{32, 64, 64}, "brick", bsp.ContentsSolid)
	b := newTexturedBox(mapfile.Vec3{32, 0, 0}, mapfile.Vec3{64, 64, 64}, "*water", bsp.ContentsWater)
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (contents differ)", len(out))
	}
}

func TestMergeConvexRejectsTextureBoundary(t *testing.T) {
	// same contents, but the coplanar outer sides carry different textures:
	// merging would lose the boundary Task 7 created
	a := newTexturedBox(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{32, 64, 64}, "brick", bsp.ContentsSolid)
	b := newTexturedBox(mapfile.Vec3{32, 0, 0}, mapfile.Vec3{64, 64, 64}, "brick", bsp.ContentsSolid)
	for _, s := range b.Sides {
		if s.Plane.Normal == (mapfile.Vec3{0, 0, 1}) {
			s.TexName = "stone"
		}
	}
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (texture boundary)", len(out))
	}
}

func TestMergeConvexRejectsNonConvexUnion(t *testing.T) {
	// L shape: boxes touching only partially (b taller than a) — the union
	// is non-convex because a's top plane cuts b
	a := newTexturedBox(mapfile.Vec3{0, 0, 0}, mapfile.Vec3{32, 64, 32}, "brick", bsp.ContentsSolid)
	b := newTexturedBox(mapfile.Vec3{32, 0, 0}, mapfile.Vec3{64, 64, 64}, "brick", bsp.ContentsSolid)
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (non-convex union)", len(out))
	}
}

func TestMergeConvexRunsOnDecompiledRoom(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	for _, b := range brushes {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(brushes)
	before := len(brushes)
	merged := mergeConvex(brushes)
	if len(merged) >= before {
		t.Fatalf("merge did not reduce brush count: before=%d after=%d", before, len(merged))
	}
	for i, b := range merged {
		if len(b.Sides) < 4 {
			t.Fatalf("merged brush %d has %d sides", i, len(b.Sides))
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestMerge' -count=1`
Expected: FAIL — undefined `mergeConvex`.

- [ ] **Step 3: Implement `merge.go`**

Create `internal/bspdec/merge.go`:

```go
package bspdec

import (
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// mergeConvex greedily merges same-contents brush pairs whose union is
// convex, until no more pairs merge. O(n^2) per pass is fine for M0 map
// sizes; revisit only if corpus timings say so.
//
// Where in C: bspc -lessbrushes merging in map_q1.c (TryMerge analogue).
func mergeConvex(brushes []*Brush) []*Brush {
	used := make([]bool, len(brushes))
	var out []*Brush
	for i := range brushes {
		if used[i] {
			continue
		}
		cur := brushes[i]
		used[i] = true
		for merged := true; merged; {
			merged = false
			for j := range brushes {
				if used[j] {
					continue
				}
				if m := tryMerge(cur, brushes[j]); m != nil {
					cur = m
					used[j] = true
					merged = true
				}
			}
		}
		out = append(out, cur)
	}
	return out
}

// tryMerge returns the convex union of a and b, or nil. Preconditions for a
// valid merge: exactly one shared coplanar side pair; neither brush's planes
// cut the other's windings (convexity); and no texture conflict on any
// coplanar side pair (a merge would collapse Task 7's boundary splits).
func tryMerge(a, b *Brush) *Brush {
	if a.Contents != b.Contents {
		return nil
	}
	shared := 0
	for _, sa := range a.Sides {
		for _, sb := range b.Sides {
			if planesOpposite(sa.Plane, sb.Plane) {
				shared++
			}
			// texture guard: same-direction coplanar sides must agree
			if planesMatch(sa.Plane, sb.Plane) && (sa.TexName != sb.TexName || !vecsEqual(sa.Vecs, sb.Vecs)) {
				return nil
			}
		}
	}
	if shared != 1 {
		return nil
	}
	if brushCuts(a, b) || brushCuts(b, a) {
		return nil
	}
	cand := &Brush{Contents: a.Contents}
	for _, s := range a.Sides {
		cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
	}
	for _, s := range b.Sides {
		dupe := false
		for _, o := range cand.Sides {
			if planesMatch(s.Plane, o.Plane) || planesOpposite(s.Plane, o.Plane) {
				dupe = true // shared pair + coplanar dupes collapse
				break
			}
		}
		if !dupe {
			cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
		}
	}
	rebuildWindings(cand)
	kept := cand.Sides[:0]
	for _, s := range cand.Sides {
		if s.Winding != nil && len(s.Winding.Points) >= 3 {
			kept = append(kept, s)
		}
	}
	cand.Sides = kept
	if len(cand.Sides) < 4 {
		return nil
	}
	rebuildWindings(cand)
	return cand
}

// brushCuts reports whether any plane of a removes area from any winding of
// b (the union would be non-convex). Coplanar contact is not a cut: on-plane
// points survive clipping.
func brushCuts(a, b *Brush) bool {
	for _, s := range a.Sides {
		for _, os := range b.Sides {
			if os.Winding == nil {
				continue
			}
			w := os.Winding.Clip(negatePlane(s.Plane))
			if w == nil {
				return true
			}
			if len(w.Points) != len(os.Winding.Points) {
				return true // partially clipped: a's plane slices b's side
			}
		}
	}
	return false
}
```

Wait — one subtlety the implementer must check in `tryMerge`: the shared side pair is dropped only implicitly (the `planesOpposite` dupe check drops b's copy of the shared plane; a's copy stays in `cand`). That is wrong: the merged box must lose BOTH shared sides. Fix: drop a's shared side too — collect a-sides excluding the one opposite-matching a b-side:

```go
	for _, s := range a.Sides {
		sharedSide := false
		for _, sb := range b.Sides {
			if planesOpposite(s.Plane, sb.Plane) {
				sharedSide = true
				break
			}
		}
		if !sharedSide {
			cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
		}
	}
```

Use this corrected loop in the implementation (the b-side loop above it already skips both same-plane dupes and the opposite shared side). Verify with the 2-box test: merged box must have exactly 6 sides.

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS, including `TestMergeConvexJoinsAdjacentBoxes` at exactly 6 sides (this catches the shared-side-dropping subtlety above).

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/merge.go internal/bspdec/merge_test.go
git commit -m "feat(bspdec): merge coplanar-adjacent same-contents convex cells"
```

---

## Task 10: Hull decompile (`--decompile-hull N`)

Bead: `ironwail-go-xxy.2`. Spec §5 step 9: walk the clipnode tree of hull N, then reverse hull expansion by subtracting the hull AABB's Minkowski offset, and drop bevel-only planes (planes that match no render-hull lump plane). Hull brushes emit as all-`clip`.

**Files:**
- Create: `internal/bspdec/hull.go`, `internal/bspdec/hull_test.go`

**Interfaces:**
- Consumes: `bsp.Load(r io.ReadSeeker) (*bsp.File, error)` (`File.Clipnodes any` = `[]bsp.DSClipNode` or `[]bsp.DLClipNode`, `Children [2]int32` with negative = contents, already normalized by the loader); `bsp.Tree.Models[i].HeadNode[hull]`.
- Produces:
  - `type clipnode struct { PlaneNum int32; Children [2]int32 }`
  - `func clipnodesOf(f *bsp.File) ([]clipnode, error)`
  - `func unexpandPlane(p mapfile.Plane, mins, maxs mapfile.Vec3) mapfile.Plane`
  - `(d *decompiler) decompileHull(modelIdx, hull int, nodes []clipnode) []*Brush`
  - `var hullMins, hullMaxs [4]mapfile.Vec3` — canonical Quake hulls

- [ ] **Step 1: Write the failing test**

Create `internal/bspdec/hull_test.go`:

```go
package bspdec

import (
	"bytes"
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

func TestDecompileHull1RecoversWallPlanes(t *testing.T) {
	tree, data := compileFixture(t, roomMap())
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	nodes, err := clipnodesOf(f)
	if err != nil {
		t.Fatalf("clipnodesOf: %v", err)
	}
	d := newDecompiler(tree, Options{GridSnap: 8, TextureFallback: "nearest"})
	brushes := d.decompileHull(0, 1, nodes)
	if len(brushes) == 0 {
		t.Fatal("hull 1 produced no brushes")
	}
	// the room's inner west wall plane is x=0: hull 1 expands it to x=16,
	// and un-expansion must bring it back to the lattice
	found := false
	for _, b := range brushes {
		if b.Contents != bsp.ContentsClip {
			t.Fatalf("hull brush contents = %d, want ContentsClip", b.Contents)
		}
		for _, s := range b.Sides {
			if s.TexName != "clip" {
				t.Fatalf("hull side texture = %q, want clip", s.TexName)
			}
			if s.Plane.Normal == (mapfile.Vec3{1, 0, 0}) && math.Abs(s.Plane.Dist-0) < planeMatchEpsilon {
				found = true
			}
			// every surviving plane must match a render-lump plane
			if !d.planeInLump(s.Plane) {
				t.Fatalf("bevel plane survived un-expansion: %+v", s.Plane)
			}
		}
	}
	if !found {
		t.Fatal("west wall plane x=0 not recovered from hull 1")
	}
}

func TestDecompileHullMissing(t *testing.T) {
	tree, data := compileFixture(t, roomMap())
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	nodes, err := clipnodesOf(f)
	if err != nil {
		t.Fatalf("clipnodesOf: %v", err)
	}
	d := newDecompiler(tree, Options{})
	// hull 3 is unused by Quake; a missing/zero headnode must not panic
	_ = d.decompileHull(0, 3, nodes)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestDecompileHull' -count=1`
Expected: FAIL — undefined `clipnodesOf`, `decompileHull`, `planeInLump`.

- [ ] **Step 3: Implement `hull.go`**

Create `internal/bspdec/hull.go`:

```go
package bspdec

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// hullMins/hullMaxs are the canonical Quake collision hull AABBs.
//
// Where in C: hull_min/hull_max in sv_main.c (hull 3 unused in Quake).
var hullMins = [4]mapfile.Vec3{{0, 0, 0}, {-16, -16, -24}, {-32, -32, -24}, {0, 0, 0}}
var hullMaxs = [4]mapfile.Vec3{{0, 0, 0}, {16, 16, 32}, {32, 32, 64}, {0, 0, 0}}

// clipnode is the normalized collision-node record (children < 0 = contents).
type clipnode struct {
	PlaneNum int32
	Children [2]int32
}

// clipnodesOf normalizes the version-dependent clipnode lump.
func clipnodesOf(f *bsp.File) ([]clipnode, error) {
	switch cn := f.Clipnodes.(type) {
	case []bsp.DSClipNode:
		out := make([]clipnode, len(cn))
		for i, c := range cn {
			out[i] = clipnode{PlaneNum: c.PlaneNum, Children: c.Children}
		}
		return out, nil
	case []bsp.DLClipNode:
		out := make([]clipnode, len(cn))
		for i, c := range cn {
			out[i] = clipnode{PlaneNum: c.PlaneNum, Children: c.Children}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported clipnode lump type %T", f.Clipnodes)
	}
}

// unexpandPlane reverses hull expansion: qbsp offsets each plane by the hull
// AABB corner most extreme along the normal (Minkowski sum), so decompile
// subtracts exactly that.
//
// Where in C: hull expansion in ericw qbsp hull.c (this is its inverse).
func unexpandPlane(p mapfile.Plane, mins, maxs mapfile.Vec3) mapfile.Plane {
	var off mapfile.Vec3
	for i := 0; i < 3; i++ {
		if p.Normal[i] >= 0 {
			off[i] = maxs[i]
		} else {
			off[i] = mins[i]
		}
	}
	p.Dist -= v3Dot(p.Normal, off)
	return p
}

// planeInLump reports whether p matches any plane in the (render) planes
// lump. Clipnodes index the same lump; a hull plane that matches nothing
// after un-expansion is a compile-added bevel.
func (d *decompiler) planeInLump(p mapfile.Plane) bool {
	for i := range d.tree.Planes {
		if planesMatch(p, d.plane(int32(i))) {
			return true
		}
	}
	return false
}

// decompileHull walks the clipnode tree of the given hull and emits clip
// brushes with expansion reversed and bevels dropped.
//
// Where in C: hull decompile in bspc map_q1.c / BSP Forge (un-expand by the
// hull AABB, drop bevel-only planes).
func (d *decompiler) decompileHull(modelIdx, hull int, nodes []clipnode) []*Brush {
	m := d.tree.Models[modelIdx]
	head := m.HeadNode[hull]
	if head < 0 || int(head) >= len(nodes) {
		return nil // model has no collision hull
	}
	mins := mapfile.Vec3{float64(m.BoundsMin.X) - bboxGrow, float64(m.BoundsMin.Y) - bboxGrow, float64(m.BoundsMin.Z) - bboxGrow}
	maxs := mapfile.Vec3{float64(m.BoundsMax.X) + bboxGrow, float64(m.BoundsMax.Y) + bboxGrow, float64(m.BoundsMax.Z) + bboxGrow}
	var out []*Brush
	d.walkClipnodes(int(head), boxBrush(mins, maxs), nodes, &out)
	for _, b := range out {
		var kept []*Side
		for _, s := range b.Sides {
			p := unexpandPlane(s.Plane, hullMins[hull], hullMaxs[hull])
			if d.planeInLump(p) {
				kept = append(kept, &Side{Plane: p, TexName: "clip"})
			}
		}
		b.Sides = kept
		b.Contents = bsp.ContentsClip
		removeRedundantPlanes(b)
		for _, s := range b.Sides {
			s.TexName = "clip"
		}
	}
	// drop brushes that lost all area to bevel removal
	alive := out[:0]
	for _, b := range out {
		if len(b.Sides) >= 4 {
			alive = append(alive, b)
		}
	}
	return alive
}

// walkClipnodes mirrors walk, but children index clipnodes and negative
// children are contents values.
func (d *decompiler) walkClipnodes(idx int, b *Brush, nodes []clipnode, out *[]*Brush) {
	n := nodes[idx]
	front, back := splitBrush(b, d.plane(n.PlaneNum))
	parts := [2]*Brush{front, back}
	for side := 0; side < 2; side++ {
		part := parts[side]
		if part == nil {
			continue
		}
		child := n.Children[side]
		if child < 0 {
			if child == bsp.ContentsSolid {
				part.Contents = bsp.ContentsSolid
				*out = append(*out, part)
			}
			continue
		}
		d.walkClipnodes(int(child), part, nodes, out)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS. If the west-wall assertion fails, print the unexpanded +x-normal dists: a systematic ±16 error means the Minkowski sign flipped; ±24/32 confusion means hull 1 vs 2 constants swapped.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/hull.go internal/bspdec/hull_test.go
git commit -m "feat(bspdec): decompile collision hulls with Minkowski un-expansion and bevel dropping"
```

---

## Task 11: Orchestrator + self-check + golden voxel test

Bead: `ironwail-go-xxy.2` (completes it). The pipeline per spec §4: parse → per model (walk → prune → texture → split → merge) → attach → stats. Plus the mandatory self-check (spec §11) and the golden occupancy proof.

**Files:**
- Create: `internal/bspdec/decompile.go`, `internal/bspdec/decompile_test.go`

**Interfaces:**
- Consumes: everything from Tasks 3–10.
- Produces:
  - `func Decompile(data []byte, opts Options) (*mapfile.Map, []ModelStats, error)` — the single entry point `cmd/bspdec` calls.
  - `func ValidateBrush(mb *mapfile.MapBrush) error`
  - `func SelfCheck(m *mapfile.Map) error`
  - Spec clarification (recorded here): `--decompile-hull N` **replaces** hull-0 output with the un-expanded hull's clip brushes — mixing both into one worldspawn would not compile; this matches bspc/bsputil `-hull` semantics.

- [ ] **Step 1: Write the failing tests**

Create `internal/bspdec/decompile_test.go`:

```go
package bspdec

import (
	"math"
	"strings"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/internal/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// brushPlanes re-derives a map brush's planes from its face points.
func brushPlanes(t *testing.T, mb *mapfile.MapBrush) []mapfile.Plane {
	t.Helper()
	planes := make([]mapfile.Plane, 0, len(mb.Faces))
	for _, f := range mb.Faces {
		p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2])
		if length < 0.01 {
			t.Fatalf("degenerate face points %v", f.Points)
		}
		planes = append(planes, p)
	}
	return planes
}

// voxelOccupancy rasterizes worldspawn brushes: the set of lattice cells
// whose center is inside any brush (inside = behind every plane).
func voxelOccupancy(t *testing.T, m *mapfile.Map, cell float64) map[[3]int]struct{} {
	t.Helper()
	ws := &m.Entities[0]
	type polyBrush struct{ planes []mapfile.Plane }
	var brushes []polyBrush
	mins := mapfile.Vec3{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
	maxs := mapfile.Vec3{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	for i := range ws.Brushes {
		brushes = append(brushes, polyBrush{brushPlanes(t, &ws.Brushes[i])})
		for _, f := range ws.Brushes[i].Faces {
			for _, p := range f.Points {
				for a := 0; a < 3; a++ {
					mins[a] = math.Min(mins[a], p[a])
					maxs[a] = math.Max(maxs[a], p[a])
				}
			}
		}
	}
	occ := map[[3]int]struct{}{}
	x0 := math.Floor(mins[0]/cell) * cell
	y0 := math.Floor(mins[1]/cell) * cell
	z0 := math.Floor(mins[2]/cell) * cell
	for x := x0; x < maxs[0]; x += cell {
		for y := y0; y < maxs[1]; y += cell {
			for z := z0; z < maxs[2]; z += cell {
				c := mapfile.Vec3{x + cell/2, y + cell/2, z + cell/2}
				for _, b := range brushes {
					inside := true
					for _, p := range b.planes {
						if v3Dot(c, p.Normal)-p.Dist > 0.01 {
							inside = false
							break
						}
					}
					if inside {
						occ[[3]int{int(x / cell), int(y / cell), int(z / cell)}] = struct{}{}
						break
					}
				}
			}
		}
	}
	return occ
}

func voxelIoU(t *testing.T, a, b map[[3]int]struct{}) float64 {
	t.Helper()
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		t.Fatal("both occupancy sets empty")
	}
	return float64(inter) / float64(union)
}

func TestDecompileGoldenVoxelIoU(t *testing.T) {
	_, data := compileFixture(t, roomMap())
	out, stats, err := Decompile(data, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if len(stats) == 0 || stats[0].Brushes == 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if stats[0].LeavesSolid == 0 || stats[0].PlanesUsed == 0 {
		t.Fatalf("stats not populated: %+v", stats[0])
	}
	orig, err := qbsp.ParseMap(strings.NewReader(roomMap()))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	iou := voxelIoU(t, voxelOccupancy(t, orig, 8), voxelOccupancy(t, out, 8))
	// Below 0.95 means a treewalk/texturing bug. Investigate; do not relax.
	if iou < 0.95 {
		t.Fatalf("voxel IoU vs original = %v, want >= 0.95", iou)
	}
}

func TestDecompileSelfCheckOnOutput(t *testing.T) {
	_, data := compileFixture(t, roomWithDoorMap())
	out, _, err := Decompile(data, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if err := SelfCheck(out); err != nil {
		t.Fatalf("SelfCheck: %v", err)
	}
}

func TestValidateBrushRejectsDegenerate(t *testing.T) {
	bad := mapfile.MapBrush{Faces: []mapfile.MapFace{
		{Points: [3]mapfile.Vec3{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}}, // collinear
		{Points: [3]mapfile.Vec3{{0, 0, 0}, {0, 1, 0}, {0, 0, 1}}},
		{Points: [3]mapfile.Vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}},
		{Points: [3]mapfile.Vec3{{0, 0, 0}, {1, 1, 0}, {0, 0, 1}}},
	}}
	if err := ValidateBrush(&bad); err == nil {
		t.Fatal("expected degenerate-plane rejection")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -run 'TestDecompile|TestValidateBrush' -count=1`
Expected: FAIL — undefined `Decompile`, `SelfCheck`, `ValidateBrush`.

- [ ] **Step 3: Implement `decompile.go`**

Create `internal/bspdec/decompile.go`:

```go
package bspdec

import (
	"bytes"
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

// Decompile runs the full pipeline over BSP file bytes and returns the map
// plus per-model stats. Pipeline per model (spec section 4): treewalk (or
// hull walk) -> redundant-plane removal -> texturing -> texture-boundary
// splits -> convex merge -> entity attach.
func Decompile(data []byte, opts Options) (*mapfile.Map, []ModelStats, error) {
	if opts.TextureFallback == "" {
		opts.TextureFallback = "nearest"
	}
	if opts.DecompileHull < 0 || opts.DecompileHull > 3 {
		return nil, nil, fmt.Errorf("invalid hull %d (want 0-3)", opts.DecompileHull)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("loading BSP: %w", err)
	}
	d := newDecompiler(tree, opts)
	ents, err := parseEntities(tree)
	if err != nil {
		return nil, nil, err
	}
	perModel := make([][]*Brush, len(tree.Models))
	stats := make([]ModelStats, 0, len(tree.Models))
	if opts.DecompileHull > 0 {
		f, err := bsp.Load(bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("loading BSP for clipnodes: %w", err)
		}
		nodes, err := clipnodesOf(f)
		if err != nil {
			return nil, nil, err
		}
		for mi := range tree.Models {
			before := d.warnings
			brushes := d.decompileHull(mi, opts.DecompileHull, nodes)
			perModel[mi] = brushes
			stats = append(stats, ModelStats{
				Model: mi, Brushes: len(brushes),
				PlanesUsed: planeSetCount(brushes), Warnings: d.warnings - before,
			})
		}
	} else {
		for mi := range tree.Models {
			beforeW, beforeL := d.warnings, d.leavesSolid
			brushes, err := d.decompileModel(mi)
			if err != nil {
				return nil, nil, err
			}
			for _, b := range brushes {
				removeRedundantPlanes(b)
			}
			d.textureBrushes(brushes)
			var split []*Brush
			for _, b := range brushes {
				split = append(split, d.splitDifferentTextures(b)...)
			}
			brushes = split
			if opts.MergeConvex {
				brushes = mergeConvex(brushes)
			}
			perModel[mi] = brushes
			stats = append(stats, ModelStats{
				Model: mi, Brushes: len(brushes),
				LeavesSolid: d.leavesSolid - beforeL,
				PlanesUsed:  planeSetCount(brushes), Warnings: d.warnings - beforeW,
			})
		}
	}
	return attachBrushes(ents, perModel, tree), stats, nil
}

// SelfCheck validates every emitted brush: >=4 faces, non-degenerate planes,
// and convexity (every face's points on or behind every other face's plane).
// The CLI maps a failure to exit code 3.
func SelfCheck(m *mapfile.Map) error {
	for ei := range m.Entities {
		for bi := range m.Entities[ei].Brushes {
			if err := ValidateBrush(&m.Entities[ei].Brushes[bi]); err != nil {
				return fmt.Errorf("entity %d brush %d: %w", ei, bi, err)
			}
		}
	}
	return nil
}

// ValidateBrush checks one brush. The epsilon is generous because grid snap
// quantizes the emitted points before this runs.
func ValidateBrush(mb *mapfile.MapBrush) error {
	if len(mb.Faces) < 4 {
		return fmt.Errorf("only %d faces", len(mb.Faces))
	}
	planes := make([]mapfile.Plane, len(mb.Faces))
	for i, f := range mb.Faces {
		p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2])
		if length < 0.01 {
			return fmt.Errorf("face %d: degenerate plane points", i)
		}
		planes[i] = p
	}
	const eps = 0.5
	for i, f := range mb.Faces {
		for j, p := range planes {
			if i == j {
				continue
			}
			for _, pt := range f.Points {
				if v3Dot(pt, p.Normal)-p.Dist > eps {
					return fmt.Errorf("face %d point %v in front of face %d's plane (non-convex)", i, pt, j)
				}
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/bspdec -count=1 && mise run verify`
Expected: PASS, including the golden `TestDecompileGoldenVoxelIoU >= 0.95`.

Bead `ironwail-go-xxy.2` core acceptance (deterministic core complete, golden IoU green) is met; corpus-scale bsputil comparison belongs to bead xxy.4 / plan 2. Close the bead: `bd close ironwail-go-xxy.2 --reason="treewalk core + texturing + splits + merge + hull un-expansion complete; golden voxel IoU >= 0.95 vs original map brushes"`.

- [ ] **Step 5: Commit**

```bash
git add internal/bspdec/decompile.go internal/bspdec/decompile_test.go
git commit -m "feat(bspdec): add Decompile orchestrator with self-check and golden voxel-IoU test"
```

---

## Task 12: `cmd/bspdec` CLI

Bead: `ironwail-go-xxy.3`. Spec §3 contract: exact flags, exit codes 0/1/2/3, slog logging, `--json` schema. Follows the `cmd/qbsp` flag-package pattern (Go's `flag` accepts both `-x` and `--x`).

**Files:**
- Create: `cmd/bspdec/main.go`, `cmd/bspdec/main_test.go`

**Interfaces:**
- Consumes: `bspdec.Decompile`, `bspdec.SelfCheck`, `bspdec.Options`, `bspdec.ModelStats` (Task 11); `mapfile.Write`, `mapfile.Parse`, `mapfile.WriteOptions` (Task 2).
- Produces: `func run(args []string, stdout, stderr io.Writer) int` — testable entry; `main()` calls it. JSON schema keys are the spec §3 contract (stable names).

- [ ] **Step 1: Write the failing tests**

Create `cmd/bspdec/main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// box is a minimal QuakeEd box brush (winding pattern compile-proven in the
// qbsp suite's slabBrush helper).
func box(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	f := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mi := [3]float64{x0, y0, z0}
	ma := [3]float64{x1, y1, z1}
	return "{\n" +
		f([3]float64{ma[0], mi[1], mi[2]}, [3]float64{ma[0], mi[1], ma[2]}, [3]float64{ma[0], ma[1], mi[2]}) +
		f([3]float64{mi[0], ma[1], mi[2]}, [3]float64{mi[0], ma[1], ma[2]}, [3]float64{mi[0], mi[1], ma[2]}) +
		f([3]float64{mi[0], ma[1], mi[2]}, [3]float64{ma[0], ma[1], mi[2]}, [3]float64{mi[0], ma[1], ma[2]}) +
		f([3]float64{mi[0], mi[1], mi[2]}, [3]float64{mi[0], mi[1], ma[2]}, [3]float64{ma[0], mi[1], mi[2]}) +
		f([3]float64{mi[0], mi[1], ma[2]}, [3]float64{mi[0], ma[1], ma[2]}, [3]float64{ma[0], mi[1], ma[2]}) +
		f([3]float64{mi[0], mi[1], mi[2]}, [3]float64{ma[0], mi[1], mi[2]}, [3]float64{mi[0], ma[1], ma[2]}) +
		"}\n"
}

// writeTestBSP compiles a sealed 256-cube room and writes it to dir.
func writeTestBSP(t *testing.T, dir string) string {
	t.Helper()
	src := "{\n\"classname\" \"worldspawn\"\n" +
		box(-64, -64, -64, 0, 320, 256, "wwall") +
		box(256, -64, -64, 320, 320, 256, "wwall") +
		box(0, -64, -64, 256, 0, 256, "wwall") +
		box(0, 256, -64, 256, 320, 256, "wwall") +
		box(0, 0, -64, 256, 256, 0, "ffloor") +
		box(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
	m, err := qbsp.ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatal("test room leaks")
	}
	p := filepath.Join(dir, "room.bsp")
	if err := os.WriteFile(p, res.Data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	outPath := filepath.Join(dir, "out.map")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-o", outPath, bspPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "worldspawn") {
		t.Fatalf("output missing worldspawn:\n%s", data)
	}
	// default output path: <stem>.bspdec.map
	var so, se bytes.Buffer
	if code := run([]string{bspPath}, &so, &se); code != 0 {
		t.Fatalf("default-output run exit = %d: %s", code, se.String())
	}
	def := filepath.Join(dir, "room.bspdec.map")
	if _, err := os.Stat(def); err != nil {
		t.Fatalf("default output missing: %v", err)
	}
}

func TestRunJSONSummarySchema(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	outPath := filepath.Join(dir, "out.map")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-json", "-o", outPath, bspPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit: stderr %s", stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	for _, k := range []string{"input", "output", "bspx_brushlist", "models", "ml", "exit"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing key %q in %v", k, doc)
		}
	}
	if doc["exit"].(float64) != 0 {
		t.Fatalf("exit field = %v", doc["exit"])
	}
	models := doc["models"].([]any)
	if len(models) == 0 {
		t.Fatal("no models in summary")
	}
	m0 := models[0].(map[string]any)
	for _, k := range []string{"model", "brushes", "leaves_solid", "planes_used", "warnings"} {
		if _, ok := m0[k]; !ok {
			t.Fatalf("missing model key %q in %v", k, m0)
		}
	}
	if m0["brushes"].(float64) < 6 {
		t.Fatalf("model 0 brushes = %v, want >= 6", m0["brushes"])
	}
	ml := doc["ml"].(map[string]any)
	if ml["stage"] != "none" || ml["model"] != nil {
		t.Fatalf("ml = %v, want {none, null}", ml)
	}
}

func TestRunExitCodes(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"/nonexistent.bsp"}, &so, &se); code != 1 {
		t.Fatalf("missing input exit = %d, want 1", code)
	}
	if code := run([]string{}, &so, &se); code != 1 {
		t.Fatalf("no-args exit = %d, want 1", code)
	}
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	if code := run([]string{"-ml", "seams", bspPath}, &so, &se); code != 1 {
		t.Fatalf("--ml exit = %d, want 1 (M0 has no ML)", code)
	}
	if code := run([]string{"-texture-fallback", "bogus", bspPath}, &so, &se); code != 1 {
		t.Fatalf("bad fallback exit = %d, want 1", code)
	}
	if code := run([]string{"-format", "obj", bspPath}, &so, &se); code != 1 {
		t.Fatalf("bad format exit = %d, want 1", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./cmd/bspdec -count=1`
Expected: FAIL — no Go files in cmd/bspdec (the package does not exist yet).

- [ ] **Step 3: Implement `cmd/bspdec/main.go`**

```go
// Command bspdec decompiles Quake BSP29 files into editable .map files.
// See docs/superpowers/specs/2026-09-07-bspdec-design.md section 3 for the
// CLI contract (flags, exit codes, --json schema).
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/internal/map"
)

const (
	exitOK        = 0
	exitInput     = 1
	exitInternal  = 2
	exitSelfCheck = 3
)

// jsonSummary is the spec section 3 --json schema (stable key names).
type jsonSummary struct {
	Input         string          `json:"input"`
	Output        string          `json:"output"`
	BSPXBrushlist bool            `json:"bspx_brushlist"`
	Models        []jsonModelStat `json:"models"`
	ML            jsonML          `json:"ml"`
	Exit          int             `json:"exit"`
}

type jsonModelStat struct {
	Model       int `json:"model"`
	Brushes     int `json:"brushes"`
	LeavesSolid int `json:"leaves_solid"`
	PlanesUsed  int `json:"planes_used"`
	Warnings    int `json:"warnings"`
}

type jsonML struct {
	Stage string  `json:"stage"`
	Model *string `json:"model"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bspdec", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output .map path (default <input stem>.bspdec.map; - = stdout)")
	format := fs.String("format", "map", "output format (only map)")
	noBrushlist := fs.Bool("no-brushlist", false, "ignore a BSPX BRUSHLIST lump even if present")
	hull := fs.Int("decompile-hull", 0, "decompile collision hull N (1-3) instead of the render hull")
	merge := fs.Bool("merge-convex", true, "merge same-contents coplanar-adjacent convex cells")
	grid := fs.Int("grid-snap", 8, "quantize output brush points to integer lattice N (0 = off)")
	texFallback := fs.String("texture-fallback", "nearest", "policy for sides with no matching face: skip|nearest|trigger")
	ml := fs.String("ml", "", "enable ML stages: seams|group|all (requires -model-dir; M2+)")
	modelDir := fs.String("model-dir", "models/bspdec", "directory of provisioned model artifacts")
	jsonOut := fs.Bool("json", false, "print a machine-readable summary on stdout")
	loglevel := fs.String("loglevel", "info", "slog level: debug|info|warn|error")
	if err := fs.Parse(args); err != nil {
		return exitInput
	}

	var lv slog.Level
	if err := lv.UnmarshalText([]byte(*loglevel)); err != nil {
		fmt.Fprintf(stderr, "bspdec: bad -loglevel %q\n", *loglevel)
		return exitInput
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: lv})))

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bspdec [-o out.map] [flags] input.bsp")
		return exitInput
	}
	if *format != "map" {
		fmt.Fprintf(stderr, "bspdec: unsupported -format %q (only map)\n", *format)
		return exitInput
	}
	if *ml != "" {
		fmt.Fprintf(stderr, "bspdec: -ml %q needs the M2 training pipeline (not built); see docs/superpowers/specs/2026-09-07-bspdec-design.md section 7\n", *ml)
		return exitInput
	}
	_ = *modelDir // reserved for M2
	switch *texFallback {
	case "skip", "nearest", "trigger":
	default:
		fmt.Fprintf(stderr, "bspdec: bad -texture-fallback %q\n", *texFallback)
		return exitInput
	}
	if *hull < 0 || *hull > 3 {
		fmt.Fprintf(stderr, "bspdec: bad -decompile-hull %d (want 0-3)\n", *hull)
		return exitInput
	}

	input := fs.Arg(0)
	output := *out
	if output == "" {
		output = strings.TrimSuffix(input, filepath.Ext(input)) + ".bspdec.map"
	}
	if output == "-" && *jsonOut {
		fmt.Fprintln(stderr, "bspdec: -o - and -json both write stdout; pick one")
		return exitInput
	}

	data, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(stderr, "bspdec: %v\n", err)
		return exitInput
	}
	m, stats, err := bspdec.Decompile(data, bspdec.Options{
		NoBrushlist:     *noBrushlist,
		DecompileHull:   *hull,
		MergeConvex:     *merge,
		GridSnap:        *grid,
		TextureFallback: *texFallback,
	})
	if err != nil {
		fmt.Fprintf(stderr, "bspdec: %v\n", err)
		return exitInternal
	}

	// Marshal first, then self-check the exact bytes that will ship.
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, m, mapfile.WriteOptions{GridSnap: *grid}); err != nil {
		fmt.Fprintf(stderr, "bspdec: write: %v\n", err)
		return exitInternal
	}
	chk, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		fmt.Fprintf(stderr, "bspdec: self-check re-parse: %v\n", err)
		return exitSelfCheck
	}
	if err := bspdec.SelfCheck(chk); err != nil {
		fmt.Fprintf(stderr, "bspdec: self-check: %v\n", err)
		return exitSelfCheck
	}

	if output == "-" {
		if _, err := stdout.Write(buf.Bytes()); err != nil {
			return exitInternal
		}
	} else if err := os.WriteFile(output, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(stderr, "bspdec: write %s: %v\n", output, err)
		return exitInput
	}

	if *jsonOut {
		s := jsonSummary{
			Input: input, Output: output,
			BSPXBrushlist: hasBSPXBrushlist(data),
			ML:            jsonML{Stage: "none", Model: nil},
			Exit:          exitOK,
		}
		for _, ms := range stats {
			s.Models = append(s.Models, jsonModelStat{
				Model: ms.Model, Brushes: ms.Brushes, LeavesSolid: ms.LeavesSolid,
				PlanesUsed: ms.PlanesUsed, Warnings: ms.Warnings,
			})
		}
		enc := json.NewEncoder(stdout)
		if err := enc.Encode(s); err != nil {
			return exitInternal
		}
	}
	return exitOK
}

// hasBSPXBrushlist reports whether the BSP carries a BSPX BRUSHLIST lump, by
// structurally validating the BSPX header and lump table. (The full reader
// moves to internal/bsp with the M1 BRUSHLIST path, bead ironwail-go-xxy.5.)
func hasBSPXBrushlist(data []byte) bool {
	for off := 0; off < len(data); {
		i := bytes.Index(data[off:], []byte("BSPX"))
		if i < 0 {
			return false
		}
		idx := off + i
		if idx+8+32 <= len(data) {
			n := binary.LittleEndian.Uint32(data[idx+4:])
			if n > 0 && n < 64 && idx+8+int(n)*32 <= len(data) {
				for k := 0; k < int(n); k++ {
					ent := idx + 8 + k*32
					if string(bytes.TrimRight(data[ent:ent+24], "\x00")) == "BRUSHLIST" {
						return true
					}
				}
			}
		}
		off = idx + 4
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./cmd/bspdec -count=1 && mise run build-bspdec && mise run verify`
Expected: PASS; `bin/bspdec` builds; full verify green.

- [ ] **Step 5: Manual smoke (optional, asset-gated)**

If `./quake-data` is present: `./ironwailgo`-style env is not needed; run `bin/bspdec "$QUAKE_DIR/id1/maps/e1m1.bsp" -o /tmp/e1m1.bspdec.map -json` (skip when assets are absent — corpus coverage is plan 2's job).

- [ ] **Step 6: Commit**

```bash
git add cmd/bspdec/main.go cmd/bspdec/main_test.go
git commit -m "feat(bspdec): add bspdec CLI with flags, exit codes, and JSON summary"
```

Bead `ironwail-go-xxy.3` acceptance (CLI matrix + JSON schema tests, wires M0 core + internal/map) is met. Close it: `bd close ironwail-go-xxy.3 --reason="cmd/bspdec with full flag table, exit codes 0-3, JSON schema test, end-to-end golden via in-repo qbsp"`.

---

## Self-review results (plan author)

Checked against spec `2026-09-07-bspdec-design.md`:

1. **Spec coverage** — §3 CLI: Task 12 (all flags incl. reserved `--format`/`--ml`; `--no-brushlist` parsed but inert until M1's path lands — noted in Task 4's `Options`). §5 core steps: 1–2 → Task 4; 3–4 → Tasks 4–5; 5 → Task 7; 6 → Task 6; 7 → Task 8; 8 → Task 6; 9 → Task 10; 10 (BRUSHLIST) → deferred to the M1 plan per bead xxy.5. §6 map package → Tasks 1–2. §8–10 eval/dataset/training → later plans (below). §11 error handling → Tasks 11–12. §12 testing → unit + golden tests in every task; corpus/vintage tiers are plan 2/6 scope. §13 conventions → Global Constraints.
2. **Placeholder scan** — every code step carries complete code; the only deferred work is explicitly bead-bound (M1+), not task-local.
3. **Type consistency** — `Winding`/`Clip`/`Area`/`Centroid`/`negatePlane` (Task 3) ← Tasks 4–11; `Brush`/`Side`/`Options`/`ModelStats`/`clipBrush`/`splitBrush`/`boxBrush`/`decompileModel`/`newDecompiler` (Task 4) ← Tasks 5–11; `planesMatch`/`planesOpposite`/`removeRedundantPlanes`/`planeSetCount` (Task 5) ← Tasks 6–11; `collectFaces`/`clipToBrush`/`overlapArea`/`textureBrush(es)`/`contentsTexture`/`vecsEqual`/`texturedFace` (Task 6) ← Tasks 7, 9; `splitDifferentTextures`/`inwardEdgePlane` (Task 7) ← Task 11; `parseEntities`/`attachBrushes`/`toMapBrush`/`pick3Points` (Task 8) ← Task 11; `mergeConvex` (Task 9) ← Task 11; `clipnodesOf`/`decompileHull` (Task 10) ← Task 11; `Decompile`/`SelfCheck` (Task 11) ← Task 12. `mapfile.{Vec3,Plane,Map,Entity,Epair,MapBrush,MapFace,TexDef,Parse,Write,WriteOptions,PlaneFromPoints}` (Tasks 1–2) ← all.
4. **Known deliberate simplifications** (recorded, testable, warn-counted): texture-boundary splits only when cleanly separable (Task 7); M0 has no BRUSHLIST read path (M1 bead xxy.5); `--decompile-hull` replaces rather than augments hull-0 output (Task 11).

## Subsequent plans (writing-plans scope check: one plan per milestone cluster)

| Plan | Beads | Scope | Written when |
| --- | --- | --- | --- |
| 2. M0 eval + corpus | xxy.4 | `internal/bspdec/eval`, `tools/bspdec_corpus` P1–P7, real `bspdec-data/eval/report` tasks, bsputil-parity acceptance run | immediately (unblocked by this plan) |
| 3. M1 oracle + gate | xxy.11, xxy.5 | `tools/bspdec_synth`, BRUSHLIST reader into `internal/bsp`, direct path, headroom study → ML go/no-go | after plan 2 (needs the corpus to measure) |
| 4. M2 ML stages | xxy.12, xxy.6, xxy.7 | training pipeline, Routes A+B | only if the M1 gate says go |
| 5. M3 research | xxy.8, xxy.9 | Routes C+D | after M2 gates |
| 6. M4 hardening | xxy.10 | vintage slice, full matrix, docs | after plan 2, parallel-safe with 3–5 |

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-07-bspdec-m0-deterministic-decompiler.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration (superpowers:subagent-driven-development).
2. **Inline Execution** — execute tasks in-session with superpowers:executing-plans, batch execution with checkpoints.
