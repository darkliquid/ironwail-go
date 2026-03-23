# Responsibility

## Purpose

`ironwailgo/runtime-entities` owns translation from client/server runtime state into renderer-facing entity collections and related runtime model caches.

## Owns

- Collection of brush, alias, sprite, beam, particle, and dynamic-light presentation data.
- Runtime alias and sprite model caching/loading helpers inside the command package.
- Stale/current entity filtering policy for dynamic client entities.
- Entity-side debug logging hooks used by debug view telemetry.

## Does not own

- The renderer implementation that consumes the collected entities.
- View/camera calculation or HUD/menu presentation.
