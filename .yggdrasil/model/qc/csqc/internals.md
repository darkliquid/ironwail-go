# Internals

## Logic

`CSQC` resets wrapper state on load, loads a program into a fresh VM, resolves required and optional entry points, caches CSQC global offsets, and then synchronizes per-frame state into those globals before entry-point calls.

It maintains separate registries for precached models, sounds, and pics so client-side scripts can refer to stable resource indices.

## Constraints

- CSQC is only considered loaded when required entry points are present.
- Per-frame state must be synchronized into globals before entry-point execution.
- CSQC behavior depends on hook availability for draw and client-state queries.

## Decisions

### Separate CSQC wrapper over the shared VM

Observed decision:
- The Go port uses a dedicated wrapper with a separate VM instance for CSQC rather than merging CSQC and SSQC state into one context.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- client-side scripting can reuse the VM core while keeping separate entry points, globals, and precache state
