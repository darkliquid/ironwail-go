# Internals

## Logic

This node centralizes the dual-representation problem: every authoritative entity exists both as typed Go state and as flat QC VM memory. The sync helpers copy data between those representations at key boundaries so QC logic, physics, and networking all observe consistent values. It also caches field offsets and protocol-related knobs used throughout the rest of the package.

The server also exposes narrow bridge helpers that project QC VM internals to host policies without leaking VM ownership; `QCProfileResults(top)` is one such bridge and only returns counters when the server and QC VM are active. The same bridge layer now includes a `DevStatsSnapshot()` surface backed by `devStats/devPeak` state on `Server`, keeping runtime developer counters available to host command code without exposing serialization/physics internals directly. That snapshot now includes a monotonic frame counter advanced from `Frame()` so command-side diagnostics can correlate per-frame server activity with other per-frame counters. A narrower `DevStatsEdictCounters()` helper also returns the current active-edict dev counter together with `MaxEdicts`, so command/runtime callers can consume just the active/max slice without pulling unrelated counters.

The QC builtin hook table now includes `IssueChangeLevel(level)` as a server-owned transition seam. Its implementation sets `ServerStatic.ChangeLevelIssued` before enqueueing `changelevel <map>` in `cmdsys`, which closes the duplicate-transition window where repeated trigger touches could otherwise spam reconnect/map spawn loops before host command processing completes.

The `Server` filesystem surface is intentionally narrowed to the `modelAssetFileSystem` contract (`OpenFile` only) for model-bound operations. This preserves bridge-layer decoupling: server bootstrap/model cache logic can stream model assets without depending on broader VFS convenience APIs, while still accepting the concrete `*fs.FileSystem` injected by spawn/bootstrap callers.

Node-owned integration tests in `server_test.go` now also pin cross-node command-bridge behavior at the public `ExecuteClientString` surface for `ban`: non-deathmatch execution mutates/query-reads the shared datagram IP-ban state via `internal/net`, while deathmatch mode remains a no-op.

## Constraints

- `EntVars` layout and QC field offsets are parity-critical.
- Freed edicts must clear QC-visible model/state surfaces so stale entities do not leak into later frames.
- QC execution context (`self`, `other`, time globals, spawned edict sync) must be preserved across builtin and callback paths.
- Server-level collaborators that introduce nondeterminism (such as network accept polling) should be injectable so protocol/session parity behaviors remain unit-testable.

## Decisions

### Typed Go wrappers around QC-backed server state

Observed decision:
- The Go port models server state with ordinary structs while keeping QC VM synchronization explicit.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Server-side gameplay code is easier to read and test than raw pointer arithmetic, but still remains constrained by the QC memory layout contract.

### Provide QC profile snapshots through a narrow bridge

Observed decision:
- The server exposes VM profile counters to host policies through `QCProfileResults(top)` rather than exposing the VM directly.

Rationale:
- This keeps VM ownership and mutation boundaries inside server/QC nodes while still supporting the parity `profile` command contract.

Observed effect:
- QC profiling is an implemented, bounded bridge feature and does not require extra telemetry-specific plumbing.
