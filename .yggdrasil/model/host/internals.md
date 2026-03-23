# Internals

## Logic

### Startup

`Host.Init` is the main startup sequence.

Observed order:
1. Register host CVars.
2. Copy init parameters onto host state.
3. Clamp host-level limits such as max clients.
4. Resolve and create `userDir`.
5. Initialize filesystem.
6. Initialize command system and register host commands.
7. Initialize console.
8. Initialize server, including compatibility RNG injection when supported.
9. Create a loopback client automatically when a server exists in non-dedicated mode but no client was supplied.
10. Initialize client.
11. Initialize audio, warning instead of failing on audio startup errors.
12. Initialize renderer.
13. Execute `quake.rc` when it exists in the game filesystem; otherwise fall back to the user config.

This ordering is a host invariant because later stages assume earlier services already exist.

### Per-frame sequencing

`Host.Frame` preserves the classic Quake orchestration pattern while making the phases explicit:
- read platform/input events
- process buffered console commands
- send client command
- advance server frame when active
- perform host-side autosave checks after the server frame
- read messages from the server when connected
- update screen
- update audio

When `maxFPS` exceeds 72 or is invalid, the host isolates client/server simulation on a fixed `netInterval` of `1/72`. When `maxFPS` is at or below 72, network simulation is allowed to run every frame. This means rendering cadence and gameplay/network cadence can diverge intentionally.

### Session modes

The host supports two structurally different client paths:

- **Local loopback session**
  - Used for non-dedicated local play.
  - Host owns the bootstrap path and synchronous signon/session transition.

- **Remote datagram session**
  - Implemented by `remoteDatagramClient`.
  - Wraps a concrete client/parser pair and a transport socket.
  - Parses incoming messages, tracks signon stage, and sends `prespawn`, `name`/`color`/`spawn`, then `begin` automatically.

### Config and command execution

Host is responsible for startup script policy, not for command parsing itself.

Observed startup config chain:
- Prefer `exec quake.rc` through the game filesystem.
- If `quake.rc` is unavailable, fall back to user config files.
- Archived cvars may be loaded from user configs before full runtime startup completes.

### Autosave policy

Autosave is intentionally conservative and combines host time, server time, and player state. It only triggers when all of the following general conditions are satisfied:
- active single-player session
- server active and signon complete enough to be in play
- player exists and is alive
- autosave cvars enable it
- not in intermission
- not in noclip/godmode/notarget
- not recently hurt or shooting
- not moving too fast
- enough effective safe time has accumulated

The host also boosts autosave score briefly after secrets and recent teleports.

## Constraints

- `maxClients` is clamped into `[1, MaxScoreboard]`.
- `deathmatch` is derived from `maxClients`.
- Dedicated mode and user directory resolution are host-owned policy.
- Save names are sanitized and constrained so saves remain inside host-managed save directories.
- Rich save/load and autosave behavior currently depends on concrete server/filesystem implementations, not only interfaces.
- Abort state suppresses normal frame work and is surfaced as a host-level error by the frame loop.

## State

Important host-owned state groups:
- timing and frame cadence: `realtime`, `frameTime`, `rawFrameTime`, `netInterval`, `accumTime`
- session state: `serverActive`, `serverPaused`, `clientState`, `signOns`, `currentSkill`, `spawnArgs`
- identity and dirs: `baseDir`, `gameDir`, `userDir`, `args`
- UI/runtime overlays: loading plaque and saving icon state
- demo loop state
- autosave heuristics state
- compatibility RNG and subsystem container

## Decisions

### Explicit host object plus subsystem container

Observed decision:
- The Go port uses a `Host` struct with a `Subsystems` container instead of the broad global host/control state style used by the original C engine.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- subsystem lifetime is explicit
- startup/shutdown order is centralized
- testing and replacement of concrete services are easier

Rejected alternatives:
- **Original C-style global control block** — still the lineage, but not the chosen implementation shape in Go

### Non-fatal audio initialization

Observed decision:
- Audio initialization failures emit a warning instead of aborting host startup.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the engine can still boot in environments where audio backend setup fails

Rejected alternatives:
- **Fail whole host startup on audio init error** — not chosen by the current code

### Synchronous local session bootstrap

Observed decision:
- Local loopback sessions are bootstrapped directly by host/session commands instead of relying purely on the older implicit global reconnect style.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- local map/changelevel/load flows are more explicit in Go
- host owns more of the transition logic directly

Rejected alternatives:
- **Defer local session transitions entirely to a reconnect-driven path** — closer to older engine structure, but not the observed Go approach
