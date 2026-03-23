# Responsibility

## Purpose

The `audio` node is the umbrella for the engine sound subsystem. It covers runtime sound orchestration, sample caching and mixing, concrete backend integration, and music playback support.

## Owns

- The top-level decomposition of audio concerns into runtime orchestration, mixing/cache logic, backend integration, and music playback.
- Package-level responsibility for converting gameplay sound events into backend-fed sample streams.

## Does not own

- Host lifecycle policy beyond integrating with the host audio interface.
- Renderer or world logic for choosing listener position or ambient leaf data; those are inputs provided by other systems.
- Filesystem policy for resolving assets; callers provide sound/music loaders.
