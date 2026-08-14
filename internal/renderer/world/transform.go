package world

import qtypes "github.com/darkliquid/ironwail-go/pkg/types"

var IdentityModelRotationMatrix = [16]float32{
	1, 0, 0, 0,
	0, 1, 0, 0,
	0, 0, 1, 0,
	0, 0, 0, 1,
}

// BuildBrushRotationMatrix builds a 4x4 rotation matrix from Euler angles.
func BuildBrushRotationMatrix(angles qtypes.Vec3) [16]float32 {
	if angles == (qtypes.Vec3{}) {
		return IdentityModelRotationMatrix
	}

	forward, right, up := qtypes.AngleVectors(qtypes.Vec3{
		X: -angles.X,
		Y: angles.Y,
		Z: angles.Z,
	})

	return [16]float32{
		forward.X, forward.Y, forward.Z, 0,
		-right.X, -right.Y, -right.Z, 0,
		up.X, up.Y, up.Z, 0,
		0, 0, 0, 1,
	}
}

// TransformModelSpacePoint transforms a model-space point into world-space.
func TransformModelSpacePoint(point, modelOffset qtypes.Vec3, modelRotation [16]float32, modelScale float32) qtypes.Vec3 {
	if modelScale <= 0 {
		modelScale = 1
	}
	x := point.X * modelScale
	y := point.Y * modelScale
	z := point.Z * modelScale
	return qtypes.Vec3{
		X: modelRotation[0]*x + modelRotation[4]*y + modelRotation[8]*z + modelOffset.X,
		Y: modelRotation[1]*x + modelRotation[5]*y + modelRotation[9]*z + modelOffset.Y,
		Z: modelRotation[2]*x + modelRotation[6]*y + modelRotation[10]*z + modelOffset.Z,
	}
}
