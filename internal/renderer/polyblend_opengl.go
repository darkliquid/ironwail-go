//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

// v_blend / polyblend full-screen color-tint pass.
//
// After the 3D scene (and any FBO blit for waterwarp) is resolved to the
// default framebuffer, this pass composites a full-screen alpha-blended quad
// in the specified RGBA color over the scene, before the 2D HUD overlay.
//
// C reference: Quake/view.c V_PolyBlend(), Quake/gl_rmain.c (glprogs.viewblend).
// The C engine merges the blend into the warpscale FBO blit when a scene FBO
// is in use; for simplicity the Go path always applies it as a separate pass.

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
)

const polyBlendVertexShaderGL = `#version 410 core

void main() {
	// Full-screen triangle from gl_VertexID – no vertex buffer needed.
	// Vertex 0 → (-1,-1), 1 → (3,-1), 2 → (-1,3).
	ivec2 v = ivec2(gl_VertexID & 1, gl_VertexID >> 1);
	gl_Position = vec4(vec2(v) * 4.0 - 1.0, 0.0, 1.0);
}`

const polyBlendFragmentShaderGL = `#version 410 core

// rgba in 0..1 range; .a is the composite blend factor.
uniform vec4 uBlendColor;

out vec4 fragColor;

void main() {
	fragColor = uBlendColor;
}`

// ensurePolyBlendProgram lazily compiles the polyblend full-screen shader.
// Mirrors C glprogs.viewblend program setup.
func (r *Renderer) ensurePolyBlendProgram() error {
	if r.polyBlendProgram != 0 {
		return nil
	}
	vs, err := compileShader(polyBlendVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("polyblend vertex shader: %w", err)
	}
	fs, err := compileShader(polyBlendFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("polyblend fragment shader: %w", err)
	}
	prog := createProgram(vs, fs)
	r.polyBlendProgram = prog
	r.polyBlendColorUniform = gl.GetUniformLocation(prog, gl.Str("uBlendColor\x00"))
	return nil
}

// renderPolyBlend draws a full-screen alpha-blended quad with the given RGBA
// color over the current framebuffer contents.
//
// blend[0..2] are R,G,B in 0..1; blend[3] is the alpha/opacity.
// Called after the 3D scene and any FBO blit, before the 2D overlay.
//
// Mirrors C view.c:V_PolyBlend() / glprogs.viewblend draw call.
func (r *Renderer) renderPolyBlend(blend [4]float32) {
	if blend[3] <= 0 {
		return
	}
	if err := r.ensurePolyBlendProgram(); err != nil {
		return
	}

	gl.UseProgram(r.polyBlendProgram)
	gl.Uniform4f(r.polyBlendColorUniform, blend[0], blend[1], blend[2], blend[3])

	// Alpha-blend over the scene; depth test and write are irrelevant.
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)

	// Full-screen triangle; no VAO/VBO needed (positions computed in vertex shader).
	gl.DrawArrays(gl.TRIANGLES, 0, 3)

	// Restore default raster state.
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Enable(gl.CULL_FACE)
	gl.Disable(gl.BLEND)

	gl.UseProgram(0)
}
