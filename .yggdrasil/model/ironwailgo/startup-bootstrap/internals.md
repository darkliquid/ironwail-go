# Internals

## Logic

Bootstrap starts with parsed startup options, then initializes base input/UI state, filesystem, QC, networking/server, renderer, and the host subsystem bundle. One crucial invariant is that the authoritative server VM is created by `server.NewServer()` and then adopted as `g.QC`, rather than maintaining a parallel startup-owned server VM. Once the renderer exists, it may provide an input backend; SDL3 can optionally override or fill in missing backend behavior. Startup also wires menu save-slot/mod providers, loopback client/server integration, gameplay bindings, archived startup cvars, and audio initialization through `host.Init`.

## Constraints

- Startup sequencing matters because later subsystems depend on earlier outputs (e.g. filesystem before `progs.dat`, server before authoritative QC, renderer before renderer-backed input).
- Renderer creation can fail and may trigger a headless fallback path at the app-shell level.
- Backend-selection policy is platform-sensitive, especially around GoGPU/X11 and SDL3 overrides.

## Decisions

### Make the server-owned QC VM authoritative during bootstrap

Observed decision:
- App bootstrap explicitly discards a parallel server-side QC ownership path and adopts `g.Server.QCVM` as the authoritative server VM.

Rationale:
- **unknown — inferred from code comments, not confirmed by a developer**

Observed effect:
- App startup follows the same QC ownership path as host/server tests and runtime behavior, reducing divergence between bootstrap and direct host/server execution.
