# Internals

## Logic

Bootstrap starts with parsed startup options, then initializes base input/UI state, filesystem, QC, networking/server, renderer, and the host subsystem bundle. One crucial invariant is that the authoritative server VM is created by `server.NewServer()` and then adopted as `g.QC`, rather than maintaining a parallel startup-owned server VM. Immediately after filesystem mount, startup derives Quake registration mode through `configureRegistrationMode`, which checks `gfx/pop.lmp` in mounted game data: when found, `registered=1` and startup logs registered mode; otherwise it sets `registered=0`, logs shareware mode, and blocks non-`id1` mods. Once the renderer exists, it may provide the concrete input backend used by runtime input state. Startup also wires menu save-slot/mod providers, runtime-sensitive menu policy callbacks (including Single Player -> New Game confirmation via `Host.ServerActive()`, resume-autosave availability via `os.Stat(filepath.Join(Host.UserDir(), "saves", "autosave", "start.sav"))`, and Single Player -> Save entry gating via `Host.SaveEntryAllowed(g.Subs)`), loopback client/server integration, gameplay bindings, archived startup cvars, CSQC bootstrap gating (`pr_checkextension`, `cl_nocsqc`), and C-parity CSQC startup load order (`csprogs.dat` then `progs.dat`, followed by optional `CSQC_Init` call). It also wires color-shift cvars (`gl_cshiftpercent` and per-channel `gl_cshiftpercent_*`) with Ironwail-parity defaults, renderer sky cvars (`r_fastsky=0`, `r_proceduralsky=0`, `r_skyfog=0.5`, `r_skysolidspeed=1`, `r_skyalphaspeed=1`) for sky parity defaults and narrow procedural-sky tuning, renderer dynamic-light and particle-mode cvars (`r_dynamic=1`, `r_particles=2`) for deterministic dynamic-light and explosion/temp-entity particle parity control, alias/world texture parity cvars (`r_nolerp_list`, `gl_texturemode=GL_NEAREST_MIPMAP_LINEAR`, `gl_lodbias=0`, `gl_texture_anisotropy=1`) consumed by alias/world draw paths, control cvars (`cl_alwaysrun`, `freelook`, `lookspring`, `cl_nolerp`, `v_centermove`, `v_centerspeed`) with client-sync callbacks, and gameplay controller-look cvars (`joy_look`, `joy_looksensitivity_*`, `joy_gyro_look`, `joy_gyro_*_scale`) so right-stick look can ship narrowly with gyro left behind an explicit toggle.

Startup now installs a host game-dir-changed callback that runs executable-owned reload logic after host command `game` swaps the active VFS. That reload path intentionally preserves the live renderer/input stack while resetting mod-scoped runtime state: it tears down active sessions, shuts down the existing server instance in place (preserving object identity and server-owned QC VM wiring for later host map/bootstrap paths), unloads CSQC, resets visual/audio/model caches, refreshes draw/HUD assets from the new mod, and forces menu state back to main. The callback intentionally does not create a replacement `server.Server` during the `game` command callback; the next `map` path re-initializes and spawns through normal `Host.CmdMap`/`Server.Init`/`Server.SpawnServer` flow.
To keep this safe with GoGPU's split update/draw threads, the callback now runs under the app-shell runtime-state lock and no longer pushes palette/conchars/world-clear operations directly to the renderer from the update thread; instead it stages those mutations for application on the render thread during `OnDraw`.
The menu mods provider was also shifted from a startup-closure over `fileSys` to a runtime lookup over `g.Subs.Files`, so menu refreshes enumerate mods from the active filesystem after mod switches.
Late startup draw-manager initialization now follows the same rule: it resolves the active filesystem from `g.Subs.Files` instead of reusing the startup-captured `fileSys` variable, so startup command sequences like `+game hipnotic +map start` do not try to load menu assets from a stale VFS that the `game` command already replaced and closed.

Regression coverage in `game_init_test.go` keeps `gl_texturemode` pinned to `GL_NEAREST_MIPMAP_LINEAR` so startup defaults do not silently drift, and now also covers runtime mod-provider filesystem selection plus game-dir reload behavior (fresh main menu and renderer preservation). The registration-mode tests in that file intentionally run serially because they mutate the global `registered` cvar.
With app-shell renderer storage decoupled behind `gameRenderer`, startup now constructs `renderer.NewRendererAdapter` directly from the interface-typed game renderer; this keeps host subsystem wiring behavior unchanged while eliminating concrete renderer downcasting from bootstrap code.
Renderer backend construction is delegated to the canonical factory file (`mkrenderer_gogpu.go`), which returns the app-shell `gameRenderer` interface into the shared `initGameRenderer` path.
At host bootstrap, `host_maxfps` now has an explicit callback that calls `Host.SetMaxFPS`, and bootstrap applies the initial cvar value immediately so host frame/network policy is active before the first runtime frame.

## Constraints

- Startup sequencing matters because later subsystems depend on earlier outputs (e.g. filesystem before `progs.dat`, server before authoritative QC, renderer before renderer-backed input).
- Registration-state detection (`gfx/pop.lmp` check) must run after filesystem init and before QC scripts/gameplay logic that gate features on the `registered` cvar.
- Renderer creation can fail and may trigger a headless fallback path at the app-shell level.
- Backend-selection policy is platform-sensitive, especially around GoGPU/X11 keyboard behavior and renderer-provided event backends.

## Decisions

### Make the server-owned QC VM authoritative during bootstrap

Observed decision:
- App bootstrap explicitly discards a parallel server-side QC ownership path and adopts `g.Server.QCVM` as the authoritative server VM.

Rationale:
- **unknown — inferred from code comments, not confirmed by a developer**

Observed effect:
- App startup follows the same QC ownership path as host/server tests and runtime behavior, reducing divergence between bootstrap and direct host/server execution.
