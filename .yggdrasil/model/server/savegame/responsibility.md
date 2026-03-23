# Responsibility

## Purpose

`server/savegame` owns persistence of authoritative server state across save/load boundaries.

## Owns

- portable native save snapshot capture and restore
- Quake/KEX text save parsing and restore support
- QC string/global persistence and reallocation rules during restore
- re-linking restored entities back into the authoritative world

## Does not own

- Initial map bootstrap for a fresh map.
- Runtime client signon/session handling aside from state needed for restore.
- Network message serialization.
