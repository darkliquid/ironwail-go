package bspdec

import (
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// Side is one convex brush face: its plane, surviving polygon, and texture.
type Side struct {
	Plane   mapfile.Plane
	Winding *Winding
	TexName string
	// Vecs is the s/t texture mapping copied from BSP texinfo, emitted as
	// Valve 220 axes by the map writer.
	Vecs [2][4]float64
}

// Brush is a convex cell: a set of halfspace sides with windings.
type Brush struct {
	Sides    []*Side
	Contents int32 // bsp.Contents* of the leaf the cell came from
}

// Options controls a Decompile run. ML flags are CLI-rejected in M0.
type Options struct {
	NoBrushlist     bool   // reserved for M1; M0 has no BRUSHLIST path
	DecompileHull   int    // 0 = render hull; 1-3 = collision hull (replaces hull 0 output)
	MergeConvex     bool   // merge same-contents coplanar-adjacent convex cells
	GridSnap        int    // quantize emitted plane points to this lattice (0 = off)
	TextureFallback string // "skip" | "nearest" | "trigger"
}

// ModelStats is the per-model summary reported by --json.
type ModelStats struct {
	Model       int
	Brushes     int
	LeavesSolid int
	PlanesUsed  int
	Warnings    int
}