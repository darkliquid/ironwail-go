//go:build gogpu
// +build gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
)

// Uniforms structure for world rendering, must match WGSL Uniforms struct
type WorldUniforms struct {
	ViewProjection [16]float32 // mat4x4
}

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

// WorldRenderData holds GPU-side resources for world rendering.
// This is what gets uploaded to the GPU and used during rendering.
type WorldRenderData struct {
	// Geometry holds preprocessed vertex/index data
	Geometry *WorldGeometry

	// TODO: GPU buffer handles (when gogpu exposes buffer API)
	// VertexBuffer  *gogpu.Buffer
	// IndexBuffer   *gogpu.Buffer
	
	// TODO: Texture arrays/atlases
	// DiffuseTextures  []*gogpu.Texture
	// LightmapTextures []*gogpu.Texture

	// Stats for debugging
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

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
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}

	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}

	worldModel := tree.Models[0]

	geom := &WorldGeometry{
		Vertices: make([]WorldVertex, 0, 4096),
		Indices:  make([]uint32, 0, 16384),
		Faces:    make([]WorldFace, 0, 256),
		Tree:     tree,
	}

	// Process all faces in the world model
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
			TextureIndex:  face.Texinfo, // TODO: Map to actual texture
			LightmapIndex: -1,           // TODO: Process lightmaps
			Flags:         0,             // TODO: Extract flags from texinfo
		}

		// Extract vertices for this face
		faceVerts, err := extractFaceVertices(tree, face)
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

		// Triangulate face using fan triangulation
		// Face with N vertices becomes (N-2) triangles
		baseVertIdx := uint32(len(geom.Vertices))

		// Add all vertices for this face
		geom.Vertices = append(geom.Vertices, faceVerts...)

		// Generate triangle indices (fan triangulation around vertex 0)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,       // Vertex 0 (fan center)
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

	return geom, nil
}

// extractFaceVertices extracts all vertices for a BSP face.
// It follows the edge/surfedge indirection to get vertex positions,
// then computes texture/lightmap coords and normals.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace) ([]WorldVertex, error) {
	numEdges := int(face.NumEdges)
	if numEdges < 3 {
		return nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, numEdges)

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
	}

	// Iterate through edges to extract vertex positions
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}

		surfEdge := tree.Surfedges[surfEdgeIdx]

		// Surfedge is signed: positive = use edge V[0], negative = use edge V[1]
		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return nil, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return nil, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			return nil, fmt.Errorf("vertex index %d out of range", vertIdx)
		}

		position := tree.Vertexes[vertIdx].Point

		// TODO: Compute texture coordinates from TexInfo
		// For now, use placeholder values
		texCoord := [2]float32{0.0, 0.0}
		lightmapCoord := [2]float32{0.0, 0.0}

		// TODO: Proper texture coordinate calculation
		// The TexInfo contains texture axis vectors that need to be
		// applied to the vertex position to get UV coordinates.
		// Formula: u = dot(position, texInfo.Vecs[0]) + texInfo.Vecs[0][3]
		//          v = dot(position, texInfo.Vecs[1]) + texInfo.Vecs[1][3]

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: lightmapCoord,
			Normal:        normal,
		})
	}

	return vertices, nil
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
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    
    let worldPos = vec4<f32>(input.position, 1.0);
    output.clipPosition = uniforms.viewProjection * worldPos;
    
    output.texCoord = input.texCoord;
    output.lightmapCoord = input.lightmapCoord;
    output.worldPos = input.position;
    output.normal = input.normal;
    
    return output;
}
`

// worldFragmentShaderWGSL is the WGSL source for world fragment shader
const worldFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
}

@group(0) @binding(1)
var diffuseSampler: sampler;

@group(0) @binding(2)
var diffuseTexture: texture_2d<f32>;

@group(0) @binding(3)
var lightmapSampler: sampler;

@group(0) @binding(4)
var lightmapTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    var diffuse = textureSample(diffuseTexture, diffuseSampler, input.texCoord);
    var lightmap = textureSample(lightmapTexture, lightmapSampler, input.lightmapCoord);
    
    var lit = diffuse.rgb * lightmap.rgb * 2.0;
    
    return vec4<f32>(lit, diffuse.a);
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
		Label: "World Vertices",
		Size:  vertexSize,
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
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
		Label: "World Indices",
		Size:  indexSize,
		Usage: gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
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

	// Create pipeline layout with empty bind group layouts for now
	pipelineLayoutDesc := &hal.PipelineLayoutDescriptor{
		Label:            "World Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{}, // Will be extended for textures/uniforms later
	}

	pipelineLayout, err := device.CreatePipelineLayout(pipelineLayoutDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	// Define primitive state (counter-clockwise winding, back-face culling)
	primitiveState := gputypes.PrimitiveState{
		Topology:  gputypes.PrimitiveTopologyTriangleList,
		FrontFace: gputypes.FrontFaceCCW,
		CullMode:  gputypes.CullModeBack,
	}

	// Define depth-stencil state (depth testing enabled)
	depthStencilState := &hal.DepthStencilState{
		Format:            gputypes.TextureFormatDepth24Plus, // Use depth24
		DepthWriteEnabled: true,
		DepthCompare:      gputypes.CompareFunctionLess, // Closer fragments pass
	}

	// Define fragment stage with color targets
	fragmentTargets := []gputypes.ColorTargetState{
		{
			Format: gputypes.TextureFormatBGRA8Unorm, // Standard backbuffer format
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

	// Create render pipeline descriptor
	pipelineDesc := &hal.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive:   primitiveState,
		DepthStencil: depthStencilState,
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
		Label:     "World White Texture",
		Size:      hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:  1,
		Dimension:    gputypes.TextureDimension2D,
		Format:       gputypes.TextureFormatRGBA8Unorm,
		Usage:        gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
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
		BaseArrayLayer: 0,
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

// Helper functions to convert Go types to byte slices
func float32ToBytes(f []float32) []byte {
	result := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(result[i*4:i*4+4], math.Float32bits(v))
	}
	return result
}

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
		Label: "World Uniforms",
		Size:  64, // sizeof(mat4x4) = 16 floats * 4 bytes
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create uniform buffer: %w", err)
	}

	// Create white texture for fallback
	whiteTexture, whiteTextureView, err := r.createWorldWhiteTexture(device, queue)
	if err != nil {
		slog.Warn("Failed to create white texture", "error", err)
		// Don't fail completely, will use fallback rendering
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
	r.mu.Unlock()

	slog.Info("World geometry uploaded to GPU",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"triangles", renderData.TotalIndices/3,
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
		return
	}

	dc.renderer.mu.RLock()
	defer dc.renderer.mu.RUnlock()

	// Check if GPU resources are ready
	if dc.renderer.worldVertexBuffer == nil || dc.renderer.worldIndexBuffer == nil {
		if worldData.TotalFaces > 0 {
			slog.Debug("World GPU buffers not ready",
				"faces", worldData.TotalFaces,
				"triangles", worldData.TotalIndices/3)
		}
		return
	}

	if dc.renderer.worldPipeline == nil {
		slog.Debug("World pipeline not ready")
		return
	}

	// Get HAL device and queue
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		slog.Warn("HAL device or queue not available for world rendering")
		return
	}

	// Create command encoder
	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{
		Label: "World Render Command Encoder",
	})
	if err != nil {
		slog.Error("Failed to create command encoder", "error", err)
		return
	}

	// Get swapchain texture view from gogpu context (if available via ctx field)
	// For now, we'll use the context's surface view if available
	surfaceView := dc.ctx.SurfaceView()
	if surfaceView == nil {
		slog.Debug("Surface view not available for world rendering")
		return
	}

	// Cast surface view to HAL TextureView
	textureView, ok := surfaceView.(hal.TextureView)
	if !ok {
		slog.Debug("Surface view could not be cast to HAL TextureView")
		return
	}

	// Create render pass descriptor with color and depth attachments
	renderPassDesc := &hal.RenderPassDescriptor{
		Label: "World Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{
			{
				View:      textureView,
				LoadOp:    gputypes.LoadOpClear,
				StoreOp:   gputypes.StoreOpStore,
				ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 1}, // Black
			},
		},
		// Depth attachment if depth texture available
		DepthStencilAttachment: nil, // TODO: Add depth texture support
	}

	// If we have a depth texture, enable it
	if dc.renderer.worldIndexCount > 0 {
		// Could set depth attachment here when implemented
		// renderPassDesc.DepthStencilAttachment = &depthAttachment
	}

	// Begin render pass
	renderPass := encoder.BeginRenderPass(renderPassDesc)

	// Set pipeline
	renderPass.SetPipeline(dc.renderer.worldPipeline)

	// Update uniform buffer with VP matrix
	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	vpBytes := matrixToBytes(vpMatrix)
	err = queue.WriteBuffer(dc.renderer.uniformBuffer, 0, vpBytes)
	if err != nil {
		slog.Error("Failed to update uniform buffer", "error", err)
		renderPass.End()
		return
	}

	// Set vertex buffer
	renderPass.SetVertexBuffer(0, dc.renderer.worldVertexBuffer, 0)

	// Set index buffer (uint16 format for indices)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	// Set bind group if available (for uniforms, textures, samplers)
	if dc.renderer.worldBindGroup != nil {
		renderPass.SetBindGroup(0, dc.renderer.worldBindGroup, nil)
	}

	// Draw all indices
	indexCount := dc.renderer.worldIndexCount
	if indexCount > 0 {
		// DrawIndexed(indexCount, instanceCount, firstIndex, baseVertex, firstInstance)
		renderPass.DrawIndexed(indexCount, 1, 0, 0, 0)

		slog.Debug("World rendered",
			"indices", indexCount,
			"triangles", indexCount/3,
			"vertices", worldData.TotalVertices)
	} else {
		slog.Warn("No world indices to render")
	}

	// End render pass
	renderPass.End()

	// Finish encoding and get command buffer
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Error("Failed to finish command encoding", "error", err)
		return
	}

	// Submit to queue
	err = queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0)
	if err != nil {
		slog.Error("Failed to submit render commands", "error", err)
		return
	}

	slog.Debug("World render commands submitted")
}

// matrixToBytes converts a gmath.Mat4 to bytes (column-major, little-endian).
func matrixToBytes(m gmath.Mat4) []byte {
	result := make([]byte, 64) // 16 floats * 4 bytes
	for i, v := range m {
		binary.LittleEndian.PutUint32(result[i*4:i*4+4], math.Float32bits(v))
	}
	return result
}

// TransformVertex applies model-view-projection transformation to a vertex.
// This is a helper for software rendering fallback.
func TransformVertex(pos [3]float32, mvp gmath.Mat4) gmath.Vec4 {
	// Convert position to Vec4 (w=1 for point)
	v := gmath.Vec4{X: pos[0], Y: pos[1], Z: pos[2], W: 1.0}

	// Transform by MVP matrix
	// result = MVP * vertex
	result := gmath.Vec4{
		X: mvp[0]*v.X + mvp[4]*v.Y + mvp[8]*v.Z + mvp[12]*v.W,
		Y: mvp[1]*v.X + mvp[5]*v.Y + mvp[9]*v.Z + mvp[13]*v.W,
		Z: mvp[2]*v.X + mvp[6]*v.Y + mvp[10]*v.Z + mvp[14]*v.W,
		W: mvp[3]*v.X + mvp[7]*v.Y + mvp[11]*v.Z + mvp[15]*v.W,
	}

	return result
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
		if r.whiteTexture != nil {
			r.whiteTexture.Destroy()
		}
		
		r.worldData = nil
		r.worldVertexBuffer = nil
		r.worldIndexBuffer = nil
		r.worldPipeline = nil
		r.worldShader = nil
		r.uniformBuffer = nil
		r.whiteTexture = nil
		r.whiteTextureView = nil
		
		slog.Debug("World geometry cleared")
	}
}

// GetWorldData returns the current world render data (for debugging).
func (r *Renderer) GetWorldData() *WorldRenderData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData
}
