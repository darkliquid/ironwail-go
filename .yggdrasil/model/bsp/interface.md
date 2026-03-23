# Interface

## Consumers

Primary inbound consumers:
- `internal/server`, which loads BSP data for world model construction, PVS/fat-PVS queries, and collision hull construction.
- `internal/renderer`, which consumes normalized tree geometry, lighting, vis data, and texture lump bytes for world rendering.

## Core API

### Raw file loading

- `Load(r io.ReadSeeker) (*File, error)`
  - Reads a BSP into the raw `File` representation.
  - Intended for consumers that need version-specific data, especially clipnodes and original model structures.

### Normalized tree loading

- `LoadTree(r io.ReadSeeker) (*Tree, error)`
  - Reads a BSP and converts it into a normalized runtime tree.
  - Intended for systems that want consistent node/leaf/face/model access across BSP variants.

### Low-level reading helpers

- `NewReader(r io.ReadSeeker) *Reader`
- `(*Reader).ReadHeader()`
- `(*Reader).ReadLump(...)`
- version helpers such as `IsValidVersion`, `IsBSP2`, and `IsQuake64`

### Spatial/visibility queries

- `(*Tree).LeafPVS(leaf int) []byte`
- `(*Tree).PointInLeaf(point [3]float32) int`
- `DecompressVis(...)`

### Lighting sidecars

- `ApplyLitFile(tree *Tree, data []byte) error`
  - Replaces BSP monochrome light samples with RGB `.lit` samples when the sidecar is structurally valid.

## Failure modes

- Unsupported BSP versions are rejected.
- Malformed lump sizes or invalid indices produce hard errors during load.
- Invalid `.lit` headers, versions, or sample counts are rejected.

## Exposed runtime data

Main owned types:
- raw `File`
- normalized `Tree`
- version-aware BSP lump structs such as nodes, leafs, models, planes, faces, and texinfo

Consumers are expected to treat these as authoritative decoded map data, not mutable game-state objects.
