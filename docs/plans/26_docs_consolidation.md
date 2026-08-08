# Implementation Plan 26: Documentation Consolidation & Stale-Doc Purge

**Priority**: #2 (Maintainability / Onboarding; unblocks 22-25)
**Status**: PLANNED
**Prerequisite**: none (do this first — it unblocks every other plan's
reading experience)
**Estimated effort**: 2-4 focused sessions

---

## 1. Executive Summary & Architectural Context

The repo ships a lot of docs: `docs/LEARNING_GUIDE.md` (excellent package map
with per-package guides), `docs/WALKTHROUGH_*.md` (three cross-subsystem
flows), `docs/PARITY.md` (live parity guide), `docs/QCVM_ENTITY_SYNC.md`
(consolidated QCVM sync story), `docs/plans/` (26 numbered plans, some marked
DONE/Completed, some archival), `docs/diagnoses/` (investigation logs), and
`article/` drafts (identifed by graphify). Research found real **stale-doc
drift**: `docs/QCVM_ENTITY_SYNC.md` still describes `syncAllToQCVM`/`syncAllFromQCVM`
"at every callback" when the code is now authoritative-accessor dual-write with
no-op sync functions; and `docs/plans/` mixes completed plans with obsolete
proposals ("archival_" prefixed files exist already).

Goal: a **single source of truth per subsystem**, every doc either current or
explicitly archived, and the index (LEARNING_GUIDE) linking to live docs only.

## 2. Audit Approach (deterministic, not vibes)

1. **Doc inventory with provenance**:
   - `docs/` + `docs/plans/` + `docs/diagnoses/` + `article/` — for each file:
     status marker (`# Status:` line where present), "where does this live in
     code?" pointer, last-relevant commit (`git log -1 -- <file>`).
   - Cross-check each doc's claims against the CURRENT code at the cited
     `file:line` (the research already found D7 in QCVM_ENTITY_SYNC.md; repeat
     for LEARNING_GUIDE walkthroughs and PARITY.md).
   - `docs/plans/` classification: `DONE` (move to `docs/plans/archive/` with
     pointer), `CURRENT` (keep, add/refresh Status), `SUPERSEDED` (archive +
     pointer to the newer plan).
2. **Stale-detection checklist** (encoded as a build-time lint where cheap):
   - doc references a symbol/file that no longer exists (`go doc`/grep);
   - doc says "runs at every callback" but the function is a no-op
     (D7 pattern);
   - doc cites `//go:build -tags gogpu/opengl` (prohibited by AGENTS.md);
   - doc describes an arch that a later plan replaced (e.g. old selective
     pusher sync vs zero-sync).

## 3. Step-by-Step Implementation Sequence

### Step 26.1: Doc inventory table + staleness markers
- **Files**: `docs/` new `INDEX.md` (or extend LEARNING_GUIDE front-matter).
- **Actions**: table: file → status (live/archive/diagnosis/plan) → owner
  subsystem → staleness verdict (from 26.2). This is the reviewable artifact
  before any edits.

### Step 26.2: Mechanical stale-scan
- **Files**: script `tasks/doc_stale.sh` (`doc_stale` mise task) + manual pass.
- **Actions**:
  - grep every `.md` for references to removed symbols
    (`syncAllToQCVM`, `syncAllFromQCVM`, `capturePusherSnapshots`,
    `-tags gogpu`, `-tags opengl`, `server_qc_sync.go` as active code);
  - grep for `docs/plans/*` referenced from learning guides;
  - run `go doc` spot-checks on cited anchors.
  - Output: a `docs/STALE.md` list of every doc line that contradicts code.

### Step 26.3: Heavy hitters
1. **`docs/QCVM_ENTITY_SYNC.md`** — rewrite to current truth:
   accessor dual-write is authoritative; `syncEdictTo/FromQCVM` are no-op
   shims; the remaining EntVars-removal roadmap is plans 10/12; D9 (RunThink
   clamp) is actually implemented (research verified) — resolve the open-gaps
   table with code truth.
2. **`docs/PARITY.md`** — add: D6 (entity-send sort) as documented intentional
   deviation (from plan 23 step 23.5); fix the "harness gap" section to
   reference plans 24 (H1-H6) instead of loose narrative; update the qbj3
   evidence to point at the photometric audit (plan 23 D5) rather than parked
   "local manual evidence".
3. **`docs/plans/` archival sweep** — move `*_done_*`, `archival_*`,
   `SUPERSEDED` plans to `docs/plans/archive/` with a one-line "superseded by
   plan N" pointer; keep an `docs/plans/README.md` index of live plans.
4. **`docs/LEARNING_GUIDE.md` + `docs/WALKTHROUGH_*.md`** — re-link to live
   docs only; add cross-links to plans 22/23/24/25 once they land (wasm
   walkthrough, parity gates, QC simulator) so onboarding points at the new
   DX tools.
5. **`article/` drafts** (graphify-found) — decide: promote to
   `docs/engine_walkthrough/` as the narrative companion for the web tour
   (plan 22.5) or archive. Recommend: archive the drafts, keep the outline as
   the seed for `docs/WALKTHROUGH_WEB.md`.

### Step 26.4: Status-consistency lint (CI-lite)
- **Files**: `tasks/doc_status.sh`, mise task `doc-check`.
- **Actions**: enforce (fail-only-warn) that every `docs/plans/*.md` has a
  `# Status:` line whose value ∈ {DONE, CURRENT, PLANNED, SUPERSEDED,
  ARCHIVED}; every `docs/**/*.md` that cites a `file:line` anchor resolves to
  a file that exists; no `.md` references the forbidden build tags.
- **Verify**: `mise run doc-check` green on the whole tree.

### Step 26.5: Regenerate graphify + report
- **Files**: `graphify-out/` (refresh), `docs/STALE.md` (move items into the
  inventory as resolved).
- **Actions**: after rewrites, `graphify update .` so the knowledge graph
  reflects current code; re-run the 26.2 scan and zero unresolved entries.

## 4. Verification & Testing Strategy
1. `mise run doc-check` (new) green; second pass of the stale-scan (26.2)
   yields an empty `docs/STALE.md`.
2. Every LEARNING_GUIDE link resolves; every plan has a status line.
3. No `.md` mentions `-tags gogpu`/`-tags opengl` or `syncAllToQCVM` as active.
4. `mise run test` untouched (docs-only change should not touch Go files —
   exception: none).

## 5. Risks & Mitigations
| Risk | Mitigation |
| --- | --- |
| Rewrites drift from code again | the stale-scan script (26.2) + `doc-check` (26.4) make staleness catchable in CI thereafter |
| Archiving plans loses history | move to `docs/plans/archive/` (git history preserves everything); pointer line keeps the trail |
| Over-consolidation hides diagnosis detail | diagnosis logs stay in `docs/diagnoses/` (they document *process*), only *status* moves |
| doc-check lint false positives | warn-only mode by default; escalate specific rules (forbidden build tags) to fail |

## 6. Out of Scope
- Rewriting the `pkg/qgo` QuakeGo guides (QGO_QUAKEGO_GUIDE/QGO_SPEC/QSPEC)
  — keep, they're the mod-author docs plan 25 builds on; refresh their
  "how to test" sections to point at `qcmod` once it lands.
