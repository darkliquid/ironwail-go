# Implementation Plan 18: Backward Compatibility Shim Removal & Educational Architecture Documentation

**Priority**: High  
**Status**: Planned  
**Target Milestone**: Phase 18  

---

## 1. Executive Summary & Architectural Context

During major architectural migrations (such as Phase 1-3 Zero-Sync QCVM, Phase 5 Modularisation, and WASM VFS expansion), temporary compatibility shims, legacy alias functions, and deprecated type wrappers were retained to prevent build breakages.

Now that all primary migrations are complete, this plan removes backward compatibility shims, streamlines function signatures to follow standard Go idioms, and updates package `doc.go` files and inline comments to serve as an educational reference for AI agents and human engineers.

---

## 2. Technical Strategy & Architecture

1. **Shim Audit & Removal**:
   - Audit and remove legacy cvar aliases, deprecated type aliases in `pkg/types`, and unused mirror callbacks in `internal/game/game_init.go`.
   - Remove dead code paths from reverted experiments (e.g. legacy model matrix uniform setters).
2. **Standardize Error Types & Sentinel Errors**:
   - Convert C-style integer status returns ($0, -1$) into standard Go `error` returns and exported sentinel errors (`ErrMapNotFound`, `ErrInvalidPakHeader`, `ErrEdictLimitExceeded`).
3. **Comprehensive Educational Documentation (`doc.go`)**:
   - Ensure every package contains a standard 5-section `doc.go` file (`Package Overview`, `Architecture & Control Flow`, `Key Types & Structs`, `Original C Lineage`, `Testing & Verification`).

---

## 3. Step-by-Step Implementation Sequence

### Step 18.1: Remove Legacy Cvar Alias Shims
- **Files**: `internal/game/game_init.go`, `internal/cvar/cvar.go`
- **Actions**: Clean up redundant legacy cvar registration wrappers where canonical cvars match C Quake names.

### Step 18.2: Standardize Error Returns
- **Files**: `internal/client/`, `internal/host/`
- **Actions**: Replace C-style status code returns with Go `error` types and `errors.Is` / `errors.As` support.

### Step 18.3: Educational Architecture Documentation Audit
- **Files**: `docs/LEARNING_GUIDE.md`, all package `doc.go` files
- **Actions**: Refresh sitemaps, subsystem walkthroughs, and package interaction diagrams.

---

## 4. Verification & Testing Strategy

```bash
mise run lint
mise run verify
```
