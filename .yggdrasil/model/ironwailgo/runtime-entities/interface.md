# Interface

## Main consumers

- runtime visual/presentation code that submits collected entities to the renderer.

## Main surface

- entity collection helpers for brush, alias, sprite, beam, particle, and light data
- runtime model cache/load helpers used during collection

## Contracts

- Dynamic entity collectors skip stale client entities by requiring `state.MsgTime == g.Client.MTime[0]`.
- `collectEntityEffectSources` keeps entities when either effect bits are present **or** the resolved model flags include `model.EFRocket`, so rocket dlights are not dropped when `state.Effects == 0`.
- Static entities are still considered even when dynamic-state freshness checks would skip runtime entities.
- Alias and sprite collection may lazily load runtime model data through the filesystem-backed caches.
- Sprite runtime cache entries keep parsed payload in two places: `runtimeSpriteModel.sprite` and `runtimeSpriteModel.model.SpriteData`, so downstream paths that only carry `*model.Model` still retain raw sprite frame pixels.
