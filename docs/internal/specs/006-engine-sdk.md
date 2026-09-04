# SPEC-006: Engine SDK for Standalone Mods and Total Conversions

Bead: ironwail-go-bvw · Status: draft

## Problem

The engine hard-codes dozens of Quake-specific assumptions: the base game
directory name (`id1`), the shareware/registered licensing gate, the network
protocol identity (`"QUAKE"` handshake magic, protocol version numbers), the
window/console titles, and the user config directory (`~/.ironwail`). A mod
author building a total conversion — a game that reuses the Ironwail-Go
engine but is not Quake — must fork the engine to change any of these.

## Goal

Centralise every Quake-specific identity into a single `GameConfig` struct
that the engine initialises from. Provide `qcmod init [dir]` to scaffold a
new standalone mod with that config pre-populated from the directory name.

## Non-Goals

- Changing default behaviour for stock Quake data (all defaults must be
  byte-identical to current output when no config override is present).
- Making the protocol wire format configurable at runtime (that would break
  interop for no gain; standalone mods simply use the identity they choose
  at build time).

## Current State Audit

### "id1" hardcoding (25 occurrences, 13 files)

| File | Count | Context |
|---|---|---|
| host/commands_gameplay_save.go | 4 | save/search paths |
| game/game_init.go | 3 | EnsureRuntimeProgsData, mod validation, profile |
| fs/fs.go | 3 | mount order, path walking |
| cmd/ironwailgo/main_wasm.go | 3 | WASM bootstrap |
| menu/manager.go | 2 | mod listing |
| host/commands_map.go | 2 | map command |
| fs/fs_search.go | 2 | well-known base dir exclusion |
| server/savegame_server.go | 1 | save path |
| game/game_profile.go | 1 | profile default |
| game/game_commands_viewpoint.go | 1 | viewpoint JSON |
| game/game_commands.go | 1 | gamedir command |
| cmd/ironwailgo/startup_args.go | 1 | default mod |
| cmd/ironwailgo-harness/main.go | 1 | harness bootstrap |

### Shareware/registered gate

`game/game_init.go:140-153`: the `registered` cvar (default 0), the
"Playing registered/shareware version" console messages, and the hard error
`"you must have the registered version to use modified games"` — which
blocks any mod from loading when `registered` is 0.

### Network identity

`net/datagram.go:430-433`: the connection handshake embeds `"QUAKE\x00"`
(6 bytes) and protocol version 3. `net/protocol.go`: `PROTOCOL_NETQUAKE=15`,
`PROTOCOL_FITZQUAKE=666`, `PROTOCOL_RMQ=999`. `net/datagram.go:678`: server
info response carries protocol version 3.

### Hard-coded strings

| Location | Current value |
|---|---|
| console.DefaultTitleString | `"Ironwail-Go"` |
| host.DefaultWindowTitle | `"Ironwail-Go"` |
| game_runtime_csqc.go CSQC init | `"Ironwail"` |
| menu_game.go | `"QUAKE DIRECTORY"` |
| menu_options.go | `"QUAKEWORLD"` |

### User config directory

`host/init.go:578`: `filepath.Join(homeDir, ".ironwail")`.

## Design

### 1. GameConfig struct (new: `internal/gameconfig`)

A single immutable struct, constructed at startup, threaded to every
subsystem that currently reads a hard-coded value.

```go
package gameconfig

type Config struct {
    // Identity
    GameName       string // window title, console title, CSQC init
    BaseGameDir    string // "id1" — the required base data directory
    UserDirName    string // ".ironwail" — user config dir under $HOME

    // Licensing
    RequireRegistered bool  // gate mods when false
    DefaultRegistered bool  // initial value of the `registered` cvar

    // Network
    ProtocolMagic  []byte    // "QUAKE\x00" — connection handshake identity
    ProtocolVer    int       // version byte in handshake and server info
    ProtocolNums   ProtocolNumbers

    // Default cvar values that mods may want to change
    DefaultSkill     int
    DefaultDeathmatch int
    DefaultCoop      int
    DefaultTeamplay  int

    // Menu strings
    ModDirMenuLabel string  // "QUAKE DIRECTORY"
    NetOptionLabel  string  // "QUAKEWORLD"
}

type ProtocolNumbers struct {
    NetQuake  int // 15
    FitzQuake int // 666
    RMQ       int // 999
}

// Default returns a Config that reproduces current stock Quake behaviour
// exactly. Used when no override is provided.
func Default() Config
```

**Threading.** `Config` is created in `cmd/ironwailgo/main.go` (or by the
mod's `main`) and passed to `game.New(config)`, which passes it to
`host.InitParams` (new fields), `fs.Init` (base game dir name), and the
net layer (protocol identity). Subsystems read from the struct — they never
read a literal.

**Alternative considered.** Global mutable config. Rejected: the engine
already has a DI pattern (InitParams, Subsystems interfaces); adding a
global would fight that. The struct is passed explicitly.

### 2. Replacing the shareware gate

The existing code in `game_init.go:140-153`:

```go
registered := g.Host.CVar.Register("registered", "0", ...)
if registered.Float != 0 {
    console.Printf("Playing registered version.\n")
} else {
    console.Printf("Playing shareware version.\n")
}
if modDir != "" && modDir != "id1" {
    return fmt.Errorf("you must have the registered version to use modified games")
}
```

becomes:

```go
registered := g.Host.CVar.Register("registered",
    strconv.Itoa(config.DefaultRegisteredInt()), ...)
if registered.Float != 0 {
    console.Printf("%s\n", config.RegisteredMessage)
} else {
    console.Printf("%s\n", config.SharewareMessage)
}
if modDir != "" && modDir != config.BaseGameDir && config.RequireRegistered {
    return fmt.Errorf("%s", config.ModRequiresRegisteredMessage)
}
```

Standalone mods set `RequireRegistered: false` and `DefaultRegistered: true`
so mods load freely. The messages are configurable too (or omitted entirely
when the strings are empty — a total conversion may not want a version
banner at all).

### 3. Replacing "id1"

`config.BaseGameDir` replaces every `"id1"` literal. The `fs.AddGameDirectory`
call becomes `filepath.Join(basedir, config.BaseGameDir)`. The
`fs_search.go` well-known-dir exclusion (`strings.EqualFold(name, "id1")`)
compares against `config.BaseGameDir`.

The 25 call sites are replaced mechanically; no logic changes.

### 4. Replacing network identity

`net/datagram.go` connection handshake:

```go
// before
copy(buf[9:], "QUAKE\x00")
// after
copy(buf[9:], config.ProtocolMagic)
```

Server info response protocol version:

```go
// before
payload = append(payload, players, maxPlayers, 3)
// after
payload = append(payload, players, maxPlayers, byte(config.ProtocolVer))
```

The net layer receives the Config via a setter or constructor parameter
(from `Server.StartDAPServer`-style DI), not a global.

### 5. Replacing hard-coded strings

Each string becomes a `Config` field with the current value as the zero-value
default. Console title, window title, CSQC init name, and menu labels all
read from the struct. Empty-string menu labels hide the menu entry entirely
(a total conversion may not want a "QUAKE DIRECTORY" row).

### 6. `qcmod init [dir]`

Creates a new mod directory from an embedded template:

```
<dir>/
  go.mod          module <basename>; require quake; replace => path
  main.go         func main() { engine.Run(RunMod) }
  gameconfig.go   gameconfig.Config pre-populated from <basename>
  progs/          empty; mod author adds QuakeGo sources here
  test/           example sim test
```

The `gameconfig.go` sets `GameName` to the title-cased directory name,
`BaseGameDir` to the directory name (so the mod is self-contained), and
`RequireRegistered` to false.

The template files are embedded via `//go:embed` in a new
`cmd/qcmod/template/` directory, so `qcmod init` works offline.

`qcmod init` also writes a `Makefile` with `build` and `run` targets that
invoke the right `go run` commands.

### 7. Who creates the Config

Two paths:

**Engine binary** (`cmd/ironwailgo`): constructs `gameconfig.Default()` and
applies any overrides from CLI flags. No change to existing behaviour.

**Standalone mod binary**: the mod's own `main.go` constructs a custom
`gameconfig.Config` and calls `engine.Run(config, modGameplay)`. The engine
package gains an `engine.Run(config gameconfig.Config, game GameHooks)`
entry point that replaces `cmd/ironwailgo/main.go`'s bootstrap for mod
binaries.

This is the "SDK" part of the bead: a mod author imports the engine package,
provides a Config and gameplay hooks, and gets a standalone game binary.

### 8. Implementation plan (milestones)

| M | Scope | Files touched |
|---|---|---|
| M0 | Add `internal/gameconfig` package with `Config` struct and `Default()` | 1 new file |
| M1 | Replace `"id1"` literals in fs/, game/, host/, menu/, cmd/ (mechanical, 25 sites) | ~13 files |
| M2 | Replace shareware/registered gate with Config fields | game_init.go |
| M3 | Replace net protocol identity (magic string, version bytes, protocol numbers) | net/datagram.go, net/protocol.go |
| M4 | Replace hard-coded strings (console title, window title, CSQC init, menu labels) | console/, host/, game/, menu/ |
| M5 | Wire `qcmod init [dir]` with embedded template | cmd/qcmod/init.go, cmd/qcmod/template/ |
| M6 | Add `engine.Run(config, hooks)` SDK entry point for standalone mod binaries | internal/game/ or new internal/engine/ |

Each milestone is independently testable and committable.

### 9. Testing

- **M0**: unit test that `Default()` reproduces every current literal.
- **M1**: existing tests must pass unchanged (proves default compatibility).
  New test: mount fs with `BaseGameDir: "mymod"` and verify paths.
- **M2**: new test: `RequireRegistered: false` + `registered=0` + mod dir →
  no error. Existing test: `RequireRegistered: true` + `registered=0` + mod
  dir → error (current behaviour preserved).
- **M3**: new test: handshake contains `ProtocolMagic` bytes; server info
  carries `ProtocolVer`. Existing net tests must pass unchanged.
- **M4**: new test: console title / window title come from Config.
- **M5**: new test: `qcmod init /tmp/mygame` creates the expected file tree;
  the generated `go.mod` has the right module name and replace directive;
  the generated `gameconfig.go` sets `GameName: "Mygame"`.
- **M6**: new test: `engine.Run` with a minimal config boots a server and
  fires a think function.

### 10. Risks

- **Missing a hard-coded literal.** The 25 `"id1"` sites are enumerated
  above, but there may be strings I haven't found. Mitigation: after M1, run
  `grep -rn '"id1"'` and require zero results (excluding tests).
- **Protocol compatibility regression.** Changing the net identity breaks
  connection to standard Quake servers/clients. Mitigation: `Default()`
  reproduces the current values exactly; the existing net tests verify this.
- **Config bloat.** Adding a field for every string is over-engineering if a
  mod only cares about the name. Mitigation: `Default()` covers the common
  case; a standalone mod only overrides the fields it cares about (Go
  zero-value semantics: zero-value fields fall back to defaults via a
  `resolve()` method).

## 11. Extensibility architecture (foundations laid now, implemented later)

The SDK must not just make values configurable — it must have the
structural foundations for mods to extend, restrict, and replace engine
behaviour. The patterns below are not implemented in this bead, but the
architecture is shaped to accommodate them without a second refactor.

### 11.1 Pattern: subsystem registry with lifecycle

The existing `Subsystems` struct already models subsystems as interfaces
(`Console`, `Server`, `Client`, `Renderer`, `Audio`). The SDK formalises
this into a registry where each subsystem has a well-known interface and a
resolution order.

```go
// internal/engine/registry.go (future)
type Registry struct {
    console  Console       // may be nil (console disabled)
    renderer Renderer
    audio    Audio
    fs       Filesystem
    // ...
}
```

This supports:

- **Disabling subsystems entirely.** A mod sets `Config.DisableConsole`
  (or a general `Config.Features` map) and the registry simply does not
  resolve that subsystem. Callers check `nil` at use sites (already the
  pattern for Audio and Renderer).
- **Replacing subsystems.** A mod provides a custom `Filesystem`
  implementation (e.g. one that reads encrypted pak files) and the registry
  resolves to it instead of the default.

The existing `Subsystems` struct already supports nil for Audio and
Renderer (headless mode). The change is: (a) the Config can programmatically
suppress any subsystem, and (b) the mod can supply a custom implementation
via the SDK entry point.

### 11.2 Pattern: console command de-registration and restriction

The `CmdSystem` currently only supports `AddCommand`. The SDK adds:

```go
// future: internal/cmdsys additions
func (c *CmdSystem) RemoveCommand(name string)
func (c *CmdSystem) SetRestricted(names []string) // deny-list or allow-list
```

A mod might remove `map`, `load`, or `save` commands for a puzzle game, or
restrict `god` / `noclip` in competitive modes. The CmdSystem already
stores commands in a map, so removal is trivial; the restriction layer is a
check at dispatch time.

The Config can also carry a `DisabledCommands []string` list that the host
applies after registering all built-in commands — so a mod can disable the
console (`Config.DisableConsole`) and with it every console-only command,
without individually listing them.

### 11.3 Pattern: renderer pass hooks

The renderer already has a pipeline structure (opaque pass, OIT
translucent pass, overlay composite, post-processing). The SDK exposes
named hook points where a mod can insert pre/post-processing passes or
replace an existing pass.

```go
// future: internal/renderer/hooks.go
type RenderPassHook func(ctx RenderContext, next func())

// Hook points (named so mods can target them without hardcoding indices):
//   "opaque.pre"          before opaque geometry
//   "opaque.post"         after opaque, before translucent
//   "translucent.post"    after OIT resolve, before overlay
//   "overlay.pre"         before HUD/menu overlay
//   "post.final"          final frame (post-processing, CRT, etc.)
```

The Config (or the mod's `GameHooks`) registers hooks by name:

```go
// future: in mod's main.go
hooks.RenderPass["post.final"] = func(ctx renderer.RenderContext, next func()) {
    drawCRTWarp(ctx)
    next()
}
```

The renderer does not need structural changes — it already has a
`RenderFrame(state, overlayFn)` callback pattern. The change is: formalise
the hook points as named constants, allow multiple hooks per point (chained
in registration order), and expose a registration API through the SDK's
`GameHooks`.

### 11.4 Pattern: post-processing shader pipeline

Post-processing (bloom, SSAO, CRT) is a special case of the renderer pass
hook at `"post.final"`. The SDK's role is to define the shader module
interface so mods can provide WGSL shaders that integrate with the existing
gogpu pipeline:

```go
// future: internal/renderer/postfx.go
type PostFXShader interface {
    WGSL() []byte                    // shader source
    BindGroup(target Target) any     // GPU resources
    Apply(ctx RenderContext)         // render pass
}
```

The renderer's post-processing pipeline becomes a chain of `PostFXShader`
implementations. Mods append to the chain via the `"post.final"` hook. The
Config can also list built-in effects to enable (`Config.PostFX = []string{"bloom", "crt"}`).

### 11.5 Pattern: filesystem wrapping

The `Filesystem` interface (`Init`, `LoadFile`, `LoadFirstAvailable`,
`FileExists`) is already an interface, so a mod can provide a custom
implementation that wraps the default with transparent encryption, remote
fetching, or mod-priority layering.

The SDK's role: the `engine.Run` entry point accepts a `Filesystem`
implementation as part of `GameHooks`. If provided, it replaces the default
`fs.FileSystem`. If not, the default is used. The Config can also carry a
`FSWrapper` function type that decorates the default:

```go
// future: in GameHooks
type GameHooks struct {
    // ...
    FSWrapper func(fs Filesystem) Filesystem // optional decorator
}
```

This supports encrypted paks (the wrapper decrypts on `LoadFile`), network
streaming (the wrapper fetches remotely on cache miss), or mod layering
(the wrapper tries the mod dir first, then the base).

### 11.6 Pattern: feature flags

Some "disabling" is not a subsystem replacement but a feature toggle: the
console dropdown, the single-player menu, the multiplayer menu, the demo
bar. These are UI-level decisions that gate access to engine features.

```go
// future: internal/gameconfig/features.go
type Features struct {
    Console        bool // dropdown console available
    SinglePlayer   bool // single-player menu visible
    Multiplayer    bool // multiplayer menu visible
    DemoPlayback   bool // demo bar / demo loop
    SaveLoad       bool // save/load commands available
    Cheats         bool // god/noclip/impulse commands functional
}
```

The Config carries a `Features` struct (defaulting to all-true). The menu
system reads it to build the menu tree; the host reads it to gate command
dispatch and console visibility. Empty/zero-value falls back to all-enabled.

This is distinct from the subsystem registry (11.1): disabling the console
removes the UI affordance, but the `CmdSystem` still exists (for StuffCmd
and aliases). Disabling the renderer would be a subsystem change. Both
patterns are needed.

### 11.7 Summary of architectural requirements

For the foundations to work, the following must be true after this bead:

1. **All subsystems are accessed through interfaces** — already true via
   `Subsystems`, but the Config must be threaded to the point where the
   registry is built, not accessed globally.
2. **The CmdSystem supports removal** — trivial addition; not in this bead
   but the interface must not prevent it (it doesn't; commands are in a map).
3. **The renderer's hook points are named** — not in this bead, but the
   `RenderFrame(state, overlayFn)` pattern must not be collapsed into a
   monolithic call (it isn't; the overlay is already a callback).
4. **The Filesystem is an interface with a decoration path** — already true;
   the SDK entry point must accept a custom or wrapped implementation.
5. **Feature flags are a struct, not scattered booleans** — the Config's
   `Features` struct centralises them so new toggles are additive.

None of these require changes in this bead beyond threading the Config —
but every hard-coded literal replaced is one less thing to refactor later.

## 12. Future work (not this bead)

- `sourcesContent` embedding in source maps (serves source to DAP clients
  without the mod sources on disk).
- Mod-specific console command registration and de-registration API
  (§11.2).
- Subsystem disabling/replacement via the registry (§11.1).
- Renderer pass hooks and post-processing shader chain (§11.3, §11.4).
- Filesystem wrappers for encrypted paks / remote streaming (§11.5).
- Feature flags for menu tree and command gating (§11.6).
- Mod asset pipeline integration (texture replacement, model loading).
