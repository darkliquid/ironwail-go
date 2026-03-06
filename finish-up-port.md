# Ironwail-menu-port plan implementation

### What’s still required for a **fully playable** port

- **1) Real asset mounting (PAK search path)**
  - You need robust `id1/pak0.pak` + loose-file override semantics (Quake search order), not just ad-hoc file reads.
  - Engine boot must reliably load: `progs.dat`, maps (`maps/*.bsp`), textures, sounds, menu/HUD assets, demos/saves.
  - This is the foundation for “load original Quake files” in production, not test mode.

- **2) Client runtime completion (major blocker)**
  - Current client side is still partial/skeleton vs upstream (`cl_main`, `cl_parse`, `cl_input`, `cl_tent`, `cl_demo` parity).
  - Missing pieces include robust server message parsing, temp entities/effects, prediction/interp, and full signon flow.
  - Without this, you can’t get stable, playable in-game behavior even if maps/entities load.

- **3) Renderer completion for world + entities**
  - Menu can now draw, but full gameplay needs reliable world/model/sprite/particle/HUD rendering.
  - GoGPU/OpenGL 2D code still contains simplified paths/stubs; 3D frame path parity is not complete.
  - Needs full frame orchestration: BSP surfaces, alias models, temp entities, light updates, HUD pass.

- **4) Menu/UI parity beyond “visible menu”**
  - Current menu is basic; upstream menu system is much richer.
  - Need options menus, keybind UI, save/load menus, multiplayer browser flow, proper text/font rendering and cursor behavior.

- **5) Save/Load + host command completeness**
  - `host/commands.go` still has TODOs (`load/save/spawn/connect/...`).
  - Savegame serialization/deserialization (entities + QC globals + world/client state) is required for real single-player completion.

### If target is “playable single-player first” (recommended order)

1. PAK/filesystem correctness  
2. Client parse/signon + prediction baseline  
3. Renderer world/entities/HUD minimum viable  
4. Host command TODO closure (`load/save/spawn`)  
5. Menu options/keybind/save-load screens  
6. Stability/perf pass (no crashes, level transitions, death/restart loop)

### Quick reality check

Your own plan file claiming many waves “done” is optimistic compared to code markers and feature depth. The **true remaining work** is mostly in:

- `internal/client/*` (runtime completeness),
- `internal/renderer/*` (non-stub parity),
- `internal/host/commands.go` (TODO command paths),
- full filesystem/PAK integration path for original game data.

If you want, I can turn this into a concrete “milestone-by-milestone implementation backlog” with exact file-level tasks and acceptance tests for each milestone.

## Milestone 0 — Baseline & Repro Harness (1 day)

**Goal:** lock a repeatable “known state” before big changes.

### File-level tasks

- `cmd/ironwailgo/main.go`
  - Add deterministic startup log markers (asset mount, qc load, map load, menu active, frame loop started).
- `internal/testutil/assets.go`
  - Add helpers to locate `id1/` from env (`QUAKE_DIR`) and skip cleanly if absent.
- `Makefile` (or `scripts/`)
  - Add smoke commands:
    - `smoke:menu`
    - `smoke:headless`
    - `smoke:map-start`

### Acceptance tests

- `go build -tags=gogpu ./cmd/ironwailgo` passes
- `WAYLAND_DISPLAY= timeout 5 ./ironwailgo` prints deterministic startup markers
- `go test ./internal/testutil` passes/skips cleanly without assets

---

## Milestone 1 — Real Quake Filesystem/PAK Mounting (2–3 days)

**Goal:** load original Quake assets from `id1/pak0.pak` + overrides.

### 2026-03-06 ordered-slice status

- [x] Slice 1 complete: deterministic numeric `pakN.pak` discovery/loading.
- [x] Slice 1 complete: case-insensitive pack-entry lookup on case-sensitive hosts.
- [x] Slice 1 complete: focused regression coverage for pack ordering and mixed-case pack entries.
- [ ] Next slice: broaden filesystem parity checks against real asset layouts and mod layering edge cases.

### File-level tasks

- `internal/fs/filesystem.go` (or equivalent)
  - Implement search path stack:
    1. later game dirs override earlier ones
    2. within a game dir: `pakN` → `pak0` → loose files
  - Add canonical Quake path normalization.
- `internal/fs/pak.go` (new if needed)
  - Parse PAK directory, expose `Open(name)` / `ReadFile(name)`.
- `cmd/ironwailgo/main.go`
  - Wire `-basedir`, `-game` args to FS init.
- `internal/host/init.go`
  - Inject initialized FS into subsystems instead of `nil`.

### Acceptance tests

- `go test ./internal/fs`:
  - can open `progs.dat`
  - can open `maps/start.bsp`
  - exact Quake precedence across mod/id1, pack, and loose assets
  - `pakN` ordering is numeric, not glob/lexicographic
  - mixed-case pack entries resolve case-insensitively
- Runtime:
  - `WAYLAND_DISPLAY= ./ironwailgo -basedir <quake>` logs mounted pak entries and successful reads

---

## Milestone 2 — Boot-to-Map Singleplayer Path (Host/Server/QC wiring) (3–4 days)

**Goal:** complete startup from binary → server spawn → map loaded.

### File-level tasks

- `internal/host/init.go`
  - Ensure proper init order: FS → QC VM → server world.
- `internal/host/commands.go`
  - Finish command paths needed for SP loop: `map`, `spawn`, `begin`, `prespawn`.
- `internal/server/sv_main.go`
  - Validate `SpawnServer("start")` end-to-end with real assets.
- `internal/qc/loader.go`, `internal/qc/exec.go`
  - Ensure progs load and initial QC calls used during map start are covered.
- `cmd/ironwailgo/main.go`
  - If no explicit map, default to `start` after menu/new game trigger path.

### Acceptance tests

- `go test ./internal/host ./internal/server ./internal/qc`
- Runtime smoke:
  - `WAYLAND_DISPLAY= ./ironwailgo -basedir <quake>` reaches “server spawned map start” log
  - no panic during first 300 frames

---

## Milestone 3 — Client Parse + Signon + Entity Update Baseline (4–6 days)

**Goal:** client becomes truly functional (not skeleton) for gameplay state updates.

### File-level tasks

- `internal/client/main.go`
  - Complete state machine (`disconnected -> connecting -> connected -> active`).
- `internal/client/parse.go`
  - Implement core svc message parsing used by signon and frame updates.
- `internal/client/input.go`
  - Build/send `UserCmd` consistently each frame.
- `internal/net/protocol.go`
  - Finalize missing constants/struct compatibility for parsed messages.
- `internal/net/datagram.go`, `internal/net/loopback.go`, `internal/net/udp.go`
  - Ensure packet read/write and sequencing work for local SP and remote path.

### Acceptance tests

- `go test ./internal/client ./internal/net`
- Add fixture-based parse tests for representative server message streams.
- Runtime:
  - player reaches active state
  - entity count > 0 and updates over time

---

## Milestone 4 — Renderer Gameplay MVP (world + models + HUD core) (1–2 weeks)

**Goal:** move from “menu visible” to “playable frame output”.

### File-level tasks

- `internal/renderer/surface.go`
  - Draw BSP world surfaces for active map.
- `internal/renderer/model.go`
  - Render alias models (weapons/monsters/items baseline).
- `internal/renderer/screen.go`
  - Frame orchestration: clear, world pass, entities, 2D overlay pass.
- `internal/renderer/renderer_gogpu.go`
  - Replace simplified 2D placeholders with real textured 2D quads.
- `internal/renderer/renderer_opengl.go`
  - Keep parity for debugging backend.
- `internal/hud/sbar.go` (new or expand)
  - Minimum HUD: health/armor/ammo + centerprint text.

### Acceptance tests

- `go test ./internal/renderer`
- Runtime:
  - map geometry visible
  - weapon model visible
  - HUD values update while playing
  - no black screen, no frame panic over 5 minutes

---

## Milestone 5 — Input/Gameplay Feel + Temp Entities (4–6 days)

**Goal:** make controls and combat feedback feel like Quake.

### File-level tasks

- `internal/input/types.go`
  - Complete key routing (game/menu/console/message) and binding application.
- `internal/client/input.go`
  - Add movement prediction/reconciliation baseline.
- `internal/client/tent.go` (new or equivalent)
  - temp entities: explosions, impacts, muzzle flashes.
- `internal/renderer/particle.go`
  - tie temp entity events to visible particle effects.

### Acceptance tests

- `go test ./internal/input ./internal/client ./internal/renderer`
- Runtime:
  - WASD/mouse look stable
  - explosions and impact effects appear
  - no major rubber-banding in loopback SP

---

## Milestone 6 — Menu/UX Completion + Save/Load (1 week)

**Goal:** full user-facing play loop without console hacks.

### File-level tasks

- `internal/menu/manager.go` + split files (`main.go`, `options.go`, `keys.go`, `loadsave.go`)
  - finish main/options/keys/load/save/quit flows
- `internal/host/commands.go`
  - implement `save`, `load`, `connect`, `reconnect` properly
- `internal/savegame/save.go` (new)
  - serialize/deserialize world, entities, QC globals, client state
- `internal/draw/manager.go`
  - use real `gfx.wad` from mounted FS (remove test-only assumptions)

### Acceptance tests

- `go test ./internal/menu ./internal/host ./internal/savegame`
- Runtime:
  - Start New Game from menu
  - Save in map, quit, reload save
  - Key rebinding works and persists

---

## Milestone 7 — Audio + Final Parity Hardening (4–6 days)

**Goal:** complete “fully playable” quality bar.

### File-level tasks

- `internal/audio/adapter.go`, `internal/audio/mix.go`, `internal/audio/spatial.go`, `internal/audio/sound.go`
  - listener updates, attenuation, channel limits, stable mixing
- `cmd/ironwailgo/main.go`
  - proper clean shutdown sequence and subsystem teardown
- `internal/host/frame.go`
  - tighten frame pacing and deterministic update order

### Acceptance tests

- `go test ./internal/audio ./internal/host`
- Runtime:
  - positional sounds correct
  - no audio crackle/leaks over long session
  - clean exit with no goroutine/resource leaks

---

## Final Definition of Done (“Fully Playable”)

- Builds cleanly: `go build -tags=gogpu ./cmd/ironwailgo`
- Can mount original Quake `id1/pak0.pak` and load `start.bsp`
- Menu is fully functional (new game/options/keys/save/load/quit)
- Player can move, fight, transition maps, save/load
- HUD and temp entities visible
- Audio positional playback works
- 30+ minute gameplay session without crash

---

## Issue 1 — M0: Baseline & Repro Harness

**Title:** `M0: establish deterministic startup/repro harness`

**Labels:** `milestone:m0`, `infra`, `testing`

**Body:**

### Objective

Create a deterministic baseline for startup/build/runtime verification.

### Tasks

- [ ] Add deterministic startup markers in `cmd/ironwailgo/main.go`:
  - [ ] FS mounted
  - [ ] QC loaded
  - [ ] map spawn started/finished
  - [ ] menu active
  - [ ] frame loop started
- [ ] Add asset-discovery helpers in `internal/testutil/assets.go` using `QUAKE_DIR`
- [ ] Add smoke commands (`Makefile` or scripts):
  - [ ] `smoke:menu`
  - [ ] `smoke:headless`
  - [ ] `smoke:map-start`

### Acceptance Criteria

- [ ] `go build -tags=gogpu ./cmd/ironwailgo` passes
- [ ] `WAYLAND_DISPLAY= timeout 5 ./ironwailgo` prints deterministic markers
- [ ] `go test ./internal/testutil` passes or cleanly skips without assets

---

## Issue 2 — M1: Real FS/PAK Mounting

**Title:** `M1: implement Quake search-path + PAK mounting parity`

**Labels:** `milestone:m1`, `fs`, `core`, `high-priority`

**Body:**

### Objective

Load original Quake assets via canonical search path behavior.

### Tasks

- [ ] Implement search path stack in `internal/fs/*`:
  - [ ] loose files in `id1/`
  - [ ] pak files in order (`pak0.pak`, `pak1.pak`, ...)
- [ ] Implement/finish PAK parser (`internal/fs/pak.go` if needed)
- [ ] Add canonical path normalization (Quake-style names)
- [ ] Wire `-basedir` and `-game` into FS init in `cmd/ironwailgo/main.go`
- [ ] Inject initialized FS via `internal/host/init.go` (no `nil` FS in normal path)

### Acceptance Criteria

- [ ] `go test ./internal/fs` covers:
  - [ ] open `progs.dat`
  - [ ] open `maps/start.bsp`
  - [x] exact Quake pack-over-loose search precedence
  - [x] numeric `pakN` override ordering
  - [x] case-insensitive pack-entry lookup
- [ ] Runtime logs show mounted pak entries and successful asset reads

---

## Issue 3 — M2: Boot-to-Map SP Pipeline

**Title:** `M2: complete host/server/qc boot path to start map`

**Labels:** `milestone:m2`, `host`, `server`, `qc`, `high-priority`

**Body:**

### Objective

Reach reliable singleplayer boot from startup to spawned `start` map.

### Tasks

- [ ] Finalize init ordering in `internal/host/init.go` (FS -> QC -> server)
- [ ] Complete core host commands in `internal/host/commands.go`:
  - [ ] `map`
  - [ ] `spawn`
  - [ ] `begin`
  - [ ] `prespawn`
- [ ] Ensure `SV_SpawnServer("start")` path is stable in `internal/server/sv_main.go`
- [ ] Fill missing startup QC calls in `internal/qc/*`
- [ ] Ensure default map path if no args (or menu new game path)

### Acceptance Criteria

- [ ] `go test ./internal/host ./internal/server ./internal/qc` passes
- [ ] Runtime reaches “server spawned map start” marker
- [ ] No panic in first 300 frames

---

## Issue 4 — M3: Client Signon + Parse Baseline

**Title:** `M3: implement client signon state machine + core message parsing`

**Labels:** `milestone:m3`, `client`, `net`, `high-priority`

**Body:**

### Objective

Make client runtime truly functional for gameplay state updates.

### Tasks

- [ ] Complete state machine in `internal/client/main.go`:
  - [ ] disconnected -> connecting -> connected -> active
- [ ] Implement core svc parsing in `internal/client/parse.go`
- [ ] Build/send `UserCmd` each frame in `internal/client/input.go`
- [ ] Finalize protocol constants/structs in `internal/net/protocol.go`
- [ ] Tighten packet sequencing/reliability in `internal/net/*`

### Acceptance Criteria

- [ ] `go test ./internal/client ./internal/net` passes
- [ ] Add fixture tests for representative signon/update message streams
- [ ] Runtime enters active client state and processes entity updates

---

## Issue 5 — M4: Renderer Gameplay MVP

**Title:** `M4: render playable scene (world + entities + HUD baseline)`

**Labels:** `milestone:m4`, `renderer`, `hud`, `high-priority`

**Body:**

### Objective

Move from menu-only output to in-game playable rendering.

### Tasks

- [ ] BSP world surface rendering in `internal/renderer/surface.go`
- [ ] Alias model rendering in `internal/renderer/model.go`
- [ ] Frame orchestration in `internal/renderer/screen.go`
- [ ] Replace simplified 2D in `internal/renderer/renderer_gogpu.go`
- [ ] Keep OpenGL path usable in `internal/renderer/renderer_opengl.go`
- [ ] Implement HUD baseline (`internal/hud/sbar.go` or equivalent):
  - [ ] health
  - [ ] armor
  - [ ] ammo
  - [ ] centerprint

### Acceptance Criteria

- [ ] `go test ./internal/renderer` passes
- [ ] Map geometry visible
- [ ] Weapon/entity models visible
- [ ] HUD updates while playing
- [ ] Stable for 5-minute run without render panic

---

## Issue 6 — M5: Gameplay Feel + Temp Entities

**Title:** `M5: complete input routing/prediction and temp entities`

**Labels:** `milestone:m5`, `input`, `client`, `fx`

**Body:**

### Objective

Make controls and combat feedback feel Quake-like.

### Tasks

- [ ] Complete key destination routing/bindings in `internal/input/types.go`
- [ ] Implement prediction/reconciliation baseline in `internal/client/input.go`
- [ ] Add temp entity pipeline (`internal/client/tent.go` or equivalent)
- [ ] Render temp entity effects via `internal/renderer/particle.go`

### Acceptance Criteria

- [ ] `go test ./internal/input ./internal/client ./internal/renderer` passes
- [ ] Movement + mouselook are stable
- [ ] Explosion/impact effects appear correctly
- [ ] No severe jitter/rubber-banding in loopback SP

---

## Issue 7 — M6: Menu/UX Completion + Save/Load

**Title:** `M6: complete menu flows and implement save/load`

**Labels:** `milestone:m6`, `menu`, `savegame`, `high-priority`

**Body:**

### Objective

Deliver full user-facing gameplay loop without console-only workarounds.

### Tasks

- [ ] Expand menu system (`internal/menu/*`):
  - [ ] main menu
  - [ ] options menu
  - [ ] keys menu
  - [ ] load/save menus
  - [ ] quit confirm flow
- [ ] Complete missing host commands in `internal/host/commands.go`:
  - [ ] `save`
  - [ ] `load`
  - [ ] `connect`
  - [ ] `reconnect`
- [ ] Implement savegame serialization in `internal/savegame/save.go`:
  - [ ] entities
  - [ ] qc globals
  - [ ] world/client state
- [ ] Ensure draw/menu assets load from mounted FS, not test-only assumptions

### Acceptance Criteria

- [ ] `go test ./internal/menu ./internal/host ./internal/savegame` passes
- [ ] Can start new game from menu
- [ ] Can save, quit, relaunch, load successfully
- [ ] Key rebinding works and persists

---

## Issue 8 — M7: Audio + Stability Hardening

**Title:** `M7: finalize positional audio and long-session stability`

**Labels:** `milestone:m7`, `audio`, `stability`, `release-blocker`

**Body:**

### Objective

Reach reliable “fully playable” quality for real sessions.

### Tasks

- [ ] Complete listener/channel behavior in `internal/audio/*`
- [ ] Finish attenuation/spatialization updates per frame
- [ ] Ensure clean subsystem shutdown in `cmd/ironwailgo/main.go`
- [ ] Tighten frame pacing/update order in `internal/host/frame.go`
- [ ] Long-session crash/alloc tracking

### Acceptance Criteria

- [ ] `go test ./internal/audio ./internal/host` passes
- [ ] Positional audio behaves correctly
- [ ] 30-minute session without crash
- [ ] Clean shutdown without leaked goroutines/resources

---

## Issue 9 — Release Gate: Fully Playable Definition of Done

**Title:** `Release Gate: fully playable Quake parity baseline`

**Labels:** `release`, `qa`, `blocking`

**Body:**

### Objective

Declare project “fully playable baseline” only when all hard gates pass.

### Hard Gates

- [ ] `go build -tags=gogpu ./cmd/ironwailgo` succeeds
- [ ] Original `id1/pak0.pak` mounted and `maps/start.bsp` loaded
- [ ] Full menu flow works (new game/options/keys/save/load/quit)
- [ ] Player can move, fight, transition maps
- [ ] HUD + temp entities visible
- [ ] Positional audio working
- [ ] 30+ minute play session stable

---

If you want, I can next provide this as **exact `gh issue create` command blocks** (copy-paste runnable), including milestone assignment and labels.

---
