# Internals

## Logic

This layer handles entity and effect categories that require OpenGL-specific draw code, especially alias models, sprites, decals, and transparency strategies.

## Constraints

- Transparency/OIT behavior is strongly backend- and parity-sensitive.
- Atlas/scrap behavior must stay aligned with the OpenGL texture path.

## Decisions

### Dedicated OpenGL entity/effect slice

Observed decision:
- OpenGL world geometry and OpenGL entity/effect rendering are split into separate nodes.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
