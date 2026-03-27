//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import "github.com/go-gl/gl/v4.6-core/gl"

type DrawUniformState struct {
	AlphaUniform         int32
	TurbulentUniform     int32
	LitWaterUniform      int32
	DynamicLightUniform  int32
	ModelOffsetUniform   int32
	ModelRotationUniform int32
	ModelScaleUniform    int32
	HasFullbrightUniform int32
	DepthWrite           bool
	LitWaterEnabled      bool
}

// RenderDrawCalls issues GL draw calls for bucketed world faces. Each call binds its
// diffuse + lightmap + fullbright textures and draws the face's index range from the VAO.
func RenderDrawCalls(calls []DrawCall, state DrawUniformState) {
	if len(calls) == 0 {
		return
	}
	gl.DepthMask(state.DepthWrite)
	if state.DepthWrite {
		gl.Disable(gl.BLEND)
	} else {
		gl.Enable(gl.BLEND)
	}

	lastVAO := uint32(0xFFFFFFFF)
	lastLitWaterValue := float32(-1)
	for _, call := range calls {
		if call.VAO != lastVAO {
			gl.BindVertexArray(call.VAO)
			lastVAO = call.VAO
		}
		gl.Uniform3f(state.ModelOffsetUniform, call.ModelOffset[0], call.ModelOffset[1], call.ModelOffset[2])
		gl.UniformMatrix4fv(state.ModelRotationUniform, 1, false, &call.ModelRotation[0])
		gl.Uniform1f(state.ModelScaleUniform, call.ModelScale)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, call.Texture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, call.Lightmap)

		gl.ActiveTexture(gl.TEXTURE2)
		if call.FullbrightTexture != 0 {
			gl.BindTexture(gl.TEXTURE_2D, call.FullbrightTexture)
			gl.Uniform1f(state.HasFullbrightUniform, 1.0)
		} else {
			gl.BindTexture(gl.TEXTURE_2D, 0)
			gl.Uniform1f(state.HasFullbrightUniform, 0.0)
		}

		gl.ActiveTexture(gl.TEXTURE0)
		if call.Turbulent {
			gl.Uniform1f(state.TurbulentUniform, 1)
		} else {
			gl.Uniform1f(state.TurbulentUniform, 0)
		}
		if state.LitWaterUniform >= 0 {
			litWaterValue := float32(0)
			if state.LitWaterEnabled && call.HasLitWater {
				litWaterValue = 1
			}
			if litWaterValue != lastLitWaterValue {
				gl.Uniform1f(state.LitWaterUniform, litWaterValue)
				lastLitWaterValue = litWaterValue
			}
		}
		gl.Uniform3f(state.DynamicLightUniform, call.Light[0], call.Light[1], call.Light[2])
		gl.Uniform1f(state.AlphaUniform, call.Alpha)
		//lint:ignore SA1019 OpenGL indexed draws require byte offsets into the bound element array buffer.
		gl.DrawElements(gl.TRIANGLES, int32(call.Face.NumIndices), gl.UNSIGNED_INT, gl.PtrOffset(int(call.Face.FirstIndex*4)))
	}
}
