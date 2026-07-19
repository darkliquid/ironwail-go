# qbj2 Start Rendering Issues — Diagnosis

## Issue 1: Button panels don't change texture when pressed

### Root Cause: Brush entity `Frame` field is plumbed but never consumed

The entire pipeline faithfully carries the QuakeC `self.Frame` value from server to renderer:

1. **QuakeC** sets `self.Frame = 1` on press, `self.Frame = 0` on return (`pkg/qgo/quakego/buttons.go:30,50`)
2. **Server** propagates `Frame` through entity state to client
3. **Game layer** maps `state.Frame` → `renderer.BrushEntity.Frame` (`internal/game/game_entity.go:57,75`)
4. **Renderer** stores `Frame` in `BrushEntityParams` (`internal/renderer/world/gogpu/brush_build.go:7`)
5. **Draw structs** carry `frame` through classified/opaque/translucent pipelines (`world_gogpu_brush_render.go:43`, `world_gogpu_translucent.go:685`)

**But every render loop ignores it.** The critical gap:

`internal/renderer/world_material_gogpu.go:33`:
```go
frameTexture, err := surfacepkg.TextureAnimation(anim, 0, float64(timeValue))  // frame HARDCODED to 0
```

No production code path ever calls `TextureAnimation` with anything other than `frame=0`. The alternate texture chain (`+Abutton`/`+Bbutton` for pressed state) is never selected.

In the brush entity render loops (`world_gogpu_brush_render.go:187-225`, `691-720`), texture selection uses `drawTextures.bindGroup` — the entire atlas bind group — with no per-face animated texture index resolution. The vertex `MaterialID` points at the base (frame-0) material slot, and the materials buffer is never rewritten for brush entities.

### Proposed Fix

The materials buffer (`r.worldMaterialsBuffer`) is a GPU uniform buffer that the shader reads as `materials[MaterialID]`. For world faces, `updateWorldMaterialsBuffer` rewrites it each frame with frame-0 animations. For brush entities, it needs to be rewritten **per entity** with that entity's frame before drawing that entity's faces.

**Changes needed:**

1. **`animateWorldMaterials`** (`world_material_gogpu.go:20`) — add `frame int` parameter:
   ```go
   func animateWorldMaterials(baseMaterials []WorldMaterialData, animations []*surfacepkg.SurfaceTexture, frame int, timeValue float32) []WorldMaterialData
   ```
   Pass `frame` at line 33: `surfacepkg.TextureAnimation(anim, frame, float64(timeValue))`

2. **Opaque brush render loop** (`world_gogpu_brush_render.go:169-226`) — before drawing each entity's faces, compute and write an animated materials buffer using `draw.frame`:
   ```go
   for _, preparedDraw := range scratch.classifiedPrepared {
       draw := scratch.classifiedDraws[preparedDraw.drawIndex]
       // NEW: write per-entity animated materials
       if draw.frame != 0 && len(baseMaterials) > 0 {
           animated := animateWorldMaterials(baseMaterials, draw.textureAnimations, draw.frame, float32(camera.Time))
           queue.WriteBuffer(materialsBuffer, 0, asBytes(animated))
       }
       // ... existing face draw loop ...
   }
   ```
   The materials buffer can be safely rewritten because brush entities render in a separate command encoder/render pass after the world pass.

3. **Translucent brush render loops** (`world_gogpu_translucent.go:490,577`) — same treatment. `draw.frame` is already available in `gogpuTranslucentBrushFaceRender.frame` (line 685), and `gogpuLateTranslucentTextureBindGroups` already takes (but ignores) `timeSeconds`.

4. **External BSP brush entities** — their materials buffer (`externalBrushBaseMaterials`) is written once at load (`renderer_gogpu_worldstate.go:466-468`) and never animated. Need to write a per-entity animated version using `draw.frame` before each external BSP entity draw.

**Edge case:** Multiple brush entities in the same render pass with different frames. Since we iterate entities one at a time and draw all faces before moving to the next entity, rewriting the materials buffer per entity is safe as long as we write before each entity's draw loop and the buffer isn't read concurrently.

### Telemetry to Add

- Log when a brush entity has `frame != 0`: entity index, frame value, model name
- Log which texture animation chains exist for the entity's textures (does `AlternateAnims` exist for `+0button`-style textures?)
- Verify the map's texture table has paired `+0*`/`+A*` textures (bspdiag can check this)

---

## Issue 2: Lift descent should have a teleport/transition

### Root Cause: Likely a trigger_teleport not firing when the lift carries the player

The map (qbj2) likely has a lift (func_plat) that carries the player down into a `trigger_teleport` volume, which should teleport the player to a different area. The teleport mechanism exists and works in principle:

1. `PushMove` (`physics.go:328`) moves the plat and calls `PushEntity` on riding entities
2. `PushEntity` (`physics.go:303`) calls `LinkEdict(ent, true)` → `touchLinks`
3. `touchLinks` (`world.go:649`) scans for overlapping `SOLID_TRIGGER` entities and fires QC touch callbacks
4. `teleport_touch` (`triggers.go:325`) moves the player to the destination

**But there are several failure modes:**

#### Failure mode A: Player not detected as riding
`PushMove` line 369 checks: `riding = (FL_ONGROUND set) && (GroundEntity == pusher)`. If the player's `GroundEntity` doesn't point to the plat (e.g., ground detection failed during descent), the riding check fails. Then the AABB overlap check (371-378) is the fallback — but if the player's bbox doesn't overlap the pusher's **swept** bounds (mins/maxs expanded by the move vector), the player is skipped entirely. No push, no `touchLinks`, no teleport.

#### Failure mode B: One-frame gap
The player's `PhysicsWalk` runs at a **lower edict index** than the plat's `PhysicsPusher`. So in the same frame:
1. Player's `PhysicsWalk` → `LinkEdict(player, true)` → `touchLinks` at the **old** position (before plat moved)
2. Plat's `PhysicsPusher` → `PushMove` → `PushEntity(player)` → `touchLinks` at the **new** position

If the teleport trigger is only reachable at the bottom of the descent, and the plat reaches the bottom but the player isn't pushed that last step (velocity=0 early return at line 329, or riding detection fails), the teleport never fires.

#### Failure mode C: Blocked-revert undoing teleport
If `PushEntity` fires the teleport (player moves to destination), but `TestEntityPosition` at the destination (line 402) returns blocked, the code reverts the player to its pre-push origin (line 431), undoing the teleport. This happens if the teleport destination is occupied.

#### Failure mode D: Entity sync issue
After `touchLinks` fires the QC `teleport_touch`, the player's origin/flags are changed in QCVM space. If `syncEdictFromQCVM` doesn't run after the touch callback, the Go-side entity state (used by subsequent physics) would be stale. The `touchLinks` function calls `executeQCFunction` which should handle sync, but this needs verification.

### Proposed Fix

The most likely cause is **Failure mode A** (player not detected as riding) or **Failure mode B** (one-frame gap at the bottom of descent). The fix depends on which:

1. **If riding detection fails:** Verify `GroundEntity` is correctly set when the player stands on the plat. Check `SV_CheckGround` (`physics.go`) and the `LinkEdict` area filtering. The fix would be ensuring ground detection works for `MOVETYPE_PUSH` entities.

2. **If one-frame gap:** After the plat finishes its descent (velocity becomes 0 at `plat_hit_bottom`), the player is at the bottom but the trigger may need one more `touchLinks` call. The fix could be: when `PushMove` has zero velocity (line 329 early return), still call `touchLinks` for any riding entities at their current position. Or: set `force_retouch = 2` in QC when the plat reaches bottom.

3. **If blocked-revert:** Ensure the teleport destination is clear, or handle the teleport-after-push case specially in `PushMove` (don't revert if the entity was teleported rather than pushed).

### Telemetry to Add

- **`PushMove` tracing:** Log pusher index, velocity, movetime, and for each checked entity: riding? (yes/no), AABB overlap? (yes/no), was pushed? (yes/no)
- **`touchLinks` tracing:** Log which triggers are found overlapping the player, their classname, and whether the touch callback fires
- **Player state on lift:** Log `GroundEntity`, `FL_ONGROUND`, origin, and the plat's origin each frame during descent
- **Trigger entity dump:** Use bspdiag to list all `trigger_teleport` entities in qbj2 with their position, target, and spawnflags
- **`teleport_touch` tracing:** Log when `teleport_touch` fires, the source and destination origins

**Quick diagnostic command:** Add a cvar `sv_trace_push` that enables `slog.Debug` logging in `PushMove` and `touchLinks` for the player entity only. Run qbj2, ride the lift down, and check logs.

---

## Issue 3: Liquid/water lighting is wrong compared to C version

### Root Cause: Likely `r_litwater=1` lighting liquids with invalid/zero lightmap data

The C engine (GLQuake) **never allocates lightmaps for `SURF_DRAWTURB` surfaces** — liquids are always rendered fullbright (`qglColor4ub(255,255,255,255)` with `GL_REPLACE`). Ironwail added optional lit water via `r_litwater`.

ironwail-go's implementation (`world_shaders_gogpu.go:565-583`):
```wgsl
var totalLight = vec3<f32>(0.5);                    // default: half-bright
if (uniforms.litWater > 0.5) {                      // only if litWater uniform set
    totalLight = textureSample(worldLightmap, ...); // sample real lightmap
}
let lit = sampled.rgb * totalLight * 2.0 + ...;     // 0.5 * 2.0 = 1.0 = fullbright
```

The math is correct: `0.5 * 2.0 = 1.0` gives fullbright when unlit. **But the issue is the lit path.** When `r_litwater=1` (default) and the face has `LightmapIndex >= 0` and `lightmapSurface != nil`, the shader samples the lightmap. If the lightmap data for liquid surfaces is **all-zero, all-white, or incorrectly sampled**, the lighting will be wrong.

The lightmap sample expansion (`world/lightmap_samples.go` — `ExpandLightmapSamples`) has **no special-casing for turbulent surfaces** — it processes them identically to solids. But in the original BSP format, `SURF_DRAWTURB` faces may have:
- `LightOfs = -1` (no lightmap data) — correctly handled by `LightmapIndex < 0` check
- `LightOfs >= 0` but with lightmap samples computed at wrong positions (turbulent surfaces have warped geometry, so the lightmap sample points may not match the visible surface)
- Lightmap data that was computed but is incorrect for the warped UVs

**Key question:** Does qbj2's liquid faces have `LightOfs >= 0`? If so, the Go renderer is lighting them with potentially bad lightmap data, while the C engine would render them fullbright.

### Proposed Fix

**Option A (quick, matches C parity):** If the map doesn't have proper lit water data, force `litWater=0` for liquid faces. Check if the lightmap data for liquid surfaces is valid (not all-zero, not all-128 midpoint). If invalid, skip lightmap sampling.

**Option B (Ironwail parity):** Keep `r_litwater=1` but verify the lightmap sample positions for turbulent surfaces match Ironwail's. The C Ironwail computes lightmap samples for water surfaces at specific positions that account for the warping. The Go version may be sampling at wrong positions.

**Option C (diagnostic):** Add a cvar `r_litwater_force` (0=auto, 1=always lit, 2=always unlit) to allow testing both modes without changing map data.

### Telemetry to Add

- **Liquid face lightmap dump:** For each `SURF_DRAWTURB` face in qbj2, log: `LightOfs`, `LightmapIndex`, whether `lightmapSurface != nil`, and the first few lightmap sample values (are they all 128? all 0? varied?)
- **`worldFaceHasLitWater` result:** Log which liquid faces have lit water enabled and which don't
- **`r_litwater` cvar value:** Log the current value at map load
- **Shader uniform check:** Log the `litWater` uniform value being written for liquid face batches
- **Comparison test:** Toggle `r_litwater 0` and `r_litwater 1` in-game and compare screenshots with C version

**Quick diagnostic:** Add bspdiag output for liquid faces:
```
=== Liquid Faces ===
  face=123  flags=TURB|WATER  LightOfs=4567  LightmapIndex=3  samples=[128,128,128,128,...]  litWater=true
  face=124  flags=TURB|WATER  LightOfs=-1    LightmapIndex=-1 samples=N/A          litWater=false
```

This will immediately show whether the map has lightmap data for liquids and whether that data is meaningful (varied values) or degenerate (all midpoint/zero).

---

## Summary

| Issue | Root Cause | Fix Complexity | Telemetry Priority |
|-------|-----------|---------------|-------------------|
| Button textures | `animateWorldMaterials` hardcodes `frame=0`; brush entity render loops never consume `draw.frame` | Medium — per-entity materials buffer rewrite in 3 render loops | Low — mechanism is clear |
| Lift teleport | `trigger_teleport` likely not firing when lift carries player down; riding detection or one-frame gap | Unknown — needs telemetry first | **High** — must trace PushMove/touchLinks |
| Liquid lighting | `r_litwater=1` lighting liquids with potentially invalid lightmap data; C engine renders liquids fullbright | Low if Option A (force unlit for bad data); Medium if Option B (fix sample positions) | **High** — must inspect lightmap data for liquid faces |
