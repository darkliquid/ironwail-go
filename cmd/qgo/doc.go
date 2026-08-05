// Package main provides the command-line driver for the QuakeGo (QGo) compiler.
//
// # Purpose
//
// Compile QuakeGo Go-dialect gameplay source code (`pkg/qgo/quakego`) into QuakeC VM
// bytecode (`progs.dat`) consumed by the server QuakeC execution engine.
//
// # Original C lineage
//
// Mirrored from original Quake tool sources:
//   - qcc.c: Original QuakeC compiler toolchain.
//
// # Role in the engine
//
// The qgo tool translates Go-syntax QuakeC source files into the binary `progs.dat`
// format expected by `internal/qc`.
//
// # Usage
//
//	mise run build-qgo
//	mise run build-progs
package main
