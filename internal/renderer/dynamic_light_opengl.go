//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"math"
)

// DynamicLight represents a temporary point light source in the world.
// Dynamic lights fade over time and are used for torches, explosions, muzzle flashes, etc.
type DynamicLight struct {
	// Position is the light source center in world space [X, Y, Z]
	Position [3]float32

	// Radius is the distance at which light falloff reaches zero
	Radius float32

	// Color is the RGB light color in linear space [0-1]
	Color [3]float32

	// Brightness is the light intensity multiplier, typically 1.0-2.0
	Brightness float32

	// Lifetime is the total lifespan of the light in seconds
	Lifetime float32

	// Age is the current age of the light in seconds (incremented per frame)
	Age float32

	// Type identifies the light source (torch, muzzle, explosion, etc) for optional filtering
	Type int
}

// IsAlive returns true if the light has not yet expired
func (l *DynamicLight) IsAlive() bool {
	return l.Age < l.Lifetime
}

// FadeMultiplier returns a brightness fade factor as the light ages (optional).
// For now, we use linear fade: 1.0 at start, 0.0 at lifetime.
func (l *DynamicLight) FadeMultiplier() float32 {
	if l.Lifetime <= 0 {
		return 1.0
	}
	remaining := 1.0 - (l.Age / l.Lifetime)
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

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
func evalLightContribution(light *DynamicLight, point [3]float32) [3]float32 {
	dx := light.Position[0] - point[0]
	dy := light.Position[1] - point[1]
	dz := light.Position[2] - point[2]

	distSq := dx*dx + dy*dy + dz*dz
	radiusSq := light.Radius * light.Radius

	// If point is outside light radius, no contribution
	if distSq > radiusSq {
		return [3]float32{0, 0, 0}
	}

	// Compute distance and linear falloff: 1.0 - (distance / radius)
	dist := float32(math.Sqrt(float64(distSq)))
	falloff := 1.0 - (dist / light.Radius)

	// Apply brightness and fade multiplier
	mul := light.Brightness * falloff * light.FadeMultiplier()

	return [3]float32{
		light.Color[0] * mul,
		light.Color[1] * mul,
		light.Color[2] * mul,
	}
}

// EvaluateLightsAtPoint computes the sum of light contributions from all active lights
// at a specific point in world space.
// The result is clamped to reasonable bounds to prevent overexposure.
func (pool *glLightPool) EvaluateLightsAtPoint(point [3]float32) [3]float32 {
	result := [3]float32{0, 0, 0}
	for i := range pool.lights {
		contrib := evalLightContribution(&pool.lights[i], point)
		result[0] += contrib[0]
		result[1] += contrib[1]
		result[2] += contrib[2]
	}
	return result
}

// AccumLightsPerFace pre-computes light contributions for all world faces.
// This is called once per frame before rendering to compute the lighting contribution
// for each face's center point.
type FaceLightAccum struct {
	FaceIndex int
	Light     [3]float32
}

// AccumLightsPerFace computes light contributions for a list of world faces.
// Returns a slice with one entry per input face, containing the summed light contribution
// at that face's center.
func (pool *glLightPool) AccumLightsPerFace(faces []WorldFace) []FaceLightAccum {
	accum := make([]FaceLightAccum, len(faces))
	for i := range faces {
		accum[i].FaceIndex = i
		accum[i].Light = pool.EvaluateLightsAtPoint(faces[i].Center)
	}
	return accum
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
func ApplyLightToColor(baseColor [3]float32, lightContrib [3]float32, mode LightContribType) [3]float32 {
	switch mode {
	case LightModeAdditive:
		// Simply add the light contribution
		result := [3]float32{
			baseColor[0] + lightContrib[0],
			baseColor[1] + lightContrib[1],
			baseColor[2] + lightContrib[2],
		}
		// Clamp to [0, 1]
		for i := range result {
			if result[i] < 0 {
				result[i] = 0
			} else if result[i] > 1 {
				result[i] = 1
			}
		}
		return result

	case LightModeModulate:
		// Light acts as a multiplier on top of base color (like lighting)
		return [3]float32{
			baseColor[0] * (1.0 + lightContrib[0]),
			baseColor[1] * (1.0 + lightContrib[1]),
			baseColor[2] * (1.0 + lightContrib[2]),
		}

	case LightModeBlend:
		// Lerp from base towards light color
		alpha := (lightContrib[0] + lightContrib[1] + lightContrib[2]) / 3.0
		return [3]float32{
			baseColor[0]*(1.0-alpha) + lightContrib[0]*alpha,
			baseColor[1]*(1.0-alpha) + lightContrib[1]*alpha,
			baseColor[2]*(1.0-alpha) + lightContrib[2]*alpha,
		}

	default:
		return baseColor
	}
}
