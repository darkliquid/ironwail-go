//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/go-gl/gl/v4.6-core/gl"
)

type MeshUpload struct {
	VAO        uint32
	VBO        uint32
	EBO        uint32
	IndexCount int32
}

func UploadWorldMesh(vertices []worldimpl.WorldVertex, indices []uint32) *MeshUpload {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil
	}

	vertexData := FlattenWorldVertices(vertices)
	mesh := &MeshUpload{IndexCount: int32(len(indices))}

	gl.GenVertexArrays(1, &mesh.VAO)
	gl.GenBuffers(1, &mesh.VBO)
	gl.GenBuffers(1, &mesh.EBO)

	gl.BindVertexArray(mesh.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, mesh.VBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mesh.EBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	const stride = 10 * 4
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 3, gl.FLOAT, false, stride, 7*4)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	return mesh
}
