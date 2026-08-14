// Package decal implements the CPU-side mark (bullet hole, scorch) system:
// mark lifetime management, atlas seeding, draw sorting and quad geometry.
// It is the single home for the shared decal helpers previously duplicated
// between the root renderer package and internal/renderer/world/gogpu.
package decal

import (
	"math"
	"sort"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// MarkEntity is the minimal geometry a projected mark needs for draw
// preparation. DecalMarkEntity in the parent renderer package satisfies it.
type MarkEntity interface {
	DecalOrigin() types.Vec3
	DecalNormal() types.Vec3
	DecalSize() float32
	DecalRotation() float32
	// DecalAlpha returns the mark's opacity in [0,1]; PrepareDraws clamps it.
	DecalAlpha() float32
	// DecalVariant returns the mark's atlas variant selector.
	DecalVariant() int
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

// AddMark appends a mark with lifetime in seconds. Non-positive lifetimes are
// ignored; marks with zero size or clamped-zero alpha are dropped, mirroring
// the original root DecalMarkSystem.AddMark.
func (s *System) AddMark(mark MarkEntity, lifetimeSeconds, timeNow float32) {
	if s == nil || lifetimeSeconds <= 0 {
		return
	}
	if mark == nil || mark.DecalSize() <= 0 || clamp01(mark.DecalAlpha()) <= 0 {
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
// Behavior mirrors the original root prepareDecalDraws: marks with
// non-positive size are dropped, zero normals are defaulted to +Z, alpha is
// clamped to [0,1] (alpha<=0 drops the mark), and the variant is normalized
// via NormalizeVariant.
func PrepareDraws(marks []MarkEntity, cameraOrigin types.Vec3) []Draw {
	draws := make([]Draw, 0, len(marks))
	for _, mark := range marks {
		if mark.DecalSize() <= 0 {
			continue
		}
		if mark.DecalNormal() == (types.Vec3{}) {
			mark = newNormalMark(mark, types.Vec3{X: 0, Y: 0, Z: 1})
		}
		alpha := clamp01(mark.DecalAlpha())
		if alpha <= 0 {
			continue
		}
		variant := NormalizeVariant(mark.DecalVariant())
		draws = append(draws, Draw{Mark: normalizedMark{mark, alpha, variant}, DistanceSq: DistanceSq(mark.DecalOrigin(), cameraOrigin)})
	}

	sort.SliceStable(draws, func(i, j int) bool {
		return draws[i].DistanceSq > draws[j].DistanceSq
	})
	return draws
}

// MarkEntity must also expose alpha and variant for filtering/clamping.
// The DecalMarkEntity in the parent renderer satisfies this extended
// interface (via Alpha()/Variant() accessors added by the shim), but to keep
// this package decoupled we re-type through the narrow accessors below.

type normalizedMark struct {
	inner   MarkEntity
	alpha   float32
	variant int
}

func (m normalizedMark) DecalOrigin() types.Vec3 { return m.inner.DecalOrigin() }
func (m normalizedMark) DecalNormal() types.Vec3 { return m.inner.DecalNormal() }
func (m normalizedMark) DecalSize() float32      { return m.inner.DecalSize() }
func (m normalizedMark) DecalRotation() float32  { return m.inner.DecalRotation() }
func (m normalizedMark) DecalAlpha() float32     { return m.alpha }
func (m normalizedMark) DecalVariant() int       { return m.variant }

func newNormalMark(m MarkEntity, normal types.Vec3) MarkEntity {
	return normalMark{inner: m, normal: normal}
}

type normalMark struct {
	inner  MarkEntity
	normal types.Vec3
}

func (m normalMark) DecalOrigin() types.Vec3 { return m.inner.DecalOrigin() }
func (m normalMark) DecalNormal() types.Vec3 { return m.normal }
func (m normalMark) DecalSize() float32      { return m.inner.DecalSize() }
func (m normalMark) DecalRotation() float32  { return m.inner.DecalRotation() }
func (m normalMark) DecalAlpha() float32     { return m.inner.DecalAlpha() }
func (m normalMark) DecalVariant() int       { return m.inner.DecalVariant() }

// NormalizeVariant clamps an atlas variant selector to the four known
// regions (bullet, chip, scorch, magic), defaulting invalid values to bullet.
func NormalizeVariant(variant int) int {
	switch variant {
	case 0, 1, 2, 3:
		return variant
	default:
		return 0
	}
}

// DistanceSq returns the squared distance between two points.
func DistanceSq(origin, camera types.Vec3) float32 {
	d := origin.Sub(camera)
	return d.X*d.X + d.Y*d.Y + d.Z*d.Z
}

// BuildQuad computes the four corners of a projected mark quad in world
// space. The quad is centered 0.05 units in front of the surface along the
// normal, oriented by the mark rotation.
func BuildQuad(mark MarkEntity) ([4]types.Vec3, bool) {
	var corners [4]types.Vec3
	normal, ok := Normalize3(mark.DecalNormal())
	if !ok {
		return corners, false
	}

	tangent, bitangent := BuildBasis(normal, mark.DecalRotation())
	half := mark.DecalSize() * 0.5
	if half <= 0 {
		return corners, false
	}

	center := mark.DecalOrigin().Add(normal.Scale(0.05))
	offsets := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for i, o := range offsets {
		a := tangent.Scale(o[0] * half)
		b := bitangent.Scale(o[1] * half)
		corners[i] = center.Add(a).Add(b)
	}
	return corners, true
}

// BuildBasis computes the tangent/bitangent basis for a mark quad given the
// surface normal and a rotation around it.
func BuildBasis(normal types.Vec3, rotation float32) (tangent types.Vec3, bitangent types.Vec3) {
	up := types.Vec3{X: 0, Y: 0, Z: 1}
	if float32(math.Abs(float64(normal.Z))) > 0.99 {
		up = types.Vec3{X: 0, Y: 1, Z: 0}
	}

	tangent, _ = Normalize3(up.Cross(normal))
	bitangent = normal.Cross(tangent)

	if rotation != 0 {
		c := float32(math.Cos(float64(rotation)))
		s := float32(math.Sin(float64(rotation)))
		rotT := tangent.Scale(c).Add(bitangent.Scale(s))
		rotB := bitangent.Scale(c).Add(tangent.Scale(-s))
		tangent, _ = Normalize3(rotT)
		bitangent, _ = Normalize3(rotB)
	}
	return tangent, bitangent
}

// Add3 returns the element-wise sum of two 3-vectors.
func Add3(a, b types.Vec3) types.Vec3 {
	return a.Add(b)
}

// Scale3 scales a 3-vector by a scalar.
func Scale3(a types.Vec3, s float32) types.Vec3 {
	return a.Scale(s)
}

// Cross3 returns the cross product of two 3-vectors.
func Cross3(a, b types.Vec3) types.Vec3 {
	return a.Cross(b)
}

// Normalize3 normalizes a 3-vector, reporting false for null vectors.
func Normalize3(v types.Vec3) (types.Vec3, bool) {
	lengthSq := v.X*v.X + v.Y*v.Y + v.Z*v.Z
	if lengthSq <= 1e-12 {
		return types.Vec3{}, false
	}
	invLen := float32(1.0 / math.Sqrt(float64(lengthSq)))
	return types.Vec3{X: v.X * invLen, Y: v.Y * invLen, Z: v.Z * invLen}, true
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
