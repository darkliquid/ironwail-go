# Internals

## Logic

This node turns both BSP geometry and bounding boxes into hull-based collision inputs so the rest of the server can reason about movement uniformly. It maintains area nodes for broadphase filtering, traverses BSP data for world traces and point contents, and links entities into the spatial structure so later traces and trigger scans see current authoritative state.

## Constraints

- Hull choice and offset math are parity-sensitive for collision correctness.
- Relink operations must keep leaf/PVS-related bookkeeping in sync with abs bounds and solids.
- Trigger touch discovery depends on world linkage staying current after origin/size/model changes.

## Decisions

### Uniform hull-based world and box collision model

Observed decision:
- The package converts bbox collision into temporary hulls instead of maintaining a separate collision path.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Collision logic stays conceptually aligned with Quake's BSP-centric tracing model at the cost of some setup work per query.
