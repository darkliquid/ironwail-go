# Responsibility

## Purpose

`qc/csqc` owns the client-side QuakeC wrapper, including CSQC-specific globals, entry-point discovery, draw/client hook integration, and per-frame global synchronization.

## Owns

- `CSQC` as a wrapper around a dedicated VM instance
- CSQC entry-point lookup and lifecycle
- CSQC-specific global offset caching
- CSQC draw/client hook integration
- precache registries for CSQC-managed resources

## Does not own

- General VM execution semantics outside the shared core
- Concrete renderer/client implementations behind the CSQC hooks
