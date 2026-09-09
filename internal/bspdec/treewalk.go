package bspdec

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// bboxGrow is how far the per-model seed box extends past the dmodel bounds.
//
// Where in C: Q1_CreateBrushesFromBSP in bspc map_q1.c (bbox + 8).
const bboxGrow = 8

type decompiler struct {
	tree        *bsp.Tree
	opts        Options
	warnings    int
	leavesSolid int
	texNames    []string // miptex index -> name (Task 6)
	faces       []texturedFace
}

func newDecompiler(tree *bsp.Tree, opts Options) *decompiler {
	d := &decompiler{tree: tree, opts: opts}
	d.texNames = textureNames(tree)
	return d
}

func (d *decompiler) warnf(format string, args ...any) {
	d.warnings++
	slog.Warn("bspdec: " + fmt.Sprintf(format, args...))
}

// plane converts a lump plane to float64 math.
func (d *decompiler) plane(i int32) mapfile.Plane {
	p := d.tree.Planes[i]
	return mapfile.Plane{
		Normal: mapfile.Vec3{X: float64(p.Normal.X), Y: float64(p.Normal.Y), Z: float64(p.Normal.Z)},
		Dist:   float64(p.Dist),
	}
}

// boxBrush builds an axial box brush from mins/maxs.
func boxBrush(mins, maxs mapfile.Vec3) *Brush {
	b := &Brush{Contents: bsp.ContentsSolid}
	planes := []mapfile.Plane{
		{Normal: vc(1, 0, 0), Dist: maxs.X},
		{Normal: vc(-1, 0, 0), Dist: -mins.X},
		{Normal: vc(0, 1, 0), Dist: maxs.Y},
		{Normal: vc(0, -1, 0), Dist: -mins.Y},
		{Normal: vc(0, 0, 1), Dist: maxs.Z},
		{Normal: vc(0, 0, -1), Dist: -mins.Z},
	}
	for _, p := range planes {
		b.Sides = append(b.Sides, &Side{Plane: p})
	}
	rebuildWindings(b)
	return b
}

// rebuildWindings recomputes every side's winding from the halfspace set:
// the side's base winding clipped by all other planes.
//
// Where in C: BuildInitialBrush in ericw-tools common/decompile.cc.
func rebuildWindings(b *Brush) {
	for i, s := range b.Sides {
		w := BaseWinding(s.Plane)
		for j, o := range b.Sides {
			if i == j {
				continue
			}
			w = w.Clip(negatePlane(o.Plane))
			if w == nil {
				break
			}
		}
		s.Winding = w
	}
}

// clipBrush keeps the part of b behind p (dot(n,x) <= dist) and adds p as a
// new side when it actually bounds the result. Returns nil when b is entirely
// in front of p. When b does not touch p the geometry is returned unchanged
// (no new side).
func clipBrush(b *Brush, p mapfile.Plane) *Brush {
	out := &Brush{Contents: b.Contents}
	for _, s := range b.Sides {
		w := s.Winding.Clip(negatePlane(p))
		if w == nil {
			continue // this side does not bound the kept part
		}
		out.Sides = append(out.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs, Winding: w})
	}
	if len(out.Sides) == 0 {
		return nil // entirely in front of p
	}
	nw := BaseWinding(p)
	for _, s := range out.Sides {
		nw = nw.Clip(negatePlane(s.Plane))
		if nw == nil {
			break
		}
	}
	if nw != nil {
		out.Sides = append(out.Sides, &Side{Plane: p, Winding: nw})
	}
	if len(out.Sides) < 4 {
		return nil // degenerate fragment, not a polyhedron
	}
	return out
}

// splitBrush splits b by p into the front and back parts (either nil).
//
// Where in C: brush splitting under Q1_CreateBrushes_r in bspc map_q1.c.
func splitBrush(b *Brush, p mapfile.Plane) (front, back *Brush) {
	return clipBrush(b, negatePlane(p)), clipBrush(b, p)
}

// decompileModel walks one model's render tree and returns one brush per
// non-empty leaf cell.
//
// Where in C: Q1_CreateBrushesFromBSP + Q1_CreateBrushes_r in bspc map_q1.c.
func (d *decompiler) decompileModel(modelIdx int) ([]*Brush, error) {
	if modelIdx < 0 || modelIdx >= len(d.tree.Models) {
		return nil, fmt.Errorf("model %d out of range (%d models)", modelIdx, len(d.tree.Models))
	}
	m := d.tree.Models[modelIdx]
	mins := mapfile.Vec3{
		X: float64(m.BoundsMin.X) - bboxGrow,
		Y: float64(m.BoundsMin.Y) - bboxGrow,
		Z: float64(m.BoundsMin.Z) - bboxGrow,
	}
	maxs := mapfile.Vec3{
		X: float64(m.BoundsMax.X) + bboxGrow,
		Y: float64(m.BoundsMax.Y) + bboxGrow,
		Z: float64(m.BoundsMax.Z) + bboxGrow,
	}
	head := m.HeadNode[0]
	if head < 0 || int(head) >= len(d.tree.Nodes) {
		return nil, fmt.Errorf("model %d: bad headnode %d", modelIdx, head)
	}
	var out []*Brush
	d.walk(int(head), boxBrush(mins, maxs), &out)
	return out, nil
}

// walk recurses the node tree. children[0] is the plane front, children[1]
// the back (WinQuake bspfile.h dnode_t; confirmed by internal/qbsp
// hull_test.go's trace convention). Every non-empty leaf termination emits a
// brush: all solid areas share leaf 0, so the path defines the cell, not the
// leaf index.
func (d *decompiler) walk(nodeIdx int, b *Brush, out *[]*Brush) {
	node := d.tree.Nodes[nodeIdx]
	front, back := splitBrush(b, d.plane(node.PlaneNum))
	parts := [2]*Brush{front, back}
	for side := 0; side < 2; side++ {
		part := parts[side]
		if part == nil {
			continue
		}
		child := node.Children[side]
		if child.IsLeaf {
			leaf := d.tree.Leafs[child.Index]
			if leaf.Contents == bsp.ContentsEmpty {
				continue
			}
			if leaf.Contents == bsp.ContentsSolid {
				d.leavesSolid++
			}
			part.Contents = leaf.Contents
			*out = append(*out, part)
			continue
		}
		d.walk(child.Index, part, out)
	}
}