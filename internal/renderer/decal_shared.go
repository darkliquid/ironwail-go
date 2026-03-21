package renderer

import (
	"math"
	"sort"
)

type decalDraw struct {
	mark       DecalMarkEntity
	distanceSq float32
}

func generateDecalAtlasData() []byte {
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
					chip := float32(1.0 - smoothstepf(0.15, 0.9, d2))
					pattern = 0.5 + 0.5*chip
				}
			case 2:
				ring := float32(math.Abs(float64(d2 - 0.55)))
				pattern = 1.0 - smoothstepf(0.02, 0.26, ring)
			case 3:
				angle := float32(math.Atan2(float64(py), float64(px)))
				swirl := 0.5 + 0.5*float32(math.Sin(float64(18.0*angle+20.0*d2)))
				pattern = 0.35 + 0.65*swirl
			}

			alpha := float32(1.0)
			if d2 > 1.0 {
				alpha = 0
			} else {
				alpha = smoothstepf(0.9, 0.5, d2)
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

func smoothstepf(edge0, edge1, x float32) float32 {
	t := clamp01((x - edge0) / (edge1 - edge0))
	return t * t * (3.0 - 2.0*t)
}

func prepareDecalDraws(marks []DecalMarkEntity, camera CameraState) []decalDraw {
	draws := make([]decalDraw, 0, len(marks))
	for _, mark := range marks {
		if mark.Size <= 0 {
			continue
		}
		if mark.Normal == ([3]float32{}) {
			mark.Normal = [3]float32{0, 0, 1}
		}
		mark.Alpha = clamp01(mark.Alpha)
		mark.Variant = normalizeDecalVariant(mark.Variant)
		if mark.Alpha <= 0 {
			continue
		}
		draws = append(draws, decalDraw{mark: mark, distanceSq: decalDistanceSq(mark.Origin, camera)})
	}

	sort.SliceStable(draws, func(i, j int) bool {
		return draws[i].distanceSq > draws[j].distanceSq
	})
	return draws
}

func decalDistanceSq(origin [3]float32, camera CameraState) float32 {
	dx := origin[0] - camera.Origin.X
	dy := origin[1] - camera.Origin.Y
	dz := origin[2] - camera.Origin.Z
	return dx*dx + dy*dy + dz*dz
}

func normalizeDecalVariant(variant DecalVariant) DecalVariant {
	switch variant {
	case DecalVariantBullet, DecalVariantChip, DecalVariantScorch, DecalVariantMagic:
		return variant
	default:
		return DecalVariantBullet
	}
}

func buildDecalQuad(mark DecalMarkEntity) ([4][3]float32, bool) {
	var corners [4][3]float32
	normal, ok := decalNormalize3(mark.Normal)
	if !ok {
		return corners, false
	}

	tangent, bitangent := buildDecalBasis(normal, mark.Rotation)
	half := mark.Size * 0.5
	if half <= 0 {
		return corners, false
	}

	center := add3(mark.Origin, scale3(normal, 0.05))
	offsets := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for i, o := range offsets {
		a := scale3(tangent, o[0]*half)
		b := scale3(bitangent, o[1]*half)
		corners[i] = add3(add3(center, a), b)
	}
	return corners, true
}

func buildDecalBasis(normal [3]float32, rotation float32) (tangent [3]float32, bitangent [3]float32) {
	up := [3]float32{0, 0, 1}
	if float32(math.Abs(float64(normal[2]))) > 0.99 {
		up = [3]float32{0, 1, 0}
	}

	tangent, _ = decalNormalize3(cross3(up, normal))
	bitangent = cross3(normal, tangent)

	if rotation != 0 {
		c := float32(math.Cos(float64(rotation)))
		s := float32(math.Sin(float64(rotation)))
		rotT := add3(scale3(tangent, c), scale3(bitangent, s))
		rotB := add3(scale3(bitangent, c), scale3(tangent, -s))
		tangent, _ = decalNormalize3(rotT)
		bitangent, _ = decalNormalize3(rotB)
	}
	return tangent, bitangent
}

func add3(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func scale3(a [3]float32, s float32) [3]float32 {
	return [3]float32{a[0] * s, a[1] * s, a[2] * s}
}

func cross3(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func decalNormalize3(v [3]float32) ([3]float32, bool) {
	lengthSq := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
	if lengthSq <= 1e-12 {
		return [3]float32{}, false
	}
	invLen := float32(1.0 / math.Sqrt(float64(lengthSq)))
	return [3]float32{v[0] * invLen, v[1] * invLen, v[2] * invLen}, true
}
