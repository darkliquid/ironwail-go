// Package decal implements the CPU-side mark (bullet hole, scorch) system:
// mark lifetime management, atlas seeding, draw sorting and quad geometry.
// It is the single home for the shared decal helpers previously duplicated
// between the root renderer package and internal/renderer/world/gogpu.
package decal

import (
	"math"
	"sort"
)

// MarkEntity is the minimal geometry a projected mark needs for draw
// preparation. DecalMarkEntity in the parent renderer package satisfies it.
type MarkEntity interface {
	DecalOrigin() [3]float32
	DecalNormal() [3]float32
	DecalSize() float32
	DecalRotation() float32
}

// System keeps projected mark entities alive for a limited lifetime.
type System struct {
	marks []timedMark
}

type timedMark struct {
	mark  MarkEntity
	dieAt float32
}

// NewSystem creates an empty mark system.
func NewSystem() *System {
	return &System{marks: make([]timedMark, 0, 256)}
}

// AddMark appends a mark with lifetime in seconds. Non-positive lifetimes are ignored.
func (s *System) AddMark(mark MarkEntity, lifetimeSeconds, timeNow float32) {
	if s == nil || lifetimeSeconds <= 0 {
		return
	}
	if mark == nil || mark.DecalSize() <= 0 {
		return
	}
	s.marks = append(s.marks, timedMark{mark: mark, dieAt: timeNow + lifetimeSeconds})
}

// Run advances mark expiration.
func (s *System) Run(timeNow float32) {
	if s == nil || len(s.marks) == 0 {
		return
	}
	alive := 0
	for i := range s.marks {
		if s.marks[i].dieAt > timeNow {
			s.marks[alive] = s.marks[i]
			alive++
		}
	}
	s.marks = s.marks[:alive]
}

// ActiveMarkEntities returns a copy of currently visible marks.
func (s *System) ActiveMarkEntities() []MarkEntity {
	if s == nil || len(s.marks) == 0 {
		return nil
	}
	out := make([]MarkEntity, 0, len(s.marks))
	for i := range s.marks {
		out = append(out, s.marks[i].mark)
	}
	return out
}

// ActiveCount returns number of currently active marks.
func (s *System) ActiveCount() int {
	if s == nil {
		return 0
	}
	return len(s.marks)
}

// AtlasData generates the deterministic 256x256 RGBA decal atlas texture
// used by the decal pipeline. The atlas is divided into four 128x128 regions
// (crack, chip, ring, swirl) selected by the mark variant.
func AtlasData() []byte {
	const atlasSize = 256
	const regionSize = 128
	data := make([]byte, atlasSize*atlasSize*4)

	for y := 0; y < atlasSize; y++ {
		for x := 0; x < atlasSize; x++ {
			regionX := x / regionSize
			regionY := y / regionSize
			variant := regionY*2 + regionX

			localX := float32(x%regionSize) / float32(regionSize)
			localY := float32(y%regionSize) / float32(regionSize)
			px := localX*2.0 - 1.0
			py := localY*2.0 - 1.0
			d2 := px*px + py*py

			var pattern float32
			switch variant {
			case 0:
				crack := float32(math.Abs(math.Sin(float64(px*28.0)) * math.Cos(float64(py*19.0))))
				pattern = 0.55 + 0.45*crack
			case 1:
				if d2 <= 1.0 {
					chip := float32(1.0 - Smoothstep(0.15, 0.9, d2))
					pattern = 0.5 + 0.5*chip
				}
			case 2:
				ring := float32(math.Abs(float64(d2 - 0.55)))
				pattern = 1.0 - Smoothstep(0.02, 0.26, ring)
			case 3:
				angle := float32(math.Atan2(float64(py), float64(px)))
				swirl := 0.5 + 0.5*float32(math.Sin(float64(18.0*angle+20.0*d2)))
				pattern = 0.35 + 0.65*swirl
			}

			var alpha float32
			if d2 > 1.0 {
				alpha = 0
			} else {
				alpha = Smoothstep(0.9, 0.5, d2)
			}

			idx := (y*atlasSize + x) * 4
			val := byte(pattern * 255)
			data[idx+0] = val
			data[idx+1] = val
			data[idx+2] = val
			data[idx+3] = byte(alpha * 255)
		}
	}

	return data
}

// Smoothstep performs clamped smooth interpolation between two edges.
func Smoothstep(edge0, edge1, x float32) float32 {
	t := clamp01((x - edge0) / (edge1 - edge0))
	return t * t * (3.0 - 2.0*t)
}

// Draw is a mark paired with its squared distance to the camera, used to
// sort far-to-near before drawing.
type Draw struct {
	Mark       MarkEntity
	DistanceSq float32
}

// PrepareDraws filters and sorts marks into far-to-near draw order.
// Marks with non-positive size or alpha are dropped. Zero normals default to
// +Z. The SortVariants bool is retained for future use.
func PrepareDraws(marks []MarkEntity, cameraOrigin [3]float32) []Draw {
	draws := make([]Draw, 0, len(marks))
	for _, mark := range marks {
		if mark.DecalSize() <= 0 {
			continue
		}
		draws = append(draws, Draw{Mark: mark, DistanceSq: DistanceSq(mark.DecalOrigin(), cameraOrigin)})
	}

	sort.SliceStable(draws, func(i, j int) bool {
		return draws[i].DistanceSq > draws[j].DistanceSq
	})
	return draws
}

// DistanceSq returns the squared distance between two points.
func DistanceSq(origin, camera [3]float32) float32 {
	dx := origin[0] - camera[0]
	dy := origin[1] - camera[1]
	dz := origin[2] - camera[2]
	return dx*dx + dy*dy + dz*dz
}

// BuildQuad computes the four corners of a projected mark quad in world
// space. The quad is centered 0.05 units in front of the surface along the
// normal, oriented by the mark rotation.
func BuildQuad(mark MarkEntity) ([4][3]float32, bool) {
	var corners [4][3]float32
	normal, ok := Normalize3(mark.DecalNormal())
	if !ok {
		return corners, false
	}

	tangent, bitangent := BuildBasis(normal, mark.DecalRotation())
	half := mark.DecalSize() * 0.5
	if half <= 0 {
		return corners, false
	}

	center := Add3(mark.DecalOrigin(), Scale3(normal, 0.05))
	offsets := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for i, o := range offsets {
		a := Scale3(tangent, o[0]*half)
		b := Scale3(bitangent, o[1]*half)
		corners[i] = Add3(Add3(center, a), b)
	}
	return corners, true
}

// BuildBasis computes the tangent/bitangent basis for a mark quad given the
// surface normal and a rotation around it.
func BuildBasis(normal [3]float32, rotation float32) (tangent [3]float32, bitangent [3]float32) {
	up := [3]float32{0, 0, 1}
	if float32(math.Abs(float64(normal[2]))) > 0.99 {
		up = [3]float32{0, 1, 0}
	}

	tangent, _ = Normalize3(Cross3(up, normal))
	bitangent = Cross3(normal, tangent)

	if rotation != 0 {
		c := float32(math.Cos(float64(rotation)))
		s := float32(math.Sin(float64(rotation)))
		rotT := Add3(Scale3(tangent, c), Scale3(bitangent, s))
		rotB := Add3(Scale3(bitangent, c), Scale3(tangent, -s))
		tangent, _ = Normalize3(rotT)
		bitangent, _ = Normalize3(rotB)
	}
	return tangent, bitangent
}

// Add3 returns the element-wise sum of two 3-vectors.
func Add3(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// Scale3 scales a 3-vector by a scalar.
func Scale3(a [3]float32, s float32) [3]float32 {
	return [3]float32{a[0] * s, a[1] * s, a[2] * s}
}

// Cross3 returns the cross product of two 3-vectors.
func Cross3(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// Normalize3 normalizes a 3-vector, reporting false for null vectors.
func Normalize3(v [3]float32) ([3]float32, bool) {
	lengthSq := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
	if lengthSq <= 1e-12 {
		return [3]float32{}, false
	}
	invLen := float32(1.0 / math.Sqrt(float64(lengthSq)))
	return [3]float32{v[0] * invLen, v[1] * invLen, v[2] * invLen}, true
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
