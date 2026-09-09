package bspdec

import (
	"bytes"
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// minSliverArea is the smallest side a brush may keep: grid snap (default 8)
// makes sub-unit slivers unemittable.
const minSliverArea = 0.5

// sideSurvivesGrid reports whether the side can be emitted: its winding must
// stay non-degenerate after grid snapping (sub-grid geometry like 4-unit
// plates collapses to zero thickness and the .map parser drops the face).
func sideSurvivesGrid(s *Side, grid int) bool {
	if s.Winding == nil || len(s.Winding.Points) < 3 {
		return false
	}
	if grid <= 1 {
		return s.Winding.Area() >= minSliverArea
	}
	step := float64(grid)
	snap := func(v float64) float64 { return math.Round(v/step) * step }
	distinct := make([]mapfile.Vec3, 0, len(s.Winding.Points))
	seen := map[mapfile.Vec3]bool{}
	for _, p := range s.Winding.Points {
		q := mapfile.Vec3{X: snap(p.X), Y: snap(p.Y), Z: snap(p.Z)}
		if !seen[q] {
			seen[q] = true
			distinct = append(distinct, q)
		}
	}
	if len(distinct) < 3 {
		return false
	}
	best := 0.0
	for i := 0; i < len(distinct); i++ {
		for j := i + 1; j < len(distinct); j++ {
			for k := j + 1; k < len(distinct); k++ {
				if a := v3Len(v3Cross(v3Sub(distinct[j], distinct[i]), v3Sub(distinct[k], distinct[i]))); a > best {
					best = a
				}
			}
		}
	}
	return best/2 >= minSliverArea
}

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
	// M1 BRUSHLIST shortcut (spec section 5 step 9): when the appended
	// BRUSHLIST lump is present and not explicitly disabled, emit the
	// original brushes directly instead of the leaf-derived treewalk. The
	// treewalk stays the fallback for lumps that are absent or unparsable,
	// and hull decompile keeps its dedicated path.
	if !opts.NoBrushlist && opts.DecompileHull == 0 {
		if ls, err := BrushListFromBSP(data); err == nil && len(ls) > 0 {
			return decompileFromBrushList(ents, tree, ls, opts)
		}
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
			// drop degenerate fragments (sides lost to pruning) so output
			// brushes are always closed polyhedra. removeRedundantPlanes
			// clears nil-winding sides first so the count is face-accurate.
			alive := brushes[:0]
			for _, b := range brushes {
				dedupeCoplanarSides(b)
				removeRedundantPlanes(b)
				kept := b.Sides[:0]
				for _, s := range b.Sides {
					if sideSurvivesGrid(s, opts.GridSnap) {
						kept = append(kept, s)
					}
				}
				b.Sides = kept
				if len(b.Sides) >= 4 {
					alive = append(alive, b)
				} else {
					d.warnf("dropping degenerate brush with %d sides", len(b.Sides))
				}
			}
			brushes = alive
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