# Responsibility

## Purpose

`engine` owns the generic primitive layer in `internal/engine`: reusable containers, queueing, synchronous event delivery, and bounded-concurrency loading helpers intended for adoption by many engine subsystems.

## Owns

- The dependency-light package boundary and its documented intent.
- The taxonomy of primitives exposed by `internal/engine`.
- The separation between mutable runtime caches, init-time registries, collections, eventing, and loading helpers.

## Does not own

- Gameplay, rendering, networking, filesystem, or Quake protocol/domain logic.
- Any specific asset/model/texture/sound formats beyond generic helper contracts.
