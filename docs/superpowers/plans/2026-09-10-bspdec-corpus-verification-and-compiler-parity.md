# BSPDec Corpus Verification, Compiler Void Parity, and Model Training Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an automated dual-compiler test harness across the full Quaddicted + classic corpus to categorize map voids, fix Go `qbsp` leak-detection disparities against `ericw-tools`, retrain the `bspdec` ML model on the expanded corpus, and verify sealed decompilation/recompilation roundtrips.

**Architecture:** 
1. A multi-compiler audit harness (`bspdec-corpus audit` / `tools/bspdec_report audit`) that evaluates forward compiles and decomp/recomp pairs with both Go `qbsp` and `ericw-tools` `qbsp`.
2. A void categorization engine that bins maps into:
   - **Category A**: Inherent geometry voids (open in `ericw-tools`).
   - **Category B**: Go compiler disparities (voids ONLY in Go `qbsp`, sealed in `ericw-tools`).
   - **Category C**: Decompilation voids, attributed to Go only, Ericw only, or both.
3. Bug analysis and fixes in `internal/qbsp` to resolve portal/adjacency/flood leakage disparities.
4. Corpus labeling and retraining of the Route A model in `tools/bspdec_train` over the verified corpus.
5. Decomp/recomp roundtrip validation with comprehensive Markdown/JSON reporting.

**Tech Stack:** Go 1.26 (`CGO_ENABLED=0`), `internal/qbsp`, `internal/bspdec`, `internal/bspdec/eval`, `ericw-tools` (`bin/ericw-tools/qbsp`), `tools/bspdec_corpus`, `tools/bspdec_train`, `tools/bspdec_report`.

---

## File Structure

- Create: `internal/bspdec/eval/audit.go`
  - Defines `AuditResult`, `ForwardCategory`, `DecompCategory`, `AuditMap(mapPath, bspPath string) AuditResult`, and batch auditing.
- Create: `internal/bspdec/eval/audit_test.go`
  - Tests forward and decomp audit classification against mock/real fixtures.
- Modify: `tools/bspdec_corpus/main.go` and `tools/bspdec_corpus/stages.go`
  - Adds `audit` stage to CLI (`bspdec-corpus audit [-data dir] [-limit N] [-workers W] [-o report.md] [-json report.json]`).
- Modify: `tools/bspdec_report/main.go`
  - Adds reporting formatting for Category A, B, and C tables.
- Modify: `internal/qbsp/world.go` (and related files in `internal/qbsp/`)
  - Fixes leaf adjacency and flood leak false-positives causing Go `qbsp` to report voids when `ericw-tools` seals properly.
- Test: `internal/qbsp/leak_disparity_test.go`
  - Unit test capturing the disparity case and ensuring sealed compilation.
- Model artifacts: `models/bspdec/route-a-v*/`
  - Updated model weights and metrics derived from expanded corpus.

---

## Tasks

### Task 1: Design & Implement Dual-Compiler Audit Engine in `internal/bspdec/eval`

**Files:**
- Create: `internal/bspdec/eval/audit.go`
- Create: `internal/bspdec/eval/audit_test.go`

- [ ] **Step 1: Write failing unit test for `AuditMap` in `audit_test.go`**
  - Test a known sealed map fixture: returns `ForwardCategory: Clean`, `DecompCategory: Clean`.
  - Test a known leaky map fixture (missing ceiling): returns `ForwardCategory: CategoryA` (void in ref compiler).
  - Test a mock disparity fixture where Go `qbsp` reports leak but `ericw-tools` is sealed: returns `ForwardCategory: CategoryB`.

- [ ] **Step 2: Run test to verify it fails**
  - Run: `TMPDIR=.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -run TestAuditMap -count=1`
  - Expected: Compilation failure or undefined functions.

- [ ] **Step 3: Implement `AuditMap` in `audit.go`**
  - Define `ForwardVerdict`: `Clean`, `CategoryA_InherentVoid`, `CategoryB_GoCompilerDisparity`.
  - Define `DecompVerdict`: `Clean`, `CategoryC_BothLeaked`, `CategoryC_GoOnlyLeaked`, `CategoryC_RefOnlyLeaked`.
  - Compile original `.map` with Go `qbsp` and `ericw-tools`.
  - Decompile `.bsp` with `bspdec.Decompile`, write `.decomp.map`, compile with Go `qbsp` and `ericw-tools`.
  - Return structured `AuditResult` with leak trail coordinates and timing.

- [ ] **Step 4: Run test to verify it passes**
  - Run: `TMPDIR=.tmp CGO_ENABLED=0 go test ./internal/bspdec/eval -run TestAuditMap -count=1`
  - Expected: PASS.

- [ ] **Step 5: Commit**
  - Git commit: `feat(bspdec): add dual-compiler audit engine in eval package`

---

### Task 2: Implement CLI Audit Stage and Report Generator

**Files:**
- Modify: `tools/bspdec_corpus/main.go`
- Modify: `tools/bspdec_corpus/stages.go`
- Modify: `tools/bspdec_report/main.go`

- [ ] **Step 1: Add `audit` stage to `tools/bspdec_corpus`**
  - Add `stageAudit` in `tools/bspdec_corpus/stages.go`.
  - Support flags: `-limit`, `-workers`, `-out-json`, `-out-md`.
  - Iterate over manifest packages in parallel workers, running `AuditMap`.
  - Write machine-readable `audit.json` and human-readable Markdown report.

- [ ] **Step 2: Implement Markdown report formatting in `tools/bspdec_report`**
  - Section 1: Executive Summary with totals and pass rates.
  - Section 2: **Category A — Inherent Geometry Voids** (table of maps that leak in the reference compiler).
  - Section 3: **Category B — Go Compiler Disparities** (table of maps that leak only in Go `qbsp`, with leak entity & origin).
  - Section 4: **Category C — Decompilation Voids** (table partitioned by: Leaks in Both, Leaks in Go Only, Leaks in Ericw Only).

- [ ] **Step 3: Test audit command on small sample**
  - Run: `./bin/bspdec-corpus audit -data dataset/bspdec -limit 20 -workers 4 -out-md .tmp/audit_sample.md`
  - Verify generated markdown table and categorizations.

- [ ] **Step 4: Commit**
  - Git commit: `feat(bspdec-corpus): add audit command and multi-category void reporting`

---

### Task 3: Identify & Fix Go `qbsp` Compiler Disparities (Category B & Category C Go-Only)

**Files:**
- Test: `internal/qbsp/leak_disparity_test.go`
- Modify: `internal/qbsp/world.go`

- [ ] **Step 1: Write reproducing test case in `internal/qbsp`**
  - Use `rpg100b1.decomp.map` or a minimized extraction from it where Go `qbsp` leaked from void leaf to entity while `ericw-tools` compiled sealed.
  - Assert that `Compile` produces `res.Leaked == false`.

- [ ] **Step 2: Run test to confirm failure**
  - Run: `TMPDIR=.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestLeakDisparity -count=1`
  - Expected: FAIL with "leaks to the void".

- [ ] **Step 3: Investigate root cause in `internal/qbsp/world.go`**
  - Trace `buildWorldPortals` / `splitByTree`:
    Check why leaf adjacency links `voidLeaf` through solid geometry to `monster_knight` at `(1104 224 632)`.
    Verify facet tolerance, polygon splitting epsilon, or bounding box clipping in `L.region.facets(bounds)`.
  - Apply fix: tighten portal facet clipping against bounding box / enforce strict solid boundary checks in flood adjacency.

- [ ] **Step 4: Verify test passes**
  - Run: `TMPDIR=.tmp CGO_ENABLED=0 go test ./internal/qbsp -run TestLeakDisparity -count=1`
  - Expected: PASS.

- [ ] **Step 5: Verify all `internal/qbsp` and repo tests pass**
  - Run: `TMPDIR=.tmp CGO_ENABLED=0 go test ./internal/qbsp/... -count=1`
  - Expected: All pass.

- [ ] **Step 6: Commit**
  - Git commit: `fix(qbsp): eliminate false-positive portal flood leakage to void`

---

### Task 4: Expand Labeled Corpus and Train `bspdec` Route A Model

**Files:**
- Modify: `dataset/bspdec/` (metadata and labels)
- Run: `tools/bspdec_corpus` (`canonicalize`, `labels`, `splits`)
- Run: `tools/bspdec_train`

- [ ] **Step 1: Canonicalize verified sealed Quaddicted packages**
  - Run `bspdec-corpus canonicalize -data dataset/bspdec` on verified non-leaking packages to populate `dataset/bspdec/paired/`.

- [ ] **Step 2: Derive labels across the expanded corpus**
  - Run `bspdec-corpus labels -data dataset/bspdec` to produce cell/seam labels in `dataset/bspdec/labeled/`.

- [ ] **Step 3: Generate updated train/val/test splits**
  - Run `bspdec-corpus splits -data dataset/bspdec`.

- [ ] **Step 4: Train updated Route A model**
  - Run `bspdec-train -data dataset/bspdec -route a -seed 24237 -out models/bspdec`.
  - Verify train metrics, feature F1 scores, and gate checks pass:
    `bspdec-train -data dataset/bspdec -validate`
    `bspdec-train -data dataset/bspdec -gate`

- [ ] **Step 5: Commit model improvements & updated splits**
  - Git commit: `feat(bspdec): retrain Route A model on expanded Quaddicted corpus`

---

### Task 5: Run Full Corpus Audit & Generate Master Void Report

**Files:**
- Run: `bspdec-corpus audit`
- Produce: `docs/BSPDEC_CORPUS_AUDIT_REPORT.md`

- [ ] **Step 1: Run comprehensive audit across corpus**
  - Execute audit harness over the full paired corpus with both compilers.
  - Collect `audit.json` and generate `docs/BSPDEC_CORPUS_AUDIT_REPORT.md`.

- [ ] **Step 2: Verify report contents**
  - Check Category A: lists only genuine author map voids confirmed by reference compiler.
  - Check Category B: confirms 0 (or documented minimal) Go compiler disparities.
  - Check Category C: documents decompiled map seal rates under both compilers.

- [ ] **Step 3: Run full verification suite**
  - `mise run test`
  - `mise run build`
  - `golangci-lint run ./...`
  - `graphify update .`

- [ ] **Step 4: Commit**
  - Git commit: `docs(bspdec): publish comprehensive corpus void and decompilation audit report`
