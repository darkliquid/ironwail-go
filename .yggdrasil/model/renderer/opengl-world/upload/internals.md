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
The geometry-build helper now mirrors the shared-world high-precision lightmap-coordinate policy when deriving face extents and per-vertex lightmap UVs. That keeps OpenGL world upload aligned with the light compiler/shared path on very large qbj/qbj2-style coordinates where float32-only extent math can under-allocate a face lightmap and make the lighting look corrupted.
The world-texture upload plan now also mirrors the shared-world fullbright split policy: embedded bright-range texels are removed from the base diffuse upload and placed only in the separate fullbright texture, while palette index `255` is treated as transparency only for cutout materials. That keeps OpenGL world uploads aligned with the current GoGPU/C-style contract instead of double-counting embedded fullbright texels in both layers.

## Constraints

- texture planning must preserve Quake-style diffuse/fullbright/sky behavior
- OpenGL texture upload helpers must copy Go-owned pixel slices into temporary C-owned upload buffers instead of passing slice headers or Go-managed backing storage directly through the cgo-backed OpenGL wrappers
- lightmap recomposition and dirty bounds are parity-sensitive
- cleanup helpers must remain mechanical and must not silently change renderer cleanup order
