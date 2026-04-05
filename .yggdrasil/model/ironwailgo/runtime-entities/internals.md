# Internals

## Logic

This node converts live client/server state into the entity lists the renderer expects. It resolves model names from client precaches, distinguishes brush submodels from alias and sprite models, lazily loads/caches alias and sprite payloads through the filesystem, and emits renderer-facing entity structs for world entities, static entities, beams, particles, and lights. Debug-view telemetry hooks record why entities were drawn or skipped.
Before the renderer-facing frame state samples `Client.CurrentFog`, this node now gives the client a chance to seed map-load fog defaults from `g.Server.WorldTree.Entities`. That keeps renderer fog state aligned with C/Ironwail worldspawn fog without bypassing the client's shared fade logic.
The renderer-facing frame-state bridge now also suppresses world/entities/particles/decals entirely unless host session state still reports an active gameplay/demo scene. That prevents stale draw lists from surviving failed map startups or disconnect-driven mod switches into the fresh menu.

## Constraints

- Dynamic entity freshness is gated by `MsgTime` matching the current client message time; stale dynamic entities are intentionally skipped.
- Effect-source collection resolves model flags before final inclusion, and now preserves rocket sources when `model.EFRocket` is present even with zero effect bits.
- Runtime caches are command-package-local convenience caches, not general asset managers.
- Entity collection depends on both client precache state and server/world state, making it a cross-subsystem translation layer.
- Sprite cache construction must preserve frame pixel payload end-to-end: parsed `*model.MSprite` stays attached to the cached runtime model (`Model.SpriteData`) in addition to the parallel sprite pointer used by sprite-entity collection.

## Decisions

### Filter stale dynamic entities before presentation while still keeping static entities visible

Observed decision:
- Dynamic entity collectors require current message time, but static entities are still eligible for collection.

Rationale:
- **unknown — inferred from code and telemetry hooks, not confirmed by a developer**

Observed effect:
- Presentation avoids rendering stale dynamic state after packet/entity churn, but the exact freshness rule is subtle and important for debugging visibility/parity issues.
