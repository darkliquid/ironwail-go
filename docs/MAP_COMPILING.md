# Map Compiling (QBSP / VIS / LIGHT)

The pure-Go map compiler pipeline (bead `ironwail-go-t63`) turns a Quake
`.map` file into a playable BSP with collision, PVS visibility, and
lightmaps — no C toolchain required.

## Pipeline

```
map.map ──► qbsp ──► map.bsp + map.prt ──► vis ──► map.bsp ──► light ──► map.bsp + map.lit
              │ (leaks -> map.pts)
```

| Tool | Purpose |
| --- | --- |
| `qbsp` | Parse `.map` (QuakeEd + Valve 220), CSG, collision clipnode hulls, leak detection, write BSP29/BSP2 + `.prt` portal file |
| `vis` | Compute per-leaf PVS visibility from the `.prt`, write it into the BSP |
| `light` | Bake direct point-light lightmaps with shadow traces, write the Lighting lump + optional `.lit` |

Build them with `mise run build-qbsp`, `build-vis`, `build-light`, or run the
whole chain with `mise run map-build MAP=name` (reads `name.map`, writes
`name.bsp` + `name.lit`).

## Compiling a map

```sh
# qbsp: map -> bsp + prt
./qbsp -o mymap.bsp mymap.map
# vis: bsp + prt -> bsp with PVS
./vis -o mymap.bsp mymap.bsp
# light: bsp -> bsp with lightmaps + mymap.lit
./light -lit -o mymap.bsp mymap.bsp
```

`qbsp` flags: `-o out.bsp`, `-bsp2` (extended 32-bit BSP2 format), `-leaktest`
(fail on leaks), `-margin n`, `-omitdetail` (drop `func_detail*` entities
entirely). On a leak it writes `mymap.pts` with the point trail from the
entity to the void.

`vis` flags: `-o out.bsp`. `light` flags: `-o out.bsp`, `-lit` (write the
colored `.lit` sidecar), `-sun` (sun/`sunlight` directional lighting),
`-bounce n` (radiosity bounce count, 0 = direct only).

## Map format

Both classic QuakeEd brushes (`( p1 ) ( p2 ) ( p3 ) texture shiftX shiftY rot
scaleX scaleY`) and Valve 220 axis brushes (`… texture [ ux uy uz uoff ] [
vx vy vz voff ] rot sx sy`) are supported, auto-detected per face. Entities
are `{ "key" "value" … }` blocks; brushes are `{ ( … ) … }` blocks.

**Hollow construction matters.** Brushes are volumes of solidity — a room is
enclosed by thin wall/floor/ceiling slabs, and the interior is the empty space
between them. A single box brush is a solid cube, not a room.

## Textures

`qbsp` reads texture dimensions from WADs passed with `-wadpath` (or defaults
to 16×16 with a warning). The miptex table in the BSP carries the texture
names; the engine renders them from the BSP lump.

## Lighting

`light` bakes per-style point lighting: each lightmap luxel (16 units
apart in S/T space) accumulates `light / dist² · cosθ` from every `light`
entity (`"origin" "x y z"`, `"light" "300"`, optional `"style" "k"` 0..31
for separate animated lightmaps and `"_color" "r g b"`), clamped to 255,
with a ray-vs-BSP shadow trace. The Lighting lump holds one `W·H` block per
face style (face `styles[4]` + `lightofs` are patched in the BSP). `-lit`
writes a QLIT v1 colored sidecar of the style-0 samples that the engine's
`ApplyLitFile` reads.

- **Sun** (`-sun`): a `sun` entity (or worldspawn `sunlight`,
  `sun_mangle`, `sunlight_color`) lights non-sky faces directionally; sky
  faces (`sky*` textures) get no lightmap.
- **Bounce** (`-bounce n`): clamped single-bounce radiosity re-emits each
  lit surface's style-0 radiance onto its neighbours (shadow-traced),
  illuminating walls with no direct light.

## Brush entities

Brush-carrying entities (`func_wall`, `func_door`, …) compile to inline
`*N` BSP submodels: each gets its own node tree, faces, and per-hull clip
tree, and the entity gains a `"model" "*N"` key the engine resolves.
World-first model records; submodel bounds are shrunken by 1 unit per the
Q1 convention. The compiler CSG is classic **solidbsp** (brush splitting)
— real-sized maps (thousands of planes) compile in linear-ish time, and a
**t-junction fixing pass** splits crack-prone edges between coplanar
faces.

## Verifying output

- `internal/bsp.LoadTree` / the version-aware loader round-trip every stage
  (the tests assert structural invariants).
- ericw-tools' `bspinfo` reads our BSP29, BSP2, vis'd, and lit output (the
  parity harness in `internal/qbsp/parity_test.go` runs this when
  `ERICW_TOOLS_DIR` points at an ericw-tools build).

## Known limits (follow-up)

Tracked in bead `ironwail-go-c05`: HDR/`-lit2`, `-extra` supersampling,
phong shading, lightgrid, `-2psb`, HL/Q2 targets, BSPX lumps, per-submodel
PVS rows, and texture-color bounce without `-wadpath`. Submodel clip hulls
use the shared ±32-unit expansion seed (per-hull trees are a follow-up).