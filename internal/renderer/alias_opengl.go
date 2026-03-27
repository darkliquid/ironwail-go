//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"github.com/ironwail/ironwail-go/internal/model"
	aliasimpl "github.com/ironwail/ironwail-go/internal/renderer/alias"
)

func interpolateVertexPosition(pose1Vert, pose2Vert model.TriVertX, scale, origin [3]float32, factor float32) [3]float32 {
	return aliasimpl.InterpolateVertexPosition(pose1Vert, pose2Vert, scale, origin, factor)
}

func buildAliasVerticesInterpolated(alias *glAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	return aliasimpl.BuildVerticesInterpolated(
		aliasimpl.MeshFromAccessor(alias.poses, alias.refs, func(ref glAliasVertexRef) aliasimpl.MeshRef {
			return aliasimpl.MeshRef{VertexIndex: ref.vertexIndex, TexCoord: ref.texCoord}
		}),
		mdl.AliasHeader,
		pose1Index,
		pose2Index,
		blend,
		origin,
		angles,
		entityScale,
		fullAngles,
	)
}

type InterpolationData = aliasimpl.InterpolationData
type AliasFrameDesc = aliasimpl.FrameDesc

// setupAliasFrameInterpolation computes which two poses to blend between and the blend factor.
// This is called each frame to update animation state.
func setupAliasFrameInterpolation(frameIndex int, frames []AliasFrameDesc, timeSeconds float64, lerpModels bool, flags int) InterpolationData {
	return aliasimpl.SetupFrameInterpolation(frameIndex, frames, timeSeconds, lerpModels, flags)
}
