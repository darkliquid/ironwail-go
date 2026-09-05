package light

import (
	"math"
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
			k := key{float32(v[0]), float32(v[1]), float32(v[2])}
			posMap[k] = append(posMap[k], struct{ fi, vi int }{fi, vi})
		}
	}

	for fi := range faces {
		f := &faces[fi]
		if f.NoDraw || f.Sky {
			continue
		}
		var vn [][3]float64
		for _, v := range f.Poly {
			// Classic phong vertex normal: the average of the face's own
			// normal and every face touching this vertex within the angle.
			sum := f.Normal
			count := 1
			for _, o := range posMap[key{float32(v[0]), float32(v[1]), float32(v[2])}] {
				if o.fi == fi {
					continue
				}
				of := &faces[o.fi]
				if of.NoDraw || of.Sky {
					continue
				}
				if dot3(f.Normal, of.Normal) < cosMax {
					continue
				}
				sum[0] += of.Normal[0]
				sum[1] += of.Normal[1]
				sum[2] += of.Normal[2]
				count++
			}
			if count > 1 {
				l := math.Sqrt(sum[0]*sum[0] + sum[1]*sum[1] + sum[2]*sum[2])
				if l > 1e-8 {
					vn = append(vn, [3]float64{sum[0] / l, sum[1] / l, sum[2] / l})
					continue
				}
			}
			vn = append(vn, f.Normal)
		}
		if len(vn) == len(f.Poly) {
			changed := false
			for i := range vn {
				if dot3(vn[i], f.Normal) < 0.9999 {
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

func dot3(a, b [3]float64) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// interpolatedNormal returns the phong-smoothed normal at sample point p
// on the face: the polygon is fanned from vertex 0; the containing
// triangle's per-vertex normals are barycentrically blended. Falls back
// to the flat face normal.
func interpolatedNormal(f *Face, p [3]float64) [3]float64 {
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
			v := [3]float64{
				w[0]*na[0] + w[1]*nb[0] + w[2]*nc[0],
				w[0]*na[1] + w[1]*nb[1] + w[2]*nc[1],
				w[0]*na[2] + w[1]*nb[2] + w[2]*nc[2],
			}
			l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
			if l > 1e-8 {
				return [3]float64{v[0] / l, v[1] / l, v[2] / l}
			}
			return f.Normal
		}
	}
	return f.Normal
}

// barycentric computes the barycentric coordinates of p in triangle
// (a,b,c). The triangle is non-degenerate (area check).
func barycentric(p, a, b, c [3]float64) ([3]float64, bool) {
	v0 := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	v1 := [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
	v2 := [3]float64{p[0] - a[0], p[1] - a[1], p[2] - a[2]}
	d00 := v0[0]*v0[0] + v0[1]*v0[1] + v0[2]*v0[2]
	d01 := v0[0]*v1[0] + v0[1]*v1[1] + v0[2]*v1[2]
	d11 := v1[0]*v1[0] + v1[1]*v1[1] + v1[2]*v1[2]
	d20 := v2[0]*v0[0] + v2[1]*v0[1] + v2[2]*v0[2]
	d21 := v2[0]*v1[0] + v2[1]*v1[1] + v2[2]*v1[2]
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
