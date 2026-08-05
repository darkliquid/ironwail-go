# Implementation Plan 14: Expanded Parity Harness & Automated Visual Regression Sweeps

**Priority**: High  
**Status**: DONE (2026-08-05)  
**Target Milestone**: Phase 14  

---

## 1. Executive Summary & Architectural Context

Behavioral and visual parity with C Ironwail is the primary oracle for Ironwail Go. While basic screenshot comparisons exist for id1 `start` and `qbj3_stickflip`, complex community maps and Quake expansion content (*Scourge of Armagon*, *Dissolution of Eternity*, *Quake Brutalist Jam*) introduce varied lighting, dynamic brush models, custom animated textures, and complex CSQC HUD elements.

This plan expands the parity harness into a multi-map, multi-viewpoint automated regression suite capable of capturing, comparing, and reporting pixel-exact deltas across reference C Ironwail and GoGPU builds.

---

## 2. Technical Strategy & Architecture

1. **Expanded Viewpoint Dataset (`testdata/parity/viewpoints.json`)**:
   - Add multi-perspective camera coordinates (`viewpos`) for official Quake maps (`e1m1`–`e1m8`, `e2m1`–`e2m7`, `e3m1`–`e3m7`, `e4m1`–`e4m8`) and expansion maps (`hip1m1`, `r1m1`).
   - Add dedicated stress viewpoints for `qbj2_zetabyt`, `qbj3_stickflip`, `ad_v1_5`, and `mots2`.
2. **Automated Batch Harness (`tools/parity_screenshots`)**:
   - Support parallel reference capture from C Ironwail and GoGPU binaries.
   - Generate SSIM (Structural Similarity Index) and pixel-diff metrics per viewpoint.
   - Output structured JSON summary report (`testdata/parity/results.json`).
3. **Mise Integration Tasks**:
   - Extend `mise run parity-all` to support filtering by map pack or tag (`mise run parity-all --tag expansion`).

---

## 3. Step-by-Step Implementation Sequence

### Step 14.1: Audit & Capture New Map Viewpoints
- **Files**: `testdata/parity/viewpoints.json`
- **Actions**: Record `viewpos` coordinates (`pos`, `angles`) across 20+ additional reference viewpoints.

### Step 14.2: Enhance Parity Harness CLI
- **Files**: `tools/parity_screenshots/main.go`
- **Actions**:
  - Add SSIM delta threshold reporting (flag `--max-ssim-diff=0.02`).
  - Add diff image artifact generation highlighting RGB color mismatches in red.

### Step 14.3: Automated Parity Regression Suite
- **Files**: `tools/parity_screenshots/compare.go`, `mise.toml`
- **Actions**:
  - Add task `mise run parity-report` generating a summary markdown table of visual diffs.

---

## 4. Verification & Testing Strategy

1. **Local Test Execution**:
   ```bash
   go test ./tools/parity_screenshots/... -count=1
   ```
2. **Batch Parity Run**:
   ```bash
   mise run parity-all
   ```
