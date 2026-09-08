package bspdec

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
	if brushCuts(a, b) || brushCuts(b, a) {
		return nil
	}
	cand := &Brush{Contents: a.Contents}
	for _, s := range a.Sides {
		sharedSide := false
		for _, sb := range b.Sides {
			if planesOpposite(s.Plane, sb.Plane) {
				sharedSide = true
				break
			}
		}
		if !sharedSide {
			cand.Sides = append(cand.Sides, &Side{Plane: s.Plane, TexName: s.TexName, Vecs: s.Vecs})
		}
	}
	for _, s := range b.Sides {
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