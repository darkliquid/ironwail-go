# Responsibility

## Purpose

`renderer/opengl-entities` owns OpenGL rendering helpers for alias models, sprites, decals, transparency/OIT, and GL-backed atlas/scrap behavior.

## Owns

- OpenGL alias and sprite rendering paths
- world-integrated OpenGL entity rendering
- decal rendering helpers
- OpenGL transparency/OIT path selection and shaders
- GL-backed scrap/atlas upload behavior

## Does not own

- shared entity data preparation independent of OpenGL
- OpenGL core lifecycle
