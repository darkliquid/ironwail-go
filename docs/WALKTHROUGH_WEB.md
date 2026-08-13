# Web Walkthrough — Layer Tour (offline reading companion)

Served live at `web/walkthrough/` by `web/server.go`. This markdown mirrors the
tour for offline reading and doubles as the C↔Go legend the parity harnesses
cite.

## Layers (left rail, in tour order)

| # | Layer | Primary Go files | What it does | C reference |
| --- | --- | --- | --- | --- |
| 1 | Boot / Filesystem | `cmd/ironwailgo/main.go`, `internal/fs` | VFS mount order, pak search, startup flags | `sys_*.c`, `files.c` |
| 2 | Console | `internal/host/commands.go`, `internal/cmdsys` | Command exec log, aliases, cvars | `cmd.c`, `con_main.c` |
| 3 | Host frame | `internal/game/game_loop.go:492`, `internal/host` | RunRuntimeFrame ordering, dt/srvTime | `host.c` |
| 4 | Server physics | `internal/server/physics/stepframe.go`, `leafs.go` | Movetype dispatch, pusher/walk/toss, phase bars | `sv_phys.c` |
| 5 | QuakeC VM | `internal/qc/vm.go`, `exec.go`, `vm_edict.go`, `pkg/qgo/quakego` | Execution + field accessors, trace lines | `pr_exec.c`, `pr_cmds.c` |
| 6 | Client | `internal/client/parse_clientdata.go`, `RelinkEntities`, `PredictPlayers` | svc_* parse, lerp/pred state | `cl_parse.c`, `cl_main.c` |
| 7 | Renderer | `internal/renderer/renderer_gogpu_frame.go`, `render_pass_parity.go` | 5-phase pipeline, pass counters | `gl_rmain.c`, `r_brush.c` |

## Inspector API (window.ironwailInspector)

Built by `cmd/ironwailgo/inspector_wasm.go` (Phase B). Read-only; the walkthrough
panel consumes it each animation frame.

- `getLayers()` → layer ids
- `getState(layer)` → JSON snapshot (console lines, host timing, server edict
  summary, client state, camera)
- `getTimeline()` → frameCount / srvTime / frameTimeMs
- `getEdict(n)` → typed field dump for one edict
- `getSourceAnchor(layer)` → `{file, line, doc}` from `web/walkthrough/anchors.json`

## Try this (per-layer experiments)

1. **Boot/FS**: check `gamedir` and map state (`server.active`, `map` = "start" when testing pak0 is loaded).
2. **Console**: open the panel and run `sv_debug_qc_trace 1` from the console;
   the dline shows QC enter/leave.
3. **Host frame**: watch `frameCount` advance one per click while paused=false;
   flip paused to freeze the timeline.
4. **Server physics**: read the edict table; find the player (classname
   "player") and watch its origin move as the main loop runs.
5. **QuakeC**: the panel shows live QC globals (time/self/world/mapname), a
   bounded ring of recent function enter/leave/builtin events, and per-function
   call counts (sourced from `internal/server/qc_trace_record.go`, filled for
   every VM call). Set a monster's `nextthink` by hand (via the wasm debug
   bridge in a future phase) and watch the pusher fire once — the D1 parity
   fix.
6. **Client**: confirm `state` = active and `entities` > 0 after signon.
7. **Renderer**: confirm `cameraOrigin` matches the player edict origin (within
   viewheight) and `worldTree` = "loaded".
