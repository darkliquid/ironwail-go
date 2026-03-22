//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/model"
)

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
		// Use sprite data directly from entity
		spr = uploadSpriteModel(entity.ModelID, entity.SpriteData)
	} else {
		// Fall back to cache
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

	// Ensure scratch VAO/VBO for transient geometry
	r.ensureAliasScratchLocked()

	// Upload vertices to scratch VBO
	vertexData := flattenWorldVertices(worldVertices)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.aliasScratchVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)

	// Set model offset (sprite origin)
	gl.Uniform3f(modelOffsetUniform, draw.origin[0], draw.origin[1], draw.origin[2])

	// Set alpha
	gl.Uniform1f(alphaUniform, draw.alpha)

	// Bind vertex array
	gl.BindVertexArray(r.aliasScratchVAO)

	// Draw sprite quad as 2 triangles using expanded transient vertices.
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(worldVertices)))
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
