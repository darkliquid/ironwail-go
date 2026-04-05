# Internals

## Organization

`renderer/opengl-world` is now the parent node for the OpenGL world split.

Mapped here:

- `internal/renderer/world_opengl.go`
- `internal/renderer/world/opengl/shaders.go`

Child helper nodes now carry the extracted helper files:

- `renderer/opengl-world/upload`
- `renderer/opengl-world/passes`
- `renderer/opengl-world/sky`

## Logic

The parent node exists to describe the part of the OpenGL world path that could not leave the renderer package cleanly:

- `*Renderer` method ownership
- renderer-owned world caches and GL wrapper types
- high-level upload / render / teardown sequencing
- program setup that consumes the shader payloads in `shaders.go`

This split follows the already-landed code seams instead of inventing new ones. Helper implementation detail remains in the extracted subpackage files; the parent keeps only the root orchestration layer that coordinates those helpers. For sky rendering, that now means root code snapshots one locked base `worldopengl.SkyPassState`, then reuses it for both world and brush passes by overriding only the per-model fields instead of duplicating the full sky-state assembly twice.
The OpenGL world upload summary log is treated as diagnostic bring-up data and now emits at `Debug` so normal runs are not dominated by backend-internal geometry stats.
The root OpenGL world program setup now also exposes an explicit `uAlphaTest` switch so the shared fragment shader can separate ordinary ALPHABRIGHT world shading from real cutout discard behavior. Root world/brush orchestration binds the same program for both buckets but toggles `uAlphaTest` per draw-call batch, keeping regular qbj/qbj2 embedded-fullbright materials on the opaque lighting-mask path while `{...}` surfaces still use discard semantics.

Dead compatibility glue should not accumulate here. Once a helper seam is fully consumed from child packages or shared-world helpers, the parent drops unused forwarding wrappers and aliases instead of preserving them as historical baggage.

## Constraints

- method-heavy lifecycle code still has to live on renderer-root types
- world visibility, fog, sky, and liquid behavior remain parity-sensitive
- the parent must keep delegating to shared-world policy for backend-neutral rules

## Decisions

### Split graph by extracted helper families

Observed decision:
- `renderer/opengl-world` was narrowed to the renderer-root orchestration files, and the extracted helper files were moved into child nodes grouped by upload, passes, and sky behavior.

Rationale:
- the node had exceeded Yggdrasil width (`W017`) and own-artifact size (`W015`) thresholds, while the code already had clean helper seams matching those three responsibilities.
