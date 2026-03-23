# Responsibility

## Purpose

`engine/registry` owns the write-once, read-many lookup table used for configuration or registration data that should become stable after initialization.

## Owns

- `Registry[K, V]` construction and lookup.
- Duplicate-registration detection.
- Optional `Freeze` semantics that permanently block later registration.
- Panic-on-misuse helpers such as `MustLookup`.

## Does not own

- Validation of the registered payload beyond key uniqueness.
- Late-bound mutable runtime storage; that belongs to `engine/cache` or callers.
