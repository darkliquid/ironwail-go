// Package net implements Quake networking primitives, including loopback
// transport, datagram handling, sockets, and protocol helpers.
//
// Purpose
//
// The package lets client and server exchange reliable and unreliable messages
// using the classic Quake network model.
//
// High-level design
//
// Callers connect, listen, send, receive, and close sockets while driver-
// specific code handles loopback or UDP details. A built-in loopback transport
// supports local play and tests by moving framed messages through in-memory
// socket pairs.
//
// Role in the engine
//
// This package is the transport substrate under client parsing, server
// messaging, and host connection management.
//
// Original C lineage
//
// The corresponding sources are net_main.c, net_dgrm.c, net_loop.c, net_udp.c,
// net.h, and related platform files.
//
// Deviations and improvements
//
// The Go port focuses on the core transport semantics and leaves historical
// platform variants behind. Typed socket structs, normal time handling, and a
// clearer split between loopback and datagram code replace driver tables and
// platform conditionals while preserving Quake's protocol model.
package net