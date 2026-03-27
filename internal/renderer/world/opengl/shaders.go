//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

const (
	WorldVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec2 aLightmapCoord;
layout(location = 3) in vec3 aNormal;

uniform mat4 uViewProjection;
uniform vec3 uModelOffset;
uniform mat4 uModelRotation;
uniform float uModelScale;

out vec2 vTexCoord;
out vec2 vLightmapCoord;
out vec3 vNormal;
out vec3 vWorldPos;

void main() {
	vTexCoord = aTexCoord;
	vLightmapCoord = aLightmapCoord;
	vec3 worldPosition = (uModelRotation * vec4(aPosition * uModelScale, 1.0)).xyz + uModelOffset;
	vNormal = (uModelRotation * vec4(aNormal, 0.0)).xyz;
	vWorldPos = worldPosition;
	gl_Position = uViewProjection * vec4(worldPosition, 1.0);
}`

	WorldFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec2 vLightmapCoord;
in vec3 vNormal;
in vec3 vWorldPos;
out vec4 fragColor;

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
	vec3 color = base.rgb * light * 2.0;
	if (uHasFullbright > 0.5) {
		vec4 fb = texture(uFullbright, uv);
		color += fb.rgb;
	}
	color = mix(color, uFogColor, uFogDensity);
	fragColor = vec4(color, base.a * uAlpha);
}`
)

const (
	WorldSkyVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;

uniform mat4 uViewProjection;
uniform vec3 uModelOffset;
uniform mat4 uModelRotation;
uniform float uModelScale;
uniform vec3 uCameraOrigin;

out vec3 vDir;

void main() {
	vec3 worldPosition = (uModelRotation * vec4(aPosition * uModelScale, 1.0)).xyz + uModelOffset;
	vDir = worldPosition - uCameraOrigin;
	vDir.z *= 3.0;
	gl_Position = uViewProjection * vec4(worldPosition, 1.0);
}`

	WorldSkyFragmentShaderGL = `#version 410 core
in vec3 vDir;
out vec4 fragColor;

uniform sampler2D uSolidLayer;
uniform sampler2D uAlphaLayer;
uniform float uTime;
uniform float uSolidLayerSpeed;
uniform float uAlphaLayerSpeed;
uniform vec3 uFogColor;
uniform float uFogDensity;

void main() {
	vec3 dir = normalize(vDir);
	vec2 uv = dir.xy * (189.0 / 64.0);
	vec4 result = texture(uSolidLayer, uv + vec2((uTime / 16.0) * uSolidLayerSpeed));
	vec4 layer = texture(uAlphaLayer, uv + vec2((uTime / 8.0) * uAlphaLayerSpeed));
	result.rgb = mix(result.rgb, layer.rgb, layer.a);
	result.rgb = mix(result.rgb, uFogColor, uFogDensity);
	fragColor = result;
}`

	WorldSkyProceduralFragmentShaderGL = `#version 410 core
in vec3 vDir;
out vec4 fragColor;

uniform vec3 uHorizonColor;
uniform vec3 uZenithColor;
uniform vec3 uFogColor;
uniform float uFogDensity;

void main() {
	vec3 dir = normalize(vDir);
	float gradient = clamp(dir.z * 0.5 + 0.5, 0.0, 1.0);
	gradient = gradient * gradient;
	vec3 result = mix(uHorizonColor, uZenithColor, gradient);
	result = mix(result, uFogColor, uFogDensity);
	fragColor = vec4(result, 1.0);
}`

	WorldSkyCubemapVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;

uniform mat4 uViewProjection;
uniform vec3 uModelOffset;
uniform mat4 uModelRotation;
uniform float uModelScale;
uniform vec3 uCameraOrigin;

out vec3 vDir;

void main() {
	vec3 worldPosition = (uModelRotation * vec4(aPosition * uModelScale, 1.0)).xyz + uModelOffset;
	vec3 eyeDelta = worldPosition - uCameraOrigin;
	vDir.x = -eyeDelta.y;
	vDir.y = eyeDelta.z;
	vDir.z = eyeDelta.x;
	gl_Position = uViewProjection * vec4(worldPosition, 1.0);
}`

	WorldSkyCubemapFragmentShaderGL = `#version 410 core
in vec3 vDir;
out vec4 fragColor;

uniform samplerCube uCubeMap;
uniform vec3 uFogColor;
uniform float uFogDensity;

void main() {
	vec4 result = texture(uCubeMap, vDir);
	result.rgb = mix(result.rgb, uFogColor, uFogDensity);
	fragColor = result;
}`

	WorldSkyExternalFaceFragmentShaderGL = `#version 410 core
in vec3 vDir;
out vec4 fragColor;

uniform sampler2D uSkyRT;
uniform sampler2D uSkyBK;
uniform sampler2D uSkyLF;
uniform sampler2D uSkyFT;
uniform sampler2D uSkyUP;
uniform sampler2D uSkyDN;
uniform vec3 uFogColor;
uniform float uFogDensity;

vec4 sampleExternalSky(vec3 dir) {
	vec3 absDir = abs(dir);
	float ma;
	vec2 uv;
	vec4 sampleColor;
	if (absDir.x >= absDir.y && absDir.x >= absDir.z) {
		ma = absDir.x;
		if (dir.x > 0.0) {
			uv = vec2((-dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyFT, uv);
		} else {
			uv = vec2((dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyBK, uv);
		}
	} else if (absDir.y >= absDir.x && absDir.y >= absDir.z) {
		ma = absDir.y;
		if (dir.y > 0.0) {
			uv = vec2((dir.x / ma + 1.0) * 0.5, (dir.z / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyUP, uv);
		} else {
			uv = vec2((dir.x / ma + 1.0) * 0.5, (-dir.z / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyDN, uv);
		}
	} else {
		ma = absDir.z;
		if (dir.z > 0.0) {
			uv = vec2((dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyRT, uv);
		} else {
			uv = vec2((-dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
			sampleColor = texture(uSkyLF, uv);
		}
	}
	return sampleColor;
}

void main() {
	vec4 result = sampleExternalSky(normalize(vDir));
	result.rgb = mix(result.rgb, uFogColor, uFogDensity);
	fragColor = result;
}`
)
