# Internals

## Organization

Mapped files:

- `build.go`
- `textures.go`
- `lightmap.go`
- `mesh.go`
- `cleanup.go`

## Logic

This node groups the extracted helpers that prepare or tear down OpenGL world resources without needing direct ownership of `*Renderer`.

The texture helpers intentionally accept palette conversion and upload callbacks from renderer root. That keeps the package boundary cycle-safe while still letting this node own BSP miptex parsing, embedded-sky extraction, flat-sky color planning, and the upload-plan application loop.

## Constraints

- texture planning must preserve Quake-style diffuse/fullbright/sky behavior
- lightmap recomposition and dirty bounds are parity-sensitive
- cleanup helpers must remain mechanical and must not silently change renderer cleanup order
