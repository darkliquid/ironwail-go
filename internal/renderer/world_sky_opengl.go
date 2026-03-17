//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"log/slog"
	"strings"
	"unsafe"
)

const (
	worldVertexShaderGL = `#version 410 core
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

	worldFragmentShaderGL = `#version 410 core
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

void main() {
	vec2 uv = vTexCoord;
	if (uTurbulent > 0.5) {
		uv = uv * 2.0 + 0.125 * sin(uv.yx * (3.14159265 * 2.0) + uTime);
	}
	vec4 base = texture(uTexture, uv);
	vec3 light = texture(uLightmap, vLightmapCoord).rgb + uDynamicLight;
	if (base.a < 0.1) {
		discard;
	}
	vec3 color = base.rgb * light * 2.0;
	
	// Add fullbright contribution (additive blend)
	if (uHasFullbright > 0.5) {
		vec4 fb = texture(uFullbright, uv);
		color = color + fb.rgb * fb.a;
	}
	
	vec3 fogPosition = vWorldPos - uCameraOrigin;
	float fog = exp2(-uFogDensity * dot(fogPosition, fogPosition));
	fog = clamp(fog, 0.0, 1.0);
	fragColor = vec4(mix(uFogColor, color, fog), base.a * uAlpha);
}`

	worldSkyVertexShaderGL = `#version 410 core
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

	worldSkyFragmentShaderGL = `#version 410 core
in vec3 vDir;
out vec4 fragColor;

uniform sampler2D uSolidLayer;
uniform sampler2D uAlphaLayer;
uniform float uTime;
uniform vec3 uFogColor;
uniform float uFogDensity;

void main() {
	vec3 dir = normalize(vDir);
	vec2 uv = dir.xy * (189.0 / 64.0);
	vec4 result = texture(uSolidLayer, uv + vec2(uTime / 16.0));
	vec4 layer = texture(uAlphaLayer, uv + vec2(uTime / 8.0));
	result.rgb = mix(result.rgb, layer.rgb, layer.a);
	result.rgb = mix(result.rgb, uFogColor, uFogDensity);
	fragColor = result;
}`

	worldSkyCubemapVertexShaderGL = `#version 410 core
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

	worldSkyCubemapFragmentShaderGL = `#version 410 core
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

	worldSkyExternalFaceFragmentShaderGL = `#version 410 core
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

type worldSkyFogOverride struct {
	hasValue bool
	value    float32
}

// ensureWorldSkyPrograms lazily compiles all three sky shader variants: embedded two-layer scrolling sky, cubemap sky (GL_TEXTURE_CUBE_MAP for external skybox), and individual-face sky (fallback for non-uniform face sizes).
func (r *Renderer) ensureWorldSkyPrograms() error {
	if r.worldSkyProgram == 0 {
		vs, err := compileShader(worldSkyVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky vertex shader: %w", err)
		}
		fs, err := compileShader(worldSkyFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile world sky fragment shader: %w", err)
		}

		program := createProgram(vs, fs)
		r.worldSkyProgram = program
		r.worldSkyVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
		r.worldSkySolidUniform = gl.GetUniformLocation(program, gl.Str("uSolidLayer\x00"))
		r.worldSkyAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlphaLayer\x00"))
		r.worldSkyModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
		r.worldSkyModelRotationUniform = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
		r.worldSkyModelScaleUniform = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
		r.worldSkyTimeUniform = gl.GetUniformLocation(program, gl.Str("uTime\x00"))
		r.worldSkyCameraOriginUniform = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
		r.worldSkyFogColorUniform = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
		r.worldSkyFogDensityUniform = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
	}

	if r.worldSkyCubemapProgram == 0 {
		vsCubemap, err := compileShader(worldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky cubemap vertex shader: %w", err)
		}
		fsCubemap, err := compileShader(worldSkyCubemapFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vsCubemap)
			return fmt.Errorf("compile world sky cubemap fragment shader: %w", err)
		}
		cubemapProgram := createProgram(vsCubemap, fsCubemap)
		r.worldSkyCubemapProgram = cubemapProgram
		r.worldSkyCubemapVPUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uViewProjection\x00"))
		r.worldSkyCubemapUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uCubeMap\x00"))
		r.worldSkyCubemapModelOffsetUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelOffset\x00"))
		r.worldSkyCubemapModelRotationUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelRotation\x00"))
		r.worldSkyCubemapModelScaleUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelScale\x00"))
		r.worldSkyCubemapCameraOriginUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uCameraOrigin\x00"))
		r.worldSkyCubemapFogColorUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uFogColor\x00"))
		r.worldSkyCubemapFogDensityUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uFogDensity\x00"))
	}
	if r.worldSkyExternalFaceProgram == 0 {
		vsExternalFaces, err := compileShader(worldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky external-face vertex shader: %w", err)
		}
		fsExternalFaces, err := compileShader(worldSkyExternalFaceFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vsExternalFaces)
			return fmt.Errorf("compile world sky external-face fragment shader: %w", err)
		}
		externalFaceProgram := createProgram(vsExternalFaces, fsExternalFaces)
		r.worldSkyExternalFaceProgram = externalFaceProgram
		r.worldSkyExternalFaceVPUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uViewProjection\x00"))
		r.worldSkyExternalFaceRTUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyRT\x00"))
		r.worldSkyExternalFaceBKUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyBK\x00"))
		r.worldSkyExternalFaceLFUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyLF\x00"))
		r.worldSkyExternalFaceFTUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyFT\x00"))
		r.worldSkyExternalFaceUPUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyUP\x00"))
		r.worldSkyExternalFaceDNUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyDN\x00"))
		r.worldSkyExternalFaceModelOffset = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelOffset\x00"))
		r.worldSkyExternalFaceModelRotation = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelRotation\x00"))
		r.worldSkyExternalFaceModelScale = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelScale\x00"))
		r.worldSkyExternalFaceCameraOrigin = gl.GetUniformLocation(externalFaceProgram, gl.Str("uCameraOrigin\x00"))
		r.worldSkyExternalFaceFogColor = gl.GetUniformLocation(externalFaceProgram, gl.Str("uFogColor\x00"))
		r.worldSkyExternalFaceFogDensity = gl.GetUniformLocation(externalFaceProgram, gl.Str("uFogDensity\x00"))
	}
	return nil
}

// shouldSplitAsQuake64Sky detects Quake 64 remaster sky textures which use different splitting dimensions than standard Quake.
func shouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return bsp.IsQuake64(treeVersion) || (width == 32 && height == 64)
}

// indexedOpaqueToRGBA converts palette-indexed pixels to RGBA, treating all pixels as fully opaque. Used for sky solid layers and other non-transparent textures.
func indexedOpaqueToRGBA(pixels []byte, palette []byte) []byte {
	rgba := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		r, g, b := GetPaletteColor(p, palette)
		rgba[i*4] = r
		rgba[i*4+1] = g
		rgba[i*4+2] = b
		rgba[i*4+3] = 255
	}
	return rgba
}

// extractEmbeddedSkyLayers splits a Quake sky texture into solid and alpha layers. Sky textures are double-width: the left half is the foreground (scrolls faster, transparent areas reveal background), the right half is the background (scrolls slower). Both scroll independently for a parallax cloud effect.
func extractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	if width <= 0 || height <= 0 || len(pixels) < width*height {
		return nil, nil, 0, 0, false
	}

	if quake64 {
		if height < 2 {
			return nil, nil, 0, 0, false
		}
		halfHeight := height / 2
		if halfHeight <= 0 {
			return nil, nil, 0, 0, false
		}
		layerWidth = width
		layerHeight = halfHeight
		layerSize := layerWidth * layerHeight
		front := pixels[:layerSize]
		back := pixels[layerSize : layerSize*2]
		solidRGBA = indexedOpaqueToRGBA(back, palette)
		alphaRGBA = make([]byte, layerSize*4)
		for i, p := range front {
			r, g, b := GetPaletteColor(p, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 128
		}
		return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
	}

	if width < 2 {
		return nil, nil, 0, 0, false
	}
	halfWidth := width / 2
	if halfWidth <= 0 {
		return nil, nil, 0, 0, false
	}
	layerWidth = halfWidth
	layerHeight = height
	layerSize := layerWidth * layerHeight
	backIndexed := make([]byte, layerSize)
	frontIndexed := make([]byte, layerSize)
	for y := 0; y < height; y++ {
		row := y * width
		copy(backIndexed[y*halfWidth:(y+1)*halfWidth], pixels[row+halfWidth:row+width])
		copy(frontIndexed[y*halfWidth:(y+1)*halfWidth], pixels[row:row+halfWidth])
	}
	solidRGBA = indexedOpaqueToRGBA(backIndexed, palette)
	alphaRGBA = make([]byte, layerSize*4)
	for i, p := range frontIndexed {
		if p == 0 || p == 255 {
			r, g, b := GetPaletteColor(255, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 0
			continue
		}
		r, g, b := GetPaletteColor(p, palette)
		alphaRGBA[i*4] = r
		alphaRGBA[i*4+1] = g
		alphaRGBA[i*4+2] = b
		alphaRGBA[i*4+3] = 255
	}
	return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
}

// uploadSkyboxCubemap uploads 6 skybox face images as a GL_TEXTURE_CUBE_MAP, reordering faces from Quake convention (rt/bk/lf/ft/up/dn) to OpenGL convention (+X/-X/+Y/-Y/+Z/-Z).
func uploadSkyboxCubemap(faces [6]externalSkyboxFace, faceSize int) uint32 {
	if faceSize <= 0 {
		return 0
	}
	var cubemap uint32
	gl.GenTextures(1, &cubemap)
	if cubemap == 0 {
		return 0
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubemap)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
	zeroFace := make([]byte, faceSize*faceSize*4)
	for i, target := range skyboxCubemapTargets {
		face := faces[skyboxCubemapFaceOrder[i]]
		faceData := zeroFace
		if face.Width > 0 && face.Height > 0 && len(face.RGBA) > 0 {
			if face.Width != faceSize || face.Height != faceSize || len(face.RGBA) != faceSize*faceSize*4 {
				gl.DeleteTextures(1, &cubemap)
				gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
				return 0
			}
			faceData = face.RGBA
		}
		if len(faceData) != faceSize*faceSize*4 {
			gl.DeleteTextures(1, &cubemap)
			gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
			return 0
		}
		gl.TexImage2D(target, 0, gl.RGBA8, int32(faceSize), int32(faceSize), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(faceData))
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
	return cubemap
}

// uploadSkyboxFaceTextures uploads each skybox face as an individual GL_TEXTURE_2D, used as fallback when faces aren't all square and can't form a cubemap.
func uploadSkyboxFaceTextures(faces [6]externalSkyboxFace) (textures [6]uint32, ok bool) {
	fallbackPixel := [4]byte{0, 0, 0, 255}
	for i := range textures {
		gl.GenTextures(1, &textures[i])
		if textures[i] == 0 {
			for j := 0; j < i; j++ {
				if textures[j] != 0 {
					gl.DeleteTextures(1, &textures[j])
					textures[j] = 0
				}
			}
			return textures, false
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		face := faces[i]
		width := face.Width
		height := face.Height
		data := face.RGBA
		if width <= 0 || height <= 0 || len(data) != width*height*4 {
			width, height = 1, 1
			data = fallbackPixel[:]
		}
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(data))
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return textures, true
}

// clearExternalSkyboxLocked deletes external skybox GL textures and resets to the embedded sky rendering mode.
func (r *Renderer) clearExternalSkyboxLocked() {
	if r.worldSkyExternalCubemap != 0 {
		gl.DeleteTextures(1, &r.worldSkyExternalCubemap)
		r.worldSkyExternalCubemap = 0
	}
	for i := range r.worldSkyExternalFaceTextures {
		if r.worldSkyExternalFaceTextures[i] != 0 {
			gl.DeleteTextures(1, &r.worldSkyExternalFaceTextures[i])
			r.worldSkyExternalFaceTextures[i] = 0
		}
	}
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalName = ""
}

// SetExternalSkybox loads an external skybox by name, attempting cubemap first and falling back to individual face textures.
func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {
	normalized := normalizeSkyboxBaseName(name)

	r.mu.Lock()
	r.worldSkyExternalRequestID++
	requestID := r.worldSkyExternalRequestID
	if normalized == r.worldSkyExternalName {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	faces, loaded := loadExternalSkyboxFaces(normalized, loadFile)
	faceSize, cubemapEligible := externalSkyboxCubemapFaceSize(faces, loaded)
	renderMode := selectExternalSkyboxRenderMode(loaded, cubemapEligible)

	r.mu.Lock()
	defer r.mu.Unlock()
	if requestID != r.worldSkyExternalRequestID {
		return
	}

	r.clearExternalSkyboxLocked()
	if normalized == "" || renderMode == externalSkyboxRenderEmbedded {
		return
	}
	if renderMode == externalSkyboxRenderCubemap {
		cubemap := uploadSkyboxCubemap(faces, faceSize)
		if cubemap == 0 {
			slog.Debug("external skybox cubemap upload failed; falling back to embedded sky", "name", normalized)
			return
		}
		r.worldSkyExternalCubemap = cubemap
		r.worldSkyExternalMode = externalSkyboxRenderCubemap
		r.worldSkyExternalName = normalized
		return
	}
	faceTextures, ok := uploadSkyboxFaceTextures(faces)
	if !ok {
		slog.Debug("external skybox face upload failed; falling back to embedded sky", "name", normalized)
		return
	}
	r.worldSkyExternalFaceTextures = faceTextures
	r.worldSkyExternalMode = externalSkyboxRenderFaces
	r.worldSkyExternalName = normalized
}

// parseWorldspawnSkyFogOverride parses the worldspawn entity for sky fog override values.
func parseWorldspawnSkyFogOverride(entities []byte) worldSkyFogOverride {
	if len(entities) == 0 {
		return worldSkyFogOverride{}
	}

	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return worldSkyFogOverride{}
	}

	fields := parseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return worldSkyFogOverride{}
	}

	value, ok := parseEntityAlphaField(fields, "skyfog")
	if !ok {
		return worldSkyFogOverride{}
	}

	return worldSkyFogOverride{hasValue: true, value: value}
}

// readWorldSkyFogCvar reads the r_skyfog cvar value with a fallback default.
func readWorldSkyFogCvar(fallback float32) float32 {
	return readWorldAlphaCvar(CvarRSkyFog, fallback)
}

// resolveWorldSkyFogMix resolves the final sky fog mix factor from the cvar value, worldspawn override, and fog density.
func resolveWorldSkyFogMix(cvarValue float32, override worldSkyFogOverride, fogDensity float32) float32 {
	if fogDensity <= 0 {
		return 0
	}
	skyFog := clamp01(cvarValue)
	if override.hasValue {
		skyFog = clamp01(override.value)
	}
	return skyFog
}

type skyPassState struct {
	program                     uint32
	cubemapProgram              uint32
	externalFaceProgram         uint32
	vpUniform                   int32
	solidUniform                int32
	alphaUniform                int32
	cubemapVPUniform            int32
	cubemapUniform              int32
	externalFaceVPUniform       int32
	externalFaceRTUniform       int32
	externalFaceBKUniform       int32
	externalFaceLFUniform       int32
	externalFaceFTUniform       int32
	externalFaceUPUniform       int32
	externalFaceDNUniform       int32
	modelOffsetUniform          int32
	modelRotationUniform        int32
	modelScaleUniform           int32
	cubemapModelOffsetUniform   int32
	cubemapModelRotationUniform int32
	cubemapModelScaleUniform    int32
	externalFaceModelOffset     int32
	externalFaceModelRotation   int32
	externalFaceModelScale      int32
	timeUniform                 int32
	cameraOriginUniform         int32
	cubemapCameraOriginUniform  int32
	externalFaceCameraOrigin    int32
	fogColorUniform             int32
	cubemapFogColorUniform      int32
	externalFaceFogColor        int32
	fogDensityUniform           int32
	cubemapFogDensityUniform    int32
	externalFaceFogDensity      int32
	vp                          [16]float32
	time                        float32
	cameraOrigin                [3]float32
	modelOffset                 [3]float32
	modelRotation               [16]float32
	modelScale                  float32
	fogColor                    [3]float32
	fogDensity                  float32
	solidTextures               map[int32]uint32
	alphaTextures               map[int32]uint32
	textureAnimations           []*SurfaceTexture
	fallbackSolid               uint32
	fallbackAlpha               uint32
	externalSkyMode             externalSkyboxRenderMode
	externalCubemap             uint32
	externalFaceTextures        [6]uint32
	frame                       int
}

// worldSkyTexturesForFace resolves the solid and alpha sky layer texture handles for a sky face, with animation support.
func worldSkyTexturesForFace(face WorldFace, solidTextures, alphaTextures map[int32]uint32, textureAnimations []*SurfaceTexture, fallbackSolid, fallbackAlpha uint32, frame int, timeSeconds float64) (solid, alpha uint32) {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}

	solid = solidTextures[textureIndex]
	alpha = alphaTextures[textureIndex]
	if (solid == 0 || alpha == 0) && textureIndex != face.TextureIndex {
		if solid == 0 {
			solid = solidTextures[face.TextureIndex]
		}
		if alpha == 0 {
			alpha = alphaTextures[face.TextureIndex]
		}
	}
	if solid == 0 {
		solid = fallbackSolid
	}
	if alpha == 0 {
		alpha = fallbackAlpha
	}
	return solid, alpha
}

// renderSkyPass renders sky surfaces using one of three sky shader programs: embedded two-layer scrolling sky, cubemap sky, or individual face textures. Draws sky as a backdrop with depth clamped to the far plane.
func renderSkyPass(calls []worldDrawCall, state skyPassState) {
	if len(calls) == 0 {
		return
	}
	useCubemap := state.externalSkyMode == externalSkyboxRenderCubemap && state.externalCubemap != 0
	useExternalFaces := state.externalSkyMode == externalSkyboxRenderFaces
	if useCubemap {
		if state.cubemapProgram == 0 {
			return
		}
		gl.UseProgram(state.cubemapProgram)
		gl.UniformMatrix4fv(state.cubemapVPUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.cubemapUniform, 2)
		gl.Uniform3f(state.cubemapModelOffsetUniform, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.cubemapModelRotationUniform, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.cubemapModelScaleUniform, state.modelScale)
		gl.Uniform3f(state.cubemapCameraOriginUniform, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.cubemapFogColorUniform, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.cubemapFogDensityUniform, state.fogDensity)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, state.externalCubemap)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		if state.externalFaceProgram == 0 {
			return
		}
		gl.UseProgram(state.externalFaceProgram)
		gl.UniformMatrix4fv(state.externalFaceVPUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.externalFaceRTUniform, 2)
		gl.Uniform1i(state.externalFaceBKUniform, 3)
		gl.Uniform1i(state.externalFaceLFUniform, 4)
		gl.Uniform1i(state.externalFaceFTUniform, 5)
		gl.Uniform1i(state.externalFaceUPUniform, 6)
		gl.Uniform1i(state.externalFaceDNUniform, 7)
		gl.Uniform3f(state.externalFaceModelOffset, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.externalFaceModelRotation, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.externalFaceModelScale, state.modelScale)
		gl.Uniform3f(state.externalFaceCameraOrigin, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.externalFaceFogColor, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.externalFaceFogDensity, state.fogDensity)
		for i, texture := range state.externalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, texture)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	} else {
		if state.program == 0 {
			return
		}
		gl.UseProgram(state.program)
		gl.UniformMatrix4fv(state.vpUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.solidUniform, 0)
		gl.Uniform1i(state.alphaUniform, 1)
		gl.Uniform3f(state.modelOffsetUniform, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.modelRotationUniform, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.modelScaleUniform, state.modelScale)
		gl.Uniform1f(state.timeUniform, state.time)
		gl.Uniform3f(state.cameraOriginUniform, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.fogColorUniform, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.fogDensityUniform, state.fogDensity)
	}

	// Sky is rendered at maximum depth but doesn't write to depth buffer
	gl.DepthFunc(gl.LEQUAL)
	gl.DepthMask(false)
	gl.Disable(gl.BLEND)

	for _, call := range calls {
		if !useCubemap && !useExternalFaces {
			solid, alpha := worldSkyTexturesForFace(
				call.face,
				state.solidTextures,
				state.alphaTextures,
				state.textureAnimations,
				state.fallbackSolid,
				state.fallbackAlpha,
				state.frame,
				float64(state.time),
			)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, solid)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, alpha)
			gl.ActiveTexture(gl.TEXTURE0)
		}
		gl.DrawElements(gl.TRIANGLES, int32(call.face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(call.face.FirstIndex*4)))
	}
	if useCubemap {
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		for i := range state.externalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, 0)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	}

	// Restore depth state
	gl.DepthFunc(gl.LESS)
	gl.DepthMask(true)
}

// ensureWorldSkyFallbackTexturesLocked creates fallback sky textures: dark blue for the solid layer, transparent black for the alpha layer.
func (r *Renderer) ensureWorldSkyFallbackTexturesLocked() {
	r.ensureWorldFallbackTextureLocked()
	if r.worldSkyAlphaFallback != 0 {
		return
	}
	r.worldSkyAlphaFallback = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 0})
}
