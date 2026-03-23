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
- Color-shift intensity cvars are registered during bootstrap with C Ironwail parity defaults: `gl_cshiftpercent` plus per-channel `gl_cshiftpercent_contents`, `gl_cshiftpercent_damage`, `gl_cshiftpercent_bonus`, and `gl_cshiftpercent_powerup` all default to `100`.
