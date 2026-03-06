//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
)

const (
	worldVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec2 aLightmapCoord;
layout(location = 3) in vec3 aNormal;

uniform mat4 uViewProjection;

out vec3 vNormal;
out vec3 vWorldPos;

void main() {
    vNormal = aNormal;
    vWorldPos = aPosition;
    gl_Position = uViewProjection * vec4(aPosition, 1.0);
}`

	worldFragmentShaderGL = `#version 410 core
in vec3 vNormal;
in vec3 vWorldPos;
out vec4 fragColor;

void main() {
    vec3 n = normalize(vNormal);
    if (length(n) < 0.01) {
        n = vec3(0.0, 0.0, 1.0);
    }
    vec3 lightDir = normalize(vec3(0.35, 0.55, 0.75));
    float diffuse = max(dot(n, lightDir), 0.0);
    float ambient = 0.35;
    float shade = ambient + diffuse * 0.65;
    vec3 base = vec3(0.72, 0.74, 0.78);
    fragColor = vec4(base * shade, 1.0);
}`
)

func flattenWorldVertices(vertices []WorldVertex) []float32 {
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

func (r *Renderer) ensureWorldProgram() error {
	if r.worldProgram != 0 {
		return nil
	}

	vs, err := compileShader(worldVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile world vertex shader: %w", err)
	}
	fs, err := compileShader(worldFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile world fragment shader: %w", err)
	}

	program := createProgram(vs, fs)
	r.worldProgram = program
	r.worldVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	return nil
}

// UploadWorld builds CPU geometry and uploads it to OpenGL buffers.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.clearWorldLocked()

	renderData, err := buildWorldRenderData(tree)
	if err != nil {
		return fmt.Errorf("build world render data: %w", err)
	}
	if renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		r.worldData = renderData
		return nil
	}

	if err := r.ensureWorldProgram(); err != nil {
		return err
	}

	vertexData := flattenWorldVertices(renderData.Geometry.Vertices)

	gl.GenVertexArrays(1, &r.worldVAO)
	gl.GenBuffers(1, &r.worldVBO)
	gl.GenBuffers(1, &r.worldEBO)

	gl.BindVertexArray(r.worldVAO)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.worldVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.worldEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(renderData.Geometry.Indices)*4, gl.Ptr(renderData.Geometry.Indices), gl.STATIC_DRAW)

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

	r.worldData = renderData
	r.worldIndexCount = int32(len(renderData.Geometry.Indices))

	slog.Info("OpenGL world uploaded",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
	)
	return nil
}

func (r *Renderer) renderWorld() {
	r.mu.RLock()
	program := r.worldProgram
	vao := r.worldVAO
	indexCount := r.worldIndexCount
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	r.mu.RUnlock()

	if program == 0 || vao == 0 || indexCount <= 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.CULL_FACE)

	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.BindVertexArray(vao)
	gl.DrawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
	gl.BindVertexArray(0)
	gl.UseProgram(0)

	gl.Enable(gl.BLEND)
}

// HasWorldData reports whether the OpenGL world path has uploaded geometry.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVAO != 0 && r.worldProgram != 0 && r.worldIndexCount > 0
}

// GetWorldBounds returns the bounds of the uploaded world geometry.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}
	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}

func (r *Renderer) clearWorldLocked() {
	if r.worldVAO != 0 {
		gl.DeleteVertexArrays(1, &r.worldVAO)
		r.worldVAO = 0
	}
	if r.worldVBO != 0 {
		gl.DeleteBuffers(1, &r.worldVBO)
		r.worldVBO = 0
	}
	if r.worldEBO != 0 {
		gl.DeleteBuffers(1, &r.worldEBO)
		r.worldEBO = 0
	}
	if r.worldProgram != 0 {
		gl.DeleteProgram(r.worldProgram)
		r.worldProgram = 0
	}
	r.worldVPUniform = -1
	r.worldIndexCount = 0
	r.worldData = nil
}

// ClearWorld releases OpenGL world resources.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearWorldLocked()
}
