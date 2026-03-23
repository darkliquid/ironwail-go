# Behavioral Differences: Go Port vs C Ironwail (Reference)

> The C Ironwail engine at `/home/darkliquid/Projects/ironwail` is the **canonical reference implementation**. Every divergence in the Go port documented here represents a **potential risk** to correctness, compatibility, or gameplay fidelity.

---

## Risk Severity Key

- **CRITICAL** — Will cause crashes, data corruption, protocol incompatibility, or fundamentally broken gameplay
- **HIGH** — Gameplay-affecting behavioral difference that players will notice
- **MEDIUM** — Subtle behavioral difference; may affect edge cases, mods, or specific maps
- **LOW** — Minor difference unlikely to affect normal gameplay but diverges from reference

---

## Recently Verified Fixes

These items were previously parity-sensitive and are now verified fixed in the Go port. Keep them out of new parity bug triage unless they regress again.

- **Intermission progression / loopback changelevel dispatch**: `Host.Init` now always registers host commands in the global command system, and host command rebinding refreshes stale closures on re-registration. Intermission input no longer gets stuck behind missing `changelevel` handling in loopback-style setups.
- **Temp-entity protocol flag decoding**: client temp-entity parsing now reads coordinates through the parser's protocol-aware helpers, so beam/explosion payloads honor `ProtocolFlags` just like main entity updates.
- **qgo emitted progs CRC**: `cmd/qgo/compiler` now writes the canonical `qc.ProgHeaderCRC` into emitted `progs.dat` headers, and compiler round-trip tests assert that the loader sees the expected value.

---

## Table of Contents

1. [Physics & Collision](#1-physics--collision)
2. [Network & Protocol](#2-network--protocol)
3. [QuakeC VM](#3-quakec-vm)
4. [Client](#4-client)
5. [Server](#5-server)
6. [Audio](#6-audio)
7. [Renderer](#7-renderer)
8. [Console, Cvars & Commands](#8-console-cvars--commands)
9. [Filesystem](#9-filesystem)
10. [Input](#10-input)
11. [Menu](#11-menu)
12. [BSP & Model Loading](#12-bsp--model-loading)
13. [Memory Management](#13-memory-management)

---

## 1. Physics & Collision

### MEDIUM: anglemod() Implementation Differs

**C** (`mathlib.c:92-102`): Uses 16-bit quantization:
```c
a = (360.0/65536) * ((int)(a*(65536/360.0)) & 65535);
```
This maps the angle through a 16-bit integer (65536 steps) and back, producing specific quantization behavior.

**Go** has **two different implementations**:
- `pkg/types/types.go:220`: Uses 8-bit quantization (`256` steps) — `int(angle*256.0/360.0)&255) * (360.0/256.0)`. This is coarser than C's 16-bit.
- `internal/client/relink.go:100`: Uses continuous `math.Mod(a, 360.0)` — no quantization at all.

**Risk**: The C `anglemod` is used in monster pathfinding (`SV_NewChaseDir`), entity rotation (`bobjrotate`), and QC `changeyaw` builtin. Different quantization means:
- Monster turning behavior will differ subtly (45-degree snapping uses different precision)
- Rotating entity angles won't match C exactly
- Any code path using the 8-bit version will have 4x less precision than C's 16-bit version
- The `math.Mod` version in relink.go doesn't quantize at all, producing floating-point drift that C avoids

---

### MEDIUM: Custom Entity Gravity Field Not Supported

**C** (`sv_phys.c:373-385`): Looks up a custom `"gravity"` field on each entity via `GetEdictFieldValueByName(ent, "gravity")`. If present and nonzero, uses it as a multiplier; otherwise defaults to 1.0.

**Go** (`physics.go:92-95`): Always uses gravity multiplier of 1.0. No field lookup.

**Risk**: Entities with custom gravity (e.g., flying dragons, low-gravity areas in mods) will fall at normal speed. Affects mods that set per-entity gravity.

---

### MEDIUM: Elevator Gameplay Fix Missing (sv_gameplayfix_elevators)

**C** (`sv_phys.c:552-578`): In `SV_PushMove`, when a pusher (door/elevator) can't push an entity clear, the `sv_gameplayfix_elevators` logic nudges entities upward slightly to prevent crushing.

**Go** (`physics.go:278-387`): Missing entirely.

**Risk**: Players or monsters riding elevators may get stuck or crushed in edge cases that C handles gracefully. Affects maps with tight elevator clearances.

---

### MEDIUM: SendInterval Calculation Simplified

**C** (`sv_phys.c:1284-1289`): Calculates precise lerp interval for FitzQuake protocol based on entity think timing, producing accurate client-side interpolation.

**Go** (`physics_loop.go:48-50`): Simplified to `SendInterval = true` only when `NextThink > Time`. No precise interval calculation.

**Risk**: Client-side entity interpolation timing will be less accurate. May cause visible jerking or snapping of entity movement, especially for entities with irregular think intervals.

---

### MEDIUM: Collision Detection Precision Downgrade

**C** (`world.c:556-581`): Uses `DoublePrecisionDotProduct()` (64-bit) for plane distance calculations on non-axial planes in `SV_HullPointContents`.

**Go** (`world.go:192-226`): Uses float32 throughout for all plane distance calculations.

**Risk**: Precision differences in collision detection for rotated brush models. Standard Quake maps use axial planes (no difference), but custom maps with rotated brushes may exhibit clipping errors.

---

### LOW: Water Transition Tracking Missing

**C** (`sv_phys.c:1018-1022`): Tracks entering/leaving water state transitions for the player entity during `SV_Physics_Client`.

**Go** (`physics.go:607-633`): Not implemented.

**Risk**: Water entry/exit state may not be correctly communicated to the client. Could affect underwater sound transitions or view effects.

---

### LOW: Pusher Blocked Callback Possibly Incomplete

**C** (`sv_phys.c:588-595`): Calls the pusher's `blocked` QC function when a door/elevator is blocked by an entity.

**Go** (`physics.go:374-378`): Code exists but may not fully implement the callback chain.

**Risk**: Doors that should reverse when blocked may not reverse, potentially trapping players.

---

### LOW: Physics Statistics Counters Absent

**C** (`sv_move.c`): Tracks `c_yes`, `c_no` counters for CheckBottom statistics.

**Go** (`movement.go`): No equivalent counters.

**Risk**: No gameplay impact. Debug/profiling information unavailable.

---

## 2. Network & Protocol

### RESOLVED: Coord/Angle Encoding Now Honors Protocol Flags

This warning is stale.

**Current Go**:
- `internal/server/message.go:149-177` routes `WriteCoord` / `WriteAngle` through the correct protocol-flag-aware encodings.
- FitzQuake with zero flags uses the same 16-bit fixed-point coord / 8-bit angle defaults as C.
- RMQ `serverinfo` includes `ProtocolFlags()` (`internal/server/sv_client.go:50-54`), and the client parser mirrors those flags via `Parser.readCoord` / related helpers (`internal/client/parse.go:451-479`).

The previous “always float32” server behavior no longer exists.

---

### RESOLVED: ServerInfo Sends Protocol Flags for RMQ

This warning is stale.

**Current Go** (`internal/server/sv_client.go:50-55`): `SendServerInfo` writes the selected protocol number and, when `s.Protocol == ProtocolRMQ`, appends `s.ProtocolFlags()` before `maxclients`, matching the C RMQ handshake layout.

**Regression coverage** (`internal/server/sv_client_test.go`):
- `TestSendServerInfoFitzQuakeOmitsProtocolFlags`
- `TestSendServerInfoRMQIncludesProtocolFlags`

---

### RESOLVED: Datagram Accept Path Uses Per-Client Sockets and Reports the Correct Port

These two networking warnings are stale.

**Current Go** (`internal/net/datagram.go:526-552`): the datagram driver opens a fresh UDP socket with `UDPOpenSocket(0)` for each accepted client, stores that socket on the accepted server-side connection, and returns that socket's port in `CCREP_ACCEPT`. The accept socket remains handshake-only, matching C's `Open_Socket(0)` + accept-reply behavior.

**Regression coverage** (`internal/net/net_test.go`):
- `TestConcurrentClientsGetDistinctSockets`
- `TestCCRepAcceptReportsPerClientSocketPort`

---

### RESOLVED: MaxMessage Matches C NET_MAXMESSAGE

This warning is stale.

**Current Go** (`internal/net/types.go:45`): `MaxMessage = 65535`, matching C's `NET_MAXMESSAGE`, and socket buffers are allocated to that exact size. `internal/server/types_protocol.go:349` separately matches C's raised `MAX_DATAGRAM = 64000` for unreliable datagrams.

**Regression coverage** (`internal/net/types_test.go`):
- `TestMaxMessageMatchesCNETMAXMESSAGE`
- `TestNewSocketAllocatesMaxMessageBuffers`

---

### RESOLVED: Datagram Connect Timeout Matches C

This warning is stale.

**Current Go** (`internal/net/datagram.go:404-415`): datagram connect retries 3 times with a 2.5 second timeout per attempt, matching C's handshake window closely enough for parity.

---

### RESOLVED: Duplicate Datagram Connections Are Closed on Reconnect

This warning is also stale.

**Current Go** (`internal/net/datagram.go:446-471,526-529`): accepted datagram sockets are tracked, and reconnects from the same remote UDP endpoint close stale server-side sockets before accepting the replacement.

**Regression coverage** (`internal/net/net_test.go`):
- `TestDuplicateConnectClosesOldServerSocket`

---

### MEDIUM: IP Banning Not Implemented

**C**: `NET_Ban_f()` provides IP-based banning.

**Go**: No equivalent.

**Risk**: Servers cannot ban disruptive players. Affects multiplayer administration.

---

### MEDIUM: Missing Network Debug/Query Commands

**C**: `test`, `test2` console commands for querying player info (`CCREQ_PLAYER_INFO`) and server cvars (`CCREQ_RULE_INFO`). `NET_Stats_f()` for network statistics.

**Go**: None of these implemented.

**Risk**: Cannot debug network issues or query remote server state.

---

### LOW: Server List Not Sorted

**C**: `NET_SlistSort()` sorts discovered servers.

**Go**: Results returned unsorted from `ServerBrowser`.

**Risk**: Server browser displays servers in random/discovery order.

---

### LOW: No Partial IP Address Resolution

**C**: `PartialIPAddress()` allows abbreviated IPs (e.g., "192" expands to full address).

**Go**: Not implemented.

**Risk**: Users cannot use abbreviated IP addresses to connect.

---

## 3. QuakeC VM

### HIGH: Builtin Coverage Still Incomplete, But Vanilla-Critical Builtins Are Present

This section was previously overstated. The Go VM now includes the vanilla-critical builtins that would immediately break standard Quake gameplay or HUD logic, including `ftos`, `vtos`, `sin`, `cos`, `sqrt`, `pow`, `stof`, `strlen`, `strcat`, `substring`, `stov`, `strzone`, `strunzone`, `etos`, `mod`, and extended trig helpers.

The remaining risk is narrower:
- some extended / mod-oriented builtins are still missing
- CSQC-oriented builtins are present only as partial support or stubs
- mod compatibility is still incomplete even though the worst “vanilla Quake is broken” case no longer applies

So builtin coverage remains a real parity gap, but it is no longer accurate to describe standard `progs.dat` HUD behavior as broken solely because `ftos` / `vtos` are absent.

---

### RESOLVED: OPCall No Longer Skips the First Statement

This warning is stale.

**Current Go** (`internal/qc/exec.go:475-483`): `callFunction` sets `vm.XStatement = int(f.FirstStatement) - 1` specifically to compensate for the main execution loop increment, matching the C `first_statement - 1` convention.

---

### RESOLVED: OPReturn Copies the Return Value into OFS_RETURN

This warning is stale.

**Current Go** (`internal/qc/exec.go:123-128`): the `OPReturn` path copies operand `A` into `OFS_RETURN` / `OFS_RETURN+1` / `OFS_RETURN+2`, matching the C vector-safe return-copy behavior.

---

### RESOLVED: Parameter Passing and Local Save/Restore Are Implemented

This warning is stale.

**Current Go** (`internal/qc/loader.go:104-157`): `EnterFunction` saves locals to `LocalStack` and copies `OFS_PARM*` values into the callee's parameter/local space; `LeaveFunction` restores the saved locals back into globals, matching the C stack discipline closely enough for parity.

**Go** (`loader.go:103-122`): `EnterFunction` only tracks `LocalBase` offset and increments `LocalUsed`. **Does not save locals and does not copy parameters.** `LeaveFunction` only decrements `LocalUsed` — **does not restore locals.**

**Risk**: **This breaks ALL non-trivial QuakeC code.** Without local save/restore:
- Any nested function call corrupts the caller's local variables
- Function parameters are not accessible in the called function's local space
- Vanilla Quake's `progs.dat` has deep call chains (e.g., `T_Damage` → `ClientObituary` → `sprint` → `ftos`) — all of these corrupt outer function state
- This makes the QC VM fundamentally incorrect for any program with function calls
- It may appear to work for simple cases where functions don't reuse overlapping global slots, but will produce incorrect behavior for standard Quake gameplay

---

### HIGH: Different Random Number Generation

**C**: Uses `srand()`/`rand()` from libc with Quake-specific seeding.

**Go**: Uses `math/rand.Float32()` with default seeding.

**Risk**: Random sequences will differ between C and Go for the same seed. Affects demo playback reproducibility and any QC code that depends on deterministic randomness.

---

### MEDIUM: Division by Zero Handling Differs

**C**: Division by zero is undefined behavior — silently produces garbage (typically NaN or infinity propagated through subsequent math).

**Go** (`exec.go`): Explicitly returns an error on division by zero, halting VM execution.

**Risk**: QC code that accidentally divides by zero (which works "fine" in C by producing garbage that gets clamped elsewhere) will crash the Go VM. This is arguably safer but is a behavioral difference.

---

### MEDIUM: No Runaway Loop Protection

**C** (`pr_exec.c:425-429`): Counts executed statements and errors after 16M:
```c
if (++profile > 0x1000000)
    PR_RunError("runaway loop error");
```

**Go**: No statement counter or loop limit.

**Risk**: A buggy QC mod with an infinite loop (e.g., `while(1){}`) will hang the engine forever instead of aborting with a clear error. The Go process will have to be killed externally.

---

### LOW: No QC Profiling

**C**: Tracks function profile counts for `pr_profile` console command.

**Go**: No equivalent.

**Risk**: Cannot profile QC performance.

---

## 4. Client

### MEDIUM: LerpPoint Missing timedemo and Local Server Bypass

**C** (`cl_main.c:439`): `CL_LerpPoint` returns 1.0 (no interpolation) in three cases:
```c
if (!f || cls.timedemo || (sv.active && !host_netinterval))
```
This means: when running timedemo, or when the local server is active with no net interval (single-player at high FPS), entities snap to their latest position without interpolation.

**Go** (`client.go:659-683`): Only checks `f == 0`. Missing the `timedemo` and `sv.active && !host_netinterval` special cases.

**Risk**: In single-player with high FPS (no net interval), entities may interpolate when they shouldn't, potentially causing visual artifacts or 1-frame-delayed entity positions. Also affects future timedemo implementation.

---

### MEDIUM: LerpPoint Missing cl_nolerp Support

**C** (`cl_main.c:467-468`): After computing the lerp fraction, checks `cl_nolerp.value` and returns 1.0 if set (disables all entity interpolation).

**Go**: No `cl_nolerp` cvar check.

**Risk**: Players cannot disable entity interpolation. Some speedrunners and competitive players prefer `cl_nolerp 1` for gameplay reasons.

---

### RESOLVED: Rocket Model-Flag Lights Preserved During Effect-Source Collection

`collectEntityEffectSources` now keeps alias-model effect sources when either explicit effect bits are present or resolved model flags include `model.EFRocket`. This closes the prior edge case where rocket dlights could be dropped when `state.Effects == 0`.

**Regression coverage** (`cmd/ironwailgo/main_test.go`):
- `TestCollectEntityEffectSourcesIncludesRocketModelFlagWithZeroEffects`

---

### HIGH: Pitch Drift Cvar Plumbing Gap

**C** (`view.c:184-243`): `V_DriftPitch` behavior is controlled through lookspring/pitch-drift cvars and call-site plumbing.

**Go**: Core pitch-drift behavior exists, but cvar/plumbing parity is still incomplete in the command-shell wiring.

**Risk**: Lookspring users can still hit behavior mismatches in specific configuration paths.

---

### HIGH: Demo Playback Incomplete

**C** (`cl_demo.c`): Full demo playback with rewind support, timedemo FPS benchmarking, and mid-game recording with synthetic state reconstruction.

**Go** (`demo.go`): Frame writing only. No playback, no rewind, no timedemo.

**Risk**: Cannot play back recorded demos. Cannot benchmark FPS with timedemo.

---

### MEDIUM: Temp Entity Beam Roll Jitter Still Differs from C

This warning was previously too broad.

**Current Go**:
- `internal/client/tent.go:199-267` updates active beam state each frame and generates beam segments
- `cmd/ironwailgo/game_entity.go:254-270` feeds those beam segments into alias-model rendering
- `internal/client/tent_test.go` now covers beam parsing, lifetime, segment generation, and protocol-aware coordinate decoding

The remaining divergence is smaller: beam segment roll is currently fixed at `0` in Go, while C randomizes the third angle component for visual jitter. Beam-based effects render; they just do not yet match the C roll variation exactly.

---

### MEDIUM: No View Blend Final Computation

**C** (`view.c`): `V_CalcBlend()` composites all 4 color shift channels (damage, bonus, contents, powerup) into final `v_blend[4]` RGBA for screen overlay.

**Go**: No final blend computation. Individual shifts are calculated but not composited.

**Risk**: Screen damage flash, water tinting, and powerup overlays may not render correctly even when individual channel data exists.

---

### MEDIUM: Client-Side Prediction Is New (Not in C)

**Go** (`prediction.go`): Implements explicit client-side prediction with command buffering, error correction, and movement prediction.

**C**: No explicit prediction in Quake 1 protocol (implicit server-authoritative).

**Risk**: This is a **Go addition**, not a divergence from C behavior. However, bugs in the prediction code could cause visible desync (rubber-banding, jitter) that wouldn't exist in C.

---

### LOW: No Nehahra Protocol Detection

**C** (`cl_parse.c`): Detects Nehahra engine's `U_TRANS` flag hack for entity transparency.

**Go**: Not implemented.

**Risk**: Nehahra mod entities with transparency will render opaque.

---

## 5. Server

### RESOLVED: Entity Update Field Write Order Now Matches C Protocol

This was a real parity risk during earlier porting work, but the current server writer is now aligned with C again.

**Current Go** (`sv_send.go:697-750`): The writer documents and emits the same field order as C:
```
MODEL, FRAME, COLORMAP, SKIN, EFFECTS,
ORIGIN1, ANGLE1, ORIGIN2, ANGLE2, ORIGIN3, ANGLE3,
ALPHA, SCALE, FRAME2, MODEL2, LERPFINISH
```

Origins and angles are interleaved, FitzQuake extension payload bytes come after the base origin/angle fields, and `U_LERPFINISH` is emitted when present. The same writer path is also used for static/spawn baseline records, while signon buffer construction writes `svc_spawnbaseline` / `svc_spawnbaseline2` records during baseline setup.

**Regression coverage** (`sv_send_test.go`):
- `TestWriteEntityUpdate_FieldOrderMatchesCProtocol`
- `TestWriteEntityUpdate_OriginsAnglesInterleaved`
- `TestWriteEntityUpdate_Frame2Model2AfterAlphaScale`

---

### RESOLVED: Entity Update Extension Bits Are Protocol-Gated

This warning is also stale.

**Current Go** (`sv_send.go:647-674`): FitzQuake/RMQ-only bits are guarded by `if s.Protocol != ProtocolNetQuake`, matching the C rule that `U_ALPHA`, `U_SCALE`, `U_FRAME2`, `U_MODEL2`, `U_LERPFINISH`, `U_EXTEND1`, and `U_EXTEND2` are not emitted on NetQuake protocol streams.

**Regression coverage** (`sv_send_test.go`):
- `TestWriteEntityUpdate_NetQuakeOmitsFitzExtensions`
- `TestWriteEntityUpdate_NonNetQuakeSetsFitzExtensions`

---

### RESOLVED: Entity Update Supports U_LERPFINISH

`encodeLerpFinish` and the entity update writer now emit `U_LERPFINISH` when the current think interval should be sent to the client, and `TestEncodeLerpFinish` covers the byte encoding behavior directly.

---

### RESOLVED: WriteClientDataToMessage Includes FitzQuake/RMQ Extensions

This warning is stale.

**Current Go** (`internal/server/sv_send.go:317-352`): the clientdata serializer sets `SU_WEAPON2`, `SU_ARMOR2`, `SU_AMMO2`, the per-ammo high-byte extensions, `SU_WEAPONFRAME2`, `SU_WEAPONALPHA`, plus `SU_EXTEND1` / `SU_EXTEND2` for non-NetQuake protocols.

---

### RESOLVED: Frag Updates Track the Changed Player Correctly

This warning is stale.

**Current Go** (`internal/server/sv_client.go:584-603`): `UpdateToReliableMessages` iterates over each active player, compares that player's `OldFrags` against that same player's current frags, broadcasts `svc_updatefrags` for changed players, then updates the changed player's `OldFrags`. `internal/server/sv_client_test.go:223-271` covers the intended behavior.

---

### MEDIUM: Missing Alkaline Active Weapon Compatibility Hack

**C** (`sv_main.c:1165-1173`): After writing the active weapon byte, checks if the full weapon value doesn't fit in a byte (`(byte)ent->v.weapon != (int)ent->v.weapon`). If so, sends an `svc_updatestat` message with `STAT_ACTIVEWEAPON` containing the full 32-bit value.

**Go** (`sv_send.go:325-332`): Only sends the bit-index byte. No `svc_updatestat` fallback.

**Risk**: Alkaline 1.1 mod uses bit flags differently for weapon storage. Without this hack, Alkaline's HUD shows the wrong active weapon. Affects one specific (but popular) mod.

---

### HIGH: No Autosave

**C** (`host.c`): `Host_CheckAutosave()` periodically saves the game state.

**Go**: No autosave implementation.

**Risk**: Players lose progress on crash with no recovery.

---

### RESOLVED: Entity Delta Encoding Uses Baselines, Origin Tolerance, and U_STEP

This warning is stale.

**Current Go** (`internal/server/sv_send.go:612-645`): entity update bits are computed against the entity baseline, origin deltas use the same `> 0.1` threshold style as C, and `U_STEP` is set for `MoveTypeStep`.

**Regression coverage** (`internal/server/sv_send_test.go`):
- `TestWriteEntityUpdate_OriginTolerance`
- `TestWriteEntityUpdate_SetsUStepForStepMoveType`
- `TestWriteEntitiesToClient_UsesBaselineNotPreviousState`

---

### MEDIUM: Epsilon Value Differences

**C**: `DIST_EPSILON = 0.03125` for collision distance.

**Go**: `OneEpsilon = 0.01` (used in some comparisons).

**Risk**: Subtle differences in when entities are considered "touching" or "on ground". May cause entities to float or clip differently in edge cases.

---

### MEDIUM: No CSQC Support

**C**: Loads client-side QuakeC via `CL_LoadCSProgs()`, supports CSQC draw functions, stat queries, and client commands.

**Go**: No CSQC infrastructure at all.

**Risk**: Mods using CSQC (modern Quake modding standard) will not function.

---

### MEDIUM: Error Recovery Model Different

**C**: Uses `setjmp`/`longjmp` for error recovery — `Host_Error()` jumps back to frame top, engine continues.

**Go**: Returns errors explicitly. If error handling is incomplete, errors may propagate to unintended locations or be silently dropped.

**Risk**: C can recover from many runtime errors (bad QC, network issues) and continue running. Go may crash or enter undefined state if error returns aren't checked at every level.

---

### RESOLVED: Entity Alpha/Scale Are Read from QC Edict Fields

This warning is stale.

**Current Go**:
- `internal/server/sv_main.go:286-292` caches QC field offsets for `alpha` / `scale`
- `internal/server/sv_send.go:486-499` reads those QC fields every frame and encodes `ent.Alpha` / `ent.Scale` from the VM values before serializing entity state

---

### RESOLVED: ENTALPHA_ZERO Entities Are Culled Unless Effects Remain Visible

This warning is stale.

**Current Go** (`internal/server/sv_send.go:766-774`): `writeEntitiesToClient` skips entities when `state.Alpha == inet.ENTALPHA_ZERO && state.Effects == 0`, matching the intended invisible-entity culling behavior.

---

### RESOLVED: Effects Are Filtered Through the Supported Effects Mask

This warning is stale.

**Current Go** (`internal/server/sv_main.go:286-292`, `internal/server/sv_send.go:501-509`): the server caches `EffectsMask` from QC capability detection and applies `int(ent.Vars.Effects) & s.effectsMask()` before emitting entity state.

---

### RESOLVED: StartSound Applies Protocol Version Checks

This warning is stale.

**Current Go** (`internal/server/sv_send_sound_protocol_test.go`): focused regressions verify that NetQuake drops large entity/sound encodings while FitzQuake keeps the extended forms where legal.

---

### RESOLVED: LocalSound Applies Protocol Version Checks

This warning is stale.

**Current Go** (`internal/server/sv_send_sound_protocol_test.go`): `TestLocalSoundNetQuakeDropsLargeSoundIndex` and `TestLocalSoundFitzQuakeUsesLargeSoundEncoding` cover the protocol split directly.

---

### MEDIUM: StartSound Channel Validation Too Permissive

**C** (`sv_main.c:272-273`): Channel must be 0-7 — `Host_Error` if outside range.

**Go** (`sv_send.go:83-85`): Channel must be 0-255 — silently returns if outside.

**Risk**: In C, channel >7 is a fatal error (indicates a bug). Go silently accepts channels 8-255, which would be packed into the entity/channel short incorrectly when `SND_LARGEENTITY` is not set (the encoding uses 3 bits for channel: `ent<<3 | channel`). Channel values 8-255 would overflow the 3-bit field and corrupt the entity number.

---

### LOW: No Dev Stats Tracking

**C** (`host.c`): Tracks active edict count, warns if >600 (approaching limits).

**Go**: No equivalent tracking.

**Risk**: No warning when approaching entity limits.

---

## 6. Audio

### MEDIUM: Tracker Music Pre-Rendered vs Streamed

**C** (`snd_modplug.c`, `snd_mikmod.c`, `snd_xmp.c`): Streams MOD/S3M/XM/IT music from codec libraries in real-time.

**Go** (`music_tracker.go`): Pre-renders the entire module to PCM before playback.

**Risk**: Long tracker modules will consume significant memory. A 10-minute module at 44.1kHz stereo 16-bit = ~100MB RAM. May cause startup delay or memory pressure.

---

### MEDIUM: No Runtime Audio Cvars

**C**: `sfxvolume`, `bgmvolume`, `sndspeed`, `snd_mixspeed`, `snd_filterquality`, `snd_waterfx` all controllable via console.

**Go**: Configuration through function parameters only. No console integration.

**Risk**: Players cannot adjust audio settings at runtime. Volume changes require code-level intervention.

---

### LOW: Linear Interpolation Added (Quality Improvement)

**Go** (`mix.go`): Adds linear interpolation between audio samples during mixing.

**C** (`snd_mix.c`): Uses nearest-neighbor sampling.

**Risk**: Audio quality is **improved** in Go, but this means audio output is not bit-identical to C. For demo playback comparison or regression testing, audio will differ.

---

### LOW: Different MP3 Library

**C**: Uses `mpg123` (C library via dynamic linking).

**Go**: Uses `hajimehoshi/go-mp3` (pure Go).

**Risk**: Different MP3 decoders may produce slightly different PCM output for the same file. Unlikely to be audible.

---

## 7. Renderer

### HIGH: GoGPU Renderer Still Lacks World PVS Culling (OpenGL Path Already Has It)

This warning was previously too broad.

**Current state**:
- **OpenGL / CGO parity path** (`internal/renderer/world_render_opengl.go:736-780`) already computes the camera leaf, loads leaf PVS, and filters world faces by visible leaves before drawing.
- **GoGPU / experimental path** (`internal/renderer/world.go:2829+`) still iterates world faces without equivalent PVS filtering.

**Risk**: The missing renderer-side PVS culling is now primarily a GoGPU performance/parity problem, not a blanket statement about the canonical OpenGL renderer.

---

### MEDIUM: Dynamic Lighting Still Differs in Implementation Detail, Not Existence

This warning was previously too strong.

**Current Go** already has:
- animated lightstyle evaluation (`internal/client/client.go:699-814`)
- transient effect/entity light spawning (`internal/renderer/client_effects.go:159-242`)
- dynamic light contribution evaluation in the renderer (`internal/renderer/dynamic_light_pool.go:88-113`)

The remaining parity gap is narrower: C still has a richer per-surface/lightcache-style implementation and some lighting polish details that Go does not fully mirror yet. Dynamic lights and flickering lightstyles are no longer absent outright.

---

### HIGH: No Procedural Sky Rendering

**C** (`gl_sky.c`): Renders the classic Quake sky from `sky.lmp` texture in two animated layers, with stencil-based optimization.

**Go** (`skybox_external.go`): Only supports external skybox cubemaps. No sky.lmp support.

**Risk**: Standard Quake maps that use the built-in sky texture will render with no sky (or a solid color fallback).

---

### HIGH: No OIT (Order-Independent Transparency)

**C** (`gl_rmain.c`): Multi-pass order-independent transparency with accumulation buffers.

**Go**: Single-pass alpha blending only.

**Risk**: Overlapping transparent surfaces (water, glass, particles) will render incorrectly — back faces may overdraw front faces.

---

### HIGH: Particle System Incomplete

**C** (`r_part.c`): 8 particle types (GRAV, SLOWGRAV, FIRE, EXPLODE, EXPLODE2, BLOB, BLOB2, STATIC) with per-type physics, color ramps, and sorted rendering.

**Go** (`particle.go`): Basic particle physics only. No type-specific behavior, no distance sorting.

**Risk**: Explosions, blood, teleport effects, and other particle effects will look wrong or be invisible.

---

### HIGH: No Lightmap Dynamic Updates

**C** (`r_brush.c`): Lightmaps updated per-frame for flickering/animated light styles.

**Go**: Static lightmaps only.

**Risk**: Light styles (flickering torches, pulsing lights) will not animate. Maps will appear frozen in their initial lighting state.

---

### MEDIUM: No Water Caustics

**C**: Framebuffer blit animation for underwater caustic effects.

**Go**: Not implemented.

**Risk**: Water surfaces lack animated caustic patterns.

---

### MEDIUM: No Fog Transitions

**C** (`gl_fog.c`): Smooth fog density/color interpolation over specified time.

**Go**: Static fog only. No fade/interpolation.

**Risk**: Server-driven fog changes (via SVC_FOG) will snap instantly instead of fading smoothly.

---

### MEDIUM: No Canvas Transform Stack

**C** (`gl_draw.c`): 12+ canvas types (MENU, CONSOLE, SBAR, CROSSHAIR, etc.) with transformation stacking, blend mode control, and scissor clipping.

**Go**: 2 canvas modes (screen, menu). No blend modes beyond alpha. No clipping.

**Risk**: HUD elements, console overlay, and menu graphics may be positioned or scaled incorrectly. Status bar overlay clipping missing.

---

### MEDIUM: No Scrap Atlas

**C** (`gl_draw.c`): Small textures packed into 4 scrap atlas blocks to minimize GPU state changes.

**Go**: Each texture uploaded individually.

**Risk**: Performance degradation from excessive texture bind calls, especially with many HUD elements.

---

### MEDIUM: Only Camera-Facing Sprites Supported

**C** (`r_sprite.c`): 3 sprite orientation types: VP_PARALLEL, VP_FACING, VP_ORIENTED.

**Go**: Only VP_PARALLEL (camera-facing).

**Risk**: Oriented sprites (some mod effects) will render at wrong angles.

---

### LOW: No Screenshot Functionality

**C** (`gl_screen.c`): `SCR_ScreenShot_f()` saves PNG/TGA/JPG screenshots.

**Go**: Not implemented (except software renderer to image).

**Risk**: Players cannot take screenshots.

---

### LOW: No Loading Plaque, Pause Icon, FPS Counter

**C**: `SCR_DrawLoading()`, `SCR_DrawPause()`, FPS/speed meter display.

**Go**: None of these implemented.

**Risk**: No visual feedback during map loads or when paused.

---

## 8. Console, Cvars & Commands

### MEDIUM: Command Source Tracking Missing

**C** (`cmd.c`): Tracks `cmd_source_t` (src_client, src_command, src_server) for security — prevents clients from executing server-only commands.

**Go** (`cmdsys/cmd.go`): No source tracking.

**Risk**: In multiplayer, malicious clients could potentially execute server-side commands. Security boundary not enforced.

---

### MEDIUM: No Command Forwarding to Server

**C** (`cmd.c`): `Cmd_ForwardToServer()` sends unrecognized commands to the remote server.

**Go**: Not implemented.

**Risk**: Client-side commands like `say`, `name`, `color` that are forwarded to the server will not work in multiplayer.

---

### MEDIUM: Missing Cvar Built-in Commands

**C**: `toggle`, `cycle`, `inc`, `reset`, `resetall`, `resetcfg` commands for cvar manipulation.

**Go**: Not implemented.

**Risk**: Players cannot toggle cvars from console, cycle through values, or reset settings.

---

### MEDIUM: No Cvar Locking Mechanism

**C**: `Cvar_LockVar()`/`Cvar_UnlockVar()` for temporary cvar protection during gameplay.

**Go**: Not implemented.

**Risk**: Cvars that should be locked during gameplay (e.g., to prevent cheating) can be freely modified.

---

### LOW: No Comment Stripping in Command Buffer

**C** (`cmd.c`): `Cbuf_Execute()` strips `//` comments from command lines.

**Go**: No comment handling.

**Risk**: Config files with comments will fail to parse correctly.

---

### LOW: No CVAR_AUTOCVAR Support

**C**: Automatically syncs cvars with QC global variables.

**Go**: Not implemented.

**Risk**: Mods using autocvars will not have their QC globals updated when cvars change.

---

## 9. Filesystem

### LOW: No Streaming File Access

**C** (`common.c`): `COM_OpenFile()`, `COM_Read()`, `COM_CloseFile()` for streaming reads.

**Go** (`fs/fs.go`): Only `LoadFile()` which reads entire file into memory.

**Risk**: Large files (multi-MB BSP, pak files) must be fully loaded into memory. No incremental reading for streaming scenarios (demo playback, large file processing).

---

## 10. Input

### MEDIUM: Key Binding Storage Not in Input Package

**C** (`keys.c`): Key bindings stored, looked up, and managed within the input subsystem. Supports shift-layer bindings and named binding labels for UI display.

**Go** (`input/`): Input package only handles raw events. Bindings handled at higher level.

**Risk**: Key binding save/load, display in menus, and multi-layer bindings may behave differently.

---

### MEDIUM: No Gyroscope Calibration Persistence

**C** (`in_sdl.c`): Full gyroscope calibration with per-axis tuning, noise threshold, and persistence.

**Go**: Framework exists but calibration not persisted.

**Risk**: Gyroscope users will need to recalibrate every session.

---

### LOW: No Flick Stick Implementation

**C**: `joy_flick` cvar enables flick stick control scheme.

**Go**: Not implemented.

**Risk**: Controller users cannot use flick stick aiming.

---

## 11. Menu

### HIGH: Save/Load Game UI Missing

**C** (`menu.c`): Full save/load game menu pages (m_load, m_save) with slot display.

**Go**: Not implemented.

**Risk**: Players cannot save or load games through the menu. Must use console commands.

---

### HIGH: Multiplayer Setup UI Missing

**C** (`menu.c`): m_lanconfig, m_gameoptions, m_setup menu pages for multiplayer configuration.

**Go**: Not implemented.

**Risk**: Cannot configure multiplayer games through the menu.

---

### MEDIUM: Key Binding UI Missing

**C** (`menu.c`): m_keys menu page for binding keys to actions.

**Go**: Not implemented.

**Risk**: Players cannot rebind keys through the menu.

---

### MEDIUM: Mod Browser Missing

**C** (`menu.c`): m_mods, m_modinfo pages for browsing and installing mods.

**Go**: Not implemented.

**Risk**: Cannot browse or switch mods from the menu.

---

## 12. BSP & Model Loading

### MEDIUM: Sprite Model Loading Incomplete

**C** (`gl_model.c`): `Mod_LoadSpriteModel()` loads SPR format with 3 orientation types.

**Go** (`model/sprite.go`): Framework exists but loading not fully implemented.

**Risk**: Sprite-based entities (explosions, some projectiles, some decorations) will not render.

---

### LOW: Texture Animation Pointers Not Implemented

**C**: `texinfo.anim_next` links animated texture sequences.

**Go**: Not implemented.

**Risk**: Animated textures (flowing water, blinking lights, teleporter surfaces) will display only the first frame.

---

### LOW: No Image Export (PNG/TGA/JPG Writers)

**C** (`image.c`): `Image_WritePNG()`, `Image_WriteTGA()`, `Image_WriteJPG()`.

**Go**: Not implemented.

**Risk**: Cannot export screenshots or generated images.

---

## 13. Memory Management

### LOW: GC vs Manual Allocation

**C** (`zone.c`): Three-tier allocation: Hunk (large, stack-like, level-reset), Zone (small dynamic, free-list), Cache (persistent between levels).

**Go**: Standard Go garbage collector.

**Risk**: GC pauses may cause frame stutters during gameplay. C's manual allocation has predictable performance but Go's GC is non-deterministic. Level transitions in C reset the Hunk; Go relies on GC to collect unreferenced data, which may be slower.

---

## Summary: Risk Count by Severity

Note: these aggregate counts lag some of the section-by-section updates above. Treat the individual entries as the source of truth until the whole summary table is renormalized.

| Severity | Count | Key Areas |
|----------|-------|-----------|
| CRITICAL | 12 | QC OPCall off-by-one, QC OPReturn no return value copy, QC local save/restore missing, WriteCoord float32 vs 16-bit, protocol flags not sent, entity update field order wrong, entity extension bits ignore protocol, entity alpha/scale never read from QC, network socket reuse, server port bug, QC missing 56 builtins, no renderer PVS culling |
| HIGH | 26 | pitch-drift cvar plumbing gap, no demo playback, no lightning beams, no dynamic lighting, no sky, no OIT, no particles, no lightmap animation, no save/load UI, no multiplayer UI, no autosave, WriteClientData missing FitzQuake, U_LERPFINISH missing, no effects mask filtering, StartSound/LocalSound missing protocol checks, ENTALPHA_ZERO culling, baseline vs previous-state delta encoding, frag update logic bug |
| MEDIUM | 37 | anglemod quantization, LerpPoint bypasses, custom gravity, elevator fix, collision precision, SendInterval, fog transitions, sprite types, canvas stack, command security, cvar locking, StartSound channel validation, QC runaway loop protection, Alkaline weapon hack, and more |
| LOW | 19 | Water transitions, statistics, debug commands, sort order, and more |

---

## Priority Recommendations

### Must Fix for Basic Functionality (Protocol/Correctness)
1. **QC OPCall off-by-one** — First statement of every called function is skipped
2. **QC OPReturn doesn't copy return value** — All QC function return values are broken
3. **QC local save/restore missing** — Nested function calls corrupt caller state
4. **WriteCoord uses float32 instead of 16-bit fixed-point** — Every coordinate in the protocol is the wrong size; completely wire-incompatible with all C clients
5. **Protocol flags not sent in ServerInfo** — Clients cannot know coordinate/angle encoding format
6. **Entity update field write order** — Wire-protocol incompatible with any C Quake client/server
7. **Entity extension bits ignore protocol version** — NQ clients get corrupted entity streams
8. **Entity alpha/scale never read from QC** — All mod transparency/scaling is broken
9. **WriteClientDataToMessage FitzQuake extensions** — Stat values >255 broken, weapon alpha missing
10. **QC builtins `ftos`, `vtos`** — Without these, vanilla Quake HUD is broken
11. **Network socket-per-client** — Multiplayer is fundamentally broken
12. **Network server CCREP_ACCEPT port** — Sends wrong port to connecting clients
13. **anglemod() quantization** — Must match C's 16-bit quantization, not 8-bit or continuous
14. **StartSound/LocalSound protocol checks** — Sound packets corrupt NQ client data stream

### Must Fix for Playable Experience
15. **Renderer PVS culling** — Performance is unplayable without this on any real map
16. **Lightning beam rendering** — Lightning gun and Shambler attacks invisible
17. **Dynamic lighting + light styles** — Maps appear flat and frozen
18. **Effects mask filtering** — Mod conflicts with QEX effect bits
19. **Delta encoding: baseline-based comparison + U_STEP** — Entity updates differ from C protocol

### Must Fix for Gameplay Parity
23. **Pitch drift cvar plumbing parity**
24. **Demo playback**
25. **LerpPoint missing timedemo/local-server bypass**
26. **U_LERPFINISH** for smooth entity interpolation
27. **Procedural sky rendering** (sky.lmp)

### Should Fix for Mod Compatibility
28. **QC trig builtins** (sin, cos, sqrt)
29. **Custom entity gravity**
30. **CSQC infrastructure**
31. **String builtins** (strlen, strcat, etc.)
32. **cl_nolerp cvar support**
33. **ENTALPHA_ZERO entity culling** — Network bandwidth optimization
