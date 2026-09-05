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
func negPlane(p plane) plane { return plane{Normal: v3(-p.Normal[0], -p.Normal[1], -p.Normal[2]), Dist: -p.Dist} }

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
// with known plane-table indices, bounded by box.
func buildBspBrushFaces(faces []brushFace, box [2]vec3) *bspBrush {
	b := &bspBrush{content: bsp.ContentsSolid}
	for fi, o := range faces {
		w := windingFromBoxPlane(o.p, box[0], box[1])
		if w == nil {
			continue
		}
		ok := true
		for gi, g := range faces {
			if fi == gi {
				continue
			}
			// Interior of g: dot(g.n, x) <= g.d  <=>  front of -g.
			clipped, cok := clipWindingKeepBack(w, g.p)
			if !cok {
				ok = false
				break
			}
			w = clipped
		}
		if !ok || len(w) < 3 {
			continue
		}
		w = windingRemoveColinear(w)
		if len(w) < 3 {
			continue
		}
		w = windingOrientTo(w, o.p.Normal)
		b.sides = append(b.sides, bspSide{planenum: o.pn, n: o.p.Normal, d: o.p.Dist, w: w})
	}
	if len(b.sides) < 4 {
		return nil
	}
	b.bounds = brushBoundsOf(b)
	return b
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
		for i := 0; i < 3; i++ {
			if m[i] < mins[i] {
				mins[i] = m[i]
			}
			if x[i] > maxs[i] {
				maxs[i] = x[i]
			}
		}
	}
	if first {
		return [2]vec3{{0, 0, 0}, {0, 0, 0}}
	}
	return [2]vec3{mins, maxs}
}

// plane-side classification bits (classic qbsp PSIDE_*).
const (
	psideFront  = 1 << 0
	psideBack   = 1 << 1
	psideFacing = 1 << 2
)

const splitEpsilon = 0.001

// classifyBrush returns the pside bits of the brush relative to plane p:
// FRONT/BACK bits for vertices on either side, FACING when a side is
// coplanar with p (the brush "touches" the plane).
func classifyBrush(b *bspBrush, p plane) int {
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
func splitBrush(b *bspBrush, pn int, p plane) (*bspBrush, *bspBrush) {
	var fs, bs []bspSide
	for _, s := range b.sides {
		fw, fok := clipWinding(s.w, p)
		if fok && len(fw) >= 3 {
			fs = append(fs, bspSide{planenum: s.planenum, n: s.n, d: s.d, w: fw})
		}
		bw, bok := clipWinding(s.w, negPlane(p))
		if bok && len(bw) >= 3 {
			bs = append(bs, bspSide{planenum: s.planenum, n: s.n, d: s.d, w: bw})
		}
	}
	// Cap polygon: the intersection of plane p with the brush volume.
	cap := brushCrossSection(b, p)
	if len(cap) < 3 {
		return nil, nil
	}
	var front, back *bspBrush
	if len(fs) >= 3 {
		// Front child region: dot(p.Normal, x) >= p.Dist; its boundary at
		// the split plane has outward normal -p.Normal.
		np := v3(-p.Normal[0], -p.Normal[1], -p.Normal[2])
		capF := windingOrientTo(cap, np)
		front = &bspBrush{
			sides:   append(fs, bspSide{planenum: pn, n: np, d: -p.Dist, w: capF}),
			content: b.content,
			sortKey: b.sortKey,
		}
		front.bounds = brushBoundsOf(front)
	}
	if len(bs) >= 3 {
		// Back child region: dot(p.Normal, x) <= p.Dist; boundary outward
		// normal is +p.Normal.
		capB := windingOrientTo(cap, p.Normal)
		back = &bspBrush{
			sides:   append(bs, bspSide{planenum: pn, n: p.Normal, d: p.Dist, w: capB}),
			content: b.content,
			sortKey: b.sortKey,
		}
		back.bounds = brushBoundsOf(back)
	}
	return front, back
}

// brushCrossSection returns the polygon of plane p inside the brush
// (seeded from the brush AABB, clipped by every side on the interior).
func brushCrossSection(b *bspBrush, p plane) winding {
	w := windingFromBoxPlane(p, b.bounds[0], b.bounds[1])
	if w == nil {
		return nil
	}
	for _, s := range b.sides {
		clipped, ok := clipWindingKeepBack(w, s.sidePlane())
		if !ok {
			return nil
		}
		w = clipped
	}
	return windingRemoveColinear(w)
}

// clipWindingKeepBack clips w to the BACK side (dot <= d) of p, keeping
// on-plane points.
func clipWindingKeepBack(w winding, p plane) (winding, bool) {
	return clipWinding(w, negPlane(p))
}

// subtractBrush returns the pieces of a that remain after subtracting the
// volume of b. The result is empty when a is entirely inside b.
func subtractBrush(a, b *bspBrush) []*bspBrush {
	cur := a
	var out []*bspBrush
	for _, s := range b.sides {
		f, bk := splitBrush(cur, 0, s.sidePlane())
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

// brushesDisjoint reports whether a and b definitely do not intersect
// (AABB disjoint or opposing planes).
func brushesDisjoint(a, b *bspBrush) bool {
	for i := 0; i < 3; i++ {
		if a.bounds[1][i] < b.bounds[0][i] || b.bounds[1][i] < a.bounds[0][i] {
			return true
		}
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
func chopBrushes(list []*bspBrush) []*bspBrush {
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
				sub = subtractBrush(b1, b2)
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
				sub2 = subtractBrush(b2, b1)
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