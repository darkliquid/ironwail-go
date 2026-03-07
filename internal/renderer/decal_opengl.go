//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"fmt"
	"math"
	"sort"

	"github.com/go-gl/gl/v4.6-core/gl"
)

const (
	decalVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec4 aColor;
layout(location = 3) in float aVariant;

uniform mat4 uViewProjection;

out vec2 vTexCoord;
out vec4 vColor;
out float vVariant;

void main() {
	vTexCoord = aTexCoord;
	vColor = aColor;
	vVariant = aVariant;
	gl_Position = uViewProjection * vec4(aPosition, 1.0);
}`

	decalFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec4 vColor;
in float vVariant;
out vec4 fragColor;

uniform sampler2D uAtlasTexture;

void main() {
	// Atlas is 2x2 grid: [Bullet, Chip] on top row, [Scorch, Magic] on bottom
	int variant = int(vVariant + 0.5);
	float atlasX = float(variant % 2) * 0.5;
	float atlasY = float(variant / 2) * 0.5;
	vec2 atlasUV = vec2(atlasX, atlasY) + vTexCoord * 0.5;
	
	vec4 texSample = texture(uAtlasTexture, atlasUV);
	
	// Apply edge fade based on distance from center
	vec2 p = vTexCoord * 2.0 - 1.0;
	float d2 = dot(p, p);
	if (d2 > 1.0) {
		discard;
	}
	float edge = smoothstep(1.0, 0.7, d2);
	
	fragColor = vec4(vColor.rgb * texSample.rgb, vColor.a * edge * texSample.a);
}`
)

type decalDraw struct {
	mark       DecalMarkEntity
	distanceSq float32
}

// generateDecalAtlasData creates a 256x256 RGBA atlas with 4 variants in a 2x2 grid.
// Each variant occupies a 128x128 region:
//   Top-left (0,0): Bullet - crack pattern
//   Top-right (128,0): Chip - concentrated chip
//   Bottom-left (0,128): Scorch - ring pattern
//   Bottom-right (128,128): Magic - swirl pattern
func generateDecalAtlasData() []byte {
	const atlasSize = 256
	const regionSize = 128
	data := make([]byte, atlasSize*atlasSize*4)
	
	for y := 0; y < atlasSize; y++ {
		for x := 0; x < atlasSize; x++ {
			// Determine which region we're in
			regionX := x / regionSize
			regionY := y / regionSize
			variant := regionY*2 + regionX
			
			// Local coordinates within region [0,1]
			localX := float32(x%regionSize) / float32(regionSize)
			localY := float32(y%regionSize) / float32(regionSize)
			
			// Center-relative coordinates [-1,1]
			px := localX*2.0 - 1.0
			py := localY*2.0 - 1.0
			d2 := px*px + py*py
			
			// Generate pattern based on variant
			var pattern float32
			switch variant {
			case 0: // Bullet - crack pattern
				crack := float32(math.Abs(math.Sin(float64(px*28.0)) * math.Cos(float64(py*19.0))))
				pattern = 0.55 + 0.45*crack
			case 1: // Chip - concentrated chip
				if d2 <= 1.0 {
					chip := float32(1.0 - smoothstepf(0.15, 0.9, d2))
					pattern = 0.5 + 0.5*chip
				} else {
					pattern = 0
				}
			case 2: // Scorch - ring pattern
				ring := float32(math.Abs(float64(d2 - 0.55)))
				pattern = 1.0 - smoothstepf(0.02, 0.26, ring)
			case 3: // Magic - swirl pattern
				angle := float32(math.Atan2(float64(py), float64(px)))
				swirl := 0.5 + 0.5*float32(math.Sin(float64(18.0*angle+20.0*d2)))
				pattern = 0.35 + 0.65*swirl
			}
			
			// Radial fade
			alpha := float32(1.0)
			if d2 > 1.0 {
				alpha = 0
			} else {
				alpha = smoothstepf(0.9, 0.5, d2)
			}
			
			// Write RGBA
			idx := (y*atlasSize + x) * 4
			val := byte(pattern * 255)
			data[idx+0] = val // R
			data[idx+1] = val // G
			data[idx+2] = val // B
			data[idx+3] = byte(alpha * 255) // A
		}
	}
	
	return data
}

func smoothstepf(edge0, edge1, x float32) float32 {
	t := clamp01((x - edge0) / (edge1 - edge0))
	return t * t * (3.0 - 2.0*t)
}

func (r *Renderer) ensureDecalProgramLocked() error {
	if r.decalProgram != 0 && r.decalVAO != 0 && r.decalVBO != 0 && r.decalAtlasTexture != 0 {
		return nil
	}

	vs, err := compileShader(decalVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile decal vertex shader: %w", err)
	}
	fs, err := compileShader(decalFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile decal fragment shader: %w", err)
	}
	program := createProgram(vs, fs)

	r.decalProgram = program
	r.decalVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	r.decalAtlasUniform = gl.GetUniformLocation(program, gl.Str("uAtlasTexture\x00"))

	gl.GenVertexArrays(1, &r.decalVAO)
	gl.GenBuffers(1, &r.decalVBO)

	gl.BindVertexArray(r.decalVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.decalVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 6*10*4, nil, gl.DYNAMIC_DRAW)

	const stride = 10 * 4
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 4, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 1, gl.FLOAT, false, stride, 9*4)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// Create and upload atlas texture
	if r.decalAtlasTexture == 0 {
		atlasData := generateDecalAtlasData()
		gl.GenTextures(1, &r.decalAtlasTexture)
		gl.BindTexture(gl.TEXTURE_2D, r.decalAtlasTexture)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 256, 256, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(atlasData))
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}

	return nil
}

func (r *Renderer) renderDecalMarks(marks []DecalMarkEntity) {
	if len(marks) == 0 {
		return
	}

	r.mu.Lock()
	if err := r.ensureDecalProgramLocked(); err != nil {
		r.mu.Unlock()
		return
	}
	program := r.decalProgram
	vpUniform := r.decalVPUniform
	atlasUniform := r.decalAtlasUniform
	atlasTexture := r.decalAtlasTexture
	vao := r.decalVAO
	vbo := r.decalVBO
	vp := r.viewMatrices.VP
	camera := r.cameraState
	r.mu.Unlock()

	draws := prepareDecalDraws(marks, camera)
	if len(draws) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Enable(gl.POLYGON_OFFSET_FILL)
	gl.PolygonOffset(-1.0, -1.0)

	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, atlasTexture)
	gl.Uniform1i(atlasUniform, 0)
	gl.BindVertexArray(vao)

	for _, draw := range draws {
		verts := buildDecalTriangleVertices(draw.mark)
		if len(verts) == 0 {
			continue
		}
		gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ARRAY_BUFFER, len(verts)*4, gl.Ptr(verts), gl.DYNAMIC_DRAW)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(verts)/9))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Disable(gl.POLYGON_OFFSET_FILL)
	gl.DepthMask(true)
}

func prepareDecalDraws(marks []DecalMarkEntity, camera CameraState) []decalDraw {
	draws := make([]decalDraw, 0, len(marks))
	for _, mark := range marks {
		if mark.Size <= 0 {
			continue
		}
		if mark.Normal == ([3]float32{}) {
			mark.Normal = [3]float32{0, 0, 1}
		}
		mark.Alpha = clamp01(mark.Alpha)
		mark.Variant = normalizeDecalVariant(mark.Variant)
		if mark.Alpha <= 0 {
			continue
		}
		draws = append(draws, decalDraw{mark: mark, distanceSq: decalDistanceSq(mark.Origin, camera)})
	}

	sort.SliceStable(draws, func(i, j int) bool {
		return draws[i].distanceSq > draws[j].distanceSq
	})

	return draws
}

func decalDistanceSq(origin [3]float32, camera CameraState) float32 {
	dx := origin[0] - camera.Origin.X
	dy := origin[1] - camera.Origin.Y
	dz := origin[2] - camera.Origin.Z
	return dx*dx + dy*dy + dz*dz
}

func buildDecalTriangleVertices(mark DecalMarkEntity) []float32 {
	corners, ok := buildDecalQuad(mark)
	if !ok {
		return nil
	}

	color := [4]float32{clamp01(mark.Color[0]), clamp01(mark.Color[1]), clamp01(mark.Color[2]), clamp01(mark.Alpha)}
	variant := float32(mark.Variant)
	uv := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	indices := [6]int{0, 1, 2, 0, 2, 3}

	out := make([]float32, 0, 6*10)
	for _, idx := range indices {
		corner := corners[idx]
		coord := uv[idx]
		out = append(out,
			corner[0], corner[1], corner[2],
			coord[0], coord[1],
			color[0], color[1], color[2], color[3],
			variant,
		)
	}
	return out
}

func normalizeDecalVariant(variant DecalVariant) DecalVariant {
	switch variant {
	case DecalVariantBullet, DecalVariantChip, DecalVariantScorch, DecalVariantMagic:
		return variant
	default:
		return DecalVariantBullet
	}
}

func buildDecalQuad(mark DecalMarkEntity) ([4][3]float32, bool) {
	var corners [4][3]float32
	normal, ok := decalNormalize3(mark.Normal)
	if !ok {
		return corners, false
	}

	tangent, bitangent := buildDecalBasis(normal, mark.Rotation)
	half := mark.Size * 0.5
	if half <= 0 {
		return corners, false
	}

	center := mark.Origin
	eps := float32(0.05)
	center = add3(center, scale3(normal, eps))

	offsets := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for i, o := range offsets {
		a := scale3(tangent, o[0]*half)
		b := scale3(bitangent, o[1]*half)
		corners[i] = add3(add3(center, a), b)
	}

	return corners, true
}

func buildDecalBasis(normal [3]float32, rotation float32) (tangent [3]float32, bitangent [3]float32) {
	up := [3]float32{0, 0, 1}
	if float32(math.Abs(float64(normal[2]))) > 0.99 {
		up = [3]float32{0, 1, 0}
	}

	tangent, _ = decalNormalize3(cross3(up, normal))
	bitangent = cross3(normal, tangent)

	if rotation != 0 {
		c := float32(math.Cos(float64(rotation)))
		s := float32(math.Sin(float64(rotation)))
		rotT := add3(scale3(tangent, c), scale3(bitangent, s))
		rotB := add3(scale3(bitangent, c), scale3(tangent, -s))
		tangent, _ = decalNormalize3(rotT)
		bitangent, _ = decalNormalize3(rotB)
	}

	return tangent, bitangent
}

func add3(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func scale3(a [3]float32, s float32) [3]float32 {
	return [3]float32{a[0] * s, a[1] * s, a[2] * s}
}

func cross3(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func decalNormalize3(v [3]float32) ([3]float32, bool) {
	lengthSq := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
	if lengthSq <= 1e-12 {
		return [3]float32{}, false
	}
	invLen := float32(1.0 / math.Sqrt(float64(lengthSq)))
	return [3]float32{v[0] * invLen, v[1] * invLen, v[2] * invLen}, true
}
