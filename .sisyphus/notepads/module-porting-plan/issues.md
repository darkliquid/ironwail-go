
- `lsp_diagnostics` returned workspace warning (`No active builds contain ...`) for changed BSP files, so verification relied on `go test ./internal/bsp` in this environment.
- `lsp_diagnostics` reported the same workspace-level `go list` warning for `internal/model/*.go`; used `go test ./internal/model` as the authoritative verification step.
