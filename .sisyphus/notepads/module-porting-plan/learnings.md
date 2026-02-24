
## internal/testutil
- Established `internal/testutil` for shared test helpers.
- `LocatePak0` checks `QUAKE_PAK0_PATH` environment variable and common relative paths (e.g., `id1/pak0.pak`).
- `SkipIfNoPak0` allows tests to gracefully skip if assets are missing, which is essential for CI environments where the full game data might not be available.
- `CompareStructs` provides a unified way to compare complex objects in tests, with hex dump support for byte slices.
Ported foundational math and string utilities from common.c and mathlib.c to internal/common and pkg/types.
Implemented COM_Parse, COM_CheckParm, path/extension utilities, and FNV-1a hash in internal/common.
Implemented Lerp, NormalizeAngle, AngleDifference, LerpAngle, VectorAngles, AngleVectors, and other math utilities in pkg/types.

## Porting wad.c and gl_texmgr.c texture parsing
- WAD2 files use a simple header and lump table.
- QPic format is basically width, height, and indexed pixels.
- MipTex format includes 4 mip levels and is used for world textures.
- Quake palette is 768 bytes (256 RGB entries).
- Index 255 is often used for transparency in Quake UI/HUD graphics.
- AlphaEdgeFix is used to prevent color bleeding from transparent pixels when using linear filtering.
- Go's image/png is a direct replacement for lodepng.

## BSP tree loading (gl_model.c -> internal/bsp/tree.go)
- The on-disk struct sizes in `bspfile.h` are critical: `dplane_t=20`, `dsnode_t=24`, `dl2node_t=44`, `dl2leaf_t=44`, and `dmodel_t=64`.
- Parsing BSP children must preserve Quake semantics: standard BSP uses `uint16` reinterpretation (`leaf = 65535 - child`), BSP2 uses bitwise complement of negative child indices.
- Loading order matters for validation parity with C path: faces -> marksurfaces -> leafs -> nodes lets node/leaf references be validated during load.

## Sprite loading (gl_model.c -> internal/model/sprite.go)
- `dspriteframetype_t` dispatch needs strict validation parity with C (`SPR_SINGLE`, `SPR_GROUP`, `SPR_ANGLED`; angled groups require exactly 8 frames).
- Group intervals must be strictly positive during decode (`interval > 0`) to match `Mod_LoadSpriteGroup` behavior.
- A robust integration test path is `progs/*.spr` from `pak0.pak` using `internal/fs` plus `testutil.SkipIfNoPak0`.

## Alias model loading (gl_model.c -> internal/model/alias.go)
- `Mod_LoadAliasModel` parsing order matters: skins first, then `stvert_t`, then `dtriangle_t`, then per-frame payloads.
- Alias frame groups contain repeated `daliasframe_t` blocks before each pose vertex block; preserving that layout is required for correct frame-group traversal.
- Quake computes alias bounds from decoded pose vertices (`scale` + `scale_origin`) and derives yaw-rotated and fully-rotated bounds from max squared radii.

## Server/world port (sv_main.c + world.c)
- A practical headless `SpawnServer` path can be validated without full QuakeC execution by loading `maps/<name>.bsp` through `internal/fs` and parsing it with `bsp.LoadTree`.
- `SV_LinkEdict` trigger behavior needs a two-pass approach (collect then execute) to avoid list mutation issues while touch callbacks run.
- Initializing brush hulls to invalid clipnode ranges (`FirstClipNode=-1`, `LastClipNode=-1`) is a safe fallback for map-load verification before full clipnode/hull conversion is implemented.

## Server physics port (sv_phys.c -> internal/server/physics.go)
- `SV_Physics_Toss` parity requires early return on `FL_ONGROUND`, angular velocity integration, bounce overbounce (`1.5`), and ground stop behavior (`normal.z > 0.7` with low z-velocity stop).
- `SV_Physics_Pusher` should use `ltime` + partial-frame movement (`movetime`) and run think only when `nextthink` crosses the new `ltime`.
- `SV_FlyMove` style sliding needs iterative clipping across multiple planes (`MAX_CLIP_PLANES`) to avoid getting stuck or tunneling through corners.

## Server movement port (sv_move.c -> internal/server/movement.go)
- Preserving the original `SV_*` API names as thin wrappers over existing world collision helpers (`Move`, `hullForEntity`, `TestEntityPosition`) enables incremental porting without disrupting already-ported physics code.
- `SV_movestep` parity depends on reproducing both branches: flying/swimming monsters attempt a two-pass enemy-height adjustment, while walking entities do step-up/step-down tracing plus `SV_CheckBottom` validation.
- Pak-aware movement tests are most robust as smoke+invariant checks (wrapper parity, `SV_TestEntityPosition` clear at sampled walkable points, zero-delta `MoveStep`) rather than fixed-coordinate assertions.

## Server user port (sv_user.c -> internal/server/user.go)
- Keeping explicit `SV_*` entry points (`SV_ClientThink`, `SV_ReadClientMessage`, `SV_ExecuteUserCommand`) while preserving idiomatic wrappers (`ClientThink`, `ReadClientMessage`) maintains C parity without breaking current call sites.
- `SV_ReadClientMessage` should decode command bytes as signed (`ReadChar`) so `-1` end-of-message handling matches Quake packet semantics.
- Command whitelist parity is best preserved with prefix matching (`q_strncasecmp`-style), not exact token matching, to mirror original permissive behavior.
- Ported host frame loop and command system from C to Go.
- Used interfaces for subsystems (Server, Client, Console, etc.) to allow for easier testing and decoupling.
- Implemented command registration using a centralized command system (cmdsys).
- Handled frame time accumulation for renderer/server isolation, similar to the original C implementation.

## Network Porting (net_main.c, net_udp.c, net_dgrm.c)
- Ported the Quake datagram protocol to Go using the `net` package.
- Simplified the driver architecture: instead of function pointer tables, we use a `driver` field in the `Socket` struct and dispatch in `net.go`.
- Used `encoding/binary` for network byte order (Big Endian for headers, Little Endian for some control message fields).
- Implemented a basic reliability layer in `datagram.go` with ACKs and retransmissions.
- Verified with tests that both reliable and unreliable messages work over UDP.

## Client logic port (cl_parse.c, cl_main.c, cl_input.c, cl_demo.c)
- Parsing server messages is easiest to validate by reproducing Quake's message terminator semantics (`0xFF` == end-of-message) and sign-on progression (`svc_signonnum` 1..4).
- `svc_serverinfo` parity requires explicit model/sound null-terminated precache list parsing and map name derivation from model slot 1 (`maps/<name>.bsp` -> `<name>`).
- A static byte-array sign-on test sequence (serverinfo + signonnum stages) gives strong confidence in protocol parsing without requiring a live server.
- Input parity benefits from preserving `kbutton_t` impulse semantics (`state` bits for down/impulse-down/impulse-up) before movement assembly.

## Audio Porting (Spatial & Mixing)
- Spatialization logic from `snd_dma.c` was ported to `internal/audio/spatial.go`.
- Software mixing logic from `snd_mix.c` was ported to `internal/audio/mix.go`.
- The mixer uses a fixed-point 24.8 representation for intermediate samples in the paint buffer.
- Low-pass filtering (Blackman window) was implemented for resampling 11025Hz sounds to 44100Hz output.
- Underwater muffling effect was implemented using a simple one-pole low-pass filter.

## Renderer core init port (gl_rmain.c -> internal/renderer/core.go)
- Headless WebGPU initialization can be done without a surface by creating a HAL instance and enumerating adapters with `EnumerateAdapters(nil)`.
- Reusing `native.NewHalBackend` from `gogpu` keeps backend selection aligned with existing renderer behavior while avoiding cgo.
- The `R_SetFrustum` z-log depth setup maps cleanly to a small frame-data helper (`zlogscale`, `zlogbias`) using `log2(zNear/zFar)`.

## Renderer surface/model port (r_brush.c + r_alias.c)
- The lightmap `Chart_Add` allocator's serpentine horizontal stepping (`x` sweep + reverse-at-edge) is important for parity; simple left-to-right packing changes placement behavior.
- `GL_FillSurfaceLightmap` has three distinct write modes (1 style RGBA, 2 styles RGBA pairs, 3/4 styles packed RGB channels) and must preserve each layout exactly.
- Alias interpolation is easiest to keep correct by mirroring state transitions (`LERP_RESETANIM`, `LERP_RESETANIM2`, `LERP_RESETMOVE`) before computing blend values.

## Renderer particle port (r_part.c -> internal/renderer/particle.go)
- Particle simulation parity is driven by `CL_RunParticles` compaction semantics: expired/not-yet-spawned particles are skipped and survivors are packed in-place.
- Draw-pass gating from `R_DrawParticles_Real` is mode-dependent (`r_particles=1` alpha pass, `r_particles=2` opaque pass), so exposing this as a helper keeps tests deterministic.
- Rocket-trail and run-effect behavior are easiest to verify with deterministic RNG injection (`*rand.Rand`) rather than global random state.
