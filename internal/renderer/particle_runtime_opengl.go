//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

const (
	particleVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec4 aColor;

uniform mat4 uViewProjection;
uniform float uPointScale;

out vec4 vColor;

void main() {
	vColor = aColor;
	gl_Position = uViewProjection * vec4(aPosition, 1.0);
	float invW = 1.0 / max(gl_Position.w, 0.001);
	gl_PointSize = clamp(uPointScale * invW, 2.0, 48.0);
}`

	particleFragmentShaderGL = `#version 410 core
in vec4 vColor;
out vec4 fragColor;

void main() {
	// gl_PointCoord is in [0, 1]. Map to [-1, 1] for radial distance.
	vec2 centered = gl_PointCoord * 2.0 - 1.0;
	float radius = length(centered);
	
	// fwidth provides the screen-space rate of change, allowing for 
	// pixel-accurate anti-aliased edges. Matches C Ironwail style.
	float delta = fwidth(radius);
	float alpha = clamp((1.0 - radius) / delta, 0.0, 1.0);
	
	if (alpha <= 0.0) {
		discard;
	}
	
	fragColor = vec4(vColor.rgb, vColor.a * alpha);
}`
)

type particleRenderPass int

const (
	particlePassOpaque particleRenderPass = iota
	particlePassTranslucent
)

// buildParticlePaletteRGBA converts indexed particle colors into RGBA lookup data suitable for GPU-side tinting.
func buildParticlePaletteRGBA(palette []byte) [256][4]byte {
	var p [256][4]byte
	if len(palette) < 768 {
		for i := range p {
			p[i] = [4]byte{byte(i), byte(i), byte(i), 255}
		}
		return p
	}
	for i := range p {
		offset := i * 3
		p[i] = [4]byte{palette[offset], palette[offset+1], palette[offset+2], 255}
	}
	return p
}

// ensureParticleProgramLocked lazily creates particle shaders for billboard rendering with additive/alpha blend variants.
func (r *Renderer) ensureParticleProgramLocked() error {
	if r.particleProgram != 0 {
		return nil
	}

	vs, err := compileShader(particleVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile particle vertex shader: %w", err)
	}
	fs, err := compileShader(particleFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile particle fragment shader: %w", err)
	}

	program := createProgram(vs, fs)
	r.particleProgram = program
	r.particleVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	r.particlePointScaleUniform = gl.GetUniformLocation(program, gl.Str("uPointScale\x00"))
	return nil
}

// ensureParticleBuffersLocked allocates dynamic VBO/IBO resources sized for the current particle budget.
func (r *Renderer) ensureParticleBuffersLocked() {
	if r.particleVAO != 0 && r.particleVBO != 0 {
		return
	}

	gl.GenVertexArrays(1, &r.particleVAO)
	gl.GenBuffers(1, &r.particleVBO)

	gl.BindVertexArray(r.particleVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.particleVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 4, nil, gl.DYNAMIC_DRAW)

	stride := int32(unsafe.Sizeof(ParticleVertex{}))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 4, gl.UNSIGNED_BYTE, true, stride, 12)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
}

// renderParticles builds camera-facing quads and submits them in blend-order-aware passes so explosions/smoke read correctly over the world.
func (r *Renderer) renderParticles(ps *ParticleSystem, palette []byte, pass particleRenderPass) {
	if ps == nil || ps.ActiveCount() == 0 {
		return
	}

	active := ps.ActiveParticles()
	if len(active) == 0 {
		return
	}
	p := buildParticlePaletteRGBA(palette)
	vertices := BuildParticleVertices(active, p, false)
	if len(vertices) == 0 {
		return
	}
	drawVertices := particleVerticesForPass(vertices, readParticleModeCvar(), pass, false)
	if len(drawVertices) == 0 {
		return
	}

	r.mu.Lock()
	if err := r.ensureParticleProgramLocked(); err != nil {
		r.mu.Unlock()
		return
	}
	r.ensureParticleBuffersLocked()
	program := r.particleProgram
	vao := r.particleVAO
	vbo := r.particleVBO
	vp := r.viewMatrices.VP
	vpUniform := r.particleVPUniform
	pointScaleUniform := r.particlePointScaleUniform
	_, height := r.Size()
	r.mu.Unlock()

	if program == 0 || vao == 0 || vbo == 0 {
		return
	}

	pointScale := float32(12)
	if height > 0 {
		pointScale = float32(height) * 0.35
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	if pass == particlePassOpaque {
		gl.DepthMask(true)
		gl.Disable(gl.BLEND)
	} else {
		gl.DepthMask(false)
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1f(pointScaleUniform, pointScale)
	// Upload and draw in fixed-size batches to match C Ironwail's partverts[] behavior.
	// C flushes at MAX_PARTVERTS (512) and again at end-of-frame.
	const maxPartVerts = 512
	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	vertSize := int(unsafe.Sizeof(ParticleVertex{}))
	for len(drawVertices) > 0 {
		batch := drawVertices
		if len(batch) > maxPartVerts {
			batch = drawVertices[:maxPartVerts]
		}
		gl.BufferData(gl.ARRAY_BUFFER, len(batch)*vertSize, gl.Ptr(batch), gl.DYNAMIC_DRAW)
		gl.DrawArrays(gl.POINTS, 0, int32(len(batch)))
		drawVertices = drawVertices[len(batch):]
	}
	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.UseProgram(0)
	gl.DepthMask(true)
	gl.Enable(gl.BLEND)
}

// readParticleModeCvar maps user cvars to concrete particle rendering modes (classic points vs textured quads, etc.).
func readParticleModeCvar() int {
	cv := cvar.Get(CvarRParticles)
	if cv == nil {
		return 1
	}
	return cv.Int
}

// shouldDrawParticlePass decides whether a particle pass should run for the current view/configuration, skipping unnecessary work.
func shouldDrawParticlePass(mode int, pass particleRenderPass, showTris bool, activeParticles int) bool {
	return ShouldDrawParticles(mode, pass == particlePassTranslucent, showTris, activeParticles)
}

// particleVerticesForPass generates pass-specific billboard vertices using camera right/up vectors for correct facing.
func particleVerticesForPass(vertices []ParticleVertex, mode int, pass particleRenderPass, showTris bool) []ParticleVertex {
	if !shouldDrawParticlePass(mode, pass, showTris, len(vertices)) {
		return nil
	}
	return vertices
}

// clearParticleResourcesLocked releases particle GPU resources when contexts reset or particle rendering is disabled.
func (r *Renderer) clearParticleResourcesLocked() {
	if r.particleProgram != 0 {
		gl.DeleteProgram(r.particleProgram)
		r.particleProgram = 0
	}
	if r.particleVAO != 0 {
		gl.DeleteVertexArrays(1, &r.particleVAO)
		r.particleVAO = 0
	}
	if r.particleVBO != 0 {
		gl.DeleteBuffers(1, &r.particleVBO)
		r.particleVBO = 0
	}
	r.particleVPUniform = -1
	r.particlePointScaleUniform = -1
}
