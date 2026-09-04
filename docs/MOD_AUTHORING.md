# Mod Authoring Guide (Ironwail-Go SDK)

This guide walks through building a standalone mod or total conversion on
the Ironwail-Go engine using the `qcmod` toolkit (beads `ironwail-go-bvw`
SPEC-006 and `ironwail-go-uxr`). For QuakeGo gameplay language details see
`docs/QGO_QUAKEGO_GUIDE.md` and `docs/QGO_SPEC.md`; for the C source your
gameplay should stay faithful to, see `pkg/qgo/quakego` (the mechanical
QuakeC port).

## The SDK surface

A mod is an ordinary Go module. It imports one public package:

```go
import "github.com/darkliquid/ironwail-go/sdk"
```

`sdk` re-exports the engine's internal bootstrap (`sdk.Config`,
`sdk.Run`) so mod binaries never touch `internal/` packages, which Go's
internal-package rule forbids outside the engine module. The scaffolded
`main.go` looks like this:

```go
package main

import (
	"os"

	"github.com/darkliquid/ironwail-go/sdk"
)

func main() {
	config := newGameConfig()
	g, err := sdk.Run(config, sdk.Args(os.Args...))
	if err != nil {
		panic(err)
	}
	_ = g // the engine owns the game loop
}
```

`sdk.Config` is `gameconfig.Config`: game name, base data directory,
registration gate, network identity, default cvar values, and menu labels.
Zero-value fields resolve to stock Quake defaults, so a mod only overrides
what it cares about.

## Scaffolding a mod

```sh
qcmod init [-kind generic|sp|dm|tc] [-engine <path>] <moddir>
```

Template kinds:

| Kind | What you get |
| --- | --- |
| `generic` | Minimal standalone mod (registration gate off, registered on) |
| `sp` | Single-player starter: spawn/think stubs, config gate off |
| `dm` | Deathmatch-first config (`DefaultDeathmatch: 1`) + item respawn stubs |
| `tc` | Total conversion: full identity override (menu labels, CSQC name, base dir) |

The generated directory is a complete Go module:

```
<moddir>/
  go.mod          module + replace directives for the engine and quake modules
  main.go         entry point via sdk.Run
  gameconfig.go   sdk.Config pre-populated from the directory name
  progs/progs.go  QuakeGo gameplay sources (compiled to progs.dat by cmd/qgo)
  game_test.go    headless simulation tests (quake/sim)
  Makefile        build / run / test targets
  README.md       layout and data placement
```

`qcmod init` resolves the engine checkout automatically (from its own build
location or `$IRONWAIL_GO_ROOT`); pass `-engine <path>` to override. The
`replace` directives are written relative to the mod directory so the
scaffold is portable.

First build:

```sh
cd <moddir>
go mod tidy     # resolves quake + engine replaces (GOWORK=off)
make test       # go test ./... — runs the sim tests, no engine boot
make build      # builds the game binary
make run        # runs from the parent dir so ./<moddir> resolves as data
```

> **Data placement.** The engine mounts the game's data as
> `./<BaseGameDir>` relative to the working directory (the mod name for a
> TC). `make run` therefore runs the binary from the *parent* of the mod
> directory, or you can drop a `<moddir>/` data directory next to the
> binary. Game data means `progs.dat`, `maps/`, `gfx/`, and any paks.

Verify the whole SDK cycle in CI with `mise run verify-mod-sdk` (scaffolds
every kind, then `go mod tidy` + `go test` + `go build` inside each).

## Gameplay: QuakeGo + simulation tests

Gameplay lives in `progs/` as QuakeGo (a Go dialect compiled to QCVM
bytecode by `cmd/qgo`, see `docs/QGO_QUAKEGO_GUIDE.md`). The same sources
build as plain Go, which is what makes headless simulation tests possible:

```go
// game_test.go
package main

import (
	"testing"

	"quake"
	"quake/sim"
)

func TestDoorSchedulesMove(t *testing.T) {
	w := sim.New()
	door := w.Spawn("func_door")
	door.Think = func() {
		door.Velocity = quake.MakeVec3(0, 0, 100)
		door.NextThink = w.Time + 1.0
	}
	if err := w.Fire(door, nil, door.Think); err != nil {
		t.Fatal(err)
	}
	if door.NextThink != w.Time+1.0 {
		t.Fatalf("nextthink = %v", door.NextThink)
	}
}
```

`qcmod test <moddir>` wraps `go test` on the mod; `qcmod vm <fn>`,
`qcmod disasm`, and `qcmod dap` operate on the compiled `progs.dat`.

## Packaging data: `.pak` archives

`qcmod pak` creates, extracts, lists, and validates Quake PAK archives:

```sh
qcmod pak pack <datadir> -o pak0.pak      # create (names validated: 56-byte,
                                          # slash-only, no traversal)
qcmod pak list pak0.pak                   # sizes + names, sorted
qcmod pak check pak0.pak                  # verify table + byte ranges
qcmod pak unpack pak0.pak -o <dest>       # extract (re-validates names)
```

Conventions: name archives `pak0.pak`, `pak1.pak`, … — the engine mounts
packs in override order (later paks take priority). The writer sorts entries
and produces byte-deterministic archives. A total conversion bundles
`progs.dat` + maps from its data dir into `pak0.pak` under its `BaseGameDir`.

## Textures: `.wad` from images

`qcmod wad` converts PNG/TGA images into Quake WAD lumps:

```sh
qcmod wad -o gfx.wad -type qpic menu.png title.png     # HUD/menu art (QPic)
qcmod wad -o textures.wad wall.png floor.png           # world textures
                                                       # (auto-detects MipTex
                                                       # for 16-multiple sizes)
qcmod wad -type miptex -palette mygame/palette.lmp -o t.wad stone.png
```

- **QPic lumps** (HUD/menu art) are width/height + palette-index pixels.
- **MipTex lumps** (world textures) carry four box-downsampled mip levels;
  inputs must be multiples of 16 (the classic Quake texture constraint).
- Pixel encoding quantises RGBA to the Quake palette via nearest colour;
  pixels with alpha < 128 map to the transparent index (255).
- Palette source: `-palette <palette.lmp>` (768 bytes) or the built-in
  Quake palette by default.

`cmd/wadgen` remains available: with no images it emits the legacy
placeholder WAD; with images it performs the same conversion (deprecated in
favour of `qcmod wad`).

## Roadmap hooks (SPEC-006 §11, SPEC-007 §11)

The SDK surface grows with the extensibility architecture: subsystem
registry, command restriction, and the post-FX registry (`SPEC-007 §11.4`)
will be exposed as additional `sdk` hooks — the `qcmod init` template
already keeps `main.go` ready for `hooks.PostFX.Register(...)`-style calls
after `sdk.Run`.