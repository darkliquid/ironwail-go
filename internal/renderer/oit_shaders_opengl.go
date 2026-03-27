//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
	worldopengl "github.com/ironwail/ironwail-go/internal/renderer/world/opengl"
)

// oitWorldFragmentShaderGL outputs weighted blended OIT accumulation and
// revealage to MRT attachments 0 and 1. It mirrors worldFragmentShaderGL
// in uniforms and varyings so the existing runtime uniform setup still
// applies when the OIT world program is active.
const oitWorldFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec2 vLightmapCoord;
in vec3 vNormal;
in vec3 vWorldPos;

layout(location = 0) out vec4 outAccum;
layout(location = 1) out float outReveal;

uniform sampler2D uTexture;
uniform sampler2D uLightmap;
uniform sampler2D uFullbright;
uniform vec3 uDynamicLight;
uniform float uAlpha;
uniform float uTime;
uniform float uTurbulent;
uniform vec3 uCameraOrigin;
uniform vec3 uFogColor;
uniform float uFogDensity;
uniform float uHasFullbright;
uniform float uLitWater;

void main() {
	vec2 uv = vTexCoord;
	if (uTurbulent > 0.5) {
		uv = uv * 2.0 + 0.125 * sin(uv.yx * (3.14159265 * 2.0) + uTime);
	}
	vec4 base = texture(uTexture, uv);
	vec3 light;
	if (uTurbulent > 0.5 && uLitWater < 0.5) {
		light = vec3(0.5) + uDynamicLight;
	} else {
		light = texture(uLightmap, vLightmapCoord).rgb + uDynamicLight;
	}
	if (base.a < 0.1) {
		discard;
	}
	vec3 litColor = base.rgb * light * 2.0;
	if (uHasFullbright > 0.5) {
		vec4 fb = texture(uFullbright, uv);
		litColor = litColor + fb.rgb * fb.a;
	}
	vec3 fogPosition = vWorldPos - uCameraOrigin;
	float fog = exp2(-uFogDensity * dot(fogPosition, fogPosition));
	fog = clamp(fog, 0.0, 1.0);
	vec4 color = vec4(mix(uFogColor, litColor, fog), base.a * uAlpha);

	float weight = clamp(
		color.a * color.a * 0.03 / (1e-5 + pow(gl_FragCoord.z / 200.0, 4.0)),
		0.01, 3000.0);
	outAccum = vec4(color.rgb * color.a * weight, color.a * weight);
	outReveal = color.a;
}` + "\x00"

// oitResolveVertexShaderGL emits a full-screen triangle from gl_VertexID
// without any vertex buffer.
const oitResolveVertexShaderGL = `#version 410 core

void main() {
	ivec2 v = ivec2(gl_VertexID & 1, gl_VertexID >> 1);
	gl_Position = vec4(vec2(v) * 4.0 - 1.0, 0.0, 1.0);
}` + "\x00"

// oitResolveFragmentShaderGL composites weighted blended OIT accumulation
// and revealage textures into the final pre-multiplied RGBA color.
const oitResolveFragmentShaderGL = `#version 410 core

uniform sampler2D uTexAccum;
uniform sampler2D uTexReveal;

out vec4 fragColor;

void main() {
	ivec2 coord = ivec2(gl_FragCoord.xy);
	vec4 accum = texelFetch(uTexAccum, coord, 0);
	float reveal = texelFetch(uTexReveal, coord, 0).r;
	fragColor = vec4(accum.rgb / max(accum.a, 1e-5), 1.0 - reveal);
}` + "\x00"

// oitResolveMSAAFragmentShaderGL resolves multisample OIT textures by averaging
// across samples before compositing.
const oitResolveMSAAFragmentShaderGL = `#version 410 core

uniform sampler2DMS uTexAccum;
uniform sampler2DMS uTexReveal;
uniform int uSamples;

out vec4 fragColor;

void main() {
	ivec2 coord = ivec2(gl_FragCoord.xy);
	vec4 accum = vec4(0.0);
	float reveal = 0.0;
	for (int i = 0; i < uSamples; i++) {
		accum += texelFetch(uTexAccum, coord, i);
		reveal += texelFetch(uTexReveal, coord, i).r;
	}
	accum /= float(uSamples);
	reveal /= float(uSamples);
	fragColor = vec4(accum.rgb / max(accum.a, 1e-5), 1.0 - reveal);
}` + "\x00"

// ensureOITShaders lazily compiles the OIT world and resolve shader programs
// and caches their uniform locations.
func (r *Renderer) ensureOITShaders() error {
	if r.oitWorldProgram != 0 && r.oitResolveProgram != 0 && r.oitResolveMSAAProgram != 0 {
		return nil
	}

	// --- OIT world program (reuses world backend vertex shader) ---
	if r.oitWorldProgram == 0 {
		vs, err := compileShader(worldopengl.WorldVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile OIT world vertex shader: %w", err)
		}
		fs, err := compileShader(oitWorldFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile OIT world fragment shader: %w", err)
		}
		prog := createProgram(vs, fs)
		r.oitWorldProgram = prog
		r.oitWorldVPUniform = gl.GetUniformLocation(prog, gl.Str("uViewProjection\x00"))
		r.oitWorldTextureUniform = gl.GetUniformLocation(prog, gl.Str("uTexture\x00"))
		r.oitWorldLightmapUniform = gl.GetUniformLocation(prog, gl.Str("uLightmap\x00"))
		r.oitWorldFullbrightUniform = gl.GetUniformLocation(prog, gl.Str("uFullbright\x00"))
		r.oitWorldHasFullbrightUniform = gl.GetUniformLocation(prog, gl.Str("uHasFullbright\x00"))
		r.oitWorldDynamicLightUniform = gl.GetUniformLocation(prog, gl.Str("uDynamicLight\x00"))
		r.oitWorldModelOffsetUniform = gl.GetUniformLocation(prog, gl.Str("uModelOffset\x00"))
		r.oitWorldModelRotationUniform = gl.GetUniformLocation(prog, gl.Str("uModelRotation\x00"))
		r.oitWorldModelScaleUniform = gl.GetUniformLocation(prog, gl.Str("uModelScale\x00"))
		r.oitWorldAlphaUniform = gl.GetUniformLocation(prog, gl.Str("uAlpha\x00"))
		r.oitWorldTimeUniform = gl.GetUniformLocation(prog, gl.Str("uTime\x00"))
		r.oitWorldTurbulentUniform = gl.GetUniformLocation(prog, gl.Str("uTurbulent\x00"))
		r.oitWorldLitWaterUniform = gl.GetUniformLocation(prog, gl.Str("uLitWater\x00"))
		r.oitWorldCameraOriginUniform = gl.GetUniformLocation(prog, gl.Str("uCameraOrigin\x00"))
		r.oitWorldFogColorUniform = gl.GetUniformLocation(prog, gl.Str("uFogColor\x00"))
		r.oitWorldFogDensityUniform = gl.GetUniformLocation(prog, gl.Str("uFogDensity\x00"))
	}

	// --- OIT resolve program ---
	if r.oitResolveProgram == 0 {
		vs, err := compileShader(oitResolveVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile OIT resolve vertex shader: %w", err)
		}
		fs, err := compileShader(oitResolveFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile OIT resolve fragment shader: %w", err)
		}
		prog := createProgram(vs, fs)
		r.oitResolveProgram = prog
		r.oitResolveAccumLoc = gl.GetUniformLocation(prog, gl.Str("uTexAccum\x00"))
		r.oitResolveRevealLoc = gl.GetUniformLocation(prog, gl.Str("uTexReveal\x00"))

		// Sampler bindings are constant; set once.
		gl.UseProgram(prog)
		gl.Uniform1i(r.oitResolveAccumLoc, 0)
		gl.Uniform1i(r.oitResolveRevealLoc, 1)
		gl.UseProgram(0)
	}

	// --- OIT resolve program (MSAA) ---
	if r.oitResolveMSAAProgram == 0 {
		vs, err := compileShader(oitResolveVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile OIT resolve MSAA vertex shader: %w", err)
		}
		fs, err := compileShader(oitResolveMSAAFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile OIT resolve MSAA fragment shader: %w", err)
		}
		prog := createProgram(vs, fs)
		r.oitResolveMSAAProgram = prog
		accumLoc := gl.GetUniformLocation(prog, gl.Str("uTexAccum\x00"))
		revealLoc := gl.GetUniformLocation(prog, gl.Str("uTexReveal\x00"))
		r.oitResolveMSAASamplesLoc = gl.GetUniformLocation(prog, gl.Str("uSamples\x00"))

		// Sampler bindings are constant; set once.
		gl.UseProgram(prog)
		gl.Uniform1i(accumLoc, 0)
		gl.Uniform1i(revealLoc, 1)
		gl.UseProgram(0)
	}

	// --- Empty VAO for core-profile full-screen draw ---
	if r.oitResolveVAO == 0 {
		gl.GenVertexArrays(1, &r.oitResolveVAO)
	}

	return nil
}

// destroyOITShaders releases the OIT shader programs and resolve VAO.
func (r *Renderer) destroyOITShaders() {
	if r.oitWorldProgram != 0 {
		gl.DeleteProgram(r.oitWorldProgram)
		r.oitWorldProgram = 0
	}
	if r.oitResolveProgram != 0 {
		gl.DeleteProgram(r.oitResolveProgram)
		r.oitResolveProgram = 0
	}
	if r.oitResolveMSAAProgram != 0 {
		gl.DeleteProgram(r.oitResolveMSAAProgram)
		r.oitResolveMSAAProgram = 0
	}
	if r.oitResolveVAO != 0 {
		gl.DeleteVertexArrays(1, &r.oitResolveVAO)
		r.oitResolveVAO = 0
	}
}
