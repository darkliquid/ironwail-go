//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/go-gl/gl/v4.6-core/gl"
)

type SpriteDrawParams struct {
	VBO                uint32
	VAO                uint32
	ModelOffset        [3]float32
	Alpha              float32
	ModelOffsetUniform int32
	AlphaUniform       int32
	DepthOffset        bool
}

func FlattenWorldVertices(vertices []worldimpl.WorldVertex) []float32 {
	data := make([]float32, 0, len(vertices)*10)
	for _, v := range vertices {
		data = append(data,
			v.Position[0], v.Position[1], v.Position[2],
			v.TexCoord[0], v.TexCoord[1],
			v.LightmapCoord[0], v.LightmapCoord[1],
			v.Normal[0], v.Normal[1], v.Normal[2],
		)
	}
	return data
}

func DrawSpriteWorldVertices(vertices []worldimpl.WorldVertex, params SpriteDrawParams) {
	if len(vertices) == 0 {
		return
	}
	vertexData := FlattenWorldVertices(vertices)
	if params.DepthOffset {
		gl.Enable(gl.POLYGON_OFFSET_FILL)
		gl.PolygonOffset(-1.0, -2.0)
		defer gl.Disable(gl.POLYGON_OFFSET_FILL)
	}
	gl.BindBuffer(gl.ARRAY_BUFFER, params.VBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
	gl.Uniform3f(params.ModelOffsetUniform, params.ModelOffset[0], params.ModelOffset[1], params.ModelOffset[2])
	gl.Uniform1f(params.AlphaUniform, params.Alpha)
	gl.BindVertexArray(params.VAO)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
}
