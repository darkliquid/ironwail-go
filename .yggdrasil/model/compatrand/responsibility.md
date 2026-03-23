# Responsibility

## Purpose

`compatrand` owns the compatibility random-number stream used where the Go port needs parity with Quake's Linux/glibc `rand()` behavior.

## Owns

- The compatibility RNG state machine and seed normalization rules.
- Thread-safe instance RNGs plus the package-global shared RNG surface.
- The exact output-sequence contract verified against libc-compatible expectations.

## Does not own

- QuakeC `random()` float transformation semantics, which live in `qc` builtins.
- Gameplay logic that consumes randomness.
- General-purpose randomness for non-parity-sensitive engine code.
