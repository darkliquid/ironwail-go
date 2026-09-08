package bspdec

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// hullMins/hullMaxs are the canonical Quake collision hull AABBs.
//
// Where in C: hull_min/hull_max in sv_main.c (hull 3 unused in Quake).
var hullMins = [4]mapfile.Vec3{{X: 0, Y: 0, Z: 0}, {X: -16, Y: -16, Z: -24}, {X: -32, Y: -32, Z: -24}, {}}
var hullMaxs = [4]mapfile.Vec3{{X: 0, Y: 0, Z: 0}, {X: 16, Y: 16, Z: 32}, {X: 32, Y: 32, Z: 64}, {}}

// clipnode is the normalized collision-node record (children < 0 = contents).
type clipnode struct {
	PlaneNum int32
	Children [2]int32
}

// clipnodesOf normalizes the version-dependent clipnode lump.
func clipnodesOf(f *bsp.File) ([]clipnode, error) {
	switch cn := f.Clipnodes.(type) {
	case []bsp.DSClipNode:
		out := make([]clipnode, len(cn))
		for i, c := range cn {
			out[i] = clipnode{PlaneNum: c.PlaneNum, Children: c.Children}
		}
		return out, nil
	case []bsp.DLClipNode:
		out := make([]clipnode, len(cn))
		for i, c := range cn {
			out[i] = clipnode{PlaneNum: c.PlaneNum, Children: c.Children}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported clipnode lump type %T", f.Clipnodes)
	}
}

// unexpandPlane reverses hull expansion: qbsp offsets each plane by the hull
// AABB corner most extreme along the normal (Minkowski sum), so decompile
// subtracts exactly that.
//
// Where in C: hull expansion in ericw qbsp hull.c (this is its inverse).
func unexpandPlane(p mapfile.Plane, mins, maxs mapfile.Vec3) mapfile.Plane {
	var off mapfile.Vec3
	if p.Normal.X >= 0 {
		off.X = maxs.X
	} else {
		off.X = mins.X
	}
	if p.Normal.Y >= 0 {
		off.Y = maxs.Y
	} else {
		off.Y = mins.Y
	}
	if p.Normal.Z >= 0 {
		off.Z = maxs.Z
	} else {
		off.Z = mins.Z
	}
	p.Dist -= v3Dot(p.Normal, off)
	return p
}

// decompileHull walks the clipnode tree of the given hull and emits clip
// brushes with expansion reversed and bevels dropped.
//
// Where in C: hull decompile in bspc map_q1.c / BSP Forge (un-expand by the
// hull AABB, drop bevel-only planes).
func (d *decompiler) decompileHull(modelIdx, hull int, nodes []clipnode) []*Brush {
	m := d.tree.Models[modelIdx]
	head := m.HeadNode[hull]
	if head < 0 || int(head) >= len(nodes) {
		return nil // model has no collision hull
	}
	// Walk from the grown box so every cell is closed, then un-expand and
	// cap each brush to the model's render bounds. The cap removes the
	// outside-fill and seed-box padding that Quake hulls treat as solid
	// (children with negative index = contents), leaving the wall slabs.
	mins := mapfile.Vec3{X: float64(m.BoundsMin.X) - bboxGrow, Y: float64(m.BoundsMin.Y) - bboxGrow, Z: float64(m.BoundsMin.Z) - bboxGrow}
	maxs := mapfile.Vec3{X: float64(m.BoundsMax.X) + bboxGrow, Y: float64(m.BoundsMax.Y) + bboxGrow, Z: float64(m.BoundsMax.Z) + bboxGrow}
	var out []*Brush
	d.walkClipnodes(int(head), boxBrush(mins, maxs), nodes, &out)
	box := boxBrush(mapfile.Vec3{X: float64(m.BoundsMin.X), Y: float64(m.BoundsMin.Y), Z: float64(m.BoundsMin.Z)},
		mapfile.Vec3{X: float64(m.BoundsMax.X), Y: float64(m.BoundsMax.Y), Z: float64(m.BoundsMax.Z)})
	alive := out[:0]
	for _, b := range out {
		b.Contents = bsp.ContentsClip
		for _, s := range b.Sides {
			s.Plane = unexpandPlane(s.Plane, hullMins[hull], hullMaxs[hull])
			s.TexName = "clip"
		}
		// cap to the render bounds (each bound plane added as a real side)
		capped := b
		ok := true
		for _, bs := range box.Sides {
			capped = clipBrush(capped, bs.Plane)
			if capped == nil {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		removeRedundantPlanes(capped)
		if len(capped.Sides) < 4 {
			continue
		}
		for _, s := range capped.Sides {
			s.TexName = "clip"
		}
		alive = append(alive, capped)
	}
	return alive
}

// walkClipnodes mirrors walk, but children index clipnodes and negative
// children are contents values.
func (d *decompiler) walkClipnodes(idx int, b *Brush, nodes []clipnode, out *[]*Brush) {
	n := nodes[idx]
	front, back := splitBrush(b, d.plane(n.PlaneNum))
	parts := [2]*Brush{front, back}
	for side := 0; side < 2; side++ {
		part := parts[side]
		if part == nil {
			continue
		}
		child := n.Children[side]
		if child < 0 {
			if child == bsp.ContentsSolid {
				part.Contents = bsp.ContentsSolid
				*out = append(*out, part)
			}
			continue
		}
		d.walkClipnodes(int(child), part, nodes, out)
	}
}