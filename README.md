# Ironwail-Go

Ironwail-Go is a port of the Ironwail Quake engine from C to Go. Ironwail is a modernized rebuild of Quake, the classic 1996 first-person shooter by id Software. This project translates the engine, keeps the same gameplay, and rewrites the internals in clean, readable Go.

The port is pure Go. It has no C code, and it builds with CGO turned off. It runs on desktops and compiles to WebAssembly for browsers.

## Why this project exists

The project has four purposes.

| Purpose | What it means |
| --- | --- |
| Learning | It explains an entire game engine in a language that is easy to read. |
| Experiment | It tests how far agentic coding can carry a large port. |
| Parity | It proves that a rewrite can match the original behavior and look. |
| Tooling | It ships the compilers and mod kit most Quake projects need. |

Agentic coding means that AI agents write the code. A large portion of this codebase was written by agents that convert the C source to Go. A human plans the work, reviews each change, and writes the manual parts. The result is not a dump of machine output. It is a guided port with a human in charge.

## Goals

The primary goal is full behavioral parity with the C engine. Parity means that the Go engine behaves and looks like the C engine. Regular audits compare the two, and the results live in the parity guide.

The secondary goals shape how the port is written:

| Goal | How the project meets it |
| --- | --- |
| Parity | The GoGPU renderer is the canonical path, and a screenshot harness gates it. |
| Readability | The code splits into small packages, with comments that explain the why. |
| Pure Go | Standard Go libraries replace the custom C implementations where it makes sense. |
| Completeness | The repo includes a game VM, a mod SDK, and a map compiler pipeline. |

## Current status

The project is under active development. It is not a finished product.

What works today:

| Area | Status |
| --- | --- |
| Gameplay | Single player and multiplayer run through the authoritative client and server split. |
| Rendering | The GoGPU path renders the world, models, sprites, particles, decals, sky, liquids, and the HUD. |
| Game logic | The QuakeC VM runs `progs.dat`, and the QGo compiler builds QuakeGo source into it. |
| Debugging | A Debug Adapter Protocol server debugs bytecode from IDEs. |
| Mod SDK | The `sdk` package and the `qcmod` toolkit build standalone games. |
| Map tools | The `qbsp`, `vis`, and `light` compilers build playable maps. |
| Web | The engine compiles to WebAssembly and runs in browsers. |

The parity screenshot harness is an automatic gate. It captures reference frames from the C engine and the Go engine, then compares them. The commands are `mise run parity-ref`, `mise run parity-go`, and `mise run parity-compare`.

The map compiler pipeline is newer than the engine port. It follows the classic Quake algorithms, and it is tested against the reference tools where those are available.

## Building and running

The project uses mise for its toolchain. Run `mise tasks` to see every task.

The core tasks:

| Task | What it does |
| --- | --- |
| `mise run build` | Builds the game binary. |
| `mise run test` | Runs the Go test suite. |
| `mise run run` | Builds and runs the game. |
| `mise run verify` | Runs the tests and the build. |
| `mise run lint` | Runs the linters. |
| `mise run smoke-all` | Runs the smoke tests. |

The game needs the original Quake data to run. Point the run task at your data with the `QUAKE_DIR` environment variable, or set up the `quake-data` symlink.

```sh
export QUAKE_DIR=/path/to/your/quake
mise run run
```

The map pipeline has its own tasks. Build the tools with `mise run build-qbsp`, `mise run build-vis`, and `mise run build-light`. Compile a whole map with `mise run map-build MAP=mymap`.

The mod SDK has an end-to-end check: `mise run verify-mod-sdk` scaffolds every template and builds each one.

## Documentation and manuals

The docs folder holds the full documentation.

| Document | What it covers |
| --- | --- |
| `docs/LEARNING_GUIDE.md` | The package map and how the engine fits together. |
| `docs/PARITY.md` | The parity goals, current gaps, and how to audit. |
| `docs/QGO_QUAKEGO_GUIDE.md` | The QuakeGo gameplay language and the QGo compiler. |
| `docs/GLOSSARY.md` | Every technical term explained in plain English. |
| `docs/manuals/qcmod.md` | The mod development toolkit. |
| `docs/manuals/sdk.md` | Building a standalone game on the engine. |
| `docs/manuals/qbsp.md` | The map compiler, first stage. |
| `docs/manuals/vis.md` | The visibility calculator. |
| `docs/manuals/light.md` | The lightmap baker. |
| `docs/manuals/DAP.md` | The interactive debugger. |
| Walkthroughs | End-to-end tours of boot, single player, and multiplayer. |

## License

The project is licensed under the GNU General Public License, version 2. The full license text is in `LICENSE.txt`.

## Attribution

This project stands on the work of three families of code.

The primary source is [Ironwail][1], the C engine maintained by andrei-drexler. Ironwail preserves and modernizes the original Quake engine, and this port follows it closely. The original Ironwail developers have not reviewed or endorsed the work in this repository.

[Quake][2] is the original game and engine by id Software. Its source code was released under the GNU General Public License. Every Quake engine, including this one, traces back to that release and to the people who wrote it.

The map compiler pipeline follows the algorithms of the classic Quake tools and their modern descendant, the [ericw-tools][3] suite by Eric Wasylishen. The Go compilers are tested against the reference tools when those binaries are present.

The Go port also builds on open libraries. The renderer uses the [gogpu WebGPU stack][4], and audio uses [Oto][5]. Their licenses live in their own repositories.

Quake and its related marks belong to id Software. This project is unofficial, it is not affiliated with id Software, and it does not distribute game data. You bring your own legally obtained data to play.

[1]:https://github.com/andrei-drexler/ironwail
[2]:https://github.com/id-Software/Quake
[3]:https://github.com/ericwa/ericw-tools
[4]:https://github.com/gogpu
[5]:https://github.com/ebitengine/oto
