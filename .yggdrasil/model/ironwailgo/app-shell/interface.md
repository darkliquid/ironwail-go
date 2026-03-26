# Interface

## Main consumers

- the operating system / launcher invoking the binary.
- tests that drive the command package as a whole.

## Main surface

- `main()`
- `Game`
- version constants and top-level runtime helpers referenced across the package

## Contracts

- `Game` is the process-wide mutable shell tying together host, client, server, renderer, UI, caches, and runtime state.
- Runtime frame-state sync now forwards CSQC extglobals (`cltime` realtime source, intermission time, local player ids, and command frame tracking) from host/client state.
- CSQC draw bridge now mirrors C DrawQC pic-cache semantics: NOLOAD cache-query, BLOCK-sensitive precache failure, and shared AUTO cache path for draw/getsize.
- Runtime overlay draw falls back to native `g.HUD.Draw` when `CSQC_DrawHud` invocation fails in-frame, so HUD visibility is preserved instead of being fully suppressed by a loaded-but-failing CSQC program.
- `main_test.go` asserts cross-cutting orchestration and policy behavior across many child areas.
