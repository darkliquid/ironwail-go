# Implementation Plan: Client-Side QuakeC (CSQC) Host/Client Runtime Integration

**Priority**: #4 (Item 4 from Roadmap)  
**Status**: Completed  
**Target Milestone**: Phase 4  

---

## 1. Executive Summary & Architectural Context

CSQC (Client-Side QuakeC) is an extended Quake engine feature that allows mods to supply a secondary `csprogs.dat` bytecode file. While server QuakeC (`progs.dat`) runs authoritatively on the server VM, CSQC runs on the client to draw custom HUD elements, manage client-side visual effects, and predict local entity movements.

Currently, `ironwail-go` has CSQC wrapper VM infrastructure (`internal/qc/csqc.go`) and unit tests (`csqc_test.go`), but the engine's client frame pump (`internal/client`) and overlay renderer (`internal/renderer`) do not execute CSQC draw hooks or event callbacks during gameplay.

The goal of this project is to integrate CSQC into the host/client runtime loop, allowing `csprogs.dat` HUD drawing, overlay passes, and client input handling to run when a mod supplies a CSQC bytecode file.

---

## 2. Existing Code Analysis & Current State

- **CSQC Wrapper**: `internal/qc/csqc.go` defines `CSQC` type with function indices for `CSQC_Init`, `CSQC_DrawHud`, `CSQC_DrawScores`, `CSQC_InputEvent`, and `CSQC_Parse_StuffCmd`.
- **Unit Tests**: `internal/qc/csqc_test.go` verifies loading `csprogs.dat`, precache registration, and global sync.
- **Client Frame Loop**: `internal/client/client.go` and `internal/client/parse.go` do not call CSQC hooks upon receiving entity snapshots.
- **Overlay Pass**: `internal/renderer/renderer_gogpu_overlay.go` (`flush2DOverlay`) composites standard HUD and console, but does not invoke CSQC draw hooks.

---

## 3. Step-by-Step Implementation Sequence

### Step 4.1: CSQC VM Lifetime in Client
- **Files**: `internal/client/client.go`, `csqc_runtime.go` (new)
- **Actions**:
  - Load `csprogs.dat` via `internal/qc/csqc.go` during signon (`SignonBegin`) if `csprogs.dat` exists in the VFS search path.
  - Execute `CSQC_Init` on client startup.
  - Shutdown CSQC VM on disconnect.

### Step 4.2: Wire Overlay Draw Hooks
- **Files**: `internal/renderer/renderer_gogpu_overlay.go`, `internal/client/`
- **Actions**:
  - In `flush2DOverlay()`, if CSQC VM is active, call `csqc.CallDrawHud()` and `csqc.CallDrawScores()`.
  - Allow CSQC to render 2D pictures, strings, and custom health/ammo counters into the overlay canvas.

### Step 4.3: Wire Input Event Dispatch
- **Files**: `internal/input/`, `internal/client/`
- **Actions**:
  - In key event handler, route events through `csqc.CallInputEvent(evType, key, ascii)` if CSQC is active. If CSQC returns `1` (handled), suppress default key processing.

### Step 4.4: Entity Snapshot Hook
- **Files**: `internal/client/parse.go`
- **Actions**:
  - Pass server entity update snapshots to CSQC entity prediction hook if present.

---

## 4. Edge Cases & C Parity Oracles

- **VFS Fallback**: If `csprogs.dat` is absent, the engine must fall back seamlessly to standard engine HUD rendering without error.
- **C Parity Oracle**: `ironwail/Quake/cl_csqc.c` (`CSQC_Init`, `CSQC_DrawHud`, `CSQC_InputEvent`).

---

## 5. Testing & Verification Plan

1. **Unit Tests**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/... ./internal/client/... -count=1
   ```
2. **Integration Test**:
   - Add `TestCSQCHudDrawLifecycle` in `internal/client/` verifying that `CSQC_Init` and `CSQC_DrawHud` execute in sequence during signon and draw phases.
