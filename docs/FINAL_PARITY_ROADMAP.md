# Ironwail Go - Final Parity Roadmap

This is the authoritative master document for reaching 100% feature and behavioral parity with the original C Ironwail engine.

## 1. Ground Rules & Authoritative Target

1. **The OpenGL Path is Absolute**: The **CGO/OpenGL runtime** (`renderer_opengl.go` + GLFW + SDL3/Oto) is the canonical parity target.
2. **Source Code Wins**: When documentation and source code disagree, the original C source is the final authority.
3. **gogpu is Secondary**: The WebGPU backend is experimental. Do not let its gaps block or redefine the main parity path.
4. **Test-First Development**: New features and bug fixes should be verified with regression tests against real assets where possible.

---

## 2. Immediate Priority: Physics & Movement ("Rough Edges")

While the core movement model exists, manual testing has revealed critical behavioral gaps that must be closed immediately for playability.

- [x] **Movement Friction**: Implement `SV_WallFriction`. The player should lose some speed when sliding against walls based on view angle.
- [x] **Unsticking Polish**: Refine `SV_CheckStuck` to match the exact recursive "nudge" behavior of C Quake.
- [x] **Collision Hull Accuracy**: Verify that all three Quake hulls (Point, Player, Large) are correctly selected and utilized in `SV_Move`.
- [x] **Fall Damage & Landing**: Verify QuakeC `PlayerPreThink`/`PlayerPostThink` correctly calculates `fall_velocity` and triggers landing sounds/damage.

---

## 3. Rendering Fidelity (OpenGL Polish)

The Go renderer draws the right things, but not always in the right way.

- [x] **Translucent Batching**: Implement `R_AddBModelCall` and `R_CanMergeBModelAlphaPasses`. Implement collection and sorting of translucent brush model and world faces.
- [x] **Particle Visual Polish**: Particle physics are bit-identical to C (same struct, ramp tables, movement/decay/gravity for all 8 types). Rendering uses point sprites with fwidth-based AA.
- [x] **Visual Blue-Shift**: `CalcBlend`/`UpdateBlend`/`SetContentsColor` in `viewblend.go` match C exactly. `polyblend_opengl.go` renders the full-screen overlay on the OpenGL path.
- [x] **Stencil/Decal Fidelity**: `decal_opengl.go` uses `GL_STENCIL_TEST` (EQUAL/INCR) + `PolygonOffset(-1.0, -2.0)` to prevent z-fighting and double-blending.
- [x] **Sky Edge Cases**: All three fallback paths implemented in `skybox_external.go`: cubemap → six separate face textures → embedded BSP sky, with proper mode selection.

---

## 4. Audio & Music (Codec & Spatialization)

- [x] **Precise Spatialization**: `spatialize()` in `spatial.go` matches C's `SND_Spatialize` exactly (distance attenuation + stereo panning via dot product with listener right vector).
- [x] **Low-Pass Filtering**: `S_ApplyFilter` implemented as `Mixer.lowpassFilter` in `mix.go`.
- [x] **Full Codec Parity**: OGG, Opus, MP3, FLAC, WAV decoders in `internal/audio/`. Tracker music formats (MOD, S3M, XM, IT) added via gotracker/playback library. Extension priority matches C (OGG → Opus → MP3 → FLAC → WAV → MOD → S3M → XM → IT). UMX container format not yet supported (nice-to-have).
- [x] **Pitch Shifting**: Doppler effect implemented in `spatial.go` with per-channel pitch multiplier applied in `mix.go`. Clamped to [0.5, 2.0] range. Go is ahead of C here (C has no Doppler).

---

## 5. Networking & Multiplayer Depth

Remote play is functional but shallow compared to the original engine.

- [x] **Server Visibility (PVS)**: Implement `SV_AddToFatPVS` and `SV_VisibleToClient` to correctly cull network updates for far-away entities.
- [ ] **Network Protocol Fidelity**: Replicate `Datagram_CanSendUnreliableMessage` and ensure large signon buffers (for maps with 1000s of entities) are handled without overflow.
- [x] **Server Browser (Slist)**: Match C Ironwail poll timing in LAN/Local server discovery (`Slist_Send` retry at 0.75s and `Slist_Poll` stop at 1.5s).
- [x] **Listen Startup Failure Handling**: `internal/net.Listen(true)` now surfaces UDP bind/open errors instead of silently failing, and host LAN-advertising setup keeps server info disabled when listen socket startup fails.
- [ ] **Deathmatch Polish**: Fully implement all `fraglimit`/`timelimit` edge cases and the richer multiplayer setup UI.

---

## 6. Console Commands & UX

Over 40 standard console commands are still missing or use non-standard naming.

### Missing Game Flow & Admin Commands
- [x] `kick` (by name, and by `# slot`), `ban`, `status` (map name, active players, ping table), `ping`.
- [x] `changelevel`, `restart`, `randmap`.
- [x] `maps`: Implemented.
- [x] `say`, `say_team`, `tell`, `messagemode`, `messagemode2`.

### Missing Debug & Cheat Commands
- [x] `fly`, `god`, `noclip`, `notarget`, `give`.
- [x] `viewpos`, `tracepos`, `pr_ents` (print entities).
- [x] `viewframe`, `viewnext`, `viewprev`.
- [x] Server debug telemetry controls: `sv_debug_telemetry*` and
  `sv_debug_qc_trace*` now expose filtered engine-side event logging and
  QuakeC call-chain tracing for parity/debug investigations.

### Missing UI & Media Commands
- [x] `demos`, `startdemos`, `stopdemo`.
- [x] `soundinfo`, `particle_texture`.

---

## 7. Menus, HUD & Add-ons

- [x] **Expansion Pack HUDs**: Hipnotic and Rogue weapon/item/inventory overlays implemented in `internal/hud/status.go`. Activated via `ModHipnotic`/`ModRogue` flags set from `gameModDir`.
- [x] **Add-on Browser**: `MenuMods` implemented in `internal/menu/manager.go` with mod list, current-mod highlighting, and back navigation.
- [x] **Player Preview**: Setup menu uses `gfx/menuplyr.lmp` with top/bottom color translation, matching C engine behavior.
- [x] **Menu Mouse Polish**: `M_Mousemove` now covers all menu screens including Setup, JoinGame, and HostGame.

---

## 8. Save/Load & Demos (Long-tail)

- [x] **Broader Save Search**: Replicate the C engine's behavior of searching for saves outside the active game directory (e.g., falling back to `id1/` or install root).
- [x] **Demo Metadata Accuracy**: Ensure all metadata and edge-case recording scenarios match `cl_demo.c`.
- [x] **Timedemo/Rewind**: Complete the advanced demo playback tooling for benchmarking and scrubbing.

---

## 9. Appendix: C-to-Go Function Mapping Audit

The following C functions from Ironwail have been identified as missing or requiring significant refactoring in Go to achieve 100% parity.

### Server Physics (`sv_phys.c`)
- [x] `SV_CheckAllEnts`
- [x] `SV_Physics_Client` (Partially refactored into `PhysicsWalk`)
- [x] `SV_TryUnstick`
- [x] `SV_WallFriction`

### Server Main & Messaging (`sv_main.c`)
- [ ] `SV_AddSignonBuffer`
- [x] `SV_AddToFatPVS`
- [ ] `SV_CheckForNewClients`
- [ ] `SV_EdictInPVS`
- [x] `SV_VisibleToClient`
- [x] `SV_WriteStats`

### Client Main & Parsing (`cl_main.c`, `cl_parse.c`)
- [x] `CL_DecayLights` → `glLightPool.UpdateAndFilter` + `SpawnOrReplaceKeyed` for per-entity slot reuse
- [x] `CL_Disconnect_f` → `Host.CmdDisconnect`
- [x] `CL_EstablishConnection` → `Host.CmdConnect`
- [x] `CL_RelinkEntities` → `Client.RelinkEntities` in `internal/client/relink.go`
- [x] `CL_ParseBaseline` → `Parser.parseSpawnBaseline`
- [x] `CL_ParseLocalSound` → `Parser.parseSound(local=true)`
- [x] `CL_ParseStartSoundPacket` → `Parser.parseSound(local=false)`
- [x] `CL_ParseStaticSound` → `Parser.parseSpawnStaticSound`

### Renderer (`r_world.c`, `r_alias.c`, `r_part.c`)
- [x] `R_AddBModelCall` → brush entity draw loop in `world_runtime_opengl.go`
- [x] `R_FlushBModelCalls` → brush entity draw loop (no indirect multi-draw, intentional)
- [x] `R_CanMergeBModelAlphaPasses` → face bucketing/sorting in `world_opengl.go`
- [x] `R_Alias_CanAddToBatch` → `AliasBatch.CanAdd` in `model.go`
- [x] `R_ClearParticles` → `ParticleSystem.Clear` in `particle.go`
- [x] `R_FlushParticleBatch` → batched upload loop (512-vertex chunks) in `particle_runtime_opengl.go`

### Audio Mixer (`snd_mix.c`, `snd_dma.c`)
- [x] `SND_PaintChannelFrom16` / `From8` → `paintChannel16`/`paintChannel8` in `mix.go` (with linear interpolation, higher quality than C)
- [x] `S_ApplyFilter` (Low-pass) → `Mixer.lowpassFilter` in `mix.go`
- [x] `S_UnderwaterIntensityForContents` → `runtimeUnderwaterIntensity` in `main.go`
