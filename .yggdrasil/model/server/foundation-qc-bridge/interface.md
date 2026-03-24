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
- dev-stats bridge (`DevStatsSnapshot`) that surfaces current/peak server-side developer counters to host commands
- Go↔QC synchronization helpers for globals and edicts

## Contracts

- Edict numbering and reserved client slots are semantic and must remain stable.
- QC-visible state must be synchronized explicitly at execution boundaries.
- QC builtin registration assumes the loaded VM layout matches the server-side field/global expectations.
