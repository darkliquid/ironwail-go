//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
)

type gpuDecalVertex struct {
	Position [3]float32
	TexCoord [2]float32
	Color    [4]float32
}

const decalUniformBufferSize = 80

const decalVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) color: vec4<f32>,
}

struct DecalUniforms {
    viewProjection: mat4x4<f32>,
    alpha: f32,
    _pad0: vec3<f32>,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: DecalUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.color = input.color;
    return output;
}
`

const decalFragmentShaderWGSL = `
struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@group(1) @binding(0)
var decalSampler: sampler;

@group(1) @binding(1)
var decalTexture: texture_2d<f32>;

@group(1) @binding(2)
var unusedTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let sampled = textureSample(decalTexture, decalSampler, input.texCoord);
    if (sampled.a < 0.01) {
        discard;
    }
    return vec4<f32>(input.color.rgb * sampled.rgb, input.color.a * sampled.a);
}
`

func (r *Renderer) destroyDecalResourcesLocked() {
	if r.decalBindGroup != nil {
		r.decalBindGroup.Destroy()
		r.decalBindGroup = nil
	}
	if r.decalAtlasView != nil {
		r.decalAtlasView.Destroy()
		r.decalAtlasView = nil
	}
	if r.decalAtlasTextureHAL != nil {
		r.decalAtlasTextureHAL.Destroy()
		r.decalAtlasTextureHAL = nil
	}
	if r.decalUniformBuffer != nil {
		r.decalUniformBuffer.Destroy()
		r.decalUniformBuffer = nil
	}
	if r.decalUniformBindGroup != nil {
		r.decalUniformBindGroup.Destroy()
		r.decalUniformBindGroup = nil
	}
	if r.decalUniformLayout != nil {
		r.decalUniformLayout.Destroy()
		r.decalUniformLayout = nil
	}
	if r.decalPipelineLayout != nil {
		r.decalPipelineLayout.Destroy()
		r.decalPipelineLayout = nil
	}
	if r.decalPipeline != nil {
		r.decalPipeline.Destroy()
		r.decalPipeline = nil
	}
	if r.decalVertexShader != nil {
		r.decalVertexShader.Destroy()
		r.decalVertexShader = nil
	}
	if r.decalFragmentShader != nil {
		r.decalFragmentShader.Destroy()
		r.decalFragmentShader = nil
	}
}

func (r *Renderer) ensureDecalResourcesLocked(device hal.Device, queue hal.Queue) error {
	if device == nil || queue == nil {
		return fmt.Errorf("nil device or queue")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.decalPipeline != nil && r.decalBindGroup != nil && r.decalUniformBindGroup != nil {
		return nil
	}

	vertexShader, err := createWorldShaderModule(device, decalVertexShaderWGSL, "Decal Vertex Shader")
	if err != nil {
		return fmt.Errorf("create decal vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, decalFragmentShaderWGSL, "Decal Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		return fmt.Errorf("create decal fragment shader: %w", err)
	}

	uniformLayout, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "Decal Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: gputypes.ShaderStageVertex,
			Buffer: &gputypes.BufferBindingLayout{
				Type:             gputypes.BufferBindingTypeUniform,
				HasDynamicOffset: false,
				MinBindingSize:   decalUniformBufferSize,
			},
		}},
	})
	if err != nil {
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform layout: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "Decal Pipeline Layout",
		BindGroupLayouts: []hal.BindGroupLayout{uniformLayout, r.aliasTextureBindGroupLayout},
	})
	if err != nil {
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal pipeline layout: %w", err)
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Decal Uniform Buffer",
		Size:             decalUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Decal Uniform BG",
		Layout: uniformLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   decalUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal uniform bind group: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Decal Render Pipeline",
		Layout: pipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 36,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x4, Offset: 20, ShaderLocation: 2},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: &hal.DepthStencilState{
			Format:            worldDepthTextureFormat,
			DepthWriteEnabled: false,
			DepthCompare:      gputypes.CompareFunctionLessEqual,
			StencilReadMask:   0xFFFFFFFF,
			StencilWriteMask:  0xFFFFFFFF,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
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
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal pipeline: %w", err)
	}

	atlasData := generateDecalAtlasData()
	atlasTexture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Decal Atlas Texture",
		Size:          hal.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal atlas texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  atlasTexture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, atlasData, &hal.ImageDataLayout{BytesPerRow: 256 * 4, RowsPerImage: 256}, &hal.Extent3D{Width: 256, Height: 256, DepthOrArrayLayers: 1}); err != nil {
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("write decal atlas texture: %w", err)
	}
	atlasView, err := device.CreateTextureView(atlasTexture, &hal.TextureViewDescriptor{
		Label:           "Decal Atlas View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal atlas view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Decal Atlas BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: atlasView.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: atlasView.NativeHandle()}},
		},
	})
	if err != nil {
		atlasView.Destroy()
		atlasTexture.Destroy()
		pipeline.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		pipelineLayout.Destroy()
		uniformLayout.Destroy()
		vertexShader.Destroy()
		fragmentShader.Destroy()
		return fmt.Errorf("create decal bind group: %w", err)
	}

	r.decalVertexShader = vertexShader
	r.decalFragmentShader = fragmentShader
	r.decalUniformLayout = uniformLayout
	r.decalPipelineLayout = pipelineLayout
	r.decalUniformBuffer = uniformBuffer
	r.decalUniformBindGroup = uniformBindGroup
	r.decalPipeline = pipeline
	r.decalAtlasTextureHAL = atlasTexture
	r.decalAtlasView = atlasView
	r.decalBindGroup = bindGroup
	return nil
}

func (dc *DrawContext) renderDecalMarksHAL(marks []DecalMarkEntity) {
	if dc == nil || dc.renderer == nil || len(marks) == 0 {
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
	if err := r.ensureDecalResourcesLocked(device, queue); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure decal resources", "error", err)
		return
	}
	if err := r.ensureAliasScratchBufferLocked(device, 36*6); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure decal scratch buffer", "error", err)
		return
	}
	pipeline := r.decalPipeline
	uniformBuffer := r.decalUniformBuffer
	uniformBindGroup := r.decalUniformBindGroup
	bindGroup := r.decalBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()

	draws := prepareDecalDraws(marks, camera)
	if len(draws) == 0 || pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || bindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Decal Render Encoder"})
	if err != nil {
		slog.Warn("failed to create decal encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("decal"); err != nil {
		slog.Warn("failed to begin decal encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Decal Render Pass",
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
	renderPass.SetBindGroup(1, bindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	if err := queue.WriteBuffer(uniformBuffer, 0, aliasUniformBytes(vpMatrix, 1)); err != nil {
		slog.Warn("failed to upload decal uniform buffer", "error", err)
		return
	}

	for _, draw := range draws {
		verts := buildDecalVerticesHAL(draw.mark)
		if len(verts) == 0 {
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, decalVertexBytesHAL(verts)); err != nil {
			slog.Warn("failed to upload decal vertices", "error", err)
			continue
		}
		renderPass.Draw(uint32(len(verts)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish decal encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit decal commands", "error", err)
	}
}

func buildDecalVerticesHAL(mark DecalMarkEntity) []gpuDecalVertex {
	corners, ok := buildDecalQuad(mark)
	if !ok {
		return nil
	}
	color := [4]float32{clamp01(mark.Color[0]), clamp01(mark.Color[1]), clamp01(mark.Color[2]), clamp01(mark.Alpha)}
	baseX := float32(int(mark.Variant)%2) * 0.5
	baseY := float32(int(mark.Variant)/2) * 0.5
	uv := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	indices := [6]int{0, 1, 2, 0, 2, 3}

	out := make([]gpuDecalVertex, 0, len(indices))
	for _, idx := range indices {
		coord := uv[idx]
		out = append(out, gpuDecalVertex{
			Position: corners[idx],
			TexCoord: [2]float32{baseX + coord[0]*0.5, baseY + coord[1]*0.5},
			Color:    color,
		})
	}
	return out
}

func decalVertexBytesHAL(vertices []gpuDecalVertex) []byte {
	data := make([]byte, len(vertices)*36)
	for i, v := range vertices {
		offset := i * 36
		putFloat32s(data[offset:offset+12], v.Position[:])
		putFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32s(data[offset+20:offset+36], v.Color[:])
	}
	return data
}
