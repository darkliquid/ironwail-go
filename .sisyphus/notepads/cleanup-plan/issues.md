## Tooling
- `lsp_diagnostics` currently reports `No active builds contain ...` for edited files in this workspace, so compile/test verification relied on `go test ./internal/qc` instead.

## Cleanup Task Issues
- Encountered a build failure due to duplicate case constants in `internal/input/types.go`. This was caused by incorrect use of `const` blocks in Go.
- Encountered a CGO-related build error in a dependency (`goffi`). Resolved by disabling CGO.
