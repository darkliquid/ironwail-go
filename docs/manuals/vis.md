# vis Manual

vis computes the visibility data of a compiled map. It reads the `.bsp` file and the `.prt` portal file next to it, computes which parts of the map can see each other, and writes the result back into the BSP.

Run qbsp before vis. The portal file comes from qbsp. Run vis before light, or at least before you play the map.

```text
map.map -> qbsp -> map.bsp + map.prt -> vis -> map.bsp -> light -> map.bsp + map.lit
```

This visibility data is the Potentially Visible Set, or PVS for short. The engine uses it to skip geometry that the camera cannot see, and the server uses it to skip entities. Without it, every map draws everything and runs slowly.

## Getting vis

Build it from the repository root:

```sh
go build -o vis ./cmd/vis
```

Or run `mise run build-vis`.

## The shortest usable command

```sh
./vis mymap.bsp
```

The tool reads `mymap.prt` from the same folder, computes the visibility, and writes the result back into `mymap.bsp`.

## Command line

```text
vis [-o out.bsp] map.bsp
```

| Flag | What it does |
| --- | --- |
| `-o <path>` | Writes the output BSP to this path. The default overwrites the input file. |

## How visibility works

A compiled map is a tree of convex rooms, called leaves. The engine finds the leaf around the camera and asks which other leaves it can see. The PVS is the answer, and it is stored as a list of bits, one bit per visible leaf.

The tool works in stages:

| Stage | What it does |
| --- | --- |
| Read | Loads the BSP tree and the portal file. |
| Link | Connects the portals to the leaves they border. |
| Flow | Walks from every leaf through its portals and marks every leaf it can reach. |
| Compress | Packs each visibility row into a small form called run-length encoding, or RLE. |
| Patch | Writes the rows into the visibility section of the BSP and fixes the leaf offsets. |

The row format is the exact format the engine reads back. The engine expands each row when it runs, so the map ships the compact form.

## When visibility is not needed

Small test maps run fine without vis. If you skip it, the engine treats every leaf as visible and draws everything.

Run vis when a map has enough rooms that culling matters. The standard chain is qbsp, then vis, then light.

```sh
./qbsp -o mymap.bsp mymap.map
./vis -o mymap.bsp mymap.bsp
./light -lit -o mymap.bsp mymap.bsp
```

Or run the chain with `mise run map-build MAP=mymap`.

## Problems

The tool stops with an error if `mymap.prt` is missing. Run qbsp first so the portal file exists.

Fix leaks before you run vis. A map with a leak has no closed boundary, and its visibility is not meaningful. qbsp reports the leak and writes a `.pts` trail to find it.