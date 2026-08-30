// pass_isolate.go provides viewport attachment inspection and runtime pass isolation.
package renderer

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// PassIsolateMode defines which pass is isolated on the viewport.
type PassIsolateMode int

const (
	PassIsolateNormal      PassIsolateMode = 0 // 0: Normal full rendering
	PassIsolateAccum       PassIsolateMode = 1 // 1: Raw OIT accumulation buffer (accum RGB)
	PassIsolateReveal      PassIsolateMode = 2 // 2: Raw OIT revealage buffer
	PassIsolateDepth       PassIsolateMode = 3 // 3: Scene depth buffer
	PassIsolateOpaque      PassIsolateMode = 4 // 4: Opaque geometry only (no translucency, UI)
	PassIsolateTranslucent PassIsolateMode = 5 // 5: Translucent geometry only (over black background)
)

var globalPassIsolateMode atomic.Int32

func (m PassIsolateMode) String() string {
	switch m {
	case PassIsolateNormal:
		return "normal"
	case PassIsolateAccum:
		return "accum"
	case PassIsolateReveal:
		return "reveal"
	case PassIsolateDepth:
		return "depth"
	case PassIsolateOpaque:
		return "opaque"
	case PassIsolateTranslucent:
		return "translucent"
	default:
		return "unknown"
	}
}

// ParsePassIsolateMode converts a string or numeric mode into a PassIsolateMode.
func ParsePassIsolateMode(s string) (PassIsolateMode, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "0", "normal", "off":
		return PassIsolateNormal, nil
	case "1", "accum", "oit_accum":
		return PassIsolateAccum, nil
	case "2", "reveal", "oit_reveal":
		return PassIsolateReveal, nil
	case "3", "depth", "z":
		return PassIsolateDepth, nil
	case "4", "opaque", "world_opaque":
		return PassIsolateOpaque, nil
	case "5", "translucent", "trans", "water":
		return PassIsolateTranslucent, nil
	}
	if v, err := strconv.Atoi(s); err == nil && v >= 0 && v <= 5 {
		return PassIsolateMode(v), nil
	}
	return PassIsolateNormal, fmt.Errorf("unknown pass isolate mode: %q", s)
}

// GetPassIsolateMode returns the active pass isolation mode.
func GetPassIsolateMode() PassIsolateMode {
	if pkgCVars != nil {
		if cv := pkgCVars.Get(CvarRPassIsolate); cv != nil {
			return PassIsolateMode(cv.Int)
		}
	}
	return PassIsolateMode(globalPassIsolateMode.Load())
}

// SetPassIsolateMode sets the active pass isolation mode.
func SetPassIsolateMode(mode PassIsolateMode) {
	globalPassIsolateMode.Store(int32(mode))
	if pkgCVars != nil {
		pkgCVars.Set(CvarRPassIsolate, strconv.Itoa(int(mode)))
	}
}

// --- Live Pass Isolation (r_pass_isolate 1..3) Shaders and Pipelines --------

const passIsolateVertexShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var uvs = array<vec2<f32>, 3>(
        vec2<f32>(0.0,  1.0),
        vec2<f32>(2.0,  1.0),
        vec2<f32>(0.0, -1.0),
    );

    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}
`

const passIsolateAccumFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var isolateSampler: sampler;

@group(0) @binding(1)
var accumTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let accum = textureSample(accumTexture, isolateSampler, input.uv);
    return vec4<f32>(accum.rgb, 1.0);
}
`

const passIsolateRevealFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var isolateSampler: sampler;

@group(0) @binding(1)
var revealTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let rev = textureSample(revealTexture, isolateSampler, input.uv).r;
    return vec4<f32>(rev, rev, rev, 1.0);
}
`

const passIsolateBlitFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0)
var isolateSampler: sampler;

@group(0) @binding(1)
var blitTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return textureSample(blitTexture, isolateSampler, input.uv);
}
`

func (r *Renderer) destroyPassIsolateResourcesLocked() {
	if r.resources.PassIsolateDepthBindGroup != nil {
		r.resources.PassIsolateDepthBindGroup.Release()
		r.resources.PassIsolateDepthBindGroup = nil
	}
	if r.resources.PassIsolateDepthTextureView != nil {
		r.resources.PassIsolateDepthTextureView.Release()
		r.resources.PassIsolateDepthTextureView = nil
	}
	if r.resources.PassIsolateDepthTexture != nil {
		r.resources.PassIsolateDepthTexture.Release()
		r.resources.PassIsolateDepthTexture = nil
	}
	r.resources.PassIsolateDepthWidth = 0
	r.resources.PassIsolateDepthHeight = 0

	if r.resources.PassIsolateAccumBindGroup != nil {
		r.resources.PassIsolateAccumBindGroup.Release()
		r.resources.PassIsolateAccumBindGroup = nil
	}
	if r.resources.PassIsolateRevealBindGroup != nil {
		r.resources.PassIsolateRevealBindGroup.Release()
		r.resources.PassIsolateRevealBindGroup = nil
	}
	if r.resources.PassIsolateAccumPipeline != nil {
		r.resources.PassIsolateAccumPipeline.Release()
		r.resources.PassIsolateAccumPipeline = nil
	}
	if r.resources.PassIsolateRevealPipeline != nil {
		r.resources.PassIsolateRevealPipeline.Release()
		r.resources.PassIsolateRevealPipeline = nil
	}
	if r.resources.PassIsolateBlitPipeline != nil {
		r.resources.PassIsolateBlitPipeline.Release()
		r.resources.PassIsolateBlitPipeline = nil
	}
	if r.resources.PassIsolateSampler != nil {
		r.resources.PassIsolateSampler.Release()
		r.resources.PassIsolateSampler = nil
	}
	if r.resources.PassIsolatePipelineLayout != nil {
		r.resources.PassIsolatePipelineLayout.Release()
		r.resources.PassIsolatePipelineLayout = nil
	}
	if r.resources.PassIsolateBindGroupLayout != nil {
		r.resources.PassIsolateBindGroupLayout.Release()
		r.resources.PassIsolateBindGroupLayout = nil
	}
	if r.resources.PassIsolateVertexShader != nil {
		r.resources.PassIsolateVertexShader.Release()
		r.resources.PassIsolateVertexShader = nil
	}
	if r.resources.PassIsolateAccumFragmentShader != nil {
		r.resources.PassIsolateAccumFragmentShader.Release()
		r.resources.PassIsolateAccumFragmentShader = nil
	}
	if r.resources.PassIsolateRevealFragmentShader != nil {
		r.resources.PassIsolateRevealFragmentShader.Release()
		r.resources.PassIsolateRevealFragmentShader = nil
	}
	if r.resources.PassIsolateBlitFragmentShader != nil {
		r.resources.PassIsolateBlitFragmentShader.Release()
		r.resources.PassIsolateBlitFragmentShader = nil
	}
}

func (r *Renderer) ensurePassIsolateResourcesLocked(device *wgpu.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.resources.PassIsolateAccumPipeline != nil &&
		r.resources.PassIsolateRevealPipeline != nil &&
		r.resources.PassIsolateBlitPipeline != nil &&
		r.resources.PassIsolateBindGroupLayout != nil &&
		r.resources.PassIsolateSampler != nil {
		return nil
	}

	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Pass Isolate BGL",
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
		return fmt.Errorf("create pass isolate BGL: %w", err)
	}

	pl, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Pass Isolate Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		bgl.Release()
		return fmt.Errorf("create pass isolate pipeline layout: %w", err)
	}

	vs, err := createWorldShaderModule(device, passIsolateVertexShaderWGSL, "Pass Isolate Vertex Shader")
	if err != nil {
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate vertex shader: %w", err)
	}

	fsAccum, err := createWorldShaderModule(device, passIsolateAccumFragmentShaderWGSL, "Pass Isolate Accum Fragment Shader")
	if err != nil {
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate accum fragment shader: %w", err)
	}

	fsReveal, err := createWorldShaderModule(device, passIsolateRevealFragmentShaderWGSL, "Pass Isolate Reveal Fragment Shader")
	if err != nil {
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate reveal fragment shader: %w", err)
	}

	fsBlit, err := createWorldShaderModule(device, passIsolateBlitFragmentShaderWGSL, "Pass Isolate Blit Fragment Shader")
	if err != nil {
		fsReveal.Release()
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate blit fragment shader: %w", err)
	}

	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "Pass Isolate Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  1,
	})
	if err != nil {
		fsBlit.Release()
		fsReveal.Release()
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate sampler: %w", err)
	}

	createPipe := func(fs *wgpu.ShaderModule, label string) (*wgpu.RenderPipeline, error) {
		return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
			Label:  label,
			Layout: pl,
			Vertex: wgpu.VertexState{
				Module:     vs,
				EntryPoint: "vs_main",
			},
			Primitive: gputypes.PrimitiveState{
				Topology:  gputypes.PrimitiveTopologyTriangleList,
				FrontFace: gputypes.FrontFaceCCW,
				CullMode:  gputypes.CullModeNone,
			},
			Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
			Fragment: &wgpu.FragmentState{
				Module:     fs,
				EntryPoint: "fs_main",
				Targets: []gputypes.ColorTargetState{{
					Format:    r.sceneSurfaceFormat(),
					WriteMask: gputypes.ColorWriteMaskAll,
				}},
			},
		})
	}

	accumPipe, err := createPipe(fsAccum, "Pass Isolate Accum Pipeline")
	if err != nil {
		sampler.Release()
		fsBlit.Release()
		fsReveal.Release()
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate accum pipeline: %w", err)
	}

	revealPipe, err := createPipe(fsReveal, "Pass Isolate Reveal Pipeline")
	if err != nil {
		accumPipe.Release()
		sampler.Release()
		fsBlit.Release()
		fsReveal.Release()
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate reveal pipeline: %w", err)
	}

	blitPipe, err := createPipe(fsBlit, "Pass Isolate Blit Pipeline")
	if err != nil {
		revealPipe.Release()
		accumPipe.Release()
		sampler.Release()
		fsBlit.Release()
		fsReveal.Release()
		fsAccum.Release()
		vs.Release()
		pl.Release()
		bgl.Release()
		return fmt.Errorf("create pass isolate blit pipeline: %w", err)
	}

	r.resources.PassIsolateVertexShader = vs
	r.resources.PassIsolateAccumFragmentShader = fsAccum
	r.resources.PassIsolateRevealFragmentShader = fsReveal
	r.resources.PassIsolateBlitFragmentShader = fsBlit
	r.resources.PassIsolateBindGroupLayout = bgl
	r.resources.PassIsolatePipelineLayout = pl
	r.resources.PassIsolateSampler = sampler
	r.resources.PassIsolateAccumPipeline = accumPipe
	r.resources.PassIsolateRevealPipeline = revealPipe
	r.resources.PassIsolateBlitPipeline = blitPipe
	return nil
}

func (dc *DrawContext) clearSurfaceView(surfaceView *wgpu.TextureView, color [4]float32) {
	if dc == nil || dc.renderer == nil || surfaceView == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}
	encoder, encoderOwned, err := dc.frameEncoder(device, "Surface Clear Encoder")
	if err != nil {
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Surface Clear Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: float64(color[0]), G: float64(color[1]), B: float64(color[2]), A: float64(color[3])},
		}},
	})
	if err != nil {
		return
	}
	_ = renderPass.End()
	dc.frameSubmit(queue, encoder, encoderOwned, "Surface Clear Encoder")
}

func (dc *DrawContext) renderIsolateAccumHAL(clearColor [4]float32) bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	r := dc.renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	surfaceView := dc.surfaceTextureView()
	if device == nil || queue == nil || surfaceView == nil {
		return false
	}

	r.mu.RLock()
	accumView := r.resources.OITAccumTextureView
	r.mu.RUnlock()

	if accumView == nil {
		dc.clearSurfaceView(surfaceView, [4]float32{0, 0, 0, 1})
		return true
	}

	r.mu.Lock()
	if err := r.ensurePassIsolateResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("ensurePassIsolateResources failed", "err", err)
		return false
	}
	if r.resources.PassIsolateAccumBindGroup == nil {
		bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Pass Isolate Accum BG",
			Layout: r.resources.PassIsolateBindGroupLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: r.resources.PassIsolateSampler},
				{Binding: 1, TextureView: accumView},
			},
		})
		if err != nil {
			r.mu.Unlock()
			slog.Warn("create isolate accum bind group failed", "err", err)
			return false
		}
		r.resources.PassIsolateAccumBindGroup = bg
	}
	pipeline := r.resources.PassIsolateAccumPipeline
	bindGroup := r.resources.PassIsolateAccumBindGroup
	r.mu.Unlock()

	if pipeline == nil || bindGroup == nil {
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "Isolate Accum Encoder")
	if err != nil {
		return false
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Isolate Accum Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})
	if err != nil {
		return false
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return false
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "Isolate Accum Encoder")
	return true
}

func (dc *DrawContext) renderIsolateRevealHAL(clearColor [4]float32) bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	r := dc.renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	surfaceView := dc.surfaceTextureView()
	if device == nil || queue == nil || surfaceView == nil {
		return false
	}

	r.mu.RLock()
	revealView := r.resources.OITRevealTextureView
	r.mu.RUnlock()

	if revealView == nil {
		dc.clearSurfaceView(surfaceView, [4]float32{1, 1, 1, 1})
		return true
	}

	r.mu.Lock()
	if err := r.ensurePassIsolateResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("ensurePassIsolateResources failed", "err", err)
		return false
	}
	if r.resources.PassIsolateRevealBindGroup == nil {
		bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Pass Isolate Reveal BG",
			Layout: r.resources.PassIsolateBindGroupLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: r.resources.PassIsolateSampler},
				{Binding: 1, TextureView: revealView},
			},
		})
		if err != nil {
			r.mu.Unlock()
			slog.Warn("create isolate reveal bind group failed", "err", err)
			return false
		}
		r.resources.PassIsolateRevealBindGroup = bg
	}
	pipeline := r.resources.PassIsolateRevealPipeline
	bindGroup := r.resources.PassIsolateRevealBindGroup
	r.mu.Unlock()

	if pipeline == nil || bindGroup == nil {
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "Isolate Reveal Encoder")
	if err != nil {
		return false
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Isolate Reveal Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 1, G: 1, B: 1, A: 1},
		}},
	})
	if err != nil {
		return false
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return false
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "Isolate Reveal Encoder")
	return true
}

func (dc *DrawContext) renderIsolateDepthHAL(clearColor [4]float32) bool {
	if dc == nil || dc.renderer == nil {
		return false
	}
	r := dc.renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	surfaceView := dc.surfaceTextureView()
	if device == nil || queue == nil || surfaceView == nil {
		return false
	}

	r.mu.RLock()
	depthTex := r.resources.WorldDepthTexture
	r.mu.RUnlock()

	if depthTex == nil {
		dc.clearSurfaceView(surfaceView, [4]float32{0, 0, 0, 1})
		return true
	}

	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return false
	}

	data, bpr, err := readbackTextureStaging(device, queue, depthTex, width, height, 4, gputypes.TextureAspectAll)
	if err != nil || len(data) == 0 {
		dc.clearSurfaceView(surfaceView, [4]float32{0, 0, 0, 1})
		return true
	}

	grayImg := EncodeDepthBytesToGrayImage(data, width, height, bpr, 4.0, 4096.0)
	if grayImg == nil {
		dc.clearSurfaceView(surfaceView, [4]float32{0, 0, 0, 1})
		return true
	}

	rgba := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		srcRow := y * grayImg.Stride
		dstRow := y * width * 4
		for x := 0; x < width; x++ {
			v := grayImg.Pix[srcRow+x]
			rgba[dstRow+x*4+0] = v
			rgba[dstRow+x*4+1] = v
			rgba[dstRow+x*4+2] = v
			rgba[dstRow+x*4+3] = 255
		}
	}

	r.mu.Lock()
	if err := r.ensurePassIsolateResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("ensurePassIsolateResources failed", "err", err)
		return false
	}

	if r.resources.PassIsolateDepthTexture == nil ||
		r.resources.PassIsolateDepthWidth != width ||
		r.resources.PassIsolateDepthHeight != height {
		if r.resources.PassIsolateDepthBindGroup != nil {
			r.resources.PassIsolateDepthBindGroup.Release()
			r.resources.PassIsolateDepthBindGroup = nil
		}
		if r.resources.PassIsolateDepthTextureView != nil {
			r.resources.PassIsolateDepthTextureView.Release()
			r.resources.PassIsolateDepthTextureView = nil
		}
		if r.resources.PassIsolateDepthTexture != nil {
			r.resources.PassIsolateDepthTexture.Release()
			r.resources.PassIsolateDepthTexture = nil
		}

		tex, err := device.CreateTexture(&wgpu.TextureDescriptor{
			Label: "Pass Isolate Depth RGBA Texture",
			Size: wgpu.Extent3D{
				Width:              uint32(width),
				Height:             uint32(height),
				DepthOrArrayLayers: 1,
			},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     gputypes.TextureDimension2D,
			Format:        gputypes.TextureFormatRGBA8Unorm,
			Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
		})
		if err != nil {
			r.mu.Unlock()
			slog.Warn("create depth isolate RGBA texture failed", "err", err)
			return false
		}
		view, err := device.CreateTextureView(tex, &wgpu.TextureViewDescriptor{
			Label:           "Pass Isolate Depth RGBA Texture View",
			Format:          gputypes.TextureFormatRGBA8Unorm,
			Dimension:       gputypes.TextureViewDimension2D,
			Aspect:          gputypes.TextureAspectAll,
			BaseMipLevel:    0,
			MipLevelCount:   1,
			BaseArrayLayer:  0,
			ArrayLayerCount: 1,
		})
		if err != nil {
			tex.Release()
			r.mu.Unlock()
			slog.Warn("create depth isolate RGBA texture view failed", "err", err)
			return false
		}
		bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Pass Isolate Depth BG",
			Layout: r.resources.PassIsolateBindGroupLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Sampler: r.resources.PassIsolateSampler},
				{Binding: 1, TextureView: view},
			},
		})
		if err != nil {
			view.Release()
			tex.Release()
			r.mu.Unlock()
			slog.Warn("create depth isolate bind group failed", "err", err)
			return false
		}

		r.resources.PassIsolateDepthTexture = tex
		r.resources.PassIsolateDepthTextureView = view
		r.resources.PassIsolateDepthBindGroup = bg
		r.resources.PassIsolateDepthWidth = width
		r.resources.PassIsolateDepthHeight = height
	}

	depthTexObj := r.resources.PassIsolateDepthTexture
	pipeline := r.resources.PassIsolateBlitPipeline
	bindGroup := r.resources.PassIsolateDepthBindGroup
	r.mu.Unlock()

	if depthTexObj == nil || pipeline == nil || bindGroup == nil {
		return false
	}

	if err := queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  depthTexObj,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		rgba,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(width * 4),
			RowsPerImage: uint32(height),
		},
		&wgpu.Extent3D{
			Width:              uint32(width),
			Height:             uint32(height),
			DepthOrArrayLayers: 1,
		},
	); err != nil {
		slog.Warn("write depth isolate texture failed", "err", err)
		return false
	}

	encoder, encoderOwned, err := dc.frameEncoder(device, "Isolate Depth Encoder")
	if err != nil {
		return false
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Isolate Depth Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       surfaceView,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})
	if err != nil {
		return false
	}
	renderPass.SetPipeline(pipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.Draw(3, 1, 0, 0)
	if err := renderPass.End(); err != nil {
		return false
	}
	dc.frameSubmit(queue, encoder, encoderOwned, "Isolate Depth Encoder")
	return true
}
