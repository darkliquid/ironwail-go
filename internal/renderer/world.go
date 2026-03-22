//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

// Uniforms structure for world rendering, must match WGSL Uniforms struct
type WorldUniforms struct {
	ViewProjection [16]float32 // mat4x4
	CameraOrigin   [3]float32
	FogDensity     float32
	FogColor       [3]float32
	_AlphaPad      float32
}

const worldUniformBufferSize = 96

// WorldGeometry holds preprocessed BSP world data ready for GPU upload.
// This structure bridges the gap between BSP file format and GPU rendering.
type WorldGeometry struct {
	// Vertices stores all unique world vertices (position + tex coords + lightmap coords)
	Vertices []WorldVertex

	// Indices stores triangle indices for indexed drawing
	// BSP faces are converted to triangles (fan triangulation)
	Indices []uint32

	// Faces stores metadata for each BSP face
	Faces []WorldFace

	// Lightmaps stores allocated static lightmap atlas pages for faces with BSP lighting.
	Lightmaps []WorldLightmapPage

	// Tree is the original BSP tree (kept for PVS, collision, etc)
	Tree *bsp.Tree
}

// WorldVertex represents a single vertex in world geometry.
// Layout matches what the GPU shader expects.
type WorldVertex struct {
	// Position in world space (X, Y, Z)
	Position [3]float32

	// TexCoord for diffuse texture sampling (U, V)
	TexCoord [2]float32

	// LightmapCoord for lightmap texture sampling (U, V)
	LightmapCoord [2]float32

	// Normal for lighting calculations (X, Y, Z)
	Normal [3]float32
}

// WorldFace stores rendering metadata for a BSP face.
type WorldFace struct {
	// FirstIndex is the index into the Indices array
	FirstIndex uint32

	// NumIndices is the number of indices (triangles * 3)
	NumIndices uint32

	// TextureIndex identifies which texture to use
	TextureIndex int32

	// LightmapIndex identifies which lightmap to use (-1 = none)
	LightmapIndex int32

	// Flags control rendering behavior (sky, water, transparent, etc)
	Flags int32
}

const worldDepthTextureFormat = gputypes.TextureFormatDepth24Plus

// WorldRenderData holds GPU-side resources for world rendering.
// This is what gets uploaded to the GPU and used during rendering.
type WorldRenderData struct {
	// Geometry holds preprocessed vertex/index data
	Geometry *WorldGeometry

	// BoundsMin is the minimum XYZ world-space coordinate of uploaded geometry.
	BoundsMin [3]float32
	// BoundsMax is the maximum XYZ world-space coordinate of uploaded geometry.
	BoundsMax [3]float32

	// Backend resource status used for diagnostics and parity tracking.
	VertexBufferUploaded bool
	IndexBufferUploaded  bool
	HasDiffuseTextures   bool
	HasLightmapTextures  bool
	HasDepthBuffer       bool

	// Stats for debugging
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

type gpuWorldTexture struct {
	texture   hal.Texture
	view      hal.TextureView
	bindGroup hal.BindGroup
}

type WorldLightmapSurface struct {
	X       int
	Y       int
	Width   int
	Height  int
	Styles  [bsp.MaxLightmaps]uint8
	Samples []byte
	Dirty   bool
}

type WorldLightmapPage struct {
	Width    int
	Height   int
	Surfaces []WorldLightmapSurface
	Dirty    bool
	rgba     []byte
}

type faceLightmapSurface struct {
	pageIndex int
}

const worldLightmapPageSize = 1024

// BuildWorldGeometry extracts renderable geometry from a BSP tree.
// This converts the BSP's face/edge/vertex structure into a simple
// vertex buffer + index buffer suitable for GPU rendering.
//
// The function:
// - Iterates all faces in the world model (model 0)
// - Extracts vertices via the edge/surfedge indirection
// - Computes texture coordinates from TexInfo
// - Triangulates faces using fan triangulation
// - Computes normals from plane data
//
// For MVP implementation, this processes ALL faces without culling.
// Future optimization: PVS culling, frustum culling, face sorting.
func BuildWorldGeometry(tree *bsp.Tree) (*WorldGeometry, error) {
	return BuildModelGeometry(tree, 0)
}

// BuildModelGeometry extracts renderable geometry for a specific BSP model index.
func BuildModelGeometry(tree *bsp.Tree, modelIndex int) (*WorldGeometry, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}

	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	worldModel := tree.Models[modelIndex]

	geom := &WorldGeometry{
		Vertices: make([]WorldVertex, 0, 4096),
		Indices:  make([]uint32, 0, 16384),
		Faces:    make([]WorldFace, 0, 256),
		Tree:     tree,
	}
	lightmapAllocator, err := NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		return nil, fmt.Errorf("create lightmap allocator: %w", err)
	}
	lightmapPages := make([]WorldLightmapPage, 0, 4)
	textureMeta := parseWorldTextureMeta(tree)

	// Process all faces in the selected model.
	numFaces := int(worldModel.NumFaces)
	firstFace := int(worldModel.FirstFace)

	slog.Debug("Building world geometry",
		"numFaces", numFaces,
		"numVertices", len(tree.Vertexes),
		"numEdges", len(tree.Edges))

	for faceIdx := 0; faceIdx < numFaces; faceIdx++ {
		globalFaceIdx := firstFace + faceIdx
		if globalFaceIdx >= len(tree.Faces) {
			break
		}

		face := &tree.Faces[globalFaceIdx]

		// Extract face metadata
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			NumIndices:    0, // Will be computed during triangulation
			TextureIndex:  worldFaceTextureIndex(tree, face),
			LightmapIndex: -1,
			Flags:         worldFaceFlags(textureMeta, tree, face),
		}

		// Extract vertices for this face
		faceVerts, lightmapSurface, err := extractFaceVertices(tree, face, lightmapAllocator, &lightmapPages)
		if err != nil {
			slog.Warn("Failed to extract face vertices",
				"faceIdx", globalFaceIdx,
				"error", err)
			continue
		}

		if len(faceVerts) < 3 {
			// Skip degenerate faces
			continue
		}
		if lightmapSurface != nil {
			faceData.LightmapIndex = int32(lightmapSurface.pageIndex)
		}

		// Triangulate face using fan triangulation
		// Face with N vertices becomes (N-2) triangles
		baseVertIdx := uint32(len(geom.Vertices))

		// Add all vertices for this face
		geom.Vertices = append(geom.Vertices, faceVerts...)

		// Generate triangle indices (fan triangulation around vertex 0)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,             // Vertex 0 (fan center)
				baseVertIdx+uint32(i),   // Vertex i
				baseVertIdx+uint32(i+1)) // Vertex i+1
		}

		faceData.NumIndices = uint32((len(faceVerts) - 2) * 3)
		geom.Faces = append(geom.Faces, faceData)
	}

	slog.Info("World geometry built",
		"vertices", len(geom.Vertices),
		"indices", len(geom.Indices),
		"faces", len(geom.Faces),
		"triangles", len(geom.Indices)/3)

	geom.Lightmaps = lightmapPages
	return geom, nil
}

// extractFaceVertices extracts all vertices for a BSP face.
// It follows the edge/surfedge indirection to get vertex positions,
// then computes texture/lightmap coords and normals.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, allocator *LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	numEdges := int(face.NumEdges)
	if numEdges < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, numEdges)
	rawLightmapCoords := make([][2]float32, 0, numEdges)

	// Get plane normal for this face
	var normal [3]float32
	if int(face.PlaneNum) < len(tree.Planes) {
		planeNormal := tree.Planes[face.PlaneNum].Normal
		normal = planeNormal
		// If face is on back side of plane, flip normal
		if face.Side != 0 {
			normal[0] = -normal[0]
			normal[1] = -normal[1]
			normal[2] = -normal[2]
		}
	} else {
		// Invalid plane number - log warning
		slog.Warn("Invalid plane number for face",
			"planeNum", face.PlaneNum,
			"numPlanes", len(tree.Planes))
	}

	// Check if normal is valid (not all zeros)
	normalLen := float32(math.Sqrt(float64(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])))
	if normalLen < 0.01 {
		slog.Warn("Invalid normal for face",
			"faceIdx", face,
			"normalLen", normalLen)
	}

	texInfo := worldFaceTexInfo(tree, face)
	textureWidth, textureHeight := worldTextureDimensions(tree, texInfo)

	// Iterate through edges to extract vertex positions
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}

		surfEdge := tree.Surfedges[surfEdgeIdx]

		// Surfedge is signed: positive = use edge V[0], negative = use edge V[1]
		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			return nil, nil, fmt.Errorf("vertex index %d out of range", vertIdx)
		}

		position := tree.Vertexes[vertIdx].Point

		texCoord := [2]float32{0.0, 0.0}
		lightmapCoord := [2]float32{0.0, 0.0}
		if texInfo != nil {
			u := position[0]*texInfo.Vecs[0][0] + position[1]*texInfo.Vecs[0][1] + position[2]*texInfo.Vecs[0][2] + texInfo.Vecs[0][3]
			v := position[0]*texInfo.Vecs[1][0] + position[1]*texInfo.Vecs[1][1] + position[2]*texInfo.Vecs[1][2] + texInfo.Vecs[1][3]
			texCoord = [2]float32{u / textureWidth, v / textureHeight}
			rawLightmapCoords = append(rawLightmapCoords, [2]float32{u, v})
		}

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: lightmapCoord,
			Normal:        normal,
		})
	}

	lightmapSurface, err := assignFaceLightmap(vertices, rawLightmapCoords, face, tree, allocator, pages)
	if err != nil {
		return nil, nil, err
	}
	return vertices, lightmapSurface, nil
}

// worldFaceTexInfo resolves the texture-info record for a BSP face, which maps geometric vertices into texture/lightmap UV space.
func worldFaceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	if tree == nil || face == nil {
		return nil
	}
	if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
		return nil
	}
	return &tree.Texinfo[face.Texinfo]
}

// worldFaceTextureIndex resolves the diffuse texture atlas slot for a face so world pass shaders can sample the correct base map.
func worldFaceTextureIndex(tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil || texInfo.Miptex < 0 {
		return -1
	}
	return texInfo.Miptex
}

// worldFaceLightmapIndex returns the lightmap atlas page/index used for static lighting lookup during world shading.
func worldFaceLightmapIndex(face *bsp.TreeFace) int32 {
	if face == nil || face.LightOfs < 0 || face.Styles[0] == 255 {
		return -1
	}
	// gogpu path does not allocate lightmap pages yet; keep a stable "present" sentinel.
	return 0
}

// worldFaceFlags exposes per-face material/render flags (sky, liquid, turbulent, etc.) that drive pass routing and shader behavior.
func worldFaceFlags(textureMeta []worldTextureMeta, tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil {
		return 0
	}
	textureType := classifyWorldTextureName("")
	if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
		textureType = textureMeta[texInfo.Miptex].Type
	}
	return deriveWorldFaceFlags(textureType, texInfo.Flags)
}

// worldTextureDimensions fetches source texture dimensions for texel-density and UV conversion computations.
func worldTextureDimensions(tree *bsp.Tree, texInfo *bsp.Texinfo) (float32, float32) {
	textureWidth := float32(1)
	textureHeight := float32(1)
	if tree == nil || texInfo == nil || texInfo.Miptex < 0 || len(tree.TextureData) < 4 {
		return textureWidth, textureHeight
	}

	textureCount := int(int32(binary.LittleEndian.Uint32(tree.TextureData[:4])))
	miptexIndex := int(texInfo.Miptex)
	if miptexIndex < 0 || miptexIndex >= textureCount {
		return textureWidth, textureHeight
	}
	offsetTableEnd := 4 + textureCount*4
	if len(tree.TextureData) < offsetTableEnd {
		return textureWidth, textureHeight
	}

	offsetPos := 4 + miptexIndex*4
	offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[offsetPos : offsetPos+4])))
	if offset <= 0 || offset >= len(tree.TextureData) {
		return textureWidth, textureHeight
	}

	miptex, err := image.ParseMipTex(tree.TextureData[offset:])
	if err != nil {
		return textureWidth, textureHeight
	}

	if miptex.Width > 0 {
		textureWidth = float32(miptex.Width)
	}
	if miptex.Height > 0 {
		textureHeight = float32(miptex.Height)
	}
	return textureWidth, textureHeight
}

// worldVertexShaderWGSL is the WGSL source for world vertex shader
const worldVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    _pad0: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    
    let worldPos = vec4<f32>(input.position, 1.0);
    let clipPos = uniforms.viewProjection * worldPos;
    output.clipPosition = clipPos;
    
    output.texCoord = input.texCoord;
    output.lightmapCoord = input.lightmapCoord;
    output.worldPos = input.position;
    output.normal = input.normal;
    output.clipPos = clipPos;
    
    return output;
}
`

// worldFragmentShaderWGSL is the WGSL source for world fragment shader.
// MVP path uses a constant lit color until texture/lightmap bindings are
// fully wired.
const worldFragmentShaderWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    _pad0: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
	let sampled = textureSample(worldTexture, worldSampler, input.texCoord);
	if (sampled.a < 0.5) {
		discard;
	}
	let lightmap = textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb;
	let fogPosition = input.worldPos - uniforms.cameraOrigin;
	let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
	return vec4<f32>(mix(uniforms.fogColor, sampled.rgb*lightmap, fog), sampled.a);
}
`

// compileWorldShader compiles a WGSL shader to SPIR-V bytecode
// For now, we pass WGSL directly to HAL which handles compilation internally
func compileWorldShader(source string) string {
	// Return WGSL source directly - HAL will compile it
	return source
}

// createWorldShaderModule creates a HAL shader module from WGSL source
func createWorldShaderModule(device hal.Device, wgslSource string, label string) (hal.ShaderModule, error) {
	shaderModule, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: label,
		Source: hal.ShaderSource{
			WGSL: wgslSource,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create shader module: %w", err)
	}

	return shaderModule, nil
}

// createWorldVertexBuffer uploads vertex data to GPU
func (r *Renderer) createWorldVertexBuffer(device hal.Device, queue hal.Queue, vertices []WorldVertex) (hal.Buffer, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("no vertices to upload")
	}

	// Calculate size
	vertexSize := uint64(len(vertices)) * 44 // sizeof(WorldVertex) = 44 bytes

	slog.Debug("Creating world vertex buffer",
		"vertexCount", len(vertices),
		"sizeBytes", vertexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "World Vertices",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create vertex buffer: %w", err)
	}

	// Write vertex data to buffer
	vertexData := make([]byte, vertexSize)
	for i, v := range vertices {
		offset := uint64(i) * 44

		// Write position (3 float32 = 12 bytes)
		posBytes := float32ToBytes(v.Position[:])
		copy(vertexData[offset:offset+12], posBytes)

		// Write texCoord (2 float32 = 8 bytes)
		texBytes := float32ToBytes(v.TexCoord[:])
		copy(vertexData[offset+12:offset+20], texBytes)

		// Write lightmapCoord (2 float32 = 8 bytes)
		lightBytes := float32ToBytes(v.LightmapCoord[:])
		copy(vertexData[offset+20:offset+28], lightBytes)

		// Write normal (3 float32 = 12 bytes)
		normBytes := float32ToBytes(v.Normal[:])
		copy(vertexData[offset+28:offset+40], normBytes)
	}

	queue.WriteBuffer(buffer, 0, vertexData)

	slog.Info("World vertex buffer uploaded", "vertices", len(vertices))

	return buffer, nil
}

// createWorldIndexBuffer uploads index data to GPU
func (r *Renderer) createWorldIndexBuffer(device hal.Device, queue hal.Queue, indices []uint32) (hal.Buffer, uint32, error) {
	if len(indices) == 0 {
		return nil, 0, fmt.Errorf("no indices to upload")
	}

	indexSize := uint64(len(indices)) * 4 // uint32 = 4 bytes

	slog.Debug("Creating world index buffer",
		"indexCount", len(indices),
		"sizeBytes", indexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "World Indices",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create index buffer: %w", err)
	}

	// Write index data to buffer
	indexData := make([]byte, indexSize)
	for i, idx := range indices {
		offset := uint64(i) * 4
		binary.LittleEndian.PutUint32(indexData[offset:offset+4], idx)
	}

	queue.WriteBuffer(buffer, 0, indexData)

	slog.Info("World index buffer uploaded", "indices", len(indices))

	return buffer, uint32(len(indices)), nil
}

// createWorldRenderTarget ensures the GoGPU world scene target exists for the current framebuffer size.
func (r *Renderer) createWorldRenderTarget() error {
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid window size: %dx%d", width, height)
	}
	device := r.getHALDevice()
	if device == nil {
		return fmt.Errorf("nil HAL device")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureWorldRenderTargetLocked(device, width, height)
}

// createWorldPipeline creates the render pipeline for world rendering.
// Configures all pipeline state: vertex layout, shaders, depth-stencil, primitive topology, etc.
func (r *Renderer) createWorldPipeline(device hal.Device, vertexShader, fragmentShader hal.ShaderModule) (hal.RenderPipeline, hal.PipelineLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil {
		return nil, nil, fmt.Errorf("invalid shader modules or device")
	}

	// Define vertex buffer layout for WorldVertex (44 bytes total)
	// Layout: Position(12B) + TexCoord(8B) + LightmapCoord(8B) + Normal(12B) + Padding(4B)
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{
				Format:         gputypes.VertexFormatFloat32x3, // Position
				Offset:         0,
				ShaderLocation: 0,
			},
			{
				Format:         gputypes.VertexFormatFloat32x2, // TexCoord
				Offset:         12,
				ShaderLocation: 1,
			},
			{
				Format:         gputypes.VertexFormatFloat32x2, // LightmapCoord
				Offset:         20,
				ShaderLocation: 2,
			},
			{
				Format:         gputypes.VertexFormatFloat32x3, // Normal
				Offset:         28,
				ShaderLocation: 3,
			},
		},
	}

	// Create bind group layout for @group(0) @binding(0) uniform buffer.
	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "World Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   worldUniformBufferSize,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create uniform bind group layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "World Texture BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		uniformLayout.Destroy()
		return nil, nil, fmt.Errorf("create texture bind group layout: %w", err)
	}

	// Create pipeline layout with the uniform bind group layout.
	pipelineLayoutDesc := &hal.PipelineLayoutDescriptor{
		Label:            "World Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout, textureLayout, textureLayout},
	}

	pipelineLayout, err := device.CreatePipelineLayout(pipelineLayoutDesc)
	if err != nil {
		textureLayout.Destroy()
		uniformLayout.Destroy()
		return nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	r.mu.Lock()
	r.uniformBindGroupLayout = uniformLayout
	r.textureBindGroupLayout = textureLayout
	r.mu.Unlock()

	// Define primitive state.
	// Disable face culling for now: BSP triangulation winding can vary and
	// aggressive culling can drop all visible geometry in this MVP renderer.
	primitiveState := gputypes.PrimitiveState{
		Topology:  gputypes.PrimitiveTopologyTriangleList,
		FrontFace: gputypes.FrontFaceCCW,
		CullMode:  gputypes.CullModeNone,
	}

	// Define fragment stage with color targets.
	// Match the active swapchain surface format to avoid format mismatch on some backends.
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	fragmentTargets := []gputypes.ColorTargetState{
		{
			Format: surfaceFormat,
			Blend: &gputypes.BlendState{
				Color: gputypes.BlendComponent{
					SrcFactor: gputypes.BlendFactorOne,
					DstFactor: gputypes.BlendFactorZero,
					Operation: gputypes.BlendOperationAdd,
				},
				Alpha: gputypes.BlendComponent{
					SrcFactor: gputypes.BlendFactorOne,
					DstFactor: gputypes.BlendFactorZero,
					Operation: gputypes.BlendOperationAdd,
				},
			},
			WriteMask: gputypes.ColorWriteMaskAll,
		},
	}

	fragmentState := &hal.FragmentState{
		Module:     fragmentShader,
		EntryPoint: "fs_main",
		Targets:    fragmentTargets,
	}

	depthStencilState := hal.DepthStencilState{
		Format:            worldDepthTextureFormat,
		DepthWriteEnabled: true,
		DepthCompare:      gputypes.CompareFunctionLessEqual,
		StencilReadMask:   0xFFFFFFFF,
		StencilWriteMask:  0xFFFFFFFF,
	}

	// Create render pipeline descriptor
	pipelineDesc := &hal.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive:    primitiveState,
		DepthStencil: &depthStencilState,
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: fragmentState,
	}

	// Create the render pipeline
	pipeline, err := device.CreateRenderPipeline(pipelineDesc)
	if err != nil {
		textureLayout.Destroy()
		uniformLayout.Destroy()
		pipelineLayout.Destroy()
		return nil, nil, fmt.Errorf("create render pipeline: %w", err)
	}

	slog.Info("World render pipeline created")
	return pipeline, pipelineLayout, nil
}

// createWorldWhiteTexture creates a simple 1x1 white texture for fallback.
// Used when actual textures are not yet available for rendering.
func (r *Renderer) createWorldWhiteTexture(device hal.Device, queue hal.Queue) (hal.Texture, hal.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid device or queue")
	}

	// Create 1x1 RGBA texture descriptor
	textureDesc := &hal.TextureDescriptor{
		Label:         "World White Texture",
		Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}

	// Create the texture
	texture, err := device.CreateTexture(textureDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create white texture: %w", err)
	}

	// Create white pixel data (RGBA: 255,255,255,255)
	whitePixel := []byte{255, 255, 255, 255}

	// Write white pixel to texture using queue
	err = queue.WriteTexture(
		&hal.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   hal.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		whitePixel,
		&hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  4, // 1 pixel × 4 bytes
			RowsPerImage: 1,
		},
		&hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	)
	if err != nil {
		texture.Destroy()
		return nil, nil, fmt.Errorf("write white texture data: %w", err)
	}

	// Create texture view
	textureViewDesc := &hal.TextureViewDescriptor{
		Label:           "World White Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	}

	textureView, err := device.CreateTextureView(texture, textureViewDesc)
	if err != nil {
		texture.Destroy()
		return nil, nil, fmt.Errorf("create white texture view: %w", err)
	}

	slog.Debug("World white texture created")
	return texture, textureView, nil
}

func shouldDrawGoGPUOpaqueWorldFace(face WorldFace) bool {
	if face.NumIndices == 0 {
		return false
	}
	if face.Flags&(model.SurfDrawSky|model.SurfDrawTurb) != 0 {
		return false
	}
	return true
}

func (r *Renderer) createWorldTextureSampler(device hal.Device) (hal.Sampler, error) {
	return device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "World Texture Sampler",
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeRepeat,
		AddressModeW: gputypes.AddressModeRepeat,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldLightmapSampler(device hal.Device) (hal.Sampler, error) {
	return device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "World Lightmap Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldTextureBindGroup(device hal.Device, sampler hal.Sampler, view hal.TextureView) (hal.BindGroup, error) {
	if device == nil || sampler == nil || view == nil || r.textureBindGroupLayout == nil {
		return nil, fmt.Errorf("missing world texture bind group resources")
	}
	return device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "World Texture BG",
		Layout: r.textureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: sampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
		},
	})
}

func (r *Renderer) createWorldDiffuseTexture(device hal.Device, queue hal.Queue, sampler hal.Sampler, miptex *image.MipTex) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil || miptex == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	pixels, width, height, err := miptex.MipLevel(0)
	if err != nil {
		return nil, fmt.Errorf("read mip level: %w", err)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid world texture size %dx%d", width, height)
	}
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "World Diffuse Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture: %w", err)
	}
	rgba := ConvertPaletteToRGBA(pixels, r.palette)
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return nil, fmt.Errorf("write world texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "World Diffuse Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Destroy()
		return nil, fmt.Errorf("create world texture view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return nil, fmt.Errorf("create world texture bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func (r *Renderer) uploadWorldDiffuseTextures(device hal.Device, queue hal.Queue, sampler hal.Sampler, tree *bsp.Tree) map[int32]*gpuWorldTexture {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil
	}
	textures := make(map[int32]*gpuWorldTexture, textureCount)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		worldTexture, err := r.createWorldDiffuseTexture(device, queue, sampler, miptex)
		if err != nil {
			slog.Warn("failed to upload world diffuse texture", "texture", miptex.Name, "error", err)
			continue
		}
		textures[int32(i)] = worldTexture
	}
	return textures
}

func (r *Renderer) createWorldLightmapPageTexture(device hal.Device, queue hal.Queue, sampler hal.Sampler, page *WorldLightmapPage, values [64]float32) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil || page == nil {
		return nil, fmt.Errorf("invalid world lightmap upload inputs")
	}
	rgba := buildWorldLightmapPageRGBA(page, values)
	if len(rgba) == 0 {
		return nil, fmt.Errorf("empty world lightmap page")
	}
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "World Lightmap Texture",
		Size:          hal.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world lightmap texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &hal.ImageDataLayout{BytesPerRow: uint32(page.Width * 4), RowsPerImage: uint32(page.Height)}, &hal.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return nil, fmt.Errorf("write world lightmap texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "World Lightmap Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Destroy()
		return nil, fmt.Errorf("create world lightmap view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return nil, fmt.Errorf("create world lightmap bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func (r *Renderer) uploadWorldLightmapPages(device hal.Device, queue hal.Queue, sampler hal.Sampler, pages []WorldLightmapPage, values [64]float32) []*gpuWorldTexture {
	if device == nil || queue == nil || sampler == nil || len(pages) == 0 {
		return nil
	}
	out := make([]*gpuWorldTexture, len(pages))
	for i := range pages {
		pageTexture, err := r.createWorldLightmapPageTexture(device, queue, sampler, &pages[i], values)
		if err != nil {
			slog.Warn("failed to upload world lightmap page", "page", i, "error", err)
			continue
		}
		out[i] = pageTexture
	}
	return out
}

func defaultWorldLightStyleValues() [64]float32 {
	var values [64]float32
	values[0] = 1
	return values
}

func worldLightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

func compositeWorldLightmapSurfaceRGBA(rgba []byte, pageWidth int, surface WorldLightmapSurface, values [64]float32) {
	if surface.Width <= 0 || surface.Height <= 0 {
		return
	}
	styleCount := 0
	for _, style := range surface.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}
	faceSize := surface.Width * surface.Height * 3
	if len(surface.Samples) < faceSize*styleCount {
		return
	}
	for y := 0; y < surface.Height; y++ {
		for x := 0; x < surface.Width; x++ {
			sampleIndex := (y*surface.Width + x) * 3
			var rSum, gSum, bSum float32
			for styleIndex := 0; styleIndex < styleCount; styleIndex++ {
				offset := styleIndex*faceSize + sampleIndex
				scale := worldLightstyleScale(values, surface.Styles[styleIndex])
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := ((surface.Y+y)*pageWidth + (surface.X + x)) * 4
			rgba[dst] = byte(clamp01(rSum/255.0) * 255)
			rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
			rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
			rgba[dst+3] = 255
		}
	}
}

func buildWorldLightmapPageRGBA(page *WorldLightmapPage, values [64]float32) []byte {
	if page.Width <= 0 || page.Height <= 0 {
		return nil
	}
	rgba := make([]byte, page.Width*page.Height*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i] = 255
		rgba[i+1] = 255
		rgba[i+2] = 255
		rgba[i+3] = 255
	}
	for _, surface := range page.Surfaces {
		compositeWorldLightmapSurfaceRGBA(rgba, page.Width, surface, values)
	}
	page.rgba = rgba
	return rgba
}

func assignFaceLightmap(vertices []WorldVertex, rawCoords [][2]float32, face *bsp.TreeFace, tree *bsp.Tree, allocator *LightmapAllocator, pages *[]WorldLightmapPage) (*faceLightmapSurface, error) {
	if face == nil || tree == nil || allocator == nil || len(vertices) == 0 || len(rawCoords) != len(vertices) || face.LightOfs < 0 || len(tree.Lighting) == 0 {
		return nil, nil
	}

	minU, maxU := rawCoords[0][0], rawCoords[0][0]
	minV, maxV := rawCoords[0][1], rawCoords[0][1]
	for i := 1; i < len(rawCoords); i++ {
		if rawCoords[i][0] < minU {
			minU = rawCoords[i][0]
		}
		if rawCoords[i][0] > maxU {
			maxU = rawCoords[i][0]
		}
		if rawCoords[i][1] < minV {
			minV = rawCoords[i][1]
		}
		if rawCoords[i][1] > maxV {
			maxV = rawCoords[i][1]
		}
	}

	textureMinU := float32(math.Floor(float64(minU/16.0))) * 16.0
	textureMinV := float32(math.Floor(float64(minV/16.0))) * 16.0
	extentU := int(math.Ceil(float64(maxU/16.0))*16.0 - float64(textureMinU))
	extentV := int(math.Ceil(float64(maxV/16.0))*16.0 - float64(textureMinV))
	if extentU < 0 {
		extentU = 0
	}
	if extentV < 0 {
		extentV = 0
	}
	smax := extentU/16 + 1
	tmax := extentV/16 + 1
	if smax <= 0 || tmax <= 0 {
		return nil, nil
	}

	texNum, x, y, err := allocator.AllocBlock(smax, tmax)
	if err != nil {
		return nil, fmt.Errorf("alloc face lightmap: %w", err)
	}
	for len(*pages) <= texNum {
		*pages = append(*pages, WorldLightmapPage{Width: worldLightmapPageSize, Height: worldLightmapPageSize})
	}

	styleCount := 0
	for _, style := range face.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}

	sampleSize8 := smax * tmax * styleCount
	samples := expandLightmapSamples(tree.Lighting, tree.LightingRGB, int(face.LightOfs), sampleSize8)
	if samples == nil {
		return nil, nil
	}

	(*pages)[texNum].Surfaces = append((*pages)[texNum].Surfaces, WorldLightmapSurface{
		X:       x,
		Y:       y,
		Width:   smax,
		Height:  tmax,
		Styles:  face.Styles,
		Samples: samples,
	})

	for i := range vertices {
		lightS := (rawCoords[i][0]-textureMinU)/16.0 + float32(x) + 0.5
		lightT := (rawCoords[i][1]-textureMinV)/16.0 + float32(y) + 0.5
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

// Helper functions to convert Go types to byte slices
func float32ToBytes(f []float32) []byte {
	result := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(result[i*4:i*4+4], math.Float32bits(v))
	}
	return result
}

// uint32ToBytes expands packed integer data into byte form for uploads to APIs expecting byte-addressable buffers/textures.
func uint32ToBytes(u uint32) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, u)
	return result
}

// UploadWorld prepares BSP world geometry for rendering.
// This should be called once when a map is loaded.
//
// Uploads vertex and index buffers to GPU, compiles shaders,
// creates the render pipeline, and prepares for rendering.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}
	r.mu.Lock()
	r.brushModelGeometry = make(map[int]*WorldGeometry)
	r.mu.Unlock()

	slog.Info("Uploading world geometry to GPU")

	// Build geometry from BSP
	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		return fmt.Errorf("build world geometry: %w", err)
	}

	// Create render data
	renderData := &WorldRenderData{
		Geometry:      geom,
		TotalVertices: len(geom.Vertices),
		TotalIndices:  len(geom.Indices),
		TotalFaces:    len(geom.Faces),
	}

	if len(geom.Vertices) > 0 {
		boundsMin := geom.Vertices[0].Position
		boundsMax := geom.Vertices[0].Position
		for index := 1; index < len(geom.Vertices); index++ {
			position := geom.Vertices[index].Position
			if position[0] < boundsMin[0] {
				boundsMin[0] = position[0]
			}
			if position[1] < boundsMin[1] {
				boundsMin[1] = position[1]
			}
			if position[2] < boundsMin[2] {
				boundsMin[2] = position[2]
			}

			if position[0] > boundsMax[0] {
				boundsMax[0] = position[0]
			}
			if position[1] > boundsMax[1] {
				boundsMax[1] = position[1]
			}
			if position[2] > boundsMax[2] {
				boundsMax[2] = position[2]
			}
		}

		renderData.BoundsMin = boundsMin
		renderData.BoundsMax = boundsMax
	}

	// Get HAL device and queue from gogpu renderer
	device := r.getHALDevice()
	queue := r.getHALQueue()
	if device == nil || queue == nil {
		slog.Warn("HAL device or queue not available, skipping GPU upload")
		// Store geometry anyway for later use
		r.mu.Lock()
		r.worldData = renderData
		r.mu.Unlock()
		return nil
	}

	// Upload vertex buffer
	vertexBuffer, err := r.createWorldVertexBuffer(device, queue, geom.Vertices)
	if err != nil {
		return fmt.Errorf("upload vertex buffer: %w", err)
	}

	// Upload index buffer
	indexBuffer, indexCount, err := r.createWorldIndexBuffer(device, queue, geom.Indices)
	if err != nil {
		return fmt.Errorf("upload index buffer: %w", err)
	}

	// Create shader modules (WGSL is compiled by HAL internally)
	vertexShader, err := createWorldShaderModule(device, worldVertexShaderWGSL, "World Vertex Shader")
	if err != nil {
		slog.Warn("Failed to create vertex shader", "error", err)
		vertexShader = nil
	}

	fragmentShader, err := createWorldShaderModule(device, worldFragmentShaderWGSL, "World Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create fragment shader", "error", err)
		fragmentShader = nil
	}

	// Create render pipeline (may fail if gogpu API not fully exposed)
	var pipeline hal.RenderPipeline
	var pipelineLayout hal.PipelineLayout
	if vertexShader != nil && fragmentShader != nil {
		var err2 error
		pipeline, pipelineLayout, err2 = r.createWorldPipeline(device, vertexShader, fragmentShader)
		if err2 != nil {
			slog.Warn("Failed to create render pipeline", "error", err2)
		}
	}

	// Create uniform buffer for VP matrix
	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "World Uniforms",
		Size:             worldUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create uniform buffer: %w", err)
	}

	// Create bind group for world uniform buffer.
	uniformLayout := r.uniformBindGroupLayout
	if uniformLayout != nil {
		uniformBindGroup, bindErr := device.CreateBindGroup(&hal.BindGroupDescriptor{
			Label:  "World Uniform BG",
			Layout: uniformLayout,
			Entries: []gputypes.BindGroupEntry{
				{
					Binding: 0,
					Resource: gputypes.BufferBinding{
						Buffer: uniformBuffer.NativeHandle(),
						Offset: 0,
						Size:   worldUniformBufferSize,
					},
				},
			},
		})
		if bindErr != nil {
			slog.Warn("Failed to create world uniform bind group", "error", bindErr)
		} else {
			r.uniformBindGroup = uniformBindGroup
			r.worldBindGroup = uniformBindGroup
		}
	}

	// Create white texture for fallback
	whiteTexture, whiteTextureView, err := r.createWorldWhiteTexture(device, queue)
	if err != nil {
		slog.Warn("Failed to create white texture", "error", err)
		// Don't fail completely, will use fallback rendering
	}
	var worldTextureSampler hal.Sampler
	var whiteTextureBindGroup hal.BindGroup
	if r.textureBindGroupLayout != nil {
		worldTextureSampler, err = r.createWorldTextureSampler(device)
		if err != nil {
			slog.Warn("Failed to create world texture sampler", "error", err)
		} else if whiteTextureView != nil {
			whiteTextureBindGroup, err = r.createWorldTextureBindGroup(device, worldTextureSampler, whiteTextureView)
			if err != nil {
				slog.Warn("Failed to create white world texture bind group", "error", err)
			}
		}
	}
	worldTextures := r.uploadWorldDiffuseTextures(device, queue, worldTextureSampler, tree)
	lightstyleValues := defaultWorldLightStyleValues()
	var worldLightmapSampler hal.Sampler
	var whiteLightmapBindGroup hal.BindGroup
	if r.textureBindGroupLayout != nil {
		worldLightmapSampler, err = r.createWorldLightmapSampler(device)
		if err != nil {
			slog.Warn("Failed to create world lightmap sampler", "error", err)
		} else if whiteTextureView != nil {
			whiteLightmapBindGroup, err = r.createWorldTextureBindGroup(device, worldLightmapSampler, whiteTextureView)
			if err != nil {
				slog.Warn("Failed to create white world lightmap bind group", "error", err)
			}
		}
	}
	worldLightmapPages := r.uploadWorldLightmapPages(device, queue, worldLightmapSampler, geom.Lightmaps, lightstyleValues)

	// Create offscreen render target for world rendering
	if err := r.createWorldRenderTarget(); err != nil {
		slog.Warn("Failed to create world render target", "error", err)
		// Don't fail completely, will use direct rendering fallback
	}

	width, height := r.Size()
	var depthTexture hal.Texture
	var depthTextureView hal.TextureView
	if width > 0 && height > 0 {
		depthTexture, depthTextureView, err = r.createWorldDepthTexture(device, width, height)
		if err != nil {
			slog.Warn("Failed to create world depth texture", "error", err)
		}
	}

	// Store GPU resources in renderer
	r.mu.Lock()
	r.worldData = renderData
	r.worldVertexBuffer = vertexBuffer
	r.worldIndexBuffer = indexBuffer
	r.worldIndexCount = indexCount
	r.worldPipeline = pipeline
	r.worldPipelineLayout = pipelineLayout
	r.worldShader = vertexShader
	r.uniformBuffer = uniformBuffer
	r.whiteTexture = whiteTexture
	r.whiteTextureView = whiteTextureView
	r.worldTextureSampler = worldTextureSampler
	r.worldTextures = worldTextures
	r.whiteTextureBindGroup = whiteTextureBindGroup
	r.worldLightmapSampler = worldLightmapSampler
	r.worldLightmapPages = worldLightmapPages
	r.whiteLightmapBindGroup = whiteLightmapBindGroup
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthTextureView
	renderData.VertexBufferUploaded = vertexBuffer != nil
	renderData.IndexBufferUploaded = indexBuffer != nil
	renderData.HasDiffuseTextures = len(worldTextures) > 0
	renderData.HasLightmapTextures = len(worldLightmapPages) > 0
	renderData.HasDepthBuffer = depthTextureView != nil
	r.mu.Unlock()

	slog.Info("World geometry uploaded to GPU",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"triangles", renderData.TotalIndices/3,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
		"vertexBufferSize", uint64(len(geom.Vertices))*44,
		"indexBufferSize", uint64(len(geom.Indices))*4)

	return nil
}

// renderWorldInternal implements world rendering.
// This records render commands to draw the world geometry with the configured pipeline,
// shaders, textures, and matrices.
func (dc *DrawContext) renderWorldInternal(state *RenderFrameState) {
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil {
		slog.Info("renderWorldInternal: no world data")
		return
	}

	slog.Info("renderWorldInternal: starting world render")

	dc.renderer.mu.RLock()
	defer dc.renderer.mu.RUnlock()

	// Check if GPU resources are ready
	if dc.renderer.worldVertexBuffer == nil || dc.renderer.worldIndexBuffer == nil {
		if worldData.TotalFaces > 0 {
			slog.Info("renderWorldInternal: World GPU buffers not ready",
				"faces", worldData.TotalFaces,
				"triangles", worldData.TotalIndices/3)
		}
		return
	}

	if dc.renderer.worldPipeline == nil {
		slog.Info("renderWorldInternal: World pipeline not ready")
		return
	}

	// Get HAL device and queue
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		slog.Info("renderWorldInternal: HAL device or queue not available for world rendering")
		return
	}

	// Create command encoder
	slog.Info("renderWorldInternal: creating command encoder")
	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "World Render Command Encoder",
	})
	if err != nil {
		slog.Error("renderWorldInternal: Failed to create command encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("world"); err != nil {
		slog.Error("renderWorldInternal: Failed to begin command encoding", "error", err)
		return
	}
	slog.Info("renderWorldInternal: command encoder started")

	// Use the current surface view for zero-copy rendering (per gogpu design)
	// This allows HAL to render directly to the same surface that gogpu will composite onto
	slog.Info("renderWorldInternal: getting surface view from gogpu context")
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		slog.Info("renderWorldInternal: Render target view not available, skipping world rendering")
		return
	}
	slog.Info("renderWorldInternal: render target view acquired", "view_type", fmt.Sprintf("%T", textureView), "queue_type", fmt.Sprintf("%T", queue))

	// Create render pass descriptor with color and depth attachments.
	// Use LoadOpClear to handle the clear ourselves since we skip gogpu's Clear().
	clearColor := gogpuWorldClearColor(state.ClearColor)
	slog.Info("renderWorldInternal: creating render pass descriptor")
	renderPassDesc := &hal.RenderPassDescriptor{
		Label: "World Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{
			{
				View:       textureView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearColor,
			},
		},
		DepthStencilAttachment: worldDepthAttachmentForView(dc.renderer.worldDepthTextureView),
	}

	// Begin render pass
	slog.Info("renderWorldInternal: beginning render pass")
	renderPass := encoder.BeginRenderPass(renderPassDesc)
	slog.Info("renderWorldInternal: render pass created", "pass", fmt.Sprintf("%T", renderPass))

	// Set pipeline
	slog.Info("renderWorldInternal: setting pipeline", "pipeline", fmt.Sprintf("%T", dc.renderer.worldPipeline))
	renderPass.SetPipeline(dc.renderer.worldPipeline)

	// Explicit viewport/scissor to avoid backend defaults that can yield zero-area rasterization.
	w, h := dc.renderer.Size()
	if w > 0 && h > 0 {
		slog.Info("renderWorldInternal: setting viewport", "x", 0, "y", 0, "w", w, "h", h)
		renderPass.SetViewport(0, 0, float32(w), float32(h), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	} else {
		slog.Warn("renderWorldInternal: invalid viewport size", "w", w, "h", h)
	}

	// Update uniform buffer with VP matrix
	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	camera := dc.renderer.cameraState
	uniformBytes := worldSceneUniformBytes(vpMatrix, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}, state.FogColor, state.FogDensity)
	slog.Info("renderWorldInternal: VP matrix",
		"m00", vpMatrix[0], "m11", vpMatrix[5], "m22", vpMatrix[10], "m33", vpMatrix[15])
	slog.Info("renderWorldInternal: writing uniform buffer", "bytes_len", len(uniformBytes))
	err = queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to update uniform buffer", "error", err)
		renderPass.End()
		return
	}

	// Set vertex buffer
	slog.Info("renderWorldInternal: setting vertex buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldVertexBuffer))
	renderPass.SetVertexBuffer(0, dc.renderer.worldVertexBuffer, 0)

	// Set index buffer (uint32 format for indices)
	slog.Info("renderWorldInternal: setting index buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldIndexBuffer), "count", dc.renderer.worldIndexCount)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	// Set uniform bind group.
	if dc.renderer.uniformBindGroup != nil {
		slog.Info("renderWorldInternal: setting bind group", "group", fmt.Sprintf("%T", dc.renderer.uniformBindGroup))
		renderPass.SetBindGroup(0, dc.renderer.uniformBindGroup, nil)
	} else {
		slog.Warn("renderWorldInternal: NO uniform bind group set")
	}

	if dc.renderer.whiteTextureBindGroup == nil || dc.renderer.whiteLightmapBindGroup == nil {
		slog.Warn("renderWorldInternal: no world texture/lightmap bind group available")
		renderPass.End()
		return
	}

	drawnIndices := uint32(0)
	for _, face := range worldData.Geometry.Faces {
		if !shouldDrawGoGPUOpaqueWorldFace(face) {
			continue
		}
		textureBindGroup := dc.renderer.whiteTextureBindGroup
		if worldTexture := dc.renderer.worldTextures[face.TextureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		lightmapBindGroup := dc.renderer.whiteLightmapBindGroup
		if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(dc.renderer.worldLightmapPages) {
			if lightmapPage := dc.renderer.worldLightmapPages[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
				lightmapBindGroup = lightmapPage.bindGroup
			}
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
		drawnIndices += face.NumIndices
	}
	if drawnIndices > 0 {
		slog.Info("World rendered",
			"indices", drawnIndices,
			"triangles", drawnIndices/3,
			"vertices", worldData.TotalVertices)
	} else {
		slog.Info("renderWorldInternal: No opaque world faces selected for textured draw")
	}

	// End render pass
	slog.Info("renderWorldInternal: ending render pass")
	renderPass.End()

	// Finish encoding and get command buffer
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Error("renderWorldInternal: Failed to finish command encoding", "error", err)
		return
	}

	// Submit to queue
	slog.Info("renderWorldInternal: submitting to queue")
	err = queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to submit render commands", "error", err)
		return
	}

	if os.Getenv("IRONWAIL_DEBUG_HAL_WAIT_IDLE") == "1" {
		slog.Warn("renderWorldInternal: IRONWAIL_DEBUG_HAL_WAIT_IDLE=1, waiting for device idle after submit")
		if waitErr := device.WaitIdle(); waitErr != nil {
			slog.Error("renderWorldInternal: device WaitIdle failed", "error", waitErr)
		} else {
			slog.Info("renderWorldInternal: device WaitIdle completed")
		}
	}

	slog.Info("World render commands submitted successfully")
}

// matrixToBytes converts a types.Mat4 to bytes (column-major, little-endian).
func matrixToBytes(m types.Mat4) []byte {
	b := types.Mat4ToBytes(m)
	return b[:]
}

func worldSceneUniformBytes(vp types.Mat4, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, worldUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	putFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[80:92], fogColor[:])
	return data
}

func gogpuWorldClearColor(clear [4]float32) gputypes.Color {
	if os.Getenv("IRONWAIL_DEBUG_WORLD_CLEAR_GREEN") == "1" {
		return gputypes.Color{R: 0.0, G: 1.0, B: 0.0, A: 1.0}
	}
	return gputypes.Color{
		R: float64(clear[0]),
		G: float64(clear[1]),
		B: float64(clear[2]),
		A: float64(clear[3]),
	}
}

func (dc *DrawContext) clearGoGPUSharedDepthStencil() {
	if dc == nil || dc.renderer == nil {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	textureView := dc.currentHALRenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	dc.renderer.mu.Lock()
	depthView := dc.renderer.worldDepthTextureView
	dc.renderer.mu.Unlock()
	attachment := gogpuSharedDepthStencilClearAttachmentForView(depthView)
	if attachment == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "GoGPU Shared Depth Clear Encoder"})
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to create encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("gogpu-shared-depth-clear"); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to begin encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "GoGPU Shared Depth Clear Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: attachment,
	})
	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to finish encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to submit clear pass", "error", err)
	}
}

// TransformVertex applies model-view-projection transformation to a vertex.
// This is a helper for software rendering fallback.
func TransformVertex(pos [3]float32, mvp types.Mat4) types.Vec4 {
	v := types.Vec4{X: pos[0], Y: pos[1], Z: pos[2], W: 1.0}
	return types.Mat4MulVec4(mvp, v)
}

// createWorldDepthTexture allocates a depth attachment used by multi-pass world rendering so later passes can depth-test against the opaque world.
func (r *Renderer) createWorldDepthTexture(device hal.Device, width, height int) (hal.Texture, hal.TextureView, error) {
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "World Depth Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        worldDepthTextureFormat,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create depth texture: %w", err)
	}

	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "World Depth Texture View",
		Format:          worldDepthTextureFormat,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectDepthOnly,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Destroy()
		return nil, nil, fmt.Errorf("create depth texture view: %w", err)
	}

	return texture, view, nil
}

// worldDepthAttachmentForView picks the correct depth target for the current view configuration and pass sequence.
func worldDepthAttachmentForView(view hal.TextureView) *hal.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &hal.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   true,
	}
}

func gogpuSharedDepthStencilClearAttachmentForView(view hal.TextureView) *hal.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &hal.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false,
	}
}

// ClearWorld releases world geometry resources.
// Called when switching maps or shutting down.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.worldData != nil {
		// Release GPU buffers
		if r.worldVertexBuffer != nil {
			r.worldVertexBuffer.Destroy()
		}
		if r.worldIndexBuffer != nil {
			r.worldIndexBuffer.Destroy()
		}
		if r.uniformBuffer != nil {
			r.uniformBuffer.Destroy()
		}
		if r.uniformBindGroup != nil {
			r.uniformBindGroup.Destroy()
		}
		if r.uniformBindGroupLayout != nil {
			r.uniformBindGroupLayout.Destroy()
		}
		if r.textureBindGroupLayout != nil {
			r.textureBindGroupLayout.Destroy()
		}
		if r.whiteTextureBindGroup != nil {
			r.whiteTextureBindGroup.Destroy()
		}
		if r.whiteLightmapBindGroup != nil {
			r.whiteLightmapBindGroup.Destroy()
		}
		if r.worldTextureSampler != nil {
			r.worldTextureSampler.Destroy()
		}
		if r.worldLightmapSampler != nil {
			r.worldLightmapSampler.Destroy()
		}
		for textureIndex, worldTexture := range r.worldTextures {
			if worldTexture == nil {
				delete(r.worldTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Destroy()
			}
			if worldTexture.view != nil {
				worldTexture.view.Destroy()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Destroy()
			}
			delete(r.worldTextures, textureIndex)
		}
		for index, worldLightmap := range r.worldLightmapPages {
			if worldLightmap == nil {
				continue
			}
			if worldLightmap.bindGroup != nil {
				worldLightmap.bindGroup.Destroy()
			}
			if worldLightmap.view != nil {
				worldLightmap.view.Destroy()
			}
			if worldLightmap.texture != nil {
				worldLightmap.texture.Destroy()
			}
			r.worldLightmapPages[index] = nil
		}
		if r.whiteTexture != nil {
			r.whiteTexture.Destroy()
		}
		if r.worldDepthTexture != nil {
			r.worldDepthTexture.Destroy()
		}

		r.worldData = nil
		r.worldVertexBuffer = nil
		r.worldIndexBuffer = nil
		r.worldPipeline = nil
		r.worldShader = nil
		r.uniformBuffer = nil
		r.uniformBindGroup = nil
		r.uniformBindGroupLayout = nil
		r.textureBindGroupLayout = nil
		r.worldTextureSampler = nil
		r.worldTextures = nil
		r.whiteTextureBindGroup = nil
		r.worldLightmapSampler = nil
		r.worldLightmapPages = nil
		r.whiteLightmapBindGroup = nil
		r.worldBindGroup = nil
		r.whiteTexture = nil
		r.whiteTextureView = nil
		r.worldDepthTexture = nil
		r.worldDepthTextureView = nil
		r.brushModelGeometry = make(map[int]*WorldGeometry)

		slog.Debug("World geometry cleared")
	}
}

// GetWorldData returns the current world render data (for debugging).
func (r *Renderer) GetWorldData() *WorldRenderData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData
}

// GetWorldBounds returns uploaded world geometry bounds when available.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}

	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}
