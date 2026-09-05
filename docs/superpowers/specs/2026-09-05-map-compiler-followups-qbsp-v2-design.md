# Map Compiler V2 — Brush Submodels, Solidbsp Scale-Up, T-Junction Fixing, Light Styles: Design Specification

> **Bead:** `ironwail-go-rhg` · **Status:** spec (draft) · **Date:** 2026-09-05
> **Parent:** `ironwail-go-t63` plan
> `docs/superpowers/plans/2026-09-05-map-compiler-pipeline-qbsp-vis-light.md`
> (M4 phase 2 + documented v1 gaps)
> **Reference C (canonical, license GPL-2 — algorithms only, never code):**
> ericw-tools `qbsp/brushbsp.cc`, `qbsp/tjunc.cc`, `qbsp/writebsp.cc`,
> `qbsp/faces.cc`, `qbsp/merge.cc`, `qbsp/qbsp.cc`, `light/ltface.cc`,
> `light/bounce.cc`, `light/write.cc`, `light/entities.cc`.

## 1. Overview

The v1 compiler pipeline (qbsp → vis → light) is functional for small,
world-only, unlighted-fancy maps. This spec closes the four documented v1
gaps so real maps compile, render, collide, and light correctly:

| # | Gap | Current state | Outcome |
|---|---|---|---|
| 1 | Brush-entity submodels | World geometry only (`func_wall`/`func_door` brushes are dropped) | Per-entity BSP trees, `*N` model records, headnodes, face ranges; renders + collides |
| 2 | Solidbsp scale-up | Arrangement CSG is O(planes³), small maps only | Classic recursive solidbsp (brush splitting); real-sized maps (thousands of planes) compile in reasonable time/memory |
| 3 | T-junction fixing | Faces share vertices only where splitting happens; cracks on large coplanar runs | Edge-splitting tjunc pass; no crack artifacts |
| 4 | Light styles / sun / bounce | Direct point lighting only, single mono lightmap per face | `styles[4]` per-style lightmaps, sun/sky entities, clamped single-bounce radiosity |

**Acceptance criteria (from the bead):**

1. qbsp emits brush-entity submodels that the engine renders and collides with.
2. qbsp compiles a real-sized map (thousands of planes) in reasonable time/memory.
3. Compiled faces have no t-junction cracks.
4. light produces styled/sunlit/bounced lightmaps the engine renders.

## 2. Current State Audit (verified 2026-09-05)

| Area | State | Evidence |
|---|---|---|
| CSG | Arrangement-based, O(planes³) | `internal/qbsp/csg.go` — `buildArrangement` (csg.go:148) enumerates every non-empty sign combination; `assignContents` (compiler.go:358), `assemble` (compiler.go:81) |
| Models | Single world model, hardcoded 64-byte record | `serializeModels` (writebsp.go:402) — fixed `headnode[0]=0`; `firstface/numfaces` written as 0; `len(numFaces)` parameter unused |
| Entities | World brushes only | `collectBrushes` (compiler.go:142) iterates `m.Entities[0]`; brush entities in `m.Entities[1:]` never enter the `worldBrush` list |
| Hulls | One expanded tree shared by hulls 1+2 | `buildHullClipNodes` (hull.go:27) — still arrangement-based, world brushes only, single root |
| Faces/edges | Direct emission from arrangement facets; no tjunc | `faces.go` `makeFaces` + `edgeTables` (writebsp.go:15); `poly.go` `windingRemoveColinear` |
| Light | Direct point lights, mono, style-less | `internal/light/light.go` — `Bake` (light.go:42), `directLight` (light.go:107); single byte/sample; `WriteLit` (light.go:136) writes R=G=B from mono. `internal/light/trace.go` — `TreeTracer` ray-vs-BSP shadow |
| Light BSP side | Face styles parsed but never produced | Engine `DFace.Styles[4]` read (bsp/loader.go:338,357); compiler writes `styles` zeros (writebsp.go:340,350) |
| Engine consumption | Ready for submodels | Renderer: `renderer_gogpu_worldstate.go:382` `ensureBrushModelGeometry(submodelIndex)`, `BuildModelGeometry(tree, submodelIndex)`; `entity_types.go:37` `BrushEntity`. Server: `server_net_main.go:58` precomputes `*0..*n` inline submodel names; collision drives hulls 0/1/2 via `headnode[4]` (`internal/bsp/tree.go`, `internal/server/collision`) |
| Formats | BSP29 + BSP2 writers, `.prt` PRT1, `.lit` QLIT v1 | writebsp.go; prtfile.go; matching `ApplyLitFile` (bsp/lit.go) |
| Reference tools | Present | ericw binaries in `build-linux/`; `testmaps/` corpus; parity harness env-gated on `ERICW_TOOLS_DIR` (`internal/qbsp/parity_test.go`) |

## 3. Target Architecture

```
.map ─► qbsp ─► map.bsp + map.prt ─► vis ─► map.bsp ─► light ─► map.bsp + map.lit
          │  per entity: solidbsp tree ─► submodel records (*1..*N)
          │  faces pass ─► tjunc fix ─► edges/surfedges/vertexes
          │  (leak -> map.pts)
```

Shared lumps (vertexes, edges, surfedges, faces, texinfo, clipnodes, nodes,
leafs, marksurfaces) accumulate **across all models** in model order; each
`dmodel_t` records its owned ranges via `headnode`, `firstface`/`numfaces`,
`mins`/`maxs`/`origin`, `visleafs` — exactly the loader's `DModel` contract
(bsp/bsp.go:164).

### Package/file change map

| File | Change |
|---|---|
| `internal/qbsp/brush.go` (new) | `bspBrush` — winding polyhedron + content + original brush; volume, plane-side classify, split, chop (CSG subtract) |
| `internal/qbsp/solidbsp.go` (new) | Recursive `BuildTree_r`, split-plane selection (`AUTO`/`FAST`/`PRECISE`), `SplitBrushList`, `LeafNode`; node/leaf emission with `firstface`/`numfaces` per node |
| `internal/qbsp/csg.go` | **Deleted** after solidbsp lands (arrangement, `windingFromBoxPlane` poly clip helpers move to `poly.go`) |
| `internal/qbsp/compiler.go` | `collectBrushes` → per-entity brush lists (world + brush entities); pipeline re-ordered to ericw stage order (solidbsp → portals → FillOutside → world rebuild → MarkVisibleSides → MakeFaces → PruneNodes); per-entity tree assembly |
| `internal/qbsp/hull.go` | Per-model clip trees into the shared clipnodes lump; hulls 1/2 per model `headnode[1]` |
| `internal/qbsp/writebsp.go` | `serializeModels` → slice of 64-byte records (bounds shrunk by 1 like Q1, origin from entity), per-model `headnode[0..3]`, `firstface`/`numfaces`; face + model lumps sized per new pipeline |
| `internal/qbsp/tjunc.go` (new) | Edge-insertion tjunc pass per model; vertex/edge table rebuild |
| `internal/qbsp/entities.go` (new) | Entity classification (`worldspawn`, `func_*` brush, point); `outputmodelnumber` allocation + `*N` naming; `origin`/`angle` keys |
| `internal/light/light.go` | `Bake` → per-style lightmap accumulation (`styles[4]`); `Light` gains `Style`, `Color`; sun + sky term; bounce pass |
| `internal/light/bsp.go` | Parse per-model surfaces (`modelinfo` equivalent), texture lump colors for bounce; write `styles[]`+`lightofs` per face |
| `internal/light/bounce.go` (new) | Single-bounce radiosity (`-bounce`), emissive surface list |
| `internal/light/sun.go` (new) | Sun entity + `sun_mangle`/`sunlight`/`sunlight_color` worldspawn keys |
| `internal/light/write.go` (new) | Lighting lump layout: per face, per style `W·H` blocks; `.lit` from style-0 samples only |
| `cmd/qbsp`, `cmd/light` | Flags `-merge`, `-bounce n`, `-sun`, `-extrasamples`, `-lmscale` |

## 4. Feature 1 — Brush-Entity Submodels

### 4.1 Entity classification

After parse, `compiler.go` classifies each map entity (ericw `qbsp.cc`
semantics, `entity.outputmodelnumber` / `*{}` naming at qbsp.cc:1035-1045):

- **index 0** — `worldspawn`: brushes are world geometry; model 0.
- **brush entity** (`func_wall`, `func_door`, `func_breakable`, `func_train`,
  `func_plat`, `func_rotating`, any entity with `Brushes` and a `classname`
  in the `*_`/`func_*` solid set, or explicitly `"model"`-less with brushes):
  gets model `*1..*N` in encounter order. Entities with **no** brushes and no
  `model` key are point entities — never in the model table. A `model` key
  naming an existing submodel (e.g. `"model" "*2"` on a trigger) aliases it.
- **omit detail**: `func_detail*` do not become submodels and never generate
  clip hulls; they are CSG'd into the enclosing structural solid (world or
  brush entity) per ericw `detail` handling.

### 4.2 Compile per entity

For each solid entity (world + each brush entity):

1. Build its brush list (planes already deduped into the shared table by
   `planeIndexFor`).
2. Run the solidbsp tree build (Feature 2) on its brushes — **hull 0 only**
   for submodels in v2 (world gets hulls 0/1/2 as today; submodel clip is
   through the world tree, matching classic Quake where brush entities were
   hull-0-only depending on `_hulls`). `_hulls` per-entity key opts into
   submodel hulls when present.
3. Emit its nodes/leafs/faces into the shared lumps; record
   `headnode[0]`, `firstface`, `numfaces`, `visleafs`, `mins`/`maxs`
   (shrunk by 1 unit, `uses_shrunk_model_bounds` semantics — the Go engine
   compensates in `Mod_LoadSubmodels` equivalently), `origin` from the
   entity's `origin` key (default (0,0,0)).
4. Serialize all model records in order, index 0 = world.

### 4.3 Rendering + collision contract

The engine already consumes exactly this:

- **Render:** `renderer_gogpu_worldstate.go:382` builds brush-model geometry
  per `submodelIndex` from `tree.Models[index]` headnode + face range;
  `entity_types.go:37` `BrushEntity` maps `*N` → submodel.
- **Collide:** server resolves `*N` → model and traces hulls via
  `headnode[4]` (`internal/bsp/tree.go:716`, `internal/server/collision`).
- **Server test:** spawn `func_wall` with a trigger; assert
  `SV_RecursiveHullCheck` blocks on the submodel and the model's
  `mins/maxs` are registered.

### 4.4 Acceptance test

`sdk`-style headless boot (asset-free, `preMountedPaks`): a map with a
`func_wall` compiles to ≥2 models; renderer `BuildModelGeometry(1)` returns
faces; a point trace at the wall's position stops against it.

## 5. Feature 2 — Solidbsp Scale-Up

### 5.1 Why

`buildArrangement` (csg.go:148) enumerates all 2^k non-empty halfspace
combinations in a bounding box: cell count explodes combinatorially with
plane count, so real maps (thousands of planes) are infeasible. Classic
solidbsp instead recursively partitions a **brush list** (winding polyhedra)
by planes chosen from the list, splitting brushes as needed — O(brushes ·
splits), independent of total plane count.

### 5.2 Data structures (`internal/qbsp/brush.go`)

```go
type bspBrush struct {
    fins    []winding   // one per face, outward normals
    planes  []int       // plane-table indices for each fin
    content int32
    bounds  [2]vec3     // for cheap rejection + node bounds
    orig    MapBrush    // debug / texture attribution
}
```

Plane-side classification and splitting follow ericw `brushbsp.cc`
(`TestBrushToPlanenum`/`SplitBrush`, SplitBrush at brushbsp.cc:409,
`PSIDE_FRONT|PSIDE_BACK|PSIDE_BOTH|PSIDE_FACING`). `ChopBrushes`
(brushbsp.cc:1447) subtracts later brushes from earlier ones so overlapping
solid brushes resolve by last-wins (water-in-pit, brush entities over
world), replacing the voxel-ish `assignContents` point test.

### 5.3 Tree build (`internal/qbsp/solidbsp.go`)

```
BuildTree_r(node, brushList, splitType):
  if brushes empty → CONTENTS_EMPTY leaf
  if all brushes have same content
     and no brush splits others (intersect test) → solid leaf of that content
  pick split plane:
    AUTO    → ChooseMidPlaneFromList (balance front/back counts + splits)
    FAST    → first plane that splits the list (brush entities)
    PRECISE → SelectSplitPlane (score = balance, splits, axial preference,
              min split surface area)            [brushbsp.cc:938]
  SplitBrushList(brushList, plane) → front, back   [brushbsp.cc:1122]
  emit node (planenum, bounds, firstface/numfaces)
  recurse into children
```

Split policy matches ericw: `FAST` for brush entities, `PRECISE` for the
world's first pass, `PRECISE` again for the post-`FillOutside` world rebuild.
Node output records `firstface`/`numfaces` (the faces-in-bounds list in
`ExportDrawNodes`), which the loader expects for the node lump's face span
(bsp29=uint16, bsp2=uint32 — already handled by `serializeNodes`; only the
`firstface/numfaces` fields currently hardcoded to 0 need wiring, writebsp.go:227-228, 242-243).

### 5.4 World pipeline re-order (ericw `qbsp.cc` order)

1. Solidbsp world (PRECISE) → tree with leaf contents.
2. `MakeTreePortals` equivalent: portal windings on leaf boundaries
   (reuses `internal/qbsp/prtfile.go` winding/clip code).
3. **`FillOutside`** — BFS flood from the void assigning `outside_distance`
   (currently `internal/qbsp/leak.go`; keep semantics, re-root on the new
   tree). Leak = content entity in a void-reachable leaf; `.pts` trail by
   walking decreasing `outside_distance` (leak.go stays).
4. World rebuild (PRECISE) with the void filled solid (`FillOutside`
   accelerates the flood by pruning solid children), then refill.
5. `MarkVisibleSides` — faces backing onto solid/void get `SURF_NODRAW`
   (new: needs a `flags` on `outFace`; only non-draw faces emit lightmaps
   later; ericw keeps them in `numfaces` but marks nodraw via texture
   `TEX_SPECIAL`/`SURF_NODRAW` — renderer skips them).
6. `MakeFaces` — from solidbsp brushes (the surviving face windings of each
   leaf's brush pieces), deduped across the shared table, then tjunc
   (Feature 3), then `edgeTables`.
7. `PruneNodes` — collapse empty-space single-child chains
   (tree.go `renumberLeaves` stays for PVS row math).

### 5.5 Hulls

`buildHullClipNodes` (hull.go:27) switches from arrangement to the same
solidbsp over **expanded brush planes** (the existing ±16/±32 extents math
stays). Per-model: world and (when `_hulls` present) submodels get their own
clip tree appended to the shared clipnodes lump, `headnode[1]` pointing at
their root — clipnode lump is one flat array across models (clipnode count
< `0xfff0` BSP29 limit enforced per existing writer).

### 5.6 Scale acceptance test

- Unit: generate a brush field (e.g. 20×20×20 cell grid = 8000 planes via a
  synthetic `Map`), assert compile time < a budget (e.g. 10 s) and memory
  sane; arrangement CSG on the same input is the control (expected to fail
  or blow memory — benchmark only, not a CI gate).
- Parity: `testmaps/q1_light_black.map` compiles; bspinfo lump table within
  tolerance vs ericw (faces drift by merge/tjunc policy per the parent
  plan's tolerance policy).

### 5.7 Removal

Delete `internal/qbsp/csg.go` (arrangement, region, facet, signature,
mirror) once solidbsp paths replace every caller; `poly.go` winding clip
primitives are reused. Keeps the tree small and prevents accidental
re-adoption of the exponential path.

## 6. Feature 3 — T-Junction Fixing

### 6.1 Problem

Without tjunc, two coplanar faces that divide a shared edge at different
vertex sets leave a T-intersection: the longer edge rasterizes with a
vertex-free span the neighbor's vertex pokes into — classic crack or
interpolated-tear artifacts (and lightmap seams).

### 6.2 Algorithm (`internal/qbsp/tjunc.go`, mirrors `qbsp/tjunc.cc`)

1. **Collect the edge universe:** for every face in a model, walk its
   polygon edges → candidate vertex points `(start, end)` that may need
   splitting.
2. **Find insertions** (`FindEdgeVerts_FaceBounds`, tjunc.cc:228): spatial
   hash over face bounds; for a face edge, gather all vertices that lie on
   the edge line within tolerance (PointOnEdge, tjunc.cc:56). Only faces
   with tjunc interaction are tested (`HasTJuncInteraction` — different
   contents/planes or non-identical planes), so coincident duplicate faces
   don't self-split.
3. **Split** (`SplitFaceIntoFragments`, tjunc.cc:248): insert the collected
   edge points into each polygon, producing fragments whose vertices are all
   atomic edge endpoints (no vertex lies mid-edge of any other fragment).
4. **Retopologize** (`RetopologizeFace`, tjunc.cc:606): minimum-weight
   triangulation into fans (`find_best_fan`, `minimum_weight_triangulation`)
   that keeps the *fragment boundary as a polygon*; `FixFaceEdges`
   (tjunc.cc:763) re-emits. For BSP purposes only the added edge vertices
   matter — fan triangulation is a renderer concern, but ericw emits
   super-face fans directly; we keep polygon output + added vertices
   (renderer triangulates), which is what ironwail expects from classic
   qbsp output.
5. **Rebuild edge tables** after splitting (new vertexes appended; vertex
   dedup via exact-position hash on float64 → keeps `edgeTables` total).

### 6.3 Node association

The pass runs per model after `MakeFaces` and before `edgeTables`,
reusing the per-node `firstface/numfaces` spans (a face belongs to one
node); tjunc may emit faces into the same node's span, so reassign spans
after the pass.

### 6.4 Acceptance test

Two stacked brushes sharing a coplanar seam (one 2 units wide, the other 1
unit wide offset by 0.5) → compile → assert no vertex of any face lies on
the interior of another face's edge (exhaustive pairwise test on the
compiled face polygons); renderer smoke: `BuildWorldGeometry` produces no
degenerate triangles.

## 7. Feature 4 — Light Styles / Sun / Bounce

### 7.1 Styles (`internal/light`)

- `Light` gains `Style int` (0..31; from entity `style` key, default 0) and
  `Color [3]float64` (from `_color`/`color` key normalized to 0..255).
- Per face, collect the distinct styles of lights that can affect it (max
  4, first 4 win — classic limit); set `face.styles[0..n]`.
- **Lighting lump layout change** (the format the engine's
  `ExpandLightmapSamples`/`lightofs` contract expects): for each lit face,
  one `W·H` mono block **per style**, laid out consecutively; `lightofs`
  points at the style-0 block; face `styles[k]` names which cvar-driven
  style each block animates with. Unlit or no-draw faces keep `lightofs=-1`.
  Lightmap gaps use `Lightmap_AllocOrClear` zero-fill semantics
  (ltface.cc:733) — blocks are never merged across faces.
- `Bake` becomes: for each face, for each style, accumulate only lights of
  that style at each luxel (`GetLightContrib`/`Lightmap_ForStyle`,
  ltface.cc:763). Direct term, shadow trace, 255 clamp unchanged.

### 7.2 Sun (`internal/light/sun.go`)

- Input: `sun` entity (or worldspawn `sunlight`, `sun_mangle`,
  `sunlight_color`) — `sun_mangle` yaw/pitch → direction; `sunlight`/`light`
  intensity; color from `sunlight_color`/`_color`.
- **Sky faces become light sources** (`LightFace_Sky`, ltface.cc:1378):
  every sample on a non-sky face accumulates
  `sun.intensity · max(dot(normal, sunDir), 0) · diffuseColor(sample point)`
  where the diffuse color is the **sky brush face's texture color**
  (default 255,255,255 for the compiler's placeholder miptex).
- Sky faces themselves get no lightmap (`TEX_SPECIAL`/`Sky` skip stays).
- Sun is style 0; `-sun` flag gates it.

### 7.3 Bounce (`internal/light/bounce.go`)

- **Emissive surfaces:** faces whose texture has nonzero brightness
  (`texlight`/ `_texlight` key or texture average when `-wadpath` textures
  are available; the placeholder 16×16 zero miptex defaults to
  `_texlight`/emissive keys only) accumulate into an emissive surface list
  (light.cc `EmissiveLightSurfaces`).
- **Single bounce** (`bounce.cc`, clamped colorbleed): for each lit
  surface, treat the baked direct sample as an area light; shoot
  `-bounce 1` (or n) gather rays from each sample against the BSP tracer;
  add the received color × face albedo (texture color/255) to the sample.
  Clamp accumulation to 255.
- Gate: `-bounce n` (default 0); `-bounce` without n = 1.

### 7.4 `.lit` sidecar

`.lit` v1 carries **style-0 samples only** (RGB triplets), matching ericw's
`Lightmap_Save`/litfile contract and `bsp.ApplyLitFile` validation. With
styles, style-0 RGB comes from the style-0 block; styled blocks remain mono
in the Lighting lump — the engine's animated-style path is mono anyway.

### 7.5 Acceptance tests

- **Styles:** two `light` entities, one `style 1` and one `style 3`, on a
  lit face → `face.styles == [0,1,3,0]`, Lighting lump = 3 consecutive
  W·H blocks, style-1 block dark where only style-0 shines; animated
  (`lightstyle 1`) cvar changes render output in the engine smoke.
- **Sun:** box lit only by a `sun` entity → top face bright, bottom face
  dark; ceiling (sky) faces unlit with `lightofs=-1`.
- **Bounce:** `-bounce 1` on a closed box with one emissive surface →
  opposing wall receives non-zero light where direct light was zero;
  clamped to 255.
- **Parity (env-gated):** `q1_light_black.map` re-lit with both lighters;
  `.lit` sizes match exactly; per-sample relative error < 20% on the
  reference `.lit`; bspinfo reads the lit BSP.

## 8. CLI & Pipeline Changes

| Tool | Flag | Meaning |
|---|---|---|
| `cmd/qbsp` | `-merge` | Merge coplanar faces before tjunc (reuses the shared winding code; off by default in v2 — parity tolerance policy already absorbs the delta) |
| `cmd/qbsp` | `-omitdetail` | Drop `func_detail*` entirely (parity with ericw testing flag) |
| `cmd/light` | `-bounce [n]` | Radiosity bounces (default 0) |
| `cmd/light` | `-sun` | Enable sun entity lighting |
| `cmd/light` | `-lit` | (existing) writes `.lit` v1 — now style-0 RGB |
| `cmd/light` | `-scale n` | (existing) luxel density — now also drives `_lmscale` per-entity |

`mise run map-build MAP=…` unchanged; logs now print per-model stats
(models, faces/model, tjunc splits, styles/face, bounce surfaces).

## 9. Testing Strategy

1. **Reader-oracle round-trips** after each writer change: `LoadTree` +
   loader structural equality on output; every face `styles`/`lightofs`
   decode to what the compiler wrote (bsp/loader.go:338).
2. **Engine smoke, asset-free** (sdk + `preMountedPaks`): submodel render +
   collide (Feature 1 AC), PVS rows (unchanged), animated style render
   (Feature 4).
3. **Unit geometry:** submodel face ranges disjoint; tjunc no-T-invariant;
   solidbsp tree contents correctness on overlapping/water-in-pit brush
   arrangements; per-style block layout; sun cosine falloff; bounce
   monotonicity/clamp.
4. **Scale benchmark:** synthetic plane fields (Feature 2.6) — compile-time
   budget gate, not a timing flake.
5. **ericw parity (env-gated, `ERICW_TOOLS_DIR`):** bspinfo lump tables with
   the parent plan's per-lump tolerance policy (faces tolerance-only);
   `.prt` cross-feed unchanged; `.lit` size == ericw and sample relative
   error < 20%; full `q1_light_black` baseline pinned in harness logs.
6. **Conventions:** `TMPDIR=…/.tmp CGO_ENABLED=0 go test <pkg> -count=1`;
   deterministic byte-identical repeat output; embedded/embedded-fixture
   testdata only; `.gitignore` allow rules for any new fixture files.

## 10. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Solidbsp correctness (overlaps, coplanar coincident faces) | Broken render/collide | ericw `brushbsp.cc` + `csg.cc` as behavioral oracle; step gates: tree contents unit tests before faces; collision point tests per milestone |
| Submodel clip semantics drift (classic Quake had hull-0-only brush entities) | Players stuck/clip through func_walls | `_hulls` key honored; parity on a `func_door` testmap vs ericw |
| Tjunc fan-vs-polygon choice | Triangle seam artifacts | Renderer's triangulator consumes polygons with added vertices; renderer smoke asserts no degenerate tris |
| Style-lump layout wrong (`lightofs` contract) | Broken/black lighting, engine crash on `ExpandLightmapSamples` | Layout property tests vs loader decode; engine smoke renders a styled face; `.lit` byte contract enforced |
| Bounce/sun lighting divergence vs ericw | Harsh or black walls | Reference `.lit` tolerance band; `-bounce 0` default keeps parity surface small |
| Porting while `csg.go` deletion churns | Long unbuildable window | Land solidbsp behind the old path (`Compile` flag), flip default, then delete; tests gate the flip |
| Scope creep (phong, lightgrid, -lit2, -extra shading, HDR) | Bead drags | Explicit non-goals below; separate follow-up beads |

## 11. Non-Goals & Follow-ups

Out of scope for `ironwail-go-rhg` (tracked separately or deferred):

- HDR/`-lit2`/`-lithdr`, `-extra`/`-extra4` supersampling, phong shading,
  lightgrid, `-2psb`, HL/Q2 targets, BSPX brushes lump (engine already
  tolerates its absence), `lightpreview` integration, BSP2 → `-vis` interop
  beyond what the engine reads.
- Per-submodel PVS membership (world-only PVS is the v1 contract; submodel
  `visleafs` counts are written, rows remain world).
- Automatic texture-color bounce without `-wadpath` (placeholder miptex
  stays 16×16; emissive requires keys).