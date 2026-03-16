# Ironwail-Go Parity Analysis: Documentation Index

## Rules

1. **Source code wins** — when docs and source disagree, the source is correct.
2. **OpenGL is the parity target** — judge parity against the CGO/OpenGL runtime, not the gogpu path.
3. **gogpu is secondary** — treat `renderer_gogpu.go` as an experimental backend, not a parity gate.

---

## Current docs (use these)

### 1. `PORT_PARITY_REVIEW.md` — Canonical current baseline

Source-backed, line-referenced review of what is actually implemented across every subsystem.
Use this as the primary reference for current implementation status.

**Best for:** Implementation work, sprint planning, status verification, README updates.

---

### 2. `PORT_PARITY_TODO.md` — Active ordered backlog

Ordered implementation plan for reaching full feature and behavior parity.
Each item has clear done/open markers, file references, and C source pointers.

**Best for:** Picking up the next parity task, understanding what is in-flight or open.

---

### 3. `PARITY_AUDIT_TABLE.md` — Source-backed parity audit table

Cross-subsystem audit table comparing ironwail-go against andrei-drexler/ironwail.
Columns: Subsystem | Ironwail feature | Go status | Evidence / notes | Gap type | Priority.

**Best for:** Planning/issue filing, quick gap identification, cross-subsystem view.

---

### 4. `PARITY_SUMMARY.md` — Current executive summary

Short executive summary of current state, biggest gaps, and what older docs got wrong.

**Best for:** Onboarding, stakeholder briefings, quick current-state overview.

---

### 5. `QUICK_REFERENCE.txt` — Operational snapshot card

Short card covering: authoritative target, current strengths, biggest gaps, doc usage rules.

**Best for:** Day-to-day reference, quick sanity checks, team briefings.

---

## Historical docs (archive context only)

> ⚠️ These documents are **not** current status tracking. They describe an earlier state of the port.
> Do **not** cite percentages, severity labels, or tier tables from these docs as current defects.

### `parity_report.md` — Historical deep-dive snapshot (2026-03)

Comprehensive early-audit analysis of all 7 functional areas.
Many of its gap claims have since been resolved. See the warning banner at the top of the file.

**Use for:** Historical context, deep C/Go algorithm comparison, older gap rationale.
**Do not use for:** Current planning, defect filing, or status reporting.

---

## Quick navigation by use case

| Question | Go to |
|---|---|
| "What is currently implemented?" | `PORT_PARITY_REVIEW.md` |
| "What should I work on next?" | `PORT_PARITY_TODO.md` |
| "Is feature X complete?" | `PORT_PARITY_REVIEW.md` → `PARITY_AUDIT_TABLE.md` |
| "We're starting a sprint" | `PORT_PARITY_TODO.md` + `PORT_PARITY_REVIEW.md` |
| "I need a quick gap summary" | `QUICK_REFERENCE.txt` |
| "Updating the README" | `PORT_PARITY_REVIEW.md` as source of truth |
| "Why was feature Y originally broken?" | `parity_report.md` (historical context) |
| "Is gogpu parity-complete?" | No — see gogpu section of `PORT_PARITY_REVIEW.md` |

---

## Verification baseline (current)

- [x] World renders with lightmaps on the OpenGL parity path
- [x] Alias/sprite/particle/decal/viewmodel runtime paths are wired
- [x] Sound dispatch, mixer, and static sounds are live
- [x] Menu/console/bind/config persistence baseline is functional
- [x] Save/load round-trip enforces C-style restrictions and restores gameplay state
- [x] Demo records and plays back (forward path with connected-state snapshot)
- [ ] Underwater visual blue-shift parity remains open
- [ ] Remote multiplayer still needs broader netgame depth
