# Responsibility

## Purpose

`model/alias` owns Quake MDL (alias model) parsing and the helper logic that decodes compressed vertex data, normals, grouped skins, grouped frames, and derived alias-model bounds.

## Owns

- `LoadAliasModel`, the authoritative MDL-to-`*Model` loader.
- Reading and validating on-disk MDL headers, skins, ST verts, triangles, frames, and poses.
- Flattening grouped skins into `AliasHeader.Skins` while preserving logical grouping in `SkinDescs`.
- Computing `Model.Mins/Maxs`, `YMins/YMaxs`, and `RMins/RMaxs` from decoded pose vertices.
- Shared MDL-format structs/constants (`MDLHeader`, `STVert`, `DTriangle`, `TriVertX`, frame/skin markers, normals table, decode helpers).

## Does not own

- Renderer upload/skin translation for alias models after parsing.
- Higher-level model caching or filesystem lookup.
