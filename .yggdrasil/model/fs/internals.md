# Internals

## Logic

`FileSystem` maintains three related views of mounted content: `searchPaths` for loose directories used by globbing, `lookupPaths` for the real front-to-back resolution stack, and `packs` for open PAK handles that can be read on demand. `Init` always mounts `id1`, opportunistically inserts `ironwail.pak` from the executable or base directory, and optionally mounts a mod directory. `AddGameDirectory` discovers numbered `pakN.pak` files in ascending numeric order, prepends each loaded pack into the per-directory lookup group, then appends the loose directory entry and prepends the whole group into the global lookup stack. `SearchPathEntries` is the read-only introspection seam over that same `lookupPaths` stack, exposing only path/is-pack/file-count snapshots for host debug commands like `path`. `FindFile` sanitizes the requested path, guards loose-file lookups against root escape, linearly scans PAK entries using a canonical case-folded lookup key, and returns the first match. Read helpers then either read from the loose `io/fs` source or seek into an open PAK handle. `OpenFile` extends that model with a streaming seam: pack hits return section readers over the open archive handle, while loose hits return opened OS files and stat-derived lengths.

## Constraints

- The implementation and tests establish same-directory precedence as higher-numbered PAKs over lower-numbered PAKs over loose files, even though some package comments still describe loose files as higher priority.
- PAK lookup is case-insensitive via canonical lowercase slash-normalized keys; loose-file lookup still follows host filesystem semantics after path sanitization.
- `ListFiles` enumerates loose files and pack entries without deduplication and does not mirror `FindFile` override order exactly.
- `Close` releases PAK handles but does not reset the object for safe reinitialization.
- `loadPack` validates the `PACK` header and directory layout shape but does not comprehensively bound-check malformed offsets/lengths beyond read failures.

## Decisions

### Isolate Quake filesystem logic into its own package instead of a broad common layer

Observed decision:
- The Go port gives the Quake virtual filesystem a dedicated `internal/fs` package with explicit search-path state, path hardening, and raw-byte loading helpers.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- Filesystem behavior is easier to reason about and test in isolation, and higher layers consume a narrow raw-byte/loading API instead of inheriting filesystem logic through a monolithic common subsystem.
