# Pure-Go Map Compiler Pipeline (QBSP, VIS, LIGHT) — Implementation Plan

> **Bead:** `ironwail-go-t63` · **Status:** plan (draft, rev 2) · **Date:** 2026-09-05
> **Related:** `internal/bsp` (reader, BSP29/BSP2/Quake64, `ApplyLitFile`),
> `cmd/bspdiag` (validation), `cmd/bspgen` (hand-built fixture),
> `internal/server/collision` (hull 0/1/2 clipnode usage), `sdk` (asset-free
> headless boots).

## Implementation status (M0 = done, M1 = landed, M3 = landed w/ known gap)

- **M0 (`.map` parser) — done.** `internal/qbsp/mapfile.go`: QuakeEd + Valve
  220 per-face detection, entity dict, Q2 extended texinfo, dup/degenerate
  face drop, CRLF, comments; texinfo vecs via the QuakeEd `baseaxis[18]`.
- **M1 (BSP29/BSP2, hulls, leaks, .prt) — landed.** `internal/qbsp`: plane
  arrangement CSG, faces/edges/surfedges, texinfo + miptex, single clip
  hull tree shared by engine hulls 1/2, solid/water/sky contents, void-flood
  leaks + `.pts`, BSP29 + BSP2 writers, `.prt` PRT1 writer. Round-tripped
  through `internal/bsp.LoadTree`/loader; **ericw-tools `bspinfo` reads both
  BSP29 and BSP2 output**. `cmd/qbsp` CLI + `mise run build-qbsp`.
  Two correctness fixes landed during hardening: content assignment now uses
  the facet-vertex centroid (AABB midpoint misclassified thin wall cells),
  and the BSP node tree is built by replaying the arrangement's split
  sequence (a balancing heuristic produced overlapping leaf regions).
- **M3 (vis / PVS) — landed.** `internal/vis`: PRT1 reader, portal-flow PVS
  (window polygon clipping), RLE row writer matching the engine's
  `DecompressVis`, leaf `visofs` + model `visleafs` patching, `cmd/vis` CLI +
  `mise run build-vis`. The pipeline runs end-to-end (qbsp→vis→bsp; ericw
  `bspinfo` reads the vis'd BSP with PVS rows). Connectivity is verified:
  room A sees room B through a doorway, sealed rooms stay invisible.
- **Key correctness fix (root cause of earlier PVS failures):** axial planes
  are now normalized to POSITIVE normals at table entry (the engine's
  PointInLeaf fast path assumes `d = p.X - dist` for planeX; negative-normal
  axial planes misrouted the descent, making the engine's leaf lookup
  disagree with the compiler's cell mapping). This also required the box
  planes and the winding-from-box trim to use the normalized orientation
  (box interior = back of +maxs, front of +mins).
- **M4 (light) — landed (direct lighting).** `internal/light`: lightmap
  extents math (`CalcExtents`, 16 units/luxel), BSP face/entity parsing
  (polygons from vertexes/edges/surfedges, texinfo vecs, plane normals,
  light entities), direct point lighting (`light/dist²·cosθ` with ray-vs-BSP
  shadow traces via `TreeTracer`, 255 clamp), and the QLIT v1 colored
  sidecar writer (round-tripped through `bsp.ApplyLitFile`). `cmd/light`
  patches each lit face's `lightofs` + the Lighting lump and writes `.lit`.
  Verified end-to-end: qbsp→light on a lit box (388 samples / 70 lit faces;
  `.lit` = 8+3·samples bytes; ericw `bspinfo` reads the lit BSP). Unit +
  integration tests cover extents, falloff, shadow blocking, sky-face
  skipping, `.lit` contract, and full BSP integration.
- **M5 (pipeline, parity, docs) — landed.** `mise run map-build MAP=name`
  chains qbsp→vis→light (verified end-to-end: a lit box compiles to a BSP
  ericw `bspinfo` reads with both visdata and lightdata). The parity harness
  (`internal/qbsp/parity_test.go`, gated on `ERICW_TOOLS_DIR`) compiles a
  map with our qbsp and verifies ericw `bspinfo` reads the qbsp, vis'd, and
  lit output with sane lump counts. `docs/MAP_COMPILING.md` documents the
  full authoring→compile→verify cycle.
- **Known v1 gaps (documented, follow-up):** brush-entity submodels (`*N`);
  arrangement CSG is small-map-only (solidbsp port is the scale path); no
  t-junction fixing; light styles/sun/bounce deferred (M4 phase 2).

## Current-state audit (verified 2026-09-05)

| Area | State | Evidence |
|---|---|---|
| `.map` parser | ❌ Missing | Nothing parses map files. ericw oracle: `common/mapfile.cc/.hh` (generic model: `brush_side_t{planepts[3], texture, texdef variant}`, `brush_t`, `map_entity_t`) with texcoord styles `quaked | valve_220 | brush_primitives | etp` and a separate `convert_to()` pass. |
| BSP reader | ✅ Landed | `internal/bsp` — BSP29 + `DL1Node`/`DL2Node`/Quake64, `tree.go` (`LoadTree`, models `HeadNode[4]`, `DecompressVis` RLE), `lit.go` (`ApplyLitFile` QLIT v1), limits/contents from `bspfile.h`. |
| BSP writer | ❌ Missing | `cmd/bspgen` hand-assembles one transparency map only. ericw oracle: `qbsp/writebsp.cc`, `common/bspfile_q1.hh` (exact encodings below). |
| Collision hulls | Engine-side | `internal/server/collision/hull.go` uses hulls 0/1/2 via `HeadNode`; ericw generates per-hull trees (up to `MAX_MAP_HULLS_Q1 = 4` with "only 3 hulls exist" + empty-tree fallback via `_hulls`). |
| PVS | Engine-side only | `DecompressVis` (LumpVisibility RLE rows, per-leaf `visofs`, `-1 = none`) is the output contract. |
| Light | Engine-side only | Lightmaps 1 byte/texel mono in `Lighting` (face `lightofs`, `styles[4]`); `.lit` = `QLIT` + version + RGB (engine validates `8 + 3·len(Lighting)`). |
| C tools + corpus | ✅ **Now available** | Binaries: `build-linux/{qbsp,vis,light,bspinfo,bsputil,lightpreview}/<tool>`. Reference outputs for dozens of `q1_*` testmaps in `build-linux/` (`q1_light_black.bsp/.lit/.prt/.log`, leak `.pts`, `lm_0.png` lightmap renders). 275 `.map` fixtures in `testmaps/`. |
| Assets | Constrained | `.gitignore` whitelist ignores non-`.go` files: fixtures need explicit allow rules or procedural generation in tests. |

## ericw-tools findings that shape the design (read 2026-09-05)

### Format details (`include/common/bspfile_q1.hh`, `common/bspfile_q1.cc`)

- **BSP29 nodes**: `int32 planenum`, `twosided<int16> children` — *negative
  values are `-(leaf+1)`, not nodes*; children[0] front, [1] back; int16
  mins/maxs; `uint16 firstface/numfaces`.
- **BSP2RMQ nodes**: int32 children + uint32 face span. **BSP2 nodes**: full
  32-bit (plane, children, firstface/numfaces), 32-bit also for leaves with
  **float bounds** (`qvec3f`), uint32 marksurfaces.
- **Clipnodes**: `int32 planenum`, `twosided<int16> children` with *negative
  = contents*; the int16 downcast only supports real clipnodes up to `0xfff0`
  (65520) — writer must error past that limit.
- **Faces**: `int16 planenum, side; int32 firstedge; int16 numedges,
  texinfo; uint8 styles[4]; int32 lightofs` (BSP29) vs all-int32 (BSP2).
- **Leaves**: `int32 contents, visofs (-1 = none)`, int16 bounds (BSP29) /
  float (BSP2), `uint16/32 firstmarksurface, nummarksurfaces`,
  `ambient_level[NUM_AMBIENTS]`.
- **Texture lump**: miptex table; `TEX_SPECIAL` flag on texinfo marks
  sky/slime (no lightmap, no 256-subdivision).
- **Contents**: Q1 EMPTY=-1, SOLID=-2, WATER=-3, SLIME=-4, LAVA=-5, SKY=-6;
  HL currents -9…-14; `CONTENTS_CLIP = -8` via BSPX.
- **Models**: `mins/maxs/origin/headnode[4]/visleafs/firstface/numfaces`.
- **Visibility lump**: no comp off-lump table in Q1 files; per-leaf
  RLE rows referenced by `visofs` (matches `DecompressVis` exactly).

### qbsp pipeline (`qbsp/qbsp.cc`, `brushbsp`, `csg`, `outside`, `prtfile`)

- Per **entity × hull** pass: `Brush_LoadEntity` → `BrushBSP` (solidbsp;
  split policy `AUTO`/`FAST`/`PRECISE` — FAST for brush entities, PRECISE
  for the world's second pass) → `MakeTreePortals` → **`FillOutside`** (BFS
  flood *from the void*, assigning `outside_distance` per leaf) → world
  rebuilds with PRECISE + new portals + refill → `MarkVisibleSides` →
  `MakeFaces` → `FreeTreePortals` → `PruneNodes`.
- **Leak = a content entity whose occupied leaf is void-reachable**
  (`outside_distance ≥ 0`); leak trail = walk from the entity leaf along
  decreasing `outside_distance` to the void portal chain, written as a
  `.pts` point file; `-leaktest` turns leaks into failures;
  `-keepprt` keeps the `.prt` on leaking maps. Point entities outside the
  map get `floodentity` special-casing to avoid spurious leaks.
- `detail` brushes (`func_detail`, `func_detail_wall`,
  `func_detail_illusionary`, `func_detail_fence`) and `_hulls` per-entity
  keys control CSG participation / hull generation.
- `MakeFaces` produces the face/edge/surfedge/vertex lumps; `PruneNodes`
  removes empty-space nodes; hull 0 world writes `name.prt`
  (`WritePortalFile`) unless leaking without `-keepprt`.

### Portal file (.prt) — the qbsp↔vis interop boundary

`common/prtfile.hh`: header (counts) + `prtfile_portal_t{winding, leafnums[2]}`
per portal; PRT1 (leaf-based) vs PRT2 (cluster-based, `dleafinfos` mapping
leaf→cluster) variants. **Adopting the same format gives full interop**:
our `vis` can consume ericw `qbsp` output and vice versa — the parity harness
cross-feeds `.prt` files both ways.

### vis (`vis/vis.cc`, `vis/flow.cc`, `vis/state.cc`)

Loads the BSP (leafs + `visofs` -1) **plus the .prt**, computes per-leaf PVS:
`BasePortalVis` (initial portal-visible test across facing/intersecting
portals) then `PortalFlow` recursion with a target/regular check budget
(`-targetchecks`, `-visdist`); `-fast` = trivial all-visible; `-nostate`
disables saved-state reuse; stats via `visstats_t`. Output rows are RLE as
the engine expects. We will implement portal-vs-portal visible tests
(winding intersection via polylib) and the flow recursion; `-fast` and
`-v` parity flags included.

### light (`light/light.cc`, `light/ltface.cc`, `light/main.cc`, `common/litfile.hh`)

- World surfaces (`GatherLightSurfaces`) → `lightsurf_t` with per-surface
  `lmscale` (`_lmscale`/`lightmapscale` keys), `faceextents_t` → lightmap
  sample grid (extents/scale math); per-style lightmaps
  (`Lightmap_ForStyle`: styles from face `styles[4]`; `INVALID_LIGHTSTYLE`);
  `-extra`/`-extra4` supersampling; `LightFace_Sky` sun entity handling;
  `bounce.cc` radiosity (clamped colorbleed); shadow traces against the BSP.
- Output: `Lightmap_Save` packs samples into the `Lighting` lump
  (mono, 1 byte/texel) and — with **`-lit`** — writes `.lit` v1
  (`QLIT`, version 1, RGB triplets, precisely what `ApplyLitFile` validates);
  `-lit2` and `-lithdr` (E5BGR9 packed, version `0x00010000|1`) exist but are
  **out of scope** (the engine's `ApplyLitFile` reads v1 only).
- Reference log example (`q1_light_black-light.log`): "0 sky faces / 37
  solid faces / 0 filtered / 0 empty lightmaps" — a per-map assert target.

### Validation tooling

- **bspinfo**: prints version + 15-lump byte counts; e.g. reference
  `q1_light_black.bsp` = 1 model / 19 planes / 51 vertexes / 8 nodes /
  6 texinfos / 45 faces / 12 clipnodes / 4 leafs / 48 marksurfaces /
  109 edges / 200 surfedges / 3 textures / lightdata 408 / visdata 0 /
  entdata 312. This dump becomes the parity table.
- **bsputil**: BSP maintenance/compression utils (uses engine-side lumps);
  usable in the harness for sanity checks on our output.
- **lightpreview**: interactive viewer (dev-time eyeball only).

---

## Design overview

```
.map ──► qbsp ──► map.bsp + map.prt ──► vis ──► map.bsp ──► light ──► map.bsp + map.lit
          │ (leak -> map.pts)
```

- **Packages:** `internal/qbsp` (map parse, CSG, texinfo, hulls, leak,
  write BSP29/BSP2, write .prt), `internal/vis` (read .prt, portal flow,
  PVS write), `internal/light` (surface lightmaps, styles, .lit v1).
- **Binaries:** `cmd/qbsp`, `cmd/vis`, `cmd/light` — flags mirror ericw
  (`-bsp2`, `-2psb`, `-wadpath`, `-leaktest`, `-keepprt`, `-fast`,
  `-lit`, `-extra`, `-bounce`, `-scale`), omitting HL/Q2 targets.
- **Pipeline:** `mise run map-build` chains the tools over a `MAP=<name>`
  argument; `mise run build-tools` builds all three.
- **Parity harness (env-gated):** `internal/qbsp/parity_test.go`-family or a
  dedicated `tests/parity_ericw_test.go` using
  `ERICW_TOOLS_DIR` (default `/home/darkliquid/Projects/ericw-tools/build-linux`);
  skipped when absent.

---

## Task 1 (M0): `.map` parser — `internal/qbsp/mapfile.go`

**Files:** Create `internal/qbsp/doc.go`, `mapfile.go`, `mapfile_test.go`,
`testdata/*` (fixtures + `.gitignore` allow rules).

- [ ] **Step 1: Write the failing tests.** Parse fixtures covering:
  - **Standard QuakeEd brushes**: `( p1 ) ( p2 ) ( p3 ) texture shiftX shiftY rot scaleX scaleY`.
  - **Valve 220 brushes**: same + axis form
    `texture u v rot scale [ux uy uz uofs] [vx vy vz vofs]` (texdef has
    `shift, rotate, scale, axis[2×3]`).
  - Entity lump `{ "classname" … }` (comments, quotes, braces in values),
    multi-brush entities, `patchDef2`/`mesh` lines skipped (Quake qbsp
    ignores primitives), unclosed braces/strings → errors.
  - **Precision**: plane points round-trip float32 (the engine's plane
    format) without folding distinct planes together.
  - Style conversion: same brush re-parsed with `quaked` vs `valve_220`
    base format (`convert_to` behaviour).
  Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestParseMap -count=1`
- [ ] **Step 2: Implement.** `ParseMap(r io.Reader) (*Map, error)` →
  `Map{Entities; Brushes}`; `BrushFace{Points[3]Vec3; TexName; TexDef}`;
  `TexDef` covers the ericw `texdef_quake_ed_t`/`texdef_valve_t` shapes.
- [ ] **Step 3: Fixtures** embedded via `//go:embed` (hermetic); `.gitignore`
  rules `!/internal/qbsp/testdata/` + `!/internal/qbsp/testdata/*.map`.
- [ ] **Step 4: Corpus feasibility.** Import 2–3 small **testmaps**
  (`testmaps/box*.map`, `q1_light_black.map` is huge — prefer the smallest
  q1 fixtures) as later parity fixtures; license note: ericw-tools is GPL-2,
  testmaps authored by the project — add an attribution note in the
  testdata README.

## Task 2 (M1): qbsp BSP29 writer, hulls, leaks — `internal/qbsp`

**Files:** Create `plane.go`, `texinfo.go`, `brush.go`, `csg.go` (solidbsp),
`faces.go`, `hull.go`, `outside.go` (leak), `writebsp.go`, `prtfile.go`,
`qbsp_test.go`, `cmd/qbsp/main.go` (+ test).

- [ ] **Step 1: Write the failing tests** (each against `LoadTree`/loader):
  - **Round-trip smoke**: `box.map` → `LoadTree` → 15 lumps, world model
    with 4 `HeadNodes`, every face `lightofs == -1` before light.
  - **Structural invariants**: node/clipnode children resolvable (leaf
    `-(leaf+1)` / `0xFFFF` / contents), surfedge→edge→vertex chains closed,
    marksurf ranges, leaf contents ∈ known set; **clipnode count < 0xfff0**.
  - **Collision**: a `func_wall` brush entity gets `*N` model clipnodes;
    tree walk at a point test stops at the wall (drive clipnodes directly).
  - **Leak**: sealed box → clean; box with a hole → leak report + `.pts`
    trail (compare point count/ordering loosely vs ericw `base1leak.pts`);
    `-leaktest` → exit 1.
  - **PRT output**: hull-0 world `box.prt` parses back via the same
    prtfile writer/reader; cross-fed to the reference `vis` binary in the
    parity harness.
  - **Determinism**: byte-identical on repeat; limits errors at
    `MaxMap*`.
- [ ] **Step 2: Implement** the pipeline, mirroring ericw stage order:
  1. Parse map → entity/brush split; `func_detail*` classification;
     `_hulls` per-entity (default generate all).
  2. Texture/WAD table: dims from `-wadpath` wads via `internal/image.LoadWad`
     (test.wad-format WAD2), else warning + 16×16.
  3. **texinfo** (`texture_axis_t`): reproduce the exact QuakeEd
     `baseaxis[18]` table (floor/ceiling/west/east/south/north with snapped
     normal + xv/yv) + valve-220 axis overrides + `TEX_SPECIAL` for
     sky/slime names.
  4. **solidbsp CSG** per entity × hull: bounds box → split per brush plane
     → leaf contents; `AUTO`/`FAST`/`PRECISE` split selection (FAST brush
     entities, PRECISE world second pass).
  5. **Portals** (`MakeTreePortals` equivalent, shared with the .prt writer)
     → **FillOutside BFS from void** (outside_distance) → world rebuild
     with PRECISE + refill → `MarkVisibleSides` → **MakeFaces**
     (faces/edges/surfedges/vertexes) → `PruneNodes`.
  6. **Leak**: void-reachable entity → `.pts` trail + failure modes above.
  7. **WriteBSP29**: 15 lumps in canonical order; then hulls 1+2 clipnode
     trees (player/body per classic hull sizes; engine collision drives
     hulls 0–2); `visleafs` count; empty vis/lighting bumps.
- [ ] **Step 3: CLI** — `cmd/qbsp map.map [-o out.bsp] [-wadpath …]
  [-leaktest] [-keepprt] [-bsp2 (Task 3)] [-scale n] [-omitdetail*]` with
  ericw-style logs (entities/brushes/faces/hulls/leak status).
- [ ] **Step 4: Engine smoke (asset-free)** via the `sdk` pattern:
  pack the compiled BSP into `preMountedPaks`, `CmdMap("box")`, assert
  `Server.Active` + world model leaf/plane counts.
- [ ] **Step 5: Parity vs ericw** (harness, env-gated): compile
  `testmaps/q1_light_black.map` with both qbsps (same wads); compare
  **bspinfo lump-count table** (exact where deterministic, ±tolerance for
  merge/tjunc-sensitive face counts); assert our `.prt` is loadable by
  `vis/vis` and their `.prt` by ours.

## Task 3 (M2): BSP2 output — `internal/qbsp/writebsp2.go`

- [ ] **Step 1: Tests.** Same map with `-bsp2` → loader's DL2 path parses
  with float leaf bounds + uint32 marksurfaces; structural invariants
  preserved (signed-int32 children, leaf `-1`); `-2psb` skipped with a
  "not supported" error (ericw supports it; our engine reads it, but the
  writer targets BSP29 + BSP2 only in v1).
- [ ] **Step 2:** Version-specific encoders per the loader's expectations
  (`dsNodeSize`/`dl1NodeSize`/`dl2NodeSize`, leaf bounds int16 vs float,
  clipnode int16 vs int32).
- [ ] **Step 3:** Parity: `-bsp2` result loads in our engine AND in ericw
  `bspinfo` (cross-validated).
- Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestBSP2 -count=1`

## Task 4 (M3): vis / PVS — `internal/vis`

**Files:** Create `doc.go`, `prtfile.go` (read+write interop),
`portal.go` (winding clip/test), `flow.go`, `pvs.go`, `vis_test.go`,
`cmd/vis/main.go` (+ test).

- [ ] **Step 1: Tests** (small maps with known geometry; also cross-fed
  ericw `.prt` files):
  - Portal extraction + winding math: rooms/doorway → portal spans the
    doorway exactly; sealed room → no void portals.
  - **PVS semantics**: walled rooms don't see each other; open door ⇒
    see; a leaf sees itself; symmetry holds (`A sees B ⇔ B sees A`).
  - **Row format**: `visofs` rows RLE in `DecompressVis` format, exact
    `(leaves+7)/8` bytes; no portals ⇒ all-visible; `-fast` ⇒ all-visible.
  - **Interop:** our `vis` consumes ericw-qbsp `.prt` (and vice versa) and
    produces rows the engine decompresses; harness compares PVS bytes vs
    `vis/vis` output on the same `.prt` (identical when `-test`-equivalent
    target ratios line up, else containment).
- [ ] **Step 2: Implement** — `LoadPrtFile`, portal-vs-portal visibility
  (winding intersection + `-visdist`/`-targetchecks` knobs), `PortalFlow`
  recursion, leaf-cluster mapping (PRT1/PRT2), RLE row writer.
- [ ] **Step 3: Engine smoke** — `LeafPVS`/`FatPVS` sane on compiled map.
- Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/vis -count=1`

## Task 5 (M4): light / lightmaps + `.lit` — `internal/light`

**Files:** Create `doc.go`, `surface.go` (extents/grid), `trace.go`
(shadow vs BSP), `light.go` (entities, styles, sun, bounce clamp),
`write.go` (Lighting lump + `.lit` v1), `light_test.go`,
`cmd/light/main.go` (+ test).

- [ ] **Step 1: Tests:**
  - **Grid sizing**: 64-unit face at 16 px/unit → expected luxel grid;
    `-scale`/`_lmscale` change density (match warpscale/atlas math).
  - **Direct lighting**: point light value/dist²·cosθ semantics; back-face
    and shadowed samples dark; occlusion ray-vs-BSP reliable on
    clipnodes.
  - **Styles**: entities with `style k` land in `styles[k]` and sample at
    the right offset in the `Lighting` lump.
  - **`.lit`**: `-lit` writes `QLIT`+v1 accepted byte-for-byte by
    `bsp.ApplyLitFile`; RGB of style-0 matches mono samples; unlit face ⇒
    `lightofs == -1`.
  - **Parity**: reference `q1_light_black.lit` sample values within a
    tolerance band on the same geometry/lights (algorithm differences in
    shadow/falloff tolerated; assert per-sample relative error < 20% and
    no "0 empty lightmaps"-style inversions; `.lit` sizes must match the
    `ApplyLitFile` contract exactly).
- [ ] **Step 2: Implement** — lightsurf gathering, `faceextents`,
  sample accumulation with shadow traces, ambient/sky terms, per-style
  lightmaps, Lighting-lump packing, `.lit` v1 writer. `-extra`/`-extra4`
  supersampling and single-bounce radiosity (`-bounce`, clamped
  colorbleed) are phase-2 within the task if time allows — direct + styles
  + `.lit` is the AC4 floor.
- [ ] **Step 3: Engine smoke** — `ExpandLightmapSamples` non-zero on a lit
  face; `bspdiag liquids`-style stats on compiled + lit output.
- Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/light -count=1`

## Task 6 (M5): pipeline, parity harness, docs

**Files:** Modify `mise.toml` (`build-tools`, `map-build`, `map-parity`),
Create `docs/MAP_COMPILING.md`, add `.gitignore` fixture rules, and a
`tests/parity_ericw_test.go` style harness (or `internal/*/parity_test.go`)
gated on `ERICW_TOOLS_DIR`.

- [ ] **Step 1: Pipeline task** — `MAP=box mise run map-build` chains
  qbsp→vis→light with per-stage logs; each tool also standalone.
- [ ] **Step 2: Parity harness** (env: `ERICW_TOOLS_DIR`, default
  `/home/darkliquid/Projects/ericw-tools/build-linux`, skip when absent):
  1. Same `.map` → both qbsps → **bspinfo lump-count tables** compared
     (tolerance policy per lump: nodes/leafs/edges exact-or-tolerance;
     faces tolerance due to tjunc/merge differences).
  2. `.prt` cross-feed: our vis on their prt, their vis on ours.
  3. Both `light -lit` → `.lit` byte-header + size contract + per-sample
     relative tolerance; leak `.pts` emitted for leaky fixtures.
  4. Engine headless: compiled map boots via `sdk` (no pak0).
  5. Baseline: `q1_light_black` reference table above is pinned as the
     target echo in the harness log.
- [ ] **Step 3: Docs** — `docs/MAP_COMPILING.md`: map format variants,
  `mise run map-build MAP=…`, `-wadpath` requirement, leak/`.pts`,
  `.lit -lit`, BSP2, limits, and parity harness usage
  (ERICW_TOOLS_DIR + how to regenerate reference outputs).
- [ ] **Step 4: Gates** — golangci-lint clean on new packages; `mise run
  verify` green; document the alignment deltas vs ericw in the harness
  output (which lumps differ and why).

---

## Acceptance criteria mapping & milestones

| M | AC | Scope | Key outputs |
|---|---|---|---|
| M0 | AC1 | `.map` parser (QuakeEd + Valve 220 styles, entity lump, convert_to) | `internal/qbsp/mapfile.go` + tests |
| M1 | AC2 | BSP29: solidbsp CSG, faces/edges, texinfo (`baseaxis[18]`), models, clipnode hulls 0–2, `FillOutside` leak + `.pts`, `.prt` writer | `internal/qbsp/*`, `cmd/qbsp` |
| M2 | AC2 | BSP2 output (float leaf bounds, 32-bit lumps) | `cmd/qbsp -bsp2` |
| M3 | AC3 | `.prt` interop + portal flow PVS, RLE rows, `-fast` | `internal/vis`, `cmd/vis` |
| M4 | AC4 | Direct + ambient lightmaps, styles, shadows, `.lit` v1 | `internal/light`, `cmd/light` |
| M5 | all | Pipeline task, parity harness vs ericw, engine smoke, docs | `mise.toml`, `docs/MAP_COMPILING.md`, parity tests |

## Testing strategy (summary)

1. **Reader-oracle round-trips**: every writer milestone → `LoadTree`/loader
   structural equality (the engine defines "valid").
2. **ericw parity (env-gated)**: reference corpus in `build-linux/` +
   `testmaps/`; bspinfo lump tables, `.prt` cross-compilation, `.lit`
   sample tolerance, leak `.pts`; skipped when `ERICW_TOOLS_DIR` absent.
   This replaces the earlier "no C tools" risk with a *measured alignment*
   policy (exact where deterministic, tolerance where algorithm choices
   differ — tjunc/merge/portal welding).
3. **Engine smoke, asset-free** via `sdk` headless + `preMountedPaks`.
4. **Conventions**: `TMPDIR=…/.tmp CGO_ENABLED=0 go test <pkg> -count=1`;
   deterministic byte-identical output everywhere; embedded fixtures.

## Risks & mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| CSG/solidbsp correctness | Broken collision/rendering | ericw `brushbsp.cc`/`csg.cc` as the reference; incremental stage gates; reader round-trip + collision point tests every milestone |
| Face/edge topology differences vs ericw (tjunc, merge, pruning order) | bspinfo tables differ | Tolerance-per-lump parity policy; our `.prt`/`.lit` interop proves the *semantic* contract even when counts drift |
| Leak detection divergence (flood directions, `floodentity`) | Maps wrongly rejected | `FillOutside` void-flood semantics copied; parity on `base1leak`/`q1_light_black` fixtures before accepting |
| PVS algorithm mismatch | Visible-set regressions | `-visdist`/`-targetchecks` knobs + cross-fed `.prt` containment tests vs ericw output |
| Lightmap math drift | Bright/banded surfaces | Reference `.lit` tolerance band; grid math unit tests; `.lit` v1 contract enforced by `ApplyLitFile` |
| `.gitignore` whitelist | Fixtures untracked | Explicit allow rules (`internal/qbsp/testdata/`, testmaps copies) with GPL-2 attribution notes |
| Scope creep (HDR lit, -lit2, -2psb, HL/Q2 targets, embree) | Bead drags | Explicit non-goals; follow-up bead for HDR/bounce/lightpreview integration |
| ericw licenses its sources GPL-2 | We must not copy code | Go tools are clean-room ports of the *algorithms* (id-origin formats), parity harness verifies behavior, never code |