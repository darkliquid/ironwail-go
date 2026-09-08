package light

import (
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// BuildPhongNormals computes interpolated per-vertex normals for faces
// whose shared vertices meet the phong angle threshold (classic
// CalcPointNormal semantics): a vertex normal averages the normals of all
// faces touching that vertex position whose face normals differ from the
// face's own by no more than maxAngle. Faces that gain a normal array are
// shaded per-sample in Bake instead of with their flat normal.
func BuildPhongNormals(faces []Face, maxAngleDeg float64) {
	if maxAngleDeg <= 0 || maxAngleDeg >= 180 {
		return
	}
	cosMax := math.Cos(maxAngleDeg * math.Pi / 180)

	// position -> faces at that vertex
	type key struct{ x, y, z float32 }
	posMap := make(map[key][]struct{ fi, vi int })
	for fi := range faces {
		f := &faces[fi]
		if f.NoDraw || f.Sky {
			continue
		}
		for vi := range f.Poly {
			v := f.Poly[vi]
			k := key{float32(v.X), float32(v.Y), float32(v.Z)}
			posMap[k] = append(posMap[k], struct{ fi, vi int }{fi, vi})
		}
	}

	for fi := range faces {
		f := &faces[fi]
		if f.NoDraw || f.Sky {
			continue
		}
		var vn []types.Vec3d
		for _, v := range f.Poly {
			// Classic phong vertex normal: the average of the face's own
			// normal and every face touching this vertex within the angle.
			sum := f.Normal
			count := 1
			for _, o := range posMap[key{float32(v.X), float32(v.Y), float32(v.Z)}] {
				if o.fi == fi {
					continue
				}
				of := &faces[o.fi]
				if of.NoDraw || of.Sky {
					continue
				}
				if f.Normal.Dot(of.Normal) < cosMax {
					continue
				}
				sum = sum.Add(of.Normal)
				count++
			}
			if count > 1 {
				l := sum.Len()
				if l > 1e-8 {
					vn = append(vn, sum.Scale(1.0/l))
					continue
				}
			}
			vn = append(vn, f.Normal)
		}
		if len(vn) == len(f.Poly) {
			changed := false
			for i := range vn {
				if vn[i].Dot(f.Normal) < 0.9999 {
					changed = true
					break
				}
			}
			if changed {
				f.VNormals = vn
			}
		}
	}
}

// interpolatedNormal returns the phong-smoothed normal at sample point p
// on the face: the polygon is fanned from vertex 0; the containing
// triangle's per-vertex normals are barycentrically blended. Falls back
// to the flat face normal.
func interpolatedNormal(f *Face, p types.Vec3d) types.Vec3d {
	n := len(f.Poly)
	if n < 3 || len(f.VNormals) != n {
		return f.Normal
	}
	p0 := f.Poly[0]
	for k := 1; k+1 < n; k++ {
		a, b, c := p0, f.Poly[k], f.Poly[k+1]
		if w, ok := barycentric(p, a, b, c); ok {
			na := f.VNormals[0]
			nb := f.VNormals[k]
			nc := f.VNormals[k+1]
			v := na.Scale(w[0]).Add(nb.Scale(w[1])).Add(nc.Scale(w[2]))
			l := v.Len()
			if l > 1e-8 {
				return v.Scale(1.0 / l)
			}
			return f.Normal
		}
	}
	return f.Normal
}

// barycentric computes the barycentric coordinates of p in triangle
// (a,b,c). The triangle is non-degenerate (area check).
func barycentric(p, a, b, c types.Vec3d) ([3]float64, bool) {
	v0 := b.Sub(a)
	v1 := c.Sub(a)
	v2 := p.Sub(a)
	d00 := v0.Dot(v0)
	d01 := v0.Dot(v1)
	d11 := v1.Dot(v1)
	d20 := v2.Dot(v0)
	d21 := v2.Dot(v1)
	den := d00*d11 - d01*d01
	if math.Abs(den) < 1e-12 {
		return [3]float64{}, false
	}
	w1 := (d11*d20 - d01*d21) / den
	w2 := (d00*d21 - d01*d20) / den
	w0 := 1 - w1 - w2
	if w0 < -1e-3 || w1 < -1e-3 || w2 < -1e-3 {
		return [3]float64{}, false
	}
	return [3]float64{w0, w1, w2}, true
}
