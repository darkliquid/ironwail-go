# Implementation Plan 19: Experimental VFS Modernisation & `io/fs` / `afero` Integration

**Priority**: High  
**Status**: Completed (2026-08-05) — 19.1 PakFS + 19.2 overlay stack (via plan 19b)  
**Target Milestone**: Phase 19  

---

## 1. Executive Summary & Architectural Context

Quake's virtual filesystem (`internal/fs`) handles loose disk files, `.pak` archives, mod overrides (`id1/`, `hipnotic/`, `rogue/`, `qbj3/`), and in-memory WebAssembly byte buffers (`LoadPackFromBytes`). While `internal/fs` is functional, its file lookup and path resolution logic combines custom path manipulation, OS-level directory traversal, and manual `PackFile` linear searches.

This plan explores an experimental refactoring to modernize `internal/fs` by wrapping loose directories, PAK archives, and HTTP WASM downloads behind Go's standard `io/fs.FS` or `spf13/afero` composable virtual filesystem interfaces (`afero.Fs`, `afero.MemMapFs`, `afero.BasePathFs`).

---

## 2. Technical Strategy & Architecture

1. **Composable `io/fs.FS` / `afero` Layering**:
   - Represent each mounted PAK archive as an `afero.Fs` / `io/fs.FS` implementation (`PakFS`).
   - Layer search paths using an `afero.OverlayFs` or custom `OverlayFS` implementation where the top layer automatically overrides lower layers.
2. **Zero-Allocation In-Memory VFS for WebAssembly**:
   - Use `afero.MemMapFs` or `io/fs.MapFS` for in-memory asset loading in browser WASM builds.
3. **Parity Maintenance**:
   - Retain Quake's case-folding lookup rules (`canonicalPackLookup`) and map override priority semantics.

---

## 3. Step-by-Step Implementation Sequence

### Step 19.1: Implement `PakFS` as an `io/fs.FS` / `afero.Fs`
- **Files**: `internal/fs/pak_fs.go` (new file)
- **Actions**: Implement `io/fs.FS`, `io/fs.ReadFileFS`, and `io/fs.StatFS` over `*Pack`.
- **Status**: ✅ **DONE (2026-08-05)**. `PakFS` implements `io/fs.FS` + `ReadFileFS` +
  `StatFS` + `ReadDirFS` over an open `*Pack`, using the standard library (no afero
  dependency — the Go module must stay dependency-light). Lookup reuses
  `canonicalPackLookup` for Quake case-folding. Tests: `TestPakFSReadFile`,
  `TestPakFSCaseInsensitive`, `TestPakFSStatAndOpen`, `TestPakFSReadDir` (all
  package-local using `LoadPackFromBytes`, no pak assets required).

### Step 19.2: Overlay VFS Stack Refactoring
- **Files**: `internal/fs/overlay_fs.go` (new file), `internal/fs/fs.go`
- **Actions**: Replace custom `lookupPaths` iteration with a clean composable overlay stack walking `io/fs.FS` sources.
- **Status**: ✅ **DONE (2026-08-05) — implemented as plan 19b
  (`docs/plans/19b_overlay_fs_replacement.md`)**. The old `lookupPaths`/
  `searchPaths` dual-slice design is deleted; `internal/fs` now resolves through a
  single ordered `mount` stack (loose `rootFS` + `PakFS`, both `io/fs.FS`), walked by
  an `OverlayFS`. Public API, `SearchResult`, `Priority`, case-folding, and traversal
  protection are unchanged; all 20 existing fs tests plus a new same-dir precedence
  test pass. NOTE: the migration preserved the pre-existing paks-above-loose ordering
  within a game dir (C Ironwail places loose above paks); that parity question is
  tracked as a separate follow-up, not changed by 19b.

### Step 19.3: Verification & Parity Sign-off
- **Files**: `internal/fs/fs_test.go`
- **Actions**: Ensure 100% test passing across mod override ordering, case insensitivity, and pak priority tests.

---

## 4. Verification & Testing Strategy

```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/fs/... -count=1
mise run verify
```
