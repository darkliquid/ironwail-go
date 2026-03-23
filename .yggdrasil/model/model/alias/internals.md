# Internals

## Logic

`LoadAliasModel` is the real production MDL loader. It reads and validates the MDL header, parses skins/ST verts/triangles, then reads frames and pose vertices while simultaneously deriving bounds from decoded vertices. Grouped skins are expanded into a flat `[][]byte` payload for easy renderer access, but `SkinDescs` preserves the original logical grouping and interval schedule. Grouped frames behave similarly: `AliasFrameDesc` stores the logical frame with `FirstPose`/`NumPoses`, while raw pose vertices are appended to `AliasHeader.Poses`. Bounds are derived in three forms: axis-aligned bounds, yaw-only bounds, and full rotational-radius bounds.

`mdl.go` also contains lower-level MDL reader helpers plus `LoadMDL`. Those helpers describe the file format and expose shared decode tables/functions, but `LoadMDL` is not the authoritative runtime path: it skips most payload details and returns a partial `AliasHeader`, while production callers use `LoadAliasModel`.

## Constraints

- Alias models must have at least one skin, one triangle, one frame, and a positive vertex count that does not exceed `MaxAliasVerts`.
- Total pose count must stay within `MaxAliasFrames`.
- Grouped-skin intervals are preserved exactly; `ResolveSkinFrame` first prefers explicit interval schedules and falls back to tenths-based cycling only when necessary.
- Bounds are derived from decoded vertices, not trusted from on-disk bbox fields alone.
- `skipAliasSkins`, `skipAliasSTVerts`, and `skipAliasTriangles` exist as helpers but are currently unused by the main load path.

## Decisions

### Keep both a format-oriented MDL helper layer and a higher-level loader

Observed decision:
- The package exposes both a low-level `MDLReader`/`LoadMDL` path and the richer `LoadAliasModel` path.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The package can document raw MDL structures and provide small helper APIs, but the true runtime contract lives in `LoadAliasModel`, so graph artifacts must not treat `LoadMDL` as equally authoritative.
