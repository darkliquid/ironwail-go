# Interface

## Main consumers

- `server/physics-core` and `server/player-movement`
- QC builtins and gameplay code that perform traces or relink entities
- map/bootstrap and save/load flows that rebuild world links

## Main surface

- world clear/rebuild helpers
- movement/trace entry points
- point-contents queries
- entity link/unlink and touched-leaf bookkeeping

## Contracts

- Linking must update abs bounds, BSP linkage, and trigger-touch discovery consistently.
- Trace semantics must remain compatible with Quake hull/contents conventions.
- World queries are authoritative inputs to movement, trigger dispatch, and water/ground classification.
