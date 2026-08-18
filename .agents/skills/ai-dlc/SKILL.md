---
name: ai-dlc
description: 'Use when the user wants to derive structured work from an idea (specification -> reviews -> ADRs -> reviews -> updated artifacts -> implementation plan -> review -> Beads breakdown). Trigger phrases: "spec this", "run the lifecycle", "AI-DLC", "speckit", "derive a spec", "write an ADR for", "make an implementation plan", "break down into beads". This is the Project Engineering Lifecycle (Spec -> Review -> ADRs -> Review -> Plan -> Review -> Work items) formalized as a repeatable skill.'
---

# AI-DLC / SpecKit Lifecycle

Formalized Project Engineering Lifecycle. Every stage produces a durable, reviewed artifact in `docs/internal/`, and the final stage fans out into `bd` (Beads) work items an agent can actually schedule.

## Core Principles

1. **Specs are elicited, not written.** Stage 1 grills the user. Ambiguity, missing context, and unstated trade-offs are pulled out _before_ prose is written, not discovered in review.
2. **Each stage has a defensive review.** Reviews exist to break the artifact, not to bless it. If a review turns up gaps, they are resolved or spun off as **research tasks** before moving on.
3. **Every artifact is an editing loop, not a one-shot write.** ADRs improve the spec, the plan improves the ADRs, the final plan improves everything. Re-integration is mandatory.
4. **Work items are concrete.** Beads created at the end must each be independently schedulable: a single issue, clear scope, dependencies, and acceptance criteria.

## Workflow

```
  ┌─────────────┐  ┌──────────────────────────────────────────────────────────────┐
  │   1. ELICIT │  │ Ask ~8-12 pointed questions (scope, users, constraints,      │
  │  (grill)    │──│ data, failure modes, scale, security, non-goals, interface,  │
  └─────────────┘  │ acceptance). Every gap found: log + spin research task.      │
        │          └──────────────────────────────────────────────────────────────┘
        v
  ┌─────────────┐  ┌──────────────────────┐        ┌────────────────────────────────────────┐
  │  2. SPEC    │  │ Write PROJECT-SPEC   │──gap?──│ RESEARCH TASK (spin-off)               │
  └─────────────┘  └──────────────────────┘        │ docs/internal/research/NNNN-*.md (doc) │
        │                                          │ + bd create -t research (bead)         │
        v                                          └──────────────┬─────────────────────────┘
  ┌─────────────┐  ┌──────────────────────┐                       │
  │ 3. REVIEW 1 │  │ Hostile review of    │──gap?─── findings ────┘
  └─────────────┘  │ the spec             │            reintegrate│
        │          └──────────────────────┘                       │
        v                                                         │
  ┌─────────────┐  ┌──────────────────────┐        ┌────────────────────────────────────────┐
  │ 4. ADRs     │  │ Derive ADRs, one per │──gap?──│ RESEARCH TASK (spin-off)               │
  └─────────────┘  │ significant decision │        │ docs/internal/research/NNNN-*.md (doc) │
        │          └──────────────────────┘        │ + bd create -t research (bead)         │
        v                                          └───────────────────────────────┬────────┘
  ┌─────────────┐  ┌──────────────────────┐                                        │
  │ 5. REVIEW 2 │  │ Review ADRs          │──gap?──────── findings ────────────────┘
  └─────────────┘  └──────────────────────┘
        │
        v
  ┌─────────────┐  ┌──────────────────────┐        ┌────────────────────────────────────────┐
  │ 6. REVISE   │  │ Fold ADR outcomes +  │──gap?──│ RESEARCH TASK (spin-off)               │
  │             │  │research findings into│        │ docs/internal/research/NNNN-*.md (doc) │
  └─────────────┘  │ spec & ADRs          │        │ + bd create -t research (bead)         │
        │          └──────────────────────┘        └──────────────┬─────────────────────────┘
        v                                                        findings
  ┌─────────────┐  ┌──────────────────────┐                        │
  │ 7. PLAN     │  │ Red-Green TDD phases,│──gap?─── re-integrate ─┘
  └─────────────┘  │ file paths, test cmds│
        │          └──────────────────────┘
        v
  ┌─────────────┐
  │ 8. REVIEW 3 │  Review the plan; gaps -> research or back to 6.
  └─────────────┘
        v
  ┌─────────────┐  ┌──────────────────────────────────────────────┐
  │ 9. BEADS    │  │ Break the approved plan into bd issues with  │
  │             │  │ dependencies, priorities, and acceptance     │
  └─────────────┘  │ criteria. Each bead = one schedulable unit.  │
                   └──────────────────────────────────────────────┘
```

## How to Run This Skill

You are the **Lifecycle Driver**. At each review stage (Stages 3, 5, and 8), present the draft artifact, state what you reviewed and what you found, and then **stop and wait for explicit human sign-off** before proceeding. Do not self-approve and continue. If running without a human available, surface the review findings and block until a response is received.

Run the lifecycle directly: take the user's idea or prompt and execute
Stages 1-9 below. No wrapper task, script, or CLI entrypoint is required —
the driver runs the process itself. (Optionally record the prompt as a bead
note at the start so the run has a durable identifier.)

## Project-Type Tailoring

This lifecycle is deliberately generic. Before grilling the user (Stage 1),
**determine what kind of project this is** so the questions and review checks
are concrete instead of hypothetical. Classify from evidence — manifest/build
files (`go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, ...), the
language and frameworks in imports, entry points, and architecture docs
(`AGENTS.md`, repo `docs/`). Record the classification in the gap log so every
later stage reuses it.

| Project kind | Evidence | Typical things to check in elicit/review |
| --- | --- | --- |
| Web / service | HTTP routes, API handlers, migrations | authN/Z, tenant or multi-user isolation, API contract stability, request validation, rate limits, session/token handling, schema + migration safety |
| Game / engine | render loop, input handling, entity code, asset pipeline | frame timing, input routing/latching, asset loading + caching, GPU/resource lifetime, determinism/replay, save formats |
| CLI / tooling | flag parsing, command dispatch, stdio | flag/env surface, exit codes, streaming output, config file format, stderr discipline, tab/completion |
| Library / SDK | exported API surface, no main | public API stability, package/module layout, semver policy, backward compatibility, docs |
| Data / pipeline | ETL, batch, streaming, consumers | schema + versioning, idempotency, backfill/recovery, partition/ordering, delivery semantics |
| Desktop app | windowing, DPI, theming, platform code | window/surface ownership, DPI scaling, theming/design system, platform quirks, IME input |
| Embedded / systems | constrained targets, no heavy deps | memory budget, allocation policy, concurrency, determinism, cross-compile matrix |

A repo can be several kinds at once (a game engine is also a library). Use the
union of applicable checkpoints. In every stage below, "what applies to this
project type" means the relevant rows of this table. Default to the most
specific matching kind over vague generics — but never invent stack details
the repo does not actually use.

### Naming & Versioning Conventions

| Artifact       | Path                                                          | Naming                                                                                      |
| -------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| Spec           | `docs/internal/specs/`                                        | `NN-slug.md`, header `PROJECT-SPEC-NNN`. `NN` = next number (25 = `.gitignore` in `docs/`). |
| ADR            | `docs/internal/adr/`                                          | `NNNN-slug.md` (4-digit, next after max)                                                    |
| Plan           | `docs/internal/plans/`                                        | `NN-slug-implementation-plan.md`, mirrors its spec number                                   |
| Research       | `docs/internal/research/`                                     | `NNNN-slug.md` (4-digit, next after max)                                                    |
| Gap log        | `.ai-dlc/<slug>/gaps.md`                                      | Per-run working file                                                                        |
| Research tasks | `docs/internal/research/` (markdown) + `bd` (type `research`) | doc = durable finding, bead = tracked deliverable                                           |
| Working dir    | `.ai-dlc/<slug>/`                                             | scratch ephemera; gitignored                                                                |

Gap log format:

```markdown
# Gap Log: <slug>

| #   | Stage  | Gap description | Severity | Resolution state | Research id | Research doc              |
| --- | ------ | --------------- | -------- | ---------------- | ----------- | ------------------------- |
| 1   | ELICIT | undefined authz | high     | OPEN             | project-xxx | docs/internal/research/NNNN-xxx.md |
```

Resolution states: `OPEN`, `RESEARCHING`, `RESOLVED`, `WONTFIX`.

---

## Stage 1: Elicit (the grill)

Ask pointed questions until the shape of the work is clear. Do not write the spec yet. Customize to the domain; suggested categories (clockwise from scope):

1. **Scope** — What is in / out? What is the minimum viable version?
2. **Users & roles** — Who uses this? What permissions do they need?
3. **Data** — What entities, what schema changes, what happens to existing data?
4. **Interfaces** — What consumes this?
5. **Failure modes** — What breaks, what is the blast radius, what degrades gracefully?
6. **Scale & performance** — Volumes, latencies, and the growth ceiling that matters for this project type.
7. **Security** — AuthN/Z and isolation boundaries (tenants, users, mods, plugins — whatever applies), encryption, secret handling, audit trail.
8. **Dependencies & constraints** — What already exists (specs/ADRs above), what constrains the design, what must NOT change.
9. **Non-goals** — Explicitly excluded so review can't sneak them in later.
10. **Acceptance criteria** — How do we know it is done? Definition of Done.
11. **Priorities** — Must / should / could / won't (MoSCoW) for this round.

Rules:

- Every gap (unanswerable question, unknown, stated uncertainty) gets a row in the gap log and, if it needs external investigation, a **research task** (see Research Tasks).
- The user may say "I don't know" — that is a gap, not a dead end. Log + spin research.
- Stop grilling when the last 2+ answers are non-surprising / the question list is exhausted and gaps are all `RESOLVED` or spun off. State what you will do next.
- Before grilling, classify the project kind per Project-Type Tailoring and let it pick the concrete categories and checkpoints used here.

## Stage 2: Write the Spec

`docs/internal/specs/NN-slug.md` following the existing template (see `docs/internal/specs/NN-*.md` for the canonical structure):

1. **Metadata & Overview** — Component Identifier (`PROJECT-SPEC-NNN`), name, language/runtime, primary dependencies, target location in codebase.
2. **High-Level AI-DLC / Agent Prompt** — a prompt block an implementer agent can be handed, with numbered constraints.
3. **Information architecture / topology** — the structural map of what the change touches: UI tree, message/subject namespaces, RPC/API surface, data schema, wire formats, or whatever the project kind dictates.
4. **Data models & schema** — what entities and state change, where they live (relational DB, files, in-memory, wire formats), and what happens to existing data. Adapt to the project's actual storage; never assume a database exists.
5. **State machines / flows** (Mermaid where the repo uses it).
6. **Security model** — the isolation and trust boundaries that apply to this project type (multi-user authz, tenant isolation, mod/plugin trust, input provenance), and how they are enforced.
7. **Edge cases & failure handling.**
8. **Acceptance criteria** — from Stage 1, each mapped to an implementer-verifiable check.
9. Cross-references to in-scope existing specs/ADRs.

## Stage 3: Review 1 (Spec defense)

Attack the spec as a hostile reviewer. Ask, per section: _what is undefined? what contradicts another section? what is unimplementable? what is unsafe? what is un-testable?_ Check specifically:

- Isolation/boundary claims (tenants, users, mods, plugins) asserted but not designed.
- Acceptance criteria not obviously verifiable.
- Interface contracts (API fields, message subjects, wire formats, exported symbols) that would break existing consumers.
- Scope items that silently need an epic of their own (split into research or defer).
- The project-kind checklist from Project-Type Tailoring: any applicable row the change touches but the spec ignores.

Every finding: resolution (fix in spec now) or gap-log entry + research task. Record the review verdict at the bottom of the spec file under a `## Review Log` section.

## Stage 4: Derive ADRs

For each significant architectural decision the spec forces (or _silently implies_), write `docs/internal/adr/NNNN-slug.md` using the existing MADR template (see `docs/internal/adr/0001-record-architecture-decisions.md`):

`Status` / `Deciders` / `Date` / `Context and Problem Statement` / `Decision Drivers` / `Considered Options` (at least 3 realistic options, pros/cons) / `Decision Outcome` (chosen option + positive/negative consequences) / optionally `## Links`.

One ADR per decision. If a decision is obvious _because an existing ADR already covers it_, reference that ADR instead of writing a new one. If a decision needs data no one has, that is a research task.

Guiding questions per decision:

- What are the real options? (not strawmen — all options must be plausible)
- What constrains us? (existing ADRs, the repo's stack and conventions in `AGENTS.md` or equivalent architecture docs, licensing, and the security posture from Stage 1)
- What did we reject and why?
- What will this decide make harder in the future?

## Stage 5: Review 2 (ADR defense)

Same hostility as Review 1, focused on:

- Option lists: are alternatives realistic, or strawmen? Are trade-offs honestly stated?
- Consistency: does each ADR contradict the spec, another ADR, or the architecture non-negotiables in AGENTS.md §3?
- Consequence honesty: are negative consequences real and specific?

Findings: fix ADRs now, or gap-log + research. Record verdict in each ADR's status line and a `## Review Log` section.

## Stage 6: Revise spec + ADRs

Fold the outcomes of Reviews 1-2 and all completed research into the artifacts:

- Spec: update schema/contracts/acceptance criteria; add a `## Change Log` entry (date + what + why-what-gap).
- ADRs: update status (`Accepted` / `Amended`) and decision outcome; add change notes.
- Gap log: mark `RESOLVED` with the resolution.

The artifacts are now internally consistent. Nothing may carry forward from a review unresolved (except explicit `WONTFIX` decisions, which must say who decided and why).

## Stage 7: Implementation Plan

`docs/internal/plans/NN-slug-implementation-plan.md`:

- **Phases** named M1, M2, ... mirroring how this repo names milestones (see `docs/internal/plans/*.md`). Each phase is independently reviewable.
- **Per task:** Red-Green TDD steps: write failing test (RED) -> minimal code (GREEN) -> make it repeatable -> verify; exact file paths; the repo's real test commands (look them up in mise.toml / AGENTS.md / project memory — never invent); acceptance criteria from the spec; dependencies on other tasks/phases.
- **Sequencing rationale**: why this order (dependencies, risk reduction, vertical slices).
- **Risks & mitigations**, each mapped to a gap-log row or research task.
- Every task traces to: the spec section, the ADRs it implements, and the acceptance criteria it satisfies. Include a traceability matrix.

## Stage 8: Review 3 (Plan defense)

- Is each task small enough to be one bead later (single issue, clear scope)? If a task is >~2 days, split it.
- Is the TDD ordering survivable (no task needs code from a task later in the sequence)?
- Are test commands real for this repo? Are file paths real?
- Are acceptance criteria testable as staged?

Fix the plan, or gap-log + research. Record verdict in a `## Review Log` section.

## Stage 9: Break down into Beads

Convert approved plan tasks into `bd` issues. One task -> one bead. This is the contract for work:

```bash
bd create "<Title>" --description="<why, spec/ADR refs, acceptance criteria>" --type=task --priority=<0-4>
```

Rules:

- **Dependency graph**: review tasks with dependent tasks, add edges `bd dep add <issue> <depends-on>` so `bd ready` shows only schedulable work.
- **Priorities from MoSCoW** (Must=1, Should=2, Could=3, Won't=N/A). Research leftovers default priority 2.
- **Type**: `task` for implementation, `research` for open investigations (never let research sit as a task type).
- **Acceptance criteria** copied verbatim from the plan into `--description`.
- Cross-reference the spec/ADR/plan paths in `--description` so the assigning agent has the artifact trail.
- **Epic collapse**: if the run is itself an epic, create the epic bead first and attach dependencies to it, mirroring the repo style (e.g. `[epic] <Name> [SPEC-NNN]`).

When done, show the final `bd ready` list.

## Research Tasks (spin-off everywhere)

Any stage can emit research. Process:

1. Log the gap (stage, description, severity).
2. Create the research bead:

```bash
bd create "<Gap title>" --description="What is unknown, why it blocks, where the answer should land (which spec/ADR section), the docs/research path where the finding must be written, and what the finding must contain to close this gap." --type=research --priority=<0-4>
```

Use type `research`, not `task` — the deliverable is a finding, not code. Untracked research is not done; a research state is only complete once its markdown document exists.

If the bd version rejects `research` as an issue type (older versions only
validate built-in types), create it as `--type=task --labels=research` instead
and note the substitution in the gap log.

1. Record the bead id in the gap log.
2. Create the durable research document at `docs/internal/research/NNNN-slug.md` the moment the finding is delivered (see Research Document Format below). Record its path in the gap log.
3. Research findings are **re-integrated** at the next artifact-write stage (spec revision, ADR derivation, ADR revision, or plan generation). The researching agent updates the gap row to `RESOLVED` with a summary, marks the research doc `Status: Integrated`, and the finding text must be real content in the artifact, not a citation.

Do not proceed past a review stage with `OPEN` or `RESEARCHING` gaps of severity `high` unless the user explicitly defers them (then mark status `WONTFIX` for this round with the deferral noted).

### Research Document Format

`docs/internal/research/NNNN-slug.md` follows the numbered `NNNN-` style of `docs/internal/adr/`. This is the deliverable of every research bead — if there is a research bead, there must be a matching document:

```markdown
# RESEARCH-NNNN: <Slug Title>

- **Status:** Draft | Delivered | Integrated
- **Owner:** <bd id of the research bead (or human)>
- **Date:** <YYYY-MM-DD>
- **Blocks:** <gap log reference, e.g. `.ai-dlc/<slug>/gaps.md #1`>

---

## Research Question

What is unknown, and why does it block the lifecycle? Which spec/ADR section will consume the answer?

## Background & Constraints

Relevant existing specs/ADRs and the repo's architecture conventions (AGENTS.md or equivalent), prior attempts, and any partial answers.

## Investigation Findings

The evidence: sources, experiments, benchmarks, diagrams. Verifiable and specific — no vibes.

## Recommended Resolution

The concrete answer to integrate into the spec/ADR/plan, stated so the next artifact-write stage can transcribe it directly.

## Open Questions / Follow-ups

Anything left unresolved and whether it spawns another gap row or a new research bead.

## Source Index

Where each claim came from (URLs from research, repo files, benchmark outputs, interviews).
```

A research doc may be created up front (at bead creation) with the Research Question and a `Status: Draft` skeleton so the finding has a home before investigation starts; update to `Delivered` when the finding lands, `Integrated` when the finding is transcribed into the spec/ADR/plan.

---

## Checklist (run before declaring done at any stage)

- [ ] Gap log: no `OPEN`/`RESEARCHING` high-severity gaps without explicit deferral.
- [ ] Every research bead has a finding **and** a `docs/internal/research/NNNN-*.md` document (or is clearly parked with a reason).
- [ ] Artifact follows the repo's existing template and conventions.
- [ ] Review Log records the verdict and the fixes made.
- [ ] Cross-references: spec <-> ADRs <-> plan traceability intact.
- [ ] `bd ready` shows only schedulable work after Stage 9.
