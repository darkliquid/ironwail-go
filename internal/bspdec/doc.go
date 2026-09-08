// Package bspdec decompiles Quake BSP29 files into editable .map files.
//
// # Original C lineage
//
// The deterministic core ports the bspc/ericw-tools decompiler family:
// id Q3A bspc/map_q1.c (Q1_CreateBrushesFromBSP, Q1_CreateBrushes_r,
// Q1_FixContentsTextures), ericw-tools common/decompile.cc
// (RemoveRedundantPlanes, BuildInitialBrush,
// SplitDifferentTexturedPartsOfBrush) and common/polylib.cc
// (BaseWindingForPlane, ClipWindingEpsilon).
package bspdec