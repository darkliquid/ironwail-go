# Implementation Plan: Engine Modularisation & Go Idiom Adoption

**Priority**: #6 (Item 7 from Roadmap)  
**Status**: Planned  
**Target Milestone**: Phase 6  

---

## 1. Executive Summary & Architectural Context

During the initial porting phase, much of the engine logic was translated directly from C source files (`sv_phys.c`, `sv_main.c`, `cl_main.c`) into flat Go packages (`internal/server`, `internal/client`, `internal/game`). This produced several large monolithic packages where physics simulation, entity state, networking, and session lifecycle are tightly coupled in the same package scope.

Additionally, some C-style mechanical idioms (integer error flags, global state mutation) remain in parts of the server and host loops.

The goal of this project is to refactor large packages (`internal/server`) into focused sub-packages, enforce strict interface contracts between subsystems, and adopt Go-idiomatic control flow (`error` returns, `context.Context` cancellation).

---

## 2. Existing Code Analysis & Current State

- **Monolithic Packages**:
  - `internal/server/`: Contains 30+ `.go` files covering edict allocation, QC VM hooks, physics movement, spatial area triggers, and network message serialization.
  - `internal/game/`: Large coordinator struct (`Game`) owning Host, Server, QC, Renderer, Client, HUD, and Audio.
- **C Idioms Remaining**:
  - Some functions return integer status codes instead of `error`.
  - Package-level global variables exist in `server.go` and `host.go`.

---

## 3. Step-by-Step Implementation Sequence

### Step 6.1: Decompose `internal/server` Package
- **Target Directories**: `internal/server/` → `internal/server/physics/`, `internal/server/entities/`
- **Actions**:
  - Move standalone physics movement routines (`SV_WalkMove`, `SV_FlyMove`, `SV_PushMove`) into `internal/server/physics/`.
  - Move entity indexing and area-grid trigger routines (`SV_LinkEdict`, `SV_TouchLinks`, `SV_AreaTriggerEdicts`) into `internal/server/entities/`.
  - Define clear package interfaces between server session manager and sub-packages.

### Step 6.2: Refactor Error Handling & Contexts
- **Files**: `internal/host/`, `internal/server/`, `internal/client/`
- **Actions**:
  - Replace integer error codes with idiomatic Go `error` types (e.g. `ErrInvalidMap`, `ErrClientDisconnected`).
  - Introduce `context.Context` for long-running async background operations (asset loading, audio streaming).

### Step 6.3: Package Boundary Verification
- **Actions**:
  - Ensure zero import cycles between sub-packages.
  - Enforce package encapsulation (unexport internal helper functions and fields).

---

## 4. Edge Cases & C Parity Oracles

- **QuakeGo Exemption**: Note that `pkg/qgo/quakego` is explicitly exempt from Go-idiomatic rewrites per `AGENTS.md` gotcha #2 (mechanical port to preserve `progs.src` resync capability).
- **Parity Oracle**: Sub-package refactoring must not change external physics or message serialization contracts.

---

## 5. Testing & Verification Plan

1. **Lint & Static Analysis**:
   ```bash
   mise run lint
   ```
2. **Full Test Suite Verification**:
   ```bash
   mise run verify
   ```
