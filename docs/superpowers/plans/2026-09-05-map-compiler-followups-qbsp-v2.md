# Pure-Go Map Compiler V2 — Brush Submodels, Solidbsp, T-Junction Fixing, Light Styles: Implementation Plan

> **Bead:** `ironwail-go-rhg` · **Status:** plan (draft) · **Date:** 2026-09-05
> **Spec:** `docs/superpowers/specs/2026-09-05-map-compiler-followups-qbsp-v2-design.md`
> **Parent plan:** `docs/superpowers/plans/2026-09-05-map-compiler-pipeline-qbsp-vis-light.md`
> (M4 phase 2 + known v1 gaps)
> **For agentic workers:** implement task-by-task, checkbox (`- [ ]`) syntax
> for tracking, failing test first per task step, then implementation, then
> engine smoke / parity gates at each milestone.

**Goal:** Close the four v1 compiler gaps so real maps compile, render,
collide, and light correctly: (1) brush-entity submodels, (2) solidbsp
scale-up replacing the O(planes³) arrangement CSG, (3) t-junction fixing,
(4) light styles / sun / bounce.

**Architecture:** `internal/qbsp` gains a winding-polyhedron solidbsp core
(`brush.go`, `solidbsp.go`) that replaces `csg.go`; the compiler pipeline
re-orders to ericw stage order and compiles *per entity* into shared lumps
with per-model records. `internal/qbsp/tjunc.go` fixes cracks before edge
emission. `internal/light` accumulates per-style lightmaps, adds sun/sky
and single-bounce radiosity. Nothing changes in the engine: renderer and
collision already consume submodels (`renderer_gogpu_worldstate.go:382`,
`server_net_main.go:58`) and `DFace.Styles[4]` (bsp/loader.go:338).

**Tech Stack:** Go 1.26, pure Go, existing `internal/qbsp`, `internal/light`,
`cmd/qbsp`, `cmd/light`, `internal/bsp` loader oracle, `mise run map-build`,
ericw-tools parity harness (`ERICW_TOOLS_DIR`-gated).

---

## Implementation status (M0–M5, populated as work lands)

- **M0 (solidbsp core) — landed.** `internal/qbsp/brush.go` +
  `solidbsp.go`: `bspBrush` polyhedra, `ChopBrushes` (last-wins solid
  subtraction), `SplitBrush` with corrected cap normals, volume-checked
  `SelectSplitPlane` (ericw-style scoring, table-normalized splits),
  `BuildTree_r` recursion with exact leaf regions, per-leaf facet/neighbour
  enumeration (`world.go`), node `firstface/numfaces`, viewpoint-aligned
  engine descent (plane `planenum` resolved via the plane table at build
  time). `internal/qbsp/csg.go` deleted. Tests: `TestSolidBSPBrushSplit`,
  `TestSolidBSPEngineDescent`, `TestSolidBSPCorridorDescent`,
  `TestSolidBSPMatrixScale`.
- **M1 (brush submodels) — landed.** `entities.go`: entity classification,
  per-entity model groups, `"model" "*N"` keys; per-model solidbsp trees
  into shared node/leaf/face/clip lumps; per-model `DModel` records
  (headnode, face ranges, shrunken bounds, origin, per-model clip hulls).
  Test: `TestCompileSubmodel`.
- **M2 (tjunc) — landed.** `internal/qbsp/tjunc.go` coplanar edge-vertex
  insertion (spatial hash, on-edge tolerance, dedup preserving collinear
  vertices). Test: `TestTJunctionNoCrack`.
- **M3 (light styles) — landed.** `internal/light`: per-face `styles[4]`
  (255-padded), per-style W·H blocks in the Lighting lump, style/`_color`
  entity keys, face style patching in `cmd/light`, `.lit` = style-0 RGB.
  Tests: `TestStyleRouting`, `TestStyleLitSidecar`.
- **M4 (sun + bounce) — landed.** `internal/light/sun.go` (`sun` +
  worldspawn keys, `-sun`), `bounce.go` clamped single-bounce radiosity
  (`-bounce n`, shadow-traced surfel gather). Tests: `TestSunLightTopFaces`,
  `TestBounceLightReachesShadowedWall`.
- **M5 (pipeline, parity, docs) — landed.** `cmd/qbsp -omitdetail`,
  `cmd/light -sun -bounce`; docs `MAP_COMPILING.md` updated; e2e verified
  (qbsp→vis→light on a func_wall map with styles + bounce); lint clean;
  `mise run verify` green. ericw parity harness remains env-gated as before.

## Known anchor points in current code (verified 2026-09-05)

| Concern | Location |
|---|---|
| Arrangement CSG (to replace) | `internal/qbsp/csg.go:148` `buildArrangement`; callers `compiler.go:347` `buildWorldArrangement`, `hull.go:79` |
| Content resolution | `assignContents` (compiler.go:358) → replaced by `ChopBrushes`-style solid subtraction |
| World-only brush collection | `collectBrushes` (compiler.go:142) iterates `m.Entities[0]` only |
| Single hardcoded model record | `serializeModels` (writebsp.go:402); `headnode[0]=0`, `firstface/numfaces` zeros |
| Node face spans hardcoded 0 | `serializeNodes` (writebsp.go:227-228 BSP2, 242-243 BSP29) |
| Hulls (arrangement-based, world only) | `buildHullClipNodes` (hull.go:27) |
| Face/edge emission | `faces.go` `makeFaces`; `edgeTables` (writebsp.go:15) |
| Light: mono style-less direct only | `internal/light/light.go` `Bake` (:42), `directLight` (:107), `WriteLit` (:136) |
| Light BSP parse | `internal/light/bsp.go` (faces, models); face `Styles` never populated |
| Engine consumers (already ready) | `bsp.DModel` (bsp/bsp.go:164), `DFace.Styles` (bsp/bsp.go:282,293), renderer `ensureBrushModelGeometry`, server `*N` names |
| ericw reference | `ERICW_TOOLS_DIR` default `/home/darkliquid/Projects/ericw-tools/build-linux`; C sources under `ericw-tools/qbsp/{brushbsp,tjunc,writebsp,faces,merge,qbsp}.cc`, `ericw-tools/light/{ltface,bounce,write,entities}.cc` |

---

## Task 1 (M0): Solidbsp core — replace arrangement CSG (world path)

**Files:** Create `internal/qbsp/brush.go`, `solidbsp.go`, `solidbsp_test.go`;
Modify `compiler.go` (pipeline re-order), `faces.go`, `writebsp.go`
(node face spans); Delete `internal/qbsp/csg.go` at the end of M0.

- [ ] **Step 1: Write the failing tests** (before implementation):
  - **Brush polyhedron**: `bspBrush` built from a 6-face box brush has 6
    fins; `Volume` bounds against `brushBounds` (compiler.go:254).
  - **Plane side classification**: `SPLIT_DIST`-style classify returns
    FRONT/BACK/BOTH for a brush axis-aligned and diagonal to a split plane.
  - **SplitBrush**: splitting a box brush by an interior plane yields the
    expected two convex sub-brushes whose union covers the original
    (winding area sums per facet).
  - **ChopBrushes**: two stacked overlapping solid brushes resolve
    last-wins; a water brush inside a solid pit produces the same leaf
    contents as `assignContents` did for the same geometry (parity on the
    old behavior — this is the semantic gate for deleting the point test).
  - **Tree contents**: `BuildTree_r` over a synthetic map (brushes from
    `Map` literals, no pak) yields leaf contents equal to the old
    arrangement results for the box/room/doorway fixtures in
    `qbsp_test.go`.
  - **Node face spans**: each node's `firstface/numfaces` covers exactly
    the faces inside its bounds (loader oracle: `LoadTree` parses node
    lumps; add an assert that face-index ranges are in-bounds and
    disjoint across sibling nodes where partitions are disjoint).
  - **Scale smoke (bench, not CI gate)**: 20×20×20 cell brush grid
    (8000 planes) compiles < 10 s with solidbsp; arrange-mounted control
    expected to explode (kept as a benchmark comparison only).
  - Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/qbsp -run 'TestSolid|TestBrush|TestChop|TestTree' -count=1`

- [ ] **Step 2: Implement** `internal/qbsp/brush.go` +
  `solidbsp.go`:
  1. `bspBrush` (fins []winding, planes []int, content, bounds, orig).
  2. Classify/split ops (`TestBrushToPlanenum`, `SplitBrush`,
     `BrushMostlyOnSide` equivalents — ericw `brushbsp.cc:263,409,379`).
  3. `ChopBrushes` (solid subtraction, brushbsp.cc:1447) with
     fragmentation control.
  4. `BuildTree_r` + `SelectSplitPlane`/`ChooseMidPlaneFromList`
     (brushbsp.cc:938,856) with `AUTO`/`FAST`/`PRECISE`; `SplitBrushList`
     (brushbsp.cc:1122); `LeafNode` (brushbsp.cc:348).
  5. Re-wire `Compile` (compiler.go:81) to the ericw stage order:
     solidbsp world (PRECISE) → portals → FillOutside (reuse leak.go BFS,
     re-root on new tree) → world rebuild PRECISE + refill →
     MarkVisibleSides (add `SURF_NODRAW` flag on `outFace`) → MakeFaces
     (from surviving brush pieces) → PruneNodes.
  6. Write per-node `firstface/numfaces` in `serializeNodes` (both
     formats) from node data collected by the builder.
  7. Switch `buildHullClipNodes` (hull.go:27) to solidbsp over expanded
     planes; keep the ±16/±32 extents math and the shared-root contract.
  8. Delete `csg.go` once no callers remain (verify via `grep` for
     `buildArrangement|region|signature|mirrorNbr`).

- [ ] **Step 3: Regression gates** — `mise run verify`; existing
  `qbsp_test.go` round-trip/structural invariants stay green; the
  `q1_light_black`-style parity table still loads in ericw `bspinfo` with
  the parent plan's per-lump tolerance policy.

---

## Task 2 (M1): Brush-entity submodels

**Files:** Create `internal/qbsp/entities.go`; Modify `compiler.go`
(per-entity pipeline), `writebsp.go` (`serializeModels`), `hull.go`
(per-model clip roots), `faces.go` (per-model face ranges), `tree.go`
(leaf renumber applies per model); Extend `qbsp_test.go`.

- [ ] **Step 1: Write the failing tests**:
  - **Classification**: `worldspawn` → model 0; `func_wall`/`func_door`
    with brushes → models 1..N in encounter order; point entity with no
    brushes → no model; `"model" "*2"` aliasing maps to model 2.
  - **Model records**: ≥2 entities compile → `Models` len ≥2; each model's
    `headnode[0]`, `firstface`, `numfaces`, `visleafs`, `mins/maxs`
    (shrunk 1u per Q1 convention), `origin` from entity key.
  - **Lump disjointness**: face/vertex/edge/surfedge ranges of models are
    pairwise disjoint; `firstface+numfaces` never overlaps.
  - **Render smoke (asset-free)**: sdk headless boot with a compiled
    func_wall map → `ensureBrushModelGeometry(1)` returns geometry with
    the wall's face count.
  - **Collision smoke**: point trace at the wall blocks; trace beside it
    passes (drive `internal/server/collision` hull 1 on model 1's
    clipnodes).
  - **Detail**: `func_detail*` never emits a model, never yields
    clipnodes, and is CSG'd into the enclosing structural solid.
  - Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestSubmodel -count=1`
    and the sdk smoke test.

- [ ] **Step 2: Implement**:
  1. `entities.go`: classify; assign `outputmodelnumber` (qbsp.cc
     :1035:1045 `*{}` naming); read `origin`/`angle`/`_hulls` keys.
  2. Per-entity compile loop in `Compile`: solidbsp each brush entity
     (FAST policy) → emit nodes/leafs/faces into shared lumps → record
     model span.
  3. `serializeModels` → emit `N` 64-byte records (mins/maxs shrunk,
     origin, headnode[0..3] — headnode[1] = clip root per `_hulls`,
     visleafs, firstface/numfaces).
  4. Clips: `_hulls` absent → submodels use world hulls (classic);
     present → per-model clip trees appended to the shared lump, root in
     `headnode[1]`. BSP29 clipnode limit `0xfff0` enforced per tree.

- [ ] **Step 3: Engine smoke** — func_wall map runs: server registers the
  submodel (`*1`), renderer draws it, a trigger on the wall fires when the
  player touches it (headless sdk).

- [ ] **Step 4: Parity (env-gated)** — `testmaps/` map with func_walls:
  our bspinfo table vs ericw; model count exact, face counts within
  tolerance; `.prt` unchanged (world-only portals).

---

## Task 3 (M2): T-junction fixing

**Files:** Create `internal/qbsp/tjunc.go`, `tjunc_test.go`; Modify
`faces.go`/`writebsp.go` (edge table rebuild after pass).

- [ ] **Step 1: Write the failing tests**:
  - **No-T invariant**: two stacked brushes with misaligned coplanar seam
    (2u face beside 1u face offset 0.5u) compile → no vertex of any face
    lies strictly inside an edge of another face (exhaustive pairwise
    edge-vertex distance test, tolerance 1e-4 over the compiled polygons).
  - **Insertion only on interactions**: coincident duplicate faces
    (identical plane+content) do not self-split (`HasTJuncInteraction`
    negative); a genuine T across different contents does split.
  - **Edge table rebuild**: `edgeTables` output remains closed
    (surfedge→edge→vertex chains, no dangling indices) after splitting;
    vertex count grows by exactly the inserted points.
  - **Node spans re-asserted**: after the pass, every node's
    `firstface/numfaces` still resolves (reassigned spans).
  - Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestTjunc -count=1`

- [ ] **Step 2: Implement** `tjunc.go` mirroring `qbsp/tjunc.cc`:
  1. Per-model edge universe: walk face polygons → candidate
     (start,end) edges.
  2. `FindEdgeVerts_FaceBounds` (tjunc.cc:228) with spatial hash over
     face bounds; `PointOnEdge` (tjunc.cc:56) tolerance test;
     `HasTJuncInteraction` (tjunc.cc:177) content/plane filter.
  3. `SplitFaceIntoFragments` (tjunc.cc:248): insert edge points into
     polygons.
  4. Emit polygons with added vertices (renderer triangulates; no fan
     retopologization needed — ironwail consumes polygons), then
     re-run `edgeTables` and reassign node face spans.
- [ ] **Step 3: Renderer smoke** — `BuildWorldGeometry` on a tjunc'd map:
  zero degenerate triangles; parity screenshot path shows no seam cracks
  (creates a `parity` fixture later if cheap).
- [ ] **Step 4: Parity (env-gated)** — face-count delta vs ericw shrinks
  toward the tolerance band; `bspinfo` reads output.

---

## Task 4 (M3): Light styles + per-style lightmaps

**Files:** Modify `internal/light/light.go` (`Light` gains `Style`,
`Color`; `Bake` per-style), `bsp.go` (style patch + modelinfo), `trace.go`
(unchanged contract); Create `internal/light/write.go` (Lighting lump
layout); Extend `light_test.go`, `internal/light/light_test.go`.

- [ ] **Step 1: Write the failing tests**:
  - **Style routing**: entities `style 0`, `style 1`, `style 3` on one
    face → `face.styles == [0,1,3,0]`; style-1 lights land in block 1.
  - **Lump layout**: per face W·H block per style, consecutive;
    `lightofs = offset of style-0 block`; clamps: >4 distinct styles on a
    face → first 4 win; unlit/NoDraw/Sky faces keep `lightofs=-1`.
  - **Loader round-trip**: written `Styles[]` + `lightofs` re-parse via
    `LoadTree` exactly (bsp/loader.go:338,357).
  - **`.lit`**: style-0 RGB triplets, `QLIT`+v1, `ApplyLitFile` byte-exact
    (bsp/lit.go); styled blocks not in `.lit` (mono-only contract).
  - **Engine smoke**: a styled face renders with `lightstyle 1` cvar
    animating block 1 (`ExpandLightmapSamples` path).
  - Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/light -run TestStyle -count=1`

- [ ] **Step 2: Implement**:
  1. `Light{Origin, Value, Style, Color}`; parse `style`,
     `_color`/`color`.
  2. `Bake` gathers per-face distinct styles (max 4), per-style lightmap
     accumulation (direct term + shadow trace + clamp unchanged), alloc
     zero-filled per-style blocks (`Lightmap_ForStyle` semantics,
     ltface.cc:763).
  3. `write.go`: Lighting lump layout (per-face, per-style W·H blocks);
     face `styles[]` patch in the BSP; `.lit` from style-0 block only.
  4. Keep Color for M4 (sun/bounce); direct accumulation uses averaged
     color when present else mono.

- [ ] **Step 3: Parity (env-gated)** — `q1_light_black` re-lit: `.lit`
  size matches ericw exactly; samples within 20% relative error where
  deterministic.

---

## Task 5 (M4): Sun and single-bounce radiosity

**Files:** Create `internal/light/sun.go`, `bounce.go`; Modify
`light.go` (sky faces become emitters, gather loop), `bsp.go` (texture
color extraction for albedo); Extend tests and `cmd/light` flags.

- [ ] **Step 1: Write the failing tests**:
  - **Sun direction**: `sun_mangle` yaw/pitch → correct direction vector;
    top face bright, bottom dark, side gradient `cos θ`.
  - **Sky as source**: non-sky sample receives sun light modulated by the
    sky face's texture color (placeholder 255 default); sky faces keep
    `lightofs=-1`.
  - **Bounce**: closed box, one `_texlight` emissive face, `-bounce 1` →
    opposing wall non-zero where direct was 0; clamp at 255; `-bounce 0`
    default reproduces today's direct-only output exactly.
  - **Albedo**: texture average used from `-wadpath` textures when
    available (via `internal/image.LoadWad`); keys `_texlight`/`_light`
    override.
  - Run: `TMPDIR=…/.tmp CGO_ENABLED=0 go test ./internal/light -run 'TestSun|TestBounce' -count=1`

- [ ] **Step 2: Implement** `sun.go` + `bounce.go`:
  1. `sun` entity + worldspawn `sunlight`/`sun_mangle`/`sunlight_color`
     parse (`-sun` flag).
  2. `LightFace_Sky` equivalent (ltface.cc:1378): sky faces → directional
     emitter into style 0 of lit faces; sky faces get no lightmap.
  3. `bounce.go`: emissive surface list (light.cc
     `EmissiveLightSurfaces`), single-bounce gather rays vs `TreeTracer`,
     clamped colorbleed (bounce.cc semantics), `-bounce n` (default 0).
- [ ] **Step 3: Engine smoke** — sunlit/bounced map renders: ceiling dark,
  top surfaces bright, bounce-lit wall visible; screenshot / `bspdiag
  liquids`-style stats on the lit BSP.
- [ ] **Step 4: Parity (env-gated)** — same map both lighters, `-sun
  -bounce 1`: `.lit` sizes match, samples within tolerance band with
  `-bounce 0`-only comparisons for the deterministic core.

---

## Task 6 (M5): Pipeline, CLI, parity harness, docs

**Files:** Modify `mise.toml` (map-build logs, per-model stats), `cmd/qbsp`
(`-merge`, `-omitdetail`), `cmd/light` (`-bounce`, `-sun`), `internal/qbsp/
parity_test.go` + `internal/light` parity tests; Extend
`docs/MAP_COMPILING.md`; Update spec/plan status sections.

- [ ] **Step 1: Flags + logging** — flags above; qbsp logs per-model
  entities/faces/hulls; light logs styles-per-face, sun, bounce surfaces.
- [ ] **Step 2: Parity harness additions** (env-gate, skip when absent):
  1. func_wall map: model-count table vs ericw (exact); per-model face
     ranges within tolerance.
  2. tjunc'd map: seam fixture both compilers; crack-free assertion on
     ours (their polygon set is the reference — vertex-set containment
     where deterministic).
  3. `q1_light_black` baseline: bspinfo table pinned; `.lit` size + sample
     tolerance; styled variant (`lightstyle`-bearing fixtures) as a
     containment check (styles may reorder; assert block layout, not
     values).
  4. Full pipeline `mise run map-build MAP=…` green on a func_wall +
     styled-lights fixture; engine headless boot of the result
     (`sdk`/`preMountedPaks`).
- [ ] **Step 3: Docs** — `docs/MAP_COMPILING.md`: submodel entities,
  `-merge`/`-omitdetail`, `-bounce`/`-sun`, style layout, new limits;
  cross-link the v2 spec.
- [ ] **Step 4: Gates** — golangci-lint clean on `internal/qbsp`,
  `internal/light` (+ new files); `mise run verify` green; deterministic
  byte-identical repeat outputs; `.gitignore` allow rules for any new
  fixtures (e.g. `!/internal/qbsp/testdata/` additions, testmaps copies).

---

## Acceptance criteria mapping

| Milestone | AC | Scope | Key outputs |
|---|---|---|---|
| M0 | AC2 (foundation) | Solidbsp world path, ChopBrushes contents, node face spans, hulls on solidbsp, `csg.go` deleted | `brush.go`, `solidbsp.go`, pipeline re-order |
| M1 | AC1 | Entity classification, per-entity trees, model records, `*N`, per-model clips | `entities.go`, multi-model `serializeModels` |
| M2 | AC3 | Tjunc edge insertion, edge rebuild, span reassignment | `tjunc.go` |
| M3 | AC4 (styles) | Per-style lightmaps, `styles[]` + `lightofs`, `.lit` style-0 | `write.go`, `Bake` per-style |
| M4 | AC4 (sun/bounce) | Sun/sky emitters, single-bounce radiosity | `sun.go`, `bounce.go` |
| M5 | all | CLI flags, parity harness, engine smoke, docs, gates | `mise.toml`, `MAP_COMPILING.md`, parity tests |

## Testing strategy (summary)

1. **Reader-oracle round-trips** at every writer change: `LoadTree` +
   loader struct equality; face `styles`/`lightofs` decode exactly.
2. **Behavioral parity with the old path** where semantics must not change
   (leaf contents, hull blocking) — gates the arrangement→solidbsp flip.
3. **Engine smoke, asset-free** (sdk + `preMountedPaks`): submodel render +
   collide, styled light animation, sun/bounce render, PVS unchanged.
4. **Geometry invariants**: no-T on tjunc output; disjoint model ranges;
   closed surfedge chains.
5. **ericw parity (env-gated)**: bspinfo lump tables (tolerance policy per
   the parent plan), `.prt` cross-feed unchanged, `.lit` size/sample
   bands, full `q1_light_black` baseline pinned.
6. **Conventions**: `TMPDIR=…/.tmp CGO_ENABLED=0 go test <pkg> -count=1`;
   deterministic byte-identical repeat output; embedded fixtures.

## Risks & mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Solidbsp overlap/coincident-face bugs | Broken collision/render | ericw `brushbsp.cc`/`csg.cc` oracle; old-vs-new content parity tests gate the flip; collision point tests |
| Submodel clip drift (classic hull-0-only) | Players clip func_walls | `_hulls` honored; func_door parity fixture vs ericw |
| Tjunc fan-vs-polygon choice | Seam artifacts | Polygon + added-vertex output; renderer triangle smoke; seam fixture parity |
| Style lump layout wrong | Black/broken lighting, engine read OOB | Loader round-trip + `ExpandLightmapSamples` smoke; `.lit` byte contract |
| Sun/bounce divergence vs ericw | Harsh/black walls | Reference `.lit` bands; `-bounce 0` default keeps surface small |
| Long unbuildable window during `csg.go` deletion | Churn | Flip behind `Compile` flag, test, then delete; gates block the flip |
| BSP29 clipnode `0xfff0` limit with per-model trees | Fails big maps | Per-tree limit error + BSP2 escape hatch (existing) |
| Scope creep (phong, lightgrid, HDR/-lit2, -extra, 2psb, HL/Q2, BSPX) | Bead drags | Explicit non-goals in the v2 spec; follow-up bead `ironwail-go-c05` |
| GPL-2 ericw sources | Cannot copy code | Clean-room algorithm ports; parity harness verifies behavior only |

## Non-goals (tracked in `ironwail-go-c05`)

HDR/`-lit2`/`-lithdr`, `-extra`/`-extra4`, phong shading, lightgrid,
`-2psb`, HL/Q2 targets, BSPX brushes lump, per-submodel PVS rows,
automatic texture-color bounce without `-wadpath`.