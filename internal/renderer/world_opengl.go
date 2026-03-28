//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	aliasimpl "github.com/ironwail/ironwail-go/internal/renderer/alias"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
	worldopengl "github.com/ironwail/ironwail-go/internal/renderer/world/opengl"
	"log/slog"
	"math"
	"sort"
	"strings"
	"unsafe"
)

// ---- merged from world_opengl.go ----
type WorldGeometry = worldimpl.WorldGeometry
type WorldVertex = worldimpl.WorldVertex
type WorldFace = worldimpl.WorldFace
type WorldLightmapSurface = worldimpl.WorldLightmapSurface
type WorldLightmapPage = worldimpl.WorldLightmapPage

type WorldRenderData = worldopengl.RenderData

const worldLightmapPageSize = worldopengl.LightmapPageSize

// BuildWorldGeometry extracts renderable geometry from BSP data.
func BuildWorldGeometry(tree *bsp.Tree) (*WorldGeometry, error) {
	return worldopengl.BuildWorldGeometry(tree)
}

// BuildModelGeometry extracts renderable geometry for a specific BSP model index.
func BuildModelGeometry(tree *bsp.Tree, modelIndex int) (*WorldGeometry, error) {
	return worldopengl.BuildModelGeometry(tree, modelIndex)
}

// buildWorldRenderData builds complete CPU-side render data: geometry, lightmaps, and bounding box.
func buildWorldRenderData(tree *bsp.Tree) (*WorldRenderData, error) {
	return worldopengl.BuildWorldRenderData(tree)
}

// buildModelRenderData builds render data for a specific BSP submodel index (used for brush entities like doors).
func buildModelRenderData(tree *bsp.Tree, modelIndex int) (*WorldRenderData, error) {
	return worldopengl.BuildModelRenderData(tree, modelIndex)
}

// ---- merged from world_render_opengl_root.go ----
// ensureWorldProgram lazily compiles the world rendering shader program. The world shader performs multi-texture rendering: diffuse texture * lightmap, with optional fullbright overlay and dynamic light contribution.
func (r *Renderer) ensureWorldProgram() error {
	if r.worldProgram != 0 {
		return nil
	}

	vs, err := compileShader(worldopengl.WorldVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile world vertex shader: %w", err)
	}
	fs, err := compileShader(worldopengl.WorldFragmentShaderGL, gl.FRAGMENT_SHADER)
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
	r.worldFogColor, r.worldFogDensity = blendFogStateTowards(r.worldFogColor, r.worldFogDensity, color, density, 0.2)
}

func cloneWorldTextureMap(src map[int32]uint32) map[int32]uint32 {
	cloned := make(map[int32]uint32, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

// buildSkyPassStateLocked snapshots the renderer-owned sky pass state.
// The caller must hold r.mu while invoking this helper.
func (r *Renderer) buildSkyPassStateLocked(vp [16]float32, camera CameraState, textureAnimations []*SurfaceTexture) worldopengl.SkyPassState {
	worldFastSky := readWorldFastSkyEnabled()
	worldProceduralSky := readWorldProceduralSkyEnabled()
	skySolidLayerSpeed := readWorldSkySolidSpeedCvar()
	skyAlphaLayerSpeed := readWorldSkyAlphaSpeedCvar()
	skyExternalMode := r.worldSkyExternalMode
	skyFogFactor := resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), r.worldSkyFogOverride, r.worldFogDensity)
	proceduralSkyHorizon, proceduralSkyZenith := proceduralSkyGradientColors()

	return worldopengl.SkyPassState{
		Program:                     r.worldSkyProgram,
		ProceduralProgram:           r.worldSkyProceduralProgram,
		CubemapProgram:              r.worldSkyCubemapProgram,
		ExternalFaceProgram:         r.worldSkyExternalFaceProgram,
		VPUniform:                   r.worldSkyVPUniform,
		SolidUniform:                r.worldSkySolidUniform,
		AlphaUniform:                r.worldSkyAlphaUniform,
		ProceduralVPUniform:         r.worldSkyProceduralVPUniform,
		CubemapVPUniform:            r.worldSkyCubemapVPUniform,
		CubemapUniform:              r.worldSkyCubemapUniform,
		ExternalFaceVPUniform:       r.worldSkyExternalFaceVPUniform,
		ExternalFaceRTUniform:       r.worldSkyExternalFaceRTUniform,
		ExternalFaceBKUniform:       r.worldSkyExternalFaceBKUniform,
		ExternalFaceLFUniform:       r.worldSkyExternalFaceLFUniform,
		ExternalFaceFTUniform:       r.worldSkyExternalFaceFTUniform,
		ExternalFaceUPUniform:       r.worldSkyExternalFaceUPUniform,
		ExternalFaceDNUniform:       r.worldSkyExternalFaceDNUniform,
		ModelOffsetUniform:          r.worldSkyModelOffsetUniform,
		ModelRotationUniform:        r.worldSkyModelRotationUniform,
		ModelScaleUniform:           r.worldSkyModelScaleUniform,
		ProceduralModelOffset:       r.worldSkyProceduralModelOffset,
		ProceduralModelRotation:     r.worldSkyProceduralModelRotation,
		ProceduralModelScale:        r.worldSkyProceduralModelScale,
		CubemapModelOffsetUniform:   r.worldSkyCubemapModelOffsetUniform,
		CubemapModelRotationUniform: r.worldSkyCubemapModelRotationUniform,
		CubemapModelScaleUniform:    r.worldSkyCubemapModelScaleUniform,
		ExternalFaceModelOffset:     r.worldSkyExternalFaceModelOffset,
		ExternalFaceModelRotation:   r.worldSkyExternalFaceModelRotation,
		ExternalFaceModelScale:      r.worldSkyExternalFaceModelScale,
		TimeUniform:                 r.worldSkyTimeUniform,
		SolidLayerSpeedUniform:      r.worldSkySolidLayerSpeedUniform,
		AlphaLayerSpeedUniform:      r.worldSkyAlphaLayerSpeedUniform,
		CameraOriginUniform:         r.worldSkyCameraOriginUniform,
		ProceduralCameraOrigin:      r.worldSkyProceduralCameraOrigin,
		CubemapCameraOriginUniform:  r.worldSkyCubemapCameraOriginUniform,
		ExternalFaceCameraOrigin:    r.worldSkyExternalFaceCameraOrigin,
		FogColorUniform:             r.worldSkyFogColorUniform,
		ProceduralFogColor:          r.worldSkyProceduralFogColor,
		CubemapFogColorUniform:      r.worldSkyCubemapFogColorUniform,
		ExternalFaceFogColor:        r.worldSkyExternalFaceFogColor,
		FogDensityUniform:           r.worldSkyFogDensityUniform,
		ProceduralFogDensity:        r.worldSkyProceduralFogDensity,
		ProceduralHorizonColor:      r.worldSkyProceduralHorizonColor,
		ProceduralZenithColor:       r.worldSkyProceduralZenithColor,
		CubemapFogDensityUniform:    r.worldSkyCubemapFogDensityUniform,
		ExternalFaceFogDensity:      r.worldSkyExternalFaceFogDensity,
		VP:                          vp,
		Time:                        camera.Time,
		SolidLayerSpeed:             skySolidLayerSpeed,
		AlphaLayerSpeed:             skyAlphaLayerSpeed,
		CameraOrigin:                [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
		ModelOffset:                 [3]float32{0, 0, 0},
		ModelRotation:               identityModelRotationMatrix,
		ModelScale:                  1,
		FogColor:                    r.worldFogColor,
		ProceduralHorizon:           proceduralSkyHorizon,
		ProceduralZenith:            proceduralSkyZenith,
		FogDensity:                  skyFogFactor,
		SolidTextures:               cloneWorldTextureMap(r.worldSkySolidTextures),
		AlphaTextures:               cloneWorldTextureMap(r.worldSkyAlphaTextures),
		FlatTextures:                cloneWorldTextureMap(r.worldSkyFlatTextures),
		TextureAnimations:           textureAnimations,
		FallbackSolid:               r.worldFallbackTexture,
		FallbackAlpha:               r.worldSkyAlphaFallback,
		ExternalSkyMode:             worldExternalSkyMode(skyExternalMode),
		ExternalCubemap:             r.worldSkyExternalCubemap,
		ExternalFaceTextures:        r.worldSkyExternalFaceTextures,
		Frame:                       0,
		FastSky:                     worldFastSky,
		ProceduralSky:               shouldUseProceduralSky(worldFastSky, worldProceduralSky, skyExternalMode),
	}
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
	fallbackTexture := r.worldFallbackTexture
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
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
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	worldLightmaps := append([]uint32(nil), r.worldLightmaps...)
	skyState := r.buildSkyPassStateLocked(vp, camera, worldTextureAnimations)
	lightPool := r.lightPool // Get light pool for light evaluation
	r.mu.RUnlock()

	if program == 0 || skyState.Program == 0 || skyState.CubemapProgram == 0 || skyState.ExternalFaceProgram == 0 || vao == 0 || indexCount <= 0 {
		return
	}
	faces := selectVisibleWorldFaces(worldTree, allFaces, leafFaces, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z})

	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(faces, worldHasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, worldLightmaps, fallbackTexture, fallbackLightmap, vao, [3]float32{}, identityModelRotationMatrix, 1, 1, 0, float64(camera.Time), camera, liquidAlpha, lightPool)
	worldProgramState := worldopengl.WorldProgramState{
		Program:              program,
		VPUniform:            vpUniform,
		TextureUniform:       textureUniform,
		LightmapUniform:      lightmapUniform,
		FullbrightUniform:    fullbrightUniform,
		HasFullbrightUniform: hasFullbrightUniform,
		DynamicLightUniform:  dynamicLightUniform,
		TimeUniform:          timeUniform,
		TurbulentUniform:     turbulentUniform,
		LitWaterUniform:      litWaterUniform,
		CameraOriginUniform:  cameraOriginUniform,
		FogColorUniform:      fogColorUniform,
		FogDensityUniform:    fogDensityUniform,
		VP:                   vp,
		Time:                 camera.Time,
		CameraOrigin:         [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
		FogColor:             fogColor,
		FogDensity:           worldFogUniformDensity(fogDensity),
	}
	worldModelState := worldopengl.WorldModelState{
		ModelOffsetUniform:   modelOffsetUniform,
		ModelRotationUniform: modelRotationUniform,
		ModelScaleUniform:    modelScaleUniform,
		ModelRotation:        identityModelRotationMatrix,
		ModelScale:           1,
		VAO:                  vao,
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	worldopengl.BindWorldProgram(worldProgramState, worldModelState)
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
			worldopengl.RenderSkyPass(skyFaces, skyState, TextureAnimation)
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				worldopengl.BindWorldProgram(worldProgramState, worldModelState)
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
	fallbackTexture := r.worldFallbackTexture
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
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
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	baseSkyState := r.buildSkyPassStateLocked(vp, camera, worldTextureAnimations)
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

	if program == 0 || baseSkyState.Program == 0 || baseSkyState.CubemapProgram == 0 || baseSkyState.ExternalFaceProgram == 0 || len(brushes) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	brushProgramState := worldopengl.WorldProgramState{
		Program:              program,
		VPUniform:            vpUniform,
		TextureUniform:       textureUniform,
		LightmapUniform:      lightmapUniform,
		FullbrightUniform:    fullbrightUniform,
		HasFullbrightUniform: hasFullbrightUniform,
		DynamicLightUniform:  dynamicLightUniform,
		TimeUniform:          timeUniform,
		TurbulentUniform:     turbulentUniform,
		LitWaterUniform:      litWaterUniform,
		CameraOriginUniform:  cameraOriginUniform,
		FogColorUniform:      fogColorUniform,
		FogDensityUniform:    fogDensityUniform,
		VP:                   vp,
		Time:                 camera.Time,
		CameraOrigin:         [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
		FogColor:             fogColor,
		FogDensity:           worldFogUniformDensity(fogDensity),
	}

	for _, brush := range brushes {
		skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(brush.mesh.faces, brush.hasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, brush.mesh.lightmaps, fallbackTexture, fallbackLightmap, brush.mesh.vao, brush.origin, brush.rotation, brush.scale, brush.alpha, brush.frame, float64(camera.Time), camera, liquidAlpha, lightPool)
		brushModelState := worldopengl.WorldModelState{
			ModelOffsetUniform:   modelOffsetUniform,
			ModelRotationUniform: modelRotationUniform,
			ModelScaleUniform:    modelScaleUniform,
			ModelOffset:          brush.origin,
			ModelRotation:        brush.rotation,
			ModelScale:           brush.scale,
			VAO:                  brush.mesh.vao,
		}
		worldopengl.BindWorldProgram(brushProgramState, brushModelState)
		if drawSky {
			skyState := baseSkyState
			skyState.ModelOffset = brush.origin
			skyState.ModelRotation = brush.rotation
			skyState.ModelScale = brush.scale
			skyState.Frame = brush.frame
			worldopengl.RenderSkyPass(skyFaces, skyState, TextureAnimation)
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				worldopengl.BindWorldProgram(brushProgramState, brushModelState)
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
	var evaluateLights worldopengl.LightEvaluator
	if lightPool != nil {
		evaluateLights = func(point [3]float32) [3]float32 {
			return lightPool.EvaluateLightsAtPoint(point)
		}
	}
	return worldopengl.BucketFacesWithLights(
		faces,
		hasLitWater,
		textures,
		fullbrightTextures,
		textureAnimations,
		lightmaps,
		fallbackTexture,
		fallbackLightmap,
		vao,
		modelOffset,
		modelRotation,
		modelScale,
		entityAlpha,
		entityFrame,
		timeSeconds,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
		liquidAlpha.toWorld(),
		TextureAnimation,
		evaluateLights,
	)
}

// renderWorldDrawCalls issues GL draw calls for bucketed world faces. Each call binds its diffuse + lightmap + fullbright textures and draws the face's index range from the VAO.
func renderWorldDrawCalls(calls []worldDrawCall, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform int32, depthWrite bool) {
	worldopengl.RenderDrawCalls(calls, worldopengl.DrawUniformState{
		AlphaUniform:         alphaUniform,
		TurbulentUniform:     turbulentUniform,
		LitWaterUniform:      litWaterUniform,
		DynamicLightUniform:  dynamicLightUniform,
		ModelOffsetUniform:   modelOffsetUniform,
		ModelRotationUniform: modelRotationUniform,
		ModelScaleUniform:    modelScaleUniform,
		HasFullbrightUniform: hasFullbrightUniform,
		DepthWrite:           depthWrite,
		LitWaterEnabled:      worldLitWaterCvarEnabled(),
	})
}

func worldLitWaterCvarEnabled() bool {
	cv := cvar.Get(CvarRLitWater)
	if cv == nil {
		return true
	}
	return cv.Int != 0
}

// ---- merged from world_alias_opengl_root.go ----
// ensureAliasScratchLocked creates a scratch VAO/VBO for alias model rendering. Alias models re-upload interpolated vertex data each frame, so the buffer uses GL_DYNAMIC_DRAW.
func (r *Renderer) ensureAliasScratchLocked() {
	if r.aliasScratchVAO != 0 && r.aliasScratchVBO != 0 {
		return
	}

	gl.GenVertexArrays(1, &r.aliasScratchVAO)
	gl.GenBuffers(1, &r.aliasScratchVBO)

	gl.BindVertexArray(r.aliasScratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.aliasScratchVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 4, nil, gl.DYNAMIC_DRAW)

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
}

// ensureAliasModelLocked lazily creates GPU data for an alias (MDL) model. Parses triangles, vertices, and texture coordinates, stores all pose vertices for CPU-side interpolation, and uploads the skin texture.
func (r *Renderer) ensureAliasModelLocked(modelID string, mdl *model.Model) *glAliasModel {
	if modelID == "" || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	if cached, ok := r.aliasModels[modelID]; ok {
		return cached
	}

	hdr := mdl.AliasHeader
	if len(hdr.STVerts) != hdr.NumVerts || len(hdr.Triangles) != hdr.NumTris || len(hdr.Poses) == 0 {
		return nil
	}

	r.ensureWorldFallbackTextureLocked()
	palette := append([]byte(nil), r.palette...)
	skins := make([]uint32, 0, len(hdr.Skins))
	fullbrightSkins := make([]uint32, 0, len(hdr.Skins))
	for _, skin := range hdr.Skins {
		if len(skin) != hdr.SkinWidth*hdr.SkinHeight {
			skins = append(skins, r.worldFallbackTexture)
			fullbrightSkins = append(fullbrightSkins, r.worldFallbackTexture)
			continue
		}
		rgba, fullbright := aliasSkinVariantRGBA(skin, palette, 0, false)
		tex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, rgba)
		if tex == 0 {
			tex = r.worldFallbackTexture
		}
		fullbrightTex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, fullbright)
		if fullbrightTex == 0 {
			fullbrightTex = r.worldFallbackTexture
		}
		skins = append(skins, tex)
		fullbrightSkins = append(fullbrightSkins, fullbrightTex)
	}

	refs := make([]aliasimpl.MeshRef, 0, len(hdr.Triangles)*3)
	for _, tri := range hdr.Triangles {
		for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
			idx := int(tri.VertIndex[vertexIndex])
			if idx < 0 || idx >= len(hdr.STVerts) {
				continue
			}
			st := hdr.STVerts[idx]
			s := float32(st.S) + 0.5
			if tri.FacesFront == 0 && st.OnSeam != 0 {
				s += float32(hdr.SkinWidth) * 0.5
			}
			refs = append(refs, aliasimpl.MeshRef{
				VertexIndex: idx,
				TexCoord: [2]float32{
					s / float32(hdr.SkinWidth),
					(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
				},
			})
		}
	}

	alias := &glAliasModel{
		modelID:          modelID,
		flags:            hdr.Flags,
		skins:            skins,
		fullbrightSkins:  fullbrightSkins,
		playerSkins:      make(map[uint32][]uint32),
		playerFullbright: make(map[uint32][]uint32),
		poses:            hdr.Poses,
		refs:             refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func uploadAliasSkinTextures(width, height int, baseRGBA, fullbrightRGBA []byte, fallback uint32) (uint32, uint32) {
	base := uploadWorldTextureRGBA(width, height, baseRGBA)
	if base == 0 {
		base = fallback
	}
	fullbright := uploadWorldTextureRGBA(width, height, fullbrightRGBA)
	if fullbright == 0 {
		fullbright = fallback
	}
	return base, fullbright
}

func (r *Renderer) resolveAliasSkinTexturesLocked(alias *glAliasModel, entity AliasModelEntity, skinSlot int) (uint32, uint32) {
	if alias == nil {
		return r.worldFallbackTexture, r.worldFallbackTexture
	}
	if entity.IsPlayer {
		if skins, ok := alias.playerSkins[entity.ColorMap]; ok && skinSlot >= 0 && skinSlot < len(skins) {
			return skins[skinSlot], alias.playerFullbright[entity.ColorMap][skinSlot]
		}
		hdr := entity.Model.AliasHeader
		palette := append([]byte(nil), r.palette...)
		playerSkins := make([]uint32, len(hdr.Skins))
		playerFullbright := make([]uint32, len(hdr.Skins))
		for i, skinPixels := range hdr.Skins {
			baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(skinPixels, palette, entity.ColorMap, true)
			playerSkins[i], playerFullbright[i] = uploadAliasSkinTextures(hdr.SkinWidth, hdr.SkinHeight, baseRGBA, fullbrightRGBA, r.worldFallbackTexture)
		}
		alias.playerSkins[entity.ColorMap] = playerSkins
		alias.playerFullbright[entity.ColorMap] = playerFullbright
		if skinSlot >= 0 && skinSlot < len(playerSkins) {
			return playerSkins[skinSlot], playerFullbright[skinSlot]
		}
	}
	if skinSlot >= 0 && skinSlot < len(alias.skins) {
		return alias.skins[skinSlot], alias.fullbrightSkins[skinSlot]
	}
	return r.worldFallbackTexture, r.worldFallbackTexture
}

// buildAliasDrawLocked prepares a complete alias model draw command: resolves the model, computes pose interpolation, builds interpolated vertices, and uploads to the scratch VBO.
func (r *Renderer) buildAliasDrawLocked(entity AliasModelEntity, fullAngles bool) *glAliasDraw {
	alias := r.ensureAliasModelLocked(entity.ModelID, entity.Model)
	if alias == nil || entity.Model == nil || entity.Model.AliasHeader == nil || len(alias.refs) == 0 {
		return nil
	}

	hdr := entity.Model.AliasHeader
	frame := entity.Frame
	if frame < 0 || frame >= len(hdr.Frames) {
		frame = 0
	}

	state := r.ensureAliasStateLocked(entity)
	state.Frame = frame
	aliasHdr := aliasHeaderFromModel(hdr)
	aliasHdr.Flags = applyAliasNoLerpListFlags(aliasHdr.Flags, entity.ModelID)
	interpData, err := SetupAliasFrame(state, aliasHdr, entity.TimeSeconds, true, false, 1)
	if err != nil {
		return nil
	}
	interpData.Origin, interpData.Angles = SetupEntityTransform(
		state,
		entity.TimeSeconds,
		true,
		entity.EntityKey == AliasViewModelEntityKey,
		false,
		false,
		1,
	)

	pose1 := interpData.Pose1
	pose2 := interpData.Pose2
	if pose1 < 0 || pose1 >= len(alias.poses) {
		pose1 = 0
	}
	if pose2 < 0 || pose2 >= len(alias.poses) {
		pose2 = 0
	}

	skin := r.worldFallbackTexture
	fullbrightSkin := r.worldFallbackTexture
	if len(alias.skins) > 0 {
		slot := resolveAliasSkinSlot(entity.Model.AliasHeader, entity.SkinNum, entity.TimeSeconds, len(alias.skins))
		skin, fullbrightSkin = r.resolveAliasSkinTexturesLocked(alias, entity, slot)
	}

	alpha, visible := visibleEntityAlpha(entity.Alpha)
	if !visible {
		return nil
	}

	return &glAliasDraw{
		alias:          alias,
		model:          entity.Model,
		pose1:          pose1,
		pose2:          pose2,
		blend:          interpData.Blend,
		skin:           skin,
		fullbrightSkin: fullbrightSkin,
		origin:         interpData.Origin,
		angles:         interpData.Angles,
		alpha:          alpha,
		scale:          entity.Scale,
		full:           fullAngles,
	}
}

// renderAliasDraws renders a batch of alias model draw commands. Sets up GL state with depth test, backface culling, and the world shader. For view model rendering, narrows the depth range to prevent the weapon from clipping into nearby walls.
func (r *Renderer) renderAliasDraws(draws []glAliasDraw, useViewModelDepthRange bool) {
	if len(draws) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
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
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	r.ensureAliasScratchLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	r.mu.Unlock()

	if program == 0 || scratchVAO == 0 || scratchVBO == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	if useViewModelDepthRange {
		gl.DepthRange(0.0, 0.3)
	}
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 1)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, r.worldFallbackTexture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, draw := range draws {
		// Use interpolated vertex building with two poses
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertices) == 0 {
			continue
		}
		vertexData := worldopengl.FlattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.BindTexture(gl.TEXTURE_2D, draw.skin)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_2D, draw.fullbrightSkin)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.Uniform1f(alphaUniform, draw.alpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	if useViewModelDepthRange {
		gl.DepthRange(0.0, 1.0)
	}
}

// renderAliasEntities renders all alias model entities by building draw commands and dispatching them to renderAliasDraws.
func (r *Renderer) renderAliasEntities(entities []AliasModelEntity) {
	r.mu.Lock()
	r.pruneAliasStatesLocked(entities)
	draws := make([]glAliasDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildAliasDrawLocked(entity, false); draw != nil {
			draws = append(draws, *draw)
		}
	}
	r.mu.Unlock()
	r.renderAliasDraws(draws, false)
}

// renderAliasShadows renders simple projected ground shadows under alias model entities as darkened, flattened copies projected onto a plane below each entity.
func (r *Renderer) renderAliasShadows(entities []AliasModelEntity) {
	if len(entities) == 0 {
		return
	}
	if cvar.FloatValue(CvarRShadows) <= 0 {
		return
	}

	excludedModels := parseAliasShadowExclusions(cvar.StringValue(CvarRNoshadowList))

	const (
		shadowSegments = 16
		shadowAlpha    = 0.5
		shadowLift     = 0.1
		minShadowSize  = 8.0
		maxShadowSize  = 48.0
	)

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	r.ensureAliasScratchLocked()
	r.ensureAliasShadowTextureLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	shadowTexture := r.aliasShadowTexture
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	r.mu.Unlock()

	if program == 0 || scratchVAO == 0 || scratchVBO == 0 || shadowTexture == 0 || fallbackLightmap == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, shadowTexture)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, entity := range entities {
		modelID := strings.ToLower(entity.ModelID)
		if _, skip := excludedModels[modelID]; skip {
			continue
		}
		if entity.Model == nil || entity.Model.AliasHeader == nil {
			continue
		}
		if _, visible := visibleEntityAlpha(entity.Alpha); !visible {
			continue
		}

		modelScale := entity.Scale
		if modelScale == 0 {
			modelScale = 1
		}
		mins := entity.Model.Mins
		maxs := entity.Model.Maxs
		spanX := (maxs[0] - mins[0]) * modelScale
		spanY := (maxs[1] - mins[1]) * modelScale
		shadowRadius := 0.5 * float32(math.Max(float64(spanX), float64(spanY)))
		if shadowRadius < minShadowSize {
			shadowRadius = minShadowSize
		}
		if shadowRadius > maxShadowSize {
			shadowRadius = maxShadowSize
		}

		shadowZ := entity.Origin[2] + mins[2]*modelScale + shadowLift
		center := WorldVertex{
			Position:      [3]float32{entity.Origin[0], entity.Origin[1], shadowZ},
			TexCoord:      [2]float32{0.5, 0.5},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		vertices := make([]WorldVertex, 0, shadowSegments*3)
		for i := 0; i < shadowSegments; i++ {
			a0 := float32(i) * 2 * float32(math.Pi) / shadowSegments
			a1 := float32(i+1) * 2 * float32(math.Pi) / shadowSegments
			p0 := WorldVertex{
				Position: [3]float32{
					entity.Origin[0] + float32(math.Cos(float64(a0)))*shadowRadius,
					entity.Origin[1] + float32(math.Sin(float64(a0)))*shadowRadius,
					shadowZ,
				},
				TexCoord:      [2]float32{0, 0},
				LightmapCoord: [2]float32{},
				Normal:        [3]float32{0, 0, 1},
			}
			p1 := WorldVertex{
				Position: [3]float32{
					entity.Origin[0] + float32(math.Cos(float64(a1)))*shadowRadius,
					entity.Origin[1] + float32(math.Sin(float64(a1)))*shadowRadius,
					shadowZ,
				},
				TexCoord:      [2]float32{1, 1},
				LightmapCoord: [2]float32{},
				Normal:        [3]float32{0, 0, 1},
			}
			vertices = append(vertices, center, p0, p1)
		}

		vertexData := worldopengl.FlattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.Uniform1f(alphaUniform, shadowAlpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.DepthMask(true)
}

// renderViewModel renders the first-person weapon model with a narrower depth range (0..0.3) to prevent it from clipping into nearby walls. This depth range trick is a classic Quake rendering technique.
func (r *Renderer) renderViewModel(entity AliasModelEntity) {
	r.mu.Lock()
	draw := r.buildAliasDrawLocked(entity, true)
	r.mu.Unlock()
	if draw == nil {
		return
	}
	r.renderAliasDraws([]glAliasDraw{*draw}, true)
}

// parseAliasShadowExclusions parses the r_noshadow_list cvar into a set of model names that should not cast ground shadows.
func parseAliasShadowExclusions(value string) map[string]struct{} {
	return parseAliasModelList(value)
}

// ensureAliasShadowTextureLocked creates a 1x1 dark semi-transparent texture used for alias model ground shadows.
func (r *Renderer) ensureAliasShadowTextureLocked() {
	if r.aliasShadowTexture != 0 {
		return
	}
	r.aliasShadowTexture = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 255})
}

// ---- merged from world_upload_opengl_root.go ----
// ensureBrushModelLocked lazily builds and uploads GPU geometry for a BSP submodel (doors, platforms, lifts). Each brush entity references a submodel by index.
func (r *Renderer) ensureBrushModelLocked(submodelIndex int) *glWorldMesh {
	if mesh, ok := r.brushModels[submodelIndex]; ok && mesh != nil {
		return mesh
	}
	tree := r.worldTree
	if tree == nil {
		return nil
	}
	renderData, err := buildModelRenderData(tree, submodelIndex)
	if err != nil {
		slog.Warn("OpenGL brush model build failed", "submodel", submodelIndex, "error", err)
		return nil
	}
	if renderData == nil || renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		return nil
	}
	mesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if mesh == nil {
		return nil
	}
	mesh.faces = append(mesh.faces, renderData.Geometry.Faces...)
	mesh.hasLitWater = renderData.HasLitWater
	mesh.lightmapPages = append(mesh.lightmapPages, renderData.Lightmaps...)
	mesh.lightmaps = uploadLightmapPages(renderData.Lightmaps, r.lightStyleValues)
	r.brushModels[submodelIndex] = mesh
	return mesh
}

// worldTextureFilters returns GL texture filter parameters: lightmaps use LINEAR for smooth interpolation; diffuse textures use NEAREST_MIPMAP_LINEAR for Quake's pixel-art look with distance mipmapping.
func worldTextureFilters(lightmap bool) (minFilter, magFilter int32) {
	if lightmap {
		return gl.LINEAR, gl.LINEAR
	}
	return worldopengl.ParseTextureMode(cvar.StringValue(CvarGLTextureMode))
}

func readTextureAnisotropy() float32 {
	raw := float32(cvar.FloatValue(CvarGLAnisotropy))
	if raw < 1 {
		return 1
	}
	return raw
}

func readTextureLodBias() float32 {
	return float32(cvar.FloatValue(CvarGLLodBias))
}

// uploadWorldTextureRGBAWithFilters creates a GL texture from RGBA data with specified min/mag filters and generates mipmaps to reduce aliasing at distance.
func uploadWorldTextureRGBAWithFilters(width, height int, rgba []byte, minFilter, magFilter int32) uint32 {
	return worldopengl.UploadTextureRGBA(width, height, rgba, worldopengl.TextureUploadOptions{
		MinFilter:  minFilter,
		MagFilter:  magFilter,
		LodBias:    readTextureLodBias(),
		Anisotropy: readTextureAnisotropy(),
	})
}

// uploadWorldTextureRGBA uploads a world diffuse texture with NEAREST filtering for Quake's pixel-art aesthetic.
func uploadWorldTextureRGBA(width, height int, rgba []byte) uint32 {
	minFilter, magFilter := worldTextureFilters(false)
	return uploadWorldTextureRGBAWithFilters(width, height, rgba, minFilter, magFilter)
}

// uploadWorldLightmapTextureRGBA uploads a lightmap texture with LINEAR filtering for smooth lighting gradients.
func uploadWorldLightmapTextureRGBA(width, height int, rgba []byte) uint32 {
	minFilter, magFilter := worldTextureFilters(true)
	return uploadWorldTextureRGBAWithFilters(width, height, rgba, minFilter, magFilter)
}

// ensureWorldFallbackTextureLocked creates a 1x1 white fallback texture for faces missing their texture data, ensuring the shader always has a valid texture bound.
func (r *Renderer) ensureWorldFallbackTextureLocked() {
	if r.worldFallbackTexture != 0 {
		return
	}
	r.worldFallbackTexture = uploadWorldTextureRGBA(1, 1, []byte{200, 200, 200, 255})
}

// ensureLightmapFallbackTextureLocked creates a 1x1 white fallback lightmap so unlit faces render at full brightness.
func (r *Renderer) ensureLightmapFallbackTextureLocked() {
	if r.worldLightmapFallback != 0 {
		return
	}
	r.worldLightmapFallback = uploadWorldLightmapTextureRGBA(1, 1, []byte{255, 255, 255, 255})
}

// setLightStyleValues updates the 64-element lightstyle brightness array. Quake's lightstyle system animates lighting using 64 independent channels for effects like flickering torches and pulsing lights.
func (r *Renderer) setLightStyleValues(values [64]float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	changed := lightStylesChanged(r.lightStyleValues, values)
	if r.worldData != nil {
		markDirtyLightmapPages(r.worldData.Lightmaps, changed)
	}
	for _, mesh := range r.brushModels {
		if mesh == nil {
			continue
		}
		markDirtyLightmapPages(mesh.lightmapPages, changed)
	}

	r.lightStyleValues = values
	r.updateUploadedLightmapsLocked()
}

// defaultLightStyleValues returns the default lightstyle array where style 0 has brightness 1.0 (normal) and all others are 0.
func defaultLightStyleValues() [64]float32 {
	var values [64]float32
	values[0] = 1
	return values
}

// uploadWorldTexturesLocked uploads all world textures to the GPU: converts from palette to RGBA, extracts fullbright masks (palette indices 224-254 glow in the dark), splits sky textures into layers, and uploads to GL textures. Called once per map load.
func (r *Renderer) uploadWorldTexturesLocked(tree *bsp.Tree) error {
	r.worldTextures = make(map[int32]uint32)
	r.worldFullbrightTextures = make(map[int32]uint32)
	r.worldSkySolidTextures = make(map[int32]uint32)
	r.worldSkyAlphaTextures = make(map[int32]uint32)
	r.worldSkyFlatTextures = make(map[int32]uint32)
	r.worldTextureAnimations = nil
	r.ensureWorldFallbackTextureLocked()
	r.ensureWorldSkyFallbackTexturesLocked()

	if tree == nil || len(tree.TextureData) < 4 {
		return nil
	}

	palette := append([]byte(nil), r.palette...)
	plan := worldopengl.BuildTextureUploadPlan(tree, palette, ConvertPaletteToRGBA, ConvertPaletteToFullbrightRGBA)
	if len(plan.TextureNames) == 0 {
		return nil
	}
	uploaded := worldopengl.ApplyTextureUploadPlan(plan, uploadWorldTextureRGBA)
	r.worldTextures = uploaded.Diffuse
	r.worldFullbrightTextures = uploaded.Fullbright
	r.worldSkySolidTextures = uploaded.SkySolid
	r.worldSkyAlphaTextures = uploaded.SkyAlpha
	r.worldSkyFlatTextures = uploaded.SkyFlat

	animations, err := BuildTextureAnimations(plan.TextureNames)
	if err != nil {
		return fmt.Errorf("build world texture animations: %w", err)
	}
	r.worldTextureAnimations = animations
	return nil
}

func lightStylesChanged(old, new_ [64]float32) [64]bool {
	return worldopengl.LightStylesChanged(old, new_)
}

func markDirtyLightmapPages(pages []WorldLightmapPage, changed [64]bool) {
	worldopengl.MarkDirtyLightmapPages(pages, changed)
}

func clearDirtyFlags(pages []WorldLightmapPage) {
	worldopengl.ClearDirtyFlags(pages)
}

func buildLightmapPageRGBA(page *WorldLightmapPage, values [64]float32) []byte {
	return worldopengl.BuildLightmapPageRGBA(page, values)
}

func recompositeDirtySurfaces(rgba []byte, page WorldLightmapPage, values [64]float32) bool {
	return worldopengl.RecompositeDirtySurfaces(rgba, page, values)
}

func uploadLightmapPages(pages []WorldLightmapPage, values [64]float32) []uint32 {
	return worldopengl.UploadLightmapPages(pages, values, uploadWorldLightmapTextureRGBA)
}

func dirtyBounds(page WorldLightmapPage) (x, y, w, h int) {
	return worldopengl.DirtyBounds(page)
}

func updateLightmapTextures(textures []uint32, pages []WorldLightmapPage, values [64]float32) {
	worldopengl.UpdateLightmapTextures(textures, pages, values)
}

// updateUploadedLightmapsLocked rebuilds and re-uploads all lightmap pages with current lightstyle values.
func (r *Renderer) updateUploadedLightmapsLocked() {
	values := r.lightStyleValues
	if r.worldData != nil {
		updateLightmapTextures(r.worldLightmaps, r.worldData.Lightmaps, values)
	}
	for _, mesh := range r.brushModels {
		if mesh == nil {
			continue
		}
		updateLightmapTextures(mesh.lightmaps, mesh.lightmapPages, values)
	}
}

// UploadWorld builds CPU geometry and uploads it to OpenGL buffers.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.clearWorldLocked()
	r.worldLiquidAlphaOverrides = parseWorldspawnLiquidAlphaOverrides(tree.Entities)
	r.worldSkyFogOverride = parseWorldspawnSkyFogOverride(tree.Entities)

	renderData, err := buildWorldRenderData(tree)
	if err != nil {
		return fmt.Errorf("build world render data: %w", err)
	}
	r.worldLiquidFaceTypes = 0
	if renderData.Geometry != nil {
		r.worldLiquidFaceTypes = worldLiquidFaceTypeMask(renderData.Geometry.Faces)
	}
	if renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		r.worldData = renderData
		r.worldHasLitWater = renderData.HasLitWater
		return nil
	}

	if err := r.ensureWorldProgram(); err != nil {
		return err
	}
	if err := r.ensureWorldSkyPrograms(); err != nil {
		return err
	}
	r.worldTree = tree
	if err := r.uploadWorldTexturesLocked(tree); err != nil {
		return err
	}
	r.ensureLightmapFallbackTextureLocked()
	worldMesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if worldMesh == nil {
		return fmt.Errorf("upload world mesh: no geometry uploaded")
	}
	r.worldVAO = worldMesh.vao
	r.worldVBO = worldMesh.vbo
	r.worldEBO = worldMesh.ebo

	r.worldData = renderData
	r.worldHasLitWater = renderData.HasLitWater
	r.worldIndexCount = worldMesh.indexCount
	r.worldLightmaps = uploadLightmapPages(renderData.Lightmaps, r.lightStyleValues)

	slog.Debug("OpenGL world uploaded",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
	)
	return nil
}

// ---- merged from world_runtime_opengl_root.go ----
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

// clearWorldLocked releases all world GPU resources: textures, lightmaps, shader programs, VAOs/VBOs, and all cached brush/alias/sprite model data.
func (r *Renderer) clearWorldLocked() {
	worldopengl.DeleteVertexArrays(&r.worldVAO)
	worldopengl.DeleteBuffers(&r.worldVBO, &r.worldEBO)
	worldopengl.DeletePrograms(
		&r.worldProgram,
		&r.worldSkyProgram,
		&r.worldSkyProceduralProgram,
		&r.worldSkyCubemapProgram,
		&r.worldSkyExternalFaceProgram,
	)
	for idx, mesh := range r.brushModels {
		if mesh != nil {
			mesh.destroy()
		}
		delete(r.brushModels, idx)
	}
	worldopengl.DeleteTextureMap(r.worldTextures)
	worldopengl.DeleteTextureMap(r.worldFullbrightTextures)
	worldopengl.DeleteTextureMap(r.worldSkySolidTextures)
	worldopengl.DeleteTextureMap(r.worldSkyAlphaTextures)
	worldopengl.DeleteTextureMap(r.worldSkyFlatTextures)
	r.worldTextureAnimations = nil
	worldopengl.DeleteTextureSlice(r.worldLightmaps)
	r.worldLightmaps = nil
	worldopengl.DeleteTextures(
		&r.worldFallbackTexture,
		&r.worldLightmapFallback,
		&r.worldSkyAlphaFallback,
		&r.aliasShadowTexture,
	)
	r.clearExternalSkyboxLocked()
	worldopengl.SetInt32Fields(-1,
		&r.worldVPUniform,
		&r.worldTextureUniform,
		&r.worldLightmapUniform,
		&r.worldFullbrightUniform,
		&r.worldHasFullbrightUniform,
		&r.worldSkyVPUniform,
		&r.worldSkySolidUniform,
		&r.worldSkyAlphaUniform,
		&r.worldSkyProceduralVPUniform,
		&r.worldSkyProceduralModelOffset,
		&r.worldSkyProceduralModelRotation,
		&r.worldSkyProceduralModelScale,
		&r.worldSkyProceduralCameraOrigin,
		&r.worldSkyProceduralFogColor,
		&r.worldSkyProceduralFogDensity,
		&r.worldSkyProceduralHorizonColor,
		&r.worldSkyProceduralZenithColor,
		&r.worldSkyCubemapVPUniform,
		&r.worldSkyCubemapUniform,
		&r.worldSkyExternalFaceVPUniform,
		&r.worldSkyExternalFaceRTUniform,
		&r.worldSkyExternalFaceBKUniform,
		&r.worldSkyExternalFaceLFUniform,
		&r.worldSkyExternalFaceFTUniform,
		&r.worldSkyExternalFaceUPUniform,
		&r.worldSkyExternalFaceDNUniform,
		&r.worldModelOffsetUniform,
		&r.worldModelRotationUniform,
		&r.worldModelScaleUniform,
		&r.worldSkyModelOffsetUniform,
		&r.worldSkyModelRotationUniform,
		&r.worldSkyModelScaleUniform,
		&r.worldSkyCubemapModelOffsetUniform,
		&r.worldSkyCubemapModelRotationUniform,
		&r.worldSkyCubemapModelScaleUniform,
		&r.worldSkyExternalFaceModelOffset,
		&r.worldSkyExternalFaceModelRotation,
		&r.worldSkyExternalFaceModelScale,
		&r.worldAlphaUniform,
		&r.worldTimeUniform,
		&r.worldSkyTimeUniform,
		&r.worldSkySolidLayerSpeedUniform,
		&r.worldSkyAlphaLayerSpeedUniform,
		&r.worldTurbulentUniform,
		&r.worldLitWaterUniform,
		&r.worldCameraOriginUniform,
		&r.worldSkyCameraOriginUniform,
		&r.worldSkyCubemapCameraOriginUniform,
		&r.worldSkyExternalFaceCameraOrigin,
		&r.worldFogColorUniform,
		&r.worldSkyFogColorUniform,
		&r.worldSkyCubemapFogColorUniform,
		&r.worldSkyExternalFaceFogColor,
		&r.worldFogDensityUniform,
		&r.worldSkyFogDensityUniform,
		&r.worldSkyCubemapFogDensityUniform,
		&r.worldSkyExternalFaceFogDensity,
	)
	r.worldIndexCount = 0
	r.worldData = nil
	r.worldTree = nil
	r.worldHasLitWater = false
	r.worldLiquidFaceTypes = 0
	r.worldLiquidAlphaOverrides = worldLiquidAlphaOverrides{}
	r.worldSkyFogOverride = worldSkyFogOverride{}
	r.worldSkyExternalName = ""
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalRequestID = 0
	r.worldFogColor = [3]float32{}
	r.worldFogDensity = 0
	for modelID, alias := range r.aliasModels {
		if alias != nil {
			alias.destroy(r.worldFallbackTexture)
		}
		delete(r.aliasModels, modelID)
	}
	worldopengl.DeleteVertexArrays(&r.aliasScratchVAO, &r.decalVAO)
	worldopengl.DeleteBuffers(&r.aliasScratchVBO, &r.decalVBO)
	worldopengl.DeletePrograms(&r.decalProgram)
	worldopengl.SetInt32Fields(-1, &r.decalVPUniform)
}

// ClearWorld releases OpenGL world resources.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearWorldLocked()
}

// clearExternalSkyboxLocked deletes external skybox GL textures and resets to the embedded sky rendering mode.
func (r *Renderer) clearExternalSkyboxLocked() {
	worldopengl.DeleteTextures(&r.worldSkyExternalCubemap)
	worldopengl.DeleteTextureSlice(r.worldSkyExternalFaceTextures[:])
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

func (r *Renderer) hasTranslucentWorldLiquidFaces() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	liquidFaceTypes := r.worldLiquidFaceTypes
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	worldTree := r.worldTree
	r.mu.RUnlock()
	if liquidFaceTypes == 0 {
		return false
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	return hasTranslucentWorldLiquidFaceType(liquidFaceTypes, liquidAlpha)
}

// ClearTranslucentCalls resets the per-frame translucent draw call list for the next frame.
func (r *Renderer) ClearTranslucentCalls() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.translucentCalls = r.translucentCalls[:0]
}

// DrawTranslucentCalls renders the accumulated translucent draw calls sorted by distance from the camera for correct alpha blending order.
func (r *Renderer) DrawTranslucentCalls() {
	r.mu.RLock()
	if len(r.translucentCalls) == 0 {
		r.mu.RUnlock()
		return
	}
	calls := append([]worldDrawCall(nil), r.translucentCalls...)
	program := r.worldProgram
	vp := r.viewMatrices.VP
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
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraTime := r.cameraState.Time
	cameraOrigin := r.cameraState.Origin
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	timeUniform := r.worldTimeUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform

	oitProg := r.oitWorldProgram
	oitVP := r.oitWorldVPUniform
	oitTex := r.oitWorldTextureUniform
	oitLM := r.oitWorldLightmapUniform
	oitFB := r.oitWorldFullbrightUniform
	oitHasFB := r.oitWorldHasFullbrightUniform
	oitDL := r.oitWorldDynamicLightUniform
	oitOff := r.oitWorldModelOffsetUniform
	oitRot := r.oitWorldModelRotationUniform
	oitScl := r.oitWorldModelScaleUniform
	oitAlpha := r.oitWorldAlphaUniform
	oitTurb := r.oitWorldTurbulentUniform
	oitLitWater := r.oitWorldLitWaterUniform
	oitTime := r.oitWorldTimeUniform
	oitCamOrig := r.oitWorldCameraOriginUniform
	oitFogCol := r.oitWorldFogColorUniform
	oitFogDen := r.oitWorldFogDensityUniform
	r.mu.RUnlock()

	if program == 0 {
		return
	}

	if oitProg != 0 && GetAlphaMode() == AlphaModeOIT {
		program = oitProg
		vpUniform = oitVP
		textureUniform = oitTex
		lightmapUniform = oitLM
		fullbrightUniform = oitFB
		hasFullbrightUniform = oitHasFB
		dynamicLightUniform = oitDL
		modelOffsetUniform = oitOff
		modelRotationUniform = oitRot
		modelScaleUniform = oitScl
		alphaUniform = oitAlpha
		turbulentUniform = oitTurb
		litWaterUniform = oitLitWater
		timeUniform = oitTime
		cameraOriginUniform = oitCamOrig
		fogColorUniform = oitFogCol
		fogDensityUniform = oitFogDen
	}

	if shouldSortTranslucentCalls(GetAlphaMode()) {
		sort.SliceStable(calls, func(i, j int) bool {
			return calls[i].DistanceSq > calls[j].DistanceSq
		})
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 0)
	gl.Uniform1f(timeUniform, cameraTime)
	gl.Uniform3f(cameraOriginUniform, cameraOrigin.X, cameraOrigin.Y, cameraOrigin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))

	renderWorldDrawCalls(calls, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, false)

	gl.UseProgram(0)
}

// ---- merged from world_sky_pass_opengl_root.go ----

func worldExternalSkyMode(mode externalSkyboxRenderMode) worldopengl.ExternalSkyMode {
	switch mode {
	case externalSkyboxRenderCubemap:
		return worldopengl.ExternalSkyCubemap
	case externalSkyboxRenderFaces:
		return worldopengl.ExternalSkyFaces
	default:
		return worldopengl.ExternalSkyEmbedded
	}
}

// ---- merged from world_sky_support_opengl_root.go ----

// uploadSkyboxCubemap uploads 6 skybox face images as a GL_TEXTURE_CUBE_MAP, reordering faces from Quake convention (rt/bk/lf/ft/up/dn) to OpenGL convention (+X/-X/+Y/-Y/+Z/-Z).
func uploadSkyboxCubemap(faces [6]externalSkyboxFace, faceSize int) uint32 {
	return worldopengl.UploadSkyboxCubemap(faces, faceSize)
}

// uploadSkyboxFaceTextures uploads each skybox face as an individual GL_TEXTURE_2D, used as fallback when faces aren't all square and can't form a cubemap.
func uploadSkyboxFaceTextures(faces [6]externalSkyboxFace) (textures [6]uint32, ok bool) {
	return worldopengl.UploadSkyboxFaceTextures(faces)
}

// ---- merged from world_probe_opengl_root.go ----
type WorldFaceProbeStats = worldopengl.FaceProbeStats

func worldRenderPassName(pass worldRenderPass) string {
	return worldopengl.RenderPassName(pass)
}

// ProbeWorldFacesInBBox inspects world/model faces in an opt-in diagnostic path.
func ProbeWorldFacesInBBox(tree *bsp.Tree, modelIndex int, boundsMin, boundsMax [3]float32, liquidAlpha worldLiquidAlphaSettings) (*WorldFaceProbeStats, error) {
	return worldopengl.ProbeFacesInBBox(tree, modelIndex, boundsMin, boundsMax, liquidAlpha.toWorld())
}

// ---- merged from world_sprite_opengl_root.go ----
// renderSpriteEntities renders sprite entities as textured billboard quads with alpha blending, no depth write, and no backface culling. Sprite vertex positions are computed on CPU based on the sprite's orientation type.
func (r *Renderer) renderSpriteEntities(entities []SpriteEntity) {
	if len(entities) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity

	draws := make([]glSpriteDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildSpriteDrawLocked(entity); draw != nil {
			draws = append(draws, *draw)
		}
	}
	r.mu.Unlock()

	if program == 0 || len(draws) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	cameraForward, cameraRight, cameraUp := spriteCameraBasis([3]float32{
		camera.Angles.X,
		camera.Angles.Y,
		camera.Angles.Z,
	})

	for _, draw := range draws {
		r.renderSpriteDraw(draw, camera, cameraForward, cameraRight, cameraUp, program, modelOffsetUniform, alphaUniform, fallbackLightmap)
	}

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
}

// glSpriteDraw holds data for rendering a single sprite.
type glSpriteDraw struct {
	sprite *glSpriteModel
	model  *model.Model
	frame  int
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
}

// buildSpriteDrawLocked prepares a sprite for rendering (must be called with mutex held).
func (r *Renderer) buildSpriteDrawLocked(entity SpriteEntity) *glSpriteDraw {
	if entity.ModelID == "" || entity.Model == nil || entity.Model.Type != model.ModSprite {
		return nil
	}

	var spr *glSpriteModel
	if entity.SpriteData != nil {
		spr = uploadSpriteModel(entity.ModelID, entity.SpriteData)
	} else {
		spr = r.ensureSpriteLocked(entity.ModelID, entity.Model)
	}

	if spr == nil {
		return nil
	}

	frame := entity.Frame
	if frame < 0 || frame >= len(spr.frames) {
		frame = 0
	}

	return &glSpriteDraw{
		sprite: spr,
		model:  entity.Model,
		frame:  frame,
		origin: entity.Origin,
		angles: entity.Angles,
		alpha:  entity.Alpha,
		scale:  entity.Scale,
	}
}

// ensureSpriteLocked retrieves or creates a cached sprite model (must be called with mutex held).
func (r *Renderer) ensureSpriteLocked(modelID string, mdl *model.Model) *glSpriteModel {
	if modelID == "" || mdl == nil || mdl.Type != model.ModSprite {
		return nil
	}

	if cached, ok := r.spriteModels[modelID]; ok {
		return cached
	}

	spr := spriteDataFromModel(mdl)
	glsprite := uploadSpriteModel(modelID, spr)
	if glsprite == nil {
		return nil
	}

	r.spriteModels[modelID] = glsprite
	return glsprite
}

// renderSpriteDraw renders a single sprite billboard.
func (r *Renderer) renderSpriteDraw(draw glSpriteDraw, camera CameraState, cameraForward, cameraRight, cameraUp [3]float32, program uint32, modelOffsetUniform, alphaUniform int32, fallbackLightmap uint32) {
	if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
		return
	}

	vertices := buildSpriteQuadVertices(draw.sprite, draw.frame, [3]float32{
		camera.Origin.X,
		camera.Origin.Y,
		camera.Origin.Z,
	}, draw.origin, draw.angles, cameraForward, cameraRight, cameraUp, draw.scale)

	if len(vertices) == 0 {
		return
	}

	triangleVertices := expandSpriteQuadVertices(vertices)
	if len(triangleVertices) == 0 {
		return
	}
	worldVertices := spriteQuadVerticesToWorldVertices(triangleVertices)

	r.ensureAliasScratchLocked()

	worldopengl.DrawSpriteWorldVertices(worldVertices, worldopengl.SpriteDrawParams{
		VBO:                r.aliasScratchVBO,
		VAO:                r.aliasScratchVAO,
		ModelOffset:        draw.origin,
		Alpha:              draw.alpha,
		ModelOffsetUniform: modelOffsetUniform,
		AlphaUniform:       alphaUniform,
	})
}

func spriteQuadVerticesToWorldVertices(vertices []spriteQuadVertex) []WorldVertex {
	out := make([]WorldVertex, len(vertices))
	for i, vertex := range vertices {
		out[i] = WorldVertex{
			Position:      vertex.Position,
			TexCoord:      vertex.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
	}
	return out
}

// ---- merged from world_support_opengl_root.go ----
type glWorldMesh struct {
	vao           uint32
	vbo           uint32
	ebo           uint32
	indexCount    int32
	hasLitWater   bool
	faces         []WorldFace
	lightmaps     []uint32
	lightmapPages []WorldLightmapPage
}

func uploadWorldMesh(vertices []WorldVertex, indices []uint32) *glWorldMesh {
	uploaded := worldopengl.UploadWorldMesh(vertices, indices)
	if uploaded == nil {
		return nil
	}
	return &glWorldMesh{
		vao:        uploaded.VAO,
		vbo:        uploaded.VBO,
		ebo:        uploaded.EBO,
		indexCount: uploaded.IndexCount,
	}
}

func (mesh *glWorldMesh) destroy() {
	if mesh == nil {
		return
	}
	worldopengl.DeleteVertexArrays(&mesh.vao)
	worldopengl.DeleteBuffers(&mesh.vbo, &mesh.ebo)
	worldopengl.DeleteTextureSlice(mesh.lightmaps)
}

type worldDrawCall = worldopengl.DrawCall

type glAliasModel struct {
	modelID          string
	flags            int
	skins            []uint32
	fullbrightSkins  []uint32
	playerSkins      map[uint32][]uint32
	playerFullbright map[uint32][]uint32
	poses            [][]model.TriVertX
	refs             []aliasimpl.MeshRef
}

func (alias *glAliasModel) destroy(fallbackTexture uint32) {
	if alias == nil {
		return
	}
	worldopengl.DeleteTextureSliceExcept(alias.skins, fallbackTexture)
	worldopengl.DeleteTextureSliceExcept(alias.fullbrightSkins, fallbackTexture)
	worldopengl.DeleteTextureGroupsExcept(alias.playerSkins, fallbackTexture)
	worldopengl.DeleteTextureGroupsExcept(alias.playerFullbright, fallbackTexture)
}

type glAliasDraw struct {
	alias          *glAliasModel
	model          *model.Model
	pose1          int
	pose2          int
	blend          float32
	skin           uint32
	fullbrightSkin uint32
	origin         [3]float32
	angles         [3]float32
	alpha          float32
	scale          float32
	full           bool
}

// ---- merged from world_sky_opengl_root.go ----
// ensureWorldSkyPrograms lazily compiles all three sky shader variants: embedded two-layer scrolling sky, cubemap sky (GL_TEXTURE_CUBE_MAP for external skybox), and individual-face sky (fallback for non-uniform face sizes).
func (r *Renderer) ensureWorldSkyPrograms() error {
	if r.worldSkyProgram == 0 {
		vs, err := compileShader(worldopengl.WorldSkyVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky vertex shader: %w", err)
		}
		fs, err := compileShader(worldopengl.WorldSkyFragmentShaderGL, gl.FRAGMENT_SHADER)
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
		r.worldSkySolidLayerSpeedUniform = gl.GetUniformLocation(program, gl.Str("uSolidLayerSpeed\x00"))
		r.worldSkyAlphaLayerSpeedUniform = gl.GetUniformLocation(program, gl.Str("uAlphaLayerSpeed\x00"))
		r.worldSkyCameraOriginUniform = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
		r.worldSkyFogColorUniform = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
		r.worldSkyFogDensityUniform = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
	}

	if r.worldSkyProceduralProgram == 0 {
		vs, err := compileShader(worldopengl.WorldSkyVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky procedural vertex shader: %w", err)
		}
		fs, err := compileShader(worldopengl.WorldSkyProceduralFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile world sky procedural fragment shader: %w", err)
		}

		program := createProgram(vs, fs)
		r.worldSkyProceduralProgram = program
		r.worldSkyProceduralVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
		r.worldSkyProceduralModelOffset = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
		r.worldSkyProceduralModelRotation = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
		r.worldSkyProceduralModelScale = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
		r.worldSkyProceduralCameraOrigin = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
		r.worldSkyProceduralFogColor = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
		r.worldSkyProceduralFogDensity = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
		r.worldSkyProceduralHorizonColor = gl.GetUniformLocation(program, gl.Str("uHorizonColor\x00"))
		r.worldSkyProceduralZenithColor = gl.GetUniformLocation(program, gl.Str("uZenithColor\x00"))
	}

	if r.worldSkyCubemapProgram == 0 {
		vsCubemap, err := compileShader(worldopengl.WorldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky cubemap vertex shader: %w", err)
		}
		fsCubemap, err := compileShader(worldopengl.WorldSkyCubemapFragmentShaderGL, gl.FRAGMENT_SHADER)
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
		vsExternalFaces, err := compileShader(worldopengl.WorldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky external-face vertex shader: %w", err)
		}
		fsExternalFaces, err := compileShader(worldopengl.WorldSkyExternalFaceFragmentShaderGL, gl.FRAGMENT_SHADER)
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

// ensureWorldSkyFallbackTexturesLocked creates fallback sky textures: dark blue for the solid layer, transparent black for the alpha layer.
func (r *Renderer) ensureWorldSkyFallbackTexturesLocked() {
	r.ensureWorldFallbackTextureLocked()
	if r.worldSkyAlphaFallback != 0 {
		return
	}
	r.worldSkyAlphaFallback = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 0})
}

// ---- merged from world_sky_texture_opengl_root.go ----
// ---- merged from world_shader_opengl_root.go ----
