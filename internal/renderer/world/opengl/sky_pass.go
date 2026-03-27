//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	surfaceimpl "github.com/ironwail/ironwail-go/internal/renderer/surface"
)

type ExternalSkyMode uint8

const (
	ExternalSkyEmbedded ExternalSkyMode = iota
	ExternalSkyCubemap
	ExternalSkyFaces
)

type SkyPassState struct {
	Program                     uint32
	ProceduralProgram           uint32
	CubemapProgram              uint32
	ExternalFaceProgram         uint32
	VPUniform                   int32
	SolidUniform                int32
	AlphaUniform                int32
	ProceduralVPUniform         int32
	CubemapVPUniform            int32
	CubemapUniform              int32
	ExternalFaceVPUniform       int32
	ExternalFaceRTUniform       int32
	ExternalFaceBKUniform       int32
	ExternalFaceLFUniform       int32
	ExternalFaceFTUniform       int32
	ExternalFaceUPUniform       int32
	ExternalFaceDNUniform       int32
	ModelOffsetUniform          int32
	ModelRotationUniform        int32
	ModelScaleUniform           int32
	ProceduralModelOffset       int32
	ProceduralModelRotation     int32
	ProceduralModelScale        int32
	CubemapModelOffsetUniform   int32
	CubemapModelRotationUniform int32
	CubemapModelScaleUniform    int32
	ExternalFaceModelOffset     int32
	ExternalFaceModelRotation   int32
	ExternalFaceModelScale      int32
	TimeUniform                 int32
	SolidLayerSpeedUniform      int32
	AlphaLayerSpeedUniform      int32
	CameraOriginUniform         int32
	ProceduralCameraOrigin      int32
	CubemapCameraOriginUniform  int32
	ExternalFaceCameraOrigin    int32
	FogColorUniform             int32
	ProceduralFogColor          int32
	CubemapFogColorUniform      int32
	ExternalFaceFogColor        int32
	FogDensityUniform           int32
	ProceduralFogDensity        int32
	ProceduralHorizonColor      int32
	ProceduralZenithColor       int32
	CubemapFogDensityUniform    int32
	ExternalFaceFogDensity      int32
	VP                          [16]float32
	Time                        float32
	SolidLayerSpeed             float32
	AlphaLayerSpeed             float32
	CameraOrigin                [3]float32
	ModelOffset                 [3]float32
	ModelRotation               [16]float32
	ModelScale                  float32
	FogColor                    [3]float32
	ProceduralHorizon           [3]float32
	ProceduralZenith            [3]float32
	FogDensity                  float32
	SolidTextures               map[int32]uint32
	AlphaTextures               map[int32]uint32
	FlatTextures                map[int32]uint32
	TextureAnimations           []*surfaceimpl.SurfaceTexture
	FallbackSolid               uint32
	FallbackAlpha               uint32
	ExternalSkyMode             ExternalSkyMode
	ExternalCubemap             uint32
	ExternalFaceTextures        [6]uint32
	Frame                       int
	FastSky                     bool
	ProceduralSky               bool
}

// RenderSkyPass renders sky surfaces using one of three sky shader programs:
// embedded two-layer scrolling sky, cubemap sky, or individual face textures.
func RenderSkyPass(calls []DrawCall, state SkyPassState, animateTexture TextureAnimationFunc) {
	if len(calls) == 0 {
		return
	}
	useCubemap := state.ExternalSkyMode == ExternalSkyCubemap && state.ExternalCubemap != 0
	useExternalFaces := state.ExternalSkyMode == ExternalSkyFaces
	useProcedural := state.ProceduralSky && !useCubemap && !useExternalFaces
	if useProcedural {
		if state.ProceduralProgram == 0 {
			return
		}
		gl.UseProgram(state.ProceduralProgram)
		gl.UniformMatrix4fv(state.ProceduralVPUniform, 1, false, &state.VP[0])
		gl.Uniform3f(state.ProceduralModelOffset, state.ModelOffset[0], state.ModelOffset[1], state.ModelOffset[2])
		gl.UniformMatrix4fv(state.ProceduralModelRotation, 1, false, &state.ModelRotation[0])
		gl.Uniform1f(state.ProceduralModelScale, state.ModelScale)
		gl.Uniform3f(state.ProceduralCameraOrigin, state.CameraOrigin[0], state.CameraOrigin[1], state.CameraOrigin[2])
		gl.Uniform3f(state.ProceduralFogColor, state.FogColor[0], state.FogColor[1], state.FogColor[2])
		gl.Uniform1f(state.ProceduralFogDensity, state.FogDensity)
		gl.Uniform3f(state.ProceduralHorizonColor, state.ProceduralHorizon[0], state.ProceduralHorizon[1], state.ProceduralHorizon[2])
		gl.Uniform3f(state.ProceduralZenithColor, state.ProceduralZenith[0], state.ProceduralZenith[1], state.ProceduralZenith[2])
	} else if useCubemap {
		if state.CubemapProgram == 0 {
			return
		}
		gl.UseProgram(state.CubemapProgram)
		gl.UniformMatrix4fv(state.CubemapVPUniform, 1, false, &state.VP[0])
		gl.Uniform1i(state.CubemapUniform, 2)
		gl.Uniform3f(state.CubemapModelOffsetUniform, state.ModelOffset[0], state.ModelOffset[1], state.ModelOffset[2])
		gl.UniformMatrix4fv(state.CubemapModelRotationUniform, 1, false, &state.ModelRotation[0])
		gl.Uniform1f(state.CubemapModelScaleUniform, state.ModelScale)
		gl.Uniform3f(state.CubemapCameraOriginUniform, state.CameraOrigin[0], state.CameraOrigin[1], state.CameraOrigin[2])
		gl.Uniform3f(state.CubemapFogColorUniform, state.FogColor[0], state.FogColor[1], state.FogColor[2])
		gl.Uniform1f(state.CubemapFogDensityUniform, state.FogDensity)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, state.ExternalCubemap)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		if state.ExternalFaceProgram == 0 {
			return
		}
		gl.UseProgram(state.ExternalFaceProgram)
		gl.UniformMatrix4fv(state.ExternalFaceVPUniform, 1, false, &state.VP[0])
		gl.Uniform1i(state.ExternalFaceRTUniform, 2)
		gl.Uniform1i(state.ExternalFaceBKUniform, 3)
		gl.Uniform1i(state.ExternalFaceLFUniform, 4)
		gl.Uniform1i(state.ExternalFaceFTUniform, 5)
		gl.Uniform1i(state.ExternalFaceUPUniform, 6)
		gl.Uniform1i(state.ExternalFaceDNUniform, 7)
		gl.Uniform3f(state.ExternalFaceModelOffset, state.ModelOffset[0], state.ModelOffset[1], state.ModelOffset[2])
		gl.UniformMatrix4fv(state.ExternalFaceModelRotation, 1, false, &state.ModelRotation[0])
		gl.Uniform1f(state.ExternalFaceModelScale, state.ModelScale)
		gl.Uniform3f(state.ExternalFaceCameraOrigin, state.CameraOrigin[0], state.CameraOrigin[1], state.CameraOrigin[2])
		gl.Uniform3f(state.ExternalFaceFogColor, state.FogColor[0], state.FogColor[1], state.FogColor[2])
		gl.Uniform1f(state.ExternalFaceFogDensity, state.FogDensity)
		for i, texture := range state.ExternalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, texture)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	} else {
		if state.Program == 0 {
			return
		}
		gl.UseProgram(state.Program)
		gl.UniformMatrix4fv(state.VPUniform, 1, false, &state.VP[0])
		gl.Uniform1i(state.SolidUniform, 0)
		gl.Uniform1i(state.AlphaUniform, 1)
		gl.Uniform3f(state.ModelOffsetUniform, state.ModelOffset[0], state.ModelOffset[1], state.ModelOffset[2])
		gl.UniformMatrix4fv(state.ModelRotationUniform, 1, false, &state.ModelRotation[0])
		gl.Uniform1f(state.ModelScaleUniform, state.ModelScale)
		gl.Uniform1f(state.TimeUniform, state.Time)
		gl.Uniform1f(state.SolidLayerSpeedUniform, state.SolidLayerSpeed)
		gl.Uniform1f(state.AlphaLayerSpeedUniform, state.AlphaLayerSpeed)
		gl.Uniform3f(state.CameraOriginUniform, state.CameraOrigin[0], state.CameraOrigin[1], state.CameraOrigin[2])
		gl.Uniform3f(state.FogColorUniform, state.FogColor[0], state.FogColor[1], state.FogColor[2])
		gl.Uniform1f(state.FogDensityUniform, state.FogDensity)
	}

	gl.DepthFunc(gl.LEQUAL)
	gl.DepthMask(false)
	gl.Disable(gl.BLEND)

	for _, call := range calls {
		if !useProcedural && !useCubemap && !useExternalFaces {
			solid, alpha := SkyTexturesForFace(
				call.Face,
				state.SolidTextures,
				state.AlphaTextures,
				state.TextureAnimations,
				state.FallbackSolid,
				state.FallbackAlpha,
				state.Frame,
				float64(state.Time),
				animateTexture,
			)
			if state.FastSky {
				solid = SkyFlatTextureForFace(
					call.Face,
					state.FlatTextures,
					state.TextureAnimations,
					state.FallbackSolid,
					state.Frame,
					float64(state.Time),
					animateTexture,
				)
				alpha = state.FallbackAlpha
			}
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, solid)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, alpha)
			gl.ActiveTexture(gl.TEXTURE0)
		}
		//lint:ignore SA1019 OpenGL indexed draws require byte offsets into the bound element array buffer.
		gl.DrawElements(gl.TRIANGLES, int32(call.Face.NumIndices), gl.UNSIGNED_INT, gl.PtrOffset(int(call.Face.FirstIndex*4)))
	}
	if useCubemap {
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		for i := range state.ExternalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, 0)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	}

	gl.DepthFunc(gl.LESS)
	gl.DepthMask(true)
}
