# IRONWAIL-GO PARITY ANALYSIS: EXECUTIVE SUMMARY

## Overview
The Go port of Ironwail is **~40% feature-complete** relative to the C original across all 7 functional areas. The largest gaps are in real-time rendering (world + entity drawing) and integration between subsystems, not algorithm implementation.

## Quick Status by Category

| Category | Status | Severity |
|----------|--------|----------|
| A. World Rendering | **40%** | **CRITICAL** - Lightmaps missing, sky stub, water flat |
| B. Entity/Model Rendering | **5%** | **CRITICAL** - No entity pipeline, alias/sprite/particle invisible |
| C. Menus/HUD/Console/Input | **30%** | **CRITICAL** - 6 submenus TODO, no key bindings, limited console |
| D. Audio/Music | **50%** | **CRITICAL** - System built but never receives events, no music |
| E. Networking/Multiplayer | **60%** | **HIGH** - Connect/kick TODO, no DM mode, limited server UI |
| F. Client Prediction/Demo | **30%** | **HIGH** - Simplified physics, no command looping, no recording |
| G. Save/Load | **80%** | **MEDIUM** - Works but QuakeC VM state not preserved |

---

## TIER 1: PLAYABILITY BLOCKERS (Implement First)

These prevent the game from being playable at all.

### 1. Entity Rendering Pipeline (Priority: 1st)
**Files:** `renderer_opengl.go:Frame()`, `renderer_gogpu.go:OnDraw()`, new file `entity_renderer.go`
**Gap:** No call to render alias models, sprites, particles, or brush submodels
**C Reference:** `gl_rmain.c:R_RenderScene()` → `R_DrawBrushModels()`, `R_DrawAliasModels()`, `R_DrawSpriteModels()`, `R_DrawParticles()`
**Scope:**
- Add method: `Renderer.RenderEntities(entities []RenderEntity, viewState CameraState)`
- Iterate entity list by type (BrushEntity, AliasModelEntity, SpriteEntity)
- Dispatch to type-specific renderers
- Call particle render after translucent world pass

**Tasks:**
1. Define `RenderEntity` interface or union type
2. Implement `renderBrushEntities()` with submodel geometry batching
3. Implement `renderAliasEntities()` using `alias_opengl.go:buildAliasVerticesInterpolated()`
4. Implement `renderSpriteEntities()` with billboard quad generation
5. Wire calls into `Frame()` render loop at correct passes
6. Add unit tests with mock geometry

---

### 2. Key Binding System (Priority: 2nd)
**Files:** New files `input/keybutton.go`, modify `input/types.go`, `client/input.go`
**Gap:** No command binding; players cannot move/attack/jump
**C Reference:** `keys.c:Key_Event()`, `cl_input.c:KeyDown()`/`KeyUp()` with state machine
**Scope:**
- Implement `KButton` type with state bits: `state` (current), `down[2]` (two simultaneously pressed keys)
- Define 12 standard buttons: `in_mlook`, `in_klook`, `in_forward`, `in_back`, `in_left`, `in_right`, `in_lookup`, `in_lookdown`, `in_strafe`, `in_speed`, `in_jump`, `in_attack`, `in_use`
- Implement edge trigger logic: bit 0=current, bit 1=edge-down, bit 2=edge-up
- Hook to `Build()` user command in client loop

**Tasks:**
1. Create `keybutton.go` with `KButton` struct and `KeyDown()/KeyUp()/State()` methods
2. Create default keybindings (WASD, arrows, space, ctrl, shift)
3. Wire `input.Key()` events to button state updates
4. Integrate buttons into `CL_BuildCmd()` / `UserCmd` generation
5. Add console command binding system: `bind KEY "command"`, `unbind KEY`
6. Test with movement in demo level

---

### 3. Sound Event Dispatch (Priority: 3rd)
**Files:** `client/parse.go`, modify `audio/system.go`
**Gap:** Parser recognizes `svc_sound` but never calls audio system
**C Reference:** `cl_parse.c:CL_ParseStartSoundPacket()`, `snd_dma.c:S_StartSound()`
**Scope:**
- Implement `parseStartSound()` to fully decode: entity num, channel, sound index, volume, attenuation, position
- Call `AudioSystem.PlaySound(sfxIndex, origin, volume, attenuation, channel)`
- Implement `parseStopSound()` to call `AudioSystem.StopSound(entity, channel)`
- Add fallback for missing sounds (console warning, not crash)

**Tasks:**
1. Finish `client/parse.go:parseStartSound()` full implementation
2. Add `audio.System:PlaySound()` method
3. Hook parse results to audio system calls
4. Implement sound cache loader integration
5. Test with weapon fire, explosions, footsteps
6. Verify attenuation formula matches C

---

### 4. Alias Model (Character) Rendering (Priority: 4th)
**Files:** `alias_opengl.go`, modify `renderer_opengl.go`
**Gap:** Model vertices built but never uploaded or drawn
**C Reference:** `r_alias.c:R_DrawAliasModel()`, `gl_rmain.c:R_DrawAliasModels()`
**Scope:**
- Use existing `buildAliasVerticesInterpolated()` from `alias_opengl.go`
- Create VAO/VBO for alias vertices
- Create shader with lighting support (diffuse + dynamic lights)
- Batch alias models by model ID for efficient drawing
- Apply skin texture from cache
- Alpha blending for transparency (`entity.alpha`)

**Tasks:**
1. Create `glAliasRenderer` with VAO/VBO and shader
2. Implement `RenderAliasModel(entity, camera)` using pose interpolation
3. Apply entity rotation/translation/scale before GPU
4. Batch by model + shader + texture
5. Test with ogre, knight, ranger models
6. Verify pose interpolation matches player animation smoothly

---

### 5. Lightmap Processing (Priority: 5th)
**Files:** `renderer/world.go`, new file `renderer/lightmap.go`, `world_runtime_opengl.go`
**Gap:** `LightmapIndex: -1` hardcoded; all surfaces black
**C Reference:** `r_brush.c:Chart_Init()`, `r_brush.c:R_BuildLightmap()`, `gl_texmgr.c` lightmap atlas
**Scope:**
- Parse lightmap data from BSP (stored after face data)
- Allocate lightmap atlas (e.g., 2048x2048 tiles)
- Compute surface texture coordinates from `texinfo` + face bounds
- Quantize coordinates to lightmap atlas space
- Upload as GPU texture with proper filtering
- Bind in fragment shader for lit surfaces

**Tasks:**
1. Create `lightmap.go` with `LightmapAtlas` struct
2. Implement lightmap unpacking from BSP binary
3. Implement `Chart` allocation algorithm (matching C)
4. Compute lightmap UVs for all faces
5. Create lightmap texture and bind to sampler
6. Update fragment shader to sample and modulate diffuse
7. Test with dark areas, torches
8. Verify brightness curve matches original

---

## TIER 2: MAJOR FEATURES (Implement Second)

These enable advanced gameplay but don't block basic play.

### 6. Connect/Reconnect Commands (E.1)
**Files:** `host/commands.go:CmdConnect()`, `host/commands.go:CmdReconnect()`
**C Reference:** `host_cmd.c:Host_Connect_f()`
**Scope:** Establish TCP/UDP connection to remote server, complete signon handshake

### 7. Menu System Submenus (C.1)
**Files:** `menu/manager.go`, new files `menu/network.go`, `menu/video.go`, etc.
**C Reference:** `menu.c` with M_Menu_Net_f, M_Menu_LanConfig_f, M_Menu_GameOptions_f, etc.
**Scope:** Implement 6 stub submenus with proper drawing and input handling

### 8. Particle Effects Integration (B.4)
**Files:** `renderer/renderer_opengl.go`, existing particle system
**C Reference:** `r_part.c:R_DrawParticles()`, `R_AllocParticle()`
**Scope:** Call particle renderer at correct passes (opaque before translucent world, transparent after)

### 9. Brush Submodel Rendering (B.6)
**Files:** `renderer_opengl.go`, `entity_renderer.go`
**C Reference:** `r_brush.c` inline BSP models
**Scope:** Apply entity transform to brush model geometry before rendering

### 10. Movement Prediction Physics (F.1)
**Files:** `client/prediction.go`
**C Reference:** QuakeC `pm_* functions` (ground detection, step, collision)
**Scope:** Add ground detection, step handling, proper air control, velocity clamping

---

## TIER 3: POLISH (Implement Last)

These enhance experience but aren't required for playability.

### 11. Music System (D.3)
**Files:** New files `audio/music.go`, `audio/codec_*.go`
**C Reference:** `bgmusic.c` with OGG Vorbis support
**Scope:** Implement music playback with separate volume control

### 12. Sprite Rendering (B.3)
**Files:** `sprite_opengl.go`, `entity_renderer.go`
**C Reference:** `r_sprite.c:R_DrawSpriteModels()`
**Scope:** Generate billboard quads, apply rotation, depth sort

### 13. Water Warp Effects (A.2)
**Files:** `world_runtime_opengl.go`, shader update
**C Reference:** `gl_warp.c` with `R_TextureAnimation()` vertex displacement
**Scope:** Add time-based sine wave displacement to water surfaces

### 14. Gun Model Rendering (B.8)
**Files:** `entity_renderer.go`, new shader for gun
**C Reference:** `cl_main.c` view entity rendering
**Scope:** Render player weapon in eye space with special FOV scaling

### 15. Demo Recording / Playback (F.10)
**Files:** `internal/client/demo.go`, `internal/host/commands.go`, `cmd/ironwailgo/main.go`
**C Reference:** `cl_demo.c`
**Status:** Forward-path parity is wired: connected-state recording emits the initial snapshot, live play writes frames, `stop` writes the disconnect trailer, and playback now respects pause and recorded server-time pacing.

---

## README / BUILD BASELINE

The repo README should now describe the current renderer/build reality directly:

- OpenGL/CGO is the default gameplay renderer and parity target.
- gogpu/WebGPU remains a secondary backend while its runtime bugs are addressed.
- CGo is no longer a forbidden dependency in practice because the canonical OpenGL path requires it.
- `mise run build-cgo` and `mise run smoke-cgo-map-start` are the primary build/smoke checks for renderer-facing parity work.

---

## FILES TO REFERENCE FOR DEEP IMPLEMENTATION

### Critical C Functions

| Function | File | Purpose |
|----------|------|---------|
| `R_RenderScene()` | `gl_rmain.c` | Main render loop orchestration |
| `R_DrawAliasModels()` | `r_alias.c` | Model rendering dispatcher |
| `R_DrawBrushModels()` | `r_brush.c` | Brush entity rendering |
| `R_DrawSpriteModels()` | `r_sprite.c` | Sprite rendering |
| `R_DrawParticles()` | `r_part.c` | Particle system render |
| `Key_Event()` | `keys.c` | Input → button state |
| `CL_BuildCmd()` | `cl_input.c` | User command assembly |
| `CL_ParseStartSoundPacket()` | `cl_parse.c` | Sound event parsing |
| `S_StartSound()` | `snd_dma.c` | Audio system dispatch |
| `Host_Connect_f()` | `host_cmd.c` | Server connection |
| `Chart_Init()` | `r_brush.c` | Lightmap atlas allocation |
| `R_TextureAnimation()` | `r_brush.c` | Texture frame animation |

### Go Files to Modify

| File | Change |
|------|--------|
| `renderer/renderer_opengl.go` | Add `RenderEntities()` call in `Frame()` |
| `client/parse.go` | Wire `parseStartSound()` to audio system |
| `input/types.go` | Add `KButton` type and bindings |
| `menu/manager.go` | Implement 6 submenu renderers |
| `renderer/world.go` | Implement lightmap processing |
| `client/input.go` | Integrate key buttons into `BuildCmd()` |

---

## INTEGRATION TEST MATRIX

To verify parity, test these scenarios in order:

1. **Start game** → Blank dark screen (stub) or Q1DM1 geometry visible (lightmap ready)
2. **Listen server, join as client** → Player spawn + gravity
3. **Press movement keys** → View angle changes, player moves forward
4. **Fire weapon** → Weapon sound plays, impact visible (once particle system wired)
5. **See monster** → Ogre visible and animating
6. **Monster shot** → Damage sound, particle effect
7. **Collect health** → HUD updates, pickup sound
8. **Change level** → New map loads with persistent player state
9. **Record demo** → Demo file created and plays back
10. **Multiplayer** → Connect to server, see other players, chat

---

## RECOMMENDED TIMELINE

If working with agentic system:
- **Sprint 1 (Week 1-2):** Entity pipeline + key bindings + sound dispatch
- **Sprint 2 (Week 3-4):** Alias models + lightmaps
- **Sprint 3 (Week 5-6):** Menus + brush submodels
- **Sprint 4 (Week 7-8):** Prediction physics + sprite/particle integration
- **Sprint 5 (Week 9+):** Music, polish, demo recording

Estimated effort: **200-300 engineer-hours** to full feature parity (assuming experienced Quake engine developer).
