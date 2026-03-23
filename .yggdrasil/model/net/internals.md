# Internals

## Logic

The net package is a layered Quake-style networking subsystem. The transport/runtime layer exposes a single facade that dispatches between loopback and UDP datagram drivers, maintains socket state, and handles connection establishment and reliable/unreliable delivery semantics. `protocol.go` is the shared wire-contract catalog consumed throughout the engine. Discovery/support helpers provide LAN browser behavior, asynchronous polling, partial-address expansion, IP banning, and statistics without changing the core send/receive state machine.

## Constraints

- Runtime transport behavior is stateful and cross-file: `net.go`, `types.go`, `udp.go`, `datagram.go`, and `loopback.go` must be understood together.
- LAN discovery helpers are concurrent and partially independent of the transport facade, but still depend on shared packet constants and response formats.
- The package keeps several legacy/Quake-style conventions (sequence numbers, fixed timing, placeholder fallbacks, packet flags) that downstream code relies on implicitly.

## Decisions

### Keep transport behavior, protocol constants, and discovery helpers in one package but separate nodes

Observed decision:
- The Go port uses one `internal/net` package while still separating runtime transport logic, protocol definitions, and support/discovery behavior at the file level.

Rationale:
- **unknown — inferred from code and package docs, not confirmed by a developer**

Observed effect:
- Call sites get one cohesive networking package, but graph documentation needs child nodes so wire-contract definitions do not get buried inside transport state-machine details.
