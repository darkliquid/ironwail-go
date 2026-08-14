package renderer

import (
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// glLightPool manages the active set of dynamic lights for the current frame.
// It handles light spawning, aging, expiration, and evaluation.
type glLightPool struct {
	lights    []DynamicLight
	maxLights int
}

// NewGLLightPool creates a new light pool with the specified capacity.
// A typical value is 512 lights for reasonable GPU performance.
func NewGLLightPool(maxLights int) *glLightPool {
	if maxLights <= 0 {
		maxLights = 512
	}
	return &glLightPool{
		lights:    make([]DynamicLight, 0, maxLights),
		maxLights: maxLights,
	}
}

// SpawnLight adds a new dynamic light to the active pool.
// If the pool is at capacity, the light is not added (first-come, first-served).
// Returns true if the light was added, false if the pool is full.
func (pool *glLightPool) SpawnLight(light DynamicLight) bool {
	if len(pool.lights) >= pool.maxLights {
		return false
	}
	pool.lights = append(pool.lights, light)
	return true
}

// SpawnOrReplaceKeyed adds a keyed dynamic light, replacing any existing light
// with the same EntityKey. This matches C's CL_AllocDlight(key) behavior where
// a muzzle flash or effect light reuses the same slot for the same entity each frame.
// If key is 0 (no key), falls back to SpawnLight.
func (pool *glLightPool) SpawnOrReplaceKeyed(light DynamicLight) bool {
	if light.EntityKey == 0 {
		return pool.SpawnLight(light)
	}
	for i := range pool.lights {
		if pool.lights[i].EntityKey == light.EntityKey {
			pool.lights[i] = light
			return true
		}
	}
	return pool.SpawnLight(light)
}

// UpdateAndFilter advances all lights' ages and removes expired lights.
// This should be called once per frame before light evaluation.
func (pool *glLightPool) UpdateAndFilter(deltaTime float32) {
	alive := 0
	for i := 0; i < len(pool.lights); i++ {
		pool.lights[i].Age += deltaTime
		if pool.lights[i].IsAlive() {
			pool.lights[alive] = pool.lights[i]
			alive++
		}
	}
	pool.lights = pool.lights[:alive]
}

// Clear removes all lights from the pool.
func (pool *glLightPool) Clear() {
	pool.lights = pool.lights[:0]
}

// ActiveCount returns the number of currently active lights.
func (pool *glLightPool) ActiveCount() int {
	return len(pool.lights)
}

// ActiveLights returns a slice of currently active lights (read-only).
func (pool *glLightPool) ActiveLights() []DynamicLight {
	return pool.lights
}

// evalLightContribution computes the light contribution from a single light source
// to a point in world space.
// The contribution is computed as:
//   - distance = distance from light position to point
//   - if distance > radius: contribution is zero
//   - otherwise: falloff = (1.0 - distance/radius) * brightness * fadeMultiplier
//   - result = light.Color * falloff
func evalLightContribution(light *DynamicLight, point types.Vec3) types.Vec3 {
	d := light.Position.Sub(point)
	distSq := d.X*d.X + d.Y*d.Y + d.Z*d.Z
	radiusSq := light.Radius * light.Radius

	// If point is outside light radius, no contribution
	if distSq > radiusSq {
		return types.Vec3{}
	}

	// Compute distance and linear falloff: 1.0 - (distance / radius)
	dist := float32(math.Sqrt(float64(distSq)))
	falloff := 1.0 - (dist / light.Radius)

	// Apply brightness and fade multiplier
	mul := light.Brightness * falloff * light.FadeMultiplier()

	return light.Color.Scale(mul)
}

// EvaluateLightsAtPoint computes the sum of light contributions from all active lights
// at a specific point in world space.
// The result is clamped to reasonable bounds to prevent overexposure.
func (pool *glLightPool) EvaluateLightsAtPoint(point types.Vec3) types.Vec3 {
	return evaluateDynamicLightsAtPoint(pool.lights, point)
}

func evaluateDynamicLightsAtPoint(lights []DynamicLight, point types.Vec3) types.Vec3 {
	if pkgCVars != nil {
		if cv := pkgCVars.Get(CvarRDynamic); cv != nil && cv.Int == 0 {
			return types.Vec3{}
		}
	}
	result := types.Vec3{}
	for i := range lights {
		contrib := evalLightContribution(&lights[i], point)
		result = result.Add(contrib)
	}
	return result
}

// LightContribType specifies how light contributions are applied to face colors
type LightContribType int

const (
	// LightModeAdditive adds the light contribution directly to the base color
	LightModeAdditive LightContribType = iota
	// LightModeModulate multiplies the light contribution with the base color
	LightModeModulate
	// LightModeBlend lerps between base color and light color
	LightModeBlend
)

// ApplyLightToColor applies a light contribution to a base RGB color.
// This function respects the light application mode and clamps the result.
func ApplyLightToColor(baseColor types.Vec3, lightContrib types.Vec3, mode LightContribType) types.Vec3 {
	switch mode {
	case LightModeAdditive:
		// Simply add the light contribution
		result := baseColor.Add(lightContrib)
		// Clamp to [0, 1]
		if result.X < 0 {
			result.X = 0
		} else if result.X > 1 {
			result.X = 1
		}
		if result.Y < 0 {
			result.Y = 0
		} else if result.Y > 1 {
			result.Y = 1
		}
		if result.Z < 0 {
			result.Z = 0
		} else if result.Z > 1 {
			result.Z = 1
		}
		return result

	case LightModeModulate:
		// Light acts as a multiplier on top of base color (like lighting)
		return types.Vec3{
			X: baseColor.X * (1.0 + lightContrib.X),
			Y: baseColor.Y * (1.0 + lightContrib.Y),
			Z: baseColor.Z * (1.0 + lightContrib.Z),
		}

	case LightModeBlend:
		// Lerp from base towards light color
		alpha := (lightContrib.X + lightContrib.Y + lightContrib.Z) / 3.0
		return types.Vec3{
			X: baseColor.X*(1.0-alpha) + lightContrib.X*alpha,
			Y: baseColor.Y*(1.0-alpha) + lightContrib.Y*alpha,
			Z: baseColor.Z*(1.0-alpha) + lightContrib.Z*alpha,
		}

	default:
		return baseColor
	}
}
