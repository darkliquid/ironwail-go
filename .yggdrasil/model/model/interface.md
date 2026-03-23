# Interface

## Main consumers

- model loading code that constructs `Model`, `AliasHeader`, and `MSprite` values.
- server collision code that consumes the collision adapter on `Model`.
- renderer code that inspects alias/sprite payloads and model bounds.

## Main surface

- Core types: `Model`, `ModelType`, `SyncType`, `Texture`, `MPlane`, `MNode`, `MLeaf`, `Hull`, `AliasHeader`, `MSprite`, `MSpriteFrame`, `MSpriteGroup`
- Shared helpers: `AliasHeader.ResolveSkinFrame`, `SideFromPlane`
- Collision-facing methods on `Model`: `ModelType`, `NumHulls`, `Hull`, `CollisionClipNodes`, `CollisionPlanes`, `IsClipBox`, `CollisionClipMins`, `CollisionClipMaxs`

## Contracts

- `Model` is a shared envelope for brush and alias data; alias payload hangs off `AliasHeader`, while sprite loaders return `*MSprite` directly.
- `AliasHeader.Skins` stores flattened skin-frame payloads, while `SkinDescs` preserves logical grouped-skin structure.
- `MSpriteFrameDesc.FramePtr` is a tagged union encoded as `interface{}` and must be type-switched by consumers.
- `SideFromPlane` classifies equality as front, never `SideOn`.
