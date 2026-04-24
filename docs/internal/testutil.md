# Package `testutil`

## Purpose
The `testutil` package provides a suite of helpers to facilitate testing of engine subsystems. It focuses on reducing boilerplate in tests that require Quake game data (like PAK files) and provides common assertion utilities to keep the test suite readable and maintainable.

## Key Types & Interfaces
This package consists primarily of utility functions rather than complex types.
- **Asset Location Functions**: `LocateQuakeDir`, `LocatePak0`.
- **Test Lifecycle Helpers**: `SkipIfNoQuakeDir`, `SkipIfNoPak0`.
- **Assertion Helpers**: `CompareStructs`, `AssertNoError`.

## Core Workflow
1. **Setup**: A test requiring game assets calls `SkipIfNoPak0(t)`.
2. **Discovery**: The utility checks environment variables (like `QUAKE_DIR` or `QUAKE_PAK0_PATH`) and searches common relative paths to find the necessary files.
3. **Execution**: If the assets are found, the test continues with the path to the assets; otherwise, the test is skipped with a descriptive message.
4. **Validation**: Tests use `CompareStructs` or `AssertNoError` to perform standard checks with high-quality failure output (e.g., hex dumps for differing byte slices).

## Integration
- **Internal Tests**: Used across various internal packages (e.g., `bsp`, `image`, `model`, `server`) in their respective `_test.go` files to ensure they can run in environments both with and without the original Quake data files.

## Learning Tips
- **Asset Discovery**: Look at `internal/testutil/assets.go` to see the logic used to find game files across different development environments.
- **Concise Testing**: Use this package as a template for how to write "skip-first" tests that gracefully handle missing external dependencies.

## Tests

**`TestLocatePak0`** — Calls `LocatePak0()` and, if a path is returned, confirms the file actually exists on disk. Does not fail if pak0 is absent. Self-tests the pak0 discovery helper that many integration tests depend on. Uses `os.Stat` on the returned path and logs the outcome.

**`TestCompareStructs`** — Calls `CompareStructs(t, 1, 1)` and `CompareStructs(t, []byte{1,2,3}, []byte{1,2,3})` and expects no failure. Self-tests the assertion helper used throughout the suite to ensure it doesn't trigger false positives. Passes equal values and relies on `t` not being marked failed.

**`TestAssertNoError`** — Calls `AssertNoError(t, nil)` and expects no failure. Self-tests the `AssertNoError` helper to ensure it doesn't panic on a nil error.

**`TestSkipIfNoPak0`** — Calls `SkipIfNoPak0(t)` and, if a path is returned, confirms it exists. Ensures the skip helper returns a valid path when pak0 is present, and doesn't panic when it's absent. The test itself may be skipped if pak0 is missing.

**`TestProjectFilesUnderLineCeiling`** — Walks the entire repo and fails if any `.go` file (excluding generated, vendored, `.git`, `.tmp`) exceeds 1000 lines, unless it is in the `knownOversizedFiles` allowlist. Also fails if an allowlisted file drops at or below 1000 (indicating the debt has been paid and the allowlist is stale). Enforces the project convention that Go files should be split at 1000 lines to keep code navigable and reviews tractable. Uses `filepath.WalkDir` with `bufio.Scanner` line-counting; skips files where the first 5 lines contain "DO NOT EDIT" or "Code generated".
