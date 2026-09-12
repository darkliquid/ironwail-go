package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

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
func buildBspBrushFaces(ba *brushArena, faces []brushFace, box [2]vec3) brushRef {
	return buildBspBrushFacesClamped(ba, faces, box, box)
}

// buildBspBrushFacesClamped builds a solidbsp brush like
// buildBspBrushFaces but clamps every surviving winding to the brush's OWN
// map AABB (clampBox). Open brushes (a missing face, e.g. an id1 floor with
// no bottom) leave the opposing faces unbounded in that direction; without
// the clamp the world-box seed extends their windings across the map, and
// the stray vertices misclassify far split planes, routing pieces to the
// wrong leaves (the e1m1 mid-east floor slice loss, bead ironwail-go-aeh).
func buildBspBrushFacesClamped(ba *brushArena, faces []brushFace, box, clampBox [2]vec3) brushRef {
	var sides []sideRec
	for fi, o := range faces {
		side := sideRec{planenum: int32(o.pn), n: o.p.Normal, d: o.p.Dist}
		w := windingFromBoxPlaneRef(ba.w, o.p, box[0], box[1])
		if w.count != 0 {
			ok := true
			for gi, g := range faces {
				if fi == gi {
					continue
				}
				// Interior of g: dot(g.n, x) <= g.d  <=>  front of -g.
				clipped, cok := clipWindingKeepBackRef(ba.w, w, g.p)
				if !cok {
					ok = false
					break
				}
				w = clipped
			}
			if ok && w.count >= 3 {
				w = windingRemoveColinearRef(ba.w, w)
				if w.count >= 3 {
					clampView := ba.w.at(w)
					for i := range clampView {
						if clampView[i].X < clampBox[0].X {
							clampView[i].X = clampBox[0].X
						}
						if clampView[i].X > clampBox[1].X {
							clampView[i].X = clampBox[1].X
						}
						if clampView[i].Y < clampBox[0].Y {
							clampView[i].Y = clampBox[0].Y
						}
						if clampView[i].Y > clampBox[1].Y {
							clampView[i].Y = clampBox[1].Y
						}
						if clampView[i].Z < clampBox[0].Z {
							clampView[i].Z = clampBox[0].Z
						}
						if clampView[i].Z > clampBox[1].Z {
							clampView[i].Z = clampBox[1].Z
						}
					}
					side.w = windingOrientToRef(ba.w, w, o.p.Normal)
				}
			}
		}
		sides = append(sides, side)
	}
	if len(sides) < 4 {
		return -1
	}
	// A brush with fewer than 3 live windings has no verifiable volume
	// (hull expansion can degenerate sliver brushes into all-dead sides);
	// the pre-SoA code carried these as Inf-bounds ghosts.
	live := 0
	for i := range sides {
		if sides[i].w.count != 0 {
			live++
		}
	}
	if live < 3 {
		return -1
	}
	return ba.addBrush(sides, bsp.ContentsSolid, 0)
}

// classifyBrush returns the pside bits of the brush relative to plane p
// (table entry pn): FRONT/BACK bits for vertices on either side, FACING
// when a side is coplanar with p (the brush "touches" the plane).
// AABB pretest: a brush whose bounds lie entirely beyond the plane has
// all vertices on one side and no coplanar side (a coplanar side would
// put vertices on the plane, dragging the bounds to it), so the bits
// follow without walking any winding. The per-node classification loops
// call this O(brushes) times, so the bound test replaces thousands of
// vertex dot products (ericw TestBrushToPlanenum boxes the brush first
// the same way). The FACING check is an integer planenum identity: the
// plane table is orientation-canonical (planeEqualNear merges either
// normal direction), so sides on the same geometric plane share the entry.
func classifyBrush(ba *brushArena, b brushRef, pn int, p plane) int {
	br := &ba.brushes[b]
	dmin, dmax := planeDotRange(p, br.bounds[0], br.bounds[1])
	if dmin > splitEpsilon {
		return psideFront
	}
	if dmax < -splitEpsilon {
		return psideBack
	}
	bits := 0
	for i := range ba.sidesOf(b) {
		s := &ba.sidesOf(b)[i]
		for _, v := range ba.sideView(s) {
			d := v3Dot(p.Normal, v) - p.Dist
			if d > splitEpsilon {
				bits |= psideFront
			} else if d < -splitEpsilon {
				bits |= psideBack
			}
		}
		if s.planenum == int32(pn) {
			bits |= psideFacing
		}
	}
	return bits
}

// planeDotRange returns the min and max of dot(p.Normal, x) - p.Dist over
// the box [b0, b1].
func planeDotRange(p plane, b0, b1 vec3) (float64, float64) {
	var mn, mx float64
	for axis, c := range [3]float64{p.Normal.X, p.Normal.Y, p.Normal.Z} {
		lo := c * getAxis(b0, axis)
		hi := c * getAxis(b1, axis)
		if lo > hi {
			lo, hi = hi, lo
		}
		mn += lo
		mx += hi
	}
	return mn - p.Dist, mx - p.Dist
}

// splitBrush splits b by the oriented plane (planenum pn): front = piece on
// the positive side (dot(n,x) >= d), back = the rest. Either may be -1.
// Both pieces gain a cap on the split plane (the cross-section of the brush
// at the plane), oriented outward for each piece.
func splitBrush(ba *brushArena, b brushRef, pn int, p plane) (brushRef, brushRef) {
	br := &ba.brushes[b]
	var fs, bs []sideRec
	ba.splitBrushSides(b, p, &fs, &bs)
	// Cap polygon: the intersection of plane p with the brush volume.
	capRef := ba.brushCrossSectionRef(b, p)
	if capRef.count < 3 {
		// ericw SplitBrush: the brush "isn't really split" — preserve it
		// WHOLE on the side with the farthest vertex instead of losing the
		// solid (which opens leaks on non-closed id1 brushes).
		if brushMostlyOnSide(ba, b, p) {
			return b, -1
		}
		return -1, b
	}
	var front, back brushRef = -1, -1
	// Each child keeps whatever side windings survive the clip plus the cap
	// (ericw ClipBrushToFace): a wedge can legitimately have 1-2 side
	// windings, so pieces are never dropped for having few sides. Only a
	// zero-volume piece is discarded.
	if len(fs) > 0 {
		np := p.Normal.Neg()
		capF := windingOrientToRef(ba.w, capRef, np)
		fs = append(fs, sideRec{planenum: int32(pn), n: np, d: -p.Dist, w: capF, onnode: true})
		front = ba.addBrushDetail(fs, br.content, br.sortKey, br.detail)
		if brushDegenerate(ba, front) {
			front = -1
		}
	}
	if len(bs) > 0 {
		capB := windingOrientToRef(ba.w, capRef, p.Normal)
		bs = append(bs, sideRec{planenum: int32(pn), n: p.Normal, d: p.Dist, w: capB, onnode: true})
		back = ba.addBrushDetail(bs, br.content, br.sortKey, br.detail)
		if brushDegenerate(ba, back) {
			back = -1
		}
	}
	return front, back
}

// brushMostlyOnSide mirrors ericw BrushMostlyOnSide: the side of the plane
// holding the vertex farthest from it wins (magnitude comparison).
func brushMostlyOnSide(ba *brushArena, b brushRef, p plane) bool {
	max, front := 0.0, true
	for i := range ba.sidesOf(b) {
		s := &ba.sidesOf(b)[i]
		for _, v := range ba.sideView(s) {
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

// clipWindingKeepBackRef clips w to the BACK side (dot <= d) of p, keeping
// on-plane points.
func clipWindingKeepBackRef(a *windingArena, w wref, p plane) (wref, bool) {
	return clipWindingRef(a, w, negPlane(p))
}

// subtractBrush returns the pieces of a that remain after subtracting the
// volume of b. The result is empty when a is entirely inside b.
func subtractBrush(ba *brushArena, a, b brushRef) []brushRef {
	cur := a
	var out []brushRef
	for i := range ba.sidesOf(b) {
		s := &ba.sidesOf(b)[i]
		f, bk := splitBrush(ba, cur, 0, sidePlaneOf(s))
		if f != -1 {
			out = append(out, f)
		}
		if bk == -1 {
			// a lies entirely on the outside of this side: no intersection.
			return []brushRef{a}
		}
		cur = bk
	}
	return out
}

// brushDegenerate reports whether a brush has no real volume (a
// zero-thickness cap sliver from splitting exactly on a face plane).
func brushDegenerate(ba *brushArena, b brushRef) bool {
	bounds := ba.brushes[b].bounds
	v := (bounds[1].X - bounds[0].X) *
		(bounds[1].Y - bounds[0].Y) *
		(bounds[1].Z - bounds[0].Z)
	return v < 0.01
}

// brushesDisjoint reports whether a and b definitely do not intersect
// (AABB disjoint or opposing planes).
func brushesDisjoint(ba *brushArena, a, b brushRef) bool {
	abounds := &ba.brushes[a].bounds
	bbounds := &ba.brushes[b].bounds
	if abounds[1].X < bbounds[0].X || bbounds[1].X < abounds[0].X ||
		abounds[1].Y < bbounds[0].Y || bbounds[1].Y < abounds[0].Y ||
		abounds[1].Z < bbounds[0].Z || bbounds[1].Z < abounds[0].Z {
		return true
	}
	for i := range ba.sidesOf(a) {
		as := &ba.sidesOf(a)[i]
		for j := range ba.sidesOf(b) {
			bs := &ba.sidesOf(b)[j]
			if planeEqualOriented(sidePlaneOf(as), sidePlaneOf(bs)) &&
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
func brushGE(ba *brushArena, b1, b2 brushRef) bool {
	if ba.brushes[b1].sortKey < ba.brushes[b2].sortKey {
		return false
	}
	return ba.brushes[b1].content == bsp.ContentsSolid && ba.brushes[b2].content == bsp.ContentsSolid
}

// chopBrushes carves intersecting solid brushes so no two solid brush
// volumes overlap, keeping the classic "later brushes win" ordering.
func chopBrushes(ba *brushArena, list []brushRef) []brushRef {
	out := list
	i := 0
	for i < len(out) {
		b1 := out[i]
		changed := false
		for j := i + 1; j < len(out) && !changed; j++ {
			b2 := out[j]
			if brushesDisjoint(ba, b1, b2) {
				continue
			}
			var sub, sub2 []brushRef
			c1, c2 := int(^uint(0)>>1), int(^uint(0)>>1)
			if brushGE(ba, b2, b1) {
				sub = subtractBrush(ba, b1, b2)
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
			if brushGE(ba, b1, b2) {
				sub2 = subtractBrush(ba, b2, b1)
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
				repl := append([]brushRef{}, sub...)
				out = append(out[:i], append(repl, out[i+1:]...)...)
				changed = true
				break
			}
			// Replace b2 with the pieces of b2 minus b1.
			repl := append([]brushRef{}, sub2...)
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
