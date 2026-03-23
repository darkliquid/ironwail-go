# Internals

## Logic

### Dual loading model

The package intentionally has two loading paths:

- `Load`
  - Produces a raw `File` with version-specific lump structures.
  - Best suited for consumers that need original-format details such as clipnodes.

- `LoadTree`
  - Produces a normalized `Tree`.
  - Best suited for renderer/server systems that want a stable runtime representation across BSP variants.

This split is one of the package’s defining design choices and differs from the original C tendency to populate one broader brush-model structure during load.

### Load ordering

`LoadTree` reads data in a dependency-sensitive order:
1. entities
2. visibility
3. textures
4. lighting
5. planes
6. vertexes
7. texinfo
8. edges
9. surfedges
10. faces
11. marksurfaces
12. leafs
13. nodes
14. models

This order is deliberate. Later structures validate and reference earlier ones. For example:
- marksurfaces depend on faces already existing
- leafs depend on marksurfaces
- nodes depend on leafs for child resolution

### Normalization

The normalized `Tree` path converts BSP variants into one common structure:
- child references become `TreeChild{IsLeaf, Index}`
- bounds are normalized to `float32`
- parent links are backfilled after node creation
- model/node/leaf/face references are range-checked during load

### Visibility and lighting

`LeafPVS` provides decompressed visibility masks on demand. When no visibility data exists for a leaf, the package falls back to an all-visible mask consistent with classic Quake conventions.

`.lit` application is optional and only succeeds when:
- the file starts with `QLIT`
- the version is supported
- the sample payload size exactly matches the expected RGB expansion of existing BSP light samples

## Constraints

- Only BSP29, 2PSB, BSP2, and Quake64 are accepted.
- Lump sizes must match the expected struct size multiples for the chosen path.
- The loader assumes leaf 0 is the solid leaf and uses the world model’s visleaf count conventions.
- Some consumers rely on the raw `File` path while others rely on the normalized `Tree` path; both are part of the public contract today.

## Decisions

### Separate raw and normalized loaders

Observed decision:
- The Go package exposes both raw `File` loading and normalized `Tree` loading instead of one unified model-load path.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- version-specific collision data can remain close to the file format
- renderer/server-facing code can use a more uniform runtime structure

Rejected alternatives:
- **Single monolithic brush-model load path** — closer to the original C structure, but not the observed Go design

### Hard load failures on malformed references

Observed decision:
- The normalized loader returns hard errors for many malformed indices and references.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- invalid BSP data fails early during load instead of being tolerated and repaired later

Rejected alternatives:
- **Warn and remap invalid references to fallback nodes/leaves** — a behavior closer to some original C paths, but not the general strategy observed here
