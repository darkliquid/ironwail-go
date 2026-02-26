//go:build opengl
// +build opengl

package renderer

package renderer

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/ironwail/ironwail-go/internal/image"
)

// 2D rendering shaders
const (
	vertexShader2D = `#version 410 core
in vec2 aPosition;
in vec2 aTexCoord;
out vec2 vTexCoord;
uniform vec2 uScreenSize;

void main() {
	// Convert pixel coordinates to clip space (-1 to 1)
	vec2 clipPos = (aPosition / uScreenSize) * 2.0 - 1.0;
	gl_Position = vec4(clipPos.x, -clipPos.y, 0.0, 1.0);
	vTexCoord = aTexCoord;
}`

	fragmentShader2D = `#version 410 core
in vec2 vTexCoord;
out vec4 fragColor;
uniform sampler2D uTexture;

void main() {
	fragColor = texture(uTexture, vTexCoord);
}`

	vertexShaderSolid = `#version 410 core
in vec2 aPosition;
uniform vec2 uScreenSize;
uniform vec4 uColor;
out vec4 vColor;

void main() {
	vec2 clipPos = (aPosition / uScreenSize) * 2.0 - 1.0;
	gl_Position = vec4(clipPos.x, -clipPos.y, 0.0, 1.0);
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
	x, y     float32
	u, v     float32
}

type glDrawContext struct {
	window   *glfw.Window
	gamma    float32
	viewport struct {
		width  int
		height int
	}
	// 2D rendering state
	shader2D       uint32
	shaderSolid    uint32
	vao2D         uint32
	vbo2D         uint32
	initialized2D bool
}
func init() {
	// OpenGL must run on main OS thread
	runtime.LockOSThread()
}

type glDrawContext struct {
	window   *glfw.Window
	gamma    float32
	viewport struct {
		width  int
		height int
	}
}

func (dc *glDrawContext) Clear(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

func (dc *glDrawContext) DrawTriangle(r, g, b, a float32) {
	// TODO: Implement modern OpenGL triangle rendering with shaders and VBOs
	// For now, just clear to the triangle color as a simple test
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT)
}

func (dc *glDrawContext) SurfaceView() interface{} {
	// In a full implementation, this would return an OpenGL texture view
	return nil
}

func (dc *glDrawContext) Gamma() float32 {
	return dc.gamma
}

// 2D Drawing API implementation

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

	// Position attribute (x, y)
	posAttr := gl.GetAttribLocation(dc.shader2D, gl.Str("aPosition\x00"))
	gl.EnableVertexAttribArray(uint32(posAttr))
	gl.VertexAttribPointerWithOffset(uint32(posAttr), 2, gl.FLOAT, false, 16, uintptr(0))

	// TexCoord attribute (u, v)
	texAttr := gl.GetAttribLocation(dc.shader2D, gl.Str("aTexCoord\x00"))
	gl.EnableVertexAttribArray(uint32(texAttr))
	gl.VertexAttribPointerWithOffset(uint32(texAttr), 2, gl.FLOAT, false, 16, 8)

	gl.BindVertexArray(0)

	dc.initialized2D = true
	return nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	cstr, free := gl.Strs(source)
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

func (dc *glDrawContext) uploadQPicTexture(pic *image.QPic) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	// Upload pixel data as grayscale (will use palette in shader)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RED, int32(pic.Width), int32(pic.Height), 0, gl.RED, gl.UNSIGNED_BYTE, unsafe.Pointer(&pic.Pixels[0]))

	// Set texture parameters
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	return tex
}

// DrawPic renders a QPic image at specified position.
func (dc *glDrawContext) DrawPic(x, y int, pic *image.QPic) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}

	// Upload texture (in real implementation, cache this)
	tex := dc.uploadQPicTexture(pic)
	defer gl.DeleteTextures(1, &tex)


	// Create quad vertices (x, y, u, v)
	w, h := int(pic.Width), int(pic.Height)
	vertices := []quadVertex{
		{float32(x), float32(y), 0.0, 0.0}, // Top-left
		{float32(x + w), float32(y), 1.0, 0.0}, // Top-right
		{float32(x), float32(y + h), 0.0, 1.0}, // Bottom-left
		{float32(x + w), float32(y + h), 1.0, 1.0}, // Bottom-right
	}

	// Render quad as triangle strip
	dc.render2DQuad(vertices, tex, dc.shader2D)
}

// DrawFill fills a rectangle with a Quake palette color.
func (dc *glDrawContext) DrawFill(x, y, w, h int, color byte) {
	if err := dc.init2DRenderer(); err != nil {
		slog.Error("Failed to init 2D renderer", "error", err)
		return
	}

	// Convert palette index to RGB (simplified - in real impl use palette.lmp)
	// For now, use gray scale based on color byte
	c := float32(color) / 255.0

	// Use solid color shader
	gl.UseProgram(dc.shaderSolid)

	// Set color uniform
	colorLoc := gl.GetUniformLocation(dc.shaderSolid, gl.Str("uColor\x00"))
	gl.Uniform4f(colorLoc, c, c, c, 1.0)

	// Set screen size uniform
	screenLoc := gl.GetUniformLocation(dc.shaderSolid, gl.Str("uScreenSize\x00"))
	gl.Uniform2f(screenLoc, float32(dc.viewport.width), float32(dc.viewport.height))

	// Create quad vertices (just x, y for solid shader)
	// We reuse the quad vertex struct, only using x, y
	vertices := []quadVertex{
		{float32(x), float32(y), 0.0, 0.0},
		{float32(x + w), float32(y), 0.0, 0.0},
		{float32(x), float32(y + h), 0.0, 0.0},
		{float32(x + w), float32(y + h), 0.0, 0.0},
	}

	// Upload vertices
	gl.BindBuffer(gl.ARRAY_BUFFER, dc.vbo2D)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(unsafe.Sizeof(quadVertex{})), unsafe.Pointer(&vertices[0]), gl.STATIC_DRAW)

	// For solid shader, only set position attribute
	posAttr := gl.GetAttribLocation(dc.shaderSolid, gl.Str("aPosition\x00"))
	gl.EnableVertexAttribArray(uint32(posAttr))
	gl.VertexAttribPointerWithOffset(uint32(posAttr), 2, gl.FLOAT, false, 16, uintptr(0))
	gl.DisableVertexAttribArray(uint32(gl.GetAttribLocation(dc.shaderSolid, gl.Str("aTexCoord\x00"))))

	// Draw quad
	gl.BindVertexArray(dc.vao2D)
	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
	gl.BindVertexArray(0)
}

// DrawCharacter renders a single character from font.
func (dc *glDrawContext) DrawCharacter(x, y int, num int) {
	// TODO: Implement proper character rendering from CONCHARS texture
	// For now, just draw a simple box
	dc.DrawFill(x, y, 8, 8, byte(num%255))
}

func (dc *glDrawContext) render2DQuad(vertices []quadVertex, tex uint32, program uint32) {
	gl.UseProgram(program)

	// Set screen size uniform
	screenLoc := gl.GetUniformLocation(program, gl.Str("uScreenSize\x00"))
	gl.Uniform2f(screenLoc, float32(dc.viewport.width), float32(dc.viewport.height))

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
type Renderer struct {
	mu sync.RWMutex

	window *glfw.Window
	config Config

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

	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize glfw: %w", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.False) // Hide initially

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

	// Show the window
	window.Show()

	r := &Renderer{
		window: window,
		config: cfg,
	}

	slog.Info("OpenGL renderer created",
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
		"maxfps", cfg.MaxFPS,
		"gl_version", gl.GoStr(gl.GetString(gl.VERSION)),
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
	// TODO: Implement input polling for GLFW
	return nil
}

// Size returns the current window size in pixels.
func (r *Renderer) Size() (width, height int) {
	return r.window.GetSize()
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

	// Apply fullscreen change
	if cfg.Fullscreen {
		monitor := glfw.GetPrimaryMonitor()
		mode := monitor.GetVideoMode()
		r.window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
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

		// Render
		width, height := r.window.GetSize()
		gl.Viewport(0, 0, int32(width), int32(height))

		r.mu.RLock()
		drawCallback := r.drawCallback
		gamma := r.config.Gamma
		r.mu.RUnlock()

		if drawCallback != nil {
			dc := &glDrawContext{
				window: r.window,
				gamma:  gamma,
				viewport: struct {
					width  int
					height int
				}{width, height},
			}
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
	if r.window != nil {
		r.window.Destroy()
	}
	glfw.Terminate()
}

// IsRunning returns true if the render loop is active.
func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}
