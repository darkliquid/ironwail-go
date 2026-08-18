# ADR-0005: Dependency Bump — gogpu v0.52.1 → v0.53.0 + gg v0.52.3 (Branch-Confined)

**Status:** Accepted (2026-08-18)
**Deciders:** darkliquid (Stage 1: yes), lifecycle driver
**Date:** 2026-08-18
**Related:** IRONWAIL-SPEC-001 §1.3, §7 (edge: bump contained); research
0002 §5; gap log row 11

## Context and Problem Statement

gogpu/ui v0.1.54's go.mod requires `gogpu v0.53.0` (ironwail-go pins
`v0.52.1`) and depends on `gg v0.52.3`, which is not in the engine's go.sum
at all. The ui widget pipeline (widget.Canvas drawn by gg; `render.NewCanvas`
over a gg context) cannot run without both. Applying the bump touches the
engine's core renderer dependency — normally a main-branch-level decision.

## Decision Drivers

- gogpu/ui is a hard requirement for the experiment.
- gg is pure Go, `CGO_ENABLED=0` — satisfies AGENTS.md gotcha #4.
- The engine's existing gogpu integration is version-sensitive
  (naga/wgpu quirks historically; `internal/renderer` uses gogpu API).
- The experiment must be rollback-able: main stays on v0.52.1.

## Considered Options

1. **Stay on gogpu v0.52.1 and vendor a patched ui** — rejected: ui v0.1.54
   builds against v0.53.0's gpucontext/window APIs; downpinning risks
   API-mismatch churn (eventSourceAdapter, WindowProvider et al); vendored
   fork of ui is a maintenance trap.
2. **Bump gogpu to v0.53.0 + add gg v0.52.3 on the experiment branch** —
   chosen. `go.mod`/`go.sum` changes confined to
   `experiment/ui-rewrite`; if the experiment is abandoned, the branch is
   discarded (git history preserves main).
3. **Bump on main first** — rejected: violates the experiment's isolation;
   main should not bear the risk until the experiment proves out.

## Decision Outcome

On `experiment/ui-rewrite` only: `go get github.com/gogpu/gogpu@v0.53.0`
and `go get github.com/gogpu/gg@v0.52.3`, then
`github.com/gogpu/ui@v0.1.54`. A verification task runs the full
`mise run verify` + renderer test package on the new dependency set to catch
API drift in `internal/renderer` (gogpu v0.53.0 CHANGELOG items: compositor-
owned render target ADR-067, pluggable DebugOverlay ADR-066, wgpu v0.31.2+
damage rects). No other dependency changes on this branch.

- **Positive:** ui v0.1.54 supported exactly; single dep delta; rollback =
  discard branch or `git checkout main -- go.mod go.sum`.
- **Negative:** bump risk lands on the experiment branch (renderer must
  still pass tests on v0.53.0); gg adds runtime weight (GPU SDF) even when
  untraced — dead-code elimination keeps it out of non-ui builds, but the
  binary grows with path 1 compiled in.

## Links

- research 0002 §5; IRONWAIL-SPEC-001 §1.3, §7, AC9; gap log 11

## Review Log

### Stage 5 — Review 2 (2026-08-18)

Verdict: APPROVED. Options realistic (vendored ui fork rejected with honest
maintenance-trap reason; main-branch bump rejected for isolation). Bump
confined to experiment branch tied to AC9/rollback = `git checkout main --
go.mod go.sum`. gg pure-Go claim verified (CGO_ENABLED=0). No changes.
