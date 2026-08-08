# Diagnosis: Intermittent Runtime Anomalies (Textures, Triggers, AI, Sound)

Status: **OPEN — investigation in progress**
Last updated: 2026-08-07
Symptom owner: four reported stochastic failures, all reproducible-on-demand, none yet root-caused.

## Symptom summary

| # | Symptom | Subsystem | Repo state at write time |
| --- | --- | --- | --- |
| 1 | Textures sometimes mis-aligned | renderer | — |
| 2 | Moveable brushes sometimes don't trigger (e.g. one of a paired double door) | server/physics + QCVM sync | — |
| 3 | Enemy AI sometimes slow to notice/navigate to player | server/QC (`FindTarget`/`visible` + tracer + PVS) | — |
| 4 | Sounds sometimes delayed vs their trigger | server→client net (`StartSound`/`svc_sound`) | — |

All four are **stochastic**: they pass the obvious deterministic path, then misfire
occasionally. That profile points at state existing in two representations that can
drift (the QCVM-`EntVars` class of bug already fixed once in
`qbj2_trigger_regression.md`), or a path that depends on a float/timing/ordering
property that isn't integer-deterministic.

## Baseline (2026-08-07)

- `TMPDIR=<repo>/.tmp CGO_ENABLED=0 go test ./...`:
  - **Only failure**: `TestProjectFilesUnderLineCeiling` (internal/testutil).
  - Root cause: `knownOversizedFiles` allowlist is an **empty map**, so the guard is a
    hard 1,000-line ceiling; the four mechanically-ported QuakeC files
    (`pkg/qgo/quakego/hellknight.go` 1319, `zombie.go` 1481, `ogre.go` 1120,
    `player.go` 1084) legitimately exceed it. Deterministic on clean `HEAD`; **not**
    caused by any of the four symptoms. The follow-up commit
    `bb3f665d` ("count only code lines") did not un-break it: `countCodeLines` strips
    only `//` and `/* */` comment content, but these files are checkerboarded with
    code-per-comment styles that still count as code.
  - **Decision**: record, don't fix (unrelated, per AGENTS.md). If it blocks CI, fix
    = either repopulate the allowlist (debt) or split the four files (large, mechanical).
- `mise run smoke-map-start` not yet run (needs display; deferred).
- Symlinks `ironwail/` → `../ironwail`, `quake-data/` → Quake Enhanced verified present.

## C reference anchors (cross-validation targets)

- Physics: `ironwail/Quake/sv_phys.c` — `SV_Physics` (1222), `SV_Physics_Pusher`
  (618), `SV_PushMove`, `SV_Physics_Step`, `SV_Physics_Toss` (1113).
- World/links/touch: `ironwail/Quake/world.c` — `SV_AreaEdicts`, `SV_LinkEdict`,
  `SV_Impact` (touch dispatch).
- QC builtins: `ironwail/Quake/pr_cmds.c` — `find`, `checkclient`, `traceline`,
  `sound`, `tracetoss`, `aim`.
- Renderer: `ironwail/Quake/gl_model.c` (Mod_LoadFaces, texture anim links),
  `ironwail/Quake/r_brush.c` (`R_TextureAnimation` 59-87), `ironwail/Quake/r_world.c`
  (brush/frame draw 470-600), `ironwail/Quake/r_bsp.c`/`r_shared` for UV/texcoord gen.
- Client sound: `ironwail/Quake/snd_dma.c`, `ironwail/Quake/cl_parse.c`
  (`CL_ParseSound`), `ironwail/Quake/sv_main.c` (net message flush).

Compared against Go:
- `internal/server/physics/leafs.go` — Go `SV_Physics_Pusher` (666), `PushMove` (433),
  `RunThink` (329), `PhysicsStep` (716). Compare carefully with C line-by-line.
- `internal/server/world.go` — `touchLinks` (157-230), `SV_AreaEdicts`,
  `SV_Impact` equivalent `Impact` (leafs.go:373).
- `internal/server/server_qc_sync.go` — `newCheckClient` (113) vs C `SV_CheckClient`.
- `internal/server/server.go:319` — QC `Traceline` builtin vs C `SV_TraceLine`/`SV_Move`.
- `internal/renderer/renderer_gogpu_world_material.go` —
  `animateWorldMaterials` (24) vs C `R_TextureAnimation`.
- `internal/renderer/world/transform.go` — `BuildBrushRotationMatrix` (13) vs C
  `R_EntityMatrix`/`Build_EntityMatrix` (row/column order + sign conventions).
- `internal/server/server_net_send.go:80` — `StartSound` vs C `SV_StartSound`;
  `internal/server/message.go` — `MessageBuffer` vs C `sizebuf_t` + NetQuake/Fitz
  encoding (`PRFL_*`).

## Symptom 1 — mis-aligned textures

**Hypotheses (highest confidence first):**

1. **Brush entity render matrix**: `BuildBrushRotationMatrix` (transform.go:13) uses
   `qtypes.AngleVectors(Vec3{X: -angles[0], Y: angles[1], Z: angles[2]})` then a
   4x4 with forward/right/up and **negated right row**. Any row/column or sign slip
   vs C's `R_EntityMatrix` (gl_rmain.c / r_shared.c) yields axis-aligned but
   offset/wrong-per-pitch UVs on world-space brush models (doors, lifts, trains)
   — exactly "sometimes mis-aligned."
2. **Texture animation sampling time**: Go `animateWorldMaterials` uses the single
   camera `timeSeconds` (renderer_gogpu_world_render.go:198-204), and swaps an
   **atlas index** once per material per frame. C `R_TextureAnimation(t, frame)`
   (r_brush.c:59) is called **per-surface per-draw** with `e->frame`, and `frame`
   differs per entity (world frame 0, brush frame `e->frame`). If the Go path
   applies a single global anim time to all surfaces, world vs brush anim textures
   desync, and atlas-bounds swap at a per-frame boundary ≠ per-entity cadence.
3. **Texcoord precision / compute path**: UV lerp or lightmap grid rounding in the
   geometry/compute shader for a subset of surfaces (check `bspdiag face` vertex UVs
   against shader input).

**Repro/verify steps:**
- Screenshot pair vs C at same `viewpos`/`viewangle` (`mise run parity-*`).
- Classify: static world only, brush-only, anim-water-only → attributes to
  hypothesis #1/#2/#3.
- `bspdiag face <id> <quake_dir> <map.bsp>` for a known-bad face; compare UVs.
- If brush-only → side-by-side `BuildBrushRotationMatrix` vs `R_EntityMatrix`.
- If anim-only → `animateWorldMaterials` vs `R_TextureAnimation` (alternate + frame).

## Symptom 2 — moveable brushes not triggering (double doors)

### FINDING (2026-08-07, corrected after deep-dive)

The qbj2_mtsch twin-door path is **C-correct and healthy**. Deterministic probe
`TestQbj2TwinDoorsBothFireViaChain` (PASS) proves:

- Twin `func_door` (*6 = #37 master, *7 = #38 chained) link correctly (`#37.enemy=38`).
- qbj2's `door_link` (quake-data/qbj2/src/doors.qc) **intentionally NULLs the
  chained door's targetname** and folds it onto the master — so asserting "both
  keep targetname" is the **wrong contract** (it misled the earlier "root cause").
- `door_fire` walks the Enemy chain and **BOTH halves moved** after the counter ->
  relay fire (`master_orig=[...,-38,0]`, `half_orig=[...,38,0]` — mirrored slide).

**What was ruled out**: spawn-time targetname clobber (mod-intended), QCVM
pointer-store ABI (global-ABI is correct; immediate-operand experiment broke
worldspawn), OPStoreP* semantics (C-faithful), LinkDoors double-run (matches C's
two `SV_Physics()` in SV_SpawnServer).

**Where the real symptom likely hides**: the deterministic spawn/fire path is
sound, so the intermittent "one door doesn't trigger" must be a RUNTIME
interleaving, not spawn/link: candidates are `AttackFinished`/`wait` timing when
the player re-triggers mid-cycle, `door_touch`'s key-card path skipping one half,
or a physics-stutter frame where `nextthink` lands exactly on a frame boundary.
Next steps: instrument `door_use`/`door_fire` per-frame with `sv_debug_qc_trace`
during a real play session; add a probe that re-triggers the doors while they are
STATE_TOP/STATE_UP (the mod's `DOOR_TOGGLE`/`wait` branches in door_fire/door_go_up).

### Interleaving probes (all PASS — these paths are NOT the bug)
- `TestParityDoorChainFiresBothHalves` — owner/enemy-chained pair both fire.
- `TestParityDoubleDoorPairAdvancesBothHalves` — both halves advance same-frame.
- `TestQbj2TwinDoorsBothFireViaChain` — real qbj2 pair fires both halves.

### Hypotheses (original, superseded for this map but kept for other cases)

1. **Pusher/rider scanning + touchLinks decoupling**: `SV_PushMove` scans
   `1..NumEdicts` and pushes only entities riding or overlapping the **final**
   AABB. Paired double doors share a target but are two separate edicts; a frame
   where one half's `groundentity`/AABB check misfires (float, or `AreaEdicts`
   leaf descent skipping one held box) leaves half the pair stationary → only one
   door triggers. Cross-check `touchLinks` candidate fetch (world.go:157) vs
   `SV_AreaEdicts` (world.c) for the leaf/box exclusion rule.
2. **QCVM-nextthink clobber**: think function set by QC (`nextthink`, `think`,
   `velocity` via `SUB_CalcMove`) must survive to the pusher loop the same frame.
   `syncQCVMState()` was removed from the frame path (`qbj2_trigger_regression.md`
   Phase 1), but any remaining `EntVars`→QCVM or QCVM→`EntVars` unilateral copy
   still risks clobbering half a frame's mutations. Look for any sync that runs
   *between* `touchLinks` and the `SV_Physics` pusher dispatch.
3. **`SV_Physics` ordering**: C runs `StartFrame`, then per-edict physics in
   **edict order**; Go must match (esp. `touchLinks` inside `LinkEdict(ent,true)`
   vs push-first). A Go path that relinks a door *after* its half was touched can
   drop one pair.

**Repro/verify steps:**
- `sv_debug_push 1` + `sv_debug_trigger 1`; capture both doors' edict numbers,
  classnames, `groundentity`, `touch`, `absmin/max` while failing.
- `bspdiag entities <quake_dir> <map.bsp>` to check target chain + both func_door
  defs with `target`/`targetname`/`wait`/`speed`.
- `sv_debug_qc_trace 1` to see which door's `door_use`/`door_go_down` fired vs not.
- Write red test: two `func_door` halves sharing a trigger; assert both nextthinks
  advance in the same frame.
- Diff `PushMove` vs C `SV_PushMove` for the `riding` AABB test and
  `touchLinks` vs `SV_AreaEdicts` box/leaf callback.

## Symptom 3 — enemy AI slow to notice/navigate to player

**Progress (2026-08-07)**: `checkclient()` PVS path verified **C-faithful**:
- C `PF_newcheckclient`/`PF_checkclient` (pr_cmds.c:771-886) and Go
  `newCheckClient`/`CheckClient` (server_qc_sync.go:113, server.go:418) implement
  the identical algorithm: cycle client slots every ≥0.1s, cache the selected
  client's view-leaf PVS, then return the client only if **self's** view leaf bit
  is set in that cached PVS. Recompute gates and dead-client skips match.
- Open-line tracer (fraction 1) and water-trace visible semantics PASS
  (`TestParityAITracelineReportsClearLOS`, `TestParityAITraceThroughWaterStillVisible`).

**Remaining candidates** (need in-session capture):
1. `SightEntity` global staleness: it is a pointer into the QCVM flat edict
   buffer; `ensureQCVMEdictStorage` grows via `make`+`copy`, so a `SightEntity`
   cached across a buffer realloc would dangle (0.1s window, low but nonzero).
   Also, edict-slot free/reuse at spawn (proven in qbj2: slots 33/37) can put a
   recycled slot inside `SightEntity`'s 0.1s validity window.
2. `visible()` in map geometry: the tracer fraction semantics in dense BSP
   (leaf-node bias, `NodeLineOffset`) — needs `sv_debug_qc_trace` on a failing
   monster.
3. Monster `Enemy`/`GoalEntity` chain and `WALK` navigation (`ai_forward`/
   `WalkMove`) — needs an in-game trace.

### Symptom 3/4 in-game capture playbook (the blockers)
Run the engine with these flags and capture a session where the symptom occurs:
```
./ironwailgo -basedir "$QUAKE_DIR" \
  +sv_debug_telemetry 1 \
  +sv_debug_qc_trace 1 \
  +sv_debug_push 1 \
  +sv_debug_trigger 1 \
  +developer 1 2>&1 | tee /tmp/iwgo_sess.log
```
Expected signatures:
- **AI not noticing**: look for `FindTarget`/`checkclient`/`visible` trace lines;
  if `checkclient` returns 0 while the player has LOS → PVS/leaf bug; if it
  returns a client but `visible()` returns FALSE → tracer/geometry bug; if the AI
  never even calls FindTarget → AI think/state bug.
- **One door not triggering**: watch `door_use`/`door_fire`/`door_go_up` trace
  lines for BOTH halves; if only one half's `door_go_up` fires or one half's
  `nextthink` is 0 → state machine / SUB_CalcMove scheduling bug (the harness
  observed `state=UP, think=SUB_CalcMoveDone, vel=0, nextthink=0` — armed but
  never scheduled — a prime suspect for both symptoms 2 and 4).
- **Delayed sound**: `sound()`/`StartSound` trace + `host_speeds` frame timing;
  if the `svc_sound` message is written but the client plays it a frame late →
  client parse/batch; if it's dropped near the watermark → datagram policy.

## Symptom 4 — delayed sounds

**Progress (2026-08-07)**: plain open-line tracer and water-trace both PASS
(`TestParityAITracelineReportsClearLOS`, `TestParityAITraceThroughWaterStillVisible`)
— `SV_Move`/`Traceline` fidelity is excluded for open and water lines. Hunt
`newCheckClient`/PVS staleness and `SightEntity` pointing at a recycled edict
slot. Note: the edict-slot-address churn proven in symptom 2 (spawn-time slot
reuse + pointer-store landing errors) is a strong candidate for "AI targets a
phantom edict slot" — same root area, worth checking `checkclient()` and
`visible()`'s entity-reference handling against slot reuse.

**Hypotheses:**

1. **Tracer fidelity**: `visible()` (quakego/ai.go:92) depends entirely on the
   `Traceline` builtin returning a **bit-exact-ish** `trace_fraction` to C
   `SV_TraceLine`/`SV_Move`. Any fp divergence in clip hulls, `trace_allsolid`,
   or water/lava `trace_inopen/inwater` semantics flips `visible()` → AI never
   agros until `infront`+range pass. This is top suspect for "struggles to notice."
2. **`newCheckClient`/PVS vs C `SV_CheckClient`**: Go caches `checkClientPVS`
   per client slot (server_qc_sync.go:168-175). If the PVS is from a stale leaf
   or client slot reuse is off-by-one, `checkclient()` returns a **different or
   stale** edict → AI targets a phantom. Symptom matches: "navigating to them"
   when the stored `SightEntity` points at freed/recycled slot.
3. **Global `SightEntity`/`SightEntityTime` staleness**: these are package-level
   `var`s (ai.go:8-11) exactly mirroring C, but if edict slots recycle without
   clearing, an AI inherits another's `SightEntity` and "believes" it saw you.
4. **`WALK` move ordering**: `ai_forward`/`WalkMove` needs correct ground-plane
   trace; reuse any pusher/step divergence from #2.

**Repro/verify steps:**
- `sv_debug_qc_trace 1` on an AI in a room it should agro; watch `FindTarget`,
  `visible`, `infront` calls and their arguments.
- Dump `newCheckClient` slot/PVS values vs C `SV_CheckClient` rotation.
- Red test: place monster + player with clear LOS at `RANGE_MID`; assert
  `FindTarget()` returns TRUE within N frames (compare to same map under C's
  `findclient`).

## Symptom 4 — delayed sounds

**Progress (2026-08-07)**: same-frame `StartSound` emits `svc_sound` immediately
and a datagram at the `MaxDatagram-21` watermark boundary still writes
(`TestParitySoundEmittedSameFrame`, `TestParitySoundNotDroppedNearWatermark`).
The message-buffer watermark is NOT the delay. Re-examine the client `parseSound`
field-mask path and audio-system batching next.

**Hypotheses:**

1. **Message-buffer waterline**: `StartSound` writes `svc_sound` into the
   per-client `MessageBuffer`; if the write lands **after** the datagram is
   truncated/flushed (capacity or `net_msg` waterline), the sound is dropped or
   delayed until the next flush. C flushes where the writes guarantees ordering;
   Go's `StartSound` (server_net_send.go:80) may append after a `sizebuf` marker
   that the flush already consumed → next-frame delivery ("delayed").
2. **NetQuake vs Fitz encoding**: `svc_sound` field-mask path (client parse.go:664-
   743) differs for large entity/channel/sound indices (`StartSound` tests in
   server_test.go:28, sv_send_sound_protocol_test.go). If the client misdecodes a
   `serversound` (channel/entity) it may skip a play call or attach to wrong ent.
3. **Audio system batching**: `internal/audio` may batch sounds per-frame and
   local audio thread buffers one frame; combined with #1 this looks like "late."

**Repro/verify steps:**
- Reproduce with `-headless`/`-screenshot` + `host_speeds`, dump the message
  buffer content on the failing frame.
- Trace `StartSound(sample)` → message write → client `ParseSvcSound` on the same
  frame with a breakpoint/bridge (or add a one-off slog line guarded by a cvar).
- Red test: fire a sound via `sound()` builtin with `host_frametime` stepped;
  assert the `svc_sound` message arrives in the client within the same or next
  server frame (vs C).

## Cross-cutting infrastructure check (shared root)

The door/AI/sound symptoms all pass through the **same** per-frame `SV_Physics`
loop and the QCVM↔`EntVars` field access layer. Before deep-diving each, run the
targeted sweep:

- Audit remaining production `ent.Vars.*` direct reads/writes (the
  `qbj2_trigger_regression.md` Phase 3 table is the road map).
- Confirm `touchLinks`/`areaTriggerEdicts`/`LinkEdict`/`SV_TestEntityPosition` all
  read via accessors so any QC mutation is visible same-frame.
- Confirm **no** sync writes QCVM between `touchLinks` dispatch and the pusher
  loop (else a QC-set `nextthink`/`think`/`velocity` set by a door's `use` can be
  clobbered for one door of a pair).

## Execution log

- 2026-08-07: baseline run — suite green except pre-existing
  `TestProjectFilesUnderLineCeiling`; doc skeleton written; symlinks verified.
- 2026-08-07: wrote `internal/server/parity_intermittent_probes_test.go` (3 guards,
  all PASS) + `parity_interleaving_probes_test.go` (3 guards, all PASS). Initial
  "red probe" attempts failed only due to test-harness bugs, which RULED OUT the
  base paths: door pusher/think, open+water tracer, same-frame + watermark sound.
- 2026-08-07: **Symptom 2 deep-dive (CORRECTED).** Initial root-cause claim
  (twin-door targetname clobber by a QC pointer store) was wrong: qbj2's
  `door_link` source (quake-data/qbj2/src/doors.qc) deliberately NULLs the
  chained door's targetname. The Go engine implements `door_fire`'s Enemy-chain
  correctly — `TestQbj2TwinDoorsBothFireViaChain` proves BOTH halves move.
  Also ruled out: QCVM pointer-store ABI (global-ABI correct; immediate-operand
  experiment broke worldspawn, reverted), OPStoreP* semantics, LinkDoors
  double-run. NOTE: a door re-trigger harness observed the doors stuck at
  `state=UP, think=SUB_CalcMoveDone, vel=0, nextthink=0` (armed, never
  scheduled) via the counter-use path — but that harness fired the wrong
  relay chain, so it remains a candidate, not confirmed. Real confirmation
  needs the in-game playbook below.
- 2026-08-07: **Symptom 3 progress.** `checkclient()` fully verified
  C-faithful (algorithm + gates identical). Open/water tracer PASS. Remaining
  AI candidates: `SightEntity` buffer-realloc staleness, tracer in dense BSP,
  WALK nav — see the in-game capture playbook.
- (next) run the in-game playbook (sv_debug_* + developer) during failing
  sessions; classify symptom 1 via parity screenshots (brush matrix verified
  equal to C for yaw-only doors).


