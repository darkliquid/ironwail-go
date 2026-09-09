package bspdec

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// CellLabel assigns a decompiled world cell to the original worldspawn brush
// it is the remnant of (spec section 9.4, step 1).
type CellLabel struct {
	Cell          int    // index into the world model's decompiled cells
	OriginalBrush int    // worldspawn brush index, or -1
	Confidence    string // "assigned" | "multi" | "none"
}

// SeamLabel marks an edge of a final (split/merged) brush side that an
// original-brush plane crosses: the locus where a merged coplanar face hides
// an original brush seam (spec section 9.4, step 2).
type SeamLabel struct {
	SideBrush int // index into the world cells
	Edge      [2]mapfile.Vec3
	Seam      bool
}

// LabelCells runs the world-model pipeline (walk, prune, texture, split,
// merge, canonicalize) and derives supervision labels against the original
// .map's worldspawn brushes.
func LabelCells(tree *bsp.Tree, orig *mapfile.Map, opts Options) ([]CellLabel, []SeamLabel, error) {
	d := newDecompiler(tree, opts)
	cells, err := d.decompileModel(0)
	if err != nil {
		return nil, nil, err
	}
	for _, b := range cells {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(cells)

	originalBrushes := originalWorldBrushes(orig)
	origPlanes := brushPlaneSets(originalBrushes)

	// Seams are the loci of hidden original-brush boundaries on merged
	// coplanar faces, which only exist before the texture-boundary split
	// breaks the face apart.
	seams := SeamEdges(d, cells, origPlanes)

	var split []*Brush
	for _, b := range cells {
		split = append(split, d.splitDifferentTextures(b)...)
	}
	cells = split
	if opts.MergeConvex {
		cells = mergeConvex(cells)
	}
	canonicalizeBrush(cells)

	labels := make([]CellLabel, len(cells))
	for ci, c := range cells {
		cen := cellCentroid(c)
		hits := 0
		hit := -1
		for bi, ps := range origPlanes {
			if pointInPlanes(cen, ps) {
				hits++
				hit = bi
			}
		}
		labels[ci] = CellLabel{Cell: ci, OriginalBrush: hit, Confidence: classifyHits(hits)}
	}

	return labels, seams, nil
}

// SeamEdges finds the loci where hidden original-brush boundaries cross a
// merged coplanar face: on each side, every pair of matched faces with
// different texinfo abuts along a boundary segment (the merged-face seam).
// The shared edge line of one face clipped inside the other is the seam.
// M2 Route A consumes these as classification targets.
func SeamEdges(d *decompiler, cells []*Brush, planeSets [][]mapfile.Plane) []SeamLabel {
	faces := d.collectFaces()
	_ = planeSets // reserved: M2 re-scores by original-brush membership
	var seams []SeamLabel
	for ci, c := range cells {
		for _, s := range c.Sides {
			var matches []texturedFace
			for _, f := range faces {
				if !planesMatch(s.Plane, f.plane) {
					continue
				}
				w := clipToBrush(f.winding, c, s.Plane)
				if w == nil || len(w.Points) < 3 {
					continue
				}
				matches = append(matches, texturedFace{plane: f.plane, winding: w, texName: f.texName})
			}
			for i := range matches {
				for j := i + 1; j < len(matches); j++ {
					if matches[i].texName == matches[j].texName {
						continue
					}
					a, b := matches[i].winding, matches[j].winding
					for e := range a.Points {
						p0 := a.Points[e]
						p1 := a.Points[(e+1)%len(a.Points)]
						if seg := segInside(p0, p1, b, s.Plane); seg != nil {
							seams = append(seams, SeamLabel{SideBrush: ci, Edge: *seg, Seam: true})
						}
					}
				}
			}
		}
	}
	return seams
}

// segInside clips segment p0-p1 to the inside of convex winding w (lying on
// plane ref) and returns the surviving sub-segment, or nil.
func segInside(p0, p1 mapfile.Vec3, w *Winding, ref mapfile.Plane) *[2]mapfile.Vec3 {
	t0, t1 := 0.0, 1.0
	dir := v3Sub(p1, p0)
	n := len(w.Points)
	cent := w.Centroid()
	for i := range w.Points {
		a := w.Points[i]
		b := w.Points[(i+1)%n]
		ep := inwardEdgePlane(a, b, ref, cent)
		da := v3Dot(p0, ep.Normal) - ep.Dist
		db := v3Dot(p1, ep.Normal) - ep.Dist
		if da <= 1e-9 && db <= 1e-9 {
			continue // fully inside this edge halfspace
		}
		if da > 1e-9 && db > 1e-9 {
			return nil // fully outside
		}
		if da > 1e-9 {
			t0 = math.Max(t0, da/(da-db))
		} else {
			t1 = math.Min(t1, da/(da-db))
		}
		if t0 > t1 {
			return nil
		}
	}
	if t1-t0 < 1e-4 {
		return nil
	}
	return &[2]mapfile.Vec3{v3Add(p0, v3Scale(dir, t0)), v3Add(p0, v3Scale(dir, t1))}
}

// originalWorldBrushes returns the worldspawn brush list (entity 0).
func originalWorldBrushes(orig *mapfile.Map) []mapfile.MapBrush {
	if len(orig.Entities) == 0 {
		return nil
	}
	return orig.Entities[0].Brushes
}

// brushPlaneSets derives oriented plane sets (outward normals per the ericw
// convention used by all well-formed mapper-authored maps) for each brush.
func brushPlaneSets(brushes []mapfile.MapBrush) [][]mapfile.Plane {
	sets := make([][]mapfile.Plane, 0, len(brushes))
	for i := range brushes {
		var ps []mapfile.Plane
		for _, f := range brushes[i].Faces {
			if p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2]); length > 0.01 {
				ps = append(ps, p)
			}
		}
		sets = append(sets, ps)
	}
	return sets
}

// pointInPlanes reports whether p is on/behind every plane (inside the
// halfspace intersection). Epsilon covers float drift.
func pointInPlanes(p mapfile.Vec3, planes []mapfile.Plane) bool {
	for _, q := range planes {
		if v3Dot(p, q.Normal)-q.Dist > onEpsilon {
			return false
		}
	}
	return true
}

// cellCentroid averages every side-winding point of the cell.
func cellCentroid(c *Brush) mapfile.Vec3 {
	var acc mapfile.Vec3
	n := 0
	for _, s := range c.Sides {
		if s.Winding == nil {
			continue
		}
		for _, p := range s.Winding.Points {
			acc = v3Add(acc, p)
			n++
		}
	}
	if n == 0 {
		return mapfile.Vec3{}
	}
	return v3Scale(acc, 1/float64(n))
}

func classifyHits(hits int) string {
	switch {
	case hits == 1:
		return "assigned"
	case hits > 1:
		return "multi"
	default:
		return "none"
	}
}
