# Responsibility

## Purpose

`server/player-movement` owns the player-specific movement rules that translate user intent into authoritative server movement.

## Owns

- ground, air, water, ladder, and stair-style movement helpers
- player command translation into velocity/origin changes
- movement-specific quirks such as ideal pitch and stepping behavior

## Does not own

- Generic collision tracing primitives.
- Client/session signon logic.
- Non-player movetype scheduling and QC callback dispatch.
