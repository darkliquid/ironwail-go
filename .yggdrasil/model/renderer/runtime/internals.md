# Internals

## Logic

This layer defines the package-level renderer role without tying callers to one concrete backend. It also carries the stub path that lets the engine boot and exercise rendering-adjacent code without a full graphics backend.

## Constraints

- Backend-specific state must stay below this layer.
- Render-pass classification logic is shared so multiple backends can be compared against common expectations.

## Decisions

### Stub renderer as first-class fallback

Observed decision:
- The package includes a real stub/headless path rather than failing outright when no backend is available.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
