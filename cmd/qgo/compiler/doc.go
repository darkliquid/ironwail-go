// Package compiler implements AST parsing, symbol resolution, type checking, and bytecode emission for QuakeGo.
//
// # Purpose
//
// Parse Go AST definitions (`go/parser`), resolve QuakeC entity struct definitions,
// generate function statements, and emit QCVM binary `progs.dat` structures.
//
// # Original C lineage
//
// Mirrored from original Quake tool sources:
//   - qcc.c: Lexer, parser, symbol table, expression trees, statement codegen.
//
// # Key types
//
//   - Compiler: Top-level compiler coordinator holding symbol tables and statement lists.
//   - Statement: QCVM bytecode instruction representation (op, a, b, c).
//   - Def: Global, local, or field definition in QCVM memory.
//
// # Testing
//
//	CGO_ENABLED=0 go test ./cmd/qgo/compiler -count=1
package compiler
