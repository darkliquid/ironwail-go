//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

type gpuSpriteFrame struct {
	meta      spriteRenderFrame
	texture   hal.Texture
	view      hal.TextureView
	bindGroup hal.BindGroup
}

type gpuSpriteModel struct {
	modelID    string
	spriteType int
	frames     []gpuSpriteFrame
	maxWidth   int
	maxHeight  int
	bounds     [3][2]float32
}

type gpuSpriteDraw struct {
	sprite *gpuSpriteModel
	frame  int
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
}

const spriteVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct SpriteUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: SpriteUniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.clipPosition = uniforms.viewProjection * vec4<f32>(input.position, 1.0);
    output.texCoord = input.texCoord;
    output.worldPosition = input.position;
    return output;
}
`

const spriteFragmentShaderWGSL = `
struct SpriteUniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) worldPosition: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: SpriteUniforms;

@group(1) @binding(0)
var spriteSampler: sampler;

@group(1) @binding(1)
var spriteTexture: texture_2d<f32>;

@group(1) @binding(2)
var unusedTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let sampled = textureSample(spriteTexture, spriteSampler, input.texCoord);
    if (sampled.a < 0.01) {
        discard;
    }
    let fogPosition = input.worldPosition - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
    let fogged = mix(uniforms.fogColor, sampled.rgb, fog);
    return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`

const spriteUniformBufferSize = 96

func (r *Renderer) clearSpriteModelsLocked() {
	for key, cached := range r.spriteModels {
		for _, frame := range cached.frames {
			if frame.bindGroup != nil {
				frame.bindGroup.Destroy()
			}
			if frame.view != nil {
				frame.view.Destroy()
			}
			if frame.texture != nil {
				frame.texture.Destroy()
			}
		}
		delete(r.spriteModels, key)
	}
}

func (r *Renderer) destroySpriteResourcesLocked() {
	r.clearSpriteModelsLocked()
	if r.spriteUniformBuffer != nil {
		r.spriteUniformBuffer.Destroy()
		r.spriteUniformBuffer = nil
	}
	if r.spriteUniformBindGroup != nil {
		r.spriteUniformBindGroup.Destroy()
		r.spriteUniformBindGroup = nil
	}
	if r.spritePipeline != nil {
		r.spritePipeline.Destroy()
		r.spritePipeline = nil
	}
	if r.spriteVertexShader != nil {
		r.spriteVertexShader.Destroy()
		r.spriteVertexShader = nil
	}
	if r.spriteFragmentShader != nil {
		r.spriteFragmentShader.Destroy()
		r.spriteFragmentShader = nil
	}
}

func (r *Renderer) ensureSpriteResourcesLocked(device hal.Device) error {
	if device == nil {
		return fmt.Errorf("nil device")
	}
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		return err
	}
	if r.spritePipeline != nil && r.spriteUniformBuffer != nil && r.spriteUniformBindGroup != nil {
		return nil
	}

	uniformBuffer, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label:            "Sprite Uniform Buffer",
		Size:             spriteUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create sprite uniform buffer: %w", err)
	}
	uniformBindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Sprite Uniform BG",
		Layout: r.aliasUniformBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{{
			Binding: 0,
			Resource: gputypes.BufferBinding{
				Buffer: uniformBuffer.NativeHandle(),
				Offset: 0,
				Size:   spriteUniformBufferSize,
			},
		}},
	})
	if err != nil {
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite uniform bind group: %w", err)
	}

	vertexShader, err := createWorldShaderModule(device, spriteVertexShaderWGSL, "Sprite Vertex Shader")
	if err != nil {
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite vertex shader: %w", err)
	}
	fragmentShader, err := createWorldShaderModule(device, spriteFragmentShaderWGSL, "Sprite Fragment Shader")
	if err != nil {
		vertexShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite fragment shader: %w", err)
	}

	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}

	pipeline, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "Sprite Render Pipeline",
		Layout: r.aliasPipelineLayout,
		Vertex: hal.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: 44,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
					{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
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
		vertexShader.Destroy()
		fragmentShader.Destroy()
		uniformBindGroup.Destroy()
		uniformBuffer.Destroy()
		return fmt.Errorf("create sprite pipeline: %w", err)
	}

	r.spriteUniformBuffer = uniformBuffer
	r.spriteUniformBindGroup = uniformBindGroup
	r.spriteVertexShader = vertexShader
	r.spriteFragmentShader = fragmentShader
	r.spritePipeline = pipeline
	return nil
}

func (r *Renderer) createSpriteFrameLocked(device hal.Device, queue hal.Queue, frame spriteRenderFrame) (gpuSpriteFrame, error) {
	width, height := frame.width, frame.height
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	pixels := frame.pixels
	if len(pixels) != width*height {
		pixels = make([]byte, width*height)
	}
	rgba := ConvertPaletteToRGBA(pixels, r.palette)
	texture, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "Sprite Frame Texture",
		Size:          hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return gpuSpriteFrame{}, fmt.Errorf("create texture: %w", err)
	}
	if err := queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &hal.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &hal.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Destroy()
		return gpuSpriteFrame{}, fmt.Errorf("write texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &hal.TextureViewDescriptor{
		Label:           "Sprite Frame View",
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
		return gpuSpriteFrame{}, fmt.Errorf("create texture view: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "Sprite Frame BG",
		Layout: r.aliasTextureBindGroupLayout,
		Entries: []gputypes.BindGroupEntry{
			{Binding: 0, Resource: gputypes.SamplerBinding{Sampler: r.aliasSampler.NativeHandle()}},
			{Binding: 1, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
			{Binding: 2, Resource: gputypes.TextureViewBinding{TextureView: view.NativeHandle()}},
		},
	})
	if err != nil {
		view.Destroy()
		texture.Destroy()
		return gpuSpriteFrame{}, fmt.Errorf("create bind group: %w", err)
	}
	return gpuSpriteFrame{
		meta:      frame,
		texture:   texture,
		view:      view,
		bindGroup: bindGroup,
	}, nil
}

func (r *Renderer) ensureSpriteModelLocked(device hal.Device, queue hal.Queue, modelID string, spr *model.MSprite) *gpuSpriteModel {
	if modelID == "" || spr == nil {
		return nil
	}
	if cached, ok := r.spriteModels[modelID]; ok {
		return cached
	}
	meta := buildSpriteRenderModel(modelID, spr)
	if meta == nil {
		return nil
	}
	frames := make([]gpuSpriteFrame, 0, len(meta.frames))
	for _, frame := range meta.frames {
		gpuFrame, err := r.createSpriteFrameLocked(device, queue, frame)
		if err != nil {
			slog.Warn("failed to upload sprite frame", "model", modelID, "error", err)
			for _, uploaded := range frames {
				if uploaded.bindGroup != nil {
					uploaded.bindGroup.Destroy()
				}
				if uploaded.view != nil {
					uploaded.view.Destroy()
				}
				if uploaded.texture != nil {
					uploaded.texture.Destroy()
				}
			}
			return nil
		}
		frames = append(frames, gpuFrame)
	}
	model := &gpuSpriteModel{
		modelID:    meta.modelID,
		spriteType: meta.spriteType,
		frames:     frames,
		maxWidth:   meta.maxWidth,
		maxHeight:  meta.maxHeight,
		bounds:     meta.bounds,
	}
	r.spriteModels[modelID] = model
	return model
}

func (r *Renderer) buildSpriteDrawLocked(device hal.Device, queue hal.Queue, entity SpriteEntity) *gpuSpriteDraw {
	if entity.ModelID == "" || entity.Model == nil || entity.Model.Type != model.ModSprite || entity.SpriteData == nil {
		return nil
	}
	sprite := r.ensureSpriteModelLocked(device, queue, entity.ModelID, entity.SpriteData)
	if sprite == nil {
		return nil
	}
	frame := entity.Frame
	if frame < 0 || frame >= len(sprite.frames) {
		frame = 0
	}
	alpha, visible := visibleEntityAlpha(entity.Alpha)
	if !visible {
		return nil
	}
	return &gpuSpriteDraw{
		sprite: sprite,
		frame:  frame,
		origin: entity.Origin,
		angles: entity.Angles,
		alpha:  alpha,
		scale:  entity.Scale,
	}
}

func (dc *DrawContext) collectSpriteDraws(entities []SpriteEntity) []gpuSpriteDraw {
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return nil
	}

	r := dc.renderer
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureSpriteResourcesLocked(device); err != nil {
		slog.Warn("failed to ensure sprite resources", "error", err)
		return nil
	}

	draws := make([]gpuSpriteDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildSpriteDrawLocked(device, queue, entity); draw != nil {
			draws = append(draws, *draw)
		}
	}
	return draws
}

func (dc *DrawContext) renderSpriteEntitiesHAL(entities []SpriteEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	draws := dc.collectSpriteDraws(entities)
	if len(draws) == 0 {
		return
	}
	dc.renderSpriteDrawsHAL(draws, fogColor, fogDensity)
}

func (dc *DrawContext) renderSpriteDrawsHAL(draws []gpuSpriteDraw, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(draws) == 0 {
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

	maxVertexBytes := uint64(44 * 6)
	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure sprite scratch buffer", "error", err)
		return
	}
	pipeline := r.spritePipeline
	uniformBuffer := r.spriteUniformBuffer
	uniformBindGroup := r.spriteUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Sprite Render Encoder"})
	if err != nil {
		slog.Warn("failed to create sprite encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("sprite"); err != nil {
		slog.Warn("failed to begin sprite encoding", "error", err)
		return
	}
	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Sprite Render Pass",
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

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraAngles := [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
	cameraForward, cameraRight, cameraUp := spriteCameraBasis(cameraAngles)

	for _, draw := range draws {
		if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
			continue
		}
		vertices := buildSpriteQuadVertices(&spriteRenderModel{
			modelID:    draw.sprite.modelID,
			spriteType: draw.sprite.spriteType,
			frames:     []spriteRenderFrame{draw.sprite.frames[draw.frame].meta},
			maxWidth:   draw.sprite.maxWidth,
			maxHeight:  draw.sprite.maxHeight,
			bounds:     draw.sprite.bounds,
		}, 0, cameraOrigin, draw.origin, draw.angles, cameraForward, cameraRight, cameraUp, draw.scale)
		if len(vertices) == 0 {
			continue
		}
		triangleVertices := expandSpriteQuadVertices(vertices)
		if len(triangleVertices) == 0 {
			continue
		}
		worldVertices := spriteQuadVerticesToWorldVerticesHAL(triangleVertices)
		if err := queue.WriteBuffer(uniformBuffer, 0, spriteUniformBytes(vpMatrix, cameraOrigin, draw.alpha, fogColor, fogDensity)); err != nil {
			slog.Warn("failed to update sprite uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(worldVertices)); err != nil {
			slog.Warn("failed to upload sprite vertices", "error", err)
			continue
		}
		renderPass.SetBindGroup(1, draw.sprite.frames[draw.frame].bindGroup, nil)
		renderPass.Draw(uint32(len(worldVertices)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish sprite encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit sprite commands", "error", err)
	}
}

func spriteQuadVerticesToWorldVerticesHAL(vertices []spriteQuadVertex) []WorldVertex {
	out := make([]WorldVertex, len(vertices))
	for i, vertex := range vertices {
		out[i] = WorldVertex{
			Position:      vertex.Position,
			TexCoord:      vertex.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
	}
	return out
}

func spriteUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	data := make([]byte, spriteUniformBufferSize)
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	putFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return data
}
