//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
)

const (
	decalFloatsPerVertex = 10

	decalVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec4 aColor;
layout(location = 3) in float aVariant;

uniform mat4 uViewProjection;

out vec2 vTexCoord;
out vec4 vColor;
out float vVariant;

void main() {
	vTexCoord = aTexCoord;
	vColor = aColor;
	vVariant = aVariant;
	gl_Position = uViewProjection * vec4(aPosition, 1.0);
}`

	decalFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec4 vColor;
in float vVariant;
out vec4 fragColor;

uniform sampler2D uAtlasTexture;

void main() {
	// Atlas is 2x2 grid: [Bullet, Chip] on top row, [Scorch, Magic] on bottom
	int variant = int(vVariant + 0.5);
	float atlasX = float(variant % 2) * 0.5;
	float atlasY = float(variant / 2) * 0.5;
	vec2 atlasUV = vec2(atlasX, atlasY) + vTexCoord * 0.5;
	
	vec4 texSample = texture(uAtlasTexture, atlasUV);
	
	// Apply edge fade based on distance from center
	vec2 p = vTexCoord * 2.0 - 1.0;
	float d2 = dot(p, p);
	if (d2 > 1.0) {
		discard;
	}
	float edge = smoothstep(1.0, 0.7, d2);
	
	fragColor = vec4(vColor.rgb * texSample.rgb, vColor.a * edge * texSample.a);
}`
)

// ensureDecalProgramLocked creates shaders used to project and blend bullet-hole/blood decals onto world geometry.
func (r *Renderer) ensureDecalProgramLocked() error {
	if r.decalProgram != 0 && r.decalVAO != 0 && r.decalVBO != 0 && r.decalAtlasTexture != 0 {
		return nil
	}

	vs, err := compileShader(decalVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile decal vertex shader: %w", err)
	}
	fs, err := compileShader(decalFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile decal fragment shader: %w", err)
	}
	program := createProgram(vs, fs)

	r.decalProgram = program
	r.decalVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	r.decalAtlasUniform = gl.GetUniformLocation(program, gl.Str("uAtlasTexture\x00"))

	gl.GenVertexArrays(1, &r.decalVAO)
	gl.GenBuffers(1, &r.decalVBO)

	gl.BindVertexArray(r.decalVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.decalVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 6*10*4, nil, gl.DYNAMIC_DRAW)

	const stride = 10 * 4
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 1, gl.FLOAT, false, stride, 9*4)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// Create and upload atlas texture
	if r.decalAtlasTexture == 0 {
		atlasData := generateDecalAtlasData()
		gl.GenTextures(1, &r.decalAtlasTexture)
		gl.BindTexture(gl.TEXTURE_2D, r.decalAtlasTexture)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 256, 256, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(atlasData))
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}

	return nil
}

// renderDecalMarks draws persistent mark geometry after opaque world passes using blending and depth bias to avoid z-fighting.
func (r *Renderer) renderDecalMarks(marks []DecalMarkEntity) {
	if len(marks) == 0 {
		return
	}

	r.mu.Lock()
	if err := r.ensureDecalProgramLocked(); err != nil {
		r.mu.Unlock()
		return
	}
	program := r.decalProgram
	vpUniform := r.decalVPUniform
	atlasUniform := r.decalAtlasUniform
	atlasTexture := r.decalAtlasTexture
	vao := r.decalVAO
	vbo := r.decalVBO
	vp := r.viewMatrices.VP
	camera := r.cameraState
	r.mu.Unlock()

	draws := prepareDecalDraws(marks, camera)
	if len(draws) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Use stencil to prevent overlapping decals from blending multiple times on same spot.
	// This avoids the "doubled-up" look where bullet holes overlap.
	gl.Enable(gl.STENCIL_TEST)
	gl.StencilFunc(gl.EQUAL, 0, 0xFF)
	gl.StencilOp(gl.KEEP, gl.KEEP, gl.INCR)

	gl.Enable(gl.POLYGON_OFFSET_FILL)
	gl.PolygonOffset(-1.0, -2.0)

	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, atlasTexture)
	gl.Uniform1i(atlasUniform, 0)
	gl.BindVertexArray(vao)

	for _, draw := range draws {
		verts := buildDecalTriangleVertices(draw.mark)
		if len(verts) == 0 {
			continue
		}
		gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(verts)/decalFloatsPerVertex))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Disable(gl.POLYGON_OFFSET_FILL)
	gl.Disable(gl.STENCIL_TEST)
	gl.DepthMask(true)
}

// buildDecalTriangleVertices clips/project triangles into decal space, producing geometry that conforms to impacted surfaces.
func buildDecalTriangleVertices(mark DecalMarkEntity) []float32 {
	corners, ok := buildDecalQuad(mark)
	if !ok {
		return nil
	}

	color := [4]float32{clamp01(mark.Color[0]), clamp01(mark.Color[1]), clamp01(mark.Color[2]), clamp01(mark.Alpha)}
	variant := float32(mark.Variant)
	uv := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	indices := [6]int{0, 1, 2, 0, 2, 3}

	out := make([]float32, 0, 6*decalFloatsPerVertex)
	for _, idx := range indices {
		corner := corners[idx]
		coord := uv[idx]
		out = append(out,
			corner[0], corner[1], corner[2],
			coord[0], coord[1],
			color[0], color[1], color[2], color[3],
			variant,
		)
	}
	return out
}
