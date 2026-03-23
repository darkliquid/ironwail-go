# Interface

## Main consumers

- OpenGL and GoGPU world/entity render paths

## Contracts

- this node prepares or exposes the backend-neutral structures that concrete world renderers consume
- this node owns backend-neutral leaf-to-face lookup and PVS face selection so OpenGL and GoGPU can consume the same visibility subset
- world data and surface/texture conventions must stay stable across backends
