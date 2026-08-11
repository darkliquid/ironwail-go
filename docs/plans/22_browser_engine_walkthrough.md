# Implementation Plan 22: Browser Engine Walkthrough — Interactive Dev Journey (`shadow`-style educational web app)

**Priority**: #1 (Educational / Developer Experience)
**Status**: IN PROGRESS (2026-08-08, updated 2026-08-11) — Phase A landed
fully: in-memory QuakeGo progs compile fallback
(`TestLoadRuntimeProgramsCompilesProgsWithNoAssets`), the synthetic box-room
map (`BuildSyntheticMap`, `TestSpawnServerFallsBackToSyntheticMap`), and a
no-assets headless boot that auto-starts `maps/synthetic` and reaches
`client active` on a playable world. The plan-28 qgo function-value sentinel
(responsible for the old `OPAddress pointer out of bounds` client-handshake
failure) was resolved by plan 28; the remaining `changelevel *0` reload loop
after first-frame `respawn()` was root-caused and fixed 2026-08-11 (plain
package vars like `NextMap` needed real global cells — see plan 28 §9). The
synthetic demo now stays running after boot.
**Boundary (next)**: Phase B (inspector bridge `inspector_wasm.go`) and
Phase C (web UI) remain — see the step-by-step below.
**Tag**: `wasm-walkthrough`
**Prerequisite**: stable baseline (all tests pass), wasm build green (`mise run build-wasm`)
**Estimated effort**: 3-6 focused sessions (phased, each phase independently shippable)

---

## 1. Executive Summary & Architectural Context

Ironwail Go is a pure-Go (`CGO_ENABLED=0`) Quake engine whose canonical
renderer is WebGPU, which makes it natively runnable in a browser
(`GOOS=js GOARCH=wasm`) — `docs/plans/07_browser_wasm_port.md` already landed
the wasm entry point, browser WebGPU surface, DOM input, Web Audio, and HTTP
VFS pieces. What is missing is a **developer-facing experience**: a website
that turns the running engine into an interactive, layer-by-layer walkthrough
"from base principles to full implementation."

The walkthrough is a **static web app** (`web/walkthrough/`) that boots the
engine wasm in the browser and exposes a small inspector bridge
(`cmd/ironwailgo/inspector_wasm.go`) so visitors can:

- flip through **7 engine layers** (Console → Host frame → Server physics →
  QuakeC VM → Client parse/prediction → Renderer → Boot/FS), each with its
  own overlay panel;
- **step the engine frame-by-frame** and inspect live state (edict table,
  QC call trace, message buffers, render pass counters, source anchors);
- follow a **guided transcript** that matches the existing repo docs
  (`docs/LEARNING_GUIDE.md`, `docs/WALKTHROUGH_*`, and the `article/` drafts
  surfaced by graphify) so the tour is an *interactive textbook* rather than a
  bare tool.

Constituent parts (all verified this session):

- wasm build: `main_wasm.go` (currently boots headless + `select {}`) — needs
  `headless=false` and a real `web/bin/ironwail.wasm`.
- Renderer: `renderer_gogpu_runtime.go` → `gogpu.NewApp(WithBackend(BackendGo))`
  → browser platform uses `requestAnimationFrame` and a `<canvas>` surface;
  `wgpu.Instance.CreateSurface(0,0)` resolves the first `<canvas>`.
- Telemetry already exists and is layer-shaped:
  `sv_debug_telemetry*`, `sv_debug_qc_trace*`, `sv_debug_push/trigger`
  (`internal/server/debug/*`), `host_speeds` phase bars
  (`internal/server/physics/stepframe.go`), renderer first-frame stats, and
  `internal/game/debug_view_telemetry.go` (cam/lerp/relink/pred logs).
- Input + audio wasm backends exist (`internal/input/wasm_input.go`,
  `internal/audio/wasm_audio.go`) but the wasm boot path never wires them.

---

## 2. Architectural Goals (what "good" looks like)

1. **No fork of engine behavior.** The walkthrough reads the SAME state the
   engine already surface: telemetry events, `host_speeds` phase timing,
   renderer pass counters, QC trace lines. The inspector adds a *read-side*
   bridge only.
2. **Layers map to real code.** Every panel links to the Go file + line that
   owns the layer (`getSourceAnchor(layer)`), and the guided transcript cites
   the same docs the repo already ships.
3. **Deterministic demo mode.** The walkthrough must run with **no Quake data**
   — a synthetic world (reuse `CreateSyntheticWorldModel` from
   `internal/server/synthetic_bsp_helper.go`) + minimal built-in progs, so a
   visitor never needs pak0/progs.dat. This is the gate for the whole app.
4. **Gameplay is optional, not required.** Layer panels must be meaningful
   even when the player is idle (spawn → a few frames → pause).

---

## 3. Step-by-Step Implementation Sequence

### Phase A — Foundation: no-assets demo mode (gate)

**Step 22.1: Drop-in synthetic map for wasm boot**
- **Files**: `cmd/ironwailgo/main_wasm.go`, `internal/game` (new tiny
  `demo_mode` hook), `internal/server/synthetic_bsp_helper.go` (already
  exists — expose a stable constructor surface if needed).
- **Actions**: boot `InitSubsystems(false /*headless*/, false, 4, "/", "id1", nil)`;
  when no `id1/pak0.pak` is mountable, load the synthetic world + a generated
  `progs.dat` (rebuilt from `pkg/qgo/quakego` via `cmd/qgo`, mirroring
  `EnsureRuntimeProgsData` in `internal/game/parity_test.go:212-218`), spawn
  `info_player_start`, run a few frames, pause. This gives the walkthrough a
  stable, asset-free scene to inspect.
- **Verify**: `GOOS=js GOARCH=wasm go build ./cmd/ironwailgo` and a manual
  browser smoke (WebGPU-capable browser; `navigator.gpu` present).

**Step 22.2: Verify the gogpu browser loop end-to-end**
- **Files**: `web/server.go` (+ `wasm_exec.js` copy), `web/walkthrough/index.html`.
- **Actions**: produce a minimal page that loads `wasm_exec.js` + the wasm,
  boots the engine, and prints `Ironwail-Go WASM initialized` + the demo map
  spawn in the console. Catch any place the engine blocks on GPU results
  (e.g. screenshot readback) and route it to the non-blocking path.
- **Verify**: browser console shows renderer stats + `PARITY_READY`-style
  readiness, no tab freeze, `host_speeds` phase bars in logs.

### Phase B — Inspector bridge (read-only)

**Step 22.3: `cmd/ironwailgo/inspector_wasm.go` (`//go:build js && wasm`)**
- **Files**: `cmd/ironwailgo/inspector_wasm.go` (new), `internal/game` hooks
  (minimal — expose already-existing read methods: `runtimeViewState`,
  camera state, telemetry ring buffer, `host_speeds` totals).
- **Actions**: export `window.ironwailInspector` with:
  - `getState(layer)` → JSON snapshot for the requested layer
    (console lines, frame/duration counters, edict table summary, QC trace
    ring, client parse log, render pass counters);
  - `stepFrame(n)` / `setPaused(bool)` / `fastForward(n)`: a FIFO queue of
    frame-control intents drained between gogpu RAF frames (never block the
    event loop; `select {}` in main_wasm is fine only because RAF keeps the
    scheduler alive);
  - `getTimeline()` → per-layer phase bars recycled from `host_speeds`
    (`stepframe.go` phase timers) + srvTime/frameTime;
  - `getEdict(n)` → typed field dump via the existing QCVM accessors
    (`internal/server/types/entity_accessors*.go`);
  - `getQCTrace()` / `getRenderPasses()` → reuse `sv_debug_qc_trace` event
    capture and `render_pass_parity.go` pass selectors;
  - `getSourceAnchor(layer)` → `{file, line, docRef}` from a static
    `layer→anchor` table (below).
- **Verify**: `go vet` green on wasm tag; browser smoke exercises every
  method; inspector JSON is stable across frames (no pointer garbage).

### Phase C — Walkthrough web UI

**Step 22.4: `web/walkthrough/` static app**
- **Files**: `web/walkthrough/index.html`, `walkthrough.css/js` (vanilla JS —
  no bundler, ships with `web/server.go` as-is; nothing to add to the build
  toolchain).
- **Actions**: seven layer panels toggled from a left rail:
  Boot/FS → Console → Host frame → Server physics → QuakeC → Client → Renderer;
  each panel = live inspector data + source anchor + "read this file, find X"
  pointer card. Step/play/fast-forward controls + a per-layer mute toggle
  (pause when a layer's data is not selected, to keep the scene stable).

**Step 22.5: Guided transcript (content)**
- **Files**: `web/walkthrough/transcript.js` (content), backed by a
  `docs/WALKTHROUGH_WEB.md` that mirrors the tour for offline reading.
- **Actions**: 7 chapters, each: (1) what the layer does, (2) the engine
  functions that run it (with `file:line`), (3) what to click/read in the
  panel, (4) the same C reference that parity harnesses cite (so the tour
  doubles as a C↔Go legend), (5) a "try this" experiment (e.g. set a
  `nextthink` by hand and watch the pusher fire once — the D1 fix from the
  parity work).

**Step 22.6: Layer→source anchor table (content, machine-readable)**
- **Files**: `web/walkthrough/anchors.json` + `getSourceAnchor` reads it.
- Anchors (validated this session):
  | layer | primary file(s) | notes |
  | --- | --- | --- |
  | boot/fs | `cmd/ironwailgo/main.go`, `internal/fs` | VFS mount order, pak search |
  | console | `internal/host/commands.go`, `internal/cmdsys` | command exec log, aliases |
  | host frame | `internal/host`, `internal/game/game_loop.go:492` | RunRuntimeFrame ordering, dt/srvTime |
  | server physics | `internal/server/physics/stepframe.go`, `leafs.go` | movetype dispatch, pusher/walk/toss, phase bars |
  | quakec | `internal/qc/vm.go`, `exec.go`, `vm_edict.go`, `pkg/qgo/quakego` | execution + field accessors, trace lines |
  | client | `internal/client` (SizeBuf parse, RelinkEntities, PredictPlayers) | svc_* log, lerp/pred state |
  | renderer | `internal/renderer/renderer_gogpu_frame.go`, `render_pass_parity.go` | 5-phase pipeline, pass counters |

### Phase D — Polish & parity tie-in

**Step 22.7: Recording/embed mode**
- A URL param (`#rec=demo1`) that replays tiny embedded NDJSON transcripts
  (frame dumps) instead of booting the engine — lets the tour run on any
  machine, even without WebGPU, and doubles as a regression viewer for the
  frame-state parity harness (links to H1/H3 in
  `docs/plans/24_*` — see plan 24).

---

## 4. Verification & Testing Strategy

1. **CI-safe (no assets, no browser)**: a `go test` on `inspector_wasm.go`
   build-tag boundary (`GOOS=js GOARCH=wasm go vet ./cmd/ironwailgo`) and a
   `build-wasm` step in `mise run verify`-style tasks.
2. **Manual browser checklist** (documented in `docs/WALKTHROUGH_WEB.md`):
   - load page on http://localhost:8080/walkthrough/ (secure context);
   - engine boots the synthetic map, renderer prints world stats, no freeze;
   - each layer panel renders data and its source anchor opens the right file;
   - step-frame shows one engine frame per click (srvTime advances by dt,
     pusher think fires exactly once — D1 assertion).
3. **Determinism**: two fresh page loads with the same synthetic seed produce
   identical `getTimeline()` phase ordering and edict table summaries.

## 5. Risks & Mitigations

| Risk | Mitigation |
| --- | --- |
| `navigator.gpu` unavailable / non-secure context | graceful error page + `#rec=` embedded-dump fallback (Step 22.7); feature-detect at boot |
| Engine blocks on GPU readback in browser loop | Step 22.2 smoke test; route screenshot/frame readback to the non-blocking path |
| wasm `main_wasm.go` heads-up: `select {}` + RAF dependency | explicit smoke: tab stays responsive, RAF fires; inspector queue never blocks |
| synthetic demo mode drifts from real gameplay | demo mode is a *bootstrap*; layer data is the same telemetry as real runs |
| page-weight: 22MB wasm + assets | embed synthetic progs as bytes, keep `#rec=` dumps tiny; document `wasm_exec.js` copy step in mise |
| doc/plan drift | Phase C content mirrors `docs/LEARNING_GUIDE.md` + `WALKTHROUGH_*`; regenerate graphify after changes |

## 6. Out of Scope (next plans)
- QCVM test-simulator (plan 25) — the walkthrough's QC layer will eventually
  *consume* the simulator's debug hooks once they land.
- Full parity harness upgrades (plan 24) — the `#rec=` embed mode is designed
  to plug into H3 render-record hashes later.
- Performance/profiling UI — plan 15's `map profiling` tooling can later feed
  the renderer layer panel.
