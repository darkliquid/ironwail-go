# Responsibility

## Purpose

`renderer/opengl-world` owns the OpenGL-specific world rendering path: world pass submission, probes, sky, liquids, and runtime world draw orchestration.

## Owns

- OpenGL world pass sequencing
- OpenGL-specific sky and liquid handling
- world probe/debug support tied to the OpenGL path

## Does not own

- backend-neutral world data preparation
- OpenGL entity-specific paths that live in sibling nodes
