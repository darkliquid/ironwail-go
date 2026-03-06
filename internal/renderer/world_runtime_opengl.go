//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
)

const (
	worldVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec2 aLightmapCoord;
layout(location = 3) in vec3 aNormal;

uniform mat4 uViewProjection;

out vec2 vTexCoord;
out vec3 vNormal;
out vec3 vWorldPos;

void main() {
	vTexCoord = aTexCoord;
    vNormal = aNormal;
    vWorldPos = aPosition;
    gl_Position = uViewProjection * vec4(aPosition, 1.0);
}`

	worldFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec3 vNormal;
in vec3 vWorldPos;
out vec4 fragColor;

uniform sampler2D uTexture;

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
	fragColor = vec4(base.rgb * shade, base.a);
}`
)

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
	return nil
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
	r.uploadWorldTexturesLocked(tree)

	vertexData := flattenWorldVertices(renderData.Geometry.Vertices)

	gl.GenVertexArrays(1, &r.worldVAO)
	gl.GenBuffers(1, &r.worldVBO)
	gl.GenBuffers(1, &r.worldEBO)

	gl.BindVertexArray(r.worldVAO)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.worldVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.worldEBO)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(renderData.Geometry.Indices)*4, gl.Ptr(renderData.Geometry.Indices), gl.STATIC_DRAW)

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

	r.worldData = renderData
	r.worldIndexCount = int32(len(renderData.Geometry.Indices))

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
	r.worldIndexCount = 0
	r.worldData = nil
}

// ClearWorld releases OpenGL world resources.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearWorldLocked()
}
