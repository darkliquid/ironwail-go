# Interface

## Main consumers

- runtime model-loading code that parses `.mdl` data.
- renderer code that consumes `AliasHeader`, decoded poses, normals, ST verts, and triangles.

## Main surface

- `LoadAliasModel`
- `LoadMDL`
- `DecodeVertex`, `GetNormal`, `Float32FromBits`
- MDL on-disk structs and markers such as `MDLHeader`, `STVert`, `DTriangle`, `TriVertX`, `AliasFrameType`, `AliasSkinType`

## Contracts

- `LoadAliasModel` returns a full `*Model` with `Type == ModAlias` and `AliasHeader` populated.
- Grouped skins are flattened into `AliasHeader.Skins`; callers must use `ResolveSkinFrame` with `SkinDescs` to recover timed logical skin selection.
- Grouped alias frames contribute multiple poses to `AliasHeader.Poses` and one logical `AliasFrameDesc` entry.
- Invalid MDL ident/version/counts, empty geometry, and excessive pose counts are rejected with explicit errors.
