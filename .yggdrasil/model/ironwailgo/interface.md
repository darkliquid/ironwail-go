# Interface

## Main consumers

- end users launching the executable.
- tests in `cmd/ironwailgo` that exercise runtime orchestration and policy decisions.

## Main surface

- `main()` and startup argument parsing
- the process-wide `Game` state bag
- internal runtime helpers for startup, loop execution, input, camera/view, entity collection, and shutdown

## Contracts

- This package is an executable composition root, not a reusable import API.
- Runtime behavior is driven by subsystem wiring, host callbacks, and per-frame orchestration rather than public library-style interfaces.
- Child nodes divide responsibilities that would otherwise be lost inside a very large `package main`.
