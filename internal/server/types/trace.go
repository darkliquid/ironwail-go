// This file belongs to the Physics/Collision subsystem: physics constants and vector math helpers.
package types

import "math"

// Physics constants used in collision detection and movement clipping.
const (
	// MoveEpsilon is the minimum movement distance threshold. Movements smaller
	// than this are discarded to prevent infinite loops in the clipping code.
	// When a trace returns a fraction that would result in less than MoveEpsilon
	// units of movement, the entity is considered "stuck" and movement stops.
	// This prevents floating-point precision issues from causing entities to
	// jitter or slide infinitely along surfaces.
	MoveEpsilon = 0.01

	// StopEpsilon is the velocity threshold below which an entity is considered
	// stopped. When an entity's velocity component drops below StopEpsilon, it
	// is snapped to zero. This prevents entities from slowly drifting forever
	// due to floating-point accumulation in friction calculations. Applied
	// independently to each axis (X, Y, Z) of the velocity vector.
	StopEpsilon = 0.1
)

// Default physics/sound constants used when entity-specific values are not set.
const (
	// DefaultSoundVolume — default volume for sound playback (0-255 scale).
	// 255 is maximum volume. QuakeC can specify lower volumes for quieter
	// effects, but most sounds use this default (full volume at the source,
	// attenuated by distance).
	DefaultSoundVolume = 255

	// DefaultSoundAttenuation — default distance falloff for sounds. Controls
	// how quickly a sound fades with distance from the listener. Standard
	// attenuation values:
	//  0.0 = no attenuation (plays at equal volume everywhere, like music)
	//  1.0 = normal (standard falloff, good for most game sounds)
	//  2.0 = idle (shorter range, for ambient/idle sounds)
	//  3.0 = static (very short range, for point-source effects like torches)
	DefaultSoundAttenuation = 1.0

	// ViewHeight — default eye-level offset from the entity's origin, in
	// Quake units. The player's origin is at their feet; the camera is
	// ViewHeight (22) units above. This produces a camera height of roughly
	// eye-level for Quake's player model. QuakeC can override this via
	// the ViewOfs field for crouching or special camera effects.
	ViewHeight = 22

	// OneEpsilon — general-purpose small epsilon for floating-point
	// comparisons. Matches C ON_EPSILON (0.1) from quakedef.h, used in
	// movement stair-stepping and plane distance checks.
	OneEpsilon = 0.1
)

// Vector math helper functions.
//
// These operate on [3]float32 vectors representing 3D positions, velocities,
// normals, and directions in Quake's coordinate system:
//   - X axis: east (+) / west (-)
//   - Y axis: north (+) / south (-)
//   - Z axis: up (+) / down (-)
//
// Quake uses a right-handed coordinate system with Z-up, which differs from
// some 3D engines that use Y-up. All positions are in "Quake units" where
// 1 unit ≈ 1 inch (a player is about 56 units tall, door frames are 128 units).

// VecAdd returns the component-wise sum of two vectors: result[i] = a[i] + b[i].
//
// Physics use: computing new positions from current position + velocity * dt,
// combining force vectors, offsetting positions (e.g., origin + view offset).
func VecAdd(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// VecSub returns the component-wise difference of two vectors: result[i] = a[i] - b[i].
//
// Physics use: computing displacement vectors between entities (target - self),
// finding velocity change (new_velocity - old_velocity), and computing relative
// positions for distance/direction calculations.
func VecSub(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

// VecScale returns a vector scaled by a scalar: result[i] = v[i] * s.
//
// Physics use: applying frametime to velocity (velocity * dt), scaling normals
// for backoff calculations, applying friction multipliers, and attenuating
// forces. For example, gravity application: velocity[2] -= sv_gravity * dt
// is equivalent to VecAdd(velocity, VecScale({0,0,-1}, gravity * dt)).
func VecScale(v [3]float32, s float32) [3]float32 {
	return [3]float32{v[0] * s, v[1] * s, v[2] * s}
}

// VecLen returns the Euclidean length (magnitude) of a vector:
// sqrt(v[0]² + v[1]² + v[2]²).
//
// Physics use: computing entity speed (length of velocity vector), measuring
// distances between points (len(VecSub(a, b))), and as a precursor to
// normalization. Note: uses float64 math internally for precision, matching
// the original C engine's use of double-precision sqrt.
func VecLen(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// VecNormalize normalizes a vector in-place to unit length and returns its
// original length. If the vector has zero length, it remains unchanged.
//
// Physics use: converting displacement vectors to direction vectors for
// AI aiming (aim_direction = normalize(target - self)), computing surface
// normals, and preparing vectors for dot product angle calculations.
// The returned length is useful for simultaneous distance+direction queries
// (e.g., "how far is the enemy and which direction?").
func VecNormalize(v *[3]float32) float32 {
	length := VecLen(*v)
	if length > 0 {
		v[0] /= length
		v[1] /= length
		v[2] /= length
	}
	return length
}

// VecDot returns the dot product of two vectors: a[0]*b[0] + a[1]*b[1] + a[2]*b[2].
//
// Physics use: the dot product is fundamental to collision response and physics.
//   - Dot(velocity, plane_normal) gives the speed of impact against a surface.
//     Used in ClipVelocity to compute the velocity component to remove.
//   - Dot(direction, normal) gives cos(angle) between them. Used for line-of-sight
//     angle checks (is the player in the monster's field of view?).
//   - Dot(point - plane_point, plane_normal) gives signed distance from a plane.
//     Used extensively in BSP traversal and collision detection.
func VecDot(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// VecCopy copies the source vector's components into the destination vector.
//
// Physics use: saving entity positions before movement (for rollback on
// collision), copying positions between entity fields (e.g., OldOrigin = Origin
// before physics runs), and initializing trace result positions.
func VecCopy(src [3]float32, dst *[3]float32) {
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
}

// VecCross returns the cross product of two vectors, producing a vector
// perpendicular to both inputs. The result follows the right-hand rule.
//
// Physics use: computing surface normals from two edge vectors (for dynamic
// geometry), calculating torque or rotational forces, and finding perpendicular
// directions. Less commonly used in Quake's physics than VecDot, but essential
// for certain geometric operations like building coordinate frames from angles.
func VecCross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
