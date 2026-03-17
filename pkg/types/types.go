// =============================================================================
// Quake 3D Vector & Scalar Math
// =============================================================================
//
// This file provides the core mathematical types and operations needed by
// every subsystem in the Quake engine: physics, collision detection (BSP hull
// tracing), rendering, and the network protocol.
//
// ─────────────────────────────────────────────────────────────────────────────
// QUAKE RIGHT-HANDED COORDINATE SYSTEM
// ─────────────────────────────────────────────────────────────────────────────
//
//	Unlike most 3D engines that use Y-up or Z-up with X-right, Quake uses a
//	unique axis assignment inherited from its 2D Wolfenstein 3D predecessor:
//
//	    X = forward  (east on the map)
//	    Y = left     (north on the map)
//	    Z = up       (vertical)
//
//	This is RIGHT-HANDED: cross(X, Y) = Z  (forward × left = up).
//
//	Euler angles follow accordingly:
//	    Pitch (X component of an angle Vec3) rotates around the Y axis (look up/down)
//	    Yaw   (Y component) rotates around the Z axis (turn left/right)
//	    Roll  (Z component) rotates around the X axis (tilt sideways)
//
// ─────────────────────────────────────────────────────────────────────────────
// ANGLE ENCODING FOR THE NETWORK PROTOCOL
// ─────────────────────────────────────────────────────────────────────────────
//
//	Quake's original network protocol encodes angles as a single byte,
//	mapping the 0–255 range linearly to 0°–360°. This gives a resolution
//	of ~1.41° per step, which is precise enough for entity orientation in
//	a fast-paced FPS. The FitzQuake/Ironwail protocol extensions add a
//	16-bit angle encoding for smoother camera interpolation. The functions
//	AngleMod, AngleByte, and ByteToAngle implement these conversions.
//
// ─────────────────────────────────────────────────────────────────────────────
// VEC3 IN THE ENGINE
// ─────────────────────────────────────────────────────────────────────────────
//
//	Vec3 is the most fundamental type in the engine. It represents:
//	  - Entity positions and velocities (physics simulation)
//	  - Surface normals (BSP plane definitions, lighting)
//	  - Movement directions (player input, projectile trajectories)
//	  - Euler angles stored as (pitch, yaw, roll)
//	  - Bounding box mins/maxs (collision hulls)
//
//	The standalone functions (Vec3Add, Vec3Dot, etc.) follow C Quake's
//	procedural style, while the method variants (v.Add, v.Dot) provide
//	an idiomatic Go alternative. Both produce identical results.
//
// =============================================================================
package types

import "math"

// Vec3 represents a 3D vector with X, Y, Z components in Quake's right-handed
// coordinate system (X=forward, Y=left, Z=up).
//
// Vec3 is the foundational type used throughout the engine for positions,
// velocities, normals, and Euler angles. When used as an angle vector, the
// components map to: X=pitch, Y=yaw, Z=roll. All components use float32
// to match C Quake's single-precision math and GPU uniform expectations.
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

// Vec3Add adds two vectors component-wise and returns the result.
//
// This is the workhorse of position updates in the physics system:
//
//	newOrigin = Vec3Add(origin, Vec3Scale(velocity, dt))
//
// It is also used to combine bounding box offsets with entity positions
// for collision hull construction.
func Vec3Add(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X + b.X,
		Y: a.Y + b.Y,
		Z: a.Z + b.Z,
	}
}

// Vec3Sub subtracts two vectors component-wise (a - b) and returns the result.
//
// Commonly used to compute direction vectors between two points (e.g.,
// from a player to a target for projectile aiming, or between two
// entity origins for distance checks in the physics/AI code).
func Vec3Sub(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X - b.X,
		Y: a.Y - b.Y,
		Z: a.Z - b.Z,
	}
}

// Vec3Scale multiplies every component of v by the scalar s.
//
// Used throughout the physics system for velocity damping (friction),
// impulse application, and normalizing vectors to a desired magnitude.
// For example, SV_FlyMove scales the remaining velocity by the fraction
// of a time step not consumed by a collision.
func Vec3Scale(v Vec3, s float32) Vec3 {
	return Vec3{
		X: v.X * s,
		Y: v.Y * s,
		Z: v.Z * s,
	}
}

// Vec3Dot returns the dot product (inner product) of two vectors.
//
// The dot product is the single most important operation in BSP traversal
// and collision detection. Given a BSP splitting plane with normal N and
// distance D, a point P is classified by:
//
//	side = Vec3Dot(P, N) - D
//	  side > 0  →  front (positive half-space)
//	  side < 0  →  back  (negative half-space)
//
// This plane-point test drives the recursive descent through the BSP tree
// for rendering (visibility), hull tracing (collision), and lighting.
// The dot product also measures projection length, making it essential for
// sliding collision response (velocity projected onto a wall normal).
func Vec3Dot(a, b Vec3) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// Vec3Cross returns the cross product of two vectors, producing a vector
// perpendicular to both inputs.
//
// In Quake's right-handed coordinate system, the result follows the
// right-hand rule: if you curl your fingers from a toward b, your thumb
// points in the direction of the result. This operation is used to:
//   - Compute surface normals from triangle edges during BSP compilation
//   - Build orthonormal basis vectors (forward/right/up) from angles
//   - Determine winding order for backface culling in the renderer
func Vec3Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// Vec3Len returns the Euclidean length (magnitude) of a vector:
// sqrt(x² + y² + z²).
//
// Used for distance checks (e.g., "is the player within 100 units of the
// trigger?"), normalization, and physics calculations. Note that when only
// comparing distances, using the squared length (Vec3Dot(v, v)) avoids the
// expensive sqrt — but Quake's original C code uses the full length in
// many places, and this port preserves that behavior.
func Vec3Len(v Vec3) float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Vec3Normalize returns a unit-length vector pointing in the same direction
// as v. If v is the zero vector, it is returned unchanged to avoid division
// by zero.
//
// Normalized vectors are essential for BSP plane normals (which must be
// unit-length for correct distance calculations), direction vectors for
// ray tracing, and surface normals for lighting dot-product shading.
func Vec3Normalize(v Vec3) Vec3 {
	length := Vec3Len(v)
	if length > 0 {
		return Vec3Scale(v, 1.0/length)
	}
	return v
}

// Clamp restricts a float32 value to the closed interval [min, max].
// If value is less than min, returns min.
// If value is greater than max, returns max.
// Otherwise returns value unchanged.
//
// Used pervasively in the engine to enforce safe ranges: limiting view
// angles (e.g., pitch clamped to ±89° to prevent gimbal issues), ensuring
// color components stay in [0,1] before GPU upload, and bounding physics
// values like friction and speed caps.
func Clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt restricts an integer value to the closed interval [min, max].
//
// Integer clamping is used for array index safety (e.g., clamping a
// texture frame index to valid bounds), network protocol field validation,
// and bounding console variable values parsed from user input.
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// AngleMod normalizes an angle to the range [0, 360) by quantizing it
// through the 8-bit byte representation and back.
//
// In Quake's original network protocol, entity angles are transmitted as
// a single byte (0–255 linearly mapping to 0°–360°), giving ~1.41°
// resolution per step. AngleMod simulates this quantization: it converts
// the float angle to byte precision (masking to 0–255) and then back to
// float degrees. This ensures that client-side predicted angles match
// exactly what the server sends over the wire, preventing visual jitter
// from floating-point drift.
func AngleMod(angle float32) float32 {
	return float32(int(angle*float32(256.0/360.0))&255) * float32(360.0/256.0)
}

// AngleByte converts a float angle (in degrees) to the 8-bit byte
// representation used by Quake's network protocol (0–255 maps to 0°–360°).
//
// This encoding is written into network messages by SizeBuf.WriteAngle
// and decoded on the receiving end by ByteToAngle. The & 255 mask handles
// wrap-around for angles outside the 0–360 range (e.g., negative angles
// or angles > 360 from accumulated rotation).
func AngleByte(angle float32) byte {
	return byte(int(angle*float32(256.0/360.0)) & 255)
}

// ByteToAngle converts a byte angle (0–255) from the network protocol back
// to float degrees in the range [0, 360). This is the inverse of AngleByte
// and is called when parsing incoming server messages to reconstruct entity
// orientations on the client side.
func ByteToAngle(b byte) float32 {
	return float32(b) * float32(360.0/256.0)
}

// Pitch, Yaw, and Roll are indices into a Vec3 used as an Euler angle
// triplet. Quake stores angles as Vec3{pitch, yaw, roll} throughout the
// engine — in entity state, client view angles, and network messages.
//
// In the Quake coordinate system:
//   - Pitch (index 0, Vec3.X): Rotation around the Y axis — looking up/down.
//     Positive pitch looks down (nose dips); negative looks up.
//   - Yaw (index 1, Vec3.Y): Rotation around the Z axis — turning left/right.
//     Positive yaw turns left (counter-clockwise when viewed from above).
//   - Roll (index 2, Vec3.Z): Rotation around the X axis — tilting sideways.
//     Rarely used for players but applied to projectiles and death animations.
const (
	Pitch = 0
	Yaw   = 1
	Roll  = 2
)

// Lerp performs linear interpolation between a and b by fraction t ∈ [0, 1].
//
// Returns a when t=0, b when t=1, and a proportional blend for values in
// between. This is the fundamental building block for all smooth transitions
// in the engine: entity position interpolation between server ticks,
// animation frame blending, color fading, and view smoothing. The form
// a + (b-a)*t is numerically stable and produces exact results at t=0 and t=1.
func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// NormalizeAngle wraps an angle in degrees into the range [-180, 180).
//
// This canonical range is needed for shortest-path angle interpolation:
// without normalization, interpolating from 350° to 10° would go the long
// way around (340° of rotation) instead of the short way (20°). By mapping
// both angles into [-180, 180) first, subtraction always gives the shortest
// signed difference.
func NormalizeAngle(degrees float32) float32 {
	degrees += 180.0
	degrees -= float32(math.Floor(float64(degrees*(1.0/360.0)))) * 360.0
	degrees -= 180.0
	return degrees
}

// AngleDifference returns the shortest signed difference between two angles
// (dega - degb), wrapped into [-180, 180).
//
// This ensures that the difference always represents the shortest rotational
// path. For example, AngleDifference(10, 350) returns 20 (not -340),
// indicating a short 20° counter-clockwise turn. Used by the client
// interpolation code to smoothly blend entity orientations between ticks.
func AngleDifference(dega, degb float32) float32 {
	return NormalizeAngle(dega - degb)
}

// LerpAngle performs shortest-path linear interpolation between two angles.
//
// Unlike plain Lerp, this function accounts for the circular nature of
// angles: it always interpolates through the shortest arc. For instance,
// LerpAngle(350, 10, 0.5) returns 0 (midpoint of the 20° arc) rather
// than 180 (midpoint of the 340° arc). This is critical for smooth entity
// rotation interpolation on the client between server update ticks.
func LerpAngle(degfrom, degto, frac float32) float32 {
	return NormalizeAngle(degfrom + AngleDifference(degto, degfrom)*frac)
}

// VectorAngles converts a direction vector into Euler angles (pitch, yaw, 0).
//
// Given a forward direction vector, this computes the pitch (up/down angle)
// and yaw (left/right angle) needed to face that direction. Roll is always
// set to zero because a direction vector alone does not encode roll.
//
// This is used by the server physics code (e.g., SV_AIM) to determine
// the aiming angle from a direction, and by the client for projectile
// orientation. The implementation projects the forward vector onto the
// XY plane to isolate yaw, then uses the Z component vs. horizontal
// length for pitch — matching the original C Quake VectorAngles exactly.
func VectorAngles(forward Vec3) Vec3 {
	var angles Vec3
	// Quake's VectorAngles implementation:
	// angles[PITCH] = -atan2(forward[2], VectorLength(temp)) / M_PI_DIV_180;
	// angles[YAW] = atan2(forward[1], forward[0]) / M_PI_DIV_180;
	temp := Vec3{X: forward.X, Y: forward.Y, Z: 0}
	angles.X = -float32(math.Atan2(float64(forward.Z), float64(Vec3Len(temp)))) * (180.0 / math.Pi)
	angles.Y = float32(math.Atan2(float64(forward.Y), float64(forward.X))) * (180.0 / math.Pi)
	angles.Z = 0
	return angles
}

// AngleVectors calculates forward, right, and up basis vectors from Euler
// angles (pitch, yaw, roll).
//
// This is the inverse of VectorAngles and one of the most important
// functions in the engine. It decomposes an orientation into three
// orthogonal direction vectors that define a local coordinate frame:
//
//   - forward: The direction the entity is facing (used for movement,
//     projectile spawning, and trace line-of-sight checks)
//   - right: Points to the entity's right (used for strafing movement
//     and placing weapon muzzle flash offsets)
//   - up: Points above the entity (used for vertical movement and
//     camera roll effects)
//
// The implementation applies rotations in Quake's YXZ (yaw-pitch-roll)
// order using precomputed sine/cosine values, matching the original C
// AngleVectors from mathlib.c. Note that "right" in Quake points to
// the entity's RIGHT (negative Y direction in world space), which is
// the opposite of the Y-left convention — hence the -1 multipliers.
func AngleVectors(angles Vec3) (forward, right, up Vec3) {
	sy := math.Sin(float64(angles.Y) * (math.Pi * 2 / 360))
	cy := math.Cos(float64(angles.Y) * (math.Pi * 2 / 360))
	sp := math.Sin(float64(angles.X) * (math.Pi * 2 / 360))
	cp := math.Cos(float64(angles.X) * (math.Pi * 2 / 360))
	sr := math.Sin(float64(angles.Z) * (math.Pi * 2 / 360))
	cr := math.Cos(float64(angles.Z) * (math.Pi * 2 / 360))

	forward.X = float32(cp * cy)
	forward.Y = float32(cp * sy)
	forward.Z = float32(-sp)

	right.X = float32(-1*sr*sp*cy + -1*cr*-sy)
	right.Y = float32(-1*sr*sp*sy + -1*cr*cy)
	right.Z = float32(-1 * sr * cp)

	up.X = float32(cr*sp*cy + -sr*-sy)
	up.Y = float32(cr*sp*sy + -sr*cy)
	up.Z = float32(cr * cp)
	return
}

// QRint rounds a float32 to the nearest integer using Quake's rounding
// convention (round half away from zero).
//
// This matches the original C Q_rint macro used throughout the engine
// for pixel-snapping coordinates, lightmap texel alignment, and
// integer conversions in the console variable system. Go's built-in
// rounding (math.Round) uses banker's rounding (round half to even),
// which would produce different results in edge cases, so we preserve
// Quake's behavior explicitly.
func QRint(x float32) int {
	if x > 0 {
		return int(x + 0.5)
	}
	return int(x - 0.5)
}

// QLog2 returns the integer base-2 logarithm of val (i.e., the position
// of the highest set bit, or equivalently floor(log2(val)) for val > 0).
//
// This is used by the texture system to determine mipmap levels: a 256×256
// texture has QLog2(256) = 8 mipmap levels. It is also used to validate
// that texture dimensions are powers of two (required by classic Quake's
// software and OpenGL renderers) and to compute subdivision levels.
func QLog2(val int) int {
	answer := 0
	for val > 1 {
		val >>= 1
		answer++
	}
	return answer
}

// QNextPow2 returns the smallest power of 2 that is ≥ val.
//
// OpenGL (especially older versions) requires texture dimensions to be
// powers of two. When loading a texture with non-power-of-two dimensions,
// the renderer uses QNextPow2 to determine the padded texture size.
// The implementation uses the classic bit-manipulation trick: decrement,
// smear the highest bit downward through successive OR-shifts, then
// increment. This runs in O(1) time with no loops or branches (aside
// from the initial zero check).
func QNextPow2(val int) int {
	if val <= 0 {
		return 1
	}
	val--
	val |= val >> 1
	val |= val >> 2
	val |= val >> 4
	val |= val >> 8
	val |= val >> 16
	val++
	return val
}

// Vec3MA performs the fused Multiply-Add operation: result = veca + scale*vecb.
//
// This combined operation is one of the most frequently called functions in
// the physics code. It computes a position displaced along a direction:
//
//	endPoint = startPoint + distance * direction
//
// Typical uses include:
//   - Advancing a trace ray: end = start + traceLen * traceDir
//   - Applying velocity: newPos = oldPos + dt * velocity
//   - Offsetting a point along a plane normal: pushed = origin + epsilon * normal
//
// The name "MA" comes from the original C macro VectorMA (Vector Multiply-Add).
func Vec3MA(veca Vec3, scale float32, vecb Vec3) Vec3 {
	return Vec3{
		X: veca.X + scale*vecb.X,
		Y: veca.Y + scale*vecb.Y,
		Z: veca.Z + scale*vecb.Z,
	}
}

// Vec3Lerp performs component-wise linear interpolation between two vectors.
//
// Returns veca when frac=0, vecb when frac=1, and a proportional blend in
// between. This is the primary mechanism for smooth entity movement on the
// client: the server sends position updates at a fixed tick rate (typically
// 72 Hz), and the client interpolates between the two most recent positions
// at the display frame rate (e.g., 144+ Hz) for visually smooth motion.
func Vec3Lerp(veca, vecb Vec3, frac float32) Vec3 {
	return Vec3{
		X: Lerp(veca.X, vecb.X, frac),
		Y: Lerp(veca.Y, vecb.Y, frac),
		Z: Lerp(veca.Z, vecb.Z, frac),
	}
}

// NewVec3 creates a Vec3 from individual float32 components.
// This is a convenience constructor; using a struct literal Vec3{X: x, Y: y, Z: z}
// is equivalent but more verbose.
func NewVec3(x, y, z float32) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// Sub returns v - other. Method form of Vec3Sub for fluent chaining:
//
//	dir := target.Sub(origin).Normalize()
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3Sub(v, other)
}

// Add returns v + other. Method form of Vec3Add for fluent chaining.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3Add(v, other)
}

// Scale returns v * s. Method form of Vec3Scale for fluent chaining.
func (v Vec3) Scale(s float32) Vec3 {
	return Vec3Scale(v, s)
}

// Dot returns the dot product of v and other. Method form of Vec3Dot.
func (v Vec3) Dot(other Vec3) float32 {
	return Vec3Dot(v, other)
}

// Cross returns the cross product of v and other. Method form of Vec3Cross.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3Cross(v, other)
}

// Len returns the Euclidean length (magnitude) of the vector.
// Method form of Vec3Len.
func (v Vec3) Len() float32 {
	return Vec3Len(v)
}

// Normalize returns a unit-length vector in the same direction.
// Method form of Vec3Normalize.
func (v Vec3) Normalize() Vec3 {
	return Vec3Normalize(v)
}
