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
- [ ] **Particle Visual Polish**: Match the exact visual appearance, decay, and movement of C Quake particles (blood, sparks, explosions).
- [ ] **Visual Blue-Shift**: Implement the full `v_blend` blue-shift for underwater vision (audio intensity is already wired).
- [ ] **Stencil/Decal Fidelity**: Ensure projected marks (bullet holes) use the correct stencil/depth-bias logic to avoid flickering.
- [ ] **Sky Edge Cases**: Validate additional non-cubemap content packs and ensure embedded BSP fallback is frame-perfect.

---

## 4. Audio & Music (Codec & Spatialization)

- [ ] **Precise Spatialization**: Match the exact distance/panning curves from C's `SND_Spatialize`.
- [ ] **Low-Pass Filtering**: Implement `S_ApplyFilter` to muffle sounds when the listener is underwater.
- [ ] **Full Codec Parity**: Port remaining `bgmusic.c` codecs to support OGG/FLAC/Opus beyond just CD tracks.
- [ ] **Pitch Shifting**: Implement entity velocity-based pitch shifting (Doppler effect simulation).

---

## 5. Networking & Multiplayer Depth

Remote play is functional but shallow compared to the original engine.

- [x] **Server Visibility (PVS)**: Implement `SV_AddToFatPVS` and `SV_VisibleToClient` to correctly cull network updates for far-away entities.
- [ ] **Network Protocol Fidelity**: Replicate `Datagram_CanSendUnreliableMessage` and ensure large signon buffers (for maps with 1000s of entities) are handled without overflow.
- [ ] **Server Browser (Slist)**: Implement LAN/Local network server broadcasting and discovery (`Slist_Poll`, `Slist_Send`).
- [ ] **Deathmatch Polish**: Fully implement all `fraglimit`/`timelimit` edge cases and the richer multiplayer setup UI.

---

## 6. Console Commands & UX

Over 40 standard console commands are still missing or use non-standard naming.

### Missing Game Flow & Admin Commands
- [ ] `kick` (by slot/name), `ban`, `status` (with full network stats), `ping`.
- [ ] `changelevel`, `restart`, `maps`, `randmap`.
- [x] `maps`: Implemented.
- [x] `say`, `say_team`, `tell`.

### Missing Debug & Cheat Commands
- [x] `fly`, `god`, `noclip`, `notarget`, `give`.
- [x] `viewpos`, `tracepos`, `pr_ents` (print entities).
- [ ] `viewframe`, `viewnext`, `viewprev`.

### Missing UI & Media Commands
- [ ] `demos`, `startdemos`, `stopdemo`.
- [ ] `soundinfo`, `particle_texture`.

---

## 7. Menus, HUD & Add-ons

- [ ] **Expansion Pack HUDs**: Add special-case overlays for Mission Pack 1 (Hipnotic) and Mission Pack 2 (Rogue).
- [ ] **Add-on Browser**: Implement the Ironwail-specific **Mods Menu** for browsing and launching installed add-ons.
- [ ] **Player Preview**: Replace the simplified color-swatch preview in the Setup menu with the C engine's translated player model art.
- [ ] **Menu Mouse Polish**: Complete the `M_Mousemove` implementation for full menu mouse navigation.

---

## 8. Save/Load & Demos (Long-tail)

- [ ] **Broader Save Search**: Replicate the C engine's behavior of searching for saves outside the active game directory (e.g., falling back to `id1/` or install root).
- [ ] **Demo Metadata Accuracy**: Ensure all metadata and edge-case recording scenarios match `cl_demo.c`.
- [ ] **Timedemo/Rewind**: Complete the advanced demo playback tooling for benchmarking and scrubbing.

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
- [ ] `CL_DecayLights`
- [ ] `CL_Disconnect_f`
- [ ] `CL_EstablishConnection`
- [ ] `CL_RelinkEntities`
- [ ] `CL_ParseBaseline`
- [ ] `CL_ParseLocalSound`
- [ ] `CL_ParseStartSoundPacket`
- [ ] `CL_ParseStaticSound`

### Renderer (`r_world.c`, `r_alias.c`, `r_part.c`)
- [ ] `R_AddBModelCall`
- [ ] `R_FlushBModelCalls`
- [ ] `R_CanMergeBModelAlphaPasses`
- [ ] `R_Alias_CanAddToBatch`
- [ ] `R_ClearParticles`
- [ ] `R_FlushParticleBatch`

### Audio Mixer (`snd_mix.c`, `snd_dma.c`)
- [ ] `SND_PaintChannelFrom16` / `From8` (Go uses internal mixer, verify fidelity)
- [ ] `S_ApplyFilter` (Low-pass)
- [ ] `S_UnderwaterIntensityForContents`
