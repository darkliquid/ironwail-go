# Building a Standalone Game with the Ironwail-Go SDK

Ironwail-Go is more than an engine binary. You can build your own Quake-based game on top of it, with your own name, your own rules, and your own data. This manual explains the whole workflow.

The engine is a Go library. A game is an ordinary Go program that calls into the engine and then lets it run. The public door into the engine is the `sdk` package.

## What the SDK gives you

The SDK is the stable, public surface of the engine. Your game imports one package:

```go
import "github.com/darkliquid/ironwail-go/sdk"
```

The engine internals live in `internal/` packages. Go forbids other modules from importing those, so the `sdk` package re-exports the small set of functions a game needs. Only stable, game-facing API lives there.

The engine brings the whole game stack: rendering, input, audio, physics, networking, the game VM, and the console. You supply the identity of your game, the gameplay logic, and the game data.

## The shape of a game

A game has four parts:

| Part | What it is |
| --- | --- |
| The Go module | Your buildable game program. |
| The config | The identity and settings of the game. |
| The gameplay | The logic in QuakeGo, compiled to `progs.dat`. |
| The data | Models, maps, textures, sounds, and the compiled logic, mounted as a game directory. |

The engine boots in this order: it reads the config, mounts the game data, loads the gameplay program, and then runs its own loop.

## Starting a project

The fastest path is the scaffolder:

```sh
qcmod init -kind tc mygame
```

The `-kind` flag picks a starting shape:

| Kind | What you get |
| --- | --- |
| `generic` | A minimal standalone game. |
| `sp` | A single-player starter with spawn and think stubs. |
| `dm` | A deathmatch-first setup with item respawn stubs. |
| `tc` | A total conversion with a full identity override. |

The generated directory is a working Go module. It has a `go.mod` with `replace` directives that point back at the engine and the Quake module. The paths are relative, so the project moves as one unit.

Build, test, and run the project from its own folder:

```sh
cd mygame
go mod tidy
make test
make build
make run
```

`make run` launches the binary from the parent directory. The engine mounts the game data at `./mygame` relative to the working directory, so the data folder must sit next to the binary.

## Configuring the game

The config is a struct with named fields. Any field you leave at zero takes the stock Quake default.

```go
config := sdk.Config{
    GameName:       "My Game",
    BaseGameDir:    "mygame",
    DefaultSkill:   1,
    DefaultDeathmatch: 0,
    DefaultCoop:       0,
}
```

The main entry point looks like this:

```go
func main() {
    config := newGameConfig()
    g, err := sdk.Run(config, sdk.Args(os.Args...))
    if err != nil {
        panic(err)
    }
    _ = g // the engine owns the game loop
}
```

The config groups the identity of your game:

| Group | What it controls |
| --- | --- |
| Identity | The game name, the base game directory, and the user config directory. |
| Registration | Whether the game requires a registered flag, and the messages around it. |
| Network | The protocol magic, version, and protocol numbers for the wire. |
| Defaults | The starting values of skill, deathmatch, coop, and teamplay. |
| Menu | The labels on the menu entries. An empty label hides the entry. |

Two options tune the run. `sdk.Headless()` runs without rendering, for a dedicated server or automated tests. `sdk.Args(...)` passes the command line, in the same format the engine binary accepts.

## Writing the gameplay

The gameplay lives in the `progs` folder as QuakeGo. QuakeGo is a dialect of Go that compiles to the bytecode of the game VM. The compiler is `qgo`:

```sh
qgo -o progs.dat progs
```

The engine runs `progs.dat` at game start.

The same sources also build as plain Go. That is the trick that makes headless tests possible. A test creates a simulated world, spawns entities, fires their functions, and makes assertions:

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

Run the tests with `qcmod test` or with plain `go test`. They need no assets, no GPU, and no engine boot.

## Building your data

The game data has several pieces, and each has its own builder:

| Data | How you make it |
| --- | --- |
| `progs.dat` | Compile the QuakeGo sources with `qgo`. |
| Maps | Compile `.map` files with qbsp, vis, and light. |
| Textures | Convert images to a WAD with `qcmod wad`. |
| Archives | Pack the data into PAKs with `qcmod pak`. |

The map pipeline is a chain of three tools:

```sh
./qbsp -o map.bsp map.map
./vis -o map.bsp map.bsp
./light -lit -o map.bsp map.bsp
```

Their manuals live next to this one.

## Packaging the game

The engine reads game data from loose folders and from PAK archives. A PAK is the classic Quake container.

```sh
qcmod pak pack mygame -o pak0.pak
```

Name the archives `pak0.pak`, `pak1.pak`, and so on. The engine mounts them in override order, so later archives win when two hold the same file.

A total conversion ships one archive that bundles `progs.dat`, the maps, and any other data under its base game directory.

## Debugging the gameplay

The engine and the toolkit share one debugger, built on the Debug Adapter Protocol. Start it inside your game with the `-qcdbg` flag, or standalone with `qcmod dap`.

The debugger sets breakpoints, steps through bytecode, and inspects entities, globals, and the call stack. The DAP manual explains the setup for your editor.

## Distributing your game

A playable distribution is the game binary plus the data archives. Keep the data in the base game directory next to the binary.

```text
mygame/
  mygame          the game binary
  mygame/         the game data
    progs.dat
    maps/
    pak0.pak
    gfx/
```

The stock base game uses the directory name `id1`. Your game uses its own `BaseGameDir` name, so nothing leaks between games.

## Where the surface grows

The SDK is a thin facade on purpose. The extension system is still landing: a subsystem registry, command restriction, and post-processing hooks. Each will appear as new functions on `sdk` when it is stable enough for games to depend on it.

Keep the shape of your `main.go` close to the template. Adding a hook after `sdk.Run` stays a one-line change.

## Related reading

- The qcmod manual covers the toolkit commands in full.
- The qbsp, vis, and light manuals cover the map pipeline.
- The QuakeGo guide covers the gameplay language.
- The DAP manual covers the debugger.
- The glossary explains every term in this manual.