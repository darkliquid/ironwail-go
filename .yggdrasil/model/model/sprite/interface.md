# Interface

## Main consumers

- runtime asset loaders and renderer code that parse `.spr` data.
- tests that inspect sprite frame/group shapes and sync semantics.

## Main surface

- `LoadSprite`
- runtime sprite structs from the parent node (`MSprite`, `MSpriteFrame`, `MSpriteGroup`, `MSpriteFrameDesc`)

## Contracts

- `LoadSprite` returns an `MSprite` whose `Frames` slice matches `NumFrames` and whose per-entry `FramePtr` must be interpreted according to `Type`.
- Single frames become `*MSpriteFrame`; grouped and angled frames become `*MSpriteGroup`.
- Angled groups must contain exactly 8 frames.
- Group intervals must all be strictly positive.
