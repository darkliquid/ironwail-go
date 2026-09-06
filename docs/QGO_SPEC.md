# QGo: Go-to-QCVM Compiler Design

## A Compiler for Compiling Go Source Code to QCVM Bytecode

This document specifies the design of **QGo**, a compiler that accepts a subset of the
Go programming language and emits QCVM bytecode (`progs.dat`). It maps Go's type system,
control flow, and program structure onto the constraints of the QuakeC Virtual Machine
as documented in [QCC_SPEC.md](QCC_SPEC.md).

The [Go Language Specification](https://go.dev/ref/spec) (version go1.26) is the
authoritative reference for Go semantics. This document describes how each supported
Go construct is lowered to QCVM representations.

---

## Table of Contents

1. [Design Goals and Non-Goals](#1-design-goals-and-non-goals)
2. [Language Subset](#2-language-subset)
3. [Compilation Pipeline](#3-compilation-pipeline)
4. [Type Mapping](#4-type-mapping)
5. [Package and Program Model](#5-package-and-program-model)
6. [Declarations and Scoping](#6-declarations-and-scoping)
7. [Functions and Methods](#7-functions-and-methods)
8. [Structs and Entity Fields](#8-structs-and-entity-fields)
9. [Control Flow](#9-control-flow)
10. [Expressions and Operators](#10-expressions-and-operators)
11. [Strings](#11-strings)
12. [Arrays and Fixed-Size Collections](#12-arrays-and-fixed-size-collections)
13. [Pointers](#13-pointers)
14. [Interfaces](#14-interfaces)
15. [Error Handling](#15-error-handling)
16. [Concurrency Mapping](#16-concurrency-mapping)
17. [Built-in Functions and Engine Bindings](#17-built-in-functions-and-engine-bindings)
18. [Runtime Support](#18-runtime-support)
19. [Calling Convention](#19-calling-convention)
20. [Optimizations](#20-optimizations)
21. [Example: End-to-End Compilation](#21-example-end-to-end-compilation)
22. [Source-Order CLI Contract (Planned)](#22-source-order-cli-contract-planned)

---

## 1. Design Goals and Non-Goals

### 1.1 Goals

- **Go syntax and semantics** — programmers write standard Go code using standard
  Go tooling (`go/ast`, `go/types`, `go/parser`)
- **Full QCVM coverage** — every QCVM capability (entity fields, builtins, string
  table, frame functions) must be expressible
- **Type safety** — leverage Go's type system to catch errors at compile time that
  QuakeC would miss at runtime
- **Interoperability** — output a standard `progs.dat` loadable by any Quake engine
  (Ironwail, QuakeSpasm, FTE, etc.)
- **Idiomatic mappings** — Go structs map to entity fields, Go interfaces map to
  behavioral contracts, Go methods map to entity behaviors

### 1.2 Non-Goals

- **Full Go compatibility** — features with no reasonable QCVM representation are
  excluded (see §2.2)
- **Standard library** — `fmt`, `os`, `net`, etc. are not available; a Quake-specific
  standard library is provided instead
- **Garbage collection** — the QCVM has no GC; entity lifecycle is managed by the
  engine's `spawn`/`remove` builtins
- **Goroutine concurrency** — the QCVM is single-threaded; `go`/`chan`/`select` are
  excluded but a think-chain scheduling model is provided (see §16)
- **Generics** — type parameters are not supported in the initial design

### 1.3 Design Principle: Annotations via Directives

Go's type system cannot express QCVM-specific concepts (builtins, entity fields, frame
numbers) natively. QGo uses **build-tag-guarded directive comments** and a special
`quake` package to bridge this gap:

```go
import "quake"         // compiler-known package providing QCVM primitives
import "quake/engine"  // engine builtin bindings
```

---

## 2. Language Subset

### 2.1 Supported Go Features

| Go Feature | QCVM Mapping | Notes |
|-----------|-------------|-------|
| Package declarations | Namespace mangling | Single output `progs.dat` |
| `var` declarations (package + local) | Global / local slots | |
| `const` declarations, `iota` | Compile-time folding | No runtime storage |
| `func` declarations | `dfunction_t` entries | Max 8 parameters |
| Multiple return values | Compiler-managed temp globals | See §7.4 |
| `func` with receiver (methods) | First-param desugaring | See §7.3 |
| `struct` types | Entity fields or global composites | See §8 |
| Pointer types (`*T`) | `ev_pointer` / entity references | See §13 |
| `if`/`else` | `OP_IFNOT_I` + `OP_GOTO` | |
| `for` (all 3 forms) | `OP_IFNOT_I` + `OP_GOTO` loops | |
| `switch` (expression) | Cascading `OP_EQ` + `OP_IFNOT_I` | |
| `break`, `continue` | `OP_GOTO` to appropriate targets | |
| `return` | `OP_RETURN` | |
| Short variable declarations (`:=`) | Local slot allocation | |
| Assignment operators (`+=`, etc.) | Compound ops or expand to op+store | |
| `++`/`--` | `OP_ADD_F` with constant 1 | |
| String literals | String table entries | Immutable |
| Numeric literals | Float or integer constants | |
| Vector literals (via `quake.Vec3`) | 3-slot vector globals | |
| Array types `[N]T` | Consecutive global slots | Fixed size only |
| Function values | `ev_function` references | No closures |
| Blank identifier `_` | Discard results | |
| Type assertions (limited) | Runtime type tag check | See §14 |
| Named types (`type X float32`) | Compile-time alias | |
| `init()` functions | Compiled into startup sequence | |

### 2.2 Excluded Go Features

| Go Feature | Reason for Exclusion |
|-----------|---------------------|
| Goroutines (`go`) | QCVM is single-threaded |
| Channels (`chan`) | No concurrency primitives |
| `select` | Requires channels |
| Slices | No dynamic memory allocation in QCVM |
| Maps | No hash table support in QCVM |
| `defer` | Would require a defer stack; see §15 for alternative |
| `panic`/`recover` | No exception mechanism; see §15 |
| Closures (capturing variables) | Requires heap allocation |
| Generics (type parameters) | Complexity; not needed for game logic |
| Complex types | No QCVM representation |
| `goto` with labels | QCVM branches are relative only; may add later |
| Multi-file packages beyond `quake/*` | Single compilation unit |
| `unsafe` | No raw memory access |
| Variadic functions (`...`) | QCVM has fixed 8-param max |

### 2.3 Restricted Features

| Go Feature | Restriction |
|-----------|------------|
| Function parameters | Maximum 8 (QCVM limit) |
| Local variables | Total slots (params + locals) must fit in global space |
| Recursion | Supported but depth limited by QCVM local stack |
| String operations | Concatenation via builtin only; no indexing or slicing |
| `for range` | Only over fixed-size arrays and entity field iterators |

---

## 3. Compilation Pipeline

QGo uses Go's standard `go/parser` and `go/types` packages for the frontend, then
performs its own lowering and code generation.

```
  .go source files
       │
       ▼
  ┌──────────────────┐
  │  go/parser        │  Parse to Go AST
  └────┬─────────────┘
       │
       ▼
  ┌──────────────────┐
  │  go/types         │  Type checking and resolution
  └────┬─────────────┘
       │
       ▼
  ┌──────────────────┐
  │  QGo IR Lowering  │  Go AST → QGo intermediate representation
  └────┬─────────────┘
       │
       ▼
  ┌──────────────────┐
  │  Code Generator   │  QGo IR → QCVM statements + globals
  └────┬─────────────┘
       │
       ▼
  ┌──────────────────┐
  │  Optimizer        │  Peephole, constant folding, dead code
  └────┬─────────────┘
       │
       ▼
  ┌──────────────────┐
  │  Linker/Emitter   │  Serialize to progs.dat
  └──────────────────┘
```

### 3.1 Frontend (Parsing + Type Checking)

The compiler uses `golang.org/x/tools/go/packages` to load and type-check the source
package and its dependencies. This provides native support for `go.mod` files,
dependency resolution, and version management.

For `.qgo` files, the compiler maintains an internal overlay that maps them to
standard `.go` filenames, allowing them to be processed by standard Go tooling.

This phase results in:
- Fully typed ASTs for the entire dependency graph
- Type information for every expression across all packages
- Resolved identifiers, constants, and import paths respecting module boundaries

### 3.2 IR Lowering

The typed Go AST is walked and lowered to a **QGo IR** — a linear sequence of
three-address instructions that maps closely to QCVM opcodes but retains type
information and virtual registers. This phase:

1. Resolves all identifiers to global offsets or local frame offsets
2. Flattens struct field accesses to entity field offset computations
3. Desugars methods to plain functions with explicit receiver parameters
4. Expands multiple return values into temp-global sequences
5. Transforms `for`/`if`/`switch` to labeled basic blocks with jumps
6. Inserts implicit type conversions (e.g., `int` → `float32` for QCVM)
7. Validates QCVM constraints (param count ≤ 8, no unsupported types)

### 3.3 Code Generation

The IR is lowered to QCVM statements:
1. Virtual registers are assigned to global offsets (with overlap optimization)
2. Labels are resolved to relative branch offsets
3. Function calls emit parameter stores to `OFS_PARM0..7` + `OP_CALL<N>`
4. The result is appended to the statements array

### 3.4 Linking and Emission

All compilation units are merged into the final `progs.dat`:
1. String table is finalized (with deduplication)
2. Global definitions, field definitions, function records are written
3. The CRC is computed for engine compatibility
4. Output follows the `dprograms_t` format (version 6 for standard engines)

---

## 4. Type Mapping

### 4.1 Primitive Type Mapping

| Go Type | QCVM Type | Slots | Notes |
|---------|-----------|-------|-------|
| `float32` | `ev_float` | 1 | Native QCVM numeric type |
| `float64` | `ev_float` | 1 | Truncated to 32-bit at compile time with warning |
| `int` | `ev_float` | 1 | Stored as float (QCVM has no native int) |
| `int32` | `ev_float` | 1 | Stored as float |
| `bool` | `ev_float` | 1 | `true` = `1.0`, `false` = `0.0` |
| `string` | `ev_string` | 1 | Offset into string table |
| `quake.Vec3` | `ev_vector` | 3 | Three consecutive float slots |
| `quake.Entity` | `ev_entity` | 1 | Entity reference |
| `quake.FieldOffset` | `ev_field` | 1 | Field offset into entity data |
| `quake.Func` | `ev_function` | 1 | Function table index |

### 4.2 Named Types

Go `type` declarations that alias or derive from the above primitives are compiled
as their underlying QCVM type. Named types exist only at compile time for type safety:

```go
type Health float32     // stored as ev_float, but Health and Damage are distinct types
type Damage float32
type Sound string       // stored as ev_string

func TakeDamage(hp *Health, dmg Damage) {
    *hp -= Health(dmg)  // explicit conversion required — catches type errors
}
```

### 4.3 The `quake` Package Types

The compiler-known `quake` package provides types that map directly to QCVM concepts:

```go
package quake

// Primitive QCVM types
type Vec3 [3]float32    // ev_vector — laid out as 3 consecutive float slots
type Entity uintptr     // ev_entity — entity reference
type Func func()        // ev_function — function reference (any signature)
type FieldOffset int    // ev_field — offset into entity field data

// Special constants
const World Entity = 0  // entity 0 is always world
```

### 4.4 Integer Representation

The standard QCVM (version 6) has no integer type — all numeric values are
`ev_float` (32-bit IEEE 754). QGo maps Go integer types as follows:

| Go Type | Representation | Precision |
|---------|---------------|-----------|
| `int`, `int32` | `ev_float` | 24-bit mantissa (~16M max exact int) |
| `int8`, `int16` | `ev_float` | Full precision within range |
| `uint8` (`byte`) | `ev_float` | Full precision |
| `bool` | `ev_float` | 0.0 or 1.0 |

The compiler emits warnings when integer operations might exceed float32 precision.
Bitwise operations (`&`, `|`, `^`, `&^`, `<<`, `>>`) are compiled using `OP_BITAND_F`
and `OP_BITOR_F` (which cast floats to ints internally).

### 4.5 Zero Values

Go's zero-value semantics are maintained:

| Go Type | Zero Value | QCVM Representation |
|---------|-----------|---------------------|
| `float32`, `int`, etc. | `0` | `0.0` in global slot |
| `bool` | `false` | `0.0` |
| `string` | `""` | `0` (offset 0 in string table = empty string) |
| `quake.Vec3` | `Vec3{}` | `0.0, 0.0, 0.0` in 3 consecutive slots |
| `quake.Entity` | `Entity(0)` | `0` (world entity / null) |
| `quake.Func` | `nil` | `0` (null function) |

---

## 5. Package and Program Model

### 5.1 Single-Package Compilation

A QGo program consists of a **single main package** plus compiler-known packages
(`quake`, `quake/engine`, etc.). All `.go` files in the main package are compiled
into one `progs.dat`.

```
mymod/
├── progs.go        // package progs — entry point
├── weapons.go      // package progs — weapon logic
├── monsters.go     // package progs — monster AI
└── quake/          // compiler-provided, not on disk
    ├── types.go    // Vec3, Entity, Func, etc.
    └── engine/
        └── builtins.go  // makevectors, setorigin, etc.
```

### 5.2 Package Clause

The main package must be named `progs`:

```go
package progs
```

This signals to the compiler that it is producing a `progs.dat` file.

### 5.3 Initialization Order

Go guarantees package-level initialization runs in dependency order, with `init()`
functions called after variable initialization. QGo compiles this into:

1. All package-level `var` declarations → initial values in the global data array
2. All `init()` functions → concatenated as a single startup function that the engine
   calls before gameplay begins (mapped to the `main` function that QC engines expect)

### 5.4 Name Mangling

Since QCVM has a flat namespace, all identifiers are mangled:

| Go Construct | Mangled Name | Example |
|-------------|-------------|---------|
| Package-level function | `funcname` | `SpawnMonster` |
| Method | `Type_Method` | `Player_TakeDamage` |
| Package-level var | `varname` | `sv_gravity` |
| Local var | No name in output | Identified by global offset only |
| Struct field (entity) | `fieldname` | `health`, `origin` |
| Init function | `__init_N` | `__init_0`, `__init_1` |

Exported identifiers (uppercase) are emitted with their original names for engine
compatibility. Unexported identifiers are prefixed with `__` to avoid collisions.

---

## 6. Declarations and Scoping

### 6.1 Package-Level Variables

```go
var gravity float32 = 800.0   // → global def "gravity" with initial value 800.0
var mapname string             // → global def "mapname" with initial value "" (offset 0)
```

Package-level variables with initializers that are **compile-time constants** are stored
directly in the global data array. Variables with non-constant initializers generate
assignment statements in the init sequence.

### 6.2 Package-Level Constants

```go
const (
    MaxHealth  float32 = 100.0
    MaxAmmo    float32 = 200.0
    GoldKey    float32 = 131072.0  // IT_KEY1
)
```

Constants are **folded inline** at every use site — no global storage is allocated.
This matches QuakeC's constant behavior exactly.

### 6.3 Iota

`iota` in `const` blocks is evaluated at compile time:

```go
const (
    ChanAuto   = iota  // 0.0
    ChanWeapon         // 1.0
    ChanVoice          // 2.0
    ChanItem           // 3.0
    ChanBody           // 4.0
)
```

Since the QCVM stores all numerics as floats, iota values are emitted as float
constants.

### 6.4 Local Variables

Local variables declared with `var` or `:=` inside a function body are allocated in
the function's local region of the global address space:

```go
func Attack(target quake.Entity, damage float32) {
    var armor float32 = target.ArmorValue  // local at parm_start+2
    remaining := damage - armor            // local at parm_start+3
    // ...
}
```

The compiler performs **liveness analysis** and overlaps locals with non-interfering
lifetimes to minimize global space usage.

### 6.5 Blank Identifier

Assignments to `_` generate the expression code (for side effects) but discard the
result — no `OP_STORE` is emitted for the blank target.

---

## 7. Functions and Methods

### 7.1 Function Declarations

```go
func SpawnMonster(class string, pos quake.Vec3, hp float32) quake.Entity {
    // ...
}
```

Compiles to a `dfunction_t` record:
- `s_name` → `"SpawnMonster"` in string table
- `numparms` → `3`
- `parm_size` → `[1, 3, 1, 0, 0, 0, 0, 0]` (string=1, Vec3=3, float=1)
- `first_statement` → index of first bytecode statement
- `locals` → total slots for 3 params + any local variables

### 7.2 Parameter Passing

Before calling a function, arguments are stored into `OFS_PARM0..7`:

```go
ent := SpawnMonster("knight", quake.Vec3{100, 200, 0}, 75.0)
```

Compiles to:
```
OP_STORE_S    "knight"_offset   OFS_PARM0         // parm0 = "knight"
OP_STORE_V    vec_100_200_0     OFS_PARM1         // parm1 = '100 200 0'
OP_STORE_F    const_75          OFS_PARM4         // parm2 = 75.0 (slot 16, after vector)
OP_CALL3      SpawnMonster                         // call with 3 args
OP_STORE_ENT  OFS_RETURN        ent_global        // ent = return value
```

Note: vector parameters consume 3 slots in the parameter region. The compiler
tracks cumulative slot offsets, not parameter indices, to determine which `OFS_PARM`
each argument maps to.

### 7.3 Methods → Desugared Functions

Go methods are compiled as plain functions with the receiver prepended as the
first parameter:

```go
type Player struct { /* fields */ }

func (p *Player) TakeDamage(damage float32) {
    // p is an entity reference
}
```

Compiles identically to:

```go
func Player_TakeDamage(p quake.Entity, damage float32) {
    // ...
}
```

Method calls `player.TakeDamage(50)` are rewritten to
`Player_TakeDamage(player, 50)`.

### 7.4 Multiple Return Values

The QCVM supports only a single return value (3 slots max for vectors). QGo supports
multiple return values by using a **continuation-passing style** with temp globals:

```go
func FindTarget() (quake.Entity, bool) {
    // ...
    return target, true
}

ent, ok := FindTarget()
```

**Strategy:** The compiler allocates **dedicated temp globals** for secondary return
values. The callee writes the primary return into `OFS_RETURN` and secondary values
into known temp globals. The caller reads them immediately after the call:

```
// Inside FindTarget:
OP_STORE_ENT  target          OFS_RETURN         // primary return
OP_STORE_F    const_1         __retval_1         // secondary return (bool)
OP_RETURN     OFS_RETURN

// At call site:
OP_CALL0      FindTarget
OP_STORE_ENT  OFS_RETURN      ent_global         // ent = primary
OP_STORE_F    __retval_1      ok_global          // ok = secondary
```

The temp globals `__retval_1`, `__retval_2`, etc. are reserved in the global space
after the standard reserved region. This is safe because QCVM execution is
single-threaded — no concurrent call can overwrite them between the return and the
caller's read.

**Limitation:** This approach is fragile across nested calls. The compiler must ensure
secondary return values are consumed before any subsequent function calls. If a nested
call is detected, the compiler inserts spill/reload instructions to local slots.

### 7.5 Function Values

Go function values (non-closure) map directly to `ev_function`:

```go
var callback func(quake.Entity)  // stored as ev_function in a global slot
callback = Player_TakeDamage     // store function reference
callback(self)                   // OP_CALL1 using the function reference
```

The compiler verifies at call sites that the number and types of arguments match
the function signature stored in the type system (this is a compile-time check only —
the QCVM does no runtime type checking on function calls).

### 7.6 Init Functions

Each `init()` function compiles to a regular `dfunction_t`. The compiler generates
a master `__progs_init` function that calls each `init()` in source order. This
master init is wired as the first thing the engine's `main`/`worldspawn` calls.

---

## 8. Structs and Entity Fields

This is the most important mapping in the compiler — it bridges Go's struct system
with the QCVM's entity field model.

### 8.1 Two Kinds of Structs

QGo distinguishes between two uses of structs:

#### Entity Structs (tagged with `quake.EntityType`)

These map to QCVM entity field definitions. Each field becomes a field definition
in `progs.dat`'s field table:

```go
//qgo:entity
type BaseEntity struct {
    ModelIndex float32  `qgo:"modelindex"`
    AbsMin     quake.Vec3 `qgo:"absmin"`
    AbsMax     quake.Vec3 `qgo:"absmax"`
    // ... standard Quake entity fields
}

//qgo:entity
type Monster struct {
    BaseEntity              // embedded = fields are inlined
    Health     float32      `qgo:"health"`
    Enemy      quake.Entity `qgo:"enemy"`
    AttackFunc quake.Func   `qgo:"th_run"`
}
```

The `//qgo:entity` directive tells the compiler to emit field definitions. The `qgo`
struct tag provides the QCVM field name (for engine compatibility). Embedding composes
fields like QuakeC's flat entity model.

#### Value Structs (no directive)

These are plain data aggregates laid out as consecutive global slots:

```go
type Rect struct {
    X, Y, W, H float32  // 4 consecutive global slots
}

var menuBounds Rect  // allocates 4 globals
```

Value structs are passed by copying all their slots. They cannot be used as entity
fields (entities use the field system, not global slots).

### 8.2 Entity Field Layout

Entity fields must be laid out in a specific order:
1. **System fields** — shared with the engine (`modelindex`, `origin`, `angles`, etc.)
   These must match the engine's `entvars_t` layout exactly.
2. **`end_sys_fields`** — boundary marker
3. **Custom fields** — game-specific fields (`health`, `enemy`, etc.)

QGo ships a `quake/entity` package defining the system fields struct. User-defined
entity structs embed this and add custom fields:

```go
import "quake/entity"

//qgo:entity
type GameEntity struct {
    entity.Base         // system fields in correct order
    Health  float32     `qgo:"health"`
    Armor   float32     `qgo:"armorvalue"`
}
```

### 8.3 Field Access Compilation

```go
hp := self.Health           // read entity field
self.Health = hp - damage   // write entity field
```

**Read** compiles to:
```
OP_LOAD_F   self_global   health_field_ofs   temp
OP_STORE_F  temp          hp_local
```

**Write** compiles to:
```
OP_SUB_F    hp_local      damage_local       temp
OP_ADDRESS  self_global   health_field_ofs   ptr_temp
OP_STOREP_F temp          ptr_temp
```

### 8.4 Field References

Go expressions like `&Monster.Health` produce a field offset value (`ev_field`). This
is useful for generic field operations:

```go
field := quake.FieldOf(&Monster{}.Health)  // compile-time field offset
value := quake.GetField(ent, field)        // OP_LOAD_F at runtime
```

---

## 9. Control Flow

### 9.1 If/Else

```go
if health <= 0 {
    Die()
} else {
    Flinch()
}
```

Compiles to:
```
OP_LE_F     health   const_0   temp        // temp = (health <= 0)
OP_IFNOT_I  temp     +3                    // if !temp, skip to else
<Die() call statements>
OP_GOTO     +2                             // skip else block
<Flinch() call statements>                 // else block
```

### 9.2 For Loops

#### Condition-only for (while loop):

```go
for health > 0 {
    // ...
}
```

```
label_top:
OP_GT_F     health   const_0   temp
OP_IFNOT_I  temp     +N                    // exit loop
<body statements>
OP_GOTO     -(offset_to_top)               // back to top
label_exit:
```

#### Three-clause for:

```go
for i := 0; i < 10; i++ {
    // ...
}
```

Desugared to init + condition-only loop + post:

```
OP_STORE_F  const_0   i_local              // init: i = 0
label_top:
OP_LT_F     i_local   const_10   temp      // condition: i < 10
OP_IFNOT_I  temp      +N                   // exit if false
<body statements>
OP_ADD_F    i_local   const_1    i_local   // post: i++
OP_GOTO     -(offset_to_top)
label_exit:
```

#### For-range over arrays:

```go
var items [5]float32
for i, v := range items {
    // i is index (float), v is value
}
```

Desugared to a three-clause for over the array's compile-time-known length,
using `OP_FETCH_GBL_F` (Hexen 2 opcode 80) for indexed access if available,
or computed global offsets for standard QCVM.

### 9.3 Switch

```go
switch weapon {
case AxeWeapon:
    SwingAxe()
case ShotgunWeapon:
    FireShotgun()
default:
    // nothing
}
```

Compiles to a **cascading if-else chain**:

```
OP_EQ_F     weapon   AxeWeapon    temp
OP_IF_I     temp     +N1                   // jump to SwingAxe
OP_EQ_F     weapon   ShotgunWeapon temp
OP_IF_I     temp     +N2                   // jump to FireShotgun
OP_GOTO     +N3                            // jump to default/end
<SwingAxe call>
OP_GOTO     +end
<FireShotgun call>
OP_GOTO     +end
<default block>
label_end:
```

For Hexen 2 targets, `OP_SWITCH_F` / `OP_CASE` opcodes can be used instead.

### 9.4 Break and Continue

`break` compiles to `OP_GOTO` targeting the loop exit label.
`continue` compiles to `OP_GOTO` targeting the loop's post-statement (or condition
re-check for condition-only loops).

The compiler maintains a **break/continue stack** that tracks the innermost enclosing
loop. Labeled `break`/`continue` (e.g., `break outer`) are resolved at IR lowering
time to the correct loop's labels.

---

## 10. Expressions and Operators

### 10.1 Arithmetic Operators

| Go Operator | QCVM Opcode | Types |
|------------|-------------|-------|
| `+` | `OP_ADD_F` / `OP_ADD_V` | float / Vec3 |
| `-` | `OP_SUB_F` / `OP_SUB_V` | float / Vec3 |
| `*` | `OP_MUL_F` / `OP_MUL_FV` / `OP_MUL_VF` | float, float×vec, vec×float |
| `/` | `OP_DIV_F` | float |
| `%` | Emulated: `a - float32(int(a/b)) * b` | float (integer semantics) |

### 10.2 Comparison Operators

| Go Operator | QCVM Opcode | Notes |
|------------|-------------|-------|
| `==` | `OP_EQ_F` / `OP_EQ_V` / `OP_EQ_S` / `OP_EQ_E` / `OP_EQ_FNC` | Type-specific |
| `!=` | `OP_NE_F` / `OP_NE_V` / `OP_NE_S` / `OP_NE_E` / `OP_NE_FNC` | Type-specific |
| `<` | `OP_LT_F` | Float only |
| `>` | `OP_GT_F` | Float only |
| `<=` | `OP_LE_F` | Float only |
| `>=` | `OP_GE_F` | Float only |

### 10.3 Logical Operators

| Go Operator | QCVM Opcode | Notes |
|------------|-------------|-------|
| `&&` | `OP_AND_F` | **Not** short-circuit in standard QCVM |
| `\|\|` | `OP_OR_F` | **Not** short-circuit in standard QCVM |
| `!` | `OP_NOT_F` / `OP_NOT_V` / `OP_NOT_S` / `OP_NOT_ENT` / `OP_NOT_FNC` | Type-specific |

**Important:** Go specifies short-circuit evaluation for `&&` and `||`. Since QCVM's
`OP_AND_F`/`OP_OR_F` evaluate both operands, QGo must emit **branch-based
short-circuit code** when operand evaluation has side effects:

```go
if a > 0 && ExpensiveCheck() {  // must not call ExpensiveCheck if a <= 0
```

Compiles to:
```
OP_GT_F     a          const_0     temp1
OP_IFNOT_I temp1      +skip                // short-circuit: skip rhs
<ExpensiveCheck() call>
OP_STORE_F  OFS_RETURN temp2
OP_AND_F    temp1      temp2       result
OP_GOTO     +end
label_skip:
OP_STORE_F  const_0    result              // false
label_end:
```

When neither operand has side effects, the compiler may use `OP_AND_F`/`OP_OR_F`
directly as an optimization.

### 10.4 Bitwise Operators

| Go Operator | QCVM Opcode | Notes |
|------------|-------------|-------|
| `&` | `OP_BITAND_F` | Float reinterpreted as int bits |
| `\|` | `OP_BITOR_F` | Float reinterpreted as int bits |
| `^` (XOR) | Emulated: `(a \| b) & !(a & b)` | Or use FTEQCC `OP_BITXOR_F` |
| `&^` (AND NOT) | Emulated: `a & !(a & b)` | Or Hexen 2 `OP_BITCLR` |
| `<<` | Emulated: multiply by power of 2 | Constant shifts only |
| `>>` | Emulated: divide by power of 2 | Constant shifts only |

For non-constant shifts, the compiler emits a helper loop or targets FTEQCC extended
opcodes (`OP_LSHIFT_I`, `OP_RSHIFT_I`).

### 10.5 String Concatenation

Go's `+` on strings cannot be implemented directly (QCVM strings are immutable offsets).
The compiler rewrites string concatenation to engine builtin calls:

```go
msg := "Hello " + name + "!"
```

Compiles to:
```
OP_STORE_S  "Hello "_ofs    OFS_PARM0
OP_STORE_S  name_global     OFS_PARM1
OP_CALL2    strcat_builtin
OP_STORE_S  OFS_RETURN      temp1
OP_STORE_S  temp1           OFS_PARM0
OP_STORE_S  "!"_ofs         OFS_PARM1
OP_CALL2    strcat_builtin
OP_STORE_S  OFS_RETURN      msg_global
```

### 10.6 Assignment Operators

| Go Operator | Compilation |
|------------|-------------|
| `=` | `OP_STORE_*` |
| `+=` | `OP_ADD_F` + `OP_STORE_F` (or `OP_ADDSTORE_F` on H2 targets) |
| `-=` | `OP_SUB_F` + `OP_STORE_F` |
| `*=` | `OP_MUL_F` + `OP_STORE_F` |
| `/=` | `OP_DIV_F` + `OP_STORE_F` |
| `++` | `OP_ADD_F` with constant `1.0` |
| `--` | `OP_SUB_F` with constant `1.0` |

### 10.7 Type Conversions

Explicit type conversions between Go numeric types compile to no-ops (all are `ev_float`
internally) or emit conversion opcodes when targeting FTEQCC:

```go
i := int(f)        // no-op on standard QCVM; OP_CONV_FTOI on FTE
f := float32(i)    // no-op on standard QCVM; OP_CONV_ITOF on FTE
```

### 10.8 Dot Product and Vector Operations

The `*` operator between two `quake.Vec3` values compiles to `OP_MUL_V` (dot product):

```go
cosAngle := dir1 * dir2   // OP_MUL_V → float result
```

Cross product and other vector operations are provided as `quake` package functions
that map to engine builtins or inline sequences.

---

## 11. Strings

### 11.1 String Representation

Go strings are stored as `ev_string` — an offset into the progs string table. They
are **immutable** at the QCVM level.

### 11.2 String Operations

| Go Operation | Implementation |
|-------------|----------------|
| String literal | Entry in string table, offset stored in global |
| `==`, `!=` | `OP_EQ_S`, `OP_NE_S` |
| `+` (concat) | Rewritten to `engine.Strcat()` builtin chain |
| `len(s)` | Rewritten to `engine.Strlen()` builtin |
| Conversion `string(f)` | Rewritten to `engine.Ftos()` builtin |
| Conversion `float32(s)` | Rewritten to `engine.Stof()` builtin |

### 11.3 String Interpolation Helper

QGo provides a compile-time helper for common patterns:

```go
msg := quake.Sprintf("%s has %d health", name, hp)
```

This is expanded at compile time to a chain of `ftos`/`strcat` builtin calls. The
format string is parsed at compile time — no runtime format parsing needed.

---

## 12. Arrays and Fixed-Size Collections

### 12.1 Array Types

Fixed-size arrays are supported and map to consecutive global slots:

```go
var ammo [4]float32  // 4 consecutive float globals: ammo[0]..ammo[3]
```

### 12.2 Array Access

Constant index access is resolved at compile time to the correct global offset:

```go
ammo[2] = 50.0  // OP_STORE_F to ammo_base+2
```

Variable index access requires bounds checking and uses `OP_FETCH_GBL_F` on Hexen 2
targets, or a computed address on standard QCVM:

```go
ammo[i] = 50.0  // variable index — needs bounds check
```

On standard QCVM (no array opcodes), variable-index access is compiled as a
sequence of comparisons:

```
// ammo[i] = 50.0 where ammo is at global offset G and has 4 elements
OP_EQ_F     i   const_0   temp
OP_IF_I     temp  +store0
OP_EQ_F     i   const_1   temp
OP_IF_I     temp  +store1
OP_EQ_F     i   const_2   temp
OP_IF_I     temp  +store2
OP_EQ_F     i   const_3   temp
OP_IF_I     temp  +store3
OP_GOTO     +end                          // bounds error / no-op
store0: OP_STORE_F const_50 global_G+0; OP_GOTO +end
store1: OP_STORE_F const_50 global_G+1; OP_GOTO +end
store2: OP_STORE_F const_50 global_G+2; OP_GOTO +end
store3: OP_STORE_F const_50 global_G+3; OP_GOTO +end
label_end:
```

For small arrays (≤8 elements), this is acceptable. For larger arrays, the compiler
generates helper functions (like QuakeC's `ArrayGet`/`ArraySet` patterns). An
alternative for H2/FTE targets uses `OP_FETCH_GBL_*` / `OP_GLOBALADDRESS`.

### 12.3 Array Limitations

- **No slices** — array size must be a compile-time constant
- **No dynamic allocation** — arrays are global or local, never heap-allocated
- **Maximum practical size** — limited by global address space (~65K slots for 16-bit)

---

## 13. Pointers

### 13.1 Entity Pointers

The primary use of pointers in QGo is **entity references**. A `*Monster` value is
an `ev_entity` — a reference to an entity whose fields are laid out per the `Monster`
struct:

```go
func SpawnKnight() *Monster {
    ent := engine.Spawn()           // returns quake.Entity
    knight := (*Monster)(ent)       // type assertion — compile-time only
    knight.Health = 75
    knight.Model = "progs/knight.mdl"
    return knight
}
```

The `(*Monster)(ent)` cast is a compile-time annotation — no runtime code is emitted.
It tells the type system that `ent`'s fields should be accessed using the `Monster`
field layout.

### 13.2 Pointer to Global Variables

Taking the address of a global variable produces an `ev_pointer` that can be used
with `OP_STOREP_*`:

```go
func Increment(p *float32) {
    *p += 1.0
}
```

**Within entity fields:** Compiles to `OP_ADDRESS` + `OP_STOREP_F`.
**Within globals:** Uses the global's offset directly (since globals are addressable
by offset in the QCVM address space).

### 13.3 Nil Pointers

Go's `nil` for pointer types maps to `0` in the QCVM global. For entities, this is
the `world` entity (entity 0). The compiler emits `OP_NOT_ENT` for nil checks:

```go
if target != nil {  // OP_NOT_ENT target → OP_IFNOT_I
```

---

## 14. Interfaces

### 14.1 Concept

Go interfaces provide behavioral polymorphism. In the QCVM, the closest analog is
**function pointer fields** on entities — different entity types can have different
functions assigned to the same field.

### 14.2 Interface Mapping Strategy

QGo maps interfaces to **entity field contracts**:

```go
type Damageable interface {
    TakeDamage(damage float32)
    Health() float32
}
```

This declares that any entity struct implementing `Damageable` must have:
- A `TakeDamage` method → stored as a `quake.Func` field on the entity
- A `Health` method → stored as a `quake.Func` field on the entity

### 14.3 Method Dispatch

When calling a method through an interface, the compiler emits an **indirect call**
via the entity's function field:

```go
func ApplyDamage(target Damageable, dmg float32) {
    target.TakeDamage(dmg)
}
```

Compiles to:
```
// target is an entity reference; TakeDamage is a function field
OP_LOAD_FNC  target   takedamage_field   temp_func
OP_STORE_ENT target   OFS_PARM0                      // self = target
OP_STORE_F   dmg      OFS_PARM1                      // damage
OP_CALL2     temp_func                                // indirect call
```

This matches the standard Quake pattern where entity behavior is controlled by
function-valued fields like `.th_run`, `.th_pain`, `.th_die`.

### 14.4 Interface Satisfaction

The compiler verifies at compile time that a struct type satisfies an interface.
Satisfaction means:
1. For each interface method, the struct has a corresponding `quake.Func` field
2. The field is assigned a function with a compatible signature during construction

There is **no runtime interface value** (no fat pointer / itab). Interfaces exist
purely as compile-time constraints.

### 14.5 Type Assertions

Type assertions and type switches are currently **not supported**.

Current compiler behavior:
- `x.(*T)` fails during lowering with: `unsupported expression type: *ast.TypeAssertExpr`
- `switch x.(type)` fails during lowering with: `unsupported statement type: *ast.TypeSwitchStmt`

Rationale for the near-term scope:
- The current lowering/codegen pipeline has no runtime type metadata, no interface-value representation, and no canonical entity-type tag abstraction beyond ad-hoc `classname` string usage.
- A "quick" implementation based only on `classname` string compares would hardcode one game-data convention into the compiler and risk non-portable behavior.
- Until a stable runtime type-tag model is designed and tested, qgo keeps these forms explicitly unsupported.

### 14.6 Dynamic Entity Field Access

Dynamic entity field access is **partially enabled** via a narrow intrinsic helper seam.

Current status:
- Static selector access (`ent.Health`) is lowered and emitted.
- The import/body-isolation prerequisite is complete: lowering no longer descends into imported package bodies.
- The compiler now recognizes `quake.FieldFloat` and `quake.SetFieldFloat` as intrinsics and lowers them directly to field opcodes.

Current limitations:
- The intrinsic surface is intentionally narrow in this slice (`FieldFloat`/`SetFieldFloat` only).
- Broader dynamic syntax and additional helper families (`FieldVector`, `FieldString`, etc.) remain deferred.
- Generic imported helper calls are still not a safe fallback because imported package functions are not lowered into target-package IR bodies.

Implemented unblock slice:
1. Compiler-recognized intrinsics with strict type gating (`entity`, `field offset`, and `float` value where applicable):
   - `quake.FieldFloat(entity, fieldOffset) float32`
   - `quake.SetFieldFloat(entity, fieldOffset, value float32)`
2. Direct opcode lowering:
   - `FieldFloat` → `OP_LOAD_F`
   - `SetFieldFloat` → `OP_ADDRESS` + `OP_STOREP_F`
3. Focused compiler tests assert opcode presence, negative type-gating failures, and
   compile→VM round-trip execution for dynamic float field read/write.

---

## 15. Error Handling

### 15.1 No Panic/Recover

The QCVM has no exception mechanism. `panic()` and `recover()` are not supported.
Instead, QGo provides:

### 15.2 Error Return Pattern

The Go idiom of returning errors works naturally:

```go
type Error struct {
    Code    float32
    Message string
}

func LoadMap(name string) (*Level, *Error) {
    if !engine.FileExists(name) {
        return nil, &Error{1, "map not found"}
    }
    // ...
    return level, nil
}
```

Multiple return values (§7.4) carry the error information without exceptions.

### 15.3 Fatal Errors

For unrecoverable errors, QGo provides:

```go
quake.Error("something went horribly wrong")  // calls engine's error builtin → kills server
```

This maps to the `error()` engine builtin, which halts execution.

### 15.4 No Defer

`defer` is not supported because it would require maintaining a per-function defer
stack in the QCVM's limited global space. Cleanup should be done explicitly:

```go
// Instead of:
// f := OpenFile(name)
// defer f.Close()
// ... use f ...

// Do:
f := OpenFile(name)
// ... use f ...
CloseFile(f)
```

---

## 16. Concurrency Mapping

### 16.1 No Goroutines

The QCVM is single-threaded. Goroutines and channels are not supported.

### 16.2 Think-Chain Scheduling

Quake's entity system provides cooperative multitasking through the **think chain**:
each entity can schedule its next think function to run at a future time. QGo
exposes this through a natural Go API:

```go
//qgo:entity
type Monster struct {
    entity.Base
    Health    float32
    ThinkFunc quake.Func  `qgo:"think"`
    NextThink float32     `qgo:"nextthink"`
}

// Schedule a think function
func (m *Monster) ScheduleThink(fn func(), delay float32) {
    m.ThinkFunc = quake.Func(fn)
    m.NextThink = engine.Time() + delay
}

// Usage:
func (m *Monster) StartPatrol() {
    m.ScheduleThink(m.PatrolStep, 0.1)  // run PatrolStep in 0.1s
}

func (m *Monster) PatrolStep() {
    // do patrol logic
    m.ScheduleThink(m.PatrolStep, 0.1)  // schedule next step
}
```

### 16.3 Frame Functions

For model animation, QGo supports frame function syntax via a directive:

```go
//qgo:frames walk1 walk2 walk3 walk4

//qgo:frame walk1 → WalkStep2
func (m *Monster) WalkStep1() {
    // animation logic
}
```

This compiles to the equivalent of QuakeC's `OP_STATE` instruction:
```
OP_STATE    walk1_framenum   WalkStep2_func
<body statements>
```

Alternatively, the frame function pattern can be expressed with the think chain API
directly.

---

## 17. Built-in Functions and Engine Bindings

### 17.1 The `quake/engine` Package

Engine builtins are declared in the synthetic `quake/engine` package:

```go
package engine

// Builtins are declared with //qgo:builtin directives

//qgo:builtin 1
func MakeVectors(ang quake.Vec3)

//qgo:builtin 2
func SetOrigin(ent quake.Entity, org quake.Vec3)

//qgo:builtin 3
func SetModel(ent quake.Entity, model string)

//qgo:builtin 4
func SetSize(ent quake.Entity, min, max quake.Vec3)

//qgo:builtin 7
func BreakStatement()

//qgo:builtin 8
func Random() float32

//qgo:builtin 9
func Sound(ent quake.Entity, chan_ float32, sample string, volume, attenuation float32)

//qgo:builtin 12
func Normalize(v quake.Vec3) quake.Vec3

//qgo:builtin 14
func Spawn() quake.Entity

//qgo:builtin 15
func Remove(ent quake.Entity)

// ... etc for all standard builtins
```

Each `//qgo:builtin N` directive causes the compiler to emit a `dfunction_t` with
`first_statement = -N` (negative = builtin number). Calls to these functions generate
normal `OP_CALL` instructions — the VM handles the dispatch to native code.

### 17.2 Engine Global Variables

Certain QCVM globals are shared with the engine and must be at specific offsets.
These are declared in `quake/engine`:

```go
package engine

//qgo:engineglobal
var (
    Self       quake.Entity   // must match engine's global layout
    Other      quake.Entity
    World      quake.Entity
    Time       float32
    FrameTime  float32
    MapName    string
    // ... etc
)
```

The `//qgo:engineglobal` directive tells the compiler to emit these in the exact
order and at the exact offsets the engine expects.

### 17.3 Console Variables

Quake console variables are accessed via builtins:

```go
gravity := engine.CvarGet("sv_gravity")      // returns float32
engine.CvarSet("sv_gravity", 400)
```

---

## 18. Runtime Support

### 18.1 Startup Sequence

The compiled `progs.dat` must expose certain well-known functions that the engine
calls at specific points:

| Engine Callback | QGo Mapping |
|----------------|-------------|
| `main` | Package `init()` functions + global init |
| `StartFrame` | `func StartFrame()` at package level |
| `PlayerPreThink` | `func PlayerPreThink()` at package level |
| `PlayerPostThink` | `func PlayerPostThink()` at package level |
| `ClientConnect` | `func ClientConnect()` at package level |
| `PutClientInServer` | `func PutClientInServer()` at package level |
| Spawn functions | `func SpawnMonsterKnight()` etc. (prefixed `Spawn` maps to `monster_knight`) |

The compiler recognizes these well-known function names and emits them with the
correct names in the progs string table.

### 18.2 Entity Spawn Functions

In Quake, entities are spawned by calling a function named after their classname.
QGo maps this with a naming convention:

```go
// This function is called when the engine spawns a "monster_knight" entity
func SpawnMonsterKnight() {
    self := engine.Self
    knight := (*Monster)(self)
    knight.Health = 75
    engine.SetModel(self, "progs/knight.mdl")
    // ...
}
```

The compiler emits this as a function named `monster_knight` in the progs (converting
from Go's PascalCase convention: `SpawnMonsterKnight` → strip `Spawn` → `MonsterKnight`
→ snake_case → `monster_knight`).

### 18.3 CRC Compatibility

The compiler computes the `progdefs.h` CRC to ensure the global and entity field
layout matches what the engine expects. The `quake/entity` package's system fields
define this layout, and the CRC is computed during emission.

---

## 19. Calling Convention

### 19.1 Standard Call Sequence

QGo follows the standard QCVM calling convention (see QCC_SPEC.md §14):

1. **Evaluate arguments** left-to-right
2. **Store** each argument into `OFS_PARM0..7` via `OP_STORE_*`
   - Scalar types use 1 slot per parameter
   - `quake.Vec3` uses 3 slots (consuming a full `OFS_PARMn` group)
3. **Emit** `OP_CALL<N>` where N is the parameter count
4. **Read** return value from `OFS_RETURN`

### 19.2 Slot Accounting

The compiler tracks the **cumulative slot offset** for parameters, not just the count:

```go
func Example(a float32, b quake.Vec3, c float32) float32
//           parm0(1)   parm1(3)     parm4(1)
//           slot 4     slots 7-9    slot 16
```

Parameter `c` lands at `OFS_PARM4` (slot 16) because `b` consumed `OFS_PARM1`
through `OFS_PARM3` (slots 7–15). The `OP_CALL` opcode number reflects the
**parameter count** (3), not the slot count.

### 19.3 Large Struct Arguments

Value structs larger than 3 slots cannot be passed directly (QCVM parameters are
max 3 slots each). Options:
- **Flatten:** Structs ≤24 slots (8 params × 3 slots) are flattened into individual
  parameters
- **Pointer:** Larger structs must be passed by pointer (entity reference or global
  address)

The compiler automatically selects the strategy based on struct size.

---

## 20. Optimizations

### 20.1 Compiler Optimizations

QGo applies standard optimizations at the IR level:

| Optimization | Description |
|-------------|-------------|
| Constant folding | `2.0 + 3.0` → `5.0` at compile time |
| Dead code elimination | Unreachable code after `return` is removed |
| Temp overlapping | Temporaries with non-overlapping lifetimes share global slots |
| Local overlapping | Locals from non-concurrent functions share global slots |
| String deduplication | Identical string literals share one string table entry |
| Constant deduplication | Constants with the same value share one global slot |
| Compound jump collapsing | `GOTO → GOTO → target` becomes `GOTO → target` |
| Identity removal | `x = x` assignments are eliminated |
| Short-circuit elision | `&&`/`||` with side-effect-free operands use `OP_AND_F`/`OP_OR_F` |
| Inline expansion | Small functions (≤5 statements) can be inlined at call sites |

### 20.2 QCVM-Specific Optimizations

| Optimization | Description |
|-------------|-------------|
| Return-only elimination | Functions ending in `OP_RETURN` skip redundant `OP_DONE` |
| Parm-direct | When an argument is a simple variable, check if it's already in the right parm slot |
| Field cache | Repeated reads of the same entity field reuse the loaded value |
| Const-name stripping | Definition names for constants are omitted (saves space) |
| Unused function stripping | Functions never called or referenced are removed |

---

## 21. Example: End-to-End Compilation

### 21.1 Source Code

```go
package progs

import (
    "quake"
    "quake/engine"
)

//qgo:entity
type Knight struct {
    entity.Base
    Health float32 `qgo:"health"`
}

func (k *Knight) TakeDamage(damage float32) {
    k.Health -= damage
    if k.Health <= 0 {
        k.Die()
    }
}

func (k *Knight) Die() {
    engine.BPrint(k.NetName() + " has died\n")
    engine.Remove(quake.Entity(k))
}

func SpawnMonsterKnight() {
    self := (*Knight)(engine.Self)
    self.Health = 75
    engine.SetModel(quake.Entity(self), "progs/knight.mdl")
}
```

### 21.2 Compilation Trace

#### Phase 1: Parse + Type Check

`go/parser` produces an AST. `go/types` resolves:
- `Knight` is an entity struct with fields `entity.Base` (system) + `Health` (custom)
- `TakeDamage` and `Die` are methods with `*Knight` receiver
- `SpawnMonsterKnight` is a spawn function

#### Phase 2: IR Lowering

**Field definitions emitted:**
- All `entity.Base` system fields (matching engine layout)
- `health` at field offset N (after system fields)

**Functions lowered:**

`Knight_TakeDamage(k entity, damage float)`:
```ir
  %hp = LOAD_F k, health_field
  %sub = SUB_F %hp, damage
  ADDRESS k, health_field → %ptr
  STOREP_F %sub, %ptr
  %cmp = LE_F %sub, 0.0
  IFNOT %cmp, skip_die
  STORE_ENT k → OFS_PARM0
  CALL1 Knight_Die
skip_die:
  DONE
```

`Knight_Die(k entity)`:
```ir
  %name = LOAD_S k, netname_field
  STORE_S %name → OFS_PARM0
  STORE_S " has died\n" → OFS_PARM1
  CALL2 strcat
  STORE_S OFS_RETURN → OFS_PARM0
  CALL1 bprint
  STORE_ENT k → OFS_PARM0
  CALL1 remove
  DONE
```

`monster_knight()`:
```ir
  %self = LOAD engine_self_global
  ADDRESS %self, health_field → %ptr
  STOREP_F 75.0, %ptr
  STORE_ENT %self → OFS_PARM0
  STORE_S "progs/knight.mdl" → OFS_PARM1
  CALL2 setmodel
  DONE
```

#### Phase 3: Code Generation

IR virtual registers are assigned to concrete global offsets. Branch labels are
resolved to relative offsets. Statements are appended to the statement array.

#### Phase 4: Emission

The final `progs.dat` contains:
- **Header**: version=6, CRC matching standard Quake
- **Statements**: ~25 statements for the three functions
- **Global defs**: `self`, engine globals, temps
- **Field defs**: system fields + `health`
- **Functions**: `Knight_TakeDamage`, `Knight_Die`, `monster_knight`, builtins
- **Strings**: `"health"`, `"progs/knight.mdl"`, `" has died\n"`, function/file names
- **Globals**: initial values for all global slots

---

## Appendix A: The `quake` Package API

```go
package quake

// Core QCVM types
type Vec3 [3]float32
type Entity uintptr
type Func func()
type FieldOffset int

// Constants
const World Entity = 0

// Vector construction
func MakeVec3(x, y, z float32) Vec3

// Error (fatal — calls engine error builtin)
func Error(msg string)

// Sprintf-like helper (compile-time expansion)
func Sprintf(format string, args ...interface{}) string
```

## Appendix B: The `quake/engine` Package (Subset)

```go
package engine

import "quake"

// Movement and position
//qgo:builtin 1
func MakeVectors(ang quake.Vec3)

//qgo:builtin 2
func SetOrigin(ent quake.Entity, org quake.Vec3)

//qgo:builtin 3
func SetModel(ent quake.Entity, model string)

//qgo:builtin 4
func SetSize(ent quake.Entity, min quake.Vec3, max quake.Vec3)

// Utility
//qgo:builtin 8
func Random() float32

// Sound
//qgo:builtin 9
func Sound(ent quake.Entity, channel float32, sample string, volume float32, attenuation float32)

// Entity management
//qgo:builtin 14
func Spawn() quake.Entity

//qgo:builtin 15
func Remove(ent quake.Entity)

// String operations
//qgo:builtin 26
func Ftos(f float32) string

//qgo:builtin 27
func Vtos(v quake.Vec3) string

// Tracing
//qgo:builtin 16
func TraceLine(v1 quake.Vec3, v2 quake.Vec3, nomonsters float32, forent quake.Entity)

// Printing
//qgo:builtin 23
func BPrint(msg string)

//qgo:builtin 24
func SPrint(ent quake.Entity, msg string)

// Console variables
//qgo:builtin 45
func CvarGet(name string) string

//qgo:builtin 72
func CvarSet(name string, value string)

// ... (full list of ~80 standard builtins)
```

## Appendix C: Directive Reference

| Directive | Placement | Purpose |
|-----------|-----------|---------|
| `//qgo:entity` | Before struct type | Marks struct as entity field layout |
| `//qgo:builtin N` | Before func decl | Maps function to engine builtin #N |
| `//qgo:engineglobal` | Before var block | Marks globals as engine-shared |
| `//qgo:frames name1 name2 ...` | Package level | Defines model animation frame names |
| `//qgo:frame NAME → NEXT` | Before method | Marks method as frame function |
| `//qgo:saveglobal` | Before var decl | Marks global for inclusion in save files |
| `` `qgo:"fieldname"` `` | Struct field tag | Specifies QCVM field name |

## Appendix D: Mapping Summary

---

## 22. Source-Order CLI Contract (Planned)

This section defines the contract for a future `qgo` source-order helper so it can
be implemented in smaller slices without revisiting external behavior decisions.

### 22.1 Purpose

The tool provides a deterministic, inspectable ordering view for source units that
contribute to final `progs.dat` symbol/function layout. It is for parity tooling and
debugging, not a replacement for compilation.

### 22.2 Command Shape

```text
qgo source-order [flags] [dir]
```

- `[dir]` defaults to `.` and must point to a single Go package directory.
- `qgo source-order` reads `.go` and `.qgo` sources using the same package discovery
  boundary as `qgo` compile.
- The command does not write `progs.dat`.

### 22.3 Flags

- `-format <text|json>` (default: `text`)
  - `text`: stable human-readable lines
  - `json`: machine-readable deterministic payload
- `-scope <files|functions>` (default: `functions`)
  - `files`: only file order
  - `functions`: file order plus declared function order
- `-o <path>` (optional)
  - when omitted, writes to stdout
  - when provided, writes the selected format to file
- `-strict` (default: false)
  - when true, treat unsupported/ambiguous declarations as hard errors

### 22.4 Output Contract

#### Text format (`-format text`)

- UTF-8, newline-delimited, deterministic ordering.
- No timestamps, durations, or host-specific paths.
- Relative paths are normalized to slash-separated paths from `[dir]`.

`-scope files` line format:

```text
<index>\t<relative-file>
```

`-scope functions` line format:

```text
<index>\t<relative-file>\t<function-name>
```

#### JSON format (`-format json`)

Top-level object:

```json
{
  "version": 1,
  "dir": ".",
  "scope": "functions",
  "order": [
    { "index": 0, "file": "a_first.qgo", "function": "Able" }
  ]
}
```

Rules:
- keys are stable and always emitted in the order shown above
- `order` entries are zero-indexed and contiguous
- when `-scope files`, each entry omits `function`

### 22.5 Stderr / Exit Status Contract

- Success: exit status `0`
- Failure: exit status `1`
- All diagnostics are emitted to stderr with `qgo:` prefix, matching the existing CLI
  error style (`qgo: <message>`).
- stdout is reserved strictly for requested output payloads.

### 22.6 Determinism Requirements

- identical inputs produce byte-identical output for the same flags
- ordering is independent of filesystem enumeration order
- path normalization and sort behavior are platform-stable
- this command must reuse compiler traversal/source-order semantics already validated
  by deterministic tests, rather than introducing a second ordering algorithm

### 22.7 Initial Non-Goals

- not responsible for parity diffing against `.qc` output
- not responsible for proving semantic equivalence of generated bytecode
- not responsible for multi-package workspace orchestration in the first slice

```
┌─────────────────────┐         ┌──────────────────────┐
│      Go World        │         │     QCVM World        │
├─────────────────────┤         ├──────────────────────┤
│ package progs        │ ──────→ │ progs.dat             │
│ var x float32        │ ──────→ │ global def + slot     │
│ const C = 5.0        │ ──────→ │ (inlined, no storage) │
│ func F()             │ ──────→ │ dfunction_t           │
│ func (t *T) M()      │ ──────→ │ T_M dfunction_t       │
│ //qgo:builtin N      │ ──────→ │ first_statement = -N  │
│ //qgo:entity struct  │ ──────→ │ field defs            │
│ struct tag `qgo:"x"` │ ──────→ │ field name "x"        │
│ if/else              │ ──────→ │ IFNOT + GOTO          │
│ for                  │ ──────→ │ IFNOT + GOTO loop     │
│ switch               │ ──────→ │ cascading EQ + IF     │
│ a + b (float)        │ ──────→ │ OP_ADD_F              │
│ a + b (Vec3)         │ ──────→ │ OP_ADD_V              │
│ a * b (Vec3·Vec3)    │ ──────→ │ OP_MUL_V (dot)        │
│ s1 + s2 (string)     │ ──────→ │ strcat builtin call   │
│ ent.Field            │ ──────→ │ OP_LOAD_*             │
│ ent.Field = val      │ ──────→ │ OP_ADDRESS+OP_STOREP_*│
│ return val           │ ──────→ │ OP_RETURN             │
│ return a, b          │ ──────→ │ OFS_RETURN + __retval  │
│ interface method     │ ──────→ │ LOAD_FNC + indirect CALL│
│ [N]T array           │ ──────→ │ N consecutive globals  │
│ init()               │ ──────→ │ startup call chain     │
│ quake.Vec3{x,y,z}    │ ──────→ │ 3-slot vector literal  │
│ nil (entity)         │ ──────→ │ 0 (world)             │
│ nil (func)           │ ──────→ │ 0 (null function)     │
└─────────────────────┘         └──────────────────────┘
```
