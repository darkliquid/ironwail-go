package types

// =============================================================================
// Quake / Ironwail Matrix & Vector Math
// =============================================================================
//
// This file ports the 4×4 matrix functions from C Ironwail's mathlib.c and
// gl_rmain.c into idiomatic Go. Every function preserves the original
// semantics — column-major storage, right-handed Quake coordinate system,
// and the same projection-matrix trick that bakes in the Quake→clip-space
// axis conversion.
//
// ─────────────────────────────────────────────────────────────────────────────
// QUAKE COORDINATE SYSTEM
// ─────────────────────────────────────────────────────────────────────────────
//
//   Axis assignments:
//       X = forward  (east)
//       Y = left     (north)
//       Z = up
//
//   Euler angle mapping:
//       Yaw   rotates around Z  (turn left/right)
//       Pitch rotates around Y  (look up/down)
//       Roll  rotates around X  (tilt sideways)
//
//   This is a RIGHT-HANDED coordinate system:
//       cross(X, Y) = Z   (forward × left = up)
//
// ─────────────────────────────────────────────────────────────────────────────
// OPENGL / GMATH CONVENTION (for reference)
// ─────────────────────────────────────────────────────────────────────────────
//
//   The "textbook" OpenGL convention is:
//       X = right,  Y = up,  -Z = forward
//
//   This is also right-handed, but with different axis assignments. Standard
//   OpenGL tutorials assume the view matrix already converted world
//   coordinates into this eye-space convention.
//
//   C Ironwail does NOT do that. Instead, the view matrix stays in Quake
//   world space, and the projection matrix handles the conversion.
//
// ─────────────────────────────────────────────────────────────────────────────
// HOW THE VIEW MATRIX WORKS
// ─────────────────────────────────────────────────────────────────────────────
//
//   In C Ironwail (R_SetupGL in gl_rmain.c), the view matrix is built
//   purely from Euler rotations in Quake world space:
//
//       View = Rx(-roll) · Ry(-pitch) · Rz(-yaw) · T(-origin)
//
//   Each rotation matrix is a standard right-hand rotation around one axis.
//   The angles are negated because we are rotating the *world* into the
//   camera's frame (inverse of the camera's own orientation).
//
//   There is NO coordinate-system remapping in the view matrix — the output
//   is still in Quake coordinates (X-forward, Y-left, Z-up).
//
// ─────────────────────────────────────────────────────────────────────────────
// HOW THE PROJECTION MATRIX WORKS
// ─────────────────────────────────────────────────────────────────────────────
//
//   The projection matrix (GL_FrustumMatrix) bakes in the Quake→clip-space
//   axis conversion so the GPU's perspective divide and clipping work
//   correctly:
//
//       clip X  =  -w · quakeY      (left → right flip: Quake Y=left becomes screen +X=right)
//       clip Y  =   h · quakeZ      (up stays up: Quake Z=up becomes screen Y=up)
//       clip Z  =  depth from quakeX (standard depth encoding from the forward axis)
//       clip W  =   quakeX          (forward becomes the perspective-divide denominator)
//
//   where w = 1/tan(fovx/2) and h = 1/tan(fovy/2).
//
//   This is different from a standard OpenGL perspective projection, which
//   assumes the view matrix already converted to eye space (X=right, Y=up,
//   -Z=forward). Here, all the remapping is in one place — the projection
//   matrix — keeping the view matrix simple.
//
// =============================================================================

import (
	"encoding/binary"
	"math"
)

// ---------------------------------------------------------------------------
// Mat4
// ---------------------------------------------------------------------------

// Mat4 is a 4×4 matrix stored in COLUMN-MAJOR order, matching C Ironwail
// and OpenGL conventions.
//
// Element access: matrix[col*4 + row]
//
// Memory layout:
//
//	Index:   0  1  2  3 | 4  5  6  7 | 8  9 10 11 | 12 13 14 15
//	Meaning: ----col 0--- ---col 1--- ---col 2--- ----col 3----
//
// So the mathematical matrix:
//
//	| m00  m01  m02  m03 |     stored as  [m00, m10, m20, m30,   ← col 0
//	| m10  m11  m12  m13 |                 m01, m11, m21, m31,   ← col 1
//	| m20  m21  m22  m23 |                 m02, m12, m22, m32,   ← col 2
//	| m30  m31  m32  m33 |                 m03, m13, m23, m33]   ← col 3
//
// This means matrix[col*4+row] gives you row `row`, column `col`.
type Mat4 [16]float32

// ---------------------------------------------------------------------------
// Vec4
// ---------------------------------------------------------------------------

// Vec4 represents a 4-component vector used for homogeneous coordinates.
//
// In 3D graphics, homogeneous coordinates extend a 3D point (x, y, z) to
// 4D by adding a W component. This allows both translation and projection
// to be expressed as matrix multiplication:
//
//   - For positions: W = 1.0 (translation applies: the point is at x/w, y/w, z/w)
//   - For directions: W = 0.0 (translation has no effect, only rotation applies)
//   - After projection: W = distance along the view axis (used for perspective divide)
//
// The GPU's rasterizer performs the "perspective divide" (x/w, y/w, z/w)
// automatically after the vertex shader, which is why the Quake projection
// matrix ([FrustumMatrix]) places the forward-axis distance into W.
type Vec4 struct {
	X, Y, Z, W float32
}

// ---------------------------------------------------------------------------
// Color
// ---------------------------------------------------------------------------

// Color represents an RGBA color with floating-point components, typically
// in the range [0.0, 1.0].
//
// The engine uses float colors internally for lighting calculations (which
// can temporarily exceed 1.0 for overbright effects) and clamps to [0,1]
// only when uploading to GPU textures or uniform buffers. The A (alpha)
// component controls transparency for blended surfaces like water, teleporters,
// and particle effects.
type Color struct {
	R, G, B, A float32
}

// ---------------------------------------------------------------------------
// IdentityMatrix
// ---------------------------------------------------------------------------

// IdentityMatrix returns the 4×4 identity matrix.
//
// Ported from C Ironwail mathlib.c IdentityMatrix:
//
//	void IdentityMatrix(float matrix[16])
//	{
//	    memset(matrix, 0, 16 * sizeof(float));
//	    matrix[0*4 + 0] = 1.0f;
//	    matrix[1*4 + 1] = 1.0f;
//	    matrix[2*4 + 2] = 1.0f;
//	    matrix[3*4 + 3] = 1.0f;
//	}
func IdentityMatrix() Mat4 {
	var m Mat4
	m[0*4+0] = 1.0
	m[1*4+1] = 1.0
	m[2*4+2] = 1.0
	m[3*4+3] = 1.0
	return m
}

// ---------------------------------------------------------------------------
// RotationMatrix
// ---------------------------------------------------------------------------

// RotationMatrix returns a rotation matrix for the given angle (in RADIANS)
// around the specified axis: 0 = X, 1 = Y, 2 = Z.
//
// Ported from C Ironwail mathlib.c RotationMatrix:
//
//	void RotationMatrix(float matrix[16], float angle, int axis)
//	{
//	    const float c = cosf(angle);
//	    const float s = sinf(angle);
//	    int i = (axis + 1) % 3;
//	    int j = (axis + 2) % 3;
//	    IdentityMatrix(matrix);
//	    matrix[i*4 + i] = c;
//	    matrix[j*4 + j] = c;
//	    matrix[j*4 + i] = -s;
//	    matrix[i*4 + j] = s;
//	}
//
// The rotation follows the right-hand rule: positive angle rotates
// counter-clockwise when looking down the axis toward the origin.
func RotationMatrix(angle float32, axis int) Mat4 {
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))
	i := (axis + 1) % 3
	j := (axis + 2) % 3
	m := IdentityMatrix()
	m[i*4+i] = c
	m[j*4+j] = c
	m[j*4+i] = -s
	m[i*4+j] = s
	return m
}

// ---------------------------------------------------------------------------
// TranslationMatrix
// ---------------------------------------------------------------------------

// TranslationMatrix returns a translation matrix that moves points by
// (x, y, z). The translation is stored in column 3 of the matrix.
//
// Ported from C Ironwail mathlib.c TranslationMatrix:
//
//	void TranslationMatrix(float matrix[16], float x, float y, float z)
//	{
//	    memset(matrix, 0, 16 * sizeof(float));
//	    matrix[0*4 + 0] = 1.0f;
//	    matrix[1*4 + 1] = 1.0f;
//	    matrix[2*4 + 2] = 1.0f;
//	    matrix[3*4 + 0] = x;
//	    matrix[3*4 + 1] = y;
//	    matrix[3*4 + 2] = z;
//	    matrix[3*4 + 3] = 1.0f;
//	}
func TranslationMatrix(x, y, z float32) Mat4 {
	var m Mat4
	m[0*4+0] = 1.0
	m[1*4+1] = 1.0
	m[2*4+2] = 1.0
	m[3*4+0] = x
	m[3*4+1] = y
	m[3*4+2] = z
	m[3*4+3] = 1.0
	return m
}

// ---------------------------------------------------------------------------
// Mat4Multiply
// ---------------------------------------------------------------------------

// Mat4Multiply returns the product left × right using column-major indexing.
//
// Ported from C Ironwail mathlib.c MatrixMultiply. The C version modifies
// `left` in-place (left = left * right). This Go version returns a new Mat4
// instead.
//
//	void MatrixMultiply(float left[16], float right[16])
//	{
//	    float temp[16];
//	    memcpy(temp, left, 16 * sizeof(float));
//	    for(row = 0; row < 4; ++row)
//	    {
//	        for(column = 0; column < 4; ++column)
//	        {
//	            float value = 0.0f;
//	            for (i = 0; i < 4; ++i)
//	                value += temp[i*4 + row] * right[column*4 + i];
//	            left[column * 4 + row] = value;
//	        }
//	    }
//	}
func Mat4Multiply(left, right Mat4) Mat4 {
	var out Mat4
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			var value float32
			for i := 0; i < 4; i++ {
				value += left[i*4+row] * right[col*4+i]
			}
			out[col*4+row] = value
		}
	}
	return out
}

// Mul returns the matrix product m × right. This is a method wrapper around
// [Mat4Multiply] for fluent chaining of transform composition:
//
//	mvp := projection.Mul(view).Mul(model)
//
// In the MVP (Model-View-Projection) pipeline, matrices are multiplied
// right-to-left: a vertex is first transformed by the model matrix, then
// the view matrix, then the projection matrix. Because matrix multiplication
// is associative, these can be pre-multiplied into a single MVP matrix and
// applied once per vertex in the shader.
func (m Mat4) Mul(right Mat4) Mat4 {
	return Mat4Multiply(m, right)
}

// ---------------------------------------------------------------------------
// FrustumMatrix
// ---------------------------------------------------------------------------

// FrustumMatrix returns a perspective projection matrix that also bakes in
// the Quake→clip-space coordinate conversion. fovx and fovy are field-of-view
// angles in RADIANS. n and f are the near and far clip plane distances.
//
// This is the STANDARD (non-reversed-Z) path. Reversed-Z is not included.
//
// Ported from C Ironwail gl_rmain.c GL_FrustumMatrix:
//
//	static void GL_FrustumMatrix(float matrix[16], float fovx, float fovy, float n, float f)
//	{
//	    const float w = 1.0f / tanf(fovx * 0.5f);
//	    const float h = 1.0f / tanf(fovy * 0.5f);
//	    memset(matrix, 0, 16 * sizeof(float));
//	    matrix[0*4 + 2] = (f + n) / (f - n);
//	    matrix[0*4 + 3] = 1.f;
//	    matrix[1*4 + 0] = -w;
//	    matrix[2*4 + 1] = h;
//	    matrix[3*4 + 2] = -2.f * f * n / (f - n);
//	}
//
// The coordinate conversion baked into this matrix:
//
//	clip X  =  -w · quakeY      Quake Y (left) → screen -X (flipped to right)
//	clip Y  =   h · quakeZ      Quake Z (up)   → screen Y (up)
//	clip Z  =  depth from quakeX   standard depth encoding
//	clip W  =  quakeX           forward axis becomes perspective denominator
func FrustumMatrix(fovx, fovy, n, f float32) Mat4 {
	w := 1.0 / float32(math.Tan(float64(fovx*0.5)))
	h := 1.0 / float32(math.Tan(float64(fovy*0.5)))
	var m Mat4
	m[0*4+2] = (f + n) / (f - n)
	m[0*4+3] = 1.0
	m[1*4+0] = -w
	m[2*4+1] = h
	m[3*4+2] = -2.0 * f * n / (f - n)
	return m
}

// ---------------------------------------------------------------------------
// Mat4MulVec4
// ---------------------------------------------------------------------------

// Mat4MulVec4 returns the product of a Mat4 and a Vec4 (column-major
// matrix–vector multiply). Each component of the result is the dot product
// of one matrix row with the vector.
//
//	result.X = m[0]*v.X + m[4]*v.Y + m[ 8]*v.Z + m[12]*v.W
//	result.Y = m[1]*v.X + m[5]*v.Y + m[ 9]*v.Z + m[13]*v.W
//	result.Z = m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]*v.W
//	result.W = m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]*v.W
func Mat4MulVec4(m Mat4, v Vec4) Vec4 {
	return Vec4{
		X: m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]*v.W,
		Y: m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]*v.W,
		Z: m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]*v.W,
		W: m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]*v.W,
	}
}

// ---------------------------------------------------------------------------
// ViewMatrix
// ---------------------------------------------------------------------------

// ViewMatrix builds a view matrix from Quake Euler angles and a camera
// origin. This matches C Ironwail's R_SetupGL exactly:
//
//	View = Rx(-roll) · Ry(-pitch) · Rz(-yaw) · T(-origin)
//
// The rotations operate in Quake world space — there is NO coordinate
// remapping in the view matrix. The projection matrix ([FrustumMatrix])
// handles the Quake→clip-space conversion.
//
// angles is a Vec3 where:
//
//	X = pitch (rotation around Y axis)
//	Y = yaw   (rotation around Z axis)
//	Z = roll  (rotation around X axis)
//
// origin is the camera position in Quake world coordinates.
func ViewMatrix(angles Vec3, origin Vec3) Mat4 {
	deg2rad := float32(math.Pi / 180.0)

	// Rx(-roll): negate roll, rotate around X axis (axis 0)
	view := RotationMatrix(-angles.Z*deg2rad, 0)
	// Ry(-pitch): negate pitch, rotate around Y axis (axis 1)
	pitch := RotationMatrix(-angles.X*deg2rad, 1)
	view = Mat4Multiply(view, pitch)
	// Rz(-yaw): negate yaw, rotate around Z axis (axis 2)
	yaw := RotationMatrix(-angles.Y*deg2rad, 2)
	view = Mat4Multiply(view, yaw)
	// T(-origin): translate by negated camera position
	trans := TranslationMatrix(-origin.X, -origin.Y, -origin.Z)
	view = Mat4Multiply(view, trans)

	return view
}

// ---------------------------------------------------------------------------
// Mat4ToBytes
// ---------------------------------------------------------------------------

// Determinant returns the determinant of the 4×4 matrix using cofactor
// expansion along the first row.
//
// The determinant measures how a transformation scales volume:
//   - |det| = 1 for pure rotations (volume-preserving)
//   - |det| > 1 for scaling up, |det| < 1 for scaling down
//   - det < 0 indicates a reflection (orientation-reversing)
//   - det = 0 means the matrix is singular (non-invertible)
//
// For a valid view or model matrix composed only of rotations and
// translations, the absolute value should be ≈ 1.0. This method is
// useful for debugging matrix construction and for computing the
// inverse (via the adjugate/determinant formula) when needed.
func (m Mat4) Determinant() float32 {
	// Column-major: element at (row, col) = m[col*4+row]
	// Cofactor expansion along row 0.
	c00 := m[5]*(m[10]*m[15]-m[14]*m[11]) - m[9]*(m[6]*m[15]-m[14]*m[7]) + m[13]*(m[6]*m[11]-m[10]*m[7])
	c01 := m[1]*(m[10]*m[15]-m[14]*m[11]) - m[9]*(m[2]*m[15]-m[14]*m[3]) + m[13]*(m[2]*m[11]-m[10]*m[3])
	c02 := m[1]*(m[6]*m[15]-m[14]*m[7]) - m[5]*(m[2]*m[15]-m[14]*m[3]) + m[13]*(m[2]*m[7]-m[6]*m[3])
	c03 := m[1]*(m[6]*m[11]-m[10]*m[7]) - m[5]*(m[2]*m[11]-m[10]*m[3]) + m[9]*(m[2]*m[7]-m[6]*m[3])
	return m[0]*c00 - m[4]*c01 + m[8]*c02 - m[12]*c03
}

// Mat4ToBytes converts a Mat4 into a 64-byte array of little-endian float32
// values, suitable for uploading to the GPU as a uniform buffer.
//
// GPU shader uniform buffers expect data in a specific byte layout. Since
// both the Mat4 and the GPU use column-major order, the 16 floats can be
// copied sequentially. Little-endian format matches x86/x64 and ARM (the
// dominant GPU host architectures). The fixed 64-byte size ([16]float32 ×
// 4 bytes each) allows the result to be passed directly as a UBO or push
// constant without heap allocation.
func Mat4ToBytes(m Mat4) [64]byte {
	var buf [64]byte
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(m[i]))
	}
	return buf
}
