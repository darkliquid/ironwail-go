# Internals

## Logic

### Startup sequence

`Host.Init` applies parameters, clamps host-wide limits, resolves the user directory, initializes subsystems in a fixed order, then runs the startup config chain. The sequence is policy, not convenience: later steps assume earlier services already exist.

Startup config ingestion routes script text through the shared command-buffer path (`executeConfigText` → `cmdsys` insert/execute), so comment stripping and command splitting semantics are inherited from cmdsys parser rules. This keeps host `exec` behavior aligned with C-style comment handling in scripted command buffers.

Host cvar registration includes gameplay-fix toggles used by server/QC parity paths, including `sv_gameplayfix_random` (default `1`) that selects QC `random()` formula behavior. It also registers the `devstats` cvar so user config/console flows can control developer stats surfaces with parity-friendly naming.

Host command registration is unconditional during `Init`, and runtime now also invokes cvar helper command registration (`cvarlist`, `toggle`, `cycle`, `cycleback`, `inc`, `reset`, `resetall`, `resetcfg`) at startup. The optional `Subsystems.Commands` wrapper only controls how buffered command text is executed/inserted; the host command surface itself is always bound into the global `cmdsys` so localcmd/changelevel-style paths work even in embedded or test harness setups that leave `Subsystems.Commands` nil.

Server-browser network advertisement wiring (`updateServerBrowserNetworking`) now enables UDP listen before installing a `ServerInfoProvider`. If listen startup fails (accept socket cannot bind/open), host runtime clears provider state and keeps LAN advertisement disabled instead of exposing stale/partial server info. The provider includes both summary server info and per-player row callbacks (slot/name/colors/frags/ping) so datagram control queries can answer remote `players` requests without exposing full server internals through the host command layer.

Host runtime also owns cross-module callback slots such as `SetGameDirChangedCallback`, which lets executable bootstrap install mod-switch follow-up behavior (for example draw asset reload and renderer palette/conchars refresh) without giving host/runtime a direct dependency on draw or renderer packages. It now also exposes a narrow `ClientSessionActive()` accessor so executable render glue can key scene visibility off authoritative host session state instead of stale client-object residue after failed transitions.

### Frame scheduling

`Host.Frame` preserves the classic host order:
- gather events
- process console commands
- send client command
- advance server frame
- read from server
- update screen
- update audio

`Host.Frame` advances the shared compat-rand stream once at frame entry (`h.compatRNG.Int()`) before any callback phases. Because the same RNG instance is injected into server and QC VM paths during init, this early draw is the deterministic upstream offset source for all later frame-local compat-rand consumers (`SV_MoveToGoal` branches and QC `random()`).

For loopback clients, send command construction now performs send-time one-shot input latching (attack/jump/impulse) through client runtime helpers, matching remote send semantics and C engine timing.
Remote datagram client frame accumulation now mirrors C signon gating by skipping `AccumulateCmd` until `Signons` is complete; this prevents pre-signon movement intent from being accumulated in remote sessions.

When `maxFPS` is above 72 or invalid, network simulation is isolated to a `1/72` interval. When `maxFPS` is at or below 72, simulation may run every frame.
`SetMaxFPS` is the authoritative derivation point for that policy (`netInterval`), and host runtime now exposes both `NetInterval()` and `LocalServerFast()` so callers can consume the exact derived state without duplicating threshold logic.

### Shutdown

`Host.Shutdown` tears down client, server, console, commands, audio, input, renderer, then filesystem. This ordering mirrors dependency direction during runtime ownership.

## Constraints

- `maxClients` is clamped into `[1, MaxScoreboard]`.
- Dedicated mode and `deathmatch` policy are derived from init parameters.
- `userDir` must exist before host-managed config and save paths can be used.
- Abort state short-circuits the frame loop and later surfaces as a host-level error.

## Decisions

### Explicit host object and subsystem container

Observed decision:
- The Go port uses `Host` plus `Subsystems` instead of a broad global control block.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime lifetime is explicit
- startup and shutdown order are centralized
- executable wiring is testable and replaceable
