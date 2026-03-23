# Responsibility

## Purpose

`audio/backends` owns concrete output backends and the DMA buffer/cursor contract used by the rest of the sound system.

## Owns

- Backend interface implementations for SDL3, oto, and null output.
- DMA buffer allocation details and sample-position updates for each backend.
- Backend-specific locking around shared DMA state.

## Does not own

- Channel scheduling, spatialization, or music selection.
- High-level runtime policy about which sounds should play.
