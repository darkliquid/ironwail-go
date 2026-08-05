# Implementation Plan 19b: Replace the `lookupPaths` Stack with a Unified Overlay FS

**Priority**: Medium-High
**Status**: Draft (planning only — no implementation yet)
**Target Milestone**: 19+ (follows 19.1 PakFS)

---

## 1. Problem Statement & Why This Is Worth Doing

Plan 19.1 added `PakFS` (`internal/fs/pak_fs.go`), a standard `io/fs.FS`
adapter over an open `*Pack`. Loose directories have *always* been exposed as
`io/fs.FS` (`os.DirFS`) in `searchPath`. But the actual file-resolution path
**does not use either adapter's `io/fs.FS` interface** — `FindFile` and
`FindFirstAvailable` hand-roll two parallel lookup loops (one for loose dirs,
one for pack files) over a `[]searchPath` slice, and `loadFromPack` /
`openSearchResult` re-implement pack reading a third way.

The result is **three overlapping implementations of the same concept**:

| Concern | Loose dir | Pak |
| --- | --- | --- |
| Filesystem abstraction | `os.DirFS` (io/fs.FS) | `PakFS` (io/fs.FS, 19.1) |
| Resolution entry point | `SearchResult{SourceFS, Path}` | `SearchResult{Pack, FilePos, FileLen}` |
| Whole-file read | `iofs.ReadFile` (fs.go:441) | `loadFromPack` seek+ReadFull (fs.go:424) |
| Streaming open | `os.Open(Path)` (fs.go:447) | `io.NewSectionReader` + nopCloser (fs.go:455) |

This is exactly the "duplicate implementations are messy and confusing" problem
the user called out — it works against the codebase's educational goal
(AGENTS.md: "clear, single-responsibility... boundaries"). The fix: a single
**overlay `io/fs.FS`** that composes the mount stack in Quake override order and
becomes the *only* resolution/read path, with `PakFS` used uniformly for
archives.

## 2. Current Architecture (ground truth, read from the tree)

### 2.1 Data structures (`internal/fs/fs.go`)

- `Pack` (fs.go:98): `Filename`, `Handle ReadSeekerCloserHandle`,
  `Files []PackFile`, `mu`.
- `PackFile` (fs.go:83): `Name`, `Lookup` (canonical, case-folded),
  `FilePos`, `FileLen`.
- `searchPath` (fs.go:147): `{root string, fs iofs.FS, pack *Pack}` — exactly
  one of (`fs`, `pack`) non-nil.
- `SearchResult` (fs.go:120): `Path, Name, SourceFS, IsPack, Pack, FilePos,
  FileLen, Priority`.
- `FileSystem` (fs.go:162): `searchPaths []searchPath` (loose dirs only),
  `lookupPaths []searchPath` (unified stack, front-to-back priority),
  `packs []*Pack`, `gameDir`, `baseDir`, `initialized`.
- `SearchPathEntry` (fs.go:166): debug snapshot for `path` command.

### 2.2 Stack construction (`fs_search.go`)

- `AddGameDirectory(dir)` (fs_search.go:80): builds `loosePath` from
  `os.DirFS(cleanDir)`, discovers `pakN.pak` numerically, and **prepends** a
  per-dir group `[pakN.sorted-desc, ..., pak0, loosePath]` onto `lookupPaths`.
  Net order per dir: higher paks > lower paks > loose files.
- `addEnginePak()` (fs_search.go:27): prepends ironwail.pak above id1/, below
  mods.
- `MountPack(pack)` (fs_search.go:110): prepends a single pack at top priority.
- `Init` (fs.go:215): id1 → ironwail.pak → gamedir, idempotent.

### 2.3 Resolution & reads

- `FindFile` (fs.go:239): iterates `lookupPaths`; loose branch does
  `sanitizePath` + `isWithinRoot` + `iofs.Stat`; pack branch does a linear
  `pf.Lookup == lookupName` scan. Returns `*SearchResult` with `Priority`.
- `FindFirstAvailable` (fs.go:319): same dual loop but tests *all* candidates
  per path level.
- `LoadFile` (fs.go:287) → `loadSearchResult` (fs.go:440): pack via
  `loadFromPack` (seek+ReadFull), loose via `iofs.ReadFile`.
- `OpenFile` (fs.go:301) → `openSearchResult` (fs.go:447): pack via
  `io.NewSectionReader`, loose via `os.Open(Path)`.
- `LoadMapBSPAndLit` (fs.go:399): uses `FindFile` for BSP then `.lit`, **relies
  on `Priority` comparison** (`litResult.Priority > bspResult.Priority` →
  ignore lower-priority lit).
- `ListFiles` (fs_search.go:280): globs `searchPaths` (loose) + scans `packs`
  — a 4th loop, but read-only/separate concern.
- `SearchPathEntries` (fs.go:174): walks `lookupPaths` for the `path` command.

### 2.4 External API surface (must keep compiling unchanged)

Consumers outside `internal/fs` (29 call sites, no test sites use internals):

- `LoadFile` (10), `FileExists` (5), `OpenFile` (3), `LoadMapBSPAndLit` (3),
  `LoadFirstAvailable` (3), `FindFile` (1), `FindFirstAvailable` (1),
  plus `BaseDir`/`GameDir`/`path` entry points.
- `*SearchResult` fields are **not** read externally (only one false-positive
  grep hit: `result.Profile` in a gameplay-debug command is unrelated).

**Conclusion:** `FindFile`/`FindFirstAvailable`/`LoadMapBSPAndLit`/
`SearchResult` are the *public* resolution contract. The overlay FS is an
internal implementation detail; the public API and `Priority` semantics must
be preserved.

## 3. Target Design

### 3.1 `OverlayFS` — a small compose-over-`io/fs.FS` stack

New file `internal/fs/overlay_fs.go`:

```go
// overlayFS resolves a name across an ordered list of io/fs.FS sources.
// Earlier sources win (Quake override order). Each source is either a
// *PakFS (an archive) or an os.DirFS-backed loose dir (via a rootFS type).
type overlayFS struct {
    sources []iofs.FS // priority order: index 0 = highest
}

func (o *overlayFS) Open(name string) (iofs.File, error)      { /* first source that opens wins */ }
func (o *overlayFS) ReadFile(name string) ([]byte, error)     { /* iofs.ReadFile per source */ }
func (o *overlayFS) Stat(name string) (iofs.FileInfo, error)   { /* first source that stats */ }
```

- Implements `iofs.FS`, `iofs.ReadFileFS`, `iofs.StatFS`.
- For `ReadFile`, a missing file in a higher source returns `ErrNotExist` and
  falls through to the next source (same semantics as today: loose `Stat` +
  pack `Lookup` miss both continue).
- Name sanitization + `isWithinRoot` checks stay **above** the overlay (in the
  public `FindFile`), unchanged — the overlay never sees `..`/absolute paths.
- Each mounted loose dir becomes a tiny `rootFS{root, dirfs}` that (a) provides
  `iofs.FS`, and (b) carries `root` for `SearchResult.Path` reconstruction —
  replacing today's `searchPath{root, fs}`. `PakFS` already carries its `*Pack`
  (via `Pack()` accessor) so `SearchResult.Pack` can be recovered per-source.

### 3.2 `FileSystem` state after the change

```go
type FileSystem struct {
    mounts  []mount   // ordered high→low; mount = {rootfs | *PakFS} + who-added-it
    packs   []*Pack   // unchanged: Close() ownership
    gameDir string
    baseDir string
    initialized bool
}
```

- `mount` replaces `searchPath`; `lookupPaths`/`searchPaths` are deleted.
- `SearchPathEntries()` and `ListFiles()` iterate `mounts` (single source of
  truth) instead of the two slice variants.
- The overlay is built lazily on first use (`ensureOverlay()`) or rebuilt when
  `mounts` changes; it simply wraps `mounts` in order. Because `overlayFS`
  holds shared `PakFS`/`rootFS`, updates are cheap and idempotent.

### 3.3 New `FileSystem` internals

- `FindFile(filename)`: sanitize → walk the overlay, ask each source
  `Stat(name)` (loose) or lookup (pack), reconstruct `*SearchResult` from the
  winning source, with `Priority` = the source's index.
  NextBest: keep a tiny per-source `Resolve(name) (SearchResult, found)` so the
  dual loop's logic moves into `PakFS.Resolve` + `rootFS.Resolve` (each knows
  its own `SearchResult` shape). `FindFile` becomes ~15 lines.
- `loadSearchResult`/`openSearchResult`: collapse to `load(overlay, result)`
  — pack reads go through `PakFS.ReadFile`/`Open`; loose through `rootFS`.
  The seek/section-reader logic moves into `PakFS` (single home) instead of
  `loadFromPack`/`openSearchResult` duplicating it.
- `addEnginePak`, `AddGameDirectory`, `MountPack` become `appendMount(mount,
  priority)` — no prepend arithmetic, order computed once.

### 3.4 What survives unchanged (parity constraints)

- Public API (`LoadFile`, `OpenFile`, `FindFile`, `FindFirstAvailable`,
  `LoadFirstAvailable`, `LoadMapBSPAndLit`, `FileExists`, `BaseDir`,
  `GameDir`, `SearchPathEntries`, `ListFiles`, `ListMods`, `Close`).
- `SearchResult` shape + `Priority` meaning (index in mount order).
- Case-insensitive lookup via `canonicalPackLookup` (inside `PakFS`).
- Path sanitization + traversal protection (above the overlay).
- `LoadMapBSPAndLit`'s `Priority`-based lower-priority-lit rejection.

## 4. Step-by-Step Sequence

### Step 19b.1: Introduce `mount` + `rootFS`, keep dual loop (prep)

- **Files**: `internal/fs/fs.go`, `internal/fs/fs_search.go`, `internal/fs/pak_fs.go`
- **Actions**:
  1. Add `rootFS{root string; fs iofs.FS}` with `Open`/`ReadFile`/`Stat` and a
     `Resolve(name) (*SearchResult, bool)` method.
  2. Add `mount` struct: `{fs iofs.FS; pack *PakFS; root string; src searchPathKind}`.
  3. Add `PakFS.Resolve(name) (*SearchResult, bool)` (moves the pack scan
     out of `FindFile`'s loop). Keep `*Pack` recovery via existing `Pack()`.
  4. Repoint `AddGameDirectory`/`addEnginePak`/`MountPack` to append `mount`
     entries (order identical to today's prepend arithmetic — verify with
     existing precedence tests).
  5. **Do not** change `FindFile`/`FindFirstAvailable`/reads yet; they still
     walk `mounts` the same way a smoke test proves identical behavior before
     the loop is deleted.
- **Verify**: `go test ./internal/fs -count=1` (all 20 tests, esp.
  `TestFilesystemSearchPathMatchesQuakePrecedence`,
  `TestPakOverrideOrderIsNumeric`, `TestPathTraversal`,
  `TestLoadMapBSPAndLitIgnoresLowerPriorityLit`).

### Step 19b.2: Build `OverlayFS` and route reads through it

- **Files**: `internal/fs/overlay_fs.go` (new)
- **Actions**:
  1. Implement `overlayFS` (`Open`/`ReadFile`/`Stat` over `mounts` in order).
  2. Add `FileSystem.overlay()` caching `overlayFS{sources: [...]}`.
  3. Replace `loadSearchResult`/`openSearchResult` implementations to call the
     overlay (pack via `PakFS.ReadFile`/`Open`; loose via `rootFS.ReadFile`)
     — deleting `loadFromPack` and `openSearchResult`'s duplicated pack logic
     in favor of `PakFS`.
- **Verify**: `TestOpenFileFromPakReturnsReadSeekHandle`, `TestFilesystemLoadsPak`,
  full fs suite.

### Step 19b.3: Rewrite `FindFile`/`FindFirstAvailable` on the overlay

- **Files**: `internal/fs/fs.go`
- **Actions**:
  1. `FindFile(filename)`: sanitize + `overlay().ResolveAll` → first
     `(*SearchResult, true)` wins; `Priority` = source index.
  2. `FindFirstAvailable`: per source, try all candidates in order (preserve
     today's "candidate list wins within a level" semantics); stop at first hit.
  3. Delete the parallel loose/pack loops.
- **Verify**: `TestPackLookupIsCaseInsensitive`,
  `TestLoadFirstAvailablePrefersSearchPathOverExtensionOrder`,
  full fs suite.

### Step 19b.4: Collapse `searchPaths`/`lookupPaths` consumers

- **Files**: `internal/fs/fs.go`, `internal/fs/fs_search.go`
- **Actions**:
  1. `SearchPathEntries()` iterates `mounts` directly (single source of truth).
  2. `ListFiles(pattern)` globs `mounts` (loose) + `packs` — or, if
     `overlayFS` grows a `Glob`, uses that; keep the 2-loop version minimal.
  3. Delete `searchPath`, `lookupPaths`, `searchPaths` fields; keep `packs`.
  4. Update `doc.go` "High-level design" + inline comments: one overlay stack,
     `io/fs.FS` everywhere.
- **Verify**: `TestListMods*`, `path`/`search` console behavior, full fs suite.

### Step 19b.5: Parity sign-off + documentation

- **Files**: `internal/fs/fs_test.go`, `docs/plans/19_vfs_modernisation_afero.md`,
  `docs/LEARNING_GUIDE.md` (fs row)
- **Actions**:
  1. Keep all 20 existing tests green; add 2–3 focused tests only if a gap
     appears (e.g. overlay `Stat` fallthrough, mixed loose-over-pak override).
  2. Mark 19.2 **DONE** in plan 19 (replacing the "scoped additive" note).
  3. LEARNING_GUIDE: note fs is now a single overlay `io/fs.FS` stack.
- **Verify**: `TMPDIR=.../go test ./internal/fs -count=1`; `mise run verify`.

## 5. Risk Analysis & Mitigations

| Risk | Mitigation |
| --- | --- |
| Override-order regression (subtle, like the earlier black-screen) | Step 19b.1 keeps the existing loops running against `mounts`; precedence tests are already comprehensive; each step lands separately with the full fs suite. |
| `SearchResult.Priority` semantics drift (LoadMapBSPAndLit depends on it) | `Priority` = source index is preserved exactly; `TestLoadMapBSPAndLit*` pins it. |
| `io/fs` interface semantic gaps (e.g. `fs.ErrNotExist` vs custom) | Overlay uses strict `errors.Is(err, iofs.ErrNotExist)` fallthrough, matching today's "miss ⇒ continue" behavior. |
| `MountPack` top-priority prepend | Become `mounts = append([]mount{top}, mounts...)` — one-line, pinned by `TestLoadPackFromBytesAndMount`. |
| Read cursor/locking duplication | All pack reads move into `PakFS` (mutex held inside), deleting `loadFromPack`/`openSearchResult`'s pack branches — removes rather than adds duplication. |
| `openSearchResult`'s `*os.File` vs `PakFS.Open` streaming shape | `PakFS.Open` returns an `iofs.File`; `OpenFile` wraps it with the same `ReadSeekCloser` contract; `TestOpenFileFromPakReturnsReadSeekHandle` covers it. |

## 6. Anti-goals (explicitly out of scope)

- No new external dependency (no `afero`; stdlib `io/fs` only).
- **No public API change**: `SearchResult`, `LoadFile`, `OpenFile`, etc. keep
  signatures; external callers are untouched.
- No WASM/in-memory overlay work (the `LoadPackFromBytes` path already works
  through `PakFS`; a future `MemMapFS` can mount the same way — note in plan,
  not built here).
- No change to `sanitizePath`/`isWithinRoot` traversal semantics.

## 7. Definition of Done

- `internal/fs` has exactly **one** resolution path: the `OverlayFS` over
  `mounts` (loose `rootFS` + `PakFS`), with no `lookupPaths`/`searchPaths`
  dual loops and no duplicated pack-read logic.
- All 20 existing `internal/fs` tests pass unchanged, plus `mise run verify`.
- `doc.go` + comments describe the single overlay design (educational clarity).
- Plan 19.2 flips from "deferred/additive" to DONE with a link to this plan.
