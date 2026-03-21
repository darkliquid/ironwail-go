package renderer

import qtypes "github.com/ironwail/ironwail-go/pkg/types"

var identityModelRotationMatrix = [16]float32{
	1, 0, 0, 0,
	0, 1, 0, 0,
	0, 0, 1, 0,
	0, 0, 0, 1,
}

// buildBrushRotationMatrix builds a 4x4 rotation matrix from Euler angles for brush entity transforms (doors, platforms that rotate).
func buildBrushRotationMatrix(angles [3]float32) [16]float32 {
	if angles == [3]float32{} {
		return identityModelRotationMatrix
	}

	forward, right, up := qtypes.AngleVectors(qtypes.Vec3{
		X: -angles[0],
		Y: angles[1],
		Z: angles[2],
	})

	return [16]float32{
		forward.X, forward.Y, forward.Z, 0,
		-right.X, -right.Y, -right.Z, 0,
		up.X, up.Y, up.Z, 0,
		0, 0, 0, 1,
	}
}

// transformModelSpacePoint transforms a point from model space to world space using the entity's offset, rotation matrix, and scale.
func transformModelSpacePoint(point, modelOffset [3]float32, modelRotation [16]float32, modelScale float32) [3]float32 {
	if modelScale <= 0 {
		modelScale = 1
	}
	x := point[0] * modelScale
	y := point[1] * modelScale
	z := point[2] * modelScale
	return [3]float32{
		modelRotation[0]*x + modelRotation[4]*y + modelRotation[8]*z + modelOffset[0],
		modelRotation[1]*x + modelRotation[5]*y + modelRotation[9]*z + modelOffset[1],
		modelRotation[2]*x + modelRotation[6]*y + modelRotation[10]*z + modelOffset[2],
	}
}
