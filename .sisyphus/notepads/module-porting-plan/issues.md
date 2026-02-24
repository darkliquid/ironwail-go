
- `lsp_diagnostics` returned workspace warning (`No active builds contain ...`) for changed BSP files, so verification relied on `go test ./internal/bsp` in this environment.
- `lsp_diagnostics` reported the same workspace-level `go list` warning for `internal/model/*.go`; used `go test ./internal/model` as the authoritative verification step.
- `lsp_diagnostics` returned the same workspace-level `No active builds contain ...` warning for `internal/server/{sv_main.go,world.go,server_test.go}`; `go test ./internal/server` was used as authoritative verification.
- `lsp_diagnostics` continued returning the workspace-level `No active builds contain ...` warning for `internal/server/physics*.go`; `go test ./internal/server` remained the authoritative verification step.
- `lsp_diagnostics` with `severity=all` reported the same workspace-level `No active builds contain ...` warning for `internal/server/movement*.go`; `severity=error` returned clean diagnostics and `go test ./internal/server` passed.
- `lsp_diagnostics` with `severity=all` reported the same workspace-level `No active builds contain ...` warning for `internal/server/user*.go`; `severity=error` returned clean diagnostics and `go test ./internal/server` passed.
- Encountered syntax errors during porting due to manual line number tracking; resolved by re-reading files and using updated line IDs.
- Some commands (like god, noclip) require direct access to player entities, which is currently limited by the subsystem interfaces.
