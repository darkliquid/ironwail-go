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

	var split []*Brush
	for _, b := range cells {
		split = append(split, d.splitDifferentTextures(b)...)
	}
	cells = split
	if opts.MergeConvex {
		cells = mergeConvex(cells)
	}
	canonicalizeBrush(cells)

	// Seams are the loci of hidden original-brush boundaries on merged
	// coplanar faces, derived from original-brush plane truth; they are
	// computed on the final cells so SideBrush indices match CellLabel.Cell.
	seams := SeamTruth(d, cells, origPlanes)

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
		if hits == 0 {
			// Intact geometry (synthetic maps, clean CSG) never leaves a
			// cell centroid outside every original brush; only float drift
			// at shared boundaries can. Recover deterministically by snapping
			// to the nearest brush within one grid cell instead of emitting
			// an unlabeled ("none") cell. Cracked real-world maps still fall
			// through to "none" when no brush is within reach.
			if nb := nearestBrush(cen, origPlanes, float64(opts.GridSnap)); nb >= 0 {
				hit, hits = nb, 1
			}
		}
		labels[ci] = CellLabel{Cell: ci, OriginalBrush: hit, Confidence: classifyHits(hits)}
	}

	return labels, seams, nil
}

// LabelCoverage returns the count of labeled cells (assigned or multi) out
// of the total; synthetic pairs must reach total == labeled.
func LabelCoverage(cells []CellLabel) (labeled, total int) {
	total = len(cells)
	for _, c := range cells {
		if c.Confidence == "assigned" || c.Confidence == "multi" {
			labeled++
		}
	}
	return labeled, total
}

// nearestBrush returns the index of the brush whose halfspace intersection
// is closest to p (minimal squared outward penetration), provided it is
// within maxD units; -1 when every brush is farther.
func nearestBrush(p mapfile.Vec3, planeSets [][]mapfile.Plane, maxD float64) int {
	best, bestD := -1, maxD
	for bi, ps := range planeSets {
		d := 0.0
		for _, q := range ps {
			if v := v3Dot(p, q.Normal) - q.Dist; v > 0 {
				d += v * v
			}
		}
		if d < bestD*bestD {
			best, bestD = bi, math.Sqrt(d)
		}
	}
	return best
}

// SeamTruth derives Route A supervision from the original brush planes
// (spec 9.4 step 2): on each pre-split cell side, an edge of one coplanar
// original brush face lying inside another's face is a hidden brush seam —
// the locus where a merged coplanar face hides an original brush join. The
// earlier texture-transient proxy found nothing because the CSG erases
// coincident coplanar faces (every corpus labels.json carries seams:null
// today). M2 Route A consumes these as classification targets.
func SeamTruth(d *decompiler, cells []*Brush, planeSets [][]mapfile.Plane) []SeamLabel {
	var seams []SeamLabel
	for ci, c := range cells {
		for _, s := range c.Sides {
			if s.Winding == nil || len(s.Winding.Points) < 3 {
				continue
			}
			// original brush faces coplanar with this side, clipped to the
			// cell: they partition (or overlap on) the merged face
			var polys []*Winding
			for _, ps := range planeSets {
				if !planeSetHas(ps, s.Plane) {
					continue
				}
				w := clipToBrush(faceOf(ps, s.Plane), c, s.Plane)
				if w == nil || len(w.Points) < 3 {
					continue
				}
				polys = append(polys, w)
			}
			if len(polys) < 2 {
				continue // single-face side: nothing hidden
			}
			// an edge of one original face lying inside another's is the
			// shared join the merged face hides
			for i := 0; i < len(polys); i++ {
				for j := i + 1; j < len(polys); j++ {
					for e := range polys[i].Points {
						p0 := polys[i].Points[e]
						p1 := polys[i].Points[(e+1)%len(polys[i].Points)]
						if seg := segInside(p0, p1, polys[j], s.Plane); seg != nil {
							seams = append(seams, SeamLabel{SideBrush: ci, Edge: *seg, Seam: true})
						}
					}
				}
			}
		}
	}
	return seams
}

// planeSetHas reports whether the original brush's planes include p (same
// direction, matching the decompiled side's outward orientation).
func planeSetHas(ps []mapfile.Plane, p mapfile.Plane) bool {
	for _, q := range ps {
		if planesMatch(q, p) {
			return true
		}
	}
	return false
}

// faceOf returns an original brush's face polygon on plane p: the base
// winding clipped by the brush's other halfspaces.
func faceOf(ps []mapfile.Plane, p mapfile.Plane) *Winding {
	w := BaseWinding(p)
	for _, o := range ps {
		if planesMatch(o, p) || planesOpposite(o, p) {
			continue
		}
		w = w.Clip(negatePlane(o))
		if w == nil {
			return nil
		}
	}
	return w
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
