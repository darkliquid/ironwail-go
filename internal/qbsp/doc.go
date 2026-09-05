// Package qbsp is a pure-Go Quake map compiler: it parses .map files
// (QuakeEd and Valve 220 brush formats), constructs a BSP tree with
// collision clipnode hulls, detects leaks, and writes BSP29/BSP2 files plus
// .prt portal files (bead ironwail-go-t63).
//
// # Original C lineage
//
// The behaviour follows ericw-tools 2.0.0-alpha11 (qbsp/qbsp.cc,
// qbsp/brushbsp.cc, qbsp/outside.cc, qbsp/writebsp.cc, common/mapfile.cc,
// common/mapfile.hh, common/bspfile_q1.hh), which itself derives from
// id Software's original Quake map compiler (Quake tools/qbsp). Byte
// formats are cross-validated against the engine's own reader in
// internal/bsp. ericw-tools is GPL-licensed; this package is a clean-room
// port of the algorithms and file formats, not a translation of its code.
package qbsp