# Responsibility

## Purpose

`renderer/opengl-world` owns the OpenGL-specific world rendering path: world pass submission, probes, sky, liquids, and runtime world draw orchestration.

## Owns

- OpenGL world pass sequencing
- OpenGL-specific sky and liquid handling
- world probe/debug support tied to the OpenGL path
- true `internal/renderer/world/opengl/` helpers that own OpenGL-only shader payloads and texture-upload primitives
- the root-owned OpenGL seam files under `internal/renderer/world_*_opengl_root.go`, which now carry the live backend implementation used by normal tagged builds

## Does not own

- backend-neutral world data preparation
- OpenGL entity-specific paths that live in sibling nodes
- deleted legacy subdirectory copies that were briefly used as quarantine scaffolding during the refactor
