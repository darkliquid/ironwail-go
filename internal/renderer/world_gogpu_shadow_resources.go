package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	aliasShadowSegments = 16
	aliasShadowAlpha    = 0.5
	aliasShadowLift     = 0.1
	aliasShadowMinSize  = 8.0
	aliasShadowMaxSize  = 48.0
)

func (dc *DrawContext) renderAliasShadowsHAL(entities []AliasModelEntity, fogColor [3]float32, fogDensity float32) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	if cvar.FloatValue(CvarRShadows) <= 0 {
		return
	}

	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}

	excludedModels := parseAliasShadowExclusionsGO(cvar.StringValue(CvarRNoshadowList))
	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias resources for shadows", "error", err)
		return
	}
	if err := r.ensureAliasShadowSkinLocked(device, queue); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias shadow skin", "error", err)
		return
	}
	shadowSkin := r.aliasShadowSkin
	r.mu.Unlock()
	if shadowSkin == nil {
		return
	}

	draws := make([]gpuAliasShadowDraw, 0, len(entities))
	for _, entity := range entities {
		modelID := strings.ToLower(entity.ModelID)
		if _, skip := excludedModels[modelID]; skip {
			continue
		}
		if _, visible := visibleEntityAlpha(entity.Alpha); !visible {
			continue
		}
		vertices := buildAliasShadowVertices(entity)
		if len(vertices) == 0 {
			continue
		}
		draws = append(draws, gpuAliasShadowDraw{vertices: vertices})
	}
	if len(draws) == 0 {
		return
	}

	dc.renderAliasShadowDrawsHAL(draws, shadowSkin, fogColor, fogDensity)
}

type gpuAliasShadowDraw struct {
	vertices []WorldVertex
}

func (dc *DrawContext) renderAliasShadowDrawsHAL(draws []gpuAliasShadowDraw, shadowSkin *gpuAliasSkin, fogColor [3]float32, fogDensity float32) {
	if len(draws) == 0 || shadowSkin == nil || shadowSkin.bindGroup == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentWGPURenderTargetView()
	if textureView == nil {
		return
	}

	// Filter and compute total vertex bytes.
	type preparedShadow struct {
		vertices []WorldVertex
	}
	prepared := make([]preparedShadow, 0, len(draws))
	totalVertexBytes := uint64(0)
	for _, draw := range draws {
		if len(draw.vertices) == 0 {
			continue
		}
		prepared = append(prepared, preparedShadow{vertices: draw.vertices})
		totalVertexBytes += uint64(len(draw.vertices) * 44)
	}
	if len(prepared) == 0 {
		return
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, totalVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer for shadows", "error", err)
		return
	}
	if err := r.ensureAliasUniformBufferLocked(device, len(prepared)); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias uniform buffer for shadows", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	shadowPipeline := r.aliasShadowPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	camera := r.cameraState
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}
	if shadowPipeline != nil {
		pipeline = shadowPipeline
	}

	vpMatrix := r.GetViewProjectionMatrix()
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}

	// Pre-upload all uniform and vertex data.
	// Shadows all share the same uniform data, but we still use dynamic offsets
	// so the bind group layout is consistent.
	shadowUniform := aliasShadowUniformBytes(vpMatrix, cameraOrigin, aliasShadowAlpha, fogColor, fogDensity)
	vertexOffsets := make([]uint64, len(prepared))
	vertexCounts := make([]uint32, len(prepared))
	uniformOffsets := make([]uint32, len(prepared))
	currentVertexOffset := uint64(0)
	for i, pd := range prepared {
		uniformOffsets[i] = uint32(i) * aliasUniformAlign
		vertexOffsets[i] = currentVertexOffset
		vertexCounts[i] = uint32(len(pd.vertices))

		if err := queue.WriteBuffer(uniformBuffer, uint64(uniformOffsets[i]), shadowUniform); err != nil {
			slog.Warn("failed to update alias shadow uniform buffer", "error", err, "draw", i)
			return
		}
		if err := queue.WriteBuffer(scratchBuffer, currentVertexOffset, aliasVertexBytes(pd.vertices)); err != nil {
			slog.Warn("failed to upload alias shadow vertices", "error", err, "draw", i)
			return
		}
		currentVertexOffset += uint64(len(pd.vertices) * 44)
	}

	// Record a single render pass with all shadow draws.
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Alias Shadow Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias shadow encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Alias Shadow Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	if err != nil {
		slog.Warn("failed to begin alias shadow render pass", "error", err)
		return
	}
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}

	for i, pd := range prepared {
		renderPass.SetVertexBuffer(0, scratchBuffer, vertexOffsets[i])
		renderPass.SetBindGroup(0, uniformBindGroup, []uint32{uniformOffsets[i]})
		renderPass.SetBindGroup(1, shadowSkin.bindGroup, nil)
		renderPass.Draw(vertexCounts[i], 1, 0, 0)
		_ = pd // used via vertexOffsets/vertexCounts
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderAliasShadowsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("failed to finish alias shadow encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("failed to submit alias shadow commands", "error", err)
	}
}

func (r *Renderer) ensureAliasShadowSkinLocked(device *wgpu.Device, queue *wgpu.Queue) error {
	if r.aliasShadowSkin != nil {
		return nil
	}
	if r.aliasTextureBindGroupLayout == nil || r.aliasSampler == nil {
		return fmt.Errorf("alias texture bind group layout not ready")
	}
	skin, err := r.createAliasSkinLocked(device, queue, 1, 1, []byte{0})
	if err != nil {
		return err
	}
	r.aliasShadowSkin = &skin
	return nil
}

func buildAliasShadowVertices(entity AliasModelEntity) []WorldVertex {
	if entity.Model == nil || entity.Model.AliasHeader == nil {
		return nil
	}

	modelScale := entity.Scale
	if modelScale == 0 {
		modelScale = 1
	}
	mins := entity.Model.Mins
	maxs := entity.Model.Maxs
	spanX := (maxs[0] - mins[0]) * modelScale
	spanY := (maxs[1] - mins[1]) * modelScale
	shadowRadius := 0.5 * float32(math.Max(float64(spanX), float64(spanY)))
	if shadowRadius < aliasShadowMinSize {
		shadowRadius = aliasShadowMinSize
	}
	if shadowRadius > aliasShadowMaxSize {
		shadowRadius = aliasShadowMaxSize
	}

	shadowZ := entity.Origin[2] + mins[2]*modelScale + aliasShadowLift
	center := WorldVertex{
		Position:      [3]float32{entity.Origin[0], entity.Origin[1], shadowZ},
		TexCoord:      [2]float32{0.5, 0.5},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices := make([]WorldVertex, 0, aliasShadowSegments*3)
	for i := 0; i < aliasShadowSegments; i++ {
		a0 := float32(i) * 2 * float32(math.Pi) / aliasShadowSegments
		a1 := float32(i+1) * 2 * float32(math.Pi) / aliasShadowSegments
		p0 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a0)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a0)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{0, 0},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		p1 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a1)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a1)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{1, 1},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		vertices = append(vertices, center, p0, p1)
	}
	return vertices
}

func parseAliasShadowExclusionsGO(value string) map[string]struct{} {
	return parseAliasModelList(value)
}

// ---- merged from world_cleanup_gogpu_root.go ----
func (r *Renderer) clearAliasModelsLocked() {
	for key, cached := range r.aliasModels {
		for _, skin := range cached.skins {
			if skin.bindGroup != nil {
				skin.bindGroup.Release()
			}
			if skin.fullbrightView != nil {
				skin.fullbrightView.Release()
			}
			if skin.fullbrightTexture != nil {
				skin.fullbrightTexture.Release()
			}
			if skin.view != nil {
				skin.view.Release()
			}
			if skin.texture != nil {
				skin.texture.Release()
			}
		}
		for _, variants := range cached.playerSkins {
			for _, skin := range variants {
				if skin.bindGroup != nil {
					skin.bindGroup.Release()
				}
				if skin.fullbrightView != nil {
					skin.fullbrightView.Release()
				}
				if skin.fullbrightTexture != nil {
					skin.fullbrightTexture.Release()
				}
				if skin.view != nil {
					skin.view.Release()
				}
				if skin.texture != nil {
					skin.texture.Release()
				}
			}
		}
		delete(r.aliasModels, key)
	}
}

func (r *Renderer) destroyAliasResourcesLocked() {
	r.clearAliasModelsLocked()
	if r.aliasShadowSkin != nil {
		if r.aliasShadowSkin.bindGroup != nil {
			r.aliasShadowSkin.bindGroup.Release()
		}
		if r.aliasShadowSkin.fullbrightView != nil {
			r.aliasShadowSkin.fullbrightView.Release()
		}
		if r.aliasShadowSkin.fullbrightTexture != nil {
			r.aliasShadowSkin.fullbrightTexture.Release()
		}
		if r.aliasShadowSkin.view != nil {
			r.aliasShadowSkin.view.Release()
		}
		if r.aliasShadowSkin.texture != nil {
			r.aliasShadowSkin.texture.Release()
		}
		r.aliasShadowSkin = nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Release()
		r.aliasScratchBuffer = nil
	}
	if r.brushEntityScratchVertexBuffer != nil {
		r.brushEntityScratchVertexBuffer.Release()
		r.brushEntityScratchVertexBuffer = nil
	}
	if r.brushEntityScratchIndexBuffer != nil {
		r.brushEntityScratchIndexBuffer.Release()
		r.brushEntityScratchIndexBuffer = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Release()
		r.aliasUniformBuffer = nil
	}
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Release()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasSampler != nil {
		r.aliasSampler.Release()
		r.aliasSampler = nil
	}
	if r.aliasPipeline != nil {
		r.aliasPipeline.Release()
		r.aliasPipeline = nil
	}
	if r.aliasShadowPipeline != nil {
		r.aliasShadowPipeline.Release()
		r.aliasShadowPipeline = nil
	}
	if r.aliasPipelineLayout != nil {
		r.aliasPipelineLayout.Release()
		r.aliasPipelineLayout = nil
	}
	if r.aliasVertexShader != nil {
		r.aliasVertexShader.Release()
		r.aliasVertexShader = nil
	}
	if r.aliasFragmentShader != nil {
		r.aliasFragmentShader.Release()
		r.aliasFragmentShader = nil
	}
	if r.aliasUniformBindGroupLayout != nil {
		r.aliasUniformBindGroupLayout.Release()
		r.aliasUniformBindGroupLayout = nil
	}
	if r.aliasTextureBindGroupLayout != nil {
		r.aliasTextureBindGroupLayout.Release()
		r.aliasTextureBindGroupLayout = nil
	}
}

func (r *Renderer) clearSpriteModelsLocked() {
	for key, cached := range r.spriteModels {
		for _, frame := range cached.frames {
			if frame.bindGroup != nil {
				frame.bindGroup.Release()
			}
			if frame.view != nil {
				frame.view.Release()
			}
			if frame.texture != nil {
				frame.texture.Release()
			}
		}
		delete(r.spriteModels, key)
	}
}

func (r *Renderer) destroySpriteResourcesLocked() {
	r.clearSpriteModelsLocked()
	if r.spriteUniformBuffer != nil {
		r.spriteUniformBuffer.Release()
		r.spriteUniformBuffer = nil
	}
	if r.spriteUniformBindGroup != nil {
		r.spriteUniformBindGroup.Release()
		r.spriteUniformBindGroup = nil
	}
	if r.spritePipeline != nil {
		r.spritePipeline.Release()
		r.spritePipeline = nil
	}
	if r.spriteDepthOffsetPipeline != nil {
		r.spriteDepthOffsetPipeline.Release()
		r.spriteDepthOffsetPipeline = nil
	}
	if r.spriteVertexShader != nil {
		r.spriteVertexShader.Release()
		r.spriteVertexShader = nil
	}
	if r.spriteFragmentShader != nil {
		r.spriteFragmentShader.Release()
		r.spriteFragmentShader = nil
	}
}

func (r *Renderer) destroyDecalResourcesLocked() {
	if r.decalBindGroup != nil {
		r.decalBindGroup.Release()
		r.decalBindGroup = nil
	}
	if r.decalAtlasView != nil {
		r.decalAtlasView.Release()
		r.decalAtlasView = nil
	}
	if r.decalAtlasTextureHAL != nil {
		r.decalAtlasTextureHAL.Release()
		r.decalAtlasTextureHAL = nil
	}
	if r.decalUniformBuffer != nil {
		r.decalUniformBuffer.Release()
		r.decalUniformBuffer = nil
	}
	if r.decalUniformBindGroup != nil {
		r.decalUniformBindGroup.Release()
		r.decalUniformBindGroup = nil
	}
	if r.decalUniformLayout != nil {
		r.decalUniformLayout.Release()
		r.decalUniformLayout = nil
	}
	if r.decalPipelineLayout != nil {
		r.decalPipelineLayout.Release()
		r.decalPipelineLayout = nil
	}
	if r.decalPipeline != nil {
		r.decalPipeline.Release()
		r.decalPipeline = nil
	}
	if r.decalVertexShader != nil {
		r.decalVertexShader.Release()
		r.decalVertexShader = nil
	}
	if r.decalFragmentShader != nil {
		r.decalFragmentShader.Release()
		r.decalFragmentShader = nil
	}
}

// ---- merged from world_support_gogpu_root.go ----
type gpuAliasSkin struct {
	texture           *wgpu.Texture
	view              *wgpu.TextureView
	fullbrightTexture *wgpu.Texture
	fullbrightView    *wgpu.TextureView
	bindGroup         *wgpu.BindGroup
}

type gpuAliasModel struct {
	modelID     string
	flags       int
	skins       []gpuAliasSkin
	playerSkins map[uint32][]gpuAliasSkin
	poses       [][]model.TriVertX
	refs        []aliasimpl.MeshRef
}

type gpuAliasDraw struct {
	alias  *gpuAliasModel
	model  *model.Model
	pose1  int
	pose2  int
	blend  float32
	skin   *gpuAliasSkin
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
	full   bool
}

type gpuSpriteFrame struct {
	meta      spriteRenderFrame
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
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

func (r *Renderer) ensureAliasScratchBufferLocked(device *wgpu.Device, size uint64) error {
	if size == 0 {
		size = 44
	}
	if r.aliasScratchBuffer != nil && r.aliasScratchBufferSize >= size {
		return nil
	}
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Release()
		r.aliasScratchBuffer = nil
		r.aliasScratchBufferSize = 0
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Scratch Buffer",
		Size:             size,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create alias scratch buffer: %w", err)
	}
	r.aliasScratchBuffer = buffer
	r.aliasScratchBufferSize = size
	return nil
}

func (r *Renderer) ensureBrushEntityScratchBuffersLocked(device *wgpu.Device, vertexSize, indexSize uint64) error {
	if vertexSize == 0 {
		vertexSize = 44
	}
	if indexSize == 0 {
		indexSize = 4
	}
	if r.brushEntityScratchVertexBuffer != nil && r.brushEntityScratchVertexSize >= vertexSize &&
		r.brushEntityScratchIndexBuffer != nil && r.brushEntityScratchIndexSize >= indexSize {
		return nil
	}
	if r.brushEntityScratchVertexBuffer != nil {
		r.brushEntityScratchVertexBuffer.Release()
		r.brushEntityScratchVertexBuffer = nil
		r.brushEntityScratchVertexSize = 0
	}
	if r.brushEntityScratchIndexBuffer != nil {
		r.brushEntityScratchIndexBuffer.Release()
		r.brushEntityScratchIndexBuffer = nil
		r.brushEntityScratchIndexSize = 0
	}
	vertexBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Brush Entity Vertex Scratch Buffer",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create brush entity vertex scratch buffer: %w", err)
	}
	indexBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Brush Entity Index Scratch Buffer",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		vertexBuffer.Release()
		return fmt.Errorf("create brush entity index scratch buffer: %w", err)
	}
	r.brushEntityScratchVertexBuffer = vertexBuffer
	r.brushEntityScratchVertexSize = vertexSize
	r.brushEntityScratchIndexBuffer = indexBuffer
	r.brushEntityScratchIndexSize = indexSize
	return nil
}

// ensureAliasUniformBufferLocked grows the alias uniform buffer and rebuilds
// the bind group when the current buffer is too small for numDraws draws.
func (r *Renderer) ensureAliasUniformBufferLocked(device *wgpu.Device, numDraws int) error {
	needed := uint64(numDraws) * aliasUniformAlign
	if needed < aliasSceneUniformBufferSize {
		needed = aliasSceneUniformBufferSize
	}
	if r.aliasUniformBuffer != nil && r.aliasUniformBuffer.Size() >= needed {
		return nil
	}
	// Release old resources.
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Release()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Release()
		r.aliasUniformBuffer = nil
	}
	buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             needed,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("grow alias uniform buffer: %w", err)
	}
	bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Alias Uniform BG",
		Layout:  r.aliasUniformBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: buf, Offset: 0, Size: aliasSceneUniformBufferSize}},
	})
	if err != nil {
		buf.Release()
		return fmt.Errorf("recreate alias uniform bind group: %w", err)
	}
	r.aliasUniformBuffer = buf
	r.aliasUniformBindGroup = bg
	return nil
}

func aliasDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpLoad,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpLoad,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   true,
	}
}

func putFloat32s(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

// ---- merged from world_translucent_sort_gogpu_root.go ----
func destroyGoGPUTransientBuffers(buffers []*wgpu.Buffer) {
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Release()
		}
	}
}
