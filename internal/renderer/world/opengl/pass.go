//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import "github.com/go-gl/gl/v4.6-core/gl"

type WorldProgramState struct {
	Program              uint32
	VPUniform            int32
	TextureUniform       int32
	LightmapUniform      int32
	FullbrightUniform    int32
	HasFullbrightUniform int32
	DynamicLightUniform  int32
	TimeUniform          int32
	TurbulentUniform     int32
	LitWaterUniform      int32
	CameraOriginUniform  int32
	FogColorUniform      int32
	FogDensityUniform    int32
	VP                   [16]float32
	Time                 float32
	CameraOrigin         [3]float32
	FogColor             [3]float32
	FogDensity           float32
}

type WorldModelState struct {
	ModelOffsetUniform   int32
	ModelRotationUniform int32
	ModelScaleUniform    int32
	ModelOffset          [3]float32
	ModelRotation        [16]float32
	ModelScale           float32
	VAO                  uint32
}

func BindWorldProgram(programState WorldProgramState, modelState WorldModelState) {
	gl.UseProgram(programState.Program)
	gl.UniformMatrix4fv(programState.VPUniform, 1, false, &programState.VP[0])
	gl.Uniform1i(programState.TextureUniform, 0)
	gl.Uniform1i(programState.LightmapUniform, 1)
	gl.Uniform1i(programState.FullbrightUniform, 2)
	gl.Uniform1f(programState.HasFullbrightUniform, 0)
	gl.Uniform3f(programState.DynamicLightUniform, 0, 0, 0)
	gl.Uniform3f(modelState.ModelOffsetUniform, modelState.ModelOffset[0], modelState.ModelOffset[1], modelState.ModelOffset[2])
	gl.UniformMatrix4fv(modelState.ModelRotationUniform, 1, false, &modelState.ModelRotation[0])
	gl.Uniform1f(modelState.ModelScaleUniform, modelState.ModelScale)
	gl.Uniform1f(programState.TimeUniform, programState.Time)
	gl.Uniform1f(programState.TurbulentUniform, 0)
	gl.Uniform1f(programState.LitWaterUniform, 0)
	gl.Uniform3f(programState.CameraOriginUniform, programState.CameraOrigin[0], programState.CameraOrigin[1], programState.CameraOrigin[2])
	gl.Uniform3f(programState.FogColorUniform, programState.FogColor[0], programState.FogColor[1], programState.FogColor[2])
	gl.Uniform1f(programState.FogDensityUniform, programState.FogDensity)
	if modelState.VAO != 0 {
		gl.BindVertexArray(modelState.VAO)
	}
}
