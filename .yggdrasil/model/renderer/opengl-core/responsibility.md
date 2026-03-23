# Responsibility

## Purpose

`renderer/opengl-core` owns the OpenGL backend lifecycle, GL context setup, frame-level utilities, and core OpenGL-specific runtime behavior.

## Owns

- OpenGL renderer construction and runtime control
- core GL state setup
- OpenGL-specific camera/polyblend/warpscale support
- small OpenGL-only stubs/helpers

## Does not own

- detailed world/entity submission logic
- GoGPU backend behavior
