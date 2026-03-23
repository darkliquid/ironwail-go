# Responsibility

## Purpose

`common/core` owns the main shared primitive layer: `SizeBuf`, C-style token parsing and argv helpers, path/hash utilities, and tiny structural helpers.

## Owns

- `SizeBuf` and its primitive read/write methods.
- `Link` and `BitArray`.
- `ComToken`, `ComArgv`, `ComArgc`, and the `COM_Parse*` / `COM_CheckParm*` helper family.
- path/name/extension helpers and FNV-based hash helpers.

## Does not own

- Endian stream helpers in the `binary` subpackage.
- Higher-level protocol codecs that depend on protocol flags or gameplay semantics.
