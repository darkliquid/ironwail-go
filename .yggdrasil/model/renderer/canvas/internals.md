# Internals

## Logic

This layer computes logical-to-physical drawing transforms and shared camera/screen helpers that let multiple backends expose the same 2D/3D coordination model.
It also centralizes renderer cvar-name constants so startup/bootstrap and backend-specific render paths use a single authoritative identifier set for features such as fast sky (`r_fastsky`) and sky fog (`r_skyfog`).

## Constraints

- Canvas semantics must stay consistent across backends.
- Screen and camera helpers are parity-sensitive because many HUD/menu and world computations assume Quake-like coordinate spaces.

## Decisions

### Backend-neutral canvas contract

Observed decision:
- Canvas and config contracts are defined independently of any one graphics backend.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
