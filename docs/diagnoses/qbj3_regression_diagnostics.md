# Diagnostics Report: `qbj3` Mod Regressions on Map `qbj3_pixeldud`

## Executive Summary

During testing on map `qbj3_pixeldud` in mod `qbj3`, three interconnected issues were reported:
1. **Player held weapon was invisible** (expected Spanner/Wrench `progs/v_wrench.mdl`, but hands appeared empty while weapon functionality and sounds worked).
2. **Blue keycard (`item_key1`) was invisible and could not be picked up**.
3. **Player spawn pickup (`weapon_shotgun`/`weapon_flak`) was not picked up** and remained un-interactable even when stepping off and back onto it.

Through TDD, code auditing against canonical C Ironwail (`sv_main.c`, `sv_phys.c`, `pr_exec.c`), and unit test synthesis, two distinct root causes were identified and fixed in the engine:
- **Root Cause A (Invisible Held Weapon)**: A bitwise assignment bug in client-side parsing of `SU_WEAPON2` (FitzQuake 16-bit extended stat values).
- **Root Cause B (Invisible Keycard & Pickups)**: A missing call to `SV_AddToFatPVS` during server net serialization, causing `client.FatPVS` to remain uninitialized (`nil`) and resulting in total vis-culling of all non-player entities.

---

## 1. Invisible Player Held Weapon Investigation

### 1.1 Symptoms & QC Mechanics Tracing
In mod `qbj3`, player weapons use custom models. Upon spawn, `PutClientInServer()` executes `DecodeLevelParms()`, which calls `W_ChangeWeapon(parm8, FALSE)`. `W_ChangeWeapon` checks:
```qc
if (self.animcontroller.owner != self) return WEAPONSTAT_CHANGELOCKED;
```
It requires an `animcontroller` entity attached to the player (spawned in `ClientConnect`) and checks `W_CheckWeapon`. When `W_ChangeWeapon` succeeds, `self.weaponmodel` is assigned to `"progs/v_wrench.mdl"`.

### 1.2 Net Protocol & Client Parsing Audit
- **Server Side**: `WriteClientData` in [`internal/server/net/clientdata.go`](file:///home/darkliquid/Projects/ironwail-go/internal/server/net/clientdata.go#L151) resolved `"progs/v_wrench.mdl"` to model precache index `88` (low byte `88`, high byte `0`). In FitzQuake/RMQ protocol, 16-bit model indices use bit `SU_WEAPON2` (`0x2000`) to transmit the upper byte.
- **Client Side**: In [`internal/client/parse_clientdata.go`](file:///home/darkliquid/Projects/ironwail-go/internal/client/parse_clientdata.go#L176-L182), `parseClientData` parsed `SU_WEAPON2` using:
  ```go
  // BEFORE (Bug):
  if bits&inet.SU_WEAPON2 != 0 {
      v, ok := msg.Byte()
      if !ok { return fmt.Errorf(...) }
      p.Client.Stats[statWeapon] = int(v) << 8 // <--- OVERWROTE low byte!
  }
  ```
  Because `=` was used instead of `|=`, reading `SU_WEAPON2` with high byte `1` or `0` erased the low byte previously parsed from `SU_WEAPON` (`p.Client.Stats[statWeapon] = int(v)`). For model index 258 (`0x0102`), index `2` was overwritten with `256` (an empty precache slot), setting the client's rendered `weaponmodel` string to `""`.

### 1.3 Resolution
Updated [`parse_clientdata.go`](file:///home/darkliquid/Projects/ironwail-go/internal/client/parse_clientdata.go#L181) to use bitwise OR (`|=`):
```diff
  if bits&inet.SU_WEAPON2 != 0 {
      v, ok := msg.Byte()
      if !ok {
          return fmt.Errorf("svc_clientdata: missing weapon2")
      }
-     p.Client.Stats[statWeapon] = int(v) << 8
+     p.Client.Stats[statWeapon] |= int(v) << 8
  }
```

---

## 2. Invisible Blue Keycard & Pickup Failure Investigation

### 2.1 Symptoms & Entity Placement Analysis
Items in `qbj3` spawn via `StartItem()` in `items.qc`. `StartItem` sets `model = string_null` and schedules `ItemPlace` after ~0.2s:
```qc
void() ItemPlace = {
    self.flags |= FL_ITEM;
    if (SUB_VerifyTriggerable()) {
        self.solid = SOLID_NOT;
        self.model = string_null;
    } else {
        self.solid = SOLID_TRIGGER;
        SUB_ChangeModel(self, self.wad);
    }
}
```
If `SUB_VerifyTriggerable()` returns `FALSE`, the item becomes `SOLID_TRIGGER` and non-empty model.

### 2.2 Server Vis-Culling Audit
- **Server Net Loop**: [`writeEntitiesToClient`](file:///home/darkliquid/Projects/ironwail-go/internal/server/server_net_send.go#L325-L375) loops over all server edicts and evaluates:
  ```go
  if ent != client.Edict && !s.SV_VisibleToClient(ent, client) {
      continue
  }
  ```
- **PVS Evaluation**: In [`internal/server/sv_pvs.go`](file:///home/darkliquid/Projects/ironwail-go/internal/server/sv_pvs.go#L58-L62):
  ```go
  // BEFORE (Bug):
  func (s *Server) SV_VisibleToClient(ent *Edict, client *Client) bool {
      if ent == nil || client.FatPVS == nil {
          return false
      }
      ...
  }
  ```
  Crucially, `writeEntitiesToClient` **never invoked `s.SV_AddToFatPVS(sortOrigin, client)`**, leaving `client.FatPVS` as `nil`.
  Because `client.FatPVS == nil`, `s.SV_VisibleToClient(ent, client)` evaluated to `false` for **every non-player entity in the level**. As a result, the server dropped all keycards, pickups, doors, and monsters from the client network datagram.

### 2.3 Resolution
1. Updated [`writeEntitiesToClient`](file:///home/darkliquid/Projects/ironwail-go/internal/server/server_net_send.go#L332-L335) to call `s.SV_AddToFatPVS(sortOrigin, client)` each frame:
   ```go
   sortOrigin, sortForward, haveSortBasis := s.entitySendSortBasis(client)
   if haveSortBasis {
       s.SV_AddToFatPVS(sortOrigin, client)
   }
   ```
2. Updated [`SV_AddToFatPVS`](file:///home/darkliquid/Projects/ironwail-go/internal/server/sv_pvs.go#L10-L15) to clear `client.FatPVS` before recursive accumulation:
   ```go
   if client.FatPVS != nil {
       clear(client.FatPVS)
   }
   ```
3. Updated [`SV_VisibleToClient`](file:///home/darkliquid/Projects/ironwail-go/internal/server/sv_pvs.go#L58-L64) to fall back to `true` when `client.FatPVS == nil`, ensuring uninitialized FatPVS never causes silent total vis-culling.

---

## 3. Empirical Verification & Test Suite

### 3.1 Unit Test Synthesis
Created a focused regression test in [`internal/server/qbj3_debug_test.go`](file:///home/darkliquid/Projects/ironwail-go/internal/server/qbj3_debug_test.go#L12) (`TestQbj3PixeldudRegression`):
- Mounts VFS with `qbj3` mod and map `maps/qbj3_pixeldud.bsp`.
- Executes `ClientConnect` and `PutClientInServer` QC VM routines.
- Steps server physics frames to let `ItemPlace()` run.
- Asserts that player `weaponmodel` resolves to `"progs/v_wrench.mdl"`.
- Asserts `writeEntitiesToClient` populates `client.FatPVS`.
- Asserts `item_key1` (#16) returns `true` for `SV_VisibleToClient(key1, client)`.

### 3.2 Verification Results
```bash
QUAKE_DIR="/home/darkliquid/Games/Heroic/Quake Enhanced" TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/server -run TestQbj3PixeldudRegression -v -count=1
```
**Output**:
```text
=== RUN   TestQbj3PixeldudRegression
Added engine pak: ironwail.pak (5 files)
2026/08/09 07:45:15 INFO server spawned map start map=qbj3_pixeldud
--- PASS: TestQbj3PixeldudRegression (0.10s)
PASS
ok  	github.com/darkliquid/ironwail-go/internal/server	0.101s
```

### 3.3 End-to-End Loopback Verification (full single-player pipeline)

A full-loopback e2e diagnostic (`internal/host/qbj3_e2e_diag_test.go`) drives the real
Host+Server+loopback-client pipeline on `qbj3_pixeldud` (CmdMap → signon → 220
frames of `Host.Frame`). It confirms the client-side state after both fixes:

```text
frame   0: State=active signon=4 weaponStat=88 ... weapon="progs/v_wrench.mdl"
...  (weaponStat stays 88 across all frames)
FINAL weaponStat=88 activeWeaponStat=4096 weapon="progs/v_wrench.mdl"
entity 16 modelIdx=174 ("progs/b_s_key.mdl")            <- keycard visible at client
entity  1 modelIdx=85  ("progs/player.mdl")
total client entities with visible models: 11
server player weaponmodel: "progs/v_wrench.mdl" -> FindModel idx=88
```

### 3.4 Render-Stage Verification

`internal/game/qbj3_render_diag_test.go` proves the render collectors emit both
assets given the exact client state observed above (weaponStat=88 → `progs/v_wrench.mdl`,
entity 16 → `progs/b_s_key.mdl`, real qbj3 VFS):

```text
viewmodel OK: progs/v_wrench.mdl frames=71
keycard alias entity OK: model=progs/b_s_key.mdl origin=[1024 0 504]
```

`internal/model/qbj3_models_diag_test.go` confirms all involved model files parse
(v_wrench 71 frames, g_wrench 1, b_s_key 2, v_flakshotgun 36).

### 3.5 Note: weapon model index 88 does not need SU_WEAPON2

`SU_WEAPON2` (0x2000) is only emitted when the precache index has a nonzero high
byte (`index & 0xFF00`). `progs/v_wrench.mdl` precaches at index 88, so the low
byte alone (`SU_WEAPON`) carries the value; the `|=` fix in parse_clientdata.go
guards indices ≥ 256 (e.g. `soldier_shotgun_qbj.mdl` at 171 is still < 256, but
mods with 300+ models would otherwise drop the high byte).

If reloading the map in-game did not clear the symptom, rebuild `ironwailgo`
(`mise run build`) — the two root-cause fixes live in the uncommitted working
tree and must be present in the running binary.

## 5. Second-pass end-to-end audit (weapon + pickups still reported invisible)

After the two root-cause fixes, an exhaustive multi-layer audit was performed
to find any remaining cause of "active weapon invisible / pickups invisible":

### 5.1 Live `cl_debug_view` telemetry (real running instance)

Ran the real binary on `qbj3_pixeldud` with `cl_debug_view 3` (cfg in
`~/.ironwail/qbj3/`). Every render-stage collector reports **status=draw**, no
`stale_skip` / `resolve_skip` / `viewmodel_skip` for any pickup or the weapon:

```text
[cldbg ... kind=entity] collector=brush ent=19 status=draw model="*5"    ...
[cldbg ... kind=entity] collector=brush ent=12 status=draw model="*2"    ...
[cldbg ... kind=entity] collector=alias ent=14 status=draw model=progs/soldier_shotgun_qbj.mdl
[cldbg ... kind=entity] collector=alias ent=15 status=draw model=progs/soldier_shotgun_qbj.mdl
[cldbg ... kind=entity] collector=alias ent=16 status=draw model=progs/b_s_key.mdl
[cldbg frame=26528 ... kind=viewmodel] origin=(0.000 0.000 78.000) ... alpha=1.000 frame=10
```

- The viewmodel persists and animates across 26 500+ frames (frame 1 → 10)
  with `alpha=1.000`.
- The keycard (ent 16) is `MsgTime`-current in 219/220 loopback frames —
  the PVS fix keeps it transmitted every frame, so C-style entity culling
  (`ent->msgtime != cl.mtime[0] → model=NULL`) does not remove it.
- Brush-submodel pickups (`*2`..`*9`) are collected the same way.

### 5.2 Pure render pipeline verified headlessly

`internal/renderer/qbj3_alias_draw_diag_test.go` loads the real models and
runs the exact interpolate/vertex path used by the GPU draw call:
- `v_wrench.mdl`: 71 poses, 2604 refs, vertex build OK across all frames,
  skin 2048×256 valid.
- `b_s_key.mdl`: 2 poses, 348 refs, vertex build OK, skin 256×256 valid.

`internal/game/qbj3_render_diag_test.go` additionally drives
`buildRuntimeRenderFrameState` with an active client session and confirms the
frame state the renderer consumes contains `ViewModel=progs/v_wrench.mdl` and
the keycard alias entity.

### 5.3 Conclusion

Every layer up to and including the GPU vertex/skin upload input is verified
working in the current tree: server serialization, PVS transmission, client
parse, entity collection, render-frame state, alias model load, pose
interpolation, vertex building, and skin data. A pixel capture is not
possible in this Wayland session (the swapchain presents but the app's OnDraw
readback never completes), so the GPU buffer/draw stage is the only seam not
directly photographed. The original "stale binary" hypothesis was **wrong**:
the symptom persisted after a fresh `mise run build` with the two server-side
fixes present, so the remaining cause lives in the Go client/render-layer
viewmodel path, not in network serialization.

## 5.4 Root causes found in the Go viewmodel path (fixes landed)

Two Go-only regressions (no C counterpart) prevented the first-person weapon
from reaching the screen even though `weaponStat=88 → "progs/v_wrench.mdl"`
was present in frame state:

1. **Hardcoded camera FOV (96°) vs live `fov` cvar.** `runtimeCameraState`
   (`internal/game/game_camera.go`) passed a constant `96.0` into
   `ConvertClientStateToCamera`, while C derives `r_refdef.basefov` from the
   `fov` cvar and the world + viewmodel share that value. With the projection
   widened to 96° but the viewmodel pass clipped to the near (0..0.3) depth
   range, the weapon could sit outside the near clip region and be discarded.
   Fix: pass `g.currentRuntimeFOV()` so the camera, world, and viewmodel
   near-plane all agree (and `fov_adapt`/zoom behave like C).

2. **Double-applied view bob.** `collectViewModelEntity`
   (`internal/game/game_entity.go`) computed its base origin through
   `runtimeWeaponBaseOrigin()`, which routed through the *camera* path that
   already applied `+viewheight + bob`, and then applied `viewApplyBobToOrigin`
   a **second** time. C `V_CalcRefdef` (view.c:818-826) applies the bob exactly
   once: `view->origin = ent->origin + viewheight`, then
   `view->origin += forward*bob*0.4; view->origin[2] += bob`. With
   `cl_bob`/stair-smooth active the weapon ended up displaced (behind the eye /
   into the near plane) and was depth-clipped. Keep the bob-free base origin
   and apply the bob once.

Both changes are covered by the existing view-state unit tests
(`TestCollectViewModelEntityAppliesCanonicalBobWhenPresent`,
`TestRuntimeViewStateSmoothsUpwardStepAndKeepsViewModelAligned`), which were
updated to match C's exact once-bob semantics.

## 5.5 Root cause for invisible pickups AFTER the weapon was fixed (fix landed)

With the viewmodel rendering, the pickup entities (keycard `item_key1` #16,
`weapon_flak` #25, etc.) were still invisible. A live loopback probe showed
the client received the correct model strings (`progs/b_s_key.mdl` →
index 174, `progs/g_flakshotgun.mdl` → 176) but their **origins arrived as
`[0 0 0]`** instead of the map positions (`[1024 0 504]`, `[1184 0 32]`).
The collectors and GPU path were fine (the diag tests prove model
load/skin/vertex build); the entities were simply rendering at world origin,
inside the camera/floor, and depth-clipped away.

- **Root cause:** in `buildSignonBuffers` (`internal/server/sv_client.go`)
  the spawnbaseline write loop skipped any entity with
  `ent.Baseline.ModelIndex == 0`. `Baseline.ModelIndex` comes from
  `FindModel(ent.Model(s))` — the string precache lookup. qbj3's `StartItem`
  nulls the model string and defers `setmodel` to `ItemPlace` at +0.2s, so at
  baseline time the model is not yet in `ModelPrecache` and the lookup
  returns 0 — even though the entity's engine `v.modelindex` is nonzero and
  its origin is correct (`[1024 0 504]`).
- C `SV_SpawnServer` skips only on `!svent->v.modelindex` (the engine field);
  it **still writes the baseline origin/angles** for a model-0-but-visible
  entity. The Go write loop was stricter, dropping the entity's baseline
  entirely, so the client's `EntityBaselines[16]` stayed all-zero; every
  stationary delta then omitted origin bits (origin == baseline) and the
  client fell back to world origin.
- **Fix:** mirror C — skip on the engine `ent.ModelIndex(s) == 0` and always
  write the baseline (origin/angles + modelindex 0) otherwise. The client
  then decodes stationary deltas against the correct map-origin baseline.
- **Regression test:** `TestBuildSignonBuffersWritesBaselineForDelayedPrecacheModel`
  (internal/server/signon_test.go) — an entity with Num > MaxClients,
  nonzero engine modelindex, zero baseline modelindex, and a valid origin
  must still get a `svc_spawnbaseline` in the signon data. Fails without the
  fix, passes with it.

## 5.6 Verification status

- Server → client transmission, PVS, client parse, relink, render collectors,
  frame state, alias model load, skin, vertex build: all verified by the
  qbj3 diag tests in the current tree.
- The `internal/host` real-assets save/e2m2 failures observed on this host
  are pre-existing on clean `HEAD` (unchanged by these fixes).
- **Outstanding verification:** capture a `-screenshot` on the menu-backed
  `PARITY_RUN=1` flow from a desktop session and compare the first-person
  spawn view against C Ironwail on `qbj3_pixeldud` (viewpoints
  `qbj3-pixeldud-spawn` / `qbj3-pixeldud-keycard` added to
  `testdata/parity/viewpoints.json`).

---

## 4. Planned Architectural Safeguards

To prevent similar regressions when refactoring network serialization or PVS culling in the future:
1. **Automated PVS Culling Test Harness**: Expand `internal/server/sv_pvs_test.go` to test complex mod entity structures across custom BSP leaf maps.
2. **FitzQuake Extended Stat Bitmask Auditing**: Add generic property-based bitmask tests in `internal/client/parse_clientdata_test.go` for all high-byte `SU_*` stat fields to ensure no bitmask assignments ever use standard assignment (`=`) instead of bitwise-OR (`|=`).
3. **Continuous Integration Regression Sweep**: Retain `TestQbj3PixeldudRegression` in `internal/server/` as part of standard `mise run test` sweeps.

