// Package qc implements the QuakeC virtual machine, loader, execution engine,
// and builtin-function bridge.
//
// # Purpose
//
// The package loads progs.dat, represents globals and entity fields, executes
// bytecode statements, and exposes engine services to QuakeC code. It supports
// both server-side QuakeC (SSQC) and client-side QuakeC (CSQC).
//
// # High-level design
//
// The VM is modeled explicitly with typed globals, entity-field offsets,
// string tables, function tables, and registered builtin hooks. The server
// drives it synchronously by setting parameters, invoking entry points, and
// reading results back through shared VM state.
//
// CSQC uses a separate VM instance (CSQC type) with its own function table,
// global variables, and precache registries. Drawing and client-state builtins
// are decoupled from the renderer via CSQCDrawHooks and CSQCClientHooks
// function-pointer structs, avoiding circular imports.
//
// # Role in the engine
//
// This is the gameplay scripting core that sits between static game data and
// server-side simulation (SSQC) or client-side HUD rendering (CSQC).
//
// # Key types
//
//   - VM: Core virtual machine with loader, executor, and builtin dispatch
//   - CSQC: Client-side VM wrapper with entry points (Init, Shutdown,
//     DrawHud, DrawScores), per-frame global sync, and precache registries
//   - CSQCDrawHooks: Drawing operations (drawpic, drawfill, drawstring, etc.)
//   - CSQCClientHooks: Client state queries (getstati, getplayerkeyvalue, etc.)
//
// # Original C lineage
//
// The direct original counterparts are pr_exec.c, pr_edict.c, pr_cmds.c, and
// declarations from pr_comp.h. CSQC support mirrors C Ironwail's cl.qcvm
// pattern from host.c and sbar.c.
//
// # Deviations and improvements
//
// The Go port replaces pointer-cast-heavy field access with typed helpers and
// explicit hook interfaces, and it makes server/VM integration more modular
// than the original global-state design. Strongly named structs and indexed
// helpers preserve QuakeC semantics while making the subsystem easier to read
// and test.
package qc
