# Interface

## Current interface status

There is no implemented public API in `internal/game` yet.

Observed from `doc.go`:
- the intended future interface is a `Game` struct created via `New()`
- the executable entrypoint would wire renderer callbacks and call `Run()`

Because those symbols are not implemented in this package today, they are architectural intent rather than current contract.

## Current consumers

- No implemented inbound consumers beyond the package-level documentation itself.

## Future-facing contract notes

If this package is materialized later, it is expected to become the executable-facing aggregation point for subsystem references and frame orchestration. That contract is not active yet and should not be documented as implemented behavior.
