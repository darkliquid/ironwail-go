# Interface

## Main consumers

- `cmd/ironwailgo` and host/runtime code that construct and drive renderer backends
- HUD/menu/draw/client-facing code that uses renderer canvas and drawing facilities

## Main exposed shape

The package exposes:
- backend-agnostic renderer interfaces and config
- backend-specific implementations selected by build tags
- world/entity/effect rendering helpers
- 2D canvas and drawing support

Detailed contracts live in the child nodes.
