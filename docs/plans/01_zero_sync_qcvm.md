# Implementation Plan: Direct-VM Accessors & Zero-Sync QCVM

**Priority**: #1 (Item 2 from Roadmap)  
**Status**: Planned  
**Target Milestone**: Phase 1  

---

## 1. Executive Summary & Architectural Context

In original C Quake, the engine and the QuakeC Virtual Machine (QCVM) share the **exact same memory**. An `edict_t` struct contains engine fields alongside embedded `entvars_t v` fields. QuakeC bytecode instructions (`OP_LOAD_*`, `OP_STORE_*`) operate directly on `&ed->v + field_offset`. There is no data copying or synchronization layer.

In `ironwail-go`, Go's garbage collector and prohibition of raw pointer arithmetic forced a **dual-storage architecture**:
1. Typed Go structs: `Edict.Vars *EntVars` (Go's source of truth for physics/networking).
2. Flat byte array: `s.QCVM.Edicts []byte` (where QC bytecode operates).

Currently, `server_qc_sync.go` reflectively copies 78 bound fields back and forth between these two representations (`syncAllToQCVM` / `syncAllFromQCVM`) at every single QC callback in `executeQCFunction`. CPU profiling on large maps (`qbj3_stickflip`) shows that reflection edict sync is one of the top CPU bottlenecks in the server frame.

The goal of this project is to complete Steps 3–5 of the QCVM migration plan: replace `EntVars` field access with direct-VM accessor methods (`ent.Origin()`, `ent.SetOrigin()`), delete `server_qc_sync.go`, delete `EntVars`, and achieve C Quake's zero-sync architecture.

---

## 2. Existing Code Analysis & Current State

- **Accessors Available**: `internal/server/entity_accessors.go` defines 157 typed methods on `Edict` (`ent.Origin()`, `ent.SetOrigin()`, `ent.Velocity()`, `ent.SetVelocity()`, `ent.Think()`, `ent.SetThink()`, `ent.Solid()`, `ent.SetSolid()`, etc.) that read/write `s.QCVM.Edicts` directly.
- **Partial Adoption**: `server.go` contains ~27 call sites already using direct-VM accessors for area-grid and leaf traversal.
- **Hot Paths Remaining**: Physics and movement loops in `physics.go`, `physics_loop.go`, `physics_push.go`, `physics_step.go`, `physics_toss.go`, and `world.go` still read and mutate `ent.Vars.*` directly.
- **Savegame Dependency**: `savegame.go` serializes `EntVars` struct fields rather than saving the `QCVM.Edicts` byte slice directly.

---

## 3. Step-by-Step Implementation Sequence

### Step 1.1: Migrate Physics and Movement Hot Paths
- **Files**: `internal/server/physics.go`, `physics_loop.go`, `physics_push.go`, `physics_step.go`, `physics_toss.go`, `world.go`
- **Actions**:
  - Replace all `ent.Vars.Origin` reads with `ent.Origin()`, and writes with `ent.SetOrigin(v)`.
  - Replace `ent.Vars.Velocity` reads with `ent.Velocity()`, and writes with `ent.SetVelocity(v)`.
  - Replace `ent.Vars.Flags`, `ent.Vars.Solid`, `ent.Vars.Movetype`, `ent.Vars.Mins`, `ent.Vars.Maxs`, `ent.Vars.Absmin`, `ent.Vars.Absmax`, `ent.Vars.Angles`, `ent.Vars.Avelocity`, `ent.Vars.Punches`, `ent.Vars.Ltime`, `ent.Vars.Nextthink`, `ent.Vars.Think` with their respective accessor methods.
  - Update `SV_LinkEdict` and `SV_TouchLinks` in `world.go` to use direct accessors.

### Step 1.2: Update Savegame Serialization
- **Files**: `internal/server/savegame.go`
- **Actions**:
  - Update level save and restore functions to write and read `s.QCVM.Edicts` byte slice directly.
  - Ensure byte-order and header metadata compatibility for saved games.

### Step 1.3: Delete Reflection Synchronization Layer
- **Files**: `internal/server/server_qc_sync.go`, `internal/server/qc_trace.go`
- **Actions**:
  - Remove `syncAllToQCVM` and `syncAllFromQCVM` calls from `executeQCFunction` in `qc_trace.go`.
  - Delete `internal/server/server_qc_sync.go` (`syncAllToQCVM`, `syncAllFromQCVM`, `syncEdictToQCVM`, `syncEdictFromQCVM`).

### Step 1.4: Delete `EntVars` Struct
- **Files**: `internal/server/types.go`, `pkg/types/`
- **Actions**:
  - Remove `EntVars` struct definition.
  - Remove `Vars` field from `Edict` struct.

---

## 4. Edge Cases & C Parity Oracles

- **IEEE Divide-by-Zero Parity**: QuakeC programs sometimes divide by zero intentionally. Go `float32` direct-VM reads must match C's IEEE float behavior (`exec_test.go`).
- **Runaway Loop Limit**: Runaway loop limit constant must remain exactly `0x1000000` (`exec.go`).
- **C Parity Oracle**: `ironwail/Quake/pr_exec.c` (`PR_ExecuteProgram`) and `ironwail/Quake/progs.h` (`EDICT_TO_PROG`).

---

## 5. Testing & Verification Plan

1. **Unit Tests**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/... -count=1
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/server/... -count=1
   ```
2. **Physics Parity Tests**:
   - Run `TestPhysicsWalkJump`, `TestPhysicsPusher`, `TestPhysicsStep` in `internal/server/`.
3. **CPU Profiling Benchmark**:
   - Run `profile_cpu_start`, `map qbj3_stickflip`, `profile_cpu_stop` in console to verify that `syncEntVarsFromQC` and `SetEFloat` disappear from top CPU consumers.
