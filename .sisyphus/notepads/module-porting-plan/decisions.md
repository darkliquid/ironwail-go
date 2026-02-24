
## Test Utilities
- Created `internal/testutil` to centralize asset discovery and comparison logic.
- Decided to use `reflect.DeepEqual` for general comparison but added specialized hex dump output for byte slices to aid in debugging binary format issues.
- Chose to return the path from `SkipIfNoPak0` to allow tests to use it directly if they don't skip.
