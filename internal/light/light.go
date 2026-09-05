package light

import (
	"encoding/binary"
	"math"
)

// Light is a point light entity.
type Light struct {
	Origin [3]float64
	Value  float64 // classic "light" key (intensity at unit distance)
}

// Face is a parsed BSP face ready for lighting: its polygon, the texinfo
// vectors (s/t axes + offsets), the plane normal, and its index.
type Face struct {
	Index    int
	Poly     [][3]float64
	Vecs     [2][4]float64
	Normal   [3]float64
	Sky      bool // sky faces receive no direct light
	NoDraw   bool // skip (no lightmap)
}

// Result is the baked lighting for a BSP.
type Result struct {
	// Lighting is the mono lightmap lump (1 byte per sample), indexed by
	// face lightofs.
	Lighting []byte
	// LightOfs is the lightmap offset per face (-1 for unlit faces).
	LightOfs []int32
	// Lit is the QLIT v1 colored sidecar (3 bytes per sample), nil unless
	// WriteLit was requested.
	Lit []byte
}

// Bake computes direct point-light lighting for every face. Each lightmap
// sample is the classic Quake formula: intensity = value/dist^2 * cos(theta)
// (theta between the face normal and the light direction), clamped to 255,
// with a shadow trace against the BSP tree (a solid leaf between the light
// and the sample blocks it). Samples are 16 units apart in S/T space.
func Bake(faces []Face, lights []Light, trace func(from, to [3]float64) bool) Result {
	var res Result
	res.LightOfs = make([]int32, len(faces))
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
		// Lightmap offset: one byte per sample (mono).
		ofs := int32(len(res.Lighting))
		res.LightOfs[fi] = ofs
		samples := make([]byte, 0, e.W*e.H)
		for j := 0; j < e.H; j++ {
			for i := 0; i < e.W; i++ {
				// Sample point at the luxel centre in S/T space, mapped to
				// world space via the face plane.
				s := e.Mins[0] + (float64(i)+0.5)*16
				t := e.Mins[1] + (float64(j)+0.5)*16
				p := samplePoint(f, s, t)
				v := directLight(f, p, lights, trace)
				samples = append(samples, byte(v))
			}
		}
		res.Lighting = append(res.Lighting, samples...)
	}
	return res
}

// samplePoint maps a (s,t) texture coordinate to a world point on the face
// plane: the point is the plane point whose s/t projections equal the
// coordinate. We solve for the point on the plane with the given s,t by
// using the plane normal as the third basis vector.
func samplePoint(f *Face, s, t float64) [3]float64 {
	// Find a point on the plane: use the plane normal component.
	// p = o + u*vecs[0] + v*vecs[1] where o is on the plane.
	// Choose o = normal * d (the plane's closest point to the origin).
	n := f.Normal
	d := 0.0
	// The plane: n . p = d (dist from origin). We need the plane dist; the
	// face polygon's first vertex gives it.
	if len(f.Poly) > 0 {
		d = n[0]*f.Poly[0][0] + n[1]*f.Poly[0][1] + n[2]*f.Poly[0][2]
	}
	// o = n * d (closest point to origin on the plane).
	o := [3]float64{n[0] * d, n[1] * d, n[2] * d}
	// p = o + s*vecs[0] + t*vecs[1] is NOT on the plane unless vecs are
	// perpendicular to n (they are, for valid texinfo). Adjust: solve for
	// the point on the plane with the given s,t via the projection.
	// Since vecs are perpendicular to n, o + s*vecs[0] + t*vecs[1] lies on
	// the plane (n . (s*vecs[0]) = 0).
	return [3]float64{
		o[0] + s*f.Vecs[0][0] + t*f.Vecs[1][0],
		o[1] + s*f.Vecs[0][1] + t*f.Vecs[1][1],
		o[2] + s*f.Vecs[0][2] + t*f.Vecs[1][2],
	}
}

// directLight accumulates the intensity of all lights at the sample point.
func directLight(f *Face, p [3]float64, lights []Light, trace func([3]float64, [3]float64) bool) float64 {
	total := 0.0
	for _, l := range lights {
		dx := l.Origin[0] - p[0]
		dy := l.Origin[1] - p[1]
		dz := l.Origin[2] - p[2]
		dist2 := dx*dx + dy*dy + dz*dz
		if dist2 < 1e-6 {
			dist2 = 1e-6
		}
		dist := math.Sqrt(dist2)
		// Cosine of the angle between the face normal and the light.
		cos := (f.Normal[0]*dx + f.Normal[1]*dy + f.Normal[2]*dz) / dist
		if cos <= 0 {
			continue // light behind the face
		}
		if trace != nil && trace(l.Origin, p) {
			continue // shadowed
		}
		total += l.Value / dist2 * cos
	}
	if total > 255 {
		return 255
	}
	return total
}

// WriteLit serialises a QLIT v1 colored lightmap: "QLIT" + uint32 version 1
// + 3 bytes (r,g,b) per sample, exactly what bsp.ApplyLitFile validates.
func WriteLit(lighting []byte) []byte {
	out := make([]byte, 8+len(lighting)*3)
	copy(out[0:4], "QLIT")
	binary.LittleEndian.PutUint32(out[4:8], 1)
	for i, v := range lighting {
		out[8+i*3+0] = v
		out[8+i*3+1] = v
		out[8+i*3+2] = v
	}
	return out
}