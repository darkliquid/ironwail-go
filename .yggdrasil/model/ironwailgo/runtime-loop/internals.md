# Internals

## Logic

The runtime loop implements `host.FrameCallbacks` and decides what happens each frame. It polls input/events, executes console commands, advances the server, and then processes the client according to the configured phase ordering. In live play, client read/send ordering is explicit and paired with host/client activation synchronization and loopback stufftext dispatch. In demo playback, the loop bypasses normal networked gameplay, advances demo timing, parses recorded messages, handles rewind/EOF behavior, and bootstraps demo world state when needed. Playback stop paths now route through demo stop-with-summary helpers so timedemo benchmark output is emitted consistently across EOF, rewind error, and explicit teardown flow. Shutdown complements startup by first closing any active manual CPU profile, then tearing down renderer/audio/networking and writing config.
`syncHostClientState` is also responsible for mirroring host-derived interpolation policy into the active client state; it now propagates `Host.LocalServerFast()` to `Client.LocalServerFast` so LerpPoint fast-bypass behavior tracks host net-interval policy during local server play.
The loading-plaque helper now returns immediately when either the render context or pic provider is nil. This aligns runtime-loop overlay helpers with the existing pause-overlay nil-safety and prevents panic-on-missing-context behavior in focused runtime/UI code paths.

## Constraints

- Frame-phase ordering is test-backed and should not be treated as incidental.
- Demo playback is not a thin wrapper around live networking; it has bespoke rewind, EOF, and bootstrap rules.
- Runtime callbacks are orchestration code and therefore depend on state produced by many subsystems without owning those subsystem implementations.

## Decisions

### Encode frame ordering explicitly in callback logic instead of hiding it inside one subsystem

Observed decision:
- The command package keeps the authoritative runtime order in its host callback implementation.

Rationale:
- **unknown — inferred from code and tests, not confirmed by a developer**

Observed effect:
- The composition root remains responsible for subtle ordering guarantees, which need graph artifacts because they are not obvious from any single subsystem alone.
