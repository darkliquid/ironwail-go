package bspdec

import mapfile "github.com/darkliquid/ironwail-go/pkg/map"

// mergeConvex greedily merges same-contents brush pairs whose union is
// convex, until no more pairs merge. O(n^2) per pass is fine for M0 map
// sizes; revisit only if corpus timings say so.
//
// Where in C: bspc -lessbrushes merging in map_q1.c (TryMerge analogue).
func mergeConvex(brushes []*Brush) []*Brush {
	used := make([]bool, len(brushes))
	var out []*Brush
	for i := range brushes {
		if used[i] {
			continue
		}
		cur := brushes[i]
		used[i] = true
		for merged := true; merged; {
			merged = false
			for j := range brushes {
				if used[j] {
					continue
				}
				if m := tryMerge(cur, brushes[j]); m != nil {
					cur = m
					used[j] = true
					merged = true
				}
			}
		}
		out = append(out, cur)
	}
	return out
}

// tryMerge returns the convex union of a and b, or nil. Preconditions for a
// valid merge: exactly one shared coplanar side pair; neither brush's planes
// cut the other's windings (convexity); and no texture conflict on any
// coplanar side pair (a merge would collapse the texture-boundary splits).
func tryMerge(a, b *Brush) *Brush {
	if a.Contents != b.Contents {
		return nil
	}
	shared := 0
	for _, sa := range a.Sides {
		for _, sb := range b.Sides {
			if planesOpposite(sa.Plane, sb.Plane) {
				shared++
			}
			// texture guard: same-direction coplanar sides must agree
			if planesMatch(sa.Plane, sb.Plane) && (sa.TexName != sb.TexName || !vecsEqual(sa.Vecs, sb.Vecs)) {
				return nil
			}
		}
	}
	if shared != 1 {
		return nil
	}
	// The shared opposite-coplanar pair's windings must coincide: a brush
	// sitting on a sliver of a larger coplanar face (a trim on a floor)
	// would otherwise merge into a polyhedron with overlapping faces that
	// recompiles with a hole (leak). Mutual containment is required.
	var sa, sb *Side
	for _, x := range a.Sides {
		for _, y := range b.Sides {
			if planesOpposite(x.Plane, y.Plane) {
				sa, sb = x, y
			}
		}
	}
	if !facesCoincide(a, b, sa, sb) {
		return nil
	}
	if brushCuts(a, b) || brushCuts(b, a) {
		return nil
	}
	cand := &Brush{Contents: a.Contents}
	var sharedPlane mapfile.Plane // the interior face removed by the union
	for _, s := range a.Sides {
		sharedSide := false
		for _, sb := range b.Sides {
			if planesOpposite(s.Plane, sb.Plane) {
				sharedSide = true
				sharedPlane = s.Plane
				break
			}
		}
		if !sharedSide {
			cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
		}
	}
	for _, s := range b.Sides {
		// the shared pair's complement is the same interior face (a's half
		// was skipped above); leaving it in would add a redundant plane
		// that clips the union to a sliver along the junction.
		if planesMatch(s.Plane, sharedPlane) || planesOpposite(s.Plane, sharedPlane) {
			continue
		}
		dupe := false
		for _, o := range cand.Sides {
			if planesMatch(s.Plane, o.Plane) || planesOpposite(s.Plane, o.Plane) {
				dupe = true // shared pair + coplanar dupes collapse
				break
			}
		}
		if !dupe {
			cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
		}
	}
	rebuildWindings(cand)
	kept := cand.Sides[:0]
	for _, s := range cand.Sides {
		if s.Winding != nil && len(s.Winding.Points) >= 3 {
			kept = append(kept, s)
		}
	}
	cand.Sides = kept
	if len(cand.Sides) < 4 {
		return nil
	}
	rebuildWindings(cand)
	return cand
}

// brushCuts reports whether any non-shared plane of a removes area from any
// winding of b (the union would be non-convex). The shared coplanar side
// pair is the interior face of the union and must be excluded; coplanar
// contact elsewhere is not a cut (on-plane points are not strictly front).
func brushCuts(a, b *Brush) bool {
	if cutsExceptShared(a, b) || cutsExceptShared(b, a) {
		return true
	}
	return false
}

func cutsExceptShared(a, b *Brush) bool {
	for _, sa := range a.Sides {
		if isSharedPlane(sa, b) {
			continue
		}
		front, back := false, false
		for _, os := range b.Sides {
			if os.Winding == nil {
				continue
			}
			for _, pt := range os.Winding.Points {
				d := v3Dot(pt, sa.Plane.Normal) - sa.Plane.Dist
				if d > onEpsilon {
					front = true
				}
				if d < -onEpsilon {
					back = true
				}
			}
		}
		// the plane slices b's interior only when b has area on both sides;
		// if b sits entirely on one side the union stays convex
		if front && back {
			return true
		}
	}
	return false
}

// isSharedPlane reports whether sa is one of the coplanar side pair joining
// two brushes (an interior face of the would-be union).
func isSharedPlane(sa *Side, b *Brush) bool {
	for _, sb := range b.Sides {
		if planesOpposite(sa.Plane, sb.Plane) {
			return true
		}
	}
	return false
}

// facesCoincide reports whether the shared opposite-coplanar face pair of a
// and b covers each other: each side's winding lies entirely inside the
// other brush's halfspaces. Coplanar contact without coverage (e.g. a small
// trim sitting on a large floor slab) yields an overlapping-face union that
// recompiles with a hole, so it must not merge.
func facesCoincide(a, b *Brush, sharedA, sharedB *Side) bool {
	return windingContained(sharedA, b, sharedB.Plane) && windingContained(sharedB, a, sharedA.Plane)
}

func windingContained(s *Side, other *Brush, skip mapfile.Plane) bool {
	w := s.Winding
	if w == nil || len(w.Points) < 3 {
		return false
	}
	for _, o := range other.Sides {
		if planesMatch(o.Plane, skip) || planesOpposite(o.Plane, skip) {
			continue
		}
		for _, p := range w.Points {
			if v3Dot(p, o.Plane.Normal)-o.Plane.Dist > onEpsilon {
				return false
			}
		}
	}
	return true
}
