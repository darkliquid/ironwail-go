# Responsibility

## Purpose

`server/world-collision` owns the authoritative spatial-query layer used by movement, triggers, QC traces, and entity/world linking.

## Owns

- box-hull generation and BSP hull selection
- point contents and hull traversal helpers
- movement traces against world and linked entities
- area-node broadphase structures
- `LinkEdict`/`UnlinkEdict`-style world integration and trigger discovery

## Does not own

- Per-frame physics policy.
- Client/session command handling.
- Network serialization.
