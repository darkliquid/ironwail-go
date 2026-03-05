- 2026-02-26: Client server-message parsing now handles `svc_clientdata`, baseline spawn messages (`svc_spawnbaseline` / `svc_spawnbaseline2`), high-bit entity delta updates, and temp entities.
- 2026-02-26: For Quake fast entity updates, command bytes with bit 7 set must be treated as entity updates (not regular svc commands), then extended bitfields consumed via `U_MOREBITS`/`U_EXTEND*`.
- 2026-02-26: Signon flow transitions cleanly by setting state to `active` at signon 4 while keeping `connected` for post-serverinfo / pre-active phase.
## Deterministic Startup Logs
- Moved 'FS mounted', 'QC loaded', and 'menu active' logs to 'main' after successful initialization.
- This ensures they are printed exactly once, even if initialization falls back to headless mode.
- 'frame loop started' is logged consistently in both GUI and headless loops.
- 'map spawn started/finished' are logged when a map is loaded via command line.

## Robust Asset Location
- Updated 'internal/testutil/assets.go' to check for 'id1' in current directory and its parents.
- Added a check for when the path itself is 'id1'.
- Tests now skip gracefully if assets are not found, or pass if they are.

## Smoke Test Matrix
- Updated 'Makefile' with 'smoke-menu', 'smoke-headless', and 'smoke-map-start' targets.
- These targets verify that all expected log markers are printed.
- Added 'smoke-all' to run all smoke tests.
- Configurable 'QUAKE_DIR' and 'TIMEOUT' in 'Makefile'.
- 2026-02-28: `internal/fs` now routes loose-file reads through `io/fs` (`os.DirFS` + `fs.ReadFile`/`fs.Stat`), preserving override precedence by scanning loose search paths in reverse add order, then packs in reverse (`pak1` > `pak0`).
- 2026-02-28: Path traversal hardening is enforced at lookup time with `filepath.Clean` normalization and explicit `../` rejection after slash normalization (including Windows-style `..\\` inputs).
- 2026-02-28: Local singleplayer signon can be driven without UDP by wiring host map startup to a loopback client handshake sequence (`serverinfo` -> `prespawn` -> `spawn` -> `begin`) and synchronizing host signon/client state from that sequence.
- 2026-02-28: Logging explicit client state transitions (`Connected`, `Active`) from the client state machine provides deterministic headless verification for startup smoke checks.
