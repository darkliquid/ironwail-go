//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
)

const (
	worldVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec2 aLightmapCoord;
layout(location = 3) in vec3 aNormal;

uniform mat4 uViewProjection;
uniform vec3 uModelOffset;

out vec2 vTexCoord;
out vec3 vNormal;
out vec3 vWorldPos;

void main() {
	vTexCoord = aTexCoord;
	vec3 worldPosition = aPosition + uModelOffset;
	vNormal = aNormal;
	vWorldPos = worldPosition;
	gl_Position = uViewProjection * vec4(worldPosition, 1.0);
}`

	worldFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec3 vNormal;
in vec3 vWorldPos;
out vec4 fragColor;

uniform sampler2D uTexture;
uniform float uAlpha;

void main() {
    vec3 n = normalize(vNormal);
    if (length(n) < 0.01) {
        n = vec3(0.0, 0.0, 1.0);
    }
    vec3 lightDir = normalize(vec3(0.35, 0.55, 0.75));
    float diffuse = max(dot(n, lightDir), 0.0);
    float ambient = 0.35;
    float shade = ambient + diffuse * 0.65;
	vec4 base = texture(uTexture, vTexCoord);
	if (base.a < 0.1) {
		discard;
	}
	fragColor = vec4(base.rgb * shade, base.a * uAlpha);
}`
)

type glWorldMesh struct {
	vao        uint32
	vbo        uint32
	ebo        uint32
	indexCount int32
	faces      []WorldFace
}

type glAliasVertexRef struct {
	vertexIndex int
	texCoord    [2]float32
}

type glAliasModel struct {
	modelID string
	flags   int
	skins   []uint32
	poses   [][]model.TriVertX
	refs    []glAliasVertexRef
}

func flattenWorldVertices(vertices []WorldVertex) []float32 {
	data := make([]float32, 0, len(vertices)*10)
	for _, v := range vertices {
		data = append(data,
			v.Position[0], v.Position[1], v.Position[2],
			v.TexCoord[0], v.TexCoord[1],
			v.LightmapCoord[0], v.LightmapCoord[1],
			v.Normal[0], v.Normal[1], v.Normal[2],
		)
	}
	return data
}

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
	r.worldModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
	r.worldAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlpha\x00"))
	return nil
}

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

func uploadWorldMesh(vertices []WorldVertex, indices []uint32) *glWorldMesh {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil
	}

	vertexData := flattenWorldVertices(vertices)
	mesh := &glWorldMesh{indexCount: int32(len(indices))}

	gl.GenVertexArrays(1, &mesh.vao)
	gl.GenBuffers(1, &mesh.vbo)
	gl.GenBuffers(1, &mesh.ebo)

	gl.BindVertexArray(mesh.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, mesh.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mesh.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

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
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	return mesh
}

func (mesh *glWorldMesh) destroy() {
	if mesh == nil {
		return
	}
	if mesh.vao != 0 {
		gl.DeleteVertexArrays(1, &mesh.vao)
		mesh.vao = 0
	}
	if mesh.vbo != 0 {
		gl.DeleteBuffers(1, &mesh.vbo)
		mesh.vbo = 0
	}
	if mesh.ebo != 0 {
		gl.DeleteBuffers(1, &mesh.ebo)
		mesh.ebo = 0
	}
}

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
	r.brushModels[submodelIndex] = mesh
	return mesh
}

func uploadWorldTextureRGBA(width, height int, rgba []byte) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return tex
}

func (r *Renderer) ensureWorldFallbackTextureLocked() {
	if r.worldFallbackTexture != 0 {
		return
	}
	r.worldFallbackTexture = uploadWorldTextureRGBA(1, 1, []byte{200, 200, 200, 255})
}

func (r *Renderer) uploadWorldTexturesLocked(tree *bsp.Tree) {
	r.worldTextures = make(map[int32]uint32)
	r.ensureWorldFallbackTextureLocked()

	if tree == nil || len(tree.TextureData) < 4 {
		return
	}

	palette := append([]byte(nil), r.palette...)
	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return
	}

	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			slog.Debug("OpenGL world texture parse failed", "index", i, "error", err)
			continue
		}
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		rgba := ConvertPaletteToRGBA(pixels, palette)
		tex := uploadWorldTextureRGBA(width, height, rgba)
		if tex != 0 {
			r.worldTextures[int32(i)] = tex
		}
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

	renderData, err := buildWorldRenderData(tree)
	if err != nil {
		return fmt.Errorf("build world render data: %w", err)
	}
	if renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		r.worldData = renderData
		return nil
	}

	if err := r.ensureWorldProgram(); err != nil {
		return err
	}
	r.worldTree = tree
	r.uploadWorldTexturesLocked(tree)
	worldMesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if worldMesh == nil {
		return fmt.Errorf("upload world mesh: no geometry uploaded")
	}
	r.worldVAO = worldMesh.vao
	r.worldVBO = worldMesh.vbo
	r.worldEBO = worldMesh.ebo

	r.worldData = renderData
	r.worldIndexCount = worldMesh.indexCount

	slog.Info("OpenGL world uploaded",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
	)
	return nil
}

func (r *Renderer) renderWorld() {
	r.mu.RLock()
	program := r.worldProgram
	vao := r.worldVAO
	indexCount := r.worldIndexCount
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	fallbackTexture := r.worldFallbackTexture
	faces := []WorldFace(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		faces = append(faces, r.worldData.Geometry.Faces...)
	}
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	r.mu.RUnlock()

	if program == 0 || vao == 0 || indexCount <= 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.CULL_FACE)

	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.Uniform1f(r.worldAlphaUniform, 1)
	gl.BindVertexArray(vao)
	gl.ActiveTexture(gl.TEXTURE0)
	if len(faces) == 0 {
		gl.BindTexture(gl.TEXTURE_2D, fallbackTexture)
		gl.DrawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
	} else {
		for _, face := range faces {
			tex := worldTextures[face.TextureIndex]
			if tex == 0 {
				tex = fallbackTexture
			}
			gl.BindTexture(gl.TEXTURE_2D, tex)
			gl.DrawElements(gl.TRIANGLES, int32(face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(face.FirstIndex*4)))
		}
	}
	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Enable(gl.BLEND)
}

func (r *Renderer) renderBrushEntities(entities []BrushEntity) {
	if len(entities) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	fallbackTexture := r.worldFallbackTexture
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	type drawBrush struct {
		origin [3]float32
		mesh   *glWorldMesh
	}
	brushes := make([]drawBrush, 0, len(entities))
	for _, entity := range entities {
		mesh := r.ensureBrushModelLocked(entity.SubmodelIndex)
		if mesh == nil {
			continue
		}
		brushes = append(brushes, drawBrush{origin: entity.Origin, mesh: mesh})
	}
	r.mu.Unlock()

	if program == 0 || len(brushes) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.CULL_FACE)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1f(r.worldAlphaUniform, 1)
	gl.ActiveTexture(gl.TEXTURE0)

	for _, brush := range brushes {
		gl.Uniform3f(modelOffsetUniform, brush.origin[0], brush.origin[1], brush.origin[2])
		gl.BindVertexArray(brush.mesh.vao)
		for _, face := range brush.mesh.faces {
			tex := worldTextures[face.TextureIndex]
			if tex == 0 {
				tex = fallbackTexture
			}
			gl.BindTexture(gl.TEXTURE_2D, tex)
			gl.DrawElements(gl.TRIANGLES, int32(face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(face.FirstIndex*4)))
		}
	}

	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Enable(gl.BLEND)
}

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
	for _, skin := range hdr.Skins {
		if len(skin) != hdr.SkinWidth*hdr.SkinHeight {
			skins = append(skins, r.worldFallbackTexture)
			continue
		}
		rgba := ConvertPaletteToRGBA(skin, palette)
		tex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, rgba)
		if tex == 0 {
			tex = r.worldFallbackTexture
		}
		skins = append(skins, tex)
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
		modelID: modelID,
		flags:   hdr.Flags,
		skins:   skins,
		poses:   hdr.Poses,
		refs:    refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func buildAliasVertices(alias *glAliasModel, mdl *model.Model, poseIndex int, origin, angles [3]float32) []WorldVertex {
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
		position = rotateAliasYaw(position, angles[1])
		normal = rotateAliasYaw(normal, angles[1])
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

func (r *Renderer) renderAliasEntities(entities []AliasModelEntity) {
	if len(entities) == 0 {
		return
	}

	type drawAlias struct {
		alias    *glAliasModel
		model    *model.Model
		pose     int
		skin     uint32
		origin   [3]float32
		angles   [3]float32
		alpha    float32
		fallback uint32
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	alphaUniform := r.worldAlphaUniform
	r.ensureAliasScratchLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	fallbackTexture := r.worldFallbackTexture
	draws := make([]drawAlias, 0, len(entities))
	for _, entity := range entities {
		alias := r.ensureAliasModelLocked(entity.ModelID, entity.Model)
		if alias == nil || entity.Model == nil || entity.Model.AliasHeader == nil || len(alias.refs) == 0 {
			continue
		}
		frame := entity.Frame
		if frame < 0 || frame >= len(entity.Model.AliasHeader.Frames) {
			frame = 0
		}
		pose := entity.Model.AliasHeader.Frames[frame].FirstPose
		if pose < 0 || pose >= len(alias.poses) {
			pose = 0
		}
		skin := fallbackTexture
		if len(alias.skins) > 0 {
			skinIndex := entity.SkinNum
			if skinIndex < 0 {
				skinIndex = 0
			}
			skin = alias.skins[skinIndex%len(alias.skins)]
			if skin == 0 {
				skin = fallbackTexture
			}
		}
		alpha := entity.Alpha
		if alpha <= 0 {
			alpha = 1
		}
		draws = append(draws, drawAlias{
			alias:    alias,
			model:    entity.Model,
			pose:     pose,
			skin:     skin,
			origin:   entity.Origin,
			angles:   entity.Angles,
			alpha:    alpha,
			fallback: fallbackTexture,
		})
	}
	r.mu.Unlock()

	if program == 0 || scratchVAO == 0 || scratchVBO == 0 || len(draws) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, draw := range draws {
		vertices := buildAliasVertices(draw.alias, draw.model, draw.pose, draw.origin, draw.angles)
		if len(vertices) == 0 {
			continue
		}
		vertexData := flattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.BindTexture(gl.TEXTURE_2D, draw.skin)
		gl.Uniform1f(alphaUniform, draw.alpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
}

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

func (r *Renderer) clearWorldLocked() {
	if r.worldVAO != 0 {
		gl.DeleteVertexArrays(1, &r.worldVAO)
		r.worldVAO = 0
	}
	if r.worldVBO != 0 {
		gl.DeleteBuffers(1, &r.worldVBO)
		r.worldVBO = 0
	}
	if r.worldEBO != 0 {
		gl.DeleteBuffers(1, &r.worldEBO)
		r.worldEBO = 0
	}
	if r.worldProgram != 0 {
		gl.DeleteProgram(r.worldProgram)
		r.worldProgram = 0
	}
	for idx, mesh := range r.brushModels {
		if mesh != nil {
			mesh.destroy()
		}
		delete(r.brushModels, idx)
	}
	for textureIndex, tex := range r.worldTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldTextures, textureIndex)
	}
	if r.worldFallbackTexture != 0 {
		gl.DeleteTextures(1, &r.worldFallbackTexture)
		r.worldFallbackTexture = 0
	}
	r.worldVPUniform = -1
	r.worldTextureUniform = -1
	r.worldModelOffsetUniform = -1
	r.worldAlphaUniform = -1
	r.worldIndexCount = 0
	r.worldData = nil
	r.worldTree = nil
	for modelID, alias := range r.aliasModels {
		if alias != nil {
			for _, tex := range alias.skins {
				if tex != 0 && tex != r.worldFallbackTexture {
					gl.DeleteTextures(1, &tex)
				}
			}
		}
		delete(r.aliasModels, modelID)
	}
	if r.aliasScratchVAO != 0 {
		gl.DeleteVertexArrays(1, &r.aliasScratchVAO)
		r.aliasScratchVAO = 0
	}
	if r.aliasScratchVBO != 0 {
		gl.DeleteBuffers(1, &r.aliasScratchVBO)
		r.aliasScratchVBO = 0
	}
}

// ClearWorld releases OpenGL world resources.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearWorldLocked()
}
