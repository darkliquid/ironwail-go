package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// bspSide is one face of a convex BSP brush: the plane (oriented so the
// brush interior is the BACK side, dot(n,x) <= d) plus its polygon.
type bspSide struct {
	planenum int     // plane-table index (for BSP face output)
	n        vec3    // oriented outward normal
	d        float64 // oriented plane distance
	w        winding // face polygon, oriented to n
	onnode   bool    // plane already used as a node splitter (ericw onnode)
}

// bspBrush is a convex polyhedron used by the solidbsp CSG (the classic
// qbsp representation: a brush is the intersection of its sides' halfspaces).
type bspBrush struct {
	sides   []bspSide
	content int32
	bounds  [2]vec3
	sortKey int64 // (entity index << 32) | line, for ChopBrushes ordering
}

// sidePlane returns the oriented plane of a side.
func (s *bspSide) sidePlane() plane { return plane{Normal: s.n, Dist: s.d} }

// negPlane flips an oriented plane.
func negPlane(p plane) plane {
	return plane{Normal: p.Normal.Neg(), Dist: -p.Dist}
}

// planeEqualOriented reports whether two oriented planes are the same
// geometric plane (allowing the same plane in either orientation).
func planeEqualOriented(a, b plane) bool {
	dpos := a.Dist - b.Dist
	if v3Dot(a.Normal, b.Normal) < 0 {
		dpos = a.Dist + b.Dist
	}
	return math.Abs(v3Dot(a.Normal, b.Normal)) > 1-1e-4 && math.Abs(dpos) < 0.01
}

// brushFace pairs an outward plane with its table index (planenum).
type brushFace struct {
	p  plane
	pn int
}

// buildBspBrushFaces builds a solidbsp brush from outward (oriented) faces
// with known plane-table indices, bounded by box. Following the reference
// (ericw CreateBrushWindings), EVERY side keeps its plane even when the
// winding fails: the halfspace still bounds the brush in the CSG, and only
// the face output is skipped. Brushes with fewer than 4 sides are dropped.
func buildBspBrushFaces(ar *windingArena, faces []brushFace, box [2]vec3) *bspBrush {
	return buildBspBrushFacesClamped(ar, faces, box, box)
}

// buildBspBrushFacesClamped builds a solidbsp brush like
// buildBspBrushFaces but clamps every surviving winding to the brush's OWN
// map AABB (clampBox). Open brushes (a missing face, e.g. an id1 floor with
// no bottom) leave the opposing faces unbounded in that direction; without
// the clamp the world-box seed extends their windings across the map, and
// the stray vertices misclassify far split planes, routing pieces to the
// wrong leaves (the e1m1 mid-east floor slice loss, bead ironwail-go-aeh).
func buildBspBrushFacesClamped(ar *windingArena, faces []brushFace, box, clampBox [2]vec3) *bspBrush {
	b := &bspBrush{content: bsp.ContentsSolid}
	for fi, o := range faces {
		side := bspSide{planenum: o.pn, n: o.p.Normal, d: o.p.Dist}
		w := windingFromBoxPlane(ar, o.p, box[0], box[1])
		if w != nil {
			ok := true
			for gi, g := range faces {
				if fi == gi {
					continue
				}
				// Interior of g: dot(g.n, x) <= g.d  <=>  front of -g.
				clipped, cok := clipWindingKeepBack(ar, w, g.p)
				if !cok {
					ok = false
					break
				}
				w = clipped
			}
			if ok && len(w) >= 3 {
				w = windingRemoveColinear(ar, w)
				if len(w) >= 3 {
					w = windingClamp(w, clampBox)
					side.w = windingOrientTo(ar, w, o.p.Normal)
				}
			}
		}
		b.sides = append(b.sides, side)
	}
	if len(b.sides) < 4 {
		return nil
	}
	b.bounds = brushBoundsOf(b)
	return b
}

// windingClamp clamps every vertex into the box (an open brush's windings
// are only clipped on the sides the map author provided).
func windingClamp(w winding, box [2]vec3) winding {
	for i := range w {
		if w[i].X < box[0].X {
			w[i].X = box[0].X
		}
		if w[i].X > box[1].X {
			w[i].X = box[1].X
		}
		if w[i].Y < box[0].Y {
			w[i].Y = box[0].Y
		}
		if w[i].Y > box[1].Y {
			w[i].Y = box[1].Y
		}
		if w[i].Z < box[0].Z {
			w[i].Z = box[0].Z
		}
		if w[i].Z > box[1].Z {
			w[i].Z = box[1].Z
		}
	}
	return w
}

// brushBoundsOf computes the AABB of a brush from its side windings.
func brushBoundsOf(b *bspBrush) [2]vec3 {
	var mins, maxs vec3
	first := true
	for _, s := range b.sides {
		m, x := windingBounds(s.w)
		if first {
			mins, maxs = m, x
			first = false
			continue
		}
		if m.X < mins.X {
			mins.X = m.X
		}
		if x.X > maxs.X {
			maxs.X = x.X
		}
		if m.Y < mins.Y {
			mins.Y = m.Y
		}
		if x.Y > maxs.Y {
			maxs.Y = x.Y
		}
		if m.Z < mins.Z {
			mins.Z = m.Z
		}
		if x.Z > maxs.Z {
			maxs.Z = x.Z
		}
	}
	if first {
		return [2]vec3{{}, {}}
	}
	return [2]vec3{mins, maxs}
}

// plane-side classification bits (classic qbsp PSIDE_*).
const (
	psideFront  = 1 << 0
	psideBack   = 1 << 1
	psideFacing = 1 << 2
)

// splitEpsilon mirrors ericw's PLANESIDE_EPSILON (brushbsp.c): vertices
// within this distance of the split plane count as on-plane, so razor-thin
// trims and boundary-coincident faces never trigger phantom straddles.
const splitEpsilon = 0.1

// classifyBrush returns the pside bits of the brush relative to plane p:
// FRONT/BACK bits for vertices on either side, FACING when a side is
// coplanar with p (the brush "touches" the plane).
// Fast reject: when the brush AABB lies entirely on one side of p, every
// winding vertex does too (windings are inside the bounds), so the scan is
// skipped — the per-node classification loops call this O(brushes) times
// and the bound test replaces thousands of vertex dot products (ericw
// TestBrushToPlanenum boxes the brush first the same way).
func classifyBrush(b *bspBrush, p plane) int {
	minD, maxD := math.Inf(1), math.Inf(-1)
	for i := 0; i < 8; i++ {
		pt := vec3{
			X: b.bounds[i&1].X,
			Y: b.bounds[(i>>1)&1].Y,
			Z: b.bounds[(i>>2)&1].Z,
		}
		d := v3Dot(p.Normal, pt) - p.Dist
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	if minD > splitEpsilon {
		return psideFront
	}
	if maxD < -splitEpsilon {
		return psideBack
	}
	bits := 0
	for _, s := range b.sides {
		for _, v := range s.w {
			d := v3Dot(p.Normal, v) - p.Dist
			if d > splitEpsilon {
				bits |= psideFront
			} else if d < -splitEpsilon {
				bits |= psideBack
			}
		}
		if planeEqualOriented(s.sidePlane(), p) {
			bits |= psideFacing
		}
	}
	return bits
}

// splitBrush splits b by the oriented plane (planenum pn): front = piece on
// the positive side (dot(n,x) >= d), back = the rest. Either may be nil.
// Both pieces gain a cap on the split plane (the cross-section of the brush
// at the plane), oriented outward for each piece.
func splitBrush(a *windingArena, b *bspBrush, pn int, p plane) (*bspBrush, *bspBrush) {
	var fs, bs []bspSide
	for _, s := range b.sides {
		fw, fok := clipWinding(a, s.w, p)
		if fok && len(fw) >= 3 {
			fs = append(fs, bspSide{planenum: s.planenum, n: s.n, d: s.d, w: fw, onnode: s.onnode})
		}
		bw, bok := clipWinding(a, s.w, negPlane(p))
		if bok && len(bw) >= 3 {
			bs = append(bs, bspSide{planenum: s.planenum, n: s.n, d: s.d, w: bw, onnode: s.onnode})
		}
	}
	// Cap polygon: the intersection of plane p with the brush volume.
	cap := brushCrossSection(a, b, p)
	if len(cap) < 3 {
		// ericw SplitBrush: the brush "isn't really split" — preserve it
		// WHOLE on the side with the farthest vertex instead of losing the
		// solid (which opens leaks on non-closed id1 brushes).
		if brushMostlyOnSide(b, p) {
			return b, nil
		}
		return nil, b
	}
	var front, back *bspBrush
	// Each child keeps whatever side windings survive the clip plus the cap
	// (ericw ClipBrushToFace): a wedge can legitimately have 1-2 side
	// windings, so pieces are never dropped for having few sides. Only a
	// zero-volume piece is discarded.
	if len(fs) > 0 {
		np := p.Normal.Neg()
		capF := windingOrientTo(a, cap, np)
		front = &bspBrush{
			sides:   append(fs, bspSide{planenum: pn, n: np, d: -p.Dist, w: capF, onnode: true}),
			content: b.content,
			sortKey: b.sortKey,
		}
		front.bounds = brushBoundsOf(front)
		if brushDegenerate(front) {
			front = nil
		}
	}
	if len(bs) > 0 {
		capB := windingOrientTo(a, cap, p.Normal)
		back = &bspBrush{
			sides:   append(bs, bspSide{planenum: pn, n: p.Normal, d: p.Dist, w: capB, onnode: true}),
			content: b.content,
			sortKey: b.sortKey,
		}
		back.bounds = brushBoundsOf(back)
		if brushDegenerate(back) {
			back = nil
		}
	}
	return front, back
}

// brushMostlyOnSide mirrors ericw BrushMostlyOnSide: the side of the plane
// holding the vertex farthest from it wins (magnitude comparison).
func brushMostlyOnSide(b *bspBrush, p plane) bool {
	max, front := 0.0, true
	for _, s := range b.sides {
		for _, v := range s.w {
			d := v3Dot(v, p.Normal) - p.Dist
			if d > max {
				max, front = d, true
			}
			if -d > max {
				max, front = -d, false
			}
		}
	}
	return front
}

// brushCrossSection returns the polygon of plane p inside the brush
// (seeded from the brush AABB, clipped by every side on the interior).
func brushCrossSection(ar *windingArena, b *bspBrush, p plane) winding {
	w := windingFromBoxPlane(ar, p, b.bounds[0], b.bounds[1])
	if w == nil {
		return nil
	}
	for _, s := range b.sides {
		clipped, ok := clipWindingKeepBack(ar, w, s.sidePlane())
		if !ok {
			return nil
		}
		w = clipped
	}
	return windingRemoveColinear(ar, w)
}

// clipWindingKeepBack clips w to the BACK side (dot <= d) of p, keeping
// on-plane points.
func clipWindingKeepBack(a *windingArena, w winding, p plane) (winding, bool) {
	return clipWinding(a, w, negPlane(p))
}

// subtractBrush returns the pieces of a that remain after subtracting the
// volume of b. The result is empty when a is entirely inside b.
func subtractBrush(ar *windingArena, a, b *bspBrush) []*bspBrush {
	cur := a
	var out []*bspBrush
	for _, s := range b.sides {
		f, bk := splitBrush(ar, cur, 0, s.sidePlane())
		if f != nil {
			out = append(out, f)
		}
		if bk == nil {
			// a lies entirely on the outside of this side: no intersection.
			return []*bspBrush{a}
		}
		cur = bk
	}
	return out
}

// brushDegenerate reports whether a brush has no real volume (a
// zero-thickness cap sliver from splitting exactly on a face plane).
func brushDegenerate(b *bspBrush) bool {
	v := (b.bounds[1].X - b.bounds[0].X) *
		(b.bounds[1].Y - b.bounds[0].Y) *
		(b.bounds[1].Z - b.bounds[0].Z)
	return v < 0.01
}

// brushesDisjoint reports whether a and b definitely do not intersect
// (AABB disjoint or opposing planes).
func brushesDisjoint(a, b *bspBrush) bool {
	if a.bounds[1].X < b.bounds[0].X || b.bounds[1].X < a.bounds[0].X ||
		a.bounds[1].Y < b.bounds[0].Y || b.bounds[1].Y < a.bounds[0].Y ||
		a.bounds[1].Z < b.bounds[0].Z || b.bounds[1].Z < a.bounds[0].Z {
		return true
	}
	for _, as := range a.sides {
		for _, bs := range b.sides {
			if planeEqualOriented(as.sidePlane(), bs.sidePlane()) &&
				v3Dot(as.n, bs.n) < 0 {
				// opposing planes: a and b face away from each other.
				return true
			}
		}
	}
	return false
}

// brushGE reports whether b1 may bite b2 in ChopBrushes: b1 is ordered
// after b2 (later brushes win) and both are solid.
func brushGE(b1, b2 *bspBrush) bool {
	if b1.sortKey < b2.sortKey {
		return false
	}
	return b1.content == bsp.ContentsSolid && b2.content == bsp.ContentsSolid
}

// chopBrushes carves intersecting solid brushes so no two solid brush
// volumes overlap, keeping the classic "later brushes win" ordering.
func chopBrushes(ar *windingArena, list []*bspBrush) []*bspBrush {
	out := list
	i := 0
	for i < len(out) {
		b1 := out[i]
		changed := false
		for j := i + 1; j < len(out) && !changed; j++ {
			b2 := out[j]
			if brushesDisjoint(b1, b2) {
				continue
			}
			var sub, sub2 []*bspBrush
			c1, c2 := int(^uint(0)>>1), int(^uint(0)>>1)
			if brushGE(b2, b1) {
				sub = subtractBrush(ar, b1, b2)
				if len(sub) == 1 && sub[0] == b1 {
					continue
				}
				if len(sub) == 0 {
					// b1 swallowed by b2.
					out = append(out[:i], out[i+1:]...)
					changed = true
					break
				}
				c1 = len(sub)
			}
			if brushGE(b1, b2) {
				sub2 = subtractBrush(ar, b2, b1)
				if len(sub2) == 1 && sub2[0] == b2 {
					continue
				}
				if len(sub2) == 0 {
					// b2 swallowed by b1.
					out = append(out[:j], out[j+1:]...)
					changed = true
					break
				}
				c2 = len(sub2)
			}
			if len(sub) == 0 && len(sub2) == 0 {
				continue // neither can bite
			}
			if c1 < c2 {
				// Replace b1 with the pieces of b1 minus b2.
				repl := append([]*bspBrush{}, sub...)
				out = append(out[:i], append(repl, out[i+1:]...)...)
				changed = true
				break
			}
			// Replace b2 with the pieces of b2 minus b1.
			repl := append([]*bspBrush{}, sub2...)
			out = append(out[:j], append(repl, out[j+1:]...)...)
			changed = true
			break
		}
		if !changed {
			i++
		}
	}
	return out
}
