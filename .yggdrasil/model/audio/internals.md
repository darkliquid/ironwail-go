# Internals

## Structure

The package is split into focused concerns:
- `audio/runtime` — host-facing system orchestration and channel lifecycle
- `audio/mixing` — sample cache, WAV decode, spatialization, and paint pipeline
- `audio/backends` — hardware/null backend implementations and DMA behavior
- `audio/music` — streamed-track and codec handling for music playback

## Decisions

### Split runtime, backend, and music concerns inside one package

Observed decision:
- The Go package keeps one public `audio` package but separates the implementation into runtime, mixing, backend, and music files.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- host integration stays simple while implementation concerns remain factored by responsibility
