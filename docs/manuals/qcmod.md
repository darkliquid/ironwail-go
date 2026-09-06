# qcmod Manual

qcmod is the mod development toolkit for Ironwail-Go. It creates mod projects, runs gameplay tests, inspects compiled game programs, packages game data, and starts a debugger. You can use most of it without booting the full engine.

This manual covers every command of qcmod. For the game logic language, read the QuakeGo guide. For the debugger, read the DAP manual.

## The commands at a glance

| Command | What it does |
| --- | --- |
| `qcmod init` | Creates a new mod project from a template |
| `qcmod test` | Runs the gameplay tests of a mod |
| `qcmod vm` | Runs one game function in the game VM |
| `qcmod sim` | Opens an interactive test console (work in progress) |
| `qcmod disasm` | Prints the disassembly of a compiled game program |
| `qcmod qc2go` | Converts QuakeC source to QuakeGo source |
| `qcmod pak` | Creates, extracts, and lists PAK archives, and tests whether they are sound |
| `qcmod wad` | Converts images into a Quake WAD |
| `qcmod dap` | Starts the standalone debug server |
| `qcmod docs` | Prints the built-in command guide |

## Getting qcmod

Build it from the repository root:

```sh
go build -o qcmod ./cmd/qcmod
```

You can also run `mise run qcmod-sim`, which builds the tool into `bin/qcmod`.

## Creating a project with qcmod init

`qcmod init` writes a complete mod project into a new directory. It is the fastest way to start a game.

```sh
qcmod init -kind tc mygame
```

The `-kind` flag selects a template:

| Kind | What you get |
| --- | --- |
| `generic` | A minimal standalone mod |
| `sp` | A single-player starter with spawn and think stubs |
| `dm` | A deathmatch-first setup with item respawn stubs |
| `tc` | A total conversion with a full identity override |

The generated directory is a Go module with this layout:

```text
mygame/
  go.mod          module file with replace directives
  main.go         entry point that calls the SDK
  gameconfig.go   game settings in code
  progs/progs.go  gameplay sources in QuakeGo
  game_test.go    simulation tests
  Makefile        build, test, and run targets
  README.md       layout and data placement notes
```

The tool finds the engine checkout automatically. If it cannot, set `IRONWAIL_GO_ROOT` or pass the path:

```sh
qcmod init -kind tc -engine /path/to/ironwail-go mygame
```

The generated `go.mod` uses `replace` directives that point at the engine and the Quake module. The paths are relative, so the project stays portable.

Build and run the project:

```sh
cd mygame
go mod tidy
make test
make build
make run
```

Run the binary from the parent directory. The engine mounts game data at `./mygame` relative to the working directory.

## Testing gameplay with qcmod test

Gameplay tests are ordinary Go tests. They import the simulation package and run without `progs.dat`, without a GPU, and without game assets.

```sh
qcmod test mygame
```

The command wraps `go test` on the mod directory. A test creates a `sim.New()` world, spawns entities, fires their functions, and asserts fields:

```go
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

The mod must have a `go.mod` that resolves the Quake module with a `replace` directive. The generated project already has one.

## Running bytecode with qcmod vm

`qcmod vm` boots the compiled game program and fires one function in it. It sets the game globals first, then prints the result.

```sh
qcmod vm <function> [self [other [time]]]
```

- `<function>`: the name of the function to run.
- `self`: the entity number to place in the `self` global.
- `other`: the entity number for the `other` global.
- `time`: the game time in seconds.

The output lines with `ok <function> self=.. other=.. time=.. spawned=..`.

## Using the interactive console with qcmod sim

`qcmod sim` opens a headless console for one mod directory. It is a work in progress.

```sh
qcmod sim mygame
```

The console accepts these commands:

| Command | What it does |
| --- | --- |
| `run <fn> [self [other [time]]]` | Runs a function like `qcmod vm` |
| `break <fn>` | Sets a breakpoint on a function |
| `watch <n>.<field>` | Watches one field of one entity |
| `inspect <n>` | Prints one entity fully |

## Reading bytecode with qcmod disasm

`qcmod disasm` prints a compiled game program as readable instructions. Without a file argument it compiles the built-in gameplay sources first.

```sh
qcmod disasm [progs] [-func <name>] [-o <path>]
```

- `-func <name>`: prints only the named function.
- `-o <path>`: writes the output to a file instead of the console.

## Converting QuakeC with qcmod qc2go

`qcmod qc2go` translates one QuakeC source file into QuakeGo. Use it when you migrate an existing mod.

```sh
qcmod qc2go [-pkg progs] [-o out.go] file.qc
```

- `-pkg <name>`: the Go package name for the output. The default is `progs`.
- `-o <path>`: writes the output to a file instead of the console.

The result is a starting point. Constructs that need human judgment carry `TODO(transpile)` comments.

## Packaging data with qcmod pak

A PAK archive is the classic Quake data container. `qcmod pak` manages these files.

```sh
qcmod pak pack <datadir> -o pak0.pak
qcmod pak list pak0.pak
qcmod pak check pak0.pak
qcmod pak unpack pak0.pak -o <dest>
```

- `pack`: creates an archive from a directory. The tool writes entries in sorted order, so the output is byte-deterministic.
- `list`: prints the archive contents with sizes and names, sorted.
- `check`: makes sure that the table and the byte ranges are sound.
- `unpack`: extracts the archive and makes sure that every name still passes the rules on the way out.

The names inside the archive follow Quake rules. Each name must be at most 56 bytes, use forward slashes only, and never escape the archive with `..`.

Name your archives `pak0.pak`, `pak1.pak`, and so on. The engine mounts them in override order, and later archives take priority.

## Making textures with qcmod wad

`qcmod wad` converts PNG or TGA images into a Quake WAD. Each image becomes one lump in the archive.

```sh
qcmod wad -o gfx.wad menu.png title.png
qcmod wad -type miptex -o textures.wad wall.png floor.png
qcmod wad -type miptex -palette mygame/palette.lmp -o textures.wad stone.png
```

- `-o <path>`: the output file. The default is `out.wad`.
- `-type <kind>`: one of `auto`, `qpic`, or `miptex`.
- `-palette <path>`: a 768-byte palette file. The default is the built-in Quake palette.

The `auto` type picks QPic for menu art and MipTex for world textures. QPic lumps hold width, height, and palette-indexed pixels. MipTex lumps carry four mip levels, and their images must be multiples of 16 in both dimensions.

Conversion turns RGBA pixels into palette colors by nearest color. Pixels with alpha below 128 become the transparent index, which is 255.

## Starting the debugger with qcmod dap

`qcmod dap` starts a standalone debug server. It attaches to the same debug protocol that the full engine uses.

```sh
qcmod dap [addr]
```

The default address is `127.0.0.1:2345`. For full instructions, read the DAP manual.

## Where to go next

- The QuakeGo guide explains how to write gameplay logic.
- The DAP manual explains interactive debugging.
- The SDK manual explains how to build a standalone game.
- The mod authoring guide shows the scaffolded project in detail.