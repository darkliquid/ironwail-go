# Responsibility

## Purpose

The `bsp` node owns BSP-format knowledge for the Go port. It reads Quake BSP files, exposes strongly typed on-disk lump data, builds normalized runtime tree structures, and applies optional `.lit` lighting sidecars.

Primary evidence:
- `internal/bsp/doc.go`
- `internal/bsp/bsp.go`
- `internal/bsp/loader.go`
- `internal/bsp/tree.go`
- `internal/bsp/lit.go`

## Owns

- BSP version recognition for BSP29, 2PSB, BSP2, and Quake64 variants.
- Header and lump reading helpers.
- Raw `File` loading for version-specific lump access, especially collision-relevant structures like clipnodes.
- Normalized `Tree` loading for runtime world, face, node, leaf, model, and visibility access.
- BSP query helpers such as decompressed leaf PVS and point-in-leaf traversal.
- `.lit` sidecar validation and lighting replacement.

## Does not own

- Renderer-specific face/material interpretation beyond exposing raw and normalized BSP data.
- Server collision or world-link logic beyond providing the data structures those systems consume.
- Texture decoding beyond returning raw texture lump bytes.
- External `.vis` or `.ent` loading behavior.

## Boundaries

`bsp` is the file-format and spatial-structure boundary between raw map assets and runtime systems like the renderer and server. It should know the BSP layout, but not the higher-level policies of rendering, visibility culling strategy, or collision response.
