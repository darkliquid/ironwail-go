# Interface

## Main consumers

- all other `server/*` nodes, which depend on the shared server/edict/client model
- `host/*`, which creates and drives the authoritative server instance
- `qc/core`, which executes the QC program against this state model

## Main surface

- `Server` / `ServerStatic` / `Client` / `Edict` data model
- server construction and builtin wiring (`NewServer` and related setup)
- edict lookup/allocation/free helpers
- QC profiling bridge (`QCProfileResults(top)`) that returns VM profile snapshots for host console commands
- dev-stats bridge (`DevStatsSnapshot`) that surfaces current/peak server-side developer counters (including monotonic frame count, packet size, and edict population) to host commands
- narrow edict-capacity bridge (`DevStatsEdictCounters`) that returns the active dev-stats edict count plus configured server max-edicts capacity for focused diagnostics
- Go↔QC synchronization helpers for globals and edicts
- builtin write-buffer routing for `MSG_ALL`, QC `checkbottom` bridging, and default server-rule cvar registration used by isolated server tests
- QC builtin bridge hook `IssueChangeLevel(level)` now marks `ServerStatic.ChangeLevelIssued` before queueing command-buffer `changelevel <map>`, so duplicate QC trigger firings in the same transition window do not queue repeated map restarts.

## Contracts

- Edict numbering and reserved client slots are semantic and must remain stable.
- QC-visible state must be synchronized explicitly at execution boundaries.
- Bridge helpers that consult Go collision or networking state (`checkbottom`, write destinations, saved globals) must sync QC mutations into authoritative Go entities first.
- QC builtin registration assumes the loaded VM layout matches the server-side field/global expectations.
- `SetCompatRNG(rng)` is the authoritative RNG provenance bridge for server/QC runtime: it stores the server RNG stream and forwards the same pointer to `QCVM.SetCompatRNG`, so server movement random branches and QC `random()` consume a single ordered stream.
