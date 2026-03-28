//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"fmt"
	stdimage "image"
	"image/png"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// 2D rendering shaders
const (
	vertexShader2D = `#version 410 core
layout(location = 0) in vec2 aPosition;
layout(location = 1) in vec2 aTexCoord;
out vec2 vTexCoord;
uniform vec2 uCanvasScale;
uniform vec2 uCanvasOffset;

void main() {
	// Canvas transform: map canvas-space coords to NDC via scale + offset.
	gl_Position = vec4(
		aPosition.x * uCanvasScale.x + uCanvasOffset.x,
		aPosition.y * uCanvasScale.y + uCanvasOffset.y,
		0.0, 1.0);
	vTexCoord = aTexCoord;
}`

	fragmentShader2D = `#version 410 core
in vec2 vTexCoord;
out vec4 fragColor;
uniform sampler2D uTexture;
uniform vec4 uColor;

void main() {
	fragColor = texture(uTexture, vTexCoord) * uColor;
}`

	vertexShaderSolid = `#version 410 core
layout(location = 0) in vec2 aPosition;
uniform vec2 uCanvasScale;
uniform vec2 uCanvasOffset;
uniform vec4 uColor;
out vec4 vColor;

void main() {
	gl_Position = vec4(
		aPosition.x * uCanvasScale.x + uCanvasOffset.x,
		aPosition.y * uCanvasScale.y + uCanvasOffset.y,
		0.0, 1.0);
	vColor = uColor;
}`

	fragmentShaderSolid = `#version 410 core
in vec4 vColor;
out vec4 fragColor;

void main() {
	fragColor = vColor;
}`
)

type quadVertex struct {
	x, y float32
	u, v float32
}

type glDrawContext struct {
	window   *glfw.Window
	gamma    float32
	renderer *Renderer
	viewport struct {
		width  int
		height int
	}
	// 2D rendering state
	shader2D      uint32
	shaderSolid   uint32
	vao2D         uint32
	vbo2D         uint32
	initialized2D bool

	// Canvas coordinate system state.
	canvas       CanvasState
	canvasParams CanvasTransformParams
}

type glCacheKey struct {
	pic *image.QPic
}

type glCachedTexture struct {
	id     uint32
	width  int
	height int
}

type glTextureWithUV struct {
	texture uint32
	u0, v0  float32
	u1, v1  float32
}

// init performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func init() {
	// OpenGL must run on main OS thread
	runtime.LockOSThread()
}

// Clear resets particle state between level loads or hard resets so stale effects do not leak into new scenes.
func (dc *glDrawContext) Clear(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)
}

// DrawTriangle is a debug helper that submits immediate triangle vertices so renderer experiments can visualize geometry without touching BSP or model pipelines.
func (dc *glDrawContext) DrawTriangle(r, g, b, a float32) {
	// Debug helper for RenderContext tests: clear to a solid color.
	// Canonical OpenGL rendering runs through world/entity draw paths.
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT)
}

// SurfaceView returns the backend view over visible world surfaces, which tools and tests use to inspect BSP marking and pass assignment decisions.
func (dc *glDrawContext) SurfaceView() interface{} {
	// In a full implementation, this would return an OpenGL texture view
	return nil
}

// Gamma exposes the current gamma configuration so post-processing and UI code can stay in sync with renderer brightness calibration.
func (dc *glDrawContext) Gamma() float32 {
	return dc.gamma
}

// SetCanvas switches the active 2D drawing canvas. Coordinates in
// subsequent draw calls are interpreted in the canvas's logical space.
func (dc *glDrawContext) SetCanvas(ct CanvasType) {
	params := dc.canvasParams
	if params.GUIWidth <= 0 {
		params.GUIWidth = float32(dc.viewport.width)
	}
	if params.GUIHeight <= 0 {
		params.GUIHeight = float32(dc.viewport.height)
	}
	if params.GLWidth <= 0 {
		params.GLWidth = float32(dc.viewport.width)
	}
	if params.GLHeight <= 0 {
		params.GLHeight = float32(dc.viewport.height)
	}
	if params.ConWidth <= 0 {
		params.ConWidth = params.GUIWidth
	}
	if params.ConHeight <= 0 {
		params.ConHeight = params.GUIHeight
	}
	SetCanvas(&dc.canvas, ct, params)
}

// Canvas returns the current canvas state.
func (dc *glDrawContext) Canvas() CanvasState {
	return dc.canvas
}

// SetCanvasParams updates the per-frame canvas transform parameters.
// Call this at the start of each frame before any SetCanvas calls.
func (dc *glDrawContext) SetCanvasParams(p CanvasTransformParams) {
	dc.canvasParams = p
	// Force re-evaluation on next SetCanvas call.
	dc.canvas.Type = CanvasNone
}

// 2D Drawing API implementation

// init2DRenderer lazily initializes the 2D rendering pipeline: compiles texture-mapped
// and solid-color shader programs, creates a shared VAO/VBO for quad rendering. The 2D
// pipeline converts pixel coordinates to normalized clip space [-1,1] in the vertex
// shader, matching Quake's screen-space drawing API for HUD elements and menus.
func (dc *glDrawContext) init2DRenderer() error {
	if dc.initialized2D {
		return nil
	}

	// Compile 2D texture shader
	vs, err := compileShader(vertexShader2D, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("failed to compile 2D vertex shader: %w", err)
	}
	fs, err := compileShader(fragmentShader2D, gl.FRAGMENT_SHADER)
	if err != nil {
		return fmt.Errorf("failed to compile 2D fragment shader: %w", err)
	}

	dc.shader2D = createProgram(vs, fs)

	// Compile solid color shader
	vs2, err := compileShader(vertexShaderSolid, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("failed to compile solid vertex shader: %w", err)
	}
	fs2, err := compileShader(fragmentShaderSolid, gl.FRAGMENT_SHADER)
	if err != nil {
		return fmt.Errorf("failed to compile solid fragment shader: %w", err)
	}

	dc.shaderSolid = createProgram(vs2, fs2)

	// Create VAO and VBO for 2D quads
	gl.GenVertexArrays(1, &dc.vao2D)
	gl.GenBuffers(1, &dc.vbo2D)

	gl.BindVertexArray(dc.vao2D)
	gl.BindBuffer(gl.ARRAY_BUFFER, dc.vbo2D)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 2, gl.FLOAT, false, 16, uintptr(0))
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, 16, 8)

	gl.BindVertexArray(0)

	dc.initialized2D = true
	return nil
}

// compileShader compiles one GLSL stage and reports detailed driver errors, the first half of creating programmable pipeline state in modern OpenGL.
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	cstr, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, cstr, nil)
	gl.CompileShader(shader)
	free()

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("shader compilation failed: %s", log)
	}

	return shader, nil
}

// createProgram links compiled shader stages into a GPU program object so later draw passes can bind one handle and execute fixed vertex/fragment logic.
func createProgram(vertexShader, fragmentShader uint32) uint32 {
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))
		panic(fmt.Sprintf("shader link failed: %s", log))
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program
}

// uploadQPicTexture converts Quake palette-indexed UI images into GPU textures, including nearest filtering choices that preserve classic pixel-art readability.
func (dc *glDrawContext) uploadQPicTexture(pic *image.QPic, rgba []byte) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)

	var data unsafe.Pointer
	if len(rgba) > 0 {
		data = unsafe.Pointer(&rgba[0])
	}

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(pic.Width), int32(pic.Height), 0, gl.RGBA, gl.UNSIGNED_BYTE, data)

	// Set texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	return tex
}

// DrawPic renders a QPic image at the specified screen-space position.
func (dc *glDrawContext) DrawPic(x, y int, pic *image.QPic) {
	dc.DrawPicAlpha(x, y, pic, 1)
}

// DrawPicAlpha renders a QPic image at the specified screen-space position with explicit alpha.
func (dc *glDrawContext) DrawPicAlpha(x, y int, pic *image.QPic, alpha float32) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}
	if pic == nil || dc.renderer == nil || alpha <= 0 {
		return
	}

	tex := dc.renderer.getTextureWithUVs(dc, pic)
	if tex.texture == 0 {
		return
	}

	rect := screenPicRect(x, y, pic)
	vertices := []quadVertex{
		{rect.x, rect.y, tex.u0, tex.v0},                   // Top-left
		{rect.x + rect.w, rect.y, tex.u1, tex.v0},          // Top-right
		{rect.x, rect.y + rect.h, tex.u0, tex.v1},          // Bottom-left
		{rect.x + rect.w, rect.y + rect.h, tex.u1, tex.v1}, // Bottom-right
	}

	// Render quad as triangle strip
	dc.render2DQuadTinted(vertices, tex.texture, dc.shader2D, [4]float32{1, 1, 1, minf(alpha, 1)})
}

// DrawMenuPic renders a QPic image in 320x200 menu-space coordinates.
func (dc *glDrawContext) DrawMenuPic(x, y int, pic *image.QPic) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}
	if pic == nil || dc.renderer == nil {
		return
	}

	tex := dc.renderer.getTextureWithUVs(dc, pic)
	if tex.texture == 0 {
		return
	}

	vertices := []quadVertex{
		{float32(x), float32(y), tex.u0, tex.v0},
		{float32(x + int(pic.Width)), float32(y), tex.u1, tex.v0},
		{float32(x), float32(y + int(pic.Height)), tex.u0, tex.v1},
		{float32(x + int(pic.Width)), float32(y + int(pic.Height)), tex.u1, tex.v1},
	}

	// Render quad as triangle strip
	dc.render2DQuad(vertices, tex.texture, dc.shader2D)
}

// DrawFill fills a rectangle with a Quake palette color.
func (dc *glDrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.DrawFillAlpha(x, y, w, h, color, 1)
}

// DrawFillAlpha fills a rectangle with a Quake palette color and explicit alpha.
func (dc *glDrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	if w <= 0 || h <= 0 || alpha <= 0 {
		return
	}
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}
	if dc.renderer == nil {
		return
	}
	if alpha > 1 {
		alpha = 1
	}

	r, g, b := GetPaletteColor(color, dc.renderer.palette)
	if IsTransparentIndex(color) {
		alpha = 0
	}
	if alpha <= 0 {
		return
	}

	vertices := []quadVertex{
		{float32(x), float32(y), 0.0, 0.0},
		{float32(x + w), float32(y), 1.0, 0.0},
		{float32(x), float32(y + h), 0.0, 1.0},
		{float32(x + w), float32(y + h), 1.0, 1.0},
	}
	dc.render2DSolidQuad(vertices, [4]float32{
		float32(r) / 255,
		float32(g) / 255,
		float32(b) / 255,
		alpha,
	})
}

// DrawCharacter renders a single character from font.
func (dc *glDrawContext) DrawCharacter(x, y int, num int) {
	dc.DrawCharacterAlpha(x, y, num, 1)
}

// DrawCharacterAlpha renders a single character from font with explicit alpha.
func (dc *glDrawContext) DrawCharacterAlpha(x, y int, num int, alpha float32) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}
	if dc.renderer == nil || num < 0 || num > 255 || alpha <= 0 {
		return
	}
	pic := dc.renderer.getCharPic(num)
	if pic == nil {
		return
	}
	tex := dc.renderer.getCharTextureWithUVs(dc, pic)
	if tex.texture == 0 {
		return
	}
	vertices := []quadVertex{
		{float32(x), float32(y), tex.u0, tex.v0},
		{float32(x + 8), float32(y), tex.u1, tex.v0},
		{float32(x), float32(y + 8), tex.u0, tex.v1},
		{float32(x + 8), float32(y + 8), tex.u1, tex.v1},
	}
	dc.render2DQuadTinted(vertices, tex.texture, dc.shader2D, [4]float32{1, 1, 1, minf(alpha, 1)})
}

// DrawMenuCharacter renders a single character from font in 320x200 menu space.
func (dc *glDrawContext) DrawMenuCharacter(x, y int, num int) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}
	if dc.renderer == nil || num < 0 || num > 255 {
		return
	}
	pic := dc.renderer.getCharPic(num)
	if pic == nil {
		return
	}
	tex := dc.renderer.getCharTextureWithUVs(dc, pic)
	if tex.texture == 0 {
		return
	}
	vertices := []quadVertex{
		{float32(x), float32(y), tex.u0, tex.v0},
		{float32(x + 8), float32(y), tex.u1, tex.v0},
		{float32(x), float32(y + 8), tex.u0, tex.v1},
		{float32(x + 8), float32(y + 8), tex.u1, tex.v1},
	}
	dc.render2DQuad(vertices, tex.texture, dc.shader2D)
}

// render2DQuad performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (dc *glDrawContext) render2DQuad(vertices []quadVertex, tex uint32, program uint32) {
	dc.render2DQuadTinted(vertices, tex, program, [4]float32{1, 1, 1, 1})
}

func (dc *glDrawContext) render2DQuadTinted(vertices []quadVertex, tex uint32, program uint32, tint [4]float32) {
	gl.UseProgram(program)

	// Use the active canvas transform, or compute a default from viewport.
	t := dc.canvas.Transform
	if dc.canvas.Type == CanvasNone {
		// Fallback: screen-pixel identity transform (equivalent to old uScreenSize path).
		w := float32(dc.viewport.width)
		h := float32(dc.viewport.height)
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}
		t = DrawTransform{
			Scale:  [2]float32{2.0 / w, -2.0 / h},
			Offset: [2]float32{-1.0, 1.0},
		}
	}

	scaleLoc := gl.GetUniformLocation(program, gl.Str("uCanvasScale\x00"))
	gl.Uniform2f(scaleLoc, t.Scale[0], t.Scale[1])
	offsetLoc := gl.GetUniformLocation(program, gl.Str("uCanvasOffset\x00"))
	gl.Uniform2f(offsetLoc, t.Offset[0], t.Offset[1])
	colorLoc := gl.GetUniformLocation(program, gl.Str("uColor\x00"))
	if colorLoc >= 0 {
		gl.Uniform4f(colorLoc, tint[0], tint[1], tint[2], tint[3])
	}

	// Upload vertices
	gl.BindBuffer(gl.ARRAY_BUFFER, dc.vbo2D)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(quadVertex{})), unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// Bind texture
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	texLoc := gl.GetUniformLocation(program, gl.Str("uTexture\x00"))
	gl.Uniform1i(texLoc, 0)

	// Draw quad
	gl.BindVertexArray(dc.vao2D)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)
}

func (dc *glDrawContext) render2DSolidQuad(vertices []quadVertex, color [4]float32) {
	gl.UseProgram(dc.shaderSolid)

	t := dc.canvas.Transform
	if dc.canvas.Type == CanvasNone {
		w := float32(dc.viewport.width)
		h := float32(dc.viewport.height)
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}
		t = DrawTransform{
			Scale:  [2]float32{2.0 / w, -2.0 / h},
			Offset: [2]float32{-1.0, 1.0},
		}
	}

	scaleLoc := gl.GetUniformLocation(dc.shaderSolid, gl.Str("uCanvasScale\x00"))
	gl.Uniform2f(scaleLoc, t.Scale[0], t.Scale[1])
	offsetLoc := gl.GetUniformLocation(dc.shaderSolid, gl.Str("uCanvasOffset\x00"))
	gl.Uniform2f(offsetLoc, t.Offset[0], t.Offset[1])
	colorLoc := gl.GetUniformLocation(dc.shaderSolid, gl.Str("uColor\x00"))
	gl.Uniform4f(colorLoc, color[0], color[1], color[2], color[3])

	gl.BindBuffer(gl.ARRAY_BUFFER, dc.vbo2D)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(quadVertex{})), unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)
	gl.BindVertexArray(dc.vao2D)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)
}

type Renderer struct {
	mu sync.RWMutex

	window *glfw.Window
	config Config

	framebufferWidth  atomic.Int32
	framebufferHeight atomic.Int32

	textureCache  map[glCacheKey]*glCachedTexture
	colorTextures [256]uint32
	palette       []byte
	concharsData  []byte
	charCache     [256]*image.QPic
	scrapAtlas    *ScrapAtlas
	scrapTextures []uint32
	scrapEntries  map[glCacheKey]*ScrapEntry
	drawContext   *glDrawContext

	cameraState                         CameraState
	viewMatrices                        ViewMatrixData
	worldData                           *WorldRenderData
	worldTree                           *bsp.Tree
	worldVAO                            uint32
	worldVBO                            uint32
	worldEBO                            uint32
	worldProgram                        uint32
	worldSkyProgram                     uint32
	worldSkyProceduralProgram           uint32
	worldSkyCubemapProgram              uint32
	worldSkyExternalFaceProgram         uint32
	worldVPUniform                      int32
	worldTextureUniform                 int32
	worldLightmapUniform                int32
	worldFullbrightUniform              int32
	worldHasFullbrightUniform           int32
	worldDynamicLightUniform            int32
	worldSkyVPUniform                   int32
	worldSkySolidUniform                int32
	worldSkyAlphaUniform                int32
	worldSkyProceduralVPUniform         int32
	worldSkyProceduralModelOffset       int32
	worldSkyProceduralModelRotation     int32
	worldSkyProceduralModelScale        int32
	worldSkyProceduralCameraOrigin      int32
	worldSkyProceduralFogColor          int32
	worldSkyProceduralFogDensity        int32
	worldSkyProceduralHorizonColor      int32
	worldSkyProceduralZenithColor       int32
	worldSkyCubemapVPUniform            int32
	worldSkyCubemapUniform              int32
	worldSkyExternalFaceVPUniform       int32
	worldSkyExternalFaceRTUniform       int32
	worldSkyExternalFaceBKUniform       int32
	worldSkyExternalFaceLFUniform       int32
	worldSkyExternalFaceFTUniform       int32
	worldSkyExternalFaceUPUniform       int32
	worldSkyExternalFaceDNUniform       int32
	worldModelOffsetUniform             int32
	worldModelRotationUniform           int32
	worldModelScaleUniform              int32
	worldSkyModelOffsetUniform          int32
	worldSkyModelRotationUniform        int32
	worldSkyModelScaleUniform           int32
	worldSkyCubemapModelOffsetUniform   int32
	worldSkyCubemapModelRotationUniform int32
	worldSkyCubemapModelScaleUniform    int32
	worldSkyExternalFaceModelOffset     int32
	worldSkyExternalFaceModelRotation   int32
	worldSkyExternalFaceModelScale      int32
	worldAlphaUniform                   int32
	worldTimeUniform                    int32
	worldSkyTimeUniform                 int32
	worldSkySolidLayerSpeedUniform      int32
	worldSkyAlphaLayerSpeedUniform      int32
	worldTurbulentUniform               int32
	worldLitWaterUniform                int32
	worldCameraOriginUniform            int32
	worldSkyCameraOriginUniform         int32
	worldSkyCubemapCameraOriginUniform  int32
	worldSkyExternalFaceCameraOrigin    int32
	worldFogColorUniform                int32
	worldSkyFogColorUniform             int32
	worldSkyCubemapFogColorUniform      int32
	worldSkyExternalFaceFogColor        int32
	worldFogDensityUniform              int32
	worldSkyFogDensityUniform           int32
	worldSkyCubemapFogDensityUniform    int32
	worldSkyExternalFaceFogDensity      int32
	worldIndexCount                     int32
	worldFallbackTexture                uint32
	worldLightmapFallback               uint32
	worldSkyAlphaFallback               uint32
	worldSkyExternalCubemap             uint32
	worldSkyExternalFaceTextures        [6]uint32
	worldSkyExternalMode                externalSkyboxRenderMode
	worldTextures                       map[int32]uint32
	worldFullbrightTextures             map[int32]uint32
	worldSkySolidTextures               map[int32]uint32
	worldSkyAlphaTextures               map[int32]uint32
	worldSkyFlatTextures                map[int32]uint32
	worldTextureAnimations              []*SurfaceTexture
	worldLightmaps                      []uint32
	worldHasLitWater                    bool
	worldLiquidFaceTypes                int32
	worldLiquidAlphaOverrides           worldLiquidAlphaOverrides
	worldSkyFogOverride                 worldSkyFogOverride
	worldSkyExternalName                string
	worldSkyExternalRequestID           uint64
	lightStyleValues                    [64]float32
	worldFogColor                       [3]float32
	worldFogDensity                     float32
	brushModels                         map[int]*glWorldMesh
	aliasModels                         map[string]*glAliasModel
	aliasEntityStates                   map[int]*AliasEntity
	viewModelAliasState                 *AliasEntity
	spriteModels                        map[string]*glSpriteModel
	aliasShadowTexture                  uint32
	aliasScratchVAO                     uint32
	aliasScratchVBO                     uint32
	particleProgram                     uint32
	particleVPUniform                   int32
	particlePointScaleUniform           int32
	particleVAO                         uint32
	particleVBO                         uint32
	decalProgram                        uint32
	decalVPUniform                      int32
	decalAtlasUniform                   int32
	decalAtlasTexture                   uint32
	decalVAO                            uint32
	decalVBO                            uint32

	// Frame-level translucency sorting (matches C ironwail: r_world.c)
	translucentCalls []worldDrawCall

	// Scene FBO and warpscale post-process for r_waterwarp == 1 underwater screen warp.
	// Mirrors C Ironwail: framebufs.scene / R_WarpScaleView / glprogs.warpscale[1].
	warpScaleProgram         uint32
	warpScaleSceneTex        int32 // uniform location: uSceneTex
	warpScaleUVScaleWarpTime int32 // uniform location: uUVScaleWarpTime
	sceneFBO                 uint32
	sceneColorTex            uint32
	sceneDepthRBO            uint32
	sceneFBOWidth            int
	sceneFBOHeight           int
	oitFB                    oitFramebuffers

	// OIT shader programs and cached uniform locations.
	oitWorldProgram              uint32
	oitWorldVPUniform            int32
	oitWorldTextureUniform       int32
	oitWorldLightmapUniform      int32
	oitWorldFullbrightUniform    int32
	oitWorldHasFullbrightUniform int32
	oitWorldDynamicLightUniform  int32
	oitWorldModelOffsetUniform   int32
	oitWorldModelRotationUniform int32
	oitWorldModelScaleUniform    int32
	oitWorldAlphaUniform         int32
	oitWorldTimeUniform          int32
	oitWorldTurbulentUniform     int32
	oitWorldLitWaterUniform      int32
	oitWorldCameraOriginUniform  int32
	oitWorldFogColorUniform      int32
	oitWorldFogDensityUniform    int32
	oitResolveProgram            uint32
	oitResolveMSAAProgram        uint32
	oitResolveAccumLoc           int32
	oitResolveRevealLoc          int32
	oitResolveMSAASamplesLoc     int32
	oitResolveVAO                uint32

	// Polyblend (v_blend) full-screen color tint pass.
	// Mirrors C Ironwail: glprogs.viewblend / V_PolyBlend().
	polyBlendProgram      uint32
	polyBlendColorUniform int32 // uniform location: uBlendColor

	lightPool      *glLightPool
	drawCallback   func(RenderContext)
	updateCallback func(dt float64)
	closeCallback  func()

	running bool
}

// New creates a new Renderer with configuration from cvars.
func New() (*Renderer, error) {
	return NewWithConfig(ConfigFromCvars())
}

// NewWithConfig creates a new Renderer with the specified configuration.
func NewWithConfig(cfg Config) (*Renderer, error) {
	slog.Debug("Creating OpenGL renderer", "width", cfg.Width, "height", cfg.Height, "fullscreen", cfg.Fullscreen)
	fmt.Println("Video initialization")

	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize glfw: %w", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.False) // Hide initially
	glfw.WindowHint(glfw.StencilBits, 8)

	window, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	if cfg.Fullscreen {
		monitor := glfw.GetPrimaryMonitor()
		mode := monitor.GetVideoMode()
		window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
	}

	window.MakeContextCurrent()

	// Initialize Glow (go function bindings)
	if err := gl.Init(); err != nil {
		window.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("failed to initialize gl: %w", err)
	}

	// Enable VSync if requested
	if cfg.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}

	// Set up OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.Enable(gl.CULL_FACE)
	gl.CullFace(gl.BACK)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Show the window
	window.Show()

	vendor := gl.GoStr(gl.GetString(gl.VENDOR))
	rendererName := gl.GoStr(gl.GetString(gl.RENDERER))
	version := gl.GoStr(gl.GetString(gl.VERSION))
	refreshRate := 0
	if cfg.Fullscreen {
		if monitor := glfw.GetPrimaryMonitor(); monitor != nil {
			if mode := monitor.GetVideoMode(); mode != nil {
				refreshRate = mode.RefreshRate
			}
		}
	}
	if refreshRate > 0 {
		fmt.Printf("Video mode: %d x %d 24bit Z24 S8 %dHz\n", cfg.Width, cfg.Height, refreshRate)
	} else {
		fmt.Printf("Video mode: %d x %d 24bit Z24 S8\n", cfg.Width, cfg.Height)
	}
	fmt.Printf("GL_VENDOR:   %s\n", vendor)
	fmt.Printf("GL_RENDERER: %s\n", rendererName)
	fmt.Printf("GL_VERSION:  %s\n", version)

	r := &Renderer{
		window:                  window,
		config:                  cfg,
		textureCache:            make(map[glCacheKey]*glCachedTexture),
		worldTextures:           make(map[int32]uint32),
		worldFullbrightTextures: make(map[int32]uint32),
		worldSkySolidTextures:   make(map[int32]uint32),
		worldSkyAlphaTextures:   make(map[int32]uint32),
		worldSkyFlatTextures:    make(map[int32]uint32),
		brushModels:             make(map[int]*glWorldMesh),
		aliasModels:             make(map[string]*glAliasModel),
		aliasEntityStates:       make(map[int]*AliasEntity),
		spriteModels:            make(map[string]*glSpriteModel),
		lightStyleValues:        defaultLightStyleValues(),
		lightPool:               NewGLLightPool(512),
	}
	r.framebufferWidth.Store(int32(cfg.Width))
	r.framebufferHeight.Store(int32(cfg.Height))
	r.initScrapAtlas()

	slog.Info("OpenGL renderer created",
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
		"maxfps", cfg.MaxFPS,
		"gl_version", version,
	)

	return r, nil
}

// OnDraw sets the callback for frame rendering.
func (r *Renderer) OnDraw(callback func(RenderContext)) {
	r.mu.Lock()
	r.drawCallback = callback
	r.mu.Unlock()
}

// OnUpdate sets the callback for game logic updates.
func (r *Renderer) OnUpdate(callback func(dt float64)) {
	r.mu.Lock()
	r.updateCallback = callback
	r.mu.Unlock()
}

// OnClose sets the callback for window close events.
func (r *Renderer) OnClose(callback func()) {
	r.mu.Lock()
	r.closeCallback = callback
	r.mu.Unlock()
}

// Input returns the input state for keyboard and mouse polling.
func (r *Renderer) Input() interface{} {
	// OpenGL backend does not expose a polling object yet.
	// Input handling currently flows through host/input callbacks.
	return nil
}

// Size returns the cached framebuffer size in pixels.
// The render loop refreshes this once per frame from GLFW so hot callback paths
// don't repeatedly query window attributes from X11/GLFW.
func (r *Renderer) Size() (width, height int) {
	if r == nil {
		return 0, 0
	}
	width = int(r.framebufferWidth.Load())
	height = int(r.framebufferHeight.Load())
	if width <= 0 {
		width = r.config.Width
	}
	if height <= 0 {
		height = r.config.Height
	}
	return width, height
}

// CaptureScreenshot reads the current OpenGL framebuffer and saves it as a PNG.
func (r *Renderer) CaptureScreenshot(filename string) error {
	if r.window == nil {
		return fmt.Errorf("capture screenshot: renderer window is nil")
	}

	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("capture screenshot: invalid framebuffer size %dx%d", width, height)
	}

	pixels := make([]byte, width*height*4)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&pixels[0]))

	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
	rowBytes := width * 4
	for y := 0; y < height; y++ {
		src := (height - 1 - y) * rowBytes
		dst := y * img.Stride
		row := img.Pix[dst : dst+rowBytes]
		copy(row, pixels[src:src+rowBytes])
		for x := 3; x < rowBytes; x += 4 {
			row[x] = 0xff
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("capture screenshot: create file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("capture screenshot: encode png: %w", err)
	}

	return nil
}

// ScaleFactor returns the DPI scale factor.
func (r *Renderer) ScaleFactor() float64 {
	monitor := r.window.GetMonitor()
	if monitor == nil {
		monitor = glfw.GetPrimaryMonitor()
	}
	if monitor == nil {
		return 1.0
	}
	xscale, _ := monitor.GetContentScale()
	return float64(xscale)
}

// Config returns the current video configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the video configuration.
func (r *Renderer) SetConfig(cfg Config) {
	r.mu.Lock()
	r.config = cfg
	r.mu.Unlock()

	if r.window == nil {
		return
	}

	// Apply fullscreen change
	if cfg.Fullscreen {
		monitor := glfw.GetPrimaryMonitor()
		if monitor != nil {
			mode := monitor.GetVideoMode()
			r.window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
		}
	} else {
		r.window.SetMonitor(nil, 0, 0, cfg.Width, cfg.Height, 0)
	}

	// Apply VSync change
	if cfg.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}
}

// SpawnDynamicLight adds a temporary point light to the world.
// The light will fade over its lifetime and be automatically removed when expired.
// Returns true if the light was added, false if the pool is at capacity.
func (r *Renderer) SpawnDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnLight(light)
}

// SpawnKeyedDynamicLight adds or replaces a keyed dynamic light.
// If light.EntityKey != 0, any existing light with the same key is replaced in-place,
// matching C's CL_AllocDlight per-entity slot reuse behavior.
func (r *Renderer) SpawnKeyedDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnOrReplaceKeyed(light)
}

// GetDynamicLightPool returns the light pool for direct access (read-only recommended).
// Use this to query active lights or manually manage the pool.
func (r *Renderer) GetDynamicLightPool() *glLightPool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lightPool
}

// UpdateLights advances all active lights by the given deltaTime.
// This should be called once per frame, typically in the game loop's update phase.
// It handles aging lights and removing expired ones automatically.
func (r *Renderer) UpdateLights(deltaTime float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.UpdateAndFilter(deltaTime)
	}
}

// ClearDynamicLights removes all active gameplay lights from the renderer.
func (r *Renderer) ClearDynamicLights() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.Clear()
	}
}

// Run starts the main rendering loop.
func (r *Renderer) Run() error {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	slog.Info("Starting OpenGL render loop")

	// Frame timing
	lastTime := time.Now()
	frameTime := time.Second / time.Duration(r.config.MaxFPS)

	for !r.window.ShouldClose() {
		// Frame rate limiting
		if r.config.MaxFPS > 0 && !r.config.VSync {
			elapsed := time.Since(lastTime)
			if elapsed < frameTime {
				time.Sleep(frameTime - elapsed)
			}
		}

		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Process window events
		glfw.PollEvents()

		// Call update callback
		r.mu.RLock()
		updateCallback := r.updateCallback
		r.mu.RUnlock()
		if updateCallback != nil {
			updateCallback(dt)
		}

		// Render — use GetFramebufferSize for the GL viewport and all 2D
		// drawing coordinate calculations.  GLFW's GetSize() returns logical
		// screen coordinates which can differ from framebuffer pixels on
		// HiDPI/Wayland displays; GetFramebufferSize() always returns the
		// actual framebuffer pixel dimensions that OpenGL expects.
		width, height := r.window.GetFramebufferSize()
		r.framebufferWidth.Store(int32(width))
		r.framebufferHeight.Store(int32(height))
		gl.Viewport(0, 0, int32(width), int32(height))

		r.mu.RLock()
		drawCallback := r.drawCallback
		gamma := r.config.Gamma
		r.mu.RUnlock()

		if drawCallback != nil {
			r.mu.Lock()
			if r.drawContext == nil {
				r.drawContext = &glDrawContext{
					window:   r.window,
					renderer: r,
				}
			}
			r.drawContext.gamma = gamma
			// Reset canvas when viewport dimensions change so that
			// SetCanvas recomputes the transform for the new size.
			// Without this, the SetCanvas early-exit (same type → no-op)
			// keeps a stale transform from a previous frame's dimensions,
			// which causes mis-scaled menus on Wayland where the initial
			// window size can differ from the final compositor-assigned size.
			if r.drawContext.viewport.width != width || r.drawContext.viewport.height != height {
				r.drawContext.canvas.Type = CanvasNone
			}
			r.drawContext.viewport = struct {
				width  int
				height int
			}{width, height}
			gldc := r.drawContext
			r.mu.Unlock()
			r.uploadDirtyScrapPages(gldc)

			dc := &DrawContext{gldc: gldc}
			drawCallback(dc)
		}

		// Swap buffers
		r.window.SwapBuffers()
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	// Call close callback
	r.mu.RLock()
	closeCallback := r.closeCallback
	r.mu.RUnlock()
	if closeCallback != nil {
		closeCallback()
	}

	slog.Info("OpenGL render loop ended")
	return nil
}

// Stop requests the renderer to stop the main loop.
func (r *Renderer) Stop() {
	r.window.SetShouldClose(true)
}

// Shutdown releases all GPU resources and destroys the window.
func (r *Renderer) Shutdown() {
	slog.Debug("OpenGL renderer shutting down")
	r.ClearWorld()
	r.mu.Lock()
	r.clearParticleResourcesLocked()
	r.mu.Unlock()
	r.destroySceneFBO()
	if r.warpScaleProgram != 0 {
		gl.DeleteProgram(r.warpScaleProgram)
		r.warpScaleProgram = 0
	}
	if r.polyBlendProgram != 0 {
		gl.DeleteProgram(r.polyBlendProgram)
		r.polyBlendProgram = 0
	}
	r.destroyOITShaders()
	r.deleteAllTextures()
	if r.window != nil {
		r.window.Destroy()
	}
	glfw.Terminate()
}

// SetPalette sets the Quake palette used for rendering.
func (r *Renderer) SetPalette(palette []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.palette = make([]byte, len(palette))
	copy(r.palette, palette)
	r.clearTextureCacheLocked()
}

// SetConchars stores the raw conchars bitmap for character rendering.
func (r *Renderer) SetConchars(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(data) < 128*128 {
		return
	}
	r.concharsData = make([]byte, len(data))
	copy(r.concharsData, data)
	r.charCache = [256]*image.QPic{}
	r.clearTextureCacheLocked()
}

// clearTextureCacheLocked performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) clearTextureCacheLocked() {
	for key, entry := range r.textureCache {
		if entry != nil && entry.id != 0 {
			tex := entry.id
			gl.DeleteTextures(1, &tex)
		}
		delete(r.textureCache, key)
	}
	for i, tex := range r.colorTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
			r.colorTextures[i] = 0
		}
	}
	r.destroyScrapAtlas()
	r.initScrapAtlas()
}

// deleteAllTextures performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) deleteAllTextures() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearTextureCacheLocked()
}

// getOrCreateTexture performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) getOrCreateTexture(dc *glDrawContext, pic *image.QPic) uint32 {
	if pic == nil {
		return 0
	}

	r.mu.RLock()
	if entry, ok := r.textureCache[glCacheKey{pic: pic}]; ok && entry != nil {
		tex := entry.id
		r.mu.RUnlock()
		return tex
	}
	palette := append([]byte(nil), r.palette...)
	r.mu.RUnlock()

	rgba := ConvertPaletteToRGBA(pic.Pixels, palette)
	tex := dc.uploadQPicTexture(pic, rgba)
	if tex == 0 {
		return 0
	}

	r.mu.Lock()
	r.textureCache[glCacheKey{pic: pic}] = &glCachedTexture{id: tex, width: int(pic.Width), height: int(pic.Height)}
	r.mu.Unlock()
	return tex
}

func (r *Renderer) getTextureWithUVs(dc *glDrawContext, pic *image.QPic) glTextureWithUV {
	if pic == nil {
		return glTextureWithUV{}
	}
	if r.scrapAtlas != nil && pic.Width <= 128 && pic.Height <= 128 {
		if entry, tex, ok := r.tryScrapAlloc(dc, pic); ok && tex != 0 {
			return glTextureWithUV{
				texture: tex,
				u0:      entry.UV.U0,
				v0:      entry.UV.V0,
				u1:      entry.UV.U1,
				v1:      entry.UV.V1,
			}
		}
	}
	tex := r.getOrCreateTexture(dc, pic)
	return glTextureWithUV{
		texture: tex,
		u0:      0,
		v0:      0,
		u1:      1,
		v1:      1,
	}
}

// getOrCreateColorTexture performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) getOrCreateColorTexture(dc *glDrawContext, color byte) uint32 {
	r.mu.RLock()
	if tex := r.colorTextures[color]; tex != 0 {
		r.mu.RUnlock()
		return tex
	}
	palette := append([]byte(nil), r.palette...)
	r.mu.RUnlock()

	rgba := make([]byte, 4)
	if IsTransparentIndex(color) {
		rgba[3] = 0
	} else {
		pr, pg, pb := GetPaletteColor(color, palette)
		rgba[0], rgba[1], rgba[2], rgba[3] = pr, pg, pb, 255
	}
	pic := &image.QPic{Width: 1, Height: 1, Pixels: []byte{color}}
	tex := dc.uploadQPicTexture(pic, rgba)
	if tex == 0 {
		return 0
	}

	r.mu.Lock()
	r.colorTextures[color] = tex
	r.mu.Unlock()
	return tex
}

// getCharPic performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) getCharPic(num int) *image.QPic {
	r.mu.RLock()
	if num < 0 || num > 255 || len(r.concharsData) < 128*128 {
		r.mu.RUnlock()
		return nil
	}
	if r.charCache[num] != nil {
		pic := r.charCache[num]
		r.mu.RUnlock()
		return pic
	}
	concharsData := r.concharsData
	r.mu.RUnlock()

	col := num % 16
	row := num / 16
	pixels := make([]byte, 8*8)
	for y := 0; y < 8; y++ {
		src := (row*8+y)*128 + col*8
		copy(pixels[y*8:y*8+8], concharsData[src:src+8])
	}
	pic := &image.QPic{Width: 8, Height: 8, Pixels: pixels}

	r.mu.Lock()
	r.charCache[num] = pic
	r.mu.Unlock()
	return pic
}

// getOrCreateCharTexture performs its step in the primary OpenGL backend that orchestrates Quake's frame passes and GL state transitions; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (r *Renderer) getOrCreateCharTexture(dc *glDrawContext, pic *image.QPic) uint32 {
	if pic == nil {
		return 0
	}

	r.mu.RLock()
	if entry, ok := r.textureCache[glCacheKey{pic: pic}]; ok && entry != nil {
		tex := entry.id
		r.mu.RUnlock()
		return tex
	}
	palette := append([]byte(nil), r.palette...)
	r.mu.RUnlock()

	rgba := ConvertConcharsToRGBA(pic.Pixels, palette)
	tex := dc.uploadQPicTexture(pic, rgba)
	if tex == 0 {
		return 0
	}

	r.mu.Lock()
	r.textureCache[glCacheKey{pic: pic}] = &glCachedTexture{id: tex, width: int(pic.Width), height: int(pic.Height)}
	r.mu.Unlock()
	return tex
}

func (r *Renderer) getCharTextureWithUVs(dc *glDrawContext, pic *image.QPic) glTextureWithUV {
	if pic == nil {
		return glTextureWithUV{}
	}
	if r.scrapAtlas != nil && pic.Width <= 128 && pic.Height <= 128 {
		if entry, tex, ok := r.tryScrapAllocConchars(dc, pic); ok && tex != 0 {
			return glTextureWithUV{
				texture: tex,
				u0:      entry.UV.U0,
				v0:      entry.UV.V0,
				u1:      entry.UV.U1,
				v1:      entry.UV.V1,
			}
		}
	}
	tex := r.getOrCreateCharTexture(dc, pic)
	return glTextureWithUV{
		texture: tex,
		u0:      0,
		v0:      0,
		u1:      1,
		v1:      1,
	}
}

// IsRunning returns true if the render loop is active.
func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}
