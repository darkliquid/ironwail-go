# qbsp Manual

qbsp turns a Quake `.map` file into a playable `.bsp` file. It is the first stage of the map pipeline:

```text
map.map -> qbsp -> map.bsp + map.prt -> vis -> map.bsp -> light -> map.bsp + map.lit
```

The BSP file holds the geometry of the level. The `.prt` file holds the portals that `vis` needs. If the map leaks, qbsp also writes a `.pts` trail file.

This manual explains how to run qbsp and what it does. For the whole pipeline, read the map compiling document.

## Getting qbsp

Build it from the repository root:

```sh
go build -o qbsp ./cmd/qbsp
```

Or run `mise run build-qbsp`.

## The shortest usable command

```sh
./qbsp mymap.map
```

This writes `mymap.bsp` and `mymap.prt` next to the source file.

## Command line

```text
qbsp [-o out.bsp] [-bsp2] [-2psb] [-leaktest] [-margin n] [-omitdetail] map.map
```

The position of the `.map` file is the only required argument.

| Flag | What it does |
| --- | --- |
| `-o <path>` | Writes the BSP to this path. The default is the map name with the `.bsp` ending. |
| `-bsp2` | Emits the extended BSP2 format with 32-bit indexes. Use it for very large maps. |
| `-2psb` | Emits the BSP2RMQ variant with 32-bit indexes and 16-bit bounds. It implies `-bsp2`. |
| `-leaktest` | Exits with an error code when the map leaks. Use it in build scripts. |
| `-margin <n>` | Sets the width of the empty ring around the map. The default is 64 units. |
| `-omitdetail` | Drops every `func_detail*` entity from the output. |

## What a compile does

The tool reads the map, merges the solid brushes, and cuts the result into a tree of convex rooms. Then it builds the data the engine needs to collide and to render.

The stages are:

| Stage | What it does |
| --- | --- |
| Parse | Reads brushes and entities from the `.map` file. It accepts QuakeEd and Valve 220 brush syntax, and it detects the style per face. |
| CSG | Merges the solid brushes into one world. The method is classic solidbsp, which splits brushes against each other. |
| Hulls | Builds collision trees for two hull sizes. Hull one suits the player box, plus or minus 16 units. Hull two suits large monsters, plus or minus 32 units. |
| Faces | Builds the faces, edges, and vertices of the world from the cut planes. |
| Fix | Splits crack-prone edges where faces meet coplanar neighbors. This stops tiny gaps between surfaces. |
| Prune | Removes empty space from the tree so the file stays lean. |

A t-junction pass runs between the face and prune stages. It splits the longer edge wherever one face edge meets the middle of another face edge.

## Brush entities

Brushes that belong to an entity become extra models inside the BSP. Doors, platforms, and walls work this way.

Each brush entity compiles into its own submodel. The submodel has its own node tree, its own faces, and its own collision tree per hull. The entity record gains a `model` key with a value like `*1` or `*2`, and the engine reads that key at run time.

## Textures

The BSP carries a texture table. For every texture name, qbsp writes placeholder data: a 16 by 16 gray image with four mip levels.

The game finds the real images by name when the game data holds them. Quake keeps world textures in the game data and matches them through the names in the table.

## Leaks

A leak is a path from the inside of the map to the empty void outside it. A leaked map cannot pass the vis stage, so fix leaks before you continue.

When qbsp finds a leak, it prints a line that starts with `LEAK`. It also writes a `.pts` file with a point trail from the entity to the hole. Open that file in a map editor to see the path.

Stop a build on a leak with `-leaktest`. The tool exits with an error code, and you can catch it in a script.

## Map format notes

Brushes are volumes of solidity. A single box brush is a solid cube, not a room. Build a room from thin wall, floor, and ceiling slabs. The empty space between the slabs becomes the walkable interior.

Entities are blocks of key and value pairs. Brushes sit inside the entity blocks of the map.

The entity blocks use the classic Quake format:

```text
{
"classname" "func_door"
"origin" "128 64 0"
}
```

Compiled maps carry an extra `BRUSHLIST` section with the per-model brushes. Tools can read it, and the engine ignores it.

## Verifying the output

You can open the result with the `bspdiag` tool. It prints the BSP version, lump sizes, entity blocks, and face details without running the engine.

The reference tool `bspinfo` from the ericw-tools set also reads the output of qbsp, vis, and light.

## What runs next

The `.prt` file is the input for the next stage:

```sh
./vis -o mymap.bsp mymap.bsp
./light -lit -o mymap.bsp mymap.bsp
```

Or run the whole chain at once with `mise run map-build MAP=mymap`.