package bspdec

import (
	"bytes"
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// Decompile runs the full pipeline over BSP file bytes and returns the map
// plus per-model stats. Pipeline per model (spec section 4): treewalk (or
// hull walk) -> redundant-plane removal -> texturing -> texture-boundary
// splits -> convex merge -> entity attach.
func Decompile(data []byte, opts Options) (*mapfile.Map, []ModelStats, error) {
	if opts.TextureFallback == "" {
		opts.TextureFallback = "nearest"
	}
	if opts.DecompileHull < 0 || opts.DecompileHull > 3 {
		return nil, nil, fmt.Errorf("invalid hull %d (want 0-3)", opts.DecompileHull)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("loading BSP: %w", err)
	}
	d := newDecompiler(tree, opts)
	ents, err := parseEntities(tree)
	if err != nil {
		return nil, nil, err
	}
	perModel := make([][]*Brush, len(tree.Models))
	stats := make([]ModelStats, 0, len(tree.Models))
	if opts.DecompileHull > 0 {
		f, err := bsp.Load(bytes.NewReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("loading BSP for clipnodes: %w", err)
		}
		nodes, err := clipnodesOf(f)
		if err != nil {
			return nil, nil, err
		}
		for mi := range tree.Models {
			before := d.warnings
			brushes := d.decompileHull(mi, opts.DecompileHull, nodes)
			canonicalizeBrush(brushes)
			perModel[mi] = brushes
			stats = append(stats, ModelStats{
				Model: mi, Brushes: len(brushes),
				PlanesUsed: planeSetCount(brushes), Warnings: d.warnings - before,
			})
		}
	} else {
		for mi := range tree.Models {
			beforeW, beforeL := d.warnings, d.leavesSolid
			brushes, err := d.decompileModel(mi)
			if err != nil {
				return nil, nil, err
			}
			for _, b := range brushes {
				removeRedundantPlanes(b)
			}
			d.textureBrushes(brushes)
			var split []*Brush
			for _, b := range brushes {
				split = append(split, d.splitDifferentTextures(b)...)
			}
			brushes = split
			if opts.MergeConvex {
				brushes = mergeConvex(brushes)
			}
			canonicalizeBrush(brushes)
			perModel[mi] = brushes
			stats = append(stats, ModelStats{
				Model: mi, Brushes: len(brushes),
				LeavesSolid: d.leavesSolid - beforeL,
				PlanesUsed:  planeSetCount(brushes), Warnings: d.warnings - beforeW,
			})
		}
	}
	return attachBrushes(ents, perModel, tree), stats, nil
}

// canonicalizeBrush orients every side winding against its outward plane.
func canonicalizeBrush(brushes []*Brush) {
	for _, b := range brushes {
		for _, s := range b.Sides {
			if s.Winding != nil {
				canonicalizeWinding(s.Winding, s.Plane)
			}
		}
	}
}

// SelfCheck validates every emitted brush: >=4 faces, non-degenerate planes,
// and convexity (every face's points on or behind every other face's plane).
// The CLI maps a failure to exit code 3.
func SelfCheck(m *mapfile.Map) error {
	for ei := range m.Entities {
		for bi := range m.Entities[ei].Brushes {
			if err := ValidateBrush(&m.Entities[ei].Brushes[bi]); err != nil {
				return fmt.Errorf("entity %d brush %d: %w", ei, bi, err)
			}
		}
	}
	return nil
}

// ValidateBrush checks one brush. The epsilon is generous because grid snap
// quantizes the emitted points before this runs.
func ValidateBrush(mb *mapfile.MapBrush) error {
	if len(mb.Faces) < 4 {
		return fmt.Errorf("only %d faces", len(mb.Faces))
	}
	planes := make([]mapfile.Plane, len(mb.Faces))
	for i, f := range mb.Faces {
		p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2])
		if length < 0.01 {
			return fmt.Errorf("face %d: degenerate plane points", i)
		}
		planes[i] = p
	}
	const eps = 0.5
	for i, f := range mb.Faces {
		for j, p := range planes {
			if i == j {
				continue
			}
			for _, pt := range f.Points {
				if v3Dot(pt, p.Normal)-p.Dist > eps {
					return fmt.Errorf("face %d point %v in front of face %d's plane (non-convex)", i, pt, j)
				}
			}
		}
	}
	return nil
}