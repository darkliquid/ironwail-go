# Responsibility

## Purpose

`client/protocol` owns the translation from server messages into client state, transient effects, and interpolated render-facing entities.

## Owns

- `Parser` and server-message decoding.
- Signon/serverinfo/clientdata/entity-update handling.
- Temp entity and beam decoding.
- Per-frame entity relink/interpolation and stale-entity removal.
- View-blend and related client-side visual effect state updates.

## Does not own

- Low-level network transport.
- Host frame scheduling.
- User input generation or local prediction.
