# Responsibility

## Purpose

`renderer/input` owns the renderer-coupled input backend adapters for GLFW, GoGPU, and stub environments.

## Owns

- backend-specific event/input bridges used by renderer runtime code

## Does not own

- engine-wide input semantics beyond the data exposed through these adapters
