// Package qc implements the QuakeC virtual machine, loader, execution engine,
// and builtin-function bridge.
//
// # Purpose
//
// The package loads progs.dat, represents globals and entity fields, executes
// bytecode statements, and exposes engine services to QuakeC code.
//
// # High-level design
//
// The VM is modeled explicitly with typed globals, entity-field offsets,
// string tables, function tables, and registered builtin hooks. The server
// drives it synchronously by setting parameters, invoking entry points, and
// reading results back through shared VM state.
//
// # Role in the engine
//
// This is the gameplay scripting core that sits between static game data and
// server-side simulation.
//
// # Original C lineage
//
// The direct original counterparts are pr_exec.c, pr_edict.c, pr_cmds.c, and
// declarations from pr_comp.h.
//
// # Deviations and improvements
//
// The Go port replaces pointer-cast-heavy field access with typed helpers and
// explicit hook interfaces, and it makes server/VM integration more modular
// than the original global-state design. Strongly named structs and indexed
// helpers preserve QuakeC semantics while making the subsystem easier to read
// and test.
package qc