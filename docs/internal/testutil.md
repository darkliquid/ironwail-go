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
