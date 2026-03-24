# QGo / QuakeGo Guide

This guide explains what `qgo` and QuakeGo are, how they fit into this repository, how to compile and test QuakeGo code, which language features work today, and which patterns to use when writing gameplay logic.

It is intentionally practical. `QGO_SPEC.md` is the design/specification document; this file is the "how do I actually use this?" companion.

## What are qgo and QuakeGo?

`qgo` is the compiler in `cmd/qgo/`. It takes a Go package and emits Quake `progs.dat` bytecode for the QCVM.

QuakeGo is the Go subset and runtime surface used by that compiler:

- `pkg/qgo/quake` defines the core QCVM-facing types such as `Entity`, `Vec3`, `Func`, and helper intrinsics.
- `pkg/qgo/quake/engine` defines engine builtins such as `Spawn`, `Remove`, `SetOrigin`, `Sound`, `WriteByte`, and friends.
- `pkg/qgo/quakego` is the translated gameplay package that proves the model works against real Quake game logic.

The short version is:

- Write Go code against the `quake` and `quake/engine` packages.
- Compile it with `qgo`.
- Load the resulting `progs.dat` in the engine.

## Mental model

QuakeGo is not "full Go running on Quake."

It is a deliberately narrow Go subset that maps cleanly onto QuakeC VM concepts:

- `float32`, `string`, `bool`, `quake.Vec3`, `*quake.Entity`, and function values map well to QCVM values.
- Struct fields tagged for qgo map to entity fields.
- Methods are lowered to QCVM-compatible functions.
- Engine calls are expressed as imports from `quake/engine`.

If you keep the QuakeC model in your head while using Go syntax and types, you will usually stay on the happy path.

## Repository layout

The pieces you will most often touch are:

```text
cmd/qgo/                 qgo CLI and compiler
pkg/qgo/quake/           core runtime-facing QuakeGo types
pkg/qgo/quake/engine/    engine builtin declarations
pkg/qgo/quakego/         real gameplay logic written in QuakeGo
QGO_SPEC.md              language and lowering spec
QCC_SPEC.md              QCVM / progs.dat format spec
```

Useful example files:

- `pkg/qgo/quakego/world.go`
- `pkg/qgo/quakego/triggers.go`
- `pkg/qgo/quakego/buttons.go`
- `pkg/qgo/quakego/doors.go`
- `cmd/qgo/testdata/minimal/progs.go`
- `cmd/qgo/testdata/vec3methods/progs.go`
- `cmd/qgo/testdata/modules/progs.qgo`

## Quick start

### Build the compiler

From the repository root:

```bash
CGO_ENABLED=0 go build ./cmd/qgo
```

The repo also exposes this via `mise` tasks.

### Compile the real QuakeGo gameplay package

The canonical example in this repository is `pkg/qgo/quakego`, which is a nested Go module.

From the repository root:

```bash
go build -o qgo ./cmd/qgo
cd pkg/qgo/quakego
../../../qgo
```

That writes `progs.dat` in `pkg/qgo/quakego/`.

This is also what `mise run build-progs` does.

### Compile to a custom output path

```bash
go run ./cmd/qgo -o /tmp/progs.dat ./pkg/qgo/quakego
```

### Verbose mode

```bash
go run ./cmd/qgo -v -o /tmp/progs.dat ./pkg/qgo/quakego
```

Today, `-v` prints a write summary:

```text
wrote /tmp/progs.dat (123456 bytes)
```

By default, successful compile output is otherwise silent.

## CLI reference

### Compile mode

```text
qgo [-o progs.dat] [-v] [dir]
```

Behavior:

- `dir` defaults to `.`
- `-o` chooses the output path
- `-v` enables the success write summary
- errors are reported on stderr with a `qgo:` prefix

Examples:

```bash
qgo
qgo ./pkg/qgo/quakego
qgo -o build/progs.dat ./pkg/qgo/quakego
```

### Source-order mode

`qgo` also exposes a deterministic source-order utility:

```text
qgo source-order [-format text|json] [-scope functions|files] [-o path] [-strict] [dir]
```

Examples:

```bash
go run ./cmd/qgo source-order ./pkg/qgo/quakego
go run ./cmd/qgo source-order -format json ./pkg/qgo/quakego
go run ./cmd/qgo source-order -scope files ./pkg/qgo/quakego
```

Notes:

- default format is `text`
- default scope is `functions`
- output goes to stdout unless `-o` is given
- paths are emitted as relative, slash-normalized paths
- JSON output is compact and ends with a trailing newline

Text output examples:

```text
0	world.go	worldspawn
1	triggers.go	trigger_multiple
```

```text
0	world.go
1	triggers.go
```

## Nested module and package-loading quirks

`pkg/qgo/quakego` is its own Go module, not just a subdirectory in the root module.

That matters because:

- it has its own `go.mod`
- it depends on `pkg/qgo/quake` through a local `replace`
- commands that work from the repo root are not always interchangeable with commands run inside `pkg/qgo/quakego`

Recommended patterns:

- if you want to compile the gameplay package from the repo root, use:

  ```bash
  go run ./cmd/qgo -o /tmp/progs.dat ./pkg/qgo/quakego
  ```

- if you have already built the compiler binary, use it from inside `pkg/qgo/quakego`:

  ```bash
  ../../../qgo
  ```

Also note that qgo supports `.qgo` source overlays in addition to `.go` files. The compiler handles those through a `go/packages` overlay internally.

## How to structure QuakeGo code

### Use the stub packages, not the standard library

QuakeGo code should primarily import:

```go
import (
    "github.com/ironwail/ironwail-go/pkg/qgo/quake"
    "github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)
```

Those packages are compiler-known surfaces. They are how qgo understands:

- entity fields
- function callbacks
- vectors
- engine builtins
- global QCVM state

Do not think in terms of "what random Go package can I import?" Think in terms of "what QCVM-representable surface do I need?"

### Entity state lives on `quake.Entity`

`quake.Entity` is the canonical entity layout for gameplay code. It carries tagged fields that qgo maps to QCVM entity fields.

Examples include:

- position/orientation: `Origin`, `Velocity`, `Angles`
- callbacks: `Touch`, `Use`, `Think`, `Blocked`
- gameplay state: `Health`, `SpawnFlags`, `Enemy`, `Target`, `TargetName`
- trigger/door fields: `Wait`, `Count`, `State`, `Pos1`, `Pos2`

Example:

```go
func trigger_once() {
    Self.Wait = -1
    trigger_multiple()
}
```

### Global QC state is exposed as package variables

The translated gameplay package uses globals such as:

- `Self`
- `Other`
- `World`
- `Time`
- `Activator`
- `MsgEntity`
- `VForward`, `VRight`, `VUp`

These represent the classic Quake globals you would expect from QuakeC.

## The receiver-adapter pattern

One of the most useful QuakeGo patterns is the "typed entity adapter" pattern. It lets you write entity-specific methods while still preserving the underlying `quake.Entity` storage model.

Example from `pkg/qgo/quakego/triggers.go`:

```go
type triggerEntity quake.Entity

func asTriggerEntity(ent *quake.Entity) *triggerEntity {
    return (*triggerEntity)(ent)
}

func (te *triggerEntity) entity() *quake.Entity {
    return (*quake.Entity)(te)
}
```

That gives you a clean place to hang behavior:

```go
func (te *triggerEntity) trigger(toucher *quake.Entity) {
    self := te.entity()
    if self.NextThink > Time {
        return
    }
    self.TakeDamage = DAMAGE_NO
    Activator = toucher
    SUB_UseTargets()
}
```

Why this pattern matters:

- it keeps entity-specific logic grouped together
- it reads better than large piles of free functions mutating `Self`
- it still compiles down to the QCVM entity model

qgo now supports the narrow pointer-conversion pattern that makes this work, so conversions like `(*triggerEntity)(ent)` and `(*quake.Entity)(te)` are accepted when they are equivalent entity-pointer wrappers.

## Event handlers and callbacks

QuakeGo uses function fields on `quake.Entity` for the same kinds of callbacks QuakeC exposes.

Common callback fields:

- `Touch`
- `Use`
- `Think`
- `Blocked`
- `ThPain`
- `ThDie`

Examples:

```go
self.Touch = multi_touch
self.Use = te.use
self.Think = te.wait
self.NextThink = Time + self.Wait
```

This is the normal way to express gameplay scheduling and event wiring.

### Common scheduling pattern

```go
if self.Wait > 0 {
    self.Think = te.wait
    self.NextThink = Time + self.Wait
} else {
    self.Touch = SUB_Null
    self.NextThink = Time + 0.1
    self.Think = SUB_Remove
}
```

Think of this as the QCVM/QuakeC equivalent of deferred state transitions rather than a general-purpose task scheduler.

## Engine builtins

Engine calls live under `pkg/qgo/quake/engine`.

Examples:

- `engine.Spawn()`
- `engine.Remove(ent)`
- `engine.SetOrigin(ent, pos)`
- `engine.Sound(ent, chanID, sample, volume, attenuation)`
- `engine.WriteByte(dest, value)`
- `engine.WriteString(dest, value)`
- `engine.PrecacheSound(path)`
- `engine.Centerprint(msg)`

These are not ordinary library calls. qgo recognizes them as QC builtins and lowers them to builtin call entries in the generated program.

### Builtin directives and names

The compiler supports builtin directives and a builtin-name registry. Named builtins and aliases are case-insensitive.

One especially important detail is that `print` is treated as an alias for builtin 24, whose canonical registry name is `sprint`.

If you see compiler errors such as:

```text
unknown //qgo:builtin alias "..."
malformed //qgo:builtin directive: expected one builtin number or alias
```

the builtin directive or alias name is wrong, not the engine package shape.

## Vectors and `quake.Vec3`

`quake.Vec3` is the vector type qgo understands natively.

It maps directly to QCVM vector values.

### Supported method-lowered Vec3 operations

Today, qgo directly lowers these method forms:

- `a.Add(b)`
- `a.Sub(b)`
- `a.Mul(s)`
- `a.Scale(s)`
- `a.Dot(b)`

Example:

```go
forward := VForward.Mul(300)
delta := end.Sub(start)
dp := forward.Dot(Self.MoveDir)
```

Important detail:

- `Mul(float32)` and `Scale(float32)` are treated as the same scalar-vector multiply lowering path

### Runtime helpers vs compiler support

The `quake.Vec3` API contains more helpers than the compiler currently lowers specially. For example, the runtime surface also exposes methods like `Div`, `Neg`, `Cross`, and `Lerp`.

That does **not** automatically mean every helper is compiler-lowered today.

If you use an unsupported Vec3 method, qgo will fail with a diagnostic like:

```text
unsupported Vec3 method: Cross
```

So when in doubt:

- prefer the already-proven method set above
- check `cmd/qgo/testdata/vec3methods/progs.go`
- check real gameplay uses in `pkg/qgo/quakego`

## Dynamic field access

qgo currently supports a narrow dynamic-field seam for float fields.

Supported forms:

```go
quake.FieldFloat(ent, ofs)
quake.SetFieldFloat(ent, ofs, value)
ent.FieldFloat(ofs)
ent.SetFieldFloat(ofs, value)
```

This is useful when a field offset is only known dynamically and direct static selector access is not appropriate.

What is **not** broadly supported yet:

- arbitrary dynamic vector field access
- arbitrary dynamic typed field access families

If you go outside the current narrow seam, qgo emits explicit deferred diagnostics instead of guessing.

## Supported language subset: the practical version

You should think of the supported subset in terms of the patterns below.

### Generally good bets

- package-level `var` and `const`
- local variables
- `if` / `else`
- classic `for` loops
- assignments and arithmetic
- named helper functions
- methods on entity adapter types
- simple function values used as entity callbacks
- struct field access on `quake.Entity`
- `quake.Vec3` value flow

### Frequently used in this repo

- pointer receivers on entity-wrapper types
- callback assignment through `Self` / `self`
- engine builtin calls
- global QC state through package variables
- numeric flag checks via integer conversions

Example:

```go
if (int(self.SpawnFlags) & int(SPAWNFLAG_NOTOUCH)) == 0 {
    self.Touch = multi_touch
}
```

## Deferred and unsupported features

qgo tries to fail clearly when a feature is intentionally deferred.

Some important examples:

### Type assertions

Deferred:

```go
x.(T)
```

Diagnostic:

```text
unsupported type assertion expression: x.(T) is deferred
```

### Type switches

Deferred:

```go
switch v := x.(type) {
...
}
```

Diagnostic:

```text
unsupported type switch statement: switch v := x.(type) is deferred
```

### General struct literals

Non-`Vec3` struct literals are not broadly lowered today.

Diagnostic:

```text
general struct literals are deferred (only Vec3 vector literals are currently supported): <type>
```

### Unsupported type conversions

If qgo cannot justify a conversion within its QCVM model, it reports it explicitly:

```text
unsupported type conversion from <src> to <dst>
```

The important philosophy here is that qgo prefers explicit "deferred" errors over silent miscompiles.

## Writing your own QuakeGo code

A good starting workflow is:

1. Create a small Go package that imports `quake` and `quake/engine`.
2. Keep the code single-purpose and QCVM-shaped.
3. Start with simple functions and direct entity field access.
4. Introduce receiver adapters once the logic is entity-centric.
5. Compile early with `qgo` and use the diagnostics to stay within the supported subset.

### Minimal shape

```go
package progs

import (
    "github.com/ironwail/ironwail-go/pkg/qgo/quake"
    "github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

var Counter float32

func worldspawn() {
    Counter = Counter + 1
    engine.DPrint("worldspawn\n")
}

func useTarget(ent *quake.Entity) {
    ent.NextThink = 0
}
```

### Trigger-style entity organization

```go
type doorEntity quake.Entity

func asDoorEntity(ent *quake.Entity) *doorEntity {
    return (*doorEntity)(ent)
}

func (de *doorEntity) entity() *quake.Entity {
    return (*quake.Entity)(de)
}

func (de *doorEntity) open() {
    self := de.entity()
    self.State = 1
    self.NextThink = Time + 0.1
}
```

That style is the closest thing to "idiomatic QuakeGo" in this repository today.

## Testing strategy

There are two distinct levels of testing:

### 1. Ordinary Go tests against the stub runtime

Some logic can be exercised with normal `go test`, especially when using the mockable engine/builtin surface.

Relevant files:

- `pkg/qgo/quake/quake_test.go`
- `pkg/qgo/quakego/triggers_test.go`

This is the fastest way to validate helper logic and certain gameplay flows.

### 2. Compile-level tests

Compiler coverage lives under:

- `cmd/qgo/compiler/compiler_test.go`
- `cmd/qgo/testdata/*`

These tests are the right place to add coverage when:

- a previously valid language pattern regresses
- a new qgo lowering feature is added
- a diagnostic contract should stay stable

### Useful commands

```bash
go test ./cmd/qgo/compiler -count=1
go test ./cmd/qgo -count=1
(cd pkg/qgo/quakego && go test ./... -count=1)
go run ./cmd/qgo -o /tmp/quakego.progs.dat ./pkg/qgo/quakego
```

That last command is especially valuable because it validates the real gameplay package end to end.

## Troubleshooting

### `qgo: unsupported Vec3 method: ...`

Cause:

- you used a Vec3 helper the compiler does not lower yet

What to do:

- switch to one of the proven method forms: `Add`, `Sub`, `Mul`, `Scale`, `Dot`
- or add compiler support plus tests before using the method widely

### `qgo: unsupported type assertion expression: x.(T) is deferred`

Cause:

- type assertions are intentionally deferred

What to do:

- redesign the code around explicit state, tagged fields, or separate helper paths rather than runtime type assertions

### `qgo: unsupported type switch statement: switch v := x.(type) is deferred`

Cause:

- type switches are intentionally deferred

What to do:

- replace the type switch with simpler QCVM-compatible branching

### `qgo: unsupported type conversion from ... to ...`

Cause:

- the conversion is outside the compiler's supported QCVM mapping

What to do:

- simplify the type boundary
- use the canonical `quake` / `engine` types
- or follow the existing entity-wrapper conversion pattern if the conversion is really an entity adapter

### Package loading or module weirdness

If a command behaves differently depending on where you run it, remember that `pkg/qgo/quakego` is a nested module.

When in doubt, prefer one of these exact commands:

```bash
go run ./cmd/qgo -o /tmp/progs.dat ./pkg/qgo/quakego
```

or:

```bash
go build -o qgo ./cmd/qgo
cd pkg/qgo/quakego
../../../qgo
```

## Recommended development workflow

For new QuakeGo work in this repo:

1. Start from an existing pattern in `pkg/qgo/quakego`.
2. Keep the first version simple and compiler-friendly.
3. Add or update a focused compiler test if you are relying on a new lowering rule.
4. Validate the real gameplay package with:

   ```bash
   go run ./cmd/qgo -o /tmp/quakego.progs.dat ./pkg/qgo/quakego
   ```

5. Only then widen usage of the new pattern across the gameplay package.

This sequence catches "works in a unit fixture but breaks the real game package" problems early.

## When to read the deeper specs

Use this guide for day-to-day authoring and debugging.

Use these documents when you need more detail:

- `QGO_SPEC.md` for the formal language/lowering model
- `QCC_SPEC.md` for QCVM and `progs.dat` details
- `cmd/qgo/plan.md` for historical compiler implementation context

## Summary

If you remember only a few rules, remember these:

- write QCVM-shaped Go, not general-purpose Go
- use `quake` and `quake/engine` as the canonical surface
- prefer proven entity-wrapper and callback-assignment patterns from `pkg/qgo/quakego`
- keep to the supported Vec3 method set
- treat explicit deferred diagnostics as guidance, not as bugs to work around blindly
- always validate with a real `qgo ./pkg/qgo/quakego` compile before declaring a new pattern safe
