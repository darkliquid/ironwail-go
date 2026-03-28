# Responsibility

## Purpose

`ironwailgo/app-shell` owns the top-level executable shell: the process-wide `Game` state bag, main entrypoint, headless/dedicated/runtime mode handoff, and the broad cross-cutting tests that exercise command-package behavior.

## Owns

- `Game` and related top-level executable state in `main.go`.
- `main()` startup flow and mode selection.
- Process-wide startup logging configuration, including `-loglvl` parsing and default logger installation.
- Cross-cutting helper glue that does not fit cleanly into narrower child nodes.
- `main_test.go`, which currently acts as the command package's broad regression suite.

## Does not own

- Detailed subsystem bootstrap steps.
- Detailed frame-loop, input, camera, or entity algorithms, which belong to narrower children.
