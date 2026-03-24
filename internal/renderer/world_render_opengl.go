//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	"sort"
	"unsafe"
)

type worldDrawCall struct {
	face              WorldFace
	texture           uint32
	fullbrightTexture uint32
	lightmap          uint32
	alpha             float32
	turbulent         bool
	hasLitWater       bool
	distanceSq        float32
	light             [3]float32
	vao               uint32
	modelOffset       [3]float32
	modelRotation     [16]float32
	modelScale        float32
}

// ensureWorldProgram lazily compiles the world rendering shader program. The world shader performs multi-texture rendering: diffuse texture * lightmap, with optional fullbright overlay and dynamic light contribution.
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
	r.worldTextureUniform = gl.GetUniformLocation(program, gl.Str("uTexture\x00"))
	r.worldLightmapUniform = gl.GetUniformLocation(program, gl.Str("uLightmap\x00"))
	r.worldFullbrightUniform = gl.GetUniformLocation(program, gl.Str("uFullbright\x00"))
	r.worldHasFullbrightUniform = gl.GetUniformLocation(program, gl.Str("uHasFullbright\x00"))
	r.worldDynamicLightUniform = gl.GetUniformLocation(program, gl.Str("uDynamicLight\x00"))
	r.worldModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
	r.worldModelRotationUniform = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
	r.worldModelScaleUniform = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
	r.worldAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlpha\x00"))
	r.worldTimeUniform = gl.GetUniformLocation(program, gl.Str("uTime\x00"))
	r.worldTurbulentUniform = gl.GetUniformLocation(program, gl.Str("uTurbulent\x00"))
	r.worldLitWaterUniform = gl.GetUniformLocation(program, gl.Str("uLitWater\x00"))
	r.worldCameraOriginUniform = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
	r.worldFogColorUniform = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
	r.worldFogDensityUniform = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
	return nil
}

// lightstyleScale looks up a lightstyle's current brightness from the 64-element value array. The 255 sentinel (no light) returns 0.
func lightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

// setFogState updates the fog color and density values used by world and sky shader fog calculations.
func (r *Renderer) setFogState(color [3]float32, density float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.worldFogColor = color
	r.worldFogDensity = density
}

// renderWorld renders the world BSP geometry using the specified pass selector. Binds the world shader, sets the view-projection matrix and camera uniforms, buckets faces by type (sky, opaque, liquid, translucent), and issues draw calls with per-face diffuse + lightmap + fullbright texture binds.
func (r *Renderer) renderWorld(selector worldBrushPassSelector) {
	selector = normalizeWorldBrushPassSelector(selector)
	drawSky := selector.includesSky()
	drawNonLiquid := selector.includesNonLiquid()
	drawLiquidOpaque := selector.includesLiquidOpaque()
	drawLiquidTranslucent := selector.includesLiquidTranslucent()

	r.mu.RLock()
	program := r.worldProgram
	skyProgram := r.worldSkyProgram
	skyCubemapProgram := r.worldSkyCubemapProgram
	skyExternalFaceProgram := r.worldSkyExternalFaceProgram
	vao := r.worldVAO
	indexCount := r.worldIndexCount
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	fullbrightUniform := r.worldFullbrightUniform
	hasFullbrightUniform := r.worldHasFullbrightUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	skyVPUniform := r.worldSkyVPUniform
	skySolidUniform := r.worldSkySolidUniform
	skyAlphaUniform := r.worldSkyAlphaUniform
	skyCubemapVPUniform := r.worldSkyCubemapVPUniform
	skyCubemapUniform := r.worldSkyCubemapUniform
	skyExternalFaceVPUniform := r.worldSkyExternalFaceVPUniform
	skyExternalFaceRTUniform := r.worldSkyExternalFaceRTUniform
	skyExternalFaceBKUniform := r.worldSkyExternalFaceBKUniform
	skyExternalFaceLFUniform := r.worldSkyExternalFaceLFUniform
	skyExternalFaceFTUniform := r.worldSkyExternalFaceFTUniform
	skyExternalFaceUPUniform := r.worldSkyExternalFaceUPUniform
	skyExternalFaceDNUniform := r.worldSkyExternalFaceDNUniform
	skyModelOffsetUniform := r.worldSkyModelOffsetUniform
	skyModelRotationUniform := r.worldSkyModelRotationUniform
	skyModelScaleUniform := r.worldSkyModelScaleUniform
	skyCubemapModelOffsetUniform := r.worldSkyCubemapModelOffsetUniform
	skyCubemapModelRotationUniform := r.worldSkyCubemapModelRotationUniform
	skyCubemapModelScaleUniform := r.worldSkyCubemapModelScaleUniform
	skyExternalFaceModelOffsetUniform := r.worldSkyExternalFaceModelOffset
	skyExternalFaceModelRotationUniform := r.worldSkyExternalFaceModelRotation
	skyExternalFaceModelScaleUniform := r.worldSkyExternalFaceModelScale
	skyTimeUniform := r.worldSkyTimeUniform
	skySolidLayerSpeedUniform := r.worldSkySolidLayerSpeedUniform
	skyAlphaLayerSpeedUniform := r.worldSkyAlphaLayerSpeedUniform
	skyCameraOriginUniform := r.worldSkyCameraOriginUniform
	skyCubemapCameraOriginUniform := r.worldSkyCubemapCameraOriginUniform
	skyExternalFaceCameraOriginUniform := r.worldSkyExternalFaceCameraOrigin
	skyFogColorUniform := r.worldSkyFogColorUniform
	skyCubemapFogColorUniform := r.worldSkyCubemapFogColorUniform
	skyExternalFaceFogColorUniform := r.worldSkyExternalFaceFogColor
	skyFogDensityUniform := r.worldSkyFogDensityUniform
	skyCubemapFogDensityUniform := r.worldSkyCubemapFogDensityUniform
	skyExternalFaceFogDensityUniform := r.worldSkyExternalFaceFogDensity
	fallbackTexture := r.worldFallbackTexture
	skyFallbackAlpha := r.worldSkyAlphaFallback
	worldFastSky := readWorldFastSkyEnabled()
	skySolidLayerSpeed := readWorldSkySolidSpeedCvar()
	skyAlphaLayerSpeed := readWorldSkyAlphaSpeedCvar()
	skyExternalCubemap := r.worldSkyExternalCubemap
	skyExternalFaceTextures := r.worldSkyExternalFaceTextures
	skyExternalMode := r.worldSkyExternalMode
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	skyFogOverride := r.worldSkyFogOverride
	worldTree := r.worldTree
	worldHasLitWater := r.worldHasLitWater
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	allFaces := []WorldFace(nil)
	leafFaces := [][]int(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		allFaces = append(allFaces, r.worldData.Geometry.Faces...)
		leafFaces = r.worldData.Geometry.LeafFaces
	}
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]uint32, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldSkySolidTextures := make(map[int32]uint32, len(r.worldSkySolidTextures))
	for k, v := range r.worldSkySolidTextures {
		worldSkySolidTextures[k] = v
	}
	worldSkyAlphaTextures := make(map[int32]uint32, len(r.worldSkyAlphaTextures))
	for k, v := range r.worldSkyAlphaTextures {
		worldSkyAlphaTextures[k] = v
	}
	worldSkyFlatTextures := make(map[int32]uint32, len(r.worldSkyFlatTextures))
	for k, v := range r.worldSkyFlatTextures {
		worldSkyFlatTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	worldLightmaps := append([]uint32(nil), r.worldLightmaps...)
	lightPool := r.lightPool // Get light pool for light evaluation
	r.mu.RUnlock()

	if program == 0 || skyProgram == 0 || skyCubemapProgram == 0 || skyExternalFaceProgram == 0 || vao == 0 || indexCount <= 0 {
		return
	}
	faces := selectVisibleWorldFaces(worldTree, allFaces, leafFaces, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z})

	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFogFactor := resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), skyFogOverride, fogDensity)
	skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(faces, worldHasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, worldLightmaps, fallbackTexture, fallbackLightmap, vao, [3]float32{}, identityModelRotationMatrix, 1, 1, 0, float64(camera.Time), camera, liquidAlpha, lightPool)
	bindWorldProgram := func() {
		gl.UseProgram(program)
		gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
		gl.Uniform1i(textureUniform, 0)
		gl.Uniform1i(lightmapUniform, 1)
		gl.Uniform1i(fullbrightUniform, 2)
		gl.Uniform1f(hasFullbrightUniform, 0)
		gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
		gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
		gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
		gl.Uniform1f(modelScaleUniform, 1)
		gl.Uniform1f(timeUniform, camera.Time)
		gl.Uniform1f(turbulentUniform, 0)
		gl.Uniform1f(litWaterUniform, 0)
		gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
		gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
		gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
		gl.BindVertexArray(vao)
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	bindWorldProgram()
	if len(faces) == 0 {
		if drawNonLiquid {
			gl.DepthMask(true)
			gl.Disable(gl.BLEND)
			gl.Uniform1f(alphaUniform, 1)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, fallbackTexture)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.DrawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
		}
	} else {
		if drawSky {
			renderSkyPass(skyFaces, skyPassState{
				program:                     skyProgram,
				cubemapProgram:              skyCubemapProgram,
				vpUniform:                   skyVPUniform,
				solidUniform:                skySolidUniform,
				alphaUniform:                skyAlphaUniform,
				cubemapVPUniform:            skyCubemapVPUniform,
				cubemapUniform:              skyCubemapUniform,
				externalFaceVPUniform:       skyExternalFaceVPUniform,
				externalFaceRTUniform:       skyExternalFaceRTUniform,
				externalFaceBKUniform:       skyExternalFaceBKUniform,
				externalFaceLFUniform:       skyExternalFaceLFUniform,
				externalFaceFTUniform:       skyExternalFaceFTUniform,
				externalFaceUPUniform:       skyExternalFaceUPUniform,
				externalFaceDNUniform:       skyExternalFaceDNUniform,
				modelOffsetUniform:          skyModelOffsetUniform,
				modelRotationUniform:        skyModelRotationUniform,
				modelScaleUniform:           skyModelScaleUniform,
				cubemapModelOffsetUniform:   skyCubemapModelOffsetUniform,
				cubemapModelRotationUniform: skyCubemapModelRotationUniform,
				cubemapModelScaleUniform:    skyCubemapModelScaleUniform,
				externalFaceModelOffset:     skyExternalFaceModelOffsetUniform,
				externalFaceModelRotation:   skyExternalFaceModelRotationUniform,
				externalFaceModelScale:      skyExternalFaceModelScaleUniform,
				timeUniform:                 skyTimeUniform,
				solidLayerSpeedUniform:      skySolidLayerSpeedUniform,
				alphaLayerSpeedUniform:      skyAlphaLayerSpeedUniform,
				cameraOriginUniform:         skyCameraOriginUniform,
				cubemapCameraOriginUniform:  skyCubemapCameraOriginUniform,
				externalFaceCameraOrigin:    skyExternalFaceCameraOriginUniform,
				fogColorUniform:             skyFogColorUniform,
				cubemapFogColorUniform:      skyCubemapFogColorUniform,
				externalFaceFogColor:        skyExternalFaceFogColorUniform,
				fogDensityUniform:           skyFogDensityUniform,
				cubemapFogDensityUniform:    skyCubemapFogDensityUniform,
				externalFaceFogDensity:      skyExternalFaceFogDensityUniform,
				vp:                          vp,
				time:                        camera.Time,
				solidLayerSpeed:             skySolidLayerSpeed,
				alphaLayerSpeed:             skyAlphaLayerSpeed,
				cameraOrigin:                [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
				modelOffset:                 [3]float32{0, 0, 0},
				modelRotation:               identityModelRotationMatrix,
				modelScale:                  1,
				fogColor:                    fogColor,
				fogDensity:                  skyFogFactor,
				solidTextures:               worldSkySolidTextures,
				alphaTextures:               worldSkyAlphaTextures,
				flatTextures:                worldSkyFlatTextures,
				textureAnimations:           worldTextureAnimations,
				fallbackSolid:               fallbackTexture,
				fallbackAlpha:               skyFallbackAlpha,
				externalFaceProgram:         skyExternalFaceProgram,
				externalCubemap:             skyExternalCubemap,
				externalFaceTextures:        skyExternalFaceTextures,
				externalSkyMode:             skyExternalMode,
				fastSky:                     worldFastSky,
			})
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				bindWorldProgram()
			}
		}
		if drawNonLiquid {
			renderWorldDrawCalls(opaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			renderWorldDrawCalls(alphaTestFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, translucentFaces...)
			r.mu.Unlock()
		}
		if drawLiquidOpaque {
			renderWorldDrawCalls(liquidOpaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
		}
		if drawLiquidTranslucent {
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, liquidTranslucentFaces...)
			r.mu.Unlock()
		}
	}
	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Enable(gl.BLEND)
}

// renderBrushEntities renders BSP brush entities (doors, platforms, lifts). Each entity has a model offset, rotation matrix, and optional alpha. Uses the same world shader with model transform uniforms.
func (r *Renderer) renderBrushEntities(entities []BrushEntity, selector worldBrushPassSelector) {
	if len(entities) == 0 {
		return
	}
	selector = normalizeWorldBrushPassSelector(selector)
	drawSky := selector.includesSky()
	drawNonLiquid := selector.includesNonLiquid()
	drawLiquidOpaque := selector.includesLiquidOpaque()
	drawLiquidTranslucent := selector.includesLiquidTranslucent()

	r.mu.Lock()
	program := r.worldProgram
	skyProgram := r.worldSkyProgram
	skyCubemapProgram := r.worldSkyCubemapProgram
	skyExternalFaceProgram := r.worldSkyExternalFaceProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	fullbrightUniform := r.worldFullbrightUniform
	hasFullbrightUniform := r.worldHasFullbrightUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	skyVPUniform := r.worldSkyVPUniform
	skySolidUniform := r.worldSkySolidUniform
	skyAlphaUniform := r.worldSkyAlphaUniform
	skyCubemapVPUniform := r.worldSkyCubemapVPUniform
	skyCubemapUniform := r.worldSkyCubemapUniform
	skyExternalFaceVPUniform := r.worldSkyExternalFaceVPUniform
	skyExternalFaceRTUniform := r.worldSkyExternalFaceRTUniform
	skyExternalFaceBKUniform := r.worldSkyExternalFaceBKUniform
	skyExternalFaceLFUniform := r.worldSkyExternalFaceLFUniform
	skyExternalFaceFTUniform := r.worldSkyExternalFaceFTUniform
	skyExternalFaceUPUniform := r.worldSkyExternalFaceUPUniform
	skyExternalFaceDNUniform := r.worldSkyExternalFaceDNUniform
	skyModelOffsetUniform := r.worldSkyModelOffsetUniform
	skyModelRotationUniform := r.worldSkyModelRotationUniform
	skyModelScaleUniform := r.worldSkyModelScaleUniform
	skyCubemapModelOffsetUniform := r.worldSkyCubemapModelOffsetUniform
	skyCubemapModelRotationUniform := r.worldSkyCubemapModelRotationUniform
	skyCubemapModelScaleUniform := r.worldSkyCubemapModelScaleUniform
	skyExternalFaceModelOffsetUniform := r.worldSkyExternalFaceModelOffset
	skyExternalFaceModelRotationUniform := r.worldSkyExternalFaceModelRotation
	skyExternalFaceModelScaleUniform := r.worldSkyExternalFaceModelScale
	skyTimeUniform := r.worldSkyTimeUniform
	skySolidLayerSpeedUniform := r.worldSkySolidLayerSpeedUniform
	skyAlphaLayerSpeedUniform := r.worldSkyAlphaLayerSpeedUniform
	skyCameraOriginUniform := r.worldSkyCameraOriginUniform
	skyCubemapCameraOriginUniform := r.worldSkyCubemapCameraOriginUniform
	skyExternalFaceCameraOriginUniform := r.worldSkyExternalFaceCameraOrigin
	skyFogColorUniform := r.worldSkyFogColorUniform
	skyCubemapFogColorUniform := r.worldSkyCubemapFogColorUniform
	skyExternalFaceFogColorUniform := r.worldSkyExternalFaceFogColor
	skyFogDensityUniform := r.worldSkyFogDensityUniform
	skyCubemapFogDensityUniform := r.worldSkyCubemapFogDensityUniform
	skyExternalFaceFogDensityUniform := r.worldSkyExternalFaceFogDensity
	fallbackTexture := r.worldFallbackTexture
	skyFallbackAlpha := r.worldSkyAlphaFallback
	worldFastSky := readWorldFastSkyEnabled()
	skySolidLayerSpeed := readWorldSkySolidSpeedCvar()
	skyAlphaLayerSpeed := readWorldSkyAlphaSpeedCvar()
	skyExternalCubemap := r.worldSkyExternalCubemap
	skyExternalFaceTextures := r.worldSkyExternalFaceTextures
	skyExternalMode := r.worldSkyExternalMode
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	skyFogOverride := r.worldSkyFogOverride
	worldTree := r.worldTree
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]uint32, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldSkySolidTextures := make(map[int32]uint32, len(r.worldSkySolidTextures))
	for k, v := range r.worldSkySolidTextures {
		worldSkySolidTextures[k] = v
	}
	worldSkyAlphaTextures := make(map[int32]uint32, len(r.worldSkyAlphaTextures))
	for k, v := range r.worldSkyAlphaTextures {
		worldSkyAlphaTextures[k] = v
	}
	worldSkyFlatTextures := make(map[int32]uint32, len(r.worldSkyFlatTextures))
	for k, v := range r.worldSkyFlatTextures {
		worldSkyFlatTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	lightPool := r.lightPool // Get light pool for light evaluation
	type drawBrush struct {
		frame       int
		origin      [3]float32
		rotation    [16]float32
		alpha       float32
		scale       float32
		hasLitWater bool
		mesh        *glWorldMesh
	}
	brushes := make([]drawBrush, 0, len(entities))
	for _, entity := range entities {
		mesh := r.ensureBrushModelLocked(entity.SubmodelIndex)
		if mesh == nil {
			continue
		}
		brushes = append(brushes, drawBrush{
			frame:       entity.Frame,
			origin:      entity.Origin,
			rotation:    buildBrushRotationMatrix(entity.Angles),
			alpha:       entity.Alpha,
			scale:       entity.Scale,
			hasLitWater: mesh.hasLitWater,
			mesh:        mesh,
		})
	}
	r.mu.Unlock()

	if program == 0 || skyProgram == 0 || skyCubemapProgram == 0 || skyExternalFaceProgram == 0 || len(brushes) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 0)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform1f(litWaterUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFogFactor := resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), skyFogOverride, fogDensity)

	for _, brush := range brushes {
		skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(brush.mesh.faces, brush.hasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, brush.mesh.lightmaps, fallbackTexture, fallbackLightmap, brush.mesh.vao, brush.origin, brush.rotation, brush.scale, brush.alpha, brush.frame, float64(camera.Time), camera, liquidAlpha, lightPool)
		bindBrushWorldProgram := func() {
			gl.UseProgram(program)
			gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
			gl.Uniform1i(textureUniform, 0)
			gl.Uniform1i(lightmapUniform, 1)
			gl.Uniform1i(fullbrightUniform, 2)
			gl.Uniform1f(hasFullbrightUniform, 0)
			gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
			gl.Uniform1f(timeUniform, camera.Time)
			gl.Uniform1f(turbulentUniform, 0)
			gl.Uniform1f(litWaterUniform, 0)
			gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
			gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
			gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
			gl.Uniform3f(modelOffsetUniform, brush.origin[0], brush.origin[1], brush.origin[2])
			gl.UniformMatrix4fv(modelRotationUniform, 1, false, &brush.rotation[0])
			gl.Uniform1f(modelScaleUniform, brush.scale)
			gl.BindVertexArray(brush.mesh.vao)
		}
		bindBrushWorldProgram()
		if drawSky {
			renderSkyPass(skyFaces, skyPassState{
				program:                     skyProgram,
				cubemapProgram:              skyCubemapProgram,
				vpUniform:                   skyVPUniform,
				solidUniform:                skySolidUniform,
				alphaUniform:                skyAlphaUniform,
				cubemapVPUniform:            skyCubemapVPUniform,
				cubemapUniform:              skyCubemapUniform,
				externalFaceVPUniform:       skyExternalFaceVPUniform,
				externalFaceRTUniform:       skyExternalFaceRTUniform,
				externalFaceBKUniform:       skyExternalFaceBKUniform,
				externalFaceLFUniform:       skyExternalFaceLFUniform,
				externalFaceFTUniform:       skyExternalFaceFTUniform,
				externalFaceUPUniform:       skyExternalFaceUPUniform,
				externalFaceDNUniform:       skyExternalFaceDNUniform,
				modelOffsetUniform:          skyModelOffsetUniform,
				modelRotationUniform:        skyModelRotationUniform,
				modelScaleUniform:           skyModelScaleUniform,
				cubemapModelOffsetUniform:   skyCubemapModelOffsetUniform,
				cubemapModelRotationUniform: skyCubemapModelRotationUniform,
				cubemapModelScaleUniform:    skyCubemapModelScaleUniform,
				externalFaceModelOffset:     skyExternalFaceModelOffsetUniform,
				externalFaceModelRotation:   skyExternalFaceModelRotationUniform,
				externalFaceModelScale:      skyExternalFaceModelScaleUniform,
				timeUniform:                 skyTimeUniform,
				solidLayerSpeedUniform:      skySolidLayerSpeedUniform,
				alphaLayerSpeedUniform:      skyAlphaLayerSpeedUniform,
				cameraOriginUniform:         skyCameraOriginUniform,
				cubemapCameraOriginUniform:  skyCubemapCameraOriginUniform,
				externalFaceCameraOrigin:    skyExternalFaceCameraOriginUniform,
				fogColorUniform:             skyFogColorUniform,
				cubemapFogColorUniform:      skyCubemapFogColorUniform,
				externalFaceFogColor:        skyExternalFaceFogColorUniform,
				fogDensityUniform:           skyFogDensityUniform,
				cubemapFogDensityUniform:    skyCubemapFogDensityUniform,
				externalFaceFogDensity:      skyExternalFaceFogDensityUniform,
				vp:                          vp,
				time:                        camera.Time,
				solidLayerSpeed:             skySolidLayerSpeed,
				alphaLayerSpeed:             skyAlphaLayerSpeed,
				cameraOrigin:                [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
				modelOffset:                 brush.origin,
				modelRotation:               brush.rotation,
				modelScale:                  brush.scale,
				fogColor:                    fogColor,
				fogDensity:                  skyFogFactor,
				solidTextures:               worldSkySolidTextures,
				alphaTextures:               worldSkyAlphaTextures,
				flatTextures:                worldSkyFlatTextures,
				textureAnimations:           worldTextureAnimations,
				fallbackSolid:               fallbackTexture,
				fallbackAlpha:               skyFallbackAlpha,
				externalFaceProgram:         skyExternalFaceProgram,
				externalCubemap:             skyExternalCubemap,
				externalFaceTextures:        skyExternalFaceTextures,
				externalSkyMode:             skyExternalMode,
				frame:                       brush.frame,
				fastSky:                     worldFastSky,
			})
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				bindBrushWorldProgram()
			}
		}
		if drawNonLiquid {
			renderWorldDrawCalls(opaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			renderWorldDrawCalls(alphaTestFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, translucentFaces...)
			r.mu.Unlock()
		}
		if drawLiquidOpaque {
			renderWorldDrawCalls(liquidOpaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
		}
		if drawLiquidTranslucent {
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, liquidTranslucentFaces...)
			r.mu.Unlock()
		}
	}

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Enable(gl.BLEND)
}

// bucketWorldFacesWithLights is like bucketWorldFaces but also evaluates dynamic lights.
// This variant accepts a light pool and computes light contributions for each face.
func bucketWorldFacesWithLights(faces []WorldFace, hasLitWater bool, textures map[int32]uint32, fullbrightTextures map[int32]uint32, textureAnimations []*SurfaceTexture, lightmaps []uint32, fallbackTexture, fallbackLightmap, vao uint32, modelOffset [3]float32, modelRotation [16]float32, modelScale, entityAlpha float32, entityFrame int, timeSeconds float64, camera CameraState, liquidAlpha worldLiquidAlphaSettings, lightPool *glLightPool) (sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent []worldDrawCall) {
	for _, face := range faces {
		center := transformModelSpacePoint(face.Center, modelOffset, modelRotation, modelScale)
		call := worldDrawCall{
			face:              face,
			texture:           worldTextureForFace(face, textures, textureAnimations, fallbackTexture, entityFrame, timeSeconds),
			fullbrightTexture: worldTextureForFace(face, fullbrightTextures, textureAnimations, 0, entityFrame, timeSeconds),
			lightmap:          worldLightmapForFace(face, lightmaps, fallbackLightmap),
			alpha:             worldFaceAlpha(face.Flags, liquidAlpha) * entityAlpha,
			turbulent:         worldFaceUsesTurb(face.Flags),
			hasLitWater:       hasLitWater,
			distanceSq:        worldFaceDistanceSq(center, camera),
			light:             [3]float32{}, // Will be populated below
			vao:               vao,
			modelOffset:       modelOffset,
			modelRotation:     modelRotation,
			modelScale:        modelScale,
		}

		// Evaluate dynamic lights at this face's center
		if lightPool != nil {
			call.light = lightPool.EvaluateLightsAtPoint(center)
		}

		switch worldFacePass(face.Flags, call.alpha) {
		case worldPassSky:
			sky = append(sky, call)
		case worldPassAlphaTest:
			alphaTest = append(alphaTest, call)
		case worldPassTranslucent:
			if worldFaceIsLiquid(face.Flags) {
				liquidTranslucent = append(liquidTranslucent, call)
				continue
			}
			translucent = append(translucent, call)
		default:
			if worldFaceIsLiquid(face.Flags) {
				liquidOpaque = append(liquidOpaque, call)
				continue
			}
			opaque = append(opaque, call)
		}
	}

	sort.SliceStable(liquidTranslucent, func(i, j int) bool {
		return liquidTranslucent[i].distanceSq > liquidTranslucent[j].distanceSq
	})
	sort.SliceStable(translucent, func(i, j int) bool {
		return translucent[i].distanceSq > translucent[j].distanceSq
	})

	return sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent
}

// bucketWorldFaces is a simplified face bucketing function without dynamic light support, used for brush entities.
func bucketWorldFaces(faces []WorldFace, textures map[int32]uint32, fullbrightTextures map[int32]uint32, textureAnimations []*SurfaceTexture, lightmaps []uint32, fallbackTexture, fallbackLightmap uint32, modelOffset [3]float32, camera CameraState, liquidAlpha worldLiquidAlphaSettings) (sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent []worldDrawCall) {
	return bucketWorldFacesWithLights(faces, worldFacesHaveLitWater(faces), textures, fullbrightTextures, textureAnimations, lightmaps, fallbackTexture, fallbackLightmap, 0, modelOffset, identityModelRotationMatrix, 1, 1, 0, float64(camera.Time), camera, liquidAlpha, nil)
}

// worldTextureForFace resolves the current diffuse texture GL handle for a face, accounting for texture animation chains.
func worldTextureForFace(face WorldFace, textures map[int32]uint32, textureAnimations []*SurfaceTexture, fallbackTexture uint32, frame int, timeSeconds float64) uint32 {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}

	tex := textures[textureIndex]
	if tex == 0 && textureIndex != face.TextureIndex {
		tex = textures[face.TextureIndex]
	}
	if tex == 0 {
		tex = fallbackTexture
	}
	return tex
}

// worldLightmapForFace resolves the lightmap texture GL handle for a face from the atlas page array.
func worldLightmapForFace(face WorldFace, lightmaps []uint32, fallbackLightmap uint32) uint32 {
	if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(lightmaps) && lightmaps[face.LightmapIndex] != 0 {
		return lightmaps[face.LightmapIndex]
	}
	return fallbackLightmap
}

// renderWorldDrawCalls issues GL draw calls for bucketed world faces. Each call binds its diffuse + lightmap + fullbright textures and draws the face's index range from the VAO.
func renderWorldDrawCalls(calls []worldDrawCall, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform int32, depthWrite bool) {
	if len(calls) == 0 {
		return
	}
	litWaterEnabled := worldLitWaterCvarEnabled()
	gl.DepthMask(depthWrite)
	if depthWrite {
		gl.Disable(gl.BLEND)
	} else {
		gl.Enable(gl.BLEND)
	}

	lastVAO := uint32(0xFFFFFFFF)
	lastLitWaterValue := float32(-1)
	for _, call := range calls {
		if call.vao != lastVAO {
			gl.BindVertexArray(call.vao)
			lastVAO = call.vao
		}
		gl.Uniform3f(modelOffsetUniform, call.modelOffset[0], call.modelOffset[1], call.modelOffset[2])
		gl.UniformMatrix4fv(modelRotationUniform, 1, false, &call.modelRotation[0])
		gl.Uniform1f(modelScaleUniform, call.modelScale)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, call.texture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, call.lightmap)

		// Bind fullbright texture if available
		gl.ActiveTexture(gl.TEXTURE2)
		if call.fullbrightTexture != 0 {
			gl.BindTexture(gl.TEXTURE_2D, call.fullbrightTexture)
			gl.Uniform1f(hasFullbrightUniform, 1.0)
		} else {
			gl.BindTexture(gl.TEXTURE_2D, 0)
			gl.Uniform1f(hasFullbrightUniform, 0.0)
		}

		gl.ActiveTexture(gl.TEXTURE0)
		if call.turbulent {
			gl.Uniform1f(turbulentUniform, 1)
		} else {
			gl.Uniform1f(turbulentUniform, 0)
		}
		if litWaterUniform >= 0 {
			litWaterValue := float32(0)
			if litWaterEnabled && call.hasLitWater {
				litWaterValue = 1
			}
			if litWaterValue != lastLitWaterValue {
				gl.Uniform1f(litWaterUniform, litWaterValue)
				lastLitWaterValue = litWaterValue
			}
		}
		gl.Uniform3f(dynamicLightUniform, call.light[0], call.light[1], call.light[2])
		gl.Uniform1f(alphaUniform, call.alpha)
		gl.DrawElements(gl.TRIANGLES, int32(call.face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(call.face.FirstIndex*4)))
	}
}

func worldLitWaterCvarEnabled() bool {
	cv := cvar.Get(CvarRLitWater)
	if cv == nil {
		return true
	}
	return cv.Int != 0
}

func worldFacesHaveLitWater(faces []WorldFace) bool {
	for _, face := range faces {
		if face.Flags&model.SurfDrawTurb != 0 && face.LightmapIndex >= 0 {
			return true
		}
	}
	return false
}
