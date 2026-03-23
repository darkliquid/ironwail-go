# Responsibility

## Purpose

`model/sprite` owns Quake sprite-file parsing and the conversion of on-disk frame/group records into runtime `MSprite`, `MSpriteFrame`, and `MSpriteGroup` structures.

## Owns

- `LoadSprite` and its frame/group parsing helpers.
- Validation of sprite ident/version, dimensions, frame counts, group intervals, and angled-group shape.
- Conversion of sprite origins/dimensions into `Up`, `Down`, `Left`, `Right`, `SMax`, and `TMax` values.
- Power-of-two padding helper used to derive normalized texture extents.

## Does not own

- Sprite animation playback timing outside the stored intervals/sync type.
- Wrapping sprite payloads into higher-level runtime entity/model containers.
