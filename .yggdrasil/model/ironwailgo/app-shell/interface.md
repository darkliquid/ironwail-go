# Interface

## Main consumers

- the operating system / launcher invoking the binary.
- tests that drive the command package as a whole.

## Main surface

- `main()`
- `Game`
- startup logging setup via `-loglvl`
- version constants and top-level runtime helpers referenced across the package

## Contracts

- `Game` is the process-wide mutable shell tying together host, client, server, renderer, UI, caches, and runtime state.
- `-loglvl` accepts either a single global level (`DEBUG`) or a comma-separated mix of baseline plus subsystem overrides (`INFO,renderer=WARN,input=DEBUG`).
- `Game.Renderer` is typed as a command-layer interface (`gameRenderer`) instead of a concrete `*renderer.Renderer`, while preserving the existing renderer method surface used by runtime/update/draw paths.
- `gameRenderer` is composed from role interfaces (`frame loop`, `assets`, `world`, `lights`, `input`) to support incremental seam-by-seam decoupling.
- Runtime frame-state sync now forwards CSQC extglobals (`cltime` realtime source, intermission time, local player ids, and command frame tracking) from host/client state.
- CSQC draw bridge now mirrors C DrawQC pic-cache semantics: NOLOAD cache-query, BLOCK-sensitive precache failure, and shared AUTO cache path for draw/getsize.
- Runtime overlay draw falls back to native `g.HUD.Draw` when `CSQC_DrawHud` invocation fails in-frame, so HUD visibility is preserved instead of being fully suppressed by a loaded-but-failing CSQC program.
- Overlay/console gating now treats `client.State == active` as authoritative gameplay readiness even if `Signon` counters lag during demo/live transitions, preventing forced-console suppression of HUD in runtime draws.
- `main_test.go` asserts cross-cutting orchestration and policy behavior across many child areas.
