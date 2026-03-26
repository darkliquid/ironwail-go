# Interface

## Main consumers

- the top-level app shell during executable startup.
- tests that verify startup flag parsing and bootstrap policies.

## Main surface

- startup option parsing helpers
- `initSubsystems` and bootstrap helpers such as `initGameQC`, `initGameServer`, `initGameRenderer`

## Contracts

- Startup builds the authoritative subsystem graph through `host.Subsystems`.
- The server-owned QC VM becomes the authoritative QC VM used by app startup.
- Renderer/input initialization follows explicit platform/build-tag policy rather than being left implicit.
- Control cvars that affect `client.Client` runtime behavior (including `cl_nolerp`, `v_centermove`, and `v_centerspeed`) are registered during bootstrap and synchronized into the active client state.
- Startup registers renderer sky parity cvars, including `r_fastsky`, `r_proceduralsky`, `r_skyfog`, `r_skysolidspeed`, and `r_skyalphaspeed`, before renderer/world paths run.
- Startup also registers console parity cvars consumed by console core/completion (`con_logcenterprint`, `con_maxcols`) alongside existing notify cvars.
- Startup registers `r_dynamic` (default `1`) so runtime visual helpers can deterministically gate dynamic-light spawn/contribution parity.
- Startup registers `r_particles` (default `2`) so temp-entity explosion effects use the C Ironwail parity particle mode when no user override exists.
- Startup also registers alias/world texture parity cvars (`r_nolerp_list`, `gl_texturemode`, `gl_lodbias`, `gl_texture_anisotropy`) with C-parity defaults before renderer world/entity draw paths consume them.
- Startup registers `pr_checkextension` and `cl_nocsqc`, then conditionally loads CSQC programs (`csprogs.dat` fallback `progs.dat`) and invokes `CSQC_Init` during bootstrap.
- Startup sets ROM-style gameplay registration mode through `configureRegistrationMode` and `registered` (`1` when `gfx/pop.lmp` exists in mounted game data, otherwise `0`) before QC/gameplay scripts run.
- Startup rejects non-`id1` mod startup when registration checks resolve to shareware mode, mirroring Quake's "registered data required for modified games" policy.
- Color-shift intensity cvars are registered during bootstrap with C Ironwail parity defaults: `gl_cshiftpercent` plus per-channel `gl_cshiftpercent_contents`, `gl_cshiftpercent_damage`, `gl_cshiftpercent_bonus`, and `gl_cshiftpercent_powerup` all default to `100`.
- Menu bootstrap wiring includes runtime policy callbacks; specifically, single-player New Game confirmation is gated by `Host.ServerActive()` through `SetNewGameConfirmationProvider`, resume availability is gated by presence of `UserDir()/saves/autosave/start.sav` through `SetResumeGameAvailableProvider`, and Save entry gating uses `Host.SaveEntryAllowed(g.Subs)`.
- Bootstrap registers gameplay controller-look cvars (`joy_look`, `joy_looksensitivity_yaw`, `joy_looksensitivity_pitch`) and optional gyro look toggles/scales (`joy_gyro_look`, `joy_gyro_yaw_scale`, `joy_gyro_pitch_scale`) used by runtime gameplay input.
