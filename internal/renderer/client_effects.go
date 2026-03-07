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

// EmitDynamicLights maps temp-entity gameplay events to transient dynamic lights.
func EmitDynamicLights(spawn func(DynamicLight) bool, tempEntities []cl.TempEntityEvent) {
	if spawn == nil || len(tempEntities) == 0 {
		return
	}

	for _, event := range tempEntities {
		switch event.Type {
		case inet.TE_GUNSHOT, inet.TE_SPIKE:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     80,
				Color:      [3]float32{1.0, 0.85, 0.65},
				Brightness: 0.7,
				Lifetime:   0.08,
			})
		case inet.TE_SUPERSPIKE:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     96,
				Color:      [3]float32{1.0, 0.85, 0.65},
				Brightness: 0.85,
				Lifetime:   0.1,
			})
		case inet.TE_WIZSPIKE:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     110,
				Color:      [3]float32{0.35, 0.45, 1.0},
				Brightness: 0.9,
				Lifetime:   0.12,
			})
		case inet.TE_KNIGHTSPIKE:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     110,
				Color:      [3]float32{0.7, 0.35, 1.0},
				Brightness: 0.9,
				Lifetime:   0.12,
			})
		case inet.TE_EXPLOSION, inet.TE_EXPLOSION2:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     320,
				Color:      [3]float32{1.0, 0.55, 0.2},
				Brightness: 1.8,
				Lifetime:   0.55,
			})
		case inet.TE_TAREXPLOSION:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     280,
				Color:      [3]float32{0.5, 0.25, 0.85},
				Brightness: 1.5,
				Lifetime:   0.5,
			})
		case inet.TE_TELEPORT:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     220,
				Color:      [3]float32{0.45, 0.55, 1.0},
				Brightness: 1.2,
				Lifetime:   0.35,
			})
		case inet.TE_LIGHTNING1, inet.TE_LIGHTNING2, inet.TE_LIGHTNING3, inet.TE_BEAM:
			spawn(DynamicLight{
				Position:   midpoint3(event.Start, event.End),
				Radius:     160,
				Color:      [3]float32{0.55, 0.7, 1.0},
				Brightness: 1.1,
				Lifetime:   0.1,
			})
		}
	}
}

func midpoint3(a, b [3]float32) [3]float32 {
	return [3]float32{
		(a[0] + b[0]) * 0.5,
		(a[1] + b[1]) * 0.5,
		(a[2] + b[2]) * 0.5,
	}
}

func randomMarkRotation(rng *rand.Rand) float32 {
	if rng == nil {
		return 0
	}
	return float32(rng.Float64() * 2.0 * math.Pi)
}
