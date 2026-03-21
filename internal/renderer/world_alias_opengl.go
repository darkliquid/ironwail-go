//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	"math"
	"strings"
)

type glAliasVertexRef struct {
	vertexIndex int
	texCoord    [2]float32
}

type glAliasModel struct {
	modelID          string
	flags            int
	skins            []uint32
	fullbrightSkins  []uint32
	playerSkins      map[uint32][]uint32
	playerFullbright map[uint32][]uint32
	poses            [][]model.TriVertX
	refs             []glAliasVertexRef
}

type glAliasDraw struct {
	alias          *glAliasModel
	model          *model.Model
	pose1          int     // First pose for interpolation
	pose2          int     // Second pose for interpolation
	blend          float32 // Blend factor between pose1 and pose2 (0 = pose1, 1 = pose2)
	skin           uint32
	fullbrightSkin uint32
	origin         [3]float32
	angles         [3]float32
	alpha          float32
	scale          float32
	full           bool
}

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

	refs := make([]glAliasVertexRef, 0, len(hdr.Triangles)*3)
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
			refs = append(refs, glAliasVertexRef{
				vertexIndex: idx,
				texCoord: [2]float32{
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

// buildAliasVertices builds world-space vertices for a single alias model pose without interpolation. Used for shadow rendering and static pose display.
func buildAliasVertices(alias *glAliasModel, mdl *model.Model, poseIndex int, origin, angles [3]float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil || poseIndex < 0 || poseIndex >= len(alias.poses) {
		return nil
	}
	pose := alias.poses[poseIndex]
	vertices := make([]WorldVertex, 0, len(alias.refs))
	for _, ref := range alias.refs {
		if ref.vertexIndex < 0 || ref.vertexIndex >= len(pose) {
			continue
		}
		compressed := pose[ref.vertexIndex]
		position := model.DecodeVertex(compressed, mdl.AliasHeader.Scale, mdl.AliasHeader.ScaleOrigin)
		normal := model.GetNormal(compressed.LightNormalIndex)
		if fullAngles {
			position = rotateAliasAngles(position, angles)
			normal = rotateAliasAngles(normal, angles)
		} else {
			position = rotateAliasYaw(position, angles[1])
			normal = rotateAliasYaw(normal, angles[1])
		}
		position[0] += origin[0]
		position[1] += origin[1]
		position[2] += origin[2]
		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      ref.texCoord,
			LightmapCoord: [2]float32{},
			Normal:        normal,
		})
	}
	return vertices
}

// rotateAliasAngles applies full pitch/yaw/roll rotation to a vertex position in alias model space.
func rotateAliasAngles(v [3]float32, angles [3]float32) [3]float32 {
	v = rotateAliasYaw(v, angles[1])
	v = rotateAliasPitch(v, angles[0])
	v = rotateAliasRoll(v, angles[2])
	return v
}

// rotateAliasYaw applies yaw-only rotation to a vertex, the most common rotation for monsters that don't pitch or roll.
func rotateAliasYaw(v [3]float32, yawDegrees float32) [3]float32 {
	if yawDegrees == 0 {
		return v
	}
	yaw := float32(math.Pi) * yawDegrees / 180.0
	sinYaw := float32(math.Sin(float64(yaw)))
	cosYaw := float32(math.Cos(float64(yaw)))
	return [3]float32{
		v[0]*cosYaw - v[1]*sinYaw,
		v[0]*sinYaw + v[1]*cosYaw,
		v[2],
	}
}

// rotateAliasPitch applies pitch rotation to a vertex position in alias model space.
func rotateAliasPitch(v [3]float32, pitchDegrees float32) [3]float32 {
	if pitchDegrees == 0 {
		return v
	}
	pitch := float32(math.Pi) * pitchDegrees / 180.0
	sinPitch := float32(math.Sin(float64(pitch)))
	cosPitch := float32(math.Cos(float64(pitch)))
	return [3]float32{
		v[0],
		v[1]*cosPitch - v[2]*sinPitch,
		v[1]*sinPitch + v[2]*cosPitch,
	}
}

// rotateAliasRoll applies roll rotation to a vertex position in alias model space.
func rotateAliasRoll(v [3]float32, rollDegrees float32) [3]float32 {
	if rollDegrees == 0 {
		return v
	}
	roll := float32(math.Pi) * rollDegrees / 180.0
	sinRoll := float32(math.Sin(float64(roll)))
	cosRoll := float32(math.Cos(float64(roll)))
	return [3]float32{
		v[0]*cosRoll + v[2]*sinRoll,
		v[1],
		-v[0]*sinRoll + v[2]*cosRoll,
	}
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
	interpData, err := SetupAliasFrame(state, aliasHeaderFromModel(hdr), entity.TimeSeconds, true, false, 1)
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
		vertexData := flattenWorldVertices(vertices)
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

		vertexData := flattenWorldVertices(vertices)
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
	fields := strings.Fields(strings.ToLower(value))
	if len(fields) == 0 {
		return nil
	}
	exclusions := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		exclusions[field] = struct{}{}
	}
	return exclusions
}

// ensureAliasShadowTextureLocked creates a 1x1 dark semi-transparent texture used for alias model ground shadows.
func (r *Renderer) ensureAliasShadowTextureLocked() {
	if r.aliasShadowTexture != 0 {
		return
	}
	r.aliasShadowTexture = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 255})
}
