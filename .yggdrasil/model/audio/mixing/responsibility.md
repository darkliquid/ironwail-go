# Responsibility

## Purpose

`audio/mixing` owns sample decode and cache policy, spatialization rules, and the actual paint path that mixes channels and raw samples into the DMA buffer.

## Owns

- SFX cache lookup and load policy.
- WAV validation and decode for sound effects and compatible music data.
- Stereo spatialization and distance attenuation.
- Paint-buffer math and conversion into the DMA output buffer.
- Auxiliary mixer implementations used for async/null behavior.

## Does not own

- Host/runtime orchestration of when updates happen.
- Music track selection and codec-level music loading logic.
- Concrete hardware backend transport.
