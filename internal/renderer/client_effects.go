package renderer

import (
	"math"
	"math/rand"

	cl "github.com/ironwail/ironwail-go/internal/client"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

func EmitClientEffects(ps *ParticleSystem, particleEvents []cl.ParticleEvent, tempEntities []cl.TempEntityEvent, rng *rand.Rand, timeNow float32) {
	if ps == nil {
		return
	}

	var zero [3]float32
	for _, event := range particleEvents {
		if event.Count <= 0 {
			continue
		}
		ps.RunParticleEffect(event.Origin, event.Dir, byte(event.Color), event.Count, rng, timeNow)
	}

	for _, event := range tempEntities {
		switch event.Type {
		case inet.TE_SPIKE:
			ps.RunParticleEffect(event.Origin, zero, 0, 10, rng, timeNow)
		case inet.TE_SUPERSPIKE:
			ps.RunParticleEffect(event.Origin, zero, 0, 20, rng, timeNow)
		case inet.TE_GUNSHOT:
			ps.RunParticleEffect(event.Origin, zero, 0, 20, rng, timeNow)
		case inet.TE_EXPLOSION:
			ps.RunParticleEffect(event.Origin, zero, 0, 1024, rng, timeNow)
		case inet.TE_TAREXPLOSION:
			ps.BlobExplosion(event.Origin, rng, timeNow)
		case inet.TE_WIZSPIKE:
			ps.RunParticleEffect(event.Origin, zero, 20, 30, rng, timeNow)
		case inet.TE_KNIGHTSPIKE:
			ps.RunParticleEffect(event.Origin, zero, 226, 20, rng, timeNow)
		case inet.TE_LAVASPLASH:
			ps.LavaSplash(event.Origin, rng, timeNow)
		case inet.TE_TELEPORT:
			ps.TeleportSplash(event.Origin, rng, timeNow)
		case inet.TE_EXPLOSION2:
			ps.ParticleExplosion2(event.Origin, event.ColorStart, event.ColorLength, rng, timeNow)
		case inet.TE_LIGHTNING1:
			ps.RocketTrail(event.Start, event.End, 5, rng, timeNow)
		case inet.TE_LIGHTNING2:
			ps.RocketTrail(event.Start, event.End, 6, rng, timeNow)
		case inet.TE_LIGHTNING3, inet.TE_BEAM:
			ps.RocketTrail(event.Start, event.End, 3, rng, timeNow)
		}
	}
}

func DrawParticles2D(dc RenderContext, ps *ParticleSystem) {
	if dc == nil || ps == nil || ps.ActiveCount() == 0 {
		return
	}

	for _, p := range ps.ActiveParticles() {
		x := int(p.Org[0])
		y := int(p.Org[1])
		dc.DrawFill(x-2, y-2, 4, 4, p.Color)
	}
}

// EmitDecalMarks maps temp-entity impact/explosion events to projected world-space marks.
func EmitDecalMarks(ms *DecalMarkSystem, tempEntities []cl.TempEntityEvent, rng *rand.Rand, timeNow float32) {
	if ms == nil || len(tempEntities) == 0 {
		return
	}

	for _, event := range tempEntities {
		switch event.Type {
		case inet.TE_GUNSHOT:
			ms.AddMark(DecalMarkEntity{
				Origin:   event.Origin,
				Normal:   [3]float32{0, 0, 1},
				Size:     8,
				Rotation: randomMarkRotation(rng),
				Color:    [3]float32{0.08, 0.08, 0.08},
				Alpha:    0.8,
				Variant:  DecalVariantBullet,
			}, 18.0, timeNow)

		case inet.TE_SPIKE, inet.TE_SUPERSPIKE:
			ms.AddMark(DecalMarkEntity{
				Origin:   event.Origin,
				Normal:   [3]float32{0, 0, 1},
				Size:     8,
				Rotation: randomMarkRotation(rng),
				Color:    [3]float32{0.08, 0.08, 0.08},
				Alpha:    0.8,
				Variant:  DecalVariantChip,
			}, 18.0, timeNow)

		case inet.TE_WIZSPIKE, inet.TE_KNIGHTSPIKE:
			ms.AddMark(DecalMarkEntity{
				Origin:   event.Origin,
				Normal:   [3]float32{0, 0, 1},
				Size:     9,
				Rotation: randomMarkRotation(rng),
				Color:    [3]float32{0.16, 0.14, 0.22},
				Alpha:    0.78,
				Variant:  DecalVariantMagic,
			}, 16.0, timeNow)

		case inet.TE_EXPLOSION, inet.TE_TAREXPLOSION, inet.TE_EXPLOSION2:
			ms.AddMark(DecalMarkEntity{
				Origin:   event.Origin,
				Normal:   [3]float32{0, 0, 1},
				Size:     24,
				Rotation: randomMarkRotation(rng),
				Color:    [3]float32{0.15, 0.10, 0.08},
				Alpha:    0.7,
				Variant:  DecalVariantScorch,
			}, 25.0, timeNow)
		}
	}
}

func randomMarkRotation(rng *rand.Rand) float32 {
	if rng == nil {
		return 0
	}
	return float32(rng.Float64() * 2.0 * math.Pi)
}
