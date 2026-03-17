package types

import "math"

// Vec3 represents a 3D vector with X, Y, Z components.
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

// Vec3Add adds two vectors component-wise.
func Vec3Add(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X + b.X,
		Y: a.Y + b.Y,
		Z: a.Z + b.Z,
	}
}

// Vec3Sub subtracts two vectors component-wise.
func Vec3Sub(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X - b.X,
		Y: a.Y - b.Y,
		Z: a.Z - b.Z,
	}
}

// Vec3Scale scales a vector by a scalar.
func Vec3Scale(v Vec3, s float32) Vec3 {
	return Vec3{
		X: v.X * s,
		Y: v.Y * s,
		Z: v.Z * s,
	}
}

// Vec3Dot returns the dot product of two vectors.
func Vec3Dot(a, b Vec3) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// Vec3Cross returns the cross product of two vectors.
func Vec3Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// Vec3Len returns the length (magnitude) of a vector.
func Vec3Len(v Vec3) float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Vec3Normalize normalizes a vector to unit length.
func Vec3Normalize(v Vec3) Vec3 {
	length := Vec3Len(v)
	if length > 0 {
		return Vec3Scale(v, 1.0/length)
	}
	return v
}

// Clamp restricts a value to be within the range [min, max].
// If value is less than min, returns min.
// If value is greater than max, returns max.
// Otherwise returns value unchanged.
func Clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt restricts an integer value to be within the range [min, max].
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// AngleMod normalizes an angle to the range [0, 360).
// In Quake, angles are stored as bytes (0-255 representing 0-360 degrees).
// This function handles both float angles and the byte representation.
func AngleMod(angle float32) float32 {
	return float32(int(angle*float32(256.0/360.0))&255) * float32(360.0/256.0)
}

// AngleByte converts a float angle to byte representation (0-255).
func AngleByte(angle float32) byte {
	return byte(int(angle*float32(256.0/360.0)) & 255)
}

// ByteToAngle converts a byte angle (0-255) to float degrees (0-360).
func ByteToAngle(b byte) float32 {
	return float32(b) * float32(360.0/256.0)
}

// Constants for angle indices.
const (
	Pitch = 0
	Yaw   = 1
	Roll  = 2
)

// Lerp performs linear interpolation between a and b by t.
func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// NormalizeAngle returns an angle in the range [-180, 180).
func NormalizeAngle(degrees float32) float32 {
	degrees += 180.0
	degrees -= float32(math.Floor(float64(degrees*(1.0/360.0)))) * 360.0
	degrees -= 180.0
	return degrees
}

// AngleDifference returns the shortest difference between two angles in [-180, 180).
func AngleDifference(dega, degb float32) float32 {
	return NormalizeAngle(dega - degb)
}

// LerpAngle performs linear interpolation between two angles.
func LerpAngle(degfrom, degto, frac float32) float32 {
	return NormalizeAngle(degfrom + AngleDifference(degto, degfrom)*frac)
}

// VectorAngles calculates pitch and yaw from a forward vector.
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

// AngleVectors calculates forward, right, and up vectors from angles.
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

// QRint rounds a float to the nearest integer.
func QRint(x float32) int {
	if x > 0 {
		return int(x + 0.5)
	}
	return int(x - 0.5)
}

// QLog2 returns the base-2 logarithm of an integer.
func QLog2(val int) int {
	answer := 0
	for val > 1 {
		val >>= 1
		answer++
	}
	return answer
}

// QNextPow2 returns the next power of 2 greater than or equal to val.
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

// Vec3MA performs Multiply-Add: dst = veca + scale*vecb.
func Vec3MA(veca Vec3, scale float32, vecb Vec3) Vec3 {
	return Vec3{
		X: veca.X + scale*vecb.X,
		Y: veca.Y + scale*vecb.Y,
		Z: veca.Z + scale*vecb.Z,
	}
}

// Vec3Lerp performs linear interpolation between two vectors.
func Vec3Lerp(veca, vecb Vec3, frac float32) Vec3 {
	return Vec3{
		X: Lerp(veca.X, vecb.X, frac),
		Y: Lerp(veca.Y, vecb.Y, frac),
		Z: Lerp(veca.Z, vecb.Z, frac),
	}
}

// NewVec3 creates a Vec3 from individual components.
func NewVec3(x, y, z float32) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// Sub returns v - other.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3Sub(v, other)
}

// Add returns v + other.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3Add(v, other)
}

// Scale returns v * s.
func (v Vec3) Scale(s float32) Vec3 {
	return Vec3Scale(v, s)
}

// Dot returns the dot product of v and other.
func (v Vec3) Dot(other Vec3) float32 {
	return Vec3Dot(v, other)
}

// Cross returns the cross product of v and other.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3Cross(v, other)
}

// Len returns the length (magnitude) of the vector.
func (v Vec3) Len() float32 {
	return Vec3Len(v)
}

// Normalize returns a unit-length vector in the same direction.
func (v Vec3) Normalize() Vec3 {
	return Vec3Normalize(v)
}
