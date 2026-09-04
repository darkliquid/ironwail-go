# Mod Scaffolding and Asset Packaging Tooling — Implementation Plan

> **Bead:** `ironwail-go-uxr` · **Status:** plan (draft) · **Date:** 2026-09-04
> **Related:** SPEC-006 (engine SDK, M5/M6 landed), SPEC-007 §11.4/§11.5 (uxr
> `qcmod init` post-FX hooks), docs/QGO_QUAKEGO_GUIDE.md, plan 25 (qcmod).
>
> **For agentic workers:** implement task-by-task; each Step uses checkbox
> (`- [ ]`) syntax. Run the listed test command after every step.

**Goal:** Turn `qcmod` into the complete mod-authoring workbench for the bead's
acceptance criteria: (1) a scaffolding generator that produces a
*ready-to-build* QuakeGo mod directory (SP / DM / TC templates, correct
`go.mod` wiring), (2) a PAK archive pack/unpack CLI, and (3) a WAD texture
generator that converts PNG/TGA images into Quake WAD lumps (QPic for HUD,
MipTex for world textures).

**Architecture:** Everything lives in the existing `cmd/qcmod` CLI (the
established mod-toolkit front door from plan 25); no separate `ironwailgo
init-mod` binary. The scaffold is built on the already-landed engine SDK entry
point (`internal/engine.Run`, SPEC-006 M6) via a **new public `sdk` package**
that re-exports it — this is required because the current scaffold imports
`internal/engine` from an external module, which the Go compiler rejects.
Binary-format writers are added next to their existing readers: PAK writing in
`internal/fs` (reader already shipped), WAD writing in `internal/image`
(reader + QPic/MipTex parsing already shipped). The placeholder-only
`cmd/wadgen` is folded into `qcmod wad` and kept as a delegating wrapper.

**Tech Stack:** Go 1.26, pure Go (`CGO_ENABLED=0`), `flag`,
`embed`, `text/template`, `image/png` (decode), `internal/fs` (PAK reader +
new writer), `internal/image` (WAD/QPic/MipTex reader + new writers),
`pkg/qgo/quake` + `quake/sim` (scaffold gameplay + tests).

**Byte-format oracles (cross-validation with C Ironwail):** PAK header/entry
layout from `Quake/common.c` (`COM_LoadPackFile`); WAD2 layout from
`Quake/wad.c` (`W_LoadWadFile`); MipTex from `Quake/model.h` (`miptex_t`);
QPic from `Quake/wad.c` (`qpic_t`). Canonical docs: `docs/QUAKE_SPECIFICATION.md`,
`docs/internal/fs.md`, `docs/internal/image.md`.

---

## Current-state audit (verified 2026-09-04)

| Area | State | Evidence |
|---|---|---|
| SDK entry point | ✅ Landed | `internal/engine/run.go` — `engine.Run(config, opts...)`, `Headless()`, `Args()`. |
| `qcmod init` | ⚠️ Broken scaffold | `cmd/qcmod/init.go` emits `replace quake => %s` (literal `%%s` bug → invalid go.mod); **no engine module require/replace**, so `main.go` cannot resolve `internal/engine`; empirically `go build` fails: `use of internal package github.com/darkliquid/ironwail-go/internal/engine not allowed`. No `init_test.go`, no `Makefile` (SPEC-006 §6 promised one), templates are Go constants, not `//go:embed`. |
| Mod simulation | ✅ Landed | `pkg/qgo/quake/sim` — `sim.New()`, `(*World).Spawn(classname)`, used by `qcmod test`. |
| PAK writer | ❌ Missing | `internal/fs` reads packs only (`LoadPackFromBytes`, `loadPackFromHandle`, `PakFS`, `canonicalPackLookup`); zero write paths in the repo (tests write literal `"fake"` bytes). |
| PAK reader | ✅ Landed | `internal/fs/pak_fs.go`, `fs_search.go` — "PACK" magic, `DirOfs`/`DirLen` header, case-insensitive lookup. |
| WAD writer | ⚠️ Placeholder | `cmd/wadgen` writes only dummy QPic lumps + grayscale palette; no image input, no MipTex, no palette quantization. |
| WAD/QPic/MipTex reader | ✅ Landed | `internal/image/wad.go` (`LoadWad`, `ParseQPic`, `CleanupName`, `TypMipTex=68`), `internal/image/texture.go` (`MipTex`, `ParseMipTex`, `MipLevel`). |
| Palette | ⚠️ None | No Quake `palette.lmp` (768 bytes) in repo; `internal/image/export.go` has `RGBAFromPalette` (indexed→RGBA) but no inverse (RGBA→indexed). |
| MDL/sprite readers | ✅ Landed | `internal/model/mdl.go`, `sprite.go` (readers); no writers. |

**Key finding:** the SDK import surface is the first blocker. An external mod
module (`module uxr-probe`) importing `github.com/darkliquid/ironwail-go/internal/engine`
is rejected by Go's internal-package rule regardless of `replace` directives
(verified). The scaffold must import a **public** package instead.

---

## Task 1: Public SDK package + buildable `qcmod init`

### Task 1.1 — Add public `sdk` re-export package

**Files:**
- Create: `sdk/sdk.go`
- Create: `sdk/sdk_test.go`
- Create: `sdk/doc.go`

The package is a thin, public facade over the internal SDK surface. Public
packages inside the engine module are allowed to import `internal/...`, which
is exactly the permission an external mod module lacks.

```go
// Package sdk is the public entry point for standalone mods and total
// conversions built on the Ironwail-Go engine. A mod's main() constructs a
// Config, calls sdk.Run, and gets a runnable game. All types re-exported
// from internal packages; the surface grows as SPEC-006 §11 hooks land.
package sdk

import (
	"github.com/darkliquid/ironwail-go/internal/engine"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

// Config is a mod's identity and feature configuration.
type Config = gameconfig.Config

// Option configures the engine bootstrap.
type Option = engine.RunOption

// Headless runs without rendering (dedicated server or automated testing).
func Headless() Option { return engine.Headless() }

// Args passes command-line arguments (same format as the engine binary).
func Args(args ...string) Option { return engine.Args(args...) }

// Run boots the engine with the given configuration.
func Run(config Config, opts ...Option) (*game.Game, error) {
	return engine.Run(config, opts...)
}
```

- [ ] **Step 1: Write the failing test** — `sdk/sdk_test.go`: `sdk.Config{}`
  resolves via `gameconfig.Default()` semantics; `sdk.Args("a","b")` and
  `sdk.Headless()` produce options that `engine.Run` accepts (assert on a
  minimal headless boot with a temp basedir, reusing the pattern from
  SPEC-006 M6's engine test if one exists; otherwise boot with
  `InitSubsystems` headless path only).
  Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./sdk -count=1`
- [ ] **Step 2: Implement `sdk/sdk.go`/`doc.go`** as sketched; confirm `go vet ./sdk`.
- [ ] **Step 3: Cross-check** that `pkg/qgo/quake` module (`quake`, `quake/sim`)
  is NOT required by the `sdk` package (mods import it directly via their own
  go.mod replace, as `pkg/qgo/quakego` does today).

### Task 1.2 — Fix `qcmod init` go.mod and engine-path resolution

**Files:**
- Modify: `cmd/qcmod/init.go`
- Create: `cmd/qcmod/init_test.go`

- [ ] **Step 1: Write the failing test** (`init_test.go`): run `runInit` into a
  temp dir; assert the generated `go.mod` parses (`go mod edit -json`) and
  contains BOTH `require github.com/darkliquid/ironwail-go v0.0.0` AND
  `require quake v0.0.0`, with `replace` directives for both; assert `main.go`
  imports only public packages (`ironwail-go/sdk`, not `internal/...`).
  Run: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./cmd/qcmod -run TestInitGoMod -count=1`
- [ ] **Step 2: Fix the templates.** Replace the broken
  `fmt.Sprintf(goModTmpl, base)` / `replace quake => %%s` scheme with
  `text/template` over a struct `{Module, GameName, BaseDir, EngineRoot}`.
  Generated `go.mod`:

  ```
  module <basename>

  go 1.26

  require (
      github.com/darkliquid/ironwail-go v0.0.0
      quake v0.0.0
  )

  replace github.com/darkliquid/ironwail-go => <engine-root>

  replace quake => <engine-root>/pkg/qgo/quake
  ```

  Update `main.go`, `gameconfig.go`, `progs/progs.go`, `game_test.go`
  templates to import `ironwail-go/sdk` (alias `sdk.Config`), keeping the
  existing `newGameConfig()` structure.
- [ ] **Step 3: Engine-root resolution.** `qcmod init -engine <path>` flag;
  default resolved in order: (1) walk up from `runtime.Caller(0)` source dir
  until a `go.mod` declaring `module github.com/darkliquid/ironwail-go` is
  found; (2) `$IRONWAIL_GO_ROOT`; (3) error with a hint. Emit the `replace`
  as a **relative** path from the mod dir to the engine root when they share
  a root (portable scaffold), otherwise absolute (warn).
- [ ] **Step 4: Test** the full static contract: registry of expected files
  per template kind, exact template variable substitution, `titleCase`
  behavior ("my-mod" → "MyMod"), engine-root fallback ordering.

### Task 1.3 — Template kinds, embedded templates, Makefile

**Files:**
- Create: `cmd/qcmod/template/generic/…`, `cmd/qcmod/template/sp/…`,
  `cmd/qcmod/template/dm/…`, `cmd/qcmod/template/tc/…`
- Create: `cmd/qcmod/template.go` (`//go:embed template/*`)
- Modify: `cmd/qcmod/init.go`, `cmd/qcmod/main.go`

- [ ] **Step 1: Write the failing test** — `qcmod init -kind sp|dm|tc` each
  produce the expected tree; `-kind` is validated (unknown → exit 2).
- [ ] **Step 2: Move every template** into `cmd/qcmod/template/<kind>/…` and
  load via `//go:embed` (offline, per SPEC-006 §6). Generic = current template
  (standalone single-player starter with `RequireRegistered:false`,
  `DefaultRegistered:true`).
- [ ] **Step 3: Kind-specific content.**
  - **sp** — adds `progs/spawn.qc.go` + `progs/think.qc.go` samples (a
    `info_player_start` plus a simple trigger stub), `game_test.go` sim
    assertions; config identical to generic.
  - **dm** — `gameconfig.go` adds `DefaultDeathmatch:1`; `progs/` adds an item
    respawn stub (`self.think` re-spawn pattern mirroring `pkg/qgo/quakego`
    item code); `game_test.go` asserts deathmatch defaults resolve.
  - **tc** — full identity override: `GameName`, `BaseGameDir` (title-cased
    name), custom menu labels `ModDirMenuLabel`/`NetOptionLabel`, protocol
    identity note in comments; `RequireRegistered:false`,
    `DefaultRegistered:true`; README explains mounting `pak0.pak` under the
    TC's `BaseGameDir`.
- [ ] **Step 4: Makefile** (all kinds): `build` (`go build -o <bin> .`),
  `run` (`go run .`), `test` (`go test ./...`), `clean`. Written as a template
  file so `<bin>` = module basename.
- [ ] **Step 5: End-to-end scaffold build test** (opt-in): env
  `QC_E2E_BUILD=1` gates a test that runs `qcmod init -kind tc <tmp>`,
  `go mod tidy`, `go build ./...`, `go test ./...` inside the generated dir
  (engine deps resolve from the local module cache; slow, so skipped by
  default). Wire `QC_E2E_BUILD=1` into `mise run verify` (or a new
  `mise run verify-mod-sdk` task) so CI exercises it without slowing daily
  `go test ./...`.

---

## Task 2: PAK writer + `qcmod pak`

### Task 2.1 — Writer in `internal/fs`

**Files:**
- Create: `internal/fs/pak_write.go`
- Create: `internal/fs/pak_write_test.go`

```go
// PakEntry is a single file to be stored in a PAK archive.
type PakEntry struct {
	Name string // virtual path, forward slashes, <= 56 bytes, NUL-free
	Data []byte
}

// ValidPakName validates a Quake PAK file name: non-empty, no leading '/',
// no ".." path elements, no backslashes, <= 56 bytes (the on-disk field).
func ValidPakName(name string) error

// WritePack writes a PAK archive: 12-byte "PACK" header, file data
// sequentially, then the 64-byte-per-entry directory table. Entries are
// sorted (deterministic byte-identical output for tests). Mirrors
// COM_LoadPackFile's on-disk layout so the existing reader round-trips.
func WritePack(w io.Writer, entries []PakEntry) error
```

- [ ] **Step 1: Write the failing test** — round-trip: `WritePack` →
  `LoadPackFromBytes` → `PakFS.ReadFile` returns identical bytes for several
  nested paths; deterministic output (same input ⇒ same bytes); `ValidPakName`
  acceptance/rejection table (56-byte boundary, backslash, `../`, empty,
  `a//b`, leading slash); duplicate names rejected.
  Run: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/fs -run TestPak -count=1`
- [ ] **Step 2: Implement** `ValidPakName`, `WritePack` (header
  `binary.LittleEndian`: magic `"PACK"`, uint32 dir offset, uint32 dir length;
  entries: 64 bytes = uint32 offset + uint32 size + 56-byte NUL-padded
  name). Sort entries by name; names normalized lowercase to match
  `canonicalPackLookup`. No 4-byte file alignment (engine reads via offsets —
  verify the C reader in `common.c` tolerates this, as stock pak0 does).
- [ ] **Step 3: C cross-validation** — byte-compare against a real
  tiny archive built with stock `pak`/ironwail tooling if available
  (`docs/PARITY.md` conventions); otherwise assert structural parity only
  (magic, dir table parse, data offsets) and record the gap.

### Task 2.2 — `qcmod pak` subcommands

**Files:**
- Create: `cmd/qcmod/pak.go`
- Create: `cmd/qcmod/pak_test.go`
- Modify: `cmd/qcmod/main.go`

- [ ] **Step 1: Write the failing tests** for all four verbs (fixtures built
  procedurally in temp dirs — no repo asset files, dodging the `.gitignore`
  whitelist):
  - `qcmod pak pack <dir> -o out.pak`: recursive walk, skips dirs, validates
    every name, writes archive; error on invalid name with file listed.
  - `qcmod pak unpack <pak> -o <dir>`: reads via existing
    `LoadPackFromBytes`/`PakFS`, re-validates every name before writing
    (traversal-safe: reject `..`/absolute), restores directory tree.
  - `qcmod pak list <pak>`: prints sorted `size name` lines.
  - `qcmod pak check <pak>`: parses, verifies magic, duplicate-free sorted
    table, every offset+size within bounds.
  - Round-trip: pack(src dir) → unpack → file tree compares equal to src.
  Run: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./cmd/qcmod -run TestPak -count=1`
- [ ] **Step 2: Implement** `runPak(args, stdout, stderr)` dispatching
  `pack|unpack|list|check` (aliases `p`). Wire into `main.go` switch and the
  `docs` usage text.
- [ ] **Step 3: Mod conventions docs** — document recommended naming
  (`pak0.pak`, `pak1.pak` …) and engine mount order (`internal/fs` searches
  packs in override order), plus how a scaffolded mod's `Makefile` `pak`
  target should bundle `progs.dat` + maps.

---

## Task 3: WAD texture generator

### Task 3.1 — Writers and palette in `internal/image`

**Files:**
- Create: `internal/image/wad_write.go`
- Create: `internal/image/wad_write_test.go`
- Modify: `internal/image/texture.go` (add `MipChain`, validate sizes)

```go
// WadLump is a single lump for WriteWad.
type WadLump struct {
	Name string   // cleaned via CleanupName, <= 15 bytes for miptex
	Type LumpType // TypQPic or TypMipTex
	Data []byte
}

// WriteWad writes a WAD2 archive (12-byte header + 32-byte LumpInfo
// directory + lump data), mirroring W_LoadWadFile's layout so LoadWad
// round-trips.
func WriteWad(w io.Writer, lumps []WadLump) error

// WriteQPicLump serialises a QPic (uint32 w, uint32 h, w*h palette-index
// bytes) from RGBA pixels using the given palette.
func WriteQPicLump(name string, rgba []byte, w, h int, pal [256]color.RGBA) ([]byte, error)

// WriteMipTexLump serialises a MipTex (16-byte name, uint32 w/h, 4 uint32
// offsets, 4 box-downsampled palette-index mip levels). Requires w,h
// divisible by 16 (classic Quake constraint; MipLevel slices by shift).
func WriteMipTexLump(name string, rgba []byte, w, h int, pal [256]color.RGBA) ([]byte, error)

// EncodePaletted quantises RGBA to palette indices via nearest-color
// (RGB distance, alpha<128 → index 0/transparent handling documented).
func EncodePaletted(rgba []byte, w, h int, pal [256]color.RGBA) []byte

// LoadPaletteLmp parses a 768-byte palette.lmp into [256]color.RGBA.
func LoadPaletteLmp(data []byte) ([256]color.RGBA, error)
```

- [ ] **Step 1: Write the failing tests** — for each writer: generate a
  synthetic RGBA image + synthetic palette in-test (via
  `internal/image/export.go` `WritePNG` round-trip for the PNG path),
  write lump, then `LoadWad` → `ParseQPic` / `ParseMipTex` and compare
  paletted pixels; miptex mip dims (w,h, w/2,h/2, w/4,h/4, w/8,h/8) and
  offset monotonicity; non-multiple-of-16 miptex input errors; QPic name
  cleanup (`"My Pic"` → `"my pic"`), miptex 16-byte name truncation;
  `LoadPaletteLmp` rejects wrong sizes.
  Run: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/image -run TestWadWrite -count=1`
- [ ] **Step 2: Implement** writers; add `MipChain(pixels, w, h)` box-filter
  downsampler in `texture.go` (or reuse an existing filter if one exists —
  check `internal/image/tga.go`/`export.go` first). Add
  `RGBAFromPalette`'s inverse `EncodePaletted`.
- [ ] **Step 3: C cross-validation** — miptex header byte layout vs
  `model.h miptex_t` (name[16], w, h, 4 offsets, 16-colour header palette
  region is *not* part of the on-disk miptex in Quake — confirm and match
  what `ParseMipTex` expects).

### Task 3.2 — `qcmod wad` + `wadgen` delegation

**Files:**
- Create: `cmd/qcmod/wad.go`
- Create: `cmd/qcmod/wad_test.go`
- Modify: `cmd/qcmod/main.go`, `cmd/wadgen/main.go`

- [ ] **Step 1: Write the failing tests** —
  `qcmod wad -o out.wad -type qpic a.png` produces a WAD loadable via
  `image.LoadWad` with one QPic lump named `a`; `-type miptex` with a
  64×64 PNG produces a MipTex lump whose four mips parse; `-type` from
  extension/`-type auto` inference; palette resolution order
  (`-palette` flag → `$QUAKE_DIR/gfx/palette.lmp` → embedded grayscale
  fallback, matching today's `wadgen`); PNG and TGA inputs (TGA via
  `internal/image/tga.go`).
  Run: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./cmd/qcmod -run TestWad -count=1`
- [ ] **Step 2: Implement** `runWad(args, stdout, stderr)`: decode inputs
  (`image.Decode` for PNG — note: allow `image/png` + `internal/image/tga.go`
  only; no GIF/JPEG in v1), resize-or-reject policy for miptex
  (reject non-multiple-of-16 with a clear message; `-resize` is a noted
  follow-up), name from basename via `image.CleanupName`, write via
  `internal/image` writers. Wire into `main.go` + `docs` text.
- [ ] **Step 3: `cmd/wadgen` becomes a wrapper** — same binary semantics;
  with no image args it keeps emitting the current placeholder QPic + palette
  (back-compat for anything relying on it); with image args it delegates to
  shared writers. Mark deprecated in its doc comment, pointing at
  `qcmod wad`.

---

## Task 4 (P3, deferred to a follow-up bead): MDL/sprite converters

**Explicitly out of v1 scope** — the bead description mentions
OBJ/glTF → MDL/sprite, but acceptance criteria require only scaffolding, PAK,
and WAD. `internal/model/mdl.go`/`sprite.go` are readers; writing these
formats (alias MDL frames/normals/skins, paletted sprites) is a substantial
project on its own.

Design sketch for the follow-up (recorded in this plan so the architecture
stays compatible):
- **Files (future):** `internal/model/mdl_write.go`, `sprite_write.go`,
  `cmd/qcmod/model.go`.
- OBJ → MDL: triangle soup → grouped frames, vertex quantisation to the
  Quake vertex table (`[256][3]byte` normals), skin = `EncodePaletted`
  re-use from Task 3, auto-generate `id_` frame names like stock tools.
- glTF → MDL: skeletal animation bakes keyframes into alias frames (bones
  merged per frame); glTF parsing needs a new dependency (e.g.
  `github.com/qmuntal/gltf`) — a dependency-introduction decision to
  revisit before implementation.
- Sprite → paletted `spr` (3-frame, alpha from RGBA) reuses the same
  palette pipeline.
- **Gate:** propose a new bead (`ironwail-go-<new>`) scoped to "model
  converters"; reuse `sdk`/pak/wad groundwork. Do not start before M1–M5
  ship.

---

## Task 5: Docs, integration, bead close

**Files:**
- Modify: `cmd/qcmod/main.go` (`docs` usage text), `docs/QGO_QUAKEGO_GUIDE.md`
- Create: `docs/MOD_AUTHORING.md`
- Modify: `.gitignore` (only if repo fixtures are added — prefer procedural
  testdata, in which case no change)

- [ ] **Step 1: `qcmod docs` + usage text** lists `init [-kind]`, `pak`,
  `wad` with one-line descriptions.
- [ ] **Step 2: `docs/MOD_AUTHORING.md`** — the full cycle: `qcmod init
  -kind tc mygame` → `go mod tidy` → `go test ./...` → `make run`; adding
  QuakeGo gameplay; bundling `progs.dat` + maps into `pak0.pak` with
  `qcmod pak pack`; generating `gfx.wad`/`textures.wad` with `qcmod wad`;
  the `sdk` package contract and the SPEC-006 §11 extensibility roadmap
  (post-FX hook from SPEC-007 §11.4 exposed through the same SDK surface).
- [ ] **Step 3: Cross-link** SPEC-006/007 (update their "future" notes where
  they reference uxr: SPEC-007 §11.4/§11.5 `PostFXRegistry` exposes its
  registration API through `sdk` hooks).
- [ ] **Step 4: Quality gates** — `mise run lint`, `mise run verify`,
  `QC_E2E_BUILD=1 mise run verify-mod-sdk` (new task), then close the bead
  per the acceptance mapping below.

---

## Acceptance criteria mapping & milestones

| M | AC | Scope | Key files |
|---|---|---|---|
| M1 | — | `sdk` package + `qcmod init` go.mod/engine-path fixes + golden tests | `sdk/`, `cmd/qcmod/init.go`, `init_test.go` |
| M2 | AC1 | template kinds (generic/sp/dm/tc), `go:embed`, Makefile, e2e scaffold test | `cmd/qcmod/template/*`, `template.go` |
| M3 | AC2 | PAK writer + `qcmod pak pack/unpack/list/check` | `internal/fs/pak_write.go`, `cmd/qcmod/pak.go` |
| M4 | AC3 | WAD writers + palette + `qcmod wad`, `wadgen` delegation | `internal/image/wad_write.go`, `cmd/qcmod/wad.go` |
| M5 | all | docs, usage text, quality gates, bead close | `docs/MOD_AUTHORING.md`, `mise.toml` |
| M6 | (desc) | OBJ/glTF → MDL/sprite converters — **P3, follow-up bead** | (deferred) |

AC1 = "CLI scaffolding generator creating a ready-to-build QuakeGo mod
directory" (SP/DM/TC templates, valid go.mod, engine SDK wiring, Makefile,
e2e build-proves-ready).
AC2 = "CLI for creating/extracting .pak archives" (`qcmod pak`).
AC3 = "WAD texture generator from images" (`qcmod wad`, QPic + MipTex).

## Testing strategy (summary)

1. **Unit:** `sdk`, `internal/fs` (pak round-trip/determinism/validation),
   `internal/image` (wad/qpic/miptex/palette round-trips) — all offline,
   deterministic, no pak0 required (`SkipIfNoQuakeDir` not needed; synthetic
   fixtures in-test).
2. **CLI:** `cmd/qcmod` tests for `init` (file-tree golden + go.mod contract),
   `pak` (pack→unpack→compare), `wad` (load-back assertions).
3. **E2E (opt-in):** `QC_E2E_BUILD=1` scaffold → `go mod tidy` → `go build` →
   `go test` inside the generated mod; wired into a `mise run` verify task.
4. **Conventions:** every test prefixed per package (TestInit*, TestPak*,
   TestWadWrite*, TestWad, TestSdk*); run with
   `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test … -count=1`.

## Risks & mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Internal-package gate blocks scaffold (verified) | Scaffold cannot build | Public `sdk` re-export package (Task 1.1); mods never import `internal/*` |
| Generated mod pulls heavy engine deps (gogpu/oto/gotracker) via `sdk` | Slow `go mod tidy`/build in the mod dir; large binaries | Accepted for v1 (engine is monolithic); e2e test gated behind `QC_E2E_BUILD`; note a "slim SDK profile" (headless core without renderer) as follow-up |
| Template drift vs `pkg/qgo/quakego` | SP/DM samples diverge from canonical QuakeC port | Sample progs follow quakego patterns; e2e test compiles them; cross-check with `progs.src` where relevant |
| Quake palette licensing (palette.lmp from id data) | Can't embed real palette in repo | Default reads `$QUAKE_DIR/gfx/palette.lmp`; embedded fallback is grayscale (current `wadgen` behaviour); `-palette` for explicit input |
| `.gitignore` whitelist ignores non-`.go` files | New fixtures silently untracked | Procedural in-test fixtures only; if repo files become necessary, add explicit allow rules (as done for `testdata/parity/`) |
| PAK format edge cases (56-byte names, case, dupes, traversal) | Broken archives / zip-slip on unpack | `ValidPakName` + `pak check` + traversal-safe extraction + table-driven unit tests |
| Go toolchain requirements inside tests (module cache, network) | Flaky CI | Replace-directive-only resolution; offline cache; TMPDIR convention; heavy tests env-gated |