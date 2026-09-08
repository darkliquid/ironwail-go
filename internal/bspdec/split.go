package bspdec

import (
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// inwardEdgePlane returns the plane through edge a->b, perpendicular to ref,
// oriented so refPt (the region) lies on its back (inside) side.
func inwardEdgePlane(a, b mapfile.Vec3, ref mapfile.Plane, refPt mapfile.Vec3) mapfile.Plane {
	n := v3Normalize(v3Cross(ref.Normal, v3Sub(b, a)))
	p := mapfile.Plane{Normal: n, Dist: v3Dot(n, a)}
	if v3Dot(p.Normal, refPt)-p.Dist > 0 {
		p = negatePlane(p)
	}
	return p
}

// splitDifferentTextures splits brushes whose side overlaps tree faces with
// more than one texinfo, so each output brush carries one texture per side.
//
// Where in C: SplitDifferentTexturedPartsOfBrush in ericw-tools
// common/decompile.cc. Simplified: only cleanly-separable planar splits are
// performed; unseparable mixes keep the dominant (largest-overlap) texture
// and warn, matching bsputil's best-match behavior.
func (d *decompiler) splitDifferentTextures(b *Brush) []*Brush {
	out := []*Brush{b}
	for i := 0; i < len(out); i++ {
		if parts := d.splitOneSide(out[i]); parts != nil {
			out = append(out[:i], append(parts, out[i+1:]...)...)
			i-- // re-check the inserted pieces for further splits
		}
	}
	return out
}

// splitOneSide performs one texture-boundary split, or returns nil when no
// side of b needs (or can cleanly make) one.
func (d *decompiler) splitOneSide(b *Brush) []*Brush {
	faces := d.collectFaces()
	for _, s := range b.Sides {
		var matches []texturedFace
		for _, f := range faces {
			if !planesMatch(s.Plane, f.plane) {
				continue
			}
			w := clipToBrush(f.winding, b, s.Plane)
			if w == nil {
				continue
			}
			matches = append(matches, texturedFace{plane: f.plane, winding: w, texName: f.texName, vecs: f.vecs})
		}
		distinct := map[string]bool{}
		for _, m := range matches {
			distinct[m.texName] = true
		}
		if len(distinct) < 2 {
			continue
		}
		// s.TexName is the dominant texinfo (assigned by textureBrush).
		// Try every edge of every minority region as a separating plane.
		for _, f := range matches {
			if f.texName == s.TexName {
				continue
			}
			for ei := range f.winding.Points {
				a := f.winding.Points[ei]
				bb := f.winding.Points[(ei+1)%len(f.winding.Points)]
				ep := inwardEdgePlane(a, bb, s.Plane, f.winding.Centroid())
				clean := true
				for _, o := range matches {
					if o.texName == f.texName {
						continue
					}
					// any other-texinfo region with area inside ep straddles
					if o.winding.Clip(negatePlane(ep)) != nil {
						clean = false
						break
					}
				}
				if !clean {
					continue
				}
				region := clipBrush(b, ep)
				rest := clipBrush(b, negatePlane(ep))
				if region == nil || rest == nil {
					continue // split plane coincides with a brush boundary
				}
				d.textureBrush(region)
				d.textureBrush(rest)
				return []*Brush{region, rest}
			}
		}
		d.warnf("side has %d mixed texinfos with no clean split plane; keeping dominant %q", len(distinct), s.TexName)
	}
	return nil
}