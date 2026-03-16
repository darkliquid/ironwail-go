//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"math"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

// Underwater screen-space warp (r_waterwarp == 1) implementation.
//
// Mirrors C Ironwail R_WarpScaleView() / glprogs.warpscale[1]:
// After the 3D scene is rendered to sceneFBO, this pass blits it to the
// default framebuffer with sinusoidal UV distortion keyed to cl.time.
//
// C reference: Quake/gl_rmain.c R_WarpScaleView(), Quake/gl_shaders.h warpscale_fragment_shader.

const warpScaleVertexShaderGL = `#version 410 core

void main() {
	// Generate a full-screen triangle from vertex index (no vertex buffer needed).
	// gl_VertexID 0→(0,0), 1→(2,0), 2→(0,2) in UV space.
	ivec2 v = ivec2(gl_VertexID & 1, gl_VertexID >> 1);
	vec2 uv = vec2(v) * 2.0;
	gl_Position = vec4(uv * 2.0 - 1.0, 0.0, 1.0);
}`

const warpScaleFragmentShaderGL = `#version 410 core

uniform sampler2D uSceneTex;

// x=smax, y=tmax, z=warpAmp, w=time
// smax/tmax: UV scale (always 1.0 in Go; C uses sub-viewport scale for r_scale).
// warpAmp: 0 = no warp (scale-only blit); non-zero = sinusoidal amplitude (1/256).
// time: cl.time (or realtime for forced-underwater menu preview).
uniform vec4 uUVScaleWarpTime;

out vec4 fragColor;

void main() {
	// Reconstruct UV from fragment position (maps screen to [0,1]).
	vec2 uv = gl_FragCoord.xy / vec2(textureSize(uSceneTex, 0));

	vec2 uvScale  = uUVScaleWarpTime.xy;
	float warpAmp = uUVScaleWarpTime.z;
	float time    = uUVScaleWarpTime.w;

	if (warpAmp > 0.0) {
		// Sinusoidal screen-space distortion: mirrors C warpscale_fragment_shader #if WARP.
		// aspect compensates dFdy/dFdx ratio so distortion is isotropic in world space.
		float aspect = abs(dFdy(uv.y)) / max(abs(dFdx(uv.x)), 0.0001);
		vec2 amp = vec2(warpAmp, warpAmp * aspect);
		// Remap UV into safe area [amp, 1-amp] to prevent border clamping artefacts.
		uv = amp + uv * (1.0 - 2.0 * amp);
		// Sinusoidal displacement: X displaced by sin(uv.y) and Y displaced by sin(uv.x).
		uv += amp * sin(vec2(uv.y / max(aspect, 0.0001), uv.x) * (3.14159265 * 8.0) + time);
	}

	fragColor = texture(uSceneTex, uv * uvScale);
}`

// ensureWarpScaleProgram lazily compiles the warpscale post-process shader.
func (r *Renderer) ensureWarpScaleProgram() error {
	if r.warpScaleProgram != 0 {
		return nil
	}
	vs, err := compileShader(warpScaleVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("warpscale vertex shader: %w", err)
	}
	fs, err := compileShader(warpScaleFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("warpscale fragment shader: %w", err)
	}
	prog := createProgram(vs, fs)
	r.warpScaleProgram = prog
	r.warpScaleSceneTex = gl.GetUniformLocation(prog, gl.Str("uSceneTex\x00"))
	r.warpScaleUVScaleWarpTime = gl.GetUniformLocation(prog, gl.Str("uUVScaleWarpTime\x00"))
	return nil
}

// ensureSceneFBO creates or recreates the offscreen scene framebuffer when the
// window size changes. On success r.sceneFBO is non-zero and matches (w, h).
func (r *Renderer) ensureSceneFBO(w, h int) error {
	if r.sceneFBOWidth == w && r.sceneFBOHeight == h && r.sceneFBO != 0 {
		return nil
	}
	r.destroySceneFBO()

	var fbo, colorTex, depthRBO uint32

	// Color texture
	gl.GenTextures(1, &colorTex)
	gl.BindTexture(gl.TEXTURE_2D, colorTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// Depth renderbuffer
	gl.GenRenderbuffers(1, &depthRBO)
	gl.BindRenderbuffer(gl.RENDERBUFFER, depthRBO)
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT24, int32(w), int32(h))
	gl.BindRenderbuffer(gl.RENDERBUFFER, 0)

	// Framebuffer
	gl.GenFramebuffers(1, &fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, colorTex, 0)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depthRBO)

	if status := gl.CheckFramebufferStatus(gl.FRAMEBUFFER); status != gl.FRAMEBUFFER_COMPLETE {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.DeleteFramebuffers(1, &fbo)
		gl.DeleteTextures(1, &colorTex)
		gl.DeleteRenderbuffers(1, &depthRBO)
		return fmt.Errorf("scene FBO incomplete: status 0x%x", status)
	}
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)

	r.sceneFBO = fbo
	r.sceneColorTex = colorTex
	r.sceneDepthRBO = depthRBO
	r.sceneFBOWidth = w
	r.sceneFBOHeight = h
	return nil
}

// destroySceneFBO releases the scene framebuffer resources.
func (r *Renderer) destroySceneFBO() {
	if r.sceneFBO != 0 {
		gl.DeleteFramebuffers(1, &r.sceneFBO)
		r.sceneFBO = 0
	}
	if r.sceneColorTex != 0 {
		gl.DeleteTextures(1, &r.sceneColorTex)
		r.sceneColorTex = 0
	}
	if r.sceneDepthRBO != 0 {
		gl.DeleteRenderbuffers(1, &r.sceneDepthRBO)
		r.sceneDepthRBO = 0
	}
	r.sceneFBOWidth = 0
	r.sceneFBOHeight = 0
}

// applyWarpScaleEffect blits the scene FBO to the default framebuffer.
// warpActive true: distort with sinusoidal warp (r_waterwarp == 1 path).
// warpActive false: plain pass-through blit (no warp; future r_scale support).
// time is cl.time or realtime (for forced-underwater menu preview).
func (r *Renderer) applyWarpScaleEffect(warpActive bool, time float32, w, h int) {
	if err := r.ensureWarpScaleProgram(); err != nil {
		return
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.Viewport(0, 0, int32(w), int32(h))
	gl.Disable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.CULL_FACE)

	gl.UseProgram(r.warpScaleProgram)

	// smax, tmax = 1.0 (Go always renders at full viewport resolution; C may use r_scale).
	warpAmp := float32(0)
	if warpActive {
		warpAmp = 1.0 / 256.0 // mirrors C: water_warp ? 1.f/256.f : 0.f
	}
	gl.Uniform4f(r.warpScaleUVScaleWarpTime, 1.0, 1.0, warpAmp, time)

	// Linear sampling for warp to reduce aliasing; nearest for plain blit.
	filter := int32(gl.NEAREST)
	if warpActive {
		filter = gl.LINEAR
	}
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, r.sceneColorTex)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter)
	gl.Uniform1i(r.warpScaleSceneTex, 0)

	// Full-screen triangle; no vertex buffer needed (see vertex shader).
	gl.DrawArrays(gl.TRIANGLES, 0, 3)

	gl.UseProgram(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Enable(gl.BLEND)
	gl.Enable(gl.CULL_FACE)
}

// readWaterwarpCvar returns the current r_waterwarp value (0, 1, or >1).
func readWaterwarpCvar() float32 {
	cv := cvar.Get(CvarRWaterwarp)
	if cv == nil {
		return 0
	}
	return cv.Float32()
}

// WaterwarpFOV reports whether the FOV-oscillation underwater warp is active
// (r_waterwarp > 1 and the given underwater flag is true).
func WaterwarpFOV(underwaterOrForced bool) bool {
	return underwaterOrForced && readWaterwarpCvar() > 1.0
}

// WaterwarpFOVScale computes the horizontal FOV scale factor for r_waterwarp > 1.
// Returns a multiplier to apply to tan(fov/2) — matches C formula:
//
//	r_fovx = atan(tan(fov_x/2) * scale) * 2 / DEG2RAD
//	scale  = 0.97 + sin(t * 1.5) * 0.03
//
// The resulting modified FOV is: 2 * atan(scale * tan(baseFOV/2)).
func WaterwarpFOVScale(t float32) float32 {
	return float32(0.97 + math.Sin(float64(t)*1.5)*0.03)
}

// ApplyWaterwarpFOV returns the FOV (in degrees) after applying the r_waterwarp > 1
// sinusoidal modulation. baseFOV is the unmodified FOV in degrees; t is cl.time.
// Mirrors C Ironwail R_SetupView r_waterwarp > 1 branch.
func ApplyWaterwarpFOV(baseFOV, t float32) float32 {
	scale := WaterwarpFOVScale(t)
	halfTan := float32(math.Tan(float64(baseFOV) * math.Pi / 360.0))
	return float32(math.Atan(float64(halfTan)*float64(scale))) * 360.0 / math.Pi
}
