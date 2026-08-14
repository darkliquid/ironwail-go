// =============================================================================
// Euler Angles (Angles)
// =============================================================================
package types

// Angles represents orientation in 3D space using Euler angles (degrees)
// in Quake's right-handed coordinate convention:
//   - Pitch (X): Rotation around Y axis (look down > 0, look up < 0)
//   - Yaw   (Y): Rotation around Z axis (turn left > 0, turn right < 0)
//   - Roll  (Z): Rotation around X axis (tilt right/left)
type Angles struct {
	Pitch float32
	Yaw   float32
	Roll  float32
}

// NewAngles creates an Angles value from individual degree components.
func NewAngles(pitch, yaw, roll float32) Angles {
	return Angles{
		Pitch: pitch,
		Yaw:   yaw,
		Roll:  roll,
	}
}

// AnglesFromVec3 constructs Angles from a Vec3 where X=Pitch, Y=Yaw, Z=Roll.
func AnglesFromVec3(v Vec3) Angles {
	return Angles{
		Pitch: v.X,
		Yaw:   v.Y,
		Roll:  v.Z,
	}
}

// ToVec3 converts Angles to a Vec3 with X=Pitch, Y=Yaw, Z=Roll.
func (a Angles) ToVec3() Vec3 {
	return Vec3{
		X: a.Pitch,
		Y: a.Yaw,
		Z: a.Roll,
	}
}

// Normalize wraps all angle components into standard ranges:
// Pitch into [-180, 180), Yaw into [0, 360), Roll into [-180, 180).
func (a Angles) Normalize() Angles {
	return Angles{
		Pitch: NormalizeAngle(a.Pitch),
		Yaw:   NormalizeAngle(a.Yaw),
		Roll:  NormalizeAngle(a.Roll),
	}
}

// Difference returns the shortest rotational difference (a - other) per axis.
func (a Angles) Difference(other Angles) Angles {
	return Angles{
		Pitch: AngleDifference(a.Pitch, other.Pitch),
		Yaw:   AngleDifference(a.Yaw, other.Yaw),
		Roll:  AngleDifference(a.Roll, other.Roll),
	}
}

// Lerp interpolates smoothly between two angle orientations via shortest arc.
func (a Angles) Lerp(target Angles, t float32) Angles {
	return Angles{
		Pitch: LerpAngle(a.Pitch, target.Pitch, t),
		Yaw:   LerpAngle(a.Yaw, target.Yaw, t),
		Roll:  LerpAngle(a.Roll, target.Roll, t),
	}
}

// Vectors decomposes the Euler angles into orthonormal basis vectors:
// forward, right (positive local right), and up.
func (a Angles) Vectors() (forward, right, up Vec3) {
	return AngleVectors(a.ToVec3())
}

// Forward computes only the unit forward direction vector.
func (a Angles) Forward() Vec3 {
	fwd, _, _ := a.Vectors()
	return fwd
}

// Right computes only the unit right direction vector.
func (a Angles) Right() Vec3 {
	_, right, _ := a.Vectors()
	return right
}

// Up computes only the unit up direction vector.
func (a Angles) Up() Vec3 {
	_, _, up := a.Vectors()
	return up
}
