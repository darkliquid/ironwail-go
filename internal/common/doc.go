// Package common provides low-level utilities shared across the engine,
// especially binary buffers and parsing helpers.
//
// # Purpose
//
// The package supplies foundational pieces such as SizeBuf, endian-aware
// reads and writes, path helpers, and small data-structure utilities that many
// other subsystems rely on.
//
// # High-level design
//
// SizeBuf is the core abstraction: it supports Quake-style message
// serialization, overflow tracking, and in-place parsing of incoming protocol
// data. The rest of the package provides the little-endian and utility helpers
// needed by networking, file decoding, and related low-level code.
//
// # Role in the engine
//
// This package underpins networking, file decoding, entity/message marshaling,
// and other shared code used by client and server subsystems.
//
// # Original C lineage
//
// The main reference is common.c/common.h, especially the message I/O and
// utility sections.
//
// # Deviations and improvements
//
// Unlike the original monolithic common layer, the Go port splits filesystem,
// commands, console, and cvars into dedicated packages and keeps this package
// focused on reusable primitives. Typed methods, standard-library endian
// helpers, and slice-based buffers replace macros and raw pointer arithmetic.
package common
