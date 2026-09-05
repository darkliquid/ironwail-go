package light

import (
	"math"
)

// BakeOpts extends Bake with optional features (sun, radiosity bounces).
type BakeOpts struct {
	// Sun lights all non-sky faces from a directional sky light.
	Sun *Sun
	// Bounce is the radiosity bounce count (0 = direct only; 1 = single
	// bounce from lit surfaces onto their neighbours, clamped).
	Bounce int
}

// Bake computes per-style lightmaps for every face, optionally adding a
// sun term and radiosity bounces. Each lightmap sample is the classic
// Quake formula: intensity = value/dist^2 * cos(theta), clamped to 255,
// with a shadow trace against the BSP tree. Samples are 16 units apart in
// S/T space; the Lighting lump holds one W*H block per face style.
func Bake(faces []Face, lights []Light, trace func(from, to [3]float64) bool) Result {
	return bakeInternal(faces, lights, trace, BakeOpts{})
}

// BakeWithOpts is Bake with sun/bounce options.
func BakeWithOpts(faces []Face, lights []Light, trace func(from, to [3]float64) bool, opts BakeOpts) Result {
	res := bakeInternal(faces, lights, trace, opts)
	if opts.Bounce > 0 {
		res = applyBounce(res, faces, opts.Bounce, trace)
	}
	return res
}

// bakeInternal does the core per-face per-style light accumulation and the
// optional sun term.
func bakeInternal(faces []Face, lights []Light, trace func(from, to [3]float64) bool, opts BakeOpts) Result {
	var res Result
	res.LightOfs = make([]int32, len(faces))
	res.Styles = make([][4]byte, len(faces))
	for i := range res.LightOfs {
		res.LightOfs[i] = -1
	}

	for fi := range faces {
		f := &faces[fi]
		if f.NoDraw || f.Sky {
			continue
		}
		e := CalcExtents(f.Poly, f.Vecs)
		if e.W <= 0 || e.H <= 0 {
			continue
		}
		n := e.W * e.H
		styles := faceStyles(lights)
		f.Styles = styles
		res.Styles[fi] = styles
		res.LightOfs[fi] = int32(len(res.Lighting))
		for bi := 0; bi < styleCount(styles); bi++ {
			for i := 0; i < n; i++ {
				s := e.Mins[0] + (float64(i%e.W)+0.5)*16
				t := e.Mins[1] + (float64(i/e.W)+0.5)*16
				p := samplePoint(f, s, t)
				var r, g, b float64
				if bi == 0 {
					r, g, b = directLight(f, p, lights, int(styles[0]), trace)
					if opts.Sun != nil {
						sr, sg, sb := opts.Sun.SunLight(f, p)
						r += sr
						g += sg
						b += sb
					}
				} else {
					r, g, b = directLight(f, p, lights, int(styles[bi]), trace)
				}
				res.Lighting = append(res.Lighting, byte((r+g+b)/3))
				if bi == 0 {
					res.Lit = append(res.Lit, byte(math.Min(r, 255)), byte(math.Min(g, 255)), byte(math.Min(b, 255)))
				}
			}
		}
	}
	return res
}

// surfel is one baked style-0 lightmap sample used as a radiosity emitter.
type surfel struct {
	p     [3]float64
	n     [3]float64
	flux  float64 // albedo-weighted radiance
}

// applyBounce adds bounce passes of clamped colour-bleed radiosity from
// lit surfaces onto their neighbours: every lit style-0 surfel re-emits
// its radiance toward other samples, attenuated by 1/dist^2 * cos (both
// emitters and receivers), shadow-traced, and clamped to 255.
func applyBounce(res Result, faces []Face, bounces int, trace func(from, to [3]float64) bool) Result {
	// Gather style-0 surfels with world-space positions.
	var surfels []surfel
	for fi := range faces {
		f := &faces[fi]
		ofs := res.LightOfs[fi]
		if ofs < 0 {
			continue
		}
		e := CalcExtents(f.Poly, f.Vecs)
		if e.W <= 0 || e.H <= 0 {
			continue
		}
		n := e.W * e.H
		for i := 0; i < n; i++ {
			s := e.Mins[0] + (float64(i%e.W)+0.5)*16
			t := e.Mins[1] + (float64(i/e.W)+0.5)*16
			p := samplePoint(f, s, t)
			idx := int(ofs) + i
			if idx >= len(res.Lighting) {
				continue
			}
			v := float64(res.Lighting[idx])
			if v <= 4 {
				continue // unlit surfels don't emit
			}
			surfels = append(surfels, surfel{
				p:    p,
				n:    [3]float64{f.Normal[0], f.Normal[1], f.Normal[2]},
				flux: v * 0.5, // classic albedo-ish falloff
			})
		}
	}
	if len(surfels) < 2 {
		return res
	}
	// Emitter stride keeps the gather bounded on large maps.
	stride := 1
	if len(surfels) > 512 {
		stride = (len(surfels) + 511) / 512
	}
	add := make([]float64, len(res.Lighting))
	for b := 0; b < bounces; b++ {
		for k := range add {
			add[k] = 0
		}
		for ei := 0; ei < len(surfels); ei += stride {
			em := surfels[ei]
			for fi := range faces {
				f := &faces[fi]
				ofs := res.LightOfs[fi]
				if ofs < 0 || f.NoDraw || f.Sky {
					continue
				}
				ec := CalcExtents(f.Poly, f.Vecs)
				if ec.W <= 0 || ec.H <= 0 {
					continue
				}
				for i := 0; i < ec.W*ec.H; i++ {
					s := ec.Mins[0] + (float64(i%ec.W)+0.5)*16
					t := ec.Mins[1] + (float64(i/ec.W)+0.5)*16
					p := samplePoint(f, s, t)
					dx := p[0] - em.p[0]
					dy := p[1] - em.p[1]
					dz := p[2] - em.p[2]
					dist2 := dx*dx + dy*dy + dz*dz
					if dist2 < 1e-4 {
						continue
					}
					dist := math.Sqrt(dist2)
					cosE := (em.n[0]*dx + em.n[1]*dy + em.n[2]*dz) / dist
					cosR := (f.Normal[0]*(-dx) + f.Normal[1]*(-dy) + f.Normal[2]*(-dz)) / dist
					if cosE <= 0 || cosR <= 0 {
						continue
					}
					if trace != nil && trace(em.p, p) {
						continue
					}
					idx := int(ofs) + i
					if idx < 0 || idx >= len(add) {
						continue
					}
					// Surfel area ~ 256 units^2; falloff ~ 1/dist^2.
					add[idx] += em.flux * 256 * cosE * cosR / dist2
				}
			}
		}
		// Merge (clamped) into the style-0 mono and the .lit sidecar
		// (style-0 RGB triplets in face order).
		litBase := make([]int, len(faces))
		litOfs := 0
		for fi := range faces {
			litBase[fi] = litOfs
			if res.LightOfs[fi] >= 0 {
				ec := CalcExtents(faces[fi].Poly, faces[fi].Vecs)
				if ec.W > 0 && ec.H > 0 {
					litOfs += ec.W * ec.H
				}
			}
		}
		for fi := range faces {
			ofs := res.LightOfs[fi]
			if ofs < 0 {
				continue
			}
			ec := CalcExtents(faces[fi].Poly, faces[fi].Vecs)
			for i := 0; i < ec.W*ec.H; i++ {
				idx := int(ofs) + i
				res.Lighting[idx] = byte(math.Min(255, float64(res.Lighting[idx])+add[idx]))
				li := (litBase[fi] + i) * 3
				if li+2 < len(res.Lit) {
					addv := byte(math.Min(255, add[idx]))
					res.Lit[li] = byte(math.Min(255, float64(res.Lit[li])+float64(addv)))
					res.Lit[li+1] = byte(math.Min(255, float64(res.Lit[li+1])+float64(addv)))
					res.Lit[li+2] = byte(math.Min(255, float64(res.Lit[li+2])+float64(addv)))
				}
			}
		}
	}
	return res
}

// samplePoint maps a (s,t) texture coordinate to a world point on the face
// plane.
func samplePoint(f *Face, s, t float64) [3]float64 {
	n := f.Normal
	d := 0.0
	if len(f.Poly) > 0 {
		d = n[0]*f.Poly[0][0] + n[1]*f.Poly[0][1] + n[2]*f.Poly[0][2]
	}
	o := [3]float64{n[0] * d, n[1] * d, n[2] * d}
	return [3]float64{
		o[0] + s*f.Vecs[0][0] + t*f.Vecs[1][0],
		o[1] + s*f.Vecs[0][1] + t*f.Vecs[1][1],
		o[2] + s*f.Vecs[0][2] + t*f.Vecs[1][2],
	}
}