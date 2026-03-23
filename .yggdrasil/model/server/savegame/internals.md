# Internals

## Logic

Save capture walks the authoritative server state, deep-copies protocol-sensitive lists, snapshots client spawn parms, records edicts, and converts QC string/global state into portable forms. Restore performs the inverse: it validates the snapshot, restores metadata and precaches, rebuilds edicts and QC globals, re-links entities into the world, and handles text/KEX compatibility shims where needed.

## Constraints

- Raw QC string handles cannot survive across VM instances.
- Restore ordering matters: models/lightstyles/world linkage must be coherent before gameplay resumes.
- Save formats are compatibility surfaces, so version checks and name-based global restore behavior are deliberate.

## Decisions

### Portable save representation over raw VM memory dumps

Observed decision:
- Save/load logic converts QC-managed state into portable named/textual forms instead of serializing raw VM memory.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Save files can survive fresh VM instances and some future ordering drift, at the cost of more explicit capture/restore logic.
