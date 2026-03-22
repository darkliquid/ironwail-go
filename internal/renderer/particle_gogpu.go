//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/pkg/types"
)

const (
	particleUniformBufferSize = 112
	particleBatchCapacity     = 512
)

const particleVertexShaderWGSL = `
struct ParticleInstance {
    @location(0) position: vec3<f32>,
    @location(1) color: vec4<f32>,
}

struct ParticleUniforms {
    viewProjection: mat4x4<f32>,
    projScale: vec2<f32>,
    uvScale: f32,
    _pad0: f32,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    _pad1: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) color: vec4<f32>,
    @location(2) fogPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: ParticleUniforms;

@vertex
fn vs_main(instance: ParticleInstance, @builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var corners = array<vec2<f32>, 4>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(-1.0,  1.0),
        vec2<f32>( 1.0, -1.0),
        vec2<f32>( 1.0,  1.0),
    );
    let corner = corners[vertexIndex & 3u];
    var clipPosition = uniforms.viewProjection * vec4<f32>(instance.position, 1.0);
    let depthScale = max(1.0 + clipPosition.w * 0.004, 1.08);
    clipPosition.xy += uniforms.projScale * corner * depthScale;

    var output: VertexOutput;
    output.clipPosition = clipPosition;
    output.uv = corner * uniforms.uvScale;
    output.color = instance.color;
    output.fogPosition = instance.position - uniforms.cameraOrigin;
    return output;
}
`

const particleFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) color: vec4<f32>,
    @location(2) fogPosition: vec3<f32>,
}

struct ParticleUniforms {
    viewProjection: mat4x4<f32>,
    projScale: vec2<f32>,
    uvScale: f32,
    _pad0: f32,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    _pad1: f32,
}

@group(0) @binding(0)
var<uniform> uniforms: ParticleUniforms;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    var color = input.color;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(input.fogPosition, input.fogPosition)), 0.0, 1.0);
    color.rgb = mix(uniforms.fogColor, color.rgb, fog);
    let radius = length(input.uv);
    let pixel = fwidth(radius);
    color.a *= clamp((1.0 - radius) / pixel, 0.0, 1.0);
    if (color.a <= 0.0) {
        discard;
    }
    return color;
}
`

func (r *Renderer) destroyParticleResourcesLocked() {
	if r.particleUniformBindGroup != nil {
		r.particleUniformBindGroup.Destroy()
		r.particleUniformBindGroup = nil
	}
	if r.particleUniformBuffer != nil {
		r.particleUniformBuffer.Destroy()
		r.particleUniformBuffer = nil
	}
	if r.particleOpaquePipeline != nil {
		r.particleOpaquePipeline.Destroy()
		r.particleOpaquePipeline = nil
	}
	if r.particleTranslucentPipeline != nil {
		r.particleTranslucentPipeline.Destroy()
		r.particleTranslucentPipeline = nil
	}
	if r.particlePipelineLayout != nil {
		r.particlePipelineLayout.Destroy()
		r.particlePipelineLayout = nil
	}
	if r.particleUniformBindGroupLayout != nil {
		r.particleUniformBindGroupLayout.Destroy()
		r.particleUniformBindGroupLayout = nil
	}
	if r.particleVertexShader != nil {
		r.particleVertexShader.Destroy()
		r.particleVertexShader = nil
	}
	if r.particleFragmentShader != nil {
		r.particleFragmentShader.Destroy()
		r.particleFragmentShader = nil
	}
}

func (r *Renderer) ensureParticleResourcesLocked(device hal.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if r.particleOpaquePipeline != nil && r.particleTranslucentPipeline != nil && r.particleUniformBuffer != nil && r.particleUniformBindGroup != nil {
		return nil
	}

	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Particle Uniform Layout",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   particleUniformBufferSize,
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("create particle uniform layout: %w", err)
	}
	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Particle Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout},
	})
	if err != nil {
		uniformLayout.Destroy()
		return fmt.Errorf("create particle pipeline layout: %w", err)
	}
	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Particle Uniform Buffer",
		Size:             particleUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create particle uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Particle Uniform BG",
		Layout: uniformLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   particleUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create particle uniform bind group: %w", err)
	}
	vertexShader, err := createWorldShaderModule(device, particleVertexShaderWGSL, "Particle Vertex Shader")
	if err != nil {
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create particle vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, particleFragmentShaderWGSL, "Particle Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create particle fragment shader: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	vertexState := hal.VertexState{
		Module:     vertexShader,
		EntryPoint: "vs_main",
		Buffers: []gputypes.VertexBufferLayout{{
			ArrayStride: uint64(unsafe.Sizeof(ParticleVertex{})),
			StepMode:    gputypes.VertexStepModeInstance,
			Attributes: []gputypes.VertexAttribute{
				{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
				{Format: gputypes.VertexFormatUnorm8x4, Offset: 12, ShaderLocation: 1},
			},
		}},
	}
	createPipeline := func(label string, depthWrite bool, blend *gputypes.BlendState) (hal.RenderPipeline, error) {
		return device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
			Label:  label,
			Layout: pipelineLayout,
			Vertex: vertexState,
			Primitive: gputypes.PrimitiveState{
				Topology:  gputypes.PrimitiveTopologyTriangleStrip,
				FrontFace: gputypes.FrontFaceCCW,
				CullMode:  gputypes.CullModeNone,
			},
			DepthStencil: &hal.DepthStencilState{
				Format:            worldDepthTextureFormat,
				DepthWriteEnabled: depthWrite,
				DepthCompare:      gputypes.CompareFunctionLessEqual,
				StencilReadMask:   0xFFFFFFFF,
				StencilWriteMask:  0xFFFFFFFF,
			},
			Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
			Fragment: &hal.FragmentState{
				Module:     fragmentShader,
				EntryPoint: "fs_main",
				Targets: []gputypes.ColorTargetState{{
					Format:    surfaceFormat,
					Blend:     blend,
					WriteMask: gputypes.ColorWriteMaskAll,
				}},
			},
		})
	}
	opaquePipeline, err := createPipeline("Particle Opaque Pipeline", true, nil)
	if err != nil {
		fragmentShader.Destroy()
		vertexShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create opaque particle pipeline: %w", err)
	}
	translucentPipeline, err := createPipeline("Particle Translucent Pipeline", false, &gputypes.BlendState{
		Color: gputypes.BlendComponent{
			SrcFactor: gputypes.BlendFactorSrcAlpha,
			DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
			Operation: gputypes.BlendOperationAdd,
		},
		Alpha: gputypes.BlendComponent{
			SrcFactor: gputypes.BlendFactorOne,
			DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
			Operation: gputypes.BlendOperationAdd,
		},
	})
	if err != nil {
		opaquePipeline.Destroy()
		fragmentShader.Destroy()
		vertexShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		return fmt.Errorf("create translucent particle pipeline: %w", err)
	}

	r.particleUniformBindGroupLayout = uniformLayout
	r.particlePipelineLayout = pipelineLayout
	r.particleUniformBuffer = uniformBuffer
	r.particleUniformBindGroup = uniformBindGroup
	r.particleVertexShader = vertexShader
	r.particleFragmentShader = fragmentShader
	r.particleOpaquePipeline = opaquePipeline
	r.particleTranslucentPipeline = translucentPipeline
	return nil
}

func particleVerticesForGoGPUPass(vertices []ParticleVertex, mode int, alpha bool) []ParticleVertex {
	if !ShouldDrawParticles(mode, alpha, false, len(vertices)) {
		return nil
	}
	return vertices
}

func particleVertexBytes(vertices []ParticleVertex) []byte {
	if len(vertices) == 0 {
		return nil
	}
	raw := unsafe.Slice((*byte)(particleVertexPtr(vertices)), len(vertices)*int(unsafe.Sizeof(ParticleVertex{})))
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func particleUniformBytes(vp types.Mat4, projScale [2]float32, uvScale float32, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, particleUniformBufferSize)
	copy(data[:64], matrixToBytes(vp))
	putFloat32s(data[64:72], projScale[:])
	binary.LittleEndian.PutUint32(data[72:76], math.Float32bits(uvScale))
	putFloat32s(data[80:92], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[96:108], fogColor[:])
	return data
}

func (dc *DrawContext) renderParticlesHAL(state *RenderFrameState, alpha bool) {
	if dc == nil || dc.renderer == nil || state == nil || state.Particles == nil || state.Particles.ActiveCount() == 0 {
		return
	}
	mode := readGoGPUParticleModeCvar()
	particles := state.Particles.ActiveParticles()
	if len(particles) == 0 {
		return
	}
	palette := buildParticlePalette(state.Palette)
	vertices := BuildParticleVertices(particles, palette, false)
	drawVertices := particleVerticesForGoGPUPass(vertices, mode, alpha)
	if len(drawVertices) == 0 {
		return
	}

	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureParticleResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure particle resources", "error", err)
		return
	}
	if err := r.ensureAliasScratchBufferLocked(device, uint64(particleBatchCapacity)*uint64(unsafe.Sizeof(ParticleVertex{}))); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure particle scratch buffer", "error", err)
		return
	}
	pipeline := r.particleOpaquePipeline
	if alpha {
		pipeline = r.particleTranslucentPipeline
	}
	uniformBuffer := r.particleUniformBuffer
	uniformBindGroup := r.particleUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	vpMatrix := r.GetViewProjectionMatrix()
	projectionMatrix := r.GetProjectionMatrix()
	uvScale, textureScaleFactor := ParticleTexture(mode)
	scaleX, scaleY := ParticleProjection(textureScaleFactor, projectionMatrix)
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	if err := queue.WriteBuffer(uniformBuffer, 0, particleUniformBytes(vpMatrix, [2]float32{scaleX, scaleY}, uvScale, cameraOrigin, state.FogColor, state.FogDensity)); err != nil {
		slog.Warn("failed to update particle uniform buffer", "error", err)
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Particle Render Encoder"})
	if err != nil {
		slog.Warn("failed to create particle encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("particles"); err != nil {
		slog.Warn("failed to begin particle encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Particle Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)

	for len(drawVertices) > 0 {
		batch := drawVertices
		if len(batch) > particleBatchCapacity {
			batch = drawVertices[:particleBatchCapacity]
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, particleVertexBytes(batch)); err != nil {
			slog.Warn("failed to upload particle vertices", "error", err)
			break
		}
		renderPass.Draw(4, uint32(len(batch)), 0, 0)
		drawVertices = drawVertices[len(batch):]
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish particle encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit particle commands", "error", err)
	}
}
