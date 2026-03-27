# Internals

## Logic

Bootstrap starts with parsed startup options, then initializes base input/UI state, filesystem, QC, networking/server, renderer, and the host subsystem bundle. One crucial invariant is that the authoritative server VM is created by `server.NewServer()` and then adopted as `g.QC`, rather than maintaining a parallel startup-owned server VM. Immediately after filesystem mount, startup derives Quake registration mode through `configureRegistrationMode`, which checks `gfx/pop.lmp` in mounted game data: when found, `registered=1` and startup logs registered mode; otherwise it sets `registered=0`, logs shareware mode, and blocks non-`id1` mods. Once the renderer exists, it may provide an input backend; SDL3 can optionally override or fill in missing backend behavior. Startup also wires menu save-slot/mod providers, runtime-sensitive menu policy callbacks (including Single Player -> New Game confirmation via `Host.ServerActive()`, resume-autosave availability via `os.Stat(filepath.Join(Host.UserDir(), "saves", "autosave", "start.sav"))`, and Single Player -> Save entry gating via `Host.SaveEntryAllowed(g.Subs)`), loopback client/server integration, gameplay bindings, archived startup cvars, CSQC bootstrap gating (`pr_checkextension`, `cl_nocsqc`), and C-parity CSQC startup load order (`csprogs.dat` then `progs.dat`, followed by optional `CSQC_Init` call). It also wires color-shift cvars (`gl_cshiftpercent` and per-channel `gl_cshiftpercent_*`) with Ironwail-parity defaults, renderer sky cvars (`r_fastsky=0`, `r_proceduralsky=0`, `r_skyfog=0.5`, `r_skysolidspeed=1`, `r_skyalphaspeed=1`) for sky parity defaults and narrow procedural-sky tuning, renderer dynamic-light and particle-mode cvars (`r_dynamic=1`, `r_particles=2`) for deterministic dynamic-light and explosion/temp-entity particle parity control, alias/world texture parity cvars (`r_nolerp_list`, `gl_texturemode=GL_NEAREST_MIPMAP_LINEAR`, `gl_lodbias=0`, `gl_texture_anisotropy=1`) consumed by alias/world draw paths, control cvars (`cl_alwaysrun`, `freelook`, `lookspring`, `cl_nolerp`, `v_centermove`, `v_centerspeed`) with client-sync callbacks, and gameplay controller-look cvars (`joy_look`, `joy_looksensitivity_*`, `joy_gyro_look`, `joy_gyro_*_scale`) so right-stick look can ship narrowly with gyro left behind an explicit toggle. Regression coverage in `game_init_test.go` keeps `gl_texturemode` pinned to `GL_NEAREST_MIPMAP_LINEAR` so startup defaults do not silently drift.
With app-shell renderer storage decoupled behind `gameRenderer`, startup now constructs `renderer.NewRendererAdapter` directly from the interface-typed game renderer; this keeps host subsystem wiring behavior unchanged while eliminating concrete renderer downcasting from bootstrap code.
Renderer backend construction is delegated to build-tag-specific factory files (`mkrenderer_gogpu.go`, `mkrenderer_opengl.go`, `mkrenderer_stub.go`) that all return the app-shell `gameRenderer` interface into the shared `initGameRenderer` path.
At host bootstrap, `host_maxfps` now has an explicit callback that calls `Host.SetMaxFPS`, and bootstrap applies the initial cvar value immediately so host frame/network policy is active before the first runtime frame.

## Constraints

- Startup sequencing matters because later subsystems depend on earlier outputs (e.g. filesystem before `progs.dat`, server before authoritative QC, renderer before renderer-backed input).
- Registration-state detection (`gfx/pop.lmp` check) must run after filesystem init and before QC scripts/gameplay logic that gate features on the `registered` cvar.
- Renderer creation can fail and may trigger a headless fallback path at the app-shell level.
- Backend-selection policy is platform-sensitive, especially around GoGPU/X11 and SDL3 overrides.

## Decisions

### Make the server-owned QC VM authoritative during bootstrap

Observed decision:
- App bootstrap explicitly discards a parallel server-side QC ownership path and adopts `g.Server.QCVM` as the authoritative server VM.

Rationale:
- **unknown — inferred from code comments, not confirmed by a developer**

Observed effect:
- App startup follows the same QC ownership path as host/server tests and runtime behavior, reducing divergence between bootstrap and direct host/server execution.
