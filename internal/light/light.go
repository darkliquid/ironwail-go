package light

import (
	"math"
)

// Light is a point light entity.
type Light struct {
	Origin [3]float64
	Value  float64 // classic "light" key (intensity at unit distance)
	// Style is the classic Quake light style (0..31): separate lightmaps
	// per style, animated by matching lightstyle cvars.
	Style int
	// Color scales the light RGB (0..255 per channel; 255,255,255 =
	// white/uncolored).
	Color [3]float64
}

// Face is a parsed BSP face ready for lighting: its polygon, the texinfo
// vectors (s/t axes + offsets), the plane normal, and its index.
type Face struct {
	Index  int
	Poly   [][3]float64
	Vecs   [2][4]float64
	Normal [3]float64
	Sky    bool // sky faces receive no direct light
	NoDraw bool // skip (no lightmap)
	// Albedo is the face's mid-gray material brightness (0..1) used by the
	// radiosity pass; derived from the miptex pixel average (0.5 default
	// for untextured/black placeholders).
	Albedo float64
	// VNormals holds per-polygon-vertex phong-blended normals (nil = flat
	// shaded); see BuildPhongNormals.
	VNormals [][3]float64
	// Styles is filled by Bake with the face's distinct light styles.
	Styles [4]byte
}

// styleCount is the number of active styles on a face (from the styles
// array; 255 slots are empty padding).
func styleCount(styles [4]byte) int {
	n := 0
	for _, s := range styles {
		if s == 255 {
			break
		}
		n++
	}
	return n
}

// faceStyles returns the distinct styles present among the lights, padded
// with 255 (INVALID_LIGHTSTYLE — style 0 is a real style and cannot be
// used as the terminator).
func faceStyles(lights []Light) [4]byte {
	var out [4]byte
	for i := range out {
		out[i] = 255
	}
	n := 0
	for _, l := range lights {
		dup := false
		for i := 0; i < n; i++ {
			if out[i] == byte(l.Style) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		if n >= 4 {
			break
		}
		out[n] = byte(l.Style)
		n++
	}
	return out
}

// Result is the baked lighting for a BSP.
type Result struct {
	// Lighting is the mono lightmap lump (1 byte per sample), laid out per
	// face as one W*H block per style, consecutively (style 0 first).
	Lighting []byte
	// LightOfs is the style-0 lightmap offset per face (-1 for unlit).
	LightOfs []int32
	// Styles is the per-face styles[4] array written back into the BSP.
	Styles [][4]byte
	// Lit is the QLIT v1 colored sidecar (3 bytes per style-0 sample),
	// nil unless WriteLit was requested.
	Lit []byte
}

// directLight accumulates the intensity of all lights of the given style
// at the sample point, using the per-sample normal n (flat face normal or
// phong-interpolated), returning the RGB contribution (equally weighted
// from the light's color, mono when uncolored).
func directLight(f *Face, n [3]float64, p [3]float64, lights []Light, style int, trace func([3]float64, [3]float64) bool) (float64, float64, float64) {
	var r, g, b float64
	for _, l := range lights {
		if l.Style != style {
			continue
		}
		dx := l.Origin[0] - p[0]
		dy := l.Origin[1] - p[1]
		dz := l.Origin[2] - p[2]
		dist2 := dx*dx + dy*dy + dz*dz
		if dist2 < 1e-6 {
			dist2 = 1e-6
		}
		dist := math.Sqrt(dist2)
		cos := (n[0]*dx + n[1]*dy + n[2]*dz) / dist
		if cos <= 0 {
			continue // light behind the face
		}
		if trace != nil && trace(l.Origin, p) {
			continue // shadowed
		}
		cr := l.Value / dist2 * cos
		cr = math.Min(cr, 255)
		col := l.Color
		if col == [3]float64{} {
			// Uncolored light: white.
			col = [3]float64{255, 255, 255}
		}
		r += cr * (col[0] / 255)
		g += cr * (col[1] / 255)
		b += cr * (col[2] / 255)
	}
	return math.Min(r, 255), math.Min(g, 255), math.Min(b, 255)
}

// WriteLit serialises a QLIT v1 colored lightmap from the style-0 blocks
// carried in the result: "QLIT" + uint32 version 1 + 3 bytes (r,g,b) per
// style-0 sample, exactly what bsp.ApplyLitFile validates.
func WriteLit(res *Result) []byte {
	if res == nil || len(res.Lit) == 0 {
		return nil
	}
	out := make([]byte, 8+len(res.Lit))
	copy(out[0:4], "QLIT")
	binary_LittleEndianPutUint32(out[4:8], 1)
	copy(out[8:], res.Lit)
	return out
}

func binary_LittleEndianPutUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
