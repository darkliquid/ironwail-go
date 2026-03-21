package renderer

import (
	"math"
	"math/rand"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

// EmitClientEffects spawns transient render-side effects (muzzle flashes, trails, impacts) from current client/entity state.
func EmitClientEffects(ps *ParticleSystem, particleEvents []cl.ParticleEvent, trailEvents []cl.TrailEvent, tempEntities []cl.TempEntityEvent, rng *rand.Rand, timeNow float32) {
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

	// Process trail events emitted by RelinkEntities based on model flags.
	for _, event := range trailEvents {
		ps.RocketTrail(event.Start, event.End, event.Type, rng, timeNow)
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
		}
	}
}

// DrawParticles2D renders UI-space particles/effects that should appear on top of the 3D scene.
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
		case inet.TE_EXPLOSION, inet.TE_EXPLOSION2:
			spawn(DynamicLight{
				Position:   event.Origin,
				Radius:     320,
				Color:      [3]float32{1.0, 0.55, 0.2},
				Brightness: 1.8,
				Lifetime:   0.55,
			})
		}
	}
}

// EmitEntityEffectParticles maps runtime entity effect flags to transient particles.
func EmitEntityEffectParticles(ps *ParticleSystem, entities []EntityEffectSource, timeNow float32) {
	if ps == nil || len(entities) == 0 {
		return
	}

	for _, entity := range entities {
		if entity.Effects&inet.EF_BRIGHTFIELD != 0 {
			ps.EntityParticles(entity.Origin, timeNow)
		}
	}
}

// EmitEntityEffectLights maps runtime entity effect flags to transient dynamic lights.
// Uses keyed spawning so each entity reuses the same light slot across frames,
// matching C's CL_AllocDlight(entityNum) per-entity slot reuse behavior.
func EmitEntityEffectLights(spawn func(DynamicLight) bool, entities []EntityEffectSource) {
	if spawn == nil || len(entities) == 0 {
		return
	}

	for _, entity := range entities {
		key := entity.EntityNum
		base := entityEffectLightOrigin(entity.Origin)
		if entity.Effects&inet.EF_MUZZLEFLASH != 0 {
			spawn(DynamicLight{
				Position:   muzzleFlashLightOrigin(entity),
				Radius:     216,
				Color:      [3]float32{1.0, 0.82, 0.45},
				Brightness: 1.1,
				Lifetime:   0.1,
				EntityKey:  key,
			})
		}
		if entity.Effects&inet.EF_BRIGHTLIGHT != 0 {
			spawn(DynamicLight{
				Position:   base,
				Radius:     416,
				Color:      [3]float32{1.0, 1.0, 0.95},
				Brightness: 1.25,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
		if entity.Effects&inet.EF_DIMLIGHT != 0 {
			// Ironwail/Quake keeps dim lights at the entity origin instead of lifting them by 16 units.
			spawn(DynamicLight{
				Position:   entity.Origin,
				Radius:     216,
				Color:      [3]float32{0.7, 0.8, 1.0},
				Brightness: 0.9,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
		if entity.ModelFlags&model.EFRocket != 0 {
			spawn(DynamicLight{
				Position:   entity.Origin,
				Radius:     200,
				Color:      [3]float32{0.9, 0.6, 0.3},
				Brightness: 1.0,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
		if entity.Effects&inet.EF_QUADLIGHT != 0 {
			spawn(DynamicLight{
				Position:   base,
				Radius:     216,
				Color:      [3]float32{0.25, 0.25, 1.0},
				Brightness: 0.9,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
		if entity.Effects&inet.EF_PENTALIGHT != 0 {
			spawn(DynamicLight{
				Position:   base,
				Radius:     216,
				Color:      [3]float32{1.0, 0.25, 0.25},
				Brightness: 0.9,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
		if entity.Effects&inet.EF_CANDLELIGHT != 0 {
			spawn(DynamicLight{
				Position:   entity.Origin,
				Radius:     160,
				Color:      [3]float32{1.0, 0.75, 0.2},
				Brightness: 0.8,
				Lifetime:   0.001,
				EntityKey:  key,
			})
		}
	}
}

// entityEffectLightOrigin derives a stable light-emission origin for dynamic lights attached to entities.
func entityEffectLightOrigin(origin [3]float32) [3]float32 {
	origin[2] += 16
	return origin
}

// muzzleFlashLightOrigin offsets muzzle-light position toward weapon muzzle so flashes appear physically attached to firing points.
func muzzleFlashLightOrigin(entity EntityEffectSource) [3]float32 {
	origin := entityEffectLightOrigin(entity.Origin)
	forward, _, _ := qtypes.AngleVectors(qtypes.Vec3{
		X: entity.Angles[0],
		Y: entity.Angles[1],
		Z: entity.Angles[2],
	})
	origin[0] += forward.X * 18
	origin[1] += forward.Y * 18
	origin[2] += forward.Z * 18
	return origin
}

// randomMarkRotation picks a random decal rotation to reduce repetition in impact marks.
func randomMarkRotation(rng *rand.Rand) float32 {
	if rng == nil {
		return 0
	}
	return float32(rng.Float64() * 2.0 * math.Pi)
}
