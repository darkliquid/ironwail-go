# Implementation Plans — Index & Status

Live plans live at `docs/plans/*.md`; completed/obsolete plans are archived at
`docs/plans/archive/*.md` (git history preserves detail — each archived plan
keeps its own `Status` line and commit references).

## Live plans (in recommended execution order)

| # | Plan | Status | Depends on |
| --- | --- | --- | --- |
| 26 | `26_docs_consolidation.md` — doc consolidation & stale purge | COMPLETED (2026-08-08) | none |
| 23 | `23_parity_hardening.md` — behavior divergences C vs Go | IN PROGRESS (D1-D4,D6,D8 closed; D5 improved: index-255 fullbright + engine-capture default, residual ~0.90 median ratio parked) | 26 |
| 24 | `24_parity_harness_expansion.md` — six deterministic parity gates | IN PROGRESS (H2,H4,H5 landed; H1,H3,H6 opt-in remain) | 23 |
| 25 | `25_qcvm_test_simulator.md` — standalone QuakeGo/QCVM dev kit | IN PROGRESS (A/B/C landed: sim, vm runner, debugger core; D REPL remains) | — |
| 27 | `27_hotpath_optimization.md` — post-parity hot-path opt | IN PROGRESS (O1+O5 landed; O3,O4,O6 remain) | 23 |
| 22 | `22_browser_engine_walkthrough.md` — interactive wasm dev journey | IN PROGRESS (Phase A landed: in-memory progs fallback + synthetic map + auto-start; client-handshake blocked on qgo closure sentinel (plan 25 boundary)) | 24, 25 |
| 28 | `28_qgo_compiler_function_values.md` — qgo function-value wiring (the closure sentinel) | PLANNED (fixes the unresolved `OP_CALL` function-index cells; unblocks plan 22) | 22, 25 |

## Archived plans (superseded/completed — see archive/)

| Plan | Outcome |
| --- | --- |
| 01 zero-sync QCVM | Completed — merged `570e806` |
| 02 texture atlas storage buffer | Completed |
| 03 arena allocators | Completed — `ab41b31` |
| 04 CSQC integration | Completed (deferred runtime wiring) |
| 05 qbj3 parity signoff | Completed (memory leak + exit deadlock fixed) |
| 06 engine modularisation (+ 06_phase2 server split) | Completed |
| 07 browser wasm port | Completed — `f326a7d`…`ce2e4e4` |
| 08 qbj2 zetabyt hang | Completed — `f865138`, `ea76301` |
| 09 renderer GPU alias animation (+ wave2 diagnostic) | Completed — `24a34f3`, `7b4db8e` |
| 10 zero-sync migration completion | Completed — `da40ae2` |
| 11/12 EntVars removal + follow-up | Completed — `47df06d` |
| 13 fix remaining test failures | Completed |
| 14 expanded parity harness | DONE (2026-08-05); superseded by live plan 24 |
| 15 map profiling/performance audit | Completed — `ac3ea90` |
| 16/16b package modularisation review | Approved/Planned → superseded by live code state (subpackages landed) |
| 17 subpackage test isolation | Completed |
| 18 shim removal + docs | Completed |
| 19/19b VFS modernisation / overlay FS | Completed — `dee46b1`+ |
| 20 zero-alloc frames | Completed |
| 21 viewpoint_json console command | Completed |
| qbj2 zetabyt deep-investigation / diagnostic | Investigation logs — kept in archive |

## Plan lifecycle rule

Every `docs/plans/*.md` must carry a `**Status**:` line ∈ {PLANNED, DONE,
COMPLETED, SUPERSEDED, APPROVED}. When a plan is completed or superseded, move
it to `docs/plans/archive/` and add a one-line outcome here.
